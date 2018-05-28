package backend

import (
	"k8s.io/api/extensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	IngressNs   = "kube-system"
	IngressName = "cube-rancher-ingress"
	host        = "test.cube.com"
	port        = 9090
)

func (c *ClientGenerator) IngressDeploy() error {
	host := v1beta1.IngressRule{
		Host: host,
	}

	ingress := &v1beta1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      IngressName,
			Namespace: IngressNs,
		},

		Spec: v1beta1.IngressSpec{
			Rules: []v1beta1.IngressRule{
				host,
			},
			Backend: &v1beta1.IngressBackend{
				ServiceName: "kubernetes-dashboard",
				ServicePort: intstr.FromInt(port),
			},
		},
	}
	_, err := c.Clientset.ExtensionsV1beta1().Ingresses(IngressNs).Create(ingress)
	if err != nil && k8serrors.IsAlreadyExists(err) {
		return nil
	}
	return err
}

func (c *ClientGenerator) IngressGet(ns, id string) (*v1beta1.Ingress, error) {
	return c.Clientset.ExtensionsV1beta1().Ingresses(ns).Get(id, metav1.GetOptions{})
}
