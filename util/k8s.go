package util

import (
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
)

var (
	AdminVerbs     = []string{"*"}
	AdminResources = []string{"*"}
)

// ListEverything is a list options used to list all resources without any filtering.
var ListEverything = metaV1.ListOptions{
	LabelSelector: labels.Everything().String(),
	FieldSelector: fields.Everything().String(),
}

var GetOptions = metaV1.GetOptions{}

var DeleteOptions = metaV1.DeleteOptions{}

//func GetNodeIP(c *backend.ClientGenerator) error {
//	nodes, err := c.ClusterNodes()
//	if err != nil || nodes == nil {
//		return errors.Wrap(err, "RancherCUBE: fail to read nodes")
//	}


	//var ip string
	//for _, address := range nodes.Items[0].Status.Addresses {
	//	//if address.Type == "ExternalIP" {
	//	//	return address.Address, nil
	//	//}
	//	if address.Type == "InternalIP" {
	//		ip = address.Address
	//	}
	//}

	//db, err := s.c.DashBoardDeploy()
	//if err != nil {
	//	return errors.Wrap(err, "failed to create dashboard")
	//}
	////ing, err := s.c.IngressGet(backend.KubeSystemNamespace, backend.DbIngressName)
	//err = s.c.ServiceGet(controller.InfrastructureNamespace, "kubernetes-dashboard")
	//if err != nil {
	//	return errors.Wrap(err, "failed to get service")
	//}
	//apiContext.Write(toInfrastructureResource(db, GetSchemaType().Dashboard, backend.Service, 0))
//	return nil
//}
