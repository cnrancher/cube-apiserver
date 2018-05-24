package backend

import (
	"reflect"

	"github.com/rancher/rancher-cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	Plural   = "infrastructures"
	Group    = "cube.rancher.io"
	Version  = "v1alpha1"
	FullName = Plural + "." + Group
)

func (c *ClientGenerator) InfrastructureCRDDeploy() error {
	crd := &v1beta1.CustomResourceDefinition{
		ObjectMeta: metaV1.ObjectMeta{Name: FullName},
		Spec: v1beta1.CustomResourceDefinitionSpec{
			Group:   Group,
			Version: Version,
			Scope:   v1beta1.NamespaceScoped,
			Names: v1beta1.CustomResourceDefinitionNames{
				Plural: Plural,
				Kind:   reflect.TypeOf(v1alpha1.Infrastructure{}).Name(),
			},
		},
	}

	_, err := c.Apiclientset.ApiextensionsV1beta1().CustomResourceDefinitions().Create(crd)
	if err != nil && apierrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}
