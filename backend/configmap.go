package backend

import (
	"k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"github.com/cnrancher/cube-apiserver/util"
)

var (
	ConfigMapNs         = "kube-system"
	ConfigMapName       = "cube-rancher"
	Dashboard_Name      = "Dashboard_Name"
	Dashboard_Namespace = "Dashboard_Namespace"
	Dashboard_Icon      = "Dashboard_Icon"
	Dashboard_Desc      = "Dashboard_Desc"
)

func (c *ClientGenerator) ConfigMapDeploy() (*v1.ConfigMap, error) {
	configMap := &v1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ConfigMapName,
			Namespace: ConfigMapNs,
			Labels: map[string]string{
				"app": ConfigMapName,
			}},

		Data: map[string]string{
			Dashboard_Name:      "kubernetes-dashboard",
			Dashboard_Namespace: "kube-system",
			Dashboard_Icon:      "",
			Dashboard_Desc:      "Kubernetes Dashboard is a general purpose, web-based UI for Kubernetes clusters. It allows users to manage applications running in the cluster and troubleshoot them, as well as manage the cluster itself.",

			"info_langhorn":   "Longhorn is a distributed block storage system for Kubernetes powered by Rancher Labs.",
			"info_rancher_vm": "info_rancher_vm",
		},
	}

	cm, err := c.Clientset.CoreV1().ConfigMaps(ConfigMapNs).Create(configMap)
	if err != nil {
		if k8serrors.IsAlreadyExists(err) {
			return c.ConfigMapGet(ConfigMapNs, ConfigMapName)
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
