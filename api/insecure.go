package api

import (
	"net/http"
	"time"

	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/backend/insecure"
	"github.com/cnrancher/cube-apiserver/backend/provider/common"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
)

func (s *Server) Login(w http.ResponseWriter, req *http.Request) error {
	token, responseType, err := insecure.CreateLoginToken(s.c, JwtSignKey, req)
	if err != nil {
		util.JSONErrorResponse(err, http.StatusUnauthorized, w)
		return err
	}

	if responseType == "cookie" {
		isSecure := false
		if req.URL.Scheme == "https" {
			isSecure = true
		}
		tokenCookie := &http.Cookie{
			Name:     util.CookieName,
			Value:    token.ObjectMeta.Name + ":" + token.Token,
			Secure:   isSecure,
			Path:     "/",
			HttpOnly: true,
		}

		http.SetCookie(w, tokenCookie)
	} else {
		token.Token = token.ObjectMeta.Name + ":" + token.Token
		util.JSONResponse(token, http.StatusCreated, w)
	}

	return nil
}

func (s *Server) Logout(w http.ResponseWriter, req *http.Request) error {
	tokenAuthValue := util.GetTokenAuthFromRequest(req)
	if tokenAuthValue == "" {
		// no cookie or auth header, cannot authenticate
		err := errors.Errorf("RancherCUBE: no valid token cookie or auth header")
		util.JSONErrorResponse(err, http.StatusUnauthorized, w)
		return err
	}

	isSecure := false
	if req.URL.Scheme == "https" {
		isSecure = true
	}

	tokenCookie := &http.Cookie{
		Name:     util.CookieName,
		Value:    "",
		Secure:   isSecure,
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
		Expires:  time.Date(1982, time.February, 10, 23, 0, 0, 0, time.UTC),
	}

	http.SetCookie(w, tokenCookie)

	// ignore empty kubeConfig location, this is singleton instance
	clientGenerator := backend.NewClientGenerator("")

	storedToken, statusCode, err := clientGenerator.GetStoredToken(tokenAuthValue)
	if err != nil {
		if statusCode == 404 {
			return nil
		}
		logrus.Errorf("RancherCUBE: %v", err)
		return err
	}

	err = common.DeleteToken(*storedToken, storedToken.AuthProvider)
	if err != nil {
		logrus.Errorf("RancherCUBE: delete token failed %v", err)
		return err
	}

	return nil
}
