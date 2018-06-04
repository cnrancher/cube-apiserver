package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
)

func (s *Server) ClusterList(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	resources, err := s.c.ClusterResources()
	if err != nil || resources == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read clusters resources")
	}
	componentStatuses, err := s.c.ClusterComponentStatuses()
	if err != nil || componentStatuses == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read clusters component statuses")
	}

	apiContext.Write(toClusterCollection(resources, componentStatuses))
	return nil
}
