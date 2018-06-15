package backend

import (
	"github.com/cnrancher/cube-apiserver/controller"

	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	configMap *v1.ConfigMap
)

func getConfigMapInfo(c *ClientGenerator) (*v1.ConfigMap, error) {
	if configMap == nil {
		var err error
		configMap, err = c.ConfigMapGet(controller.DashboardNamespace, ConfigMapName)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				return c.ConfigMapDeploy()
			}
			return nil, err
		}
	}
	return configMap, nil

}

func ensureNamespaceExists(c *ClientGenerator, namespace string) error {
	_, err := c.Clientset.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
	return err
}
