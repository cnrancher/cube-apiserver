package util

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"github.com/Sirupsen/logrus"
	"github.com/dgrijalva/jwt-go"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

const (
	CookieName      = "RC_SESS"
	AuthHeaderName  = "Authorization"
	AuthValuePrefix = "Bearer"
	BasicAuthPrefix = "Basic"
	RsaDirectory    = "/var/lib/rancher/cube"
	RsaBitSize      = 4096
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

func JSONErrorResponse(err error, statusCode int, w http.ResponseWriter) {
	response := map[string]string{
		"msg": err.Error(),
	}
	json, err := json.Marshal(response)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(statusCode)
	w.Header().Set("Content-Type", "application/json")
	w.Write(json)
}

func JSONResponse(response interface{}, statusCode int, w http.ResponseWriter) {
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

func SplitTokenParts(tokenID string) (string, string) {
	parts := strings.Split(tokenID, ":")
	if len(parts) != 2 {
		return parts[0], ""
	}
	return parts[0], parts[1]
}

func GetTokenAuthFromRequest(req *http.Request) string {
	var tokenAuthValue string
	authHeader := req.Header.Get(AuthHeaderName)
	authHeader = strings.TrimSpace(authHeader)

	if authHeader != "" {
		parts := strings.SplitN(authHeader, " ", 2)
		if strings.EqualFold(parts[0], AuthValuePrefix) {
			if len(parts) > 1 {
				tokenAuthValue = strings.TrimSpace(parts[1])
			}
		} else if strings.EqualFold(parts[0], BasicAuthPrefix) {
			if len(parts) > 1 {
				base64Value := strings.TrimSpace(parts[1])
				data, err := base64.URLEncoding.DecodeString(base64Value)
				if err != nil {
					logrus.Errorf("RancherCUBE: error %v parsing %v header", err, AuthHeaderName)
				} else {
					tokenAuthValue = string(data)
				}
			}
		}
	} else {
		cookie, err := req.Cookie(CookieName)
		if err != nil {
			logrus.Errorf("RancherCUBE: error %v get cookie %s", err, CookieName)
		} else {
			tokenAuthValue = cookie.Value
		}
	}
	return tokenAuthValue
}

func Int32ToString(n int32) string {
	buf := [11]byte{}
	pos := len(buf)
	i := int64(n)
	signed := i < 0
	if signed {
		i = -i
	}
	for {
		pos--
		buf[pos], i = '0'+byte(i%10), i/10
		if i == 0 {
			if signed {
				pos--
				buf[pos] = '-'
			}
			return string(buf[pos:])
		}
	}
}

func GenerateRSA256() error {
	// make sure the rsa directory is exist
	if _, err := os.Stat(RsaDirectory); err != nil {
		err = os.MkdirAll(RsaDirectory, os.ModeDir|0700)
		if err != nil {
			return err
		}
	}

	// if no rsa private/public key file re-generate it
	if !CheckRSAKeyFileExist() {
		privateKey, err := GeneratePrivateKey()
		if err != nil {
			logrus.Errorf("generate private key error: %v", err)
			return err
		}

		publicKeyBytes, err := GeneratePublicKey(privateKey)
		if err != nil {
			logrus.Errorf("generate public key bytes error: %v", err)
			return err
		}

		privateKeyBytes := PrivateKeyToPEM(privateKey)

		err = WriteKeyToFile(privateKeyBytes, RsaDirectory+"/id_rsa")
		if err != nil {
			logrus.Errorf("write private key file error: %v", err)
			return err
		}

		err = WriteKeyToFile([]byte(publicKeyBytes), RsaDirectory+"/id_rsa.pub")
		if err != nil {
			logrus.Errorf("write public key file error: %v", err)
			return err
		}
	}

	return nil
}

func GeneratePrivateKey() (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, RsaBitSize)
	if err != nil {
		return nil, err
	}

	err = privateKey.Validate()
	if err != nil {
		return nil, err
	}

	return privateKey, nil
}

func PrivateKeyToPEM(privateKey *rsa.PrivateKey) []byte {
	privateDER := x509.MarshalPKCS1PrivateKey(privateKey)

	privateBlock := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privateDER,
	}

	privatePEM := pem.EncodeToMemory(&privateBlock)

	return privatePEM
}

func GeneratePublicKey(privateKey *rsa.PrivateKey) ([]byte, error) {
	publicKey := privateKey.PublicKey
	publicDER, err := x509.MarshalPKIXPublicKey(&publicKey)
	if err != nil {
		return nil, err
	}

	publicBlock := pem.Block{
		Type:    "RSA PUBLIC KEY",
		Headers: nil,
		Bytes:   publicDER,
	}

	publicPEM := pem.EncodeToMemory(&publicBlock)

	return publicPEM, nil
}

func CheckRSAKeyFileExist() bool {
	if _, err := os.Stat(RsaDirectory + "/id_rsa"); err == nil {
		if _, err = os.Stat(RsaDirectory + "/id_rsa.pub"); err == nil {
			return true
		}

		err = os.Remove(RsaDirectory + "/id_rsa.pub")
		if err != nil {
			logrus.Errorf("remove id_rsa.pub file error: %v", err)
		}

		err = os.Remove(RsaDirectory + "/id_rsa")
		if err != nil {
			logrus.Errorf("remove id_rsa file error: %v", err)
		}
	}

	return false
}

func WriteKeyToFile(keyBytes []byte, saveFileTo string) error {
	err := ioutil.WriteFile(saveFileTo, keyBytes, 0600)
	if err != nil {
		return err
	}

	return nil
}

func CopyDir(src string, dest string) {
	srcOriginal := src
	err := filepath.Walk(src, func(src string, f os.FileInfo, err error) error {
		if f == nil {
			return err
		}
		if f.IsDir() {
			//CopyDir(f.Name(), dest+"/"+f.Name())
		} else {
			destNew := strings.Replace(src, srcOriginal, dest, -1)
			CopyFile(src, destNew)
		}
		return nil
	})
	if err != nil {
		logrus.Errorf("file path walk error: %v", err)
	}
}

func CopyFile(src, dst string) (w int64, err error) {
	srcFile, err := os.Open(src)
	if err != nil {
		logrus.Errorf("open file error: %v", err)
		return
	}
	defer srcFile.Close()
	dstSlices := strings.Split(dst, "/")
	dstSlicesLen := len(dstSlices)
	destDir := ""
	for i := 0; i < dstSlicesLen-1; i++ {
		destDir = destDir + dstSlices[i] + "/"
	}
	b, err := PathExists(destDir)
	if b == false {
		err := os.Mkdir(destDir, os.ModePerm)
		if err != nil {
			logrus.Errorf("mkdir error: %v", err)
		}
	}
	dstFile, err := os.Create(dst)
	if err != nil {
		logrus.Errorf("create file error: %v", err)
		return
	}

	defer dstFile.Close()

	return io.Copy(dstFile, srcFile)
}

func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
