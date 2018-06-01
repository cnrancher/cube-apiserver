package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/cnrancher/cube-apiserver/controller"
)

func (s *Server) LonghornList(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	list, err := s.c.LonghornList()
	if err != nil || list == nil {
		return errors.Wrap(err, "failed to read longhorn")
	}
	apiContext.Write(toInfrastructureCollection(list, GetSchemaType().Longhorn))
	return nil
}

func (s *Server) LonghornGet(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	lh, err := s.c.LonghornGet()
	if err != nil || lh == nil {
		return errors.Wrap(err, "failed to read longhorn")
	}
	ing, err := s.c.IngressGet(controller.LonghornNamespace, backend.LhIngressName)
	if err != nil {
		return errors.Wrap(err, "failed to get ingress")
	}
	apiContext.Write(toInfrastructureResource(lh, GetSchemaType().Longhorn, ing, 0))
	return nil
}

func (s *Server) LonghornCreate(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	lh, err := s.c.LonghornDeploy()
	if err != nil {
		return errors.Wrap(err, "failed to create longhorn")
	}
	ing, err := s.c.IngressGet(controller.LonghornNamespace, backend.LhIngressName)
	if err != nil {
		return errors.Wrap(err, "failed to get ingress")
	}
	apiContext.Write(toInfrastructureResource(lh, GetSchemaType().Longhorn, ing, 0))
	return nil
}

func (s *Server) LonghornDelete(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	err := s.c.LonghornDelete()
	if err != nil {
		return errors.Wrap(err, "failed to delete longhorn")
	}
	apiContext.Write(toDeleteResource(GetSchemaType().Longhorn))
	return nil
}
