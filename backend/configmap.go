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

	DashboardName      = "DashboardName"
	DashboardNamespace = "DashboardNamespace"
	DashboardIcon      = "DashboardIcon"
	DashboardDesc      = "DashboardDesc"

	LanghornName      = "LanghornName"
	LanghornNamespace = "LanghornNamespace"
	LanghornIcon      = "LanghornIcon"
	LanghornDesc      = "LanghornDesc"
)

func (c *ClientGenerator) ConfigMapDeploy() (*v1.ConfigMap, error) {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: KubeSystemNamespace,
			Labels: map[string]string{
				"app": ConfigMapName,
			}},

		Data: map[string]string{
			DashboardName:      "kubernetes-dashboard",
			DashboardNamespace: controller.InfrastructureNamespace,
			DashboardIcon:      "",
			DashboardDesc:      controller.DashboardDesc,

			LanghornName:      "longhorn-ui",
			LanghornNamespace: controller.LonghornNamespace,
			LanghornIcon:      "",
			LanghornDesc:      controller.LanghornDesc,

			"info_rancher_vm": "info_rancher_vm",
		},
	}

	cm, err := c.Clientset.CoreV1().ConfigMaps(KubeSystemNamespace).Create(configMap)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return c.ConfigMapGet(KubeSystemNamespace, ConfigMapName)
		}
		return nil, err
	}

	return cm, err
}

func (c *ClientGenerator) ConfigMapGet(ns, id string) (*v1.ConfigMap, error) {
	return c.Clientset.CoreV1().ConfigMaps(ns).Get(id, util.GetOptions)
}

//func (c *ClientGenerator) ConfigMapList() (*v1.ConfigMapList, error) {
//	return c.Clientset.CoreV1().ConfigMaps("").List(util.ListEverything)
//}
