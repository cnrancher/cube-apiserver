package backend

import (
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/cnrancher/cube-apiserver/controller"
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

//func loop(watcher watch.Interface, db *v1alpha1.Infrastructure) {
//Loop:
//	for {
//		select {
//		case data := <-watcher.ResultChan():
//			object := data.Object.(*v1alpha1.Infrastructure)
//			if data.Type == "MODIFIED" && object.Name == db.Name {
//				break Loop
//			}
//		case <-time.After(time.Duration(10) * time.Second):
//			break Loop
//		}
//	}
//	return
//}

func ensureNamespaceExists(c *ClientGenerator, namespace string) error {
	_, err := c.Clientset.CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
	return err
}
