package api

import (
	"github.com/cnrancher/cube-apiserver/backend"

	"github.com/rancher/go-rancher/client"
	"k8s.io/api/core/v1"
)

type Server struct {
	// TODO: HTTP reverse proxy should be here
	c *backend.ClientGenerator
}

type Node struct {
	client.Resource

	Node *v1.Node `json:"node"`
}

type Cluster struct {
	client.Resource

	Name              string                   `json:"name"`
	Resources         *backend.ClusterResource `json:"resources"`
	ComponentStatuses *v1.ComponentStatusList  `json:"componentStatuses"`
}

type Token struct {
	Name      string `json:"name"`
	LoginName string `json:"loginName"`
	Password  string `json:"password"`
	Token     string `json:"token"`
}

func NewServer(c *backend.ClientGenerator) *Server {
	s := &Server{
		c: c,
	}
	return s
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("error", client.ServerApiError{})

	nodeSchema(schemas.AddType("node", Node{}))
	clusterSchema(schemas.AddType("cluster", Cluster{}))

	return schemas
}

func clusterSchema(cluster *client.Schema) {
	cluster.CollectionMethods = []string{"GET"}
	cluster.ResourceMethods = []string{"GET"}

	cluster.ResourceFields["resources"] = client.Field{
		Type:     "struct",
		Nullable: true,
	}
	cluster.ResourceFields["componentStatuses"] = client.Field{
		Type:     "struct",
		Nullable: true,
	}
}

func nodeSchema(node *client.Schema) {
	node.CollectionMethods = []string{"GET"}
	node.ResourceMethods = []string{"GET"}

	node.ResourceFields["node"] = client.Field{
		Type:     "struct",
		Nullable: true,
	}
}

func toNodeResource(node *v1.Node) *Node {
	name := node.GetName()
	if name == "" {
		return nil
	}
	return &Node{
		Resource: client.Resource{
			Id:      string(name),
			Type:    "node",
			Actions: map[string]string{},
		},
		Node: node,
	}
}

func toNodeCollection(nodeList *v1.NodeList) *client.GenericCollection {
	data := []interface{}{}
	for _, node := range nodeList.Items {
		data = append(data, toNodeResource(&node))
	}
	return &client.GenericCollection{Data: data, Collection: client.Collection{ResourceType: "node"}}
}

func toClusterResource(clusterResource *backend.ClusterResource, componentStatuses *v1.ComponentStatusList) *Cluster {
	return &Cluster{
		Resource: client.Resource{
			Id:      "cube",
			Type:    "cluster",
			Actions: map[string]string{},
		},
		Name:              "cube",
		Resources:         clusterResource,
		ComponentStatuses: componentStatuses,
	}
}

func toClusterCollection(clusterResource *backend.ClusterResource, componentStatuses *v1.ComponentStatusList) *client.GenericCollection {
	data := []interface{}{}
	data = append(data, toClusterResource(clusterResource, componentStatuses))
	return &client.GenericCollection{Data: data, Collection: client.Collection{ResourceType: "cluster"}}
}
