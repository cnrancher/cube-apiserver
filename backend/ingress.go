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
	if ok := cache.WaitForCacheSync(make(chan struct{}), c.serviceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync service")
	}
	doneCh := make(chan string)
	go c.syncService(ns, id, doneCh)
	<-doneCh
	return nil
}

func (c *ClientGenerator) syncService(ns, id string, doneCh chan string) {
	for true {
		svc, err := c.serviceLister.Services(ns).Get(id)
		if err == nil && svc != nil {
			Service = svc
			break
		}
		time.Sleep(20)
	}
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
