package local

import (
	"fmt"
	"strings"

	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/backend/provider"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	v1alpha1lister "github.com/cnrancher/cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

const (
	ProviderName = "local"
	ConfigType   = "localConfig"
)

type Provider struct {
	userLister      v1alpha1lister.UserLister
	userInformer    cache.SharedIndexInformer
	tokenLister     v1alpha1lister.TokenLister
	tokenInformer   cache.SharedIndexInformer
	clientGenerator *backend.ClientGenerator
}

func Configure(clientGenerator *backend.ClientGenerator) provider.AuthProvider {
	informer := clientGenerator.CubeInformerFactory.Cube().V1alpha1().Users().Informer()
	indexers := map[string]cache.IndexFunc{controller.UserByUsernameIndex: controller.UserByUsername, controller.UserSearchIndex: controller.UserSearchIndexer}
	informer.AddIndexers(indexers)

	tokenInformer := clientGenerator.CubeInformerFactory.Cube().V1alpha1().Tokens().Informer()
	tokenIndexers := map[string]cache.IndexFunc{controller.TokenByNameIndex: controller.TokenByName}
	tokenInformer.AddIndexers(tokenIndexers)

	return &Provider{
		userLister:      clientGenerator.CubeInformerFactory.Cube().V1alpha1().Users().Lister(),
		userInformer:    informer,
		tokenLister:     clientGenerator.CubeInformerFactory.Cube().V1alpha1().Tokens().Lister(),
		tokenInformer:   tokenInformer,
		clientGenerator: clientGenerator,
	}
}

func (p *Provider) GetName() string {
	return ProviderName
}

func (p *Provider) SearchPrincipals(searchKey, principalType string, token v1alpha1.Token) ([]v1alpha1.Principal, error) {
	return p.SearchPrincipalsDedupe(searchKey, principalType, token, nil)
}

func (p *Provider) SearchPrincipalsDedupe(searchKey, principalType string, token v1alpha1.Token, principalsFromOtherProviders []v1alpha1.Principal) ([]v1alpha1.Principal, error) {
	fromOtherProviders := map[string]bool{}
	for _, p := range principalsFromOtherProviders {
		fromOtherProviders[p.Name] = true
	}
	var principals []v1alpha1.Principal
	var localUsers []*v1alpha1.User
	var err error

	if len(searchKey) > controller.SearchIndexDefaultLen {
		localUsers, err = p.listAllUsers(searchKey)
	} else {
		localUsers, err = p.listUsersByIndex(searchKey)
	}

	if err != nil {
		logrus.Infof("RancherCUBE: failed to search User resources for %v: %v", searchKey, err)
		return principals, err
	}

	if principalType == "" || principalType == "user" {
	User:
		for _, user := range localUsers {
			for _, p := range user.PrincipalIDs {
				if fromOtherProviders[p] {
					continue User
				}
			}
			principalID := controller.GetLocalPrincipalID(user)
			userPrincipal := controller.ToPrincipal("user", user.DisplayName, user.Username, user.Namespace, principalID, &token)
			principals = append(principals, userPrincipal)
		}
	}

	return principals, nil
}

func (p *Provider) GetPrincipal(principalID string, token v1alpha1.Token) (v1alpha1.Principal, error) {
	var name string
	user, err := p.userLister.Users("").Get(name)
	if err != nil {
		return v1alpha1.Principal{}, err
	}

	princID := controller.GetLocalPrincipalID(user)
	princ := controller.ToPrincipal("user", user.DisplayName, user.Username, user.Namespace, princID, &token)
	return princ, nil
}

func (p *Provider) AuthenticateUser(input interface{}, providerName string) (v1alpha1.Principal, map[string]string, error) {
	localInput, ok := input.(*provider.BasicLogin)
	if !ok {
		return v1alpha1.Principal{}, nil, errors.New("RancherCUBE: unexpected input type")
	}

	username := localInput.Username
	pwd := localInput.Password

	objs, err := p.userInformer.GetIndexer().ByIndex(controller.UserByUsernameIndex, username)
	if err != nil {
		return v1alpha1.Principal{}, nil, err
	}
	if len(objs) == 0 {
		return v1alpha1.Principal{}, nil, errors.Wrap(err, "RancherCUBE: authentication failed")
	}
	if len(objs) > 1 {
		return v1alpha1.Principal{}, nil, fmt.Errorf("RancherCUBE: found more than one users with username %v", username)
	}
	user, ok := objs[0].(*v1alpha1.User)
	if !ok {
		return v1alpha1.Principal{}, nil, fmt.Errorf("RancherCUBE: fatal error. %v is not a user", objs[0])
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(pwd)); err != nil {
		return v1alpha1.Principal{}, nil, errors.Wrap(err, "RancherCUBE: authentication failed")
	}

	principalID := getLocalPrincipalID(user)
	userPrincipal := controller.ToPrincipal("user", user.DisplayName, user.Username, user.Namespace, principalID, nil)
	userPrincipal.Me = true

	allowed, err := p.CheckAccess(userPrincipal)

	if !allowed {
		return v1alpha1.Principal{}, nil, errors.New("RancherCUBE: unauthorized")
	}

	return userPrincipal, map[string]string{}, nil
}

func (p *Provider) CheckAccess(userPrincipal v1alpha1.Principal) (bool, error) {
	user, err := p.clientGenerator.CheckUserCache(userPrincipal.Name)
	if err != nil {
		return false, err
	}

	principal, err := p.clientGenerator.CheckPrincipalCache(userPrincipal.Name)
	if err != nil {
		return false, err
	}
	if user.Username == principal.LoginName {
		return true, nil
	}

	return false, errors.Errorf("RancherCUBE: no allowed principalIDs")
}

func (p *Provider) GenerateToken(namespace string, token v1alpha1.Token, providerName string) (*v1alpha1.Token, error) {
	createdToken, err := p.clientGenerator.Infraclientset.CubeV1alpha1().Tokens(token.UserPrincipal.Namespace).Create(&token)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return &v1alpha1.Token{}, err
	}

	return createdToken, nil
}

func (p *Provider) listAllUsers(searchKey string) ([]*v1alpha1.User, error) {
	var localUsers []*v1alpha1.User

	allUsers, err := p.userLister.Users("").List(labels.NewSelector())
	if err != nil {
		logrus.Infof("RancherCUBE: failed to search User resources for %v: %v", searchKey, err)
		return localUsers, err
	}
	for _, user := range allUsers {
		if !(strings.HasPrefix(user.ObjectMeta.Name, searchKey) || strings.HasPrefix(user.Username, searchKey) || strings.HasPrefix(user.DisplayName, searchKey)) {
			continue
		}
		localUsers = append(localUsers, user)
	}

	return localUsers, err
}

func (p *Provider) listUsersByIndex(searchKey string) ([]*v1alpha1.User, error) {
	var localUsers []*v1alpha1.User
	var err error

	objs, err := p.userInformer.GetIndexer().ByIndex(controller.UserSearchIndex, searchKey)
	if err != nil {
		logrus.Infof("RancherCUBE: failed to search User resources for %v: %v", searchKey, err)
		return localUsers, err
	}

	for _, obj := range objs {
		user, ok := obj.(*v1alpha1.User)
		if !ok {
			logrus.Errorf("RancherCUBE: user isn't a user %v", obj)
			return localUsers, err
		}
		localUsers = append(localUsers, user)
	}

	return localUsers, err
}

func getLocalPrincipalID(user *v1alpha1.User) string {
	var principalID string
	for _, p := range user.PrincipalIDs {
		if strings.HasPrefix(p, ProviderName+".") {
			principalID = p
		}
	}
	return principalID
}
