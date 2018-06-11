package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/backend"
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

	err = s.c.ServiceGet(controller.LonghornNamespace, "longhorn-frontend")
	if err != nil {
		return errors.Wrap(err, "failed to get longhorn service")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}
	apiContext.Write(toInfrastructureResource(lh, GetSchemaType().Longhorn, backend.Service, ip))
	return nil
}

func (s *Server) LonghornCreate(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	lh, err := s.c.LonghornDeploy()
	if err != nil {
		return errors.Wrap(err, "failed to create longhorn")
	}

	err = s.c.ServiceGet(controller.LonghornNamespace, "longhorn-frontend")
	if err != nil {
		return errors.Wrap(err, "failed to get longhorn service")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}
	apiContext.Write(toInfrastructureResource(lh, GetSchemaType().Longhorn, backend.Service, ip))
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
