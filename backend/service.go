package backend

import (
	"fmt"
	"time"

	"github.com/pkg/errors"
	"k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

var (
	Service *v1.Service
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
	var d time.Duration
	for true {
		svc, err := c.serviceLister.Services(ns).Get(id)
		if err == nil && svc != nil {
			Service = svc
			break
		}
		time.Sleep(20)
		d = d + 20
		if d > 2*time.Minute {
			break
		}
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
	return ip, nil
}
