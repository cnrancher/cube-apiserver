package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/backend"
)

func (s *Server) DashboardList(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	list, err := s.c.DashBoardList()
	if err != nil || list == nil {
		return errors.Wrap(err, "failed to read dashboard")
	}
	apiContext.Write(toInfrastructureCollection(list, GetSchemaType().Dashboard))
	return nil
}

func (s *Server) DashboardGet(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	db, err := s.c.DashBoardGet()
	if err != nil || db == nil {
		return errors.Wrap(err, "failed to read dashboard")
	}

	err = s.c.ServiceGet(controller.DashboardNamespace, "kubernetes-dashboard")
	if err != nil {
		return errors.Wrap(err, "failed to get service")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}
	apiContext.Write(toInfrastructureResource(db, GetSchemaType().Dashboard, backend.Service, ip))
	return nil
}

func (s *Server) DashboardCreate(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	db, err := s.c.DashBoardDeploy()
	if err != nil {
		return errors.Wrap(err, "failed to create dashboard")
	}

	err = s.c.ServiceGet(controller.DashboardNamespace, "kubernetes-dashboard")
	if err != nil {
		return errors.Wrap(err, "failed to get service")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}
	apiContext.Write(toInfrastructureResource(db, GetSchemaType().Dashboard, backend.Service, ip))
	return nil
}

func (s *Server) DashboardDelete(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	err := s.c.DashBoardDelete()
	if err != nil {
		return errors.Wrap(err, "failed to delete dashboard")
	}
	apiContext.Write(toDeleteResource(GetSchemaType().Dashboard))
	return nil
}
