package backend

import (
	"github.com/rancher/rancher-cube-apiserver/util"

	"k8s.io/api/core/v1"
)

func (c *ClientGenerator) ClusterNodes() (*v1.NodeList, error) {
	nodeList, err := c.Clientset.CoreV1().Nodes().List(util.ListEverything)
	return nodeList, err
}

func (c *ClientGenerator) ClusterNode(id string) (*v1.Node, error) {
	node, err := c.Clientset.CoreV1().Nodes().Get(id, util.GetOptions)
	return node, err
}
