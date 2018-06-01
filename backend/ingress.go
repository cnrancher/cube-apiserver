package backend

import (
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
	"github.com/cnrancher/cube-apiserver/controller"
)

var (
	DbIngressName = "cube-rancher-ingress-db"
	LhIngressName = "cube-rancher-ingress-lh"
	dbHost        = "dashboard.cube.rancher.io"
	lhHost        = "longhorn.cube.rancher.io"
)

func (c *ClientGenerator) DashboardIngressDeploy() error {
	dbBackend := v1beta1.IngressBackend{
		ServiceName: "kubernetes-dashboard",
		ServicePort: intstr.FromInt(9090),
	}

	dbIngress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      DbIngressName,
			Namespace: KubeSystemNamespace,
		},

		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: dbHost,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path:    "/",
									Backend: dbBackend,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := c.Clientset.ExtensionsV1beta1().Ingresses(KubeSystemNamespace).Create(dbIngress)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func (c *ClientGenerator) LonghornIngressDeploy() error {
	lhBackend := v1beta1.IngressBackend{
		ServiceName: "longhorn-frontend",
		ServicePort: intstr.FromInt(9091),
	}

	lhIngress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      LhIngressName,
			Namespace: controller.LonghornNamespace,
		},

		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				{
					Host: lhHost,
					IngressRuleValue: v1beta1.IngressRuleValue{
						HTTP: &v1beta1.HTTPIngressRuleValue{
							Paths: []v1beta1.HTTPIngressPath{
								{
									Path:    "/",
									Backend: lhBackend,
								},
							},
						},
					},
				},
			},
		},
	}

	_, err := c.Clientset.ExtensionsV1beta1().Ingresses(controller.LonghornNamespace).Create(lhIngress)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}

	return err
}

func (c *ClientGenerator) IngressGet(ns, id string) (*v1beta1.Ingress, error) {
	return c.Clientset.ExtensionsV1beta1().Ingresses(ns).Get(id, metav1.GetOptions{})
}
