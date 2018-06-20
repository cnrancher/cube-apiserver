package backend

import (
	"crypto/rsa"
	"io/ioutil"

	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	PrivateKeyPath = util.RsaDirectory + "/id_rsa"
	PublicKeyPath  = util.RsaDirectory + "/id_rsa.pub"
	SecretName     = "cube-rancher-token"
	PrivateKey     = "privateKey"
	PublicKey      = "publicKey"
)

func (c *ClientGenerator) SecretGet(ns, id string) (*v1.Secret, error) {
	return c.Clientset.CoreV1().Secrets(ns).Get(id, util.GetOptions)
}

func (c *ClientGenerator) SecretList() (*v1.SecretList, error) {
	return c.Clientset.CoreV1().Secrets("").List(util.ListEverything)
}

func (c *ClientGenerator) GetRSAKey() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	secret, err := c.SecretGet(controller.UserNamespace, SecretName)
	if err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, nil, err
		}
		//generate RSA key
		err := util.GenerateRSA256()
		if err != nil {
			return nil, nil, err
		}
		//read RSA key
		privateKey, publicKey, err := readRSAKey()
		if err != nil {
			return nil, nil, err
		}
		//save RSA key
		secret, err = c.Clientset.CoreV1().Secrets(controller.UserNamespace).Create(&v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      SecretName,
				Namespace: controller.UserNamespace,
			},
			Type: v1.SecretTypeOpaque,
			Data: map[string][]byte{
				PrivateKey: privateKey,
				PublicKey:  publicKey,
			},
		})
		if err != nil {
			return nil, nil, err
		}

	}

	return ParseRSAKey(secret)
}
func readRSAKey() (privateBytes, publicBytes []byte, err error) {
	signBytes, err := ioutil.ReadFile(PrivateKeyPath)
	if err != nil {
		return nil, nil, err
	}

	verifyBytes, err := ioutil.ReadFile(PublicKeyPath)
	if err != nil {
		return nil, nil, err
	}

	return signBytes, verifyBytes, nil
}

func ParseRSAKey(s *v1.Secret) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	var jwtSignKey *rsa.PrivateKey
	var jwtVerifyKey *rsa.PublicKey
	var err error
	if s != nil && s.DeletionTimestamp == nil {
		signBytes := s.Data[PrivateKey]
		verifyBytes := s.Data[PublicKey]

		jwtSignKey, err = jwt.ParseRSAPrivateKeyFromPEM(signBytes)
		if err != nil {
			logrus.Fatal(err)
		}

		jwtVerifyKey, err = jwt.ParseRSAPublicKeyFromPEM(verifyBytes)
		if err != nil {
			logrus.Fatal(err)
		}
	}
	return jwtSignKey, jwtVerifyKey, nil
}
