package api

import (
	"net/http"

	"github.com/cnrancher/cube-apiserver/backend/unsecure"
	"github.com/cnrancher/cube-apiserver/util"
)

func (s *Server) Login(w http.ResponseWriter, req *http.Request) error {
	token, responseType, err := unsecure.CreateLoginToken(s.c, JwtSignKey, req)
	if err != nil {
		util.JsonErrorResponse(err, http.StatusUnauthorized, w)
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
		w.WriteHeader(http.StatusCreated)
		http.SetCookie(w, tokenCookie)
	} else {
		token.Token = token.ObjectMeta.Name + ":" + token.Token
		util.JsonResponse(token, http.StatusCreated, w)
	}

	return nil
}

func (s *Server) Logout(w http.ResponseWriter, req *http.Request) error {
	return nil
}
