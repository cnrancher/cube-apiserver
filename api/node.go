package api

import (
	"net/http"

	"github.com/gorilla/mux"
	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
)

func (s *Server) NodeList(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	nodes, err := s.c.ClusterNodes()
	if err != nil || nodes == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read nodes")
	}
	apiContext.Write(toNodeCollection(nodes))
	return nil
}

func (s *Server) NodeGet(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	nodeId := mux.Vars(req)["id"]

	node, err := s.c.ClusterNode(nodeId)
	if err != nil || node == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read node")
	}
	apiContext.Write(toNodeResource(node))
	return nil
}
