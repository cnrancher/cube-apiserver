package api

import (
	"net/http"

	"github.com/cnrancher/cube-apiserver/backend/unsecure"
	"github.com/cnrancher/cube-apiserver/util"
)

const (
	CookieName = "RC_SESS"
)

func (s *Server) Login(w http.ResponseWriter, req *http.Request) error {
	token, responseType, err := unsecure.CreateLoginToken(s.c, JwtSignKey, req)
	if err != nil {
		util.JsonErrorResponse(err, http.StatusUnauthorized, w)
		return err
	}

	if responseType == "cookie" {
		tokenCookie := &http.Cookie{
			Name:     CookieName,
			Value:    token.ObjectMeta.Name + ":" + token.Token,
			Secure:   true,
			Path:     "/",
			HttpOnly: true,
		}
		http.SetCookie(w, tokenCookie)
	} else {
		util.JsonResponse(token, http.StatusCreated, w)
	}

	return nil
}
