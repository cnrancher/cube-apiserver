package backend

import (
	"encoding/base32"
	"reflect"

	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
)

func (c *ClientGenerator) UserCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: "users.cube.rancher.io"},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   "cube.rancher.io",
			Version: "v1alpha1",
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: "users",
				Kind:   reflect.TypeOf(v1alpha1.User{}).Name(),
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)

	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	err = c.PrincipalCRDDeploy()
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	err = c.TokenCRDDeploy()
	if err != nil && !apierrors.IsAlreadyExists(err) {
		return err
	}

	return c.InitCubeAdmin()
}

func (c *ClientGenerator) PrincipalCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: "principals.cube.rancher.io"},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   "cube.rancher.io",
			Version: "v1alpha1",
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: "principals",
				Kind:   reflect.TypeOf(v1alpha1.Principal{}).Name(),
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err == nil || (err != nil && apierrors.IsAlreadyExists(err)) {
		return nil
	}

	return err
}

func (c *ClientGenerator) TokenCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: "tokens.cube.rancher.io"},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   "cube.rancher.io",
			Version: "v1alpha1",
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: "tokens",
				Kind:   reflect.TypeOf(v1alpha1.Token{}).Name(),
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err == nil || (err != nil && apierrors.IsAlreadyExists(err)) {
		return nil
	}

	return err
}

func (c *ClientGenerator) InitCubeAdmin() error {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("cube-admin"))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	labelSet := labels.Set(map[string]string{encodedPrincipalID: "hashed-principal-name"})

	// TODO: need remove default admin password, use setPassword instead
	hashPassword, err := util.HashPasswordString("admin")
	if err != nil {
		logrus.Fatal(err)
	}

	cubeAdmin, err := c.Infraclientset.CubeV1alpha1().Users("kube-system").Get("cube-admin", util.GetOptions)

	if err != nil {
		cubeAdmin = &v1alpha1.User{
			ObjectMeta: metaV1.ObjectMeta{
				GenerateName: "cube-",
				Name:         "cube-admin",
				Namespace:    "kube-system",
				Labels:       labelSet,
			},
			DisplayName: "admin",
			Description: "CUBE admin user, has the highest authority",
			Username:    "admin",
			// TODO: need remove default admin password, use setPassword instead
			Password:           hashPassword,
			MustChangePassword: false,
			PrincipalIDs:       []string{"local.kube-system.cube-admin"},
			Me:                 false,
		}

		if !apierrors.IsAlreadyExists(err) {
			_, err = c.Infraclientset.CubeV1alpha1().Users("kube-system").Create(cubeAdmin)
		} else {
			_, err = c.Infraclientset.CubeV1alpha1().Users("kube-system").Update(cubeAdmin)
		}

		if err == nil || (err != nil && apierrors.IsAlreadyExists(err)) {
			return err
		}
	}

	return nil
}

func (c *ClientGenerator) EnsureUser(principalName, displayName string) (*v1alpha1.User, error) {
	// First check the local cache
	u, err := c.CheckUserCache(principalName)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// Not in cache, query API by label
	u, labelSet, err := c.checkLabels(principalName)
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}

	// Doesn't exist, create user
	user := &v1alpha1.User{
		ObjectMeta: metaV1.ObjectMeta{
			GenerateName: "cube-",
			Labels:       labelSet,
		},
		DisplayName:  displayName,
		PrincipalIDs: []string{principalName},
	}

	created, err := c.Infraclientset.CubeV1alpha1().Users("kube-system").Create(user)
	if err != nil {
		return nil, err
	}

	return created, nil
}

func (c *ClientGenerator) CheckUserCache(principalName string) (*v1alpha1.User, error) {
	informer := c.CubeInformerFactory.Cube().V1alpha1().Users().Informer()
	indexers := map[string]cache.IndexFunc{controller.UserByUsernameIndex: controller.UserByUsername,
		controller.UserSearchIndex: controller.UserSearchIndexer, controller.UserByPrincipalIndex: controller.UserByPrincipal}
	informer.AddIndexers(indexers)

	users, err := informer.GetIndexer().ByIndex(controller.UserByPrincipalIndex, principalName)
	if err != nil {
		return nil, err
	}
	if len(users) > 1 {
		return nil, errors.Errorf("RancherCUBE: can't find unique user for principal %v", principalName)
	}
	if len(users) == 1 {
		u := users[0].(*v1alpha1.User)
		return u.DeepCopy(), nil
	}
	return nil, nil
}

func (c *ClientGenerator) CheckPrincipalCache(principalName string) (*v1alpha1.Principal, error) {
	informer := c.CubeInformerFactory.Cube().V1alpha1().Principals().Informer()
	indexers := map[string]cache.IndexFunc{controller.PrincipalByIdIndex: controller.PrincipalById}
	informer.AddIndexers(indexers)

	principals, err := informer.GetIndexer().ByIndex(controller.PrincipalByIdIndex, principalName)
	if err != nil {
		return nil, err
	}
	if len(principals) > 1 {
		return nil, errors.Errorf("RancherCUBE: can't find unique principal %v", principalName)
	}
	if len(principals) == 1 {
		u := principals[0].(*v1alpha1.Principal)
		return u.DeepCopy(), nil
	}
	return nil, nil
}

func (c *ClientGenerator) checkLabels(principalName string) (*v1alpha1.User, labels.Set, error) {
	encodedPrincipalID := base32.HexEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte(principalName))
	if len(encodedPrincipalID) > 63 {
		encodedPrincipalID = encodedPrincipalID[:63]
	}
	set := labels.Set(map[string]string{encodedPrincipalID: "hashed-principal-name"})
	users, err := c.Infraclientset.CubeV1alpha1().Users("").List(metaV1.ListOptions{LabelSelector: set.String()})
	if err != nil {
		return nil, nil, err
	}

	if len(users.Items) == 0 {
		return nil, set, nil
	}

	var match *v1alpha1.User
	for _, u := range users.Items {
		if util.ContainsString(u.PrincipalIDs, principalName) {
			if match != nil {
				// error out on duplicates
				return nil, nil, errors.Errorf("RancherCUBE: can't find unique user for principal %v", principalName)
			}
			match = &u
		}
	}

	return match, set, nil
}
