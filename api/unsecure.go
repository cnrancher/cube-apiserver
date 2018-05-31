package api

import (
	"net/http"
	"time"

	"github.com/cnrancher/cube-apiserver/util"

	"github.com/dgrijalva/jwt-go"
)

func (s *Server) Login(w http.ResponseWriter, req *http.Request) error {
	s.c.Login()

	token := jwt.New(jwt.SigningMethodRS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Second * time.Duration(20)).Unix()
	claims["iat"] = time.Now().Unix()
	token.Claims = claims

	tokenStr, err := token.SignedString(JwtSignKey)
	if err != nil {
		return err
	}

	response := &Token{
		Name:      "zhaozy",
		LoginName: "zhaozy",
		Token:     tokenStr,
	}

	util.JsonResponse(response, w)
	return nil
}
