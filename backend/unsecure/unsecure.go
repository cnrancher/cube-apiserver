package unsecure

import (
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/backend/provider"
	"github.com/cnrancher/cube-apiserver/backend/provider/common"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateLoginToken(clientGenerator *backend.ClientGenerator, signKey *rsa.PrivateKey, req *http.Request) (v1alpha1.Token, string, error) {
	logrus.Debugf("RancherCUBE: create login token invoked")

	bytes, err := ioutil.ReadAll(req.Body)
	if err != nil {
		logrus.Errorf("RancherCUBE: login failed with error: %v", err)
		return v1alpha1.Token{}, "cookie", errors.Wrap(err, "RancherCUBE: server error while authenticating")
	}

	generic := &provider.GenericLogin{}
	err = json.Unmarshal(bytes, generic)
	if err != nil {
		logrus.Errorf("RancherCUBE: unmarshal failed with error: %v", err)
		return v1alpha1.Token{}, "", errors.Wrap(err, "RancherCUBE: server error while authenticating")
	}
	responseType := generic.ResponseType
	description := generic.Description
	ttl := generic.TTLMillis
	providerType := generic.ProviderType

	var input interface{}
	switch providerType {
	case "local":
		input = &provider.BasicLogin{}
	default:
		return v1alpha1.Token{}, responseType, errors.Wrap(err, "RancherCUBE: unknown authentication provider")
	}

	err = json.Unmarshal(bytes, input)
	if err != nil {
		logrus.Errorf("RancherCUBE: unmarshal failed with error: %v", err)
		return v1alpha1.Token{}, responseType, errors.Wrap(err, "RancherCUBE: server error while authenticating")
	}

	userPrincipal, providerInfo, err := common.AuthenticateUser(input, providerType)
	if err != nil {
		return v1alpha1.Token{}, responseType, err
	}

	logrus.Debug("RancherCUBE: user authenticated")
	displayName := userPrincipal.DisplayName
	if displayName == "" {
		displayName = userPrincipal.LoginName
	}
	user, err := clientGenerator.EnsureUser(userPrincipal.Name, displayName)
	if err != nil {
		return v1alpha1.Token{}, responseType, err
	}

	rToken, err := NewLoginToken(clientGenerator, user.Name, userPrincipal, providerInfo, ttl, description, signKey)
	return rToken, responseType, err
}

func NewLoginToken(clientGenerator *backend.ClientGenerator, userID string, userPrincipal v1alpha1.Principal, providerInfo map[string]string, ttl int64, description string, signKey *rsa.PrivateKey) (v1alpha1.Token, error) {
	token := &v1alpha1.Token{
		TypeMeta: metaV1.TypeMeta{
			APIVersion: "cube.rancher.io/v1alpha1",
			Kind:       "Token",
		},
		UserPrincipal: userPrincipal,
		IsDerived:     true,
		TTLMillis:     ttl,
		ExpiresAt:     "",
		UserID:        userID,
		AuthProvider:  util.GetAuthProviderName(userPrincipal.Name),
		ProviderInfo:  providerInfo,
		Description:   description,
	}

	return CreateOrUpdateToken(clientGenerator, *token, signKey)
}

func CreateOrUpdateToken(clientGenerator *backend.ClientGenerator, token v1alpha1.Token, signKey *rsa.PrivateKey) (v1alpha1.Token, error) {
	key, err := util.GenerateToken(token.TTLMillis, signKey)
	if err != nil {
		logrus.Errorf("RancherCUBE: failed to generate token key: %v", err)
		return v1alpha1.Token{}, fmt.Errorf("RancherCUBE: failed to generate token key")
	}

	labels := make(map[string]string)
	labels[controller.UserIDLabel] = token.UserID
	token.Token = key
	token.ObjectMeta = metaV1.ObjectMeta{
		GenerateName: "token-",
		Labels:       labels,
	}

	generateToken, err := common.GenerateToken(token.UserPrincipal.Namespace, token, token.AuthProvider)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return v1alpha1.Token{}, err
	}

	generateToken = util.SetTokenExpiresAt(generateToken)
	generateToken, err = clientGenerator.Infraclientset.CubeV1alpha1().Tokens(token.UserPrincipal.Namespace).Update(generateToken)
	if err != nil {
		return v1alpha1.Token{}, err
	}

	return *generateToken, nil
}
