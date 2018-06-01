package backend

import (
	"reflect"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
	if err == nil || (err != nil && apierrors.IsAlreadyExists(err)) {
//		return c.InitCubeAdmin()
		return nil
	}

	return err
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

func (c *ClientGenerator) InitCubeAdmin() error {
	cubeAdmin := &v1alpha1.User{
		ObjectMeta: metaV1.ObjectMeta{
			Name:      "cube-admin",
			Namespace: "kube-system",
		},
		DisplayName:        "admin",
		Description:        "CUBE admin user, has the highest authority",
		Username:           "admin",
		MustChangePassword: true,
		PrincipalIDs:       []string{"local.kube-system.cube-admin"},
		Me:                 false,
	}

	_, err := c.Infraclientset.CubeV1alpha1().Users("kube-system").Create(cubeAdmin)
	if err == nil || (err != nil && apierrors.IsAlreadyExists(err)) {
		return err
	}
	return nil
}
