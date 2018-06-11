package backend

import (
	//"k8s.io/api/extensions/v1beta1"
	//metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//k8serrors "k8s.io/apimachinery/pkg/api/errors"
	//"k8s.io/apimachinery/pkg/util/intstr"
	//"github.com/cnrancher/cube-apiserver/controller"
	"fmt"
	"k8s.io/client-go/tools/cache"
	"k8s.io/api/core/v1"
	"github.com/pkg/errors"
	"github.com/Sirupsen/logrus"
	"time"
)

var (
	DbIngressName = "cube-rancher-ingress-db"
	LhIngressName = "cube-rancher-ingress-lh"
	dbHost        = "dashboard.cube.rancher.io"
	lhHost        = "longhorn.cube.rancher.io"
	Service       *v1.Service
)

func (c *ClientGenerator) ServiceGet(ns, id string) error {
	logrus.Infof("-----ns====%s====name====%s", ns, id)
	if ok := cache.WaitForCacheSync(make(chan struct{}), c.serviceSynced); !ok {
		logrus.Info("11111111=================")
		return fmt.Errorf("failed to wait for caches to sync service")
	}
	logrus.Info("222222222=================")
	doneCh := make(chan string)
	logrus.Info("33333=================")
	go c.syncService(ns, id, doneCh)
	logrus.Info("44444=================")
	<-doneCh
	logrus.Info("555555=================")
	return nil
}

func (c *ClientGenerator) syncService(ns, id string, doneCh chan string) {
	logrus.Info("666666=================")
	for true {
		svc, err := c.serviceLister.Services(ns).Get(id)
		logrus.Info("777777=================")
		if err == nil && svc != nil {
			logrus.Info("88888888=================")
			Service = svc
			break
		}
		time.Sleep(20)
	}
	logrus.Info("999999=================")
	doneCh <- "done"
}

func (c *ClientGenerator) NodeIPGet() (string, error) {
	nodes, err := c.ClusterNodes()
	if err != nil || nodes == nil {
		return "", errors.Wrap(err, "RancherCUBE: fail to read nodes")
	}

	var ip string
	for _, address := range nodes.Items[0].Status.Addresses {
		if address.Type == "ExternalIP" {
			return address.Address, nil
		}
		if address.Type == "InternalIP" {
			ip = address.Address
		}
	}

	//from annotation
	//if nodes.Items[0].Annotations == nil {
	//	nodes.Items[0].Annotations = make(map[string]string)
	//}
	//
	//if ip, ok := nodes.Items[0].Annotations[rkeExternalAddressAnnotation]; ok {
	//	return ip, nil
	//}
	//
	//if ip, ok := nodes.Items[0].Annotations[rkeInternalAddressAnnotation]; ok {
	//	return ip, nil
	//}

	return ip, nil
}

//func (c *ClientGenerator) DashboardIngressDeploy() error {
//	dbBackend := v1beta1.IngressBackend{
//		ServiceName: "kubernetes-dashboard",
//		ServicePort: intstr.FromInt(9090),
//	}
//
//	dbIngress := &v1beta1.Ingress{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      DbIngressName,
//			Namespace: KubeSystemNamespace,
//		},
//
//		Spec: v1beta1.IngressSpec{
//			Rules: []v1beta1.IngressRule{
//				{
//					Host: dbHost,
//					IngressRuleValue: v1beta1.IngressRuleValue{
//						HTTP: &v1beta1.HTTPIngressRuleValue{
//							Paths: []v1beta1.HTTPIngressPath{
//								{
//									Path:    "/",
//									Backend: dbBackend,
//								},
//							},
//						},
//					},
//				},
//			},
//		},
//	}
//
//	_, err := c.Clientset.ExtensionsV1beta1().Ingresses(KubeSystemNamespace).Create(dbIngress)
//	if err != nil && k8serrors.IsAlreadyExists(err) {
//		return nil
//	}
//
//	return err
//}
//
//func (c *ClientGenerator) LonghornIngressDeploy() error {
//	lhBackend := v1beta1.IngressBackend{
//		ServiceName: "longhorn-frontend",
//		ServicePort: intstr.FromInt(9091),
//	}
//
//	lhIngress := &v1beta1.Ingress{
//		ObjectMeta: metav1.ObjectMeta{
//			Name:      LhIngressName,
//			Namespace: controller.LonghornNamespace,
//		},
//
//		Spec: v1beta1.IngressSpec{
//			Rules: []v1beta1.IngressRule{
//				{
//					Host: lhHost,
//					IngressRuleValue: v1beta1.IngressRuleValue{
//						HTTP: &v1beta1.HTTPIngressRuleValue{
//							Paths: []v1beta1.HTTPIngressPath{
//								{
//									Path:    "/",
//									Backend: lhBackend,
//								},
//							},
//						},
//					},
//				},
//			},
//		},
//	}
//
//	_, err := c.Clientset.ExtensionsV1beta1().Ingresses(controller.LonghornNamespace).Create(lhIngress)
//	if err != nil && k8serrors.IsAlreadyExists(err) {
//		return nil
//	}
//
//	return err
//}
//
//func (c *ClientGenerator) IngressGet(ns, id string) (*v1beta1.Ingress, error) {
//	return c.Clientset.ExtensionsV1beta1().Ingresses(ns).Get(id, metav1.GetOptions{})
//}
