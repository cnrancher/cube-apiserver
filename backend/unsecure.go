package backend

import (
	"reflect"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	UserPlural   = "users"
	UserGroup    = "cube.rancher.io"
	UserVersion  = "v1alpha1"
	UserFullName = UserPlural + "." + UserGroup
)

func (c *ClientGenerator) UserCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: UserFullName},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   UserGroup,
			Version: UserVersion,
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: UserPlural,
				Kind:   reflect.TypeOf(v1alpha1.User{}).Name(),
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func (c *ClientGenerator) Login() error {

	return nil
}
