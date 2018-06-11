package api

import (
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"github.com/rancher/go-rancher/client"
	"k8s.io/api/core/v1"
	"github.com/cnrancher/cube-apiserver/util"
	"github.com/Sirupsen/logrus"
)

var KubeConfigLocation string

var sType *schemaType

type schemaType struct {
	Configmap string
	Dashboard string
	Longhorn  string
	BaseInfo  string
	RancherVM string
}

func GetSchemaType() *schemaType {
	if sType == nil {
		sType = &schemaType{
			Configmap: "configmap",
			Dashboard: "dashboard",
			Longhorn:  "longhorn",
			BaseInfo:  "baseinfo",
			RancherVM: "ranchervm",
		}
	}

	return sType
}

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

type BaseInfo struct {
	client.Resource

	BaseInfo []map[string]string `json:"baseinfo"`
}

type Infrastructure struct {
	client.Resource

	Dashboard *v1alpha1.Infrastructure `json:"dashboard"`
	Longhorn  *v1alpha1.Infrastructure `json:"longhorn"`
	RancherVM *v1alpha1.Infrastructure `json:"ranchervm"`
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
	//configMapSchema(schemas.AddType(GetSchemaType().Configmap, ConfigMap{}))
	dashboardSchema(schemas.AddType(GetSchemaType().Dashboard, Infrastructure{}))
	longhornSchema(schemas.AddType(GetSchemaType().Longhorn, Infrastructure{}))
	rancherVMSchema(schemas.AddType(GetSchemaType().RancherVM, Infrastructure{}))
	baseInfoschema(schemas.AddType(GetSchemaType().BaseInfo, BaseInfo{}))
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

	cm.ResourceFields[GetSchemaType().Configmap] = client.Field{
		Type:     "struct",
		Nullable: true,
	}
}

func baseInfoschema(cm *client.Schema) {
	cm.CollectionMethods = []string{"GET"}
	cm.ResourceMethods = []string{"GET"}

	cm.ResourceFields[GetSchemaType().BaseInfo] = client.Field{
		Type:     "struct",
		Nullable: true,
	}
}

func dashboardSchema(dashboard *client.Schema) {
	dashboard.CollectionMethods = []string{"GET", "POST"}
	dashboard.ResourceMethods = []string{"GET", "DELETE"}

	dashboard.ResourceFields[GetSchemaType().Dashboard] = client.Field{
		Type:     "struct",
		Nullable: true,
	}

}

func longhornSchema(longhorn *client.Schema) {
	longhorn.CollectionMethods = []string{"GET", "POST"}
	longhorn.ResourceMethods = []string{"GET", "DELETE"}

	longhorn.ResourceFields[GetSchemaType().Longhorn] = client.Field{
		Type:     "struct",
		Nullable: true,
	}

}

func rancherVMSchema(longhorn *client.Schema) {
	longhorn.CollectionMethods = []string{"GET", "POST"}
	longhorn.ResourceMethods = []string{"GET", "DELETE"}

	longhorn.ResourceFields[GetSchemaType().RancherVM] = client.Field{
		Type:     "struct",
		Nullable: true,
	}

}

func toInfrastructureCollection(list *v1alpha1.InfrastructureList, infraType string) *client.GenericCollection {
	data := []interface{}{}
	for _, item := range list.Items {
		data = append(data, toInfrastructureResource(&item, infraType, nil, ""))
	}
	return &client.GenericCollection{Data: data, Collection: client.Collection{ResourceType: infraType}}
}

func toInfrastructureResource(infra *v1alpha1.Infrastructure, infraType string, service *v1.Service, ip string) *Infrastructure {
	name := infra.GetName()
	host := ip

	if service != nil {
		logrus.Infof("===========aaaaaa============%s", "toInfrastructureResource")
		port := service.Spec.Ports[0].NodePort
		host = host + ":" + util.Int32ToString(port)
		logrus.Infof("===========host============%s", host)
	}
	logrus.Infof("===========bbbb============%s", "toInfrastructureResource")
	if name == "" {
		return nil
	}
	return &Infrastructure{
		Resource: client.Resource{
			Id:      string(name),
			Type:    infraType,
			Actions: map[string]string{},
		},
		Host:      host,
		Dashboard: infra,
		Longhorn:  infra,
		RancherVM: infra,
	}
}

func toDeleteResource(resType string) *Result {
	msg := "delete success"
	return &Result{
		Resource: client.Resource{
			Id:      string("delete"),
			Type:    resType,
			Actions: map[string]string{},
		},
		Message: msg,
	}
}

func toBaseInfo(baseInfo []map[string]string) *BaseInfo {
	if baseInfo == nil {
		return nil
	}
	return &BaseInfo{
		Resource: client.Resource{
			Id:      string("baseInfo"),
			Type:    GetSchemaType().BaseInfo,
			Actions: map[string]string{},
		},
		BaseInfo: baseInfo,
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
			Type:    GetSchemaType().Configmap,
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
	return &client.GenericCollection{Data: data, Collection: client.Collection{ResourceType: GetSchemaType().Configmap}}
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
