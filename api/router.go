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
	PrivateKeyPath   = "/etc/rancher/cube/id_rsa"
	PublicKeyPath    = "/etc/rancher/cube/id_rsa.pub"
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
	clientGenerator := backend.NewClientGenerator("")

	// If there was an error, do not call next.
	if tokenAuthValue != "" && next != nil {
		tokenName, tokenKey := util.SplitTokenParts(tokenAuthValue)
		if tokenName == "" || tokenKey == "" {
			util.JsonErrorResponse(errors.New("RancherCUBE: couldn't split token"), http.StatusUnauthorized, w)
			return
		}

		storedToken, err := clientGenerator.CheckTokenCache(tokenKey)
		useTokenClient := false
		if err != nil {
			if apierrors.IsNotFound(err) {
				useTokenClient = true
			} else {
				util.JsonErrorResponse(errors.New("RancherCUBE: couldn't found stored token"), http.StatusUnauthorized, w)
				return
			}
		}

		if useTokenClient {
			storedToken, err = clientGenerator.Infraclientset.CubeV1alpha1().Tokens("").Get(tokenName, util.GetOptions)
			if err != nil {
				util.JsonErrorResponse(errors.New("RancherCUBE: couldn't found stored token"), http.StatusUnauthorized, w)
				return
			}
		}

		if storedToken.Token != tokenKey || storedToken.ObjectMeta.Name != tokenName {
			util.JsonErrorResponse(errors.New("RancherCUBE: stored token not match request token"), http.StatusUnauthorized, w)
			return
		}

		if util.IsExpired(*storedToken) {
			util.JsonErrorResponse(errors.New("RancherCUBE: stored token is expired"), http.StatusUnauthorized, w)
			return
		}

		r.Header.Set("Authorization", "Bearer "+storedToken.Token)
		// TODO: impersonate user header

		next(w, r)
	}
}

func NewRouter(s *Server) *mux.Router {
	schemas := NewSchema()
	router := mux.NewRouter()
	apiRouter := mux.NewRouter().PathPrefix("/v1").Subrouter().StrictSlash(true)
	unSecureRouter := mux.NewRouter().PathPrefix("/").Subrouter().StrictSlash(true)
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

	unSecureRouter.Methods("POST").Path("/login").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		s.Login(w, req)
	})

	unSecureRouter.Methods("POST").Path("/logout").HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
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
		negroni.Wrap(unSecureRouter),
	))

	return router
}
