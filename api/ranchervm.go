package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/cnrancher/cube-apiserver/controller"
	"github.com/cnrancher/cube-apiserver/backend"
	"github.com/Sirupsen/logrus"
)

func (s *Server) RancherVMList(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	list, err := s.c.RancherVMList()
	if err != nil || list == nil {
		return errors.Wrap(err, "failed to read ranchervm")
	}
	apiContext.Write(toInfrastructureCollection(list, GetSchemaType().RancherVM))
	return nil
}

func (s *Server) RancherVMGet(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	vm, err := s.c.RancherVMGet()
	if err != nil || vm == nil {
		return errors.Wrap(err, "failed to read ranchervm")
	}

	err = s.c.ServiceGet(controller.RancherVMNamespace, "ranchervm-frontend")
	if err != nil {
		return errors.Wrap(err, "failed to get ranchervm service")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}

	logrus.Infof("======RancherVMGet=====ip============%s", ip)
	apiContext.Write(toInfrastructureResource(vm, GetSchemaType().RancherVM, backend.Service, ip))
	return nil
}

func (s *Server) RancherVMCreate(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	vm, err := s.c.RancherVMDeploy()
	if err != nil {
		return errors.Wrap(err, "failed to create ranchervm")
	}

	err = s.c.ServiceGet(controller.RancherVMNamespace, "ranchervm-frontend")
	if err != nil {
		return errors.Wrap(err, "failed to get ranchervm service")
	}

	ip, err := s.c.NodeIPGet()
	if err != nil {
		return errors.Wrap(err, "failed to get node ip")
	}

	logrus.Infof("======RancherVMCreate=====ip============%s", ip)
	apiContext.Write(toInfrastructureResource(vm, GetSchemaType().RancherVM, backend.Service, ip))
	return nil
}

func (s *Server) RancherVMDelete(rw http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	err := s.c.RancherVMDelete()
	if err != nil {
		return errors.Wrap(err, "failed to delete ranchervm")
	}
	apiContext.Write(toDeleteResource(GetSchemaType().RancherVM))
	return nil
}
