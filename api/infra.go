package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/gorilla/mux"
)

func (s *Server) InfrastructureList(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	list, err := s.c.InfrastructureList()
	if err != nil || list == nil {
		return errors.Wrap(err, "failed to read infrastructure")
	}
	apiContext.Write(toInfrastructureCollection(list))
	return nil
}

func (s *Server) InfrastructureGet(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	kind := mux.Vars(req)["kind"]

	db, err := s.c.InfrastructureGet(kind,false)
	if err != nil || db == nil {
		return errors.Wrap(err, "failed to read infrastructure")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}
	apiContext.Write(toInfrastructureResource(db, backend.Service, ip))
	return nil
}

func (s *Server) InfrastructureCreate(rw http.ResponseWriter, req *http.Request) error {
	var input Infrastructure
	apiContext := api.GetApiContext(req)
	if err := apiContext.Read(&input); err != nil {
		return err
	}
	db, err := s.c.InfrastructureDeploy(input.Kind)
	if err != nil {
		return errors.Wrap(err, "failed to create infrastructure")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}
	apiContext.Write(toInfrastructureResource(db, backend.Service, ip))
	return nil
}

func (s *Server) InfrastructureDelete(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	kind := mux.Vars(req)["kind"]

	err := s.c.InfrastructureDelete(kind)
	if err != nil {
		return errors.Wrapf(err, "failed to delete infrastructure %s", kind)
	}
	apiContext.Write(toDeleteResource(kind))
	return nil
}
