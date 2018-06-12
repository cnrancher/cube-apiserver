package api

import (
	"crypto/rsa"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/auth0/go-jwt-middleware"
	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/rancher/go-rancher/client"
	"github.com/urfave/negroni"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
)

const (
	UserPropertyName = "CUBE-USER-PROPERTY"
	PrivateKeyPath   = "/var/lib/rancher/cube/id_rsa"
	PublicKeyPath    = "/var/lib/rancher/cube/id_rsa.pub"
)

var (
	RetryCounts   = 5
	RetryInterval = 100 * time.Millisecond
	JwtVerifyKey  *rsa.PublicKey
	JwtSignKey    *rsa.PrivateKey
)

type HandleFuncWithError func(http.ResponseWriter, *http.Request) error

func init() {
	signBytes, err := ioutil.ReadFile(PrivateKeyPath)
	if err != nil {
		logrus.Fatal(err)
	}

	JwtSignKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes)
	if err != nil {
		logrus.Fatal(err)
	}

	verifyBytes, err := ioutil.ReadFile(PublicKeyPath)
	if err != nil {
		logrus.Fatal(err)
	}

	JwtVerifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
	if err != nil {
		logrus.Fatal(err)
	}
}

func HandleError(s *client.Schemas, t HandleFuncWithError) http.Handler {
	return api.ApiHandler(s, http.HandlerFunc(func(rw http.ResponseWriter, req *http.Request) {
		var err error
		for i := 0; i < RetryCounts; i++ {
			err = t(rw, req)
			if !apierrors.IsConflict(errors.Cause(err)) {
				break
			}
			logrus.Warnf("RancherCUBE: retry API call due to conflict")
			time.Sleep(RetryInterval)
		}
		if err != nil {
			logrus.Warnf("RancherCUBE: http handling error %v", err)
			apiContext := api.GetApiContext(req)
			apiContext.WriteErr(err)
		}
	}))
}

func generatePrivateKey() *jwtmiddleware.JWTMiddleware {
	return jwtmiddleware.New(jwtmiddleware.Options{
		UserProperty: UserPropertyName,
		ValidationKeyGetter: func(token *jwt.Token) (interface{}, error) {
			return JwtVerifyKey, nil
		},
		SigningMethod: jwt.SigningMethodRS256,
	})
}

func TokenObtainMiddleware(w http.ResponseWriter, r *http.Request, next http.HandlerFunc) {
	tokenAuthValue := util.GetTokenAuthFromRequest(r)

	// ignore empty kubeConfig location, this is singleton instance
	clientGenerator := backend.NewClientGenerator("", nil)

	// If there was an error, do not call next.
	if tokenAuthValue != "" && next != nil {
		storedToken, statusCode, err := clientGenerator.GetStoredToken(tokenAuthValue)
		if err != nil && statusCode != 0 {
			util.JSONErrorResponse(err, statusCode, w)
			return
		}

		r.Header.Set("Authorization", "Bearer "+storedToken.Token)
		r.Header.Set("Impersonate-User", storedToken.UserID)

		next(w, r)
	} else {
		util.JSONErrorResponse(errors.New("RancherCUBE: couldn't get token auth form request"), http.StatusNotFound, w)
	}
}

func NewRouter(s *Server) *mux.Router {
	schemas := NewSchema()
	router := mux.NewRouter()
	apiRouter := mux.NewRouter().PathPrefix("/v1").Subrouter().StrictSlash(true)
	inSecureRouter := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
	f := HandleError

	versionsHandler := api.VersionsHandler(schemas, "v1")
	versionHandler := api.VersionHandler(schemas, "v1")

	apiRouter.Methods("GET").Path("/").Handler(versionHandler)
	apiRouter.Methods("GET").Path("/apiversions").Handler(versionsHandler)
	apiRouter.Methods("GET").Path("/apiversions/v1").Handler(versionHandler)
	apiRouter.Methods("GET").Path("/schemas").Handler(api.SchemasHandler(schemas))
	apiRouter.Methods("GET").Path("/schemas/{id}").Handler(api.SchemaHandler(schemas))

	apiRouter.Methods("GET").Path("/nodes").Handler(f(schemas, s.NodeList))
	apiRouter.Methods("GET").Path("/nodes/{id}").Handler(f(schemas, s.NodeGet))
	apiRouter.Methods("GET").Path("/clusters").Handler(f(schemas, s.ClusterList))

	apiRouter.Methods("GET").Path("/baseinfos").Handler(f(schemas, s.BaseInfoGet))

	apiRouter.Methods("GET").Path("/dashboards").Handler(f(schemas, s.DashboardList))
	apiRouter.Methods("GET").Path("/dashboards/{id}").Handler(f(schemas, s.DashboardGet))
	apiRouter.Methods("POST").Path("/dashboards").Handler(f(schemas, s.DashboardCreate))
	apiRouter.Methods("DELETE").Path("/dashboards/{id}").Handler(f(schemas, s.DashboardDelete))

	apiRouter.Methods("GET").Path("/longhorns").Handler(f(schemas, s.LonghornList))
	apiRouter.Methods("GET").Path("/longhorns/{id}").Handler(f(schemas, s.LonghornGet))
	apiRouter.Methods("POST").Path("/longhorns").Handler(f(schemas, s.LonghornCreate))
	apiRouter.Methods("DELETE").Path("/longhorns/{id}").Handler(f(schemas, s.LonghornDelete))

	apiRouter.Methods("GET").Path("/ranchervms").Handler(f(schemas, s.RancherVMList))
	apiRouter.Methods("GET").Path("/ranchervms/{id}").Handler(f(schemas, s.RancherVMGet))
	apiRouter.Methods("POST").Path("/ranchervms").Handler(f(schemas, s.RancherVMCreate))
	apiRouter.Methods("DELETE").Path("/ranchervms/{id}").Handler(f(schemas, s.RancherVMDelete))

	inSecureRouter.Methods("POST").Path("/login").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s.Login(w, req)
	})

	inSecureRouter.Methods("POST").Path("/logout").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s.Logout(w, req)
	})

	commonMiddleware := negroni.New(
		negroni.NewRecovery(),
		negroni.NewLogger(),
	)

	jwtMiddleware := generatePrivateKey()

	router.PathPrefix("/v1").Handler(commonMiddleware.With(
		negroni.HandlerFunc(TokenObtainMiddleware),
		negroni.HandlerFunc(jwtMiddleware.HandlerWithNext),
		negroni.Wrap(apiRouter),
	))

	router.PathPrefix("/").Handler(commonMiddleware.With(
		negroni.Wrap(inSecureRouter),
	))

	return router
}
