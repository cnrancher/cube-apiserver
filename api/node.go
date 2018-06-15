package api

import (
	"net/http"

	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/rancher/go-rancher/api"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
)

func (s *Server) NodeList(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	impersonateUser := req.Header.Get("Impersonate-User")

	c := backend.NewImpersonateGenerator(KubeConfigLocation, impersonateUser)

	nodes, err := c.ClusterNodes()
	if err != nil || nodes == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read nodes")
	}
	apiContext.Write(toNodeCollection(nodes))
	return nil
}

func (s *Server) NodeGet(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	nodeID := mux.Vars(req)["id"]

	impersonateUser := req.Header.Get("Impersonate-User")

	c := backend.NewImpersonateGenerator(KubeConfigLocation, impersonateUser)

	node, err := c.ClusterNode(nodeID)
	if err != nil || node == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read node")
	}
	apiContext.Write(toNodeResource(node))
	return nil
}
