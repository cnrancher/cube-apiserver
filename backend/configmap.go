package backend

import (
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/cnrancher/cube-apiserver/util"
	"github.com/cnrancher/cube-apiserver/controller"
)

var (
	ConfigMapName = "cube-rancher"

	dashboardName      = "dashboardName"
	dashboardNamespace = "dashboardNamespace"
	dashboardIcon      = "dashboardIcon"
	dashboardDesc      = "dashboardDesc"

	langhornName      = "langhornName"
	langhornNamespace = "langhornNamespace"
	langhornIcon      = "langhornIcon"
	langhornDesc      = "langhornDesc"

	rancherVMName      = "rancherVMName"
	rancherVMNamespace = "rancherVMNamespace"
	rancherVMIcon      = "rancherVMIcon"
	rancherVMDesc      = "rancherVMDesc"
)

func (c *ClientGenerator) ConfigMapDeploy() (*v1.ConfigMap, error) {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: controller.DashboardNamespace,
			Labels: map[string]string{
				"app": ConfigMapName,
			}},

		Data: map[string]string{
			dashboardName:      controller.DashboardName,
			dashboardNamespace: controller.DashboardNamespace,
			dashboardIcon:      "https://avatars3.githubusercontent.com/u/13629408?s=200&v=4",
			dashboardDesc:      controller.DashboardDesc,

			langhornName:      controller.LonghornName,
			langhornNamespace: controller.LonghornNamespace,
			langhornIcon:      "",
			langhornDesc:      controller.LanghornDesc,

			rancherVMName:      controller.RancherVMName,
			rancherVMNamespace: controller.RancherVMNamespace,
			rancherVMIcon:      "",
			rancherVMDesc:      controller.RancherVMDesc,
		},
	}

	cm, err := c.Clientset.CoreV1().ConfigMaps(controller.DashboardNamespace).Create(configMap)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return c.ConfigMapGet(controller.DashboardNamespace, ConfigMapName)
		}
		return nil, err
	}

	return cm, err
}

func (c *ClientGenerator) ConfigMapGet(ns, id string) (*v1.ConfigMap, error) {
	return c.Clientset.CoreV1().ConfigMaps(ns).Get(id, util.GetOptions)
}

func (c *ClientGenerator) ConfigMapList() (*v1.ConfigMapList, error) {
	return c.Clientset.CoreV1().ConfigMaps("").List(util.ListEverything)
}
