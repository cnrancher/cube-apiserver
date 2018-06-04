package util

import (
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

func RegisterShutdownChannel(done chan struct{}) {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		logrus.Infof("RancherCUBE: receive %v to exit", sig)
		close(done)
	}()
}

func JsonResponse(response interface{}, statusCode int, w http.ResponseWriter) {
	json, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func ContainsString(slice []string, item string) bool {
	for _, j := range slice {
		if j == item {
			return true
		}
	}
	return false
}

func GetAuthProviderName(principalID string) string {
	parts := strings.Split(principalID, ".")
	providerName := parts[0]

	return providerName
}

func GenerateToken(ttl int64, signKey *rsa.PrivateKey) (string, error) {
	token := jwt.New(jwt.SigningMethodRS256)
	claims := make(jwt.MapClaims)
	claims["exp"] = time.Now().Add(time.Millisecond * time.Duration(ttl)).Unix()
	claims["iat"] = time.Now().Unix()
	token.Claims = claims

	tokenStr, err := token.SignedString(signKey)
	if err != nil {
		return "", err
	}
	return tokenStr, nil
}

func SetTokenExpiresAt(token *v1alpha1.Token) *v1alpha1.Token {
	if token.TTLMillis != 0 {
		created := token.ObjectMeta.CreationTimestamp.Time
		ttlDuration := time.Duration(token.TTLMillis) * time.Millisecond
		expiresAtTime := created.Add(ttlDuration)
		token.ExpiresAt = expiresAtTime.UTC().Format(time.RFC3339)
	}

	return token
}

func IsExpired(token v1alpha1.Token) bool {
	if token.TTLMillis == 0 {
		return false
	}

	created := token.ObjectMeta.CreationTimestamp.Time
	durationElapsed := time.Since(created)

	ttlDuration := time.Duration(token.TTLMillis) * time.Millisecond
	return durationElapsed.Seconds() >= ttlDuration.Seconds()
}

func HashPasswordString(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", errors.Wrap(err, "RancherCUBE: problem encrypting password")
	}
	return string(hash), nil
}
