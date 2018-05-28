package api

import (
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"github.com/rancher/go-rancher/client"
	"k8s.io/api/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

var KubeConfigLocation string

type Server struct {
	// TODO: HTTP reverse proxy should be here
	c *backend.ClientGenerator
}

type Node struct {
	client.Resource

	Node *v1.Node `json:"node"`
}

type ConfigMap struct {
	client.Resource

	ConfigMap *v1.ConfigMap `json:"configmap"`
}

type Dashboard struct {
	client.Resource

	Dashboard *v1alpha1.Infrastructure `json:"dashboard"`
	Host      string                   `json:"host"`
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

func NewServer(c *backend.ClientGenerator, kubeConfig string) *Server {
	KubeConfigLocation = kubeConfig
	s := &Server{
		c: c,
	}
	return s
}

type Result struct {
	client.Resource

	Message string `json:"message"`
}

func NewSchema() *client.Schemas {
	schemas := &client.Schemas{}

	schemas.AddType("apiVersion", client.Resource{})
	schemas.AddType("schema", client.Schema{})
	schemas.AddType("error", client.ServerApiError{})

	nodeSchema(schemas.AddType("node", Node{}))
	clusterSchema(schemas.AddType("cluster", Cluster{}))
	configMapSchema(schemas.AddType("configmap", ConfigMap{}))
	dashboardSchema(schemas.AddType("dashboard", Dashboard{}))

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

func configMapSchema(cm *client.Schema) {
	cm.CollectionMethods = []string{"GET"}
	cm.ResourceMethods = []string{"GET"}

	cm.ResourceFields["configmap"] = client.Field{
		Type:     "struct",
		Nullable: true,
	}
}

func dashboardSchema(dashboard *client.Schema) {
	dashboard.CollectionMethods = []string{"GET", "POST"}
	dashboard.ResourceMethods = []string{"GET", "DELETE"}

	dashboard.ResourceFields["dashboard"] = client.Field{
		Type:     "struct",
		Nullable: true,
	}

}

func toDashboardCollection(list *v1alpha1.InfrastructureList) *client.GenericCollection {
	data := []interface{}{}
	for _, item := range list.Items {
		data = append(data, toDashboardResource(&item, nil))
	}
	return &client.GenericCollection{Data: data, Collection: client.Collection{ResourceType: "dashboard"}}
}

func toDashboardResource(dashboard *v1alpha1.Infrastructure, ingress *v1beta1.Ingress) *Dashboard {
	name := dashboard.GetName()
	host := ""
	if ingress != nil {
		host = ingress.Spec.Rules[0].Host
	}
	if name == "" {
		return nil
	}
	return &Dashboard{
		Resource: client.Resource{
			Id:      string(name),
			Type:    "dashboard",
			Actions: map[string]string{},
		},
		Dashboard: dashboard,
		Host:      host,
	}
}

func toDeleteResource() *Result {
	msg := "success"
	return &Result{
		Resource: client.Resource{
			Id:      string(msg),
			Type:    "dashboard",
			Actions: map[string]string{},
		},
		Message: msg,
	}
}

func toConfigMapResource(configmap *v1.ConfigMap) *ConfigMap {
	name := configmap.GetName()
	if name == "" {
		return nil
	}
	return &ConfigMap{
		Resource: client.Resource{
			Id:      string(name),
			Type:    "configmap",
			Actions: map[string]string{},
		},
		ConfigMap: configmap,
	}
}

func toConfigMapCollection(configmapList *v1.ConfigMapList) *client.GenericCollection {
	data := []interface{}{}
	for _, cm := range configmapList.Items {
		data = append(data, toConfigMapResource(&cm))
	}
	return &client.GenericCollection{Data: data, Collection: client.Collection{ResourceType: "configmap"}}
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
