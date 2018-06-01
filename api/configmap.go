package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
	"github.com/cnrancher/cube-apiserver/backend"
)

func (s *Server) ConfigMapGet(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)
	//ns := mux.Vars(req)["ns"]
	//cmId := mux.Vars(req)["id"]
	cm, err := s.c.ConfigMapGet(backend.KubeSystemNamespace, backend.ConfigMapName)
	if err != nil || cm == nil {
		return errors.Wrap(err, "failed to read configmap")
	}
	apiContext.Write(toConfigMapResource(cm))
	return nil
}

//func (s *Server) ConfigMapList(rw http.ResponseWriter, req *http.Request) (err error) {
//	apiContext := api.GetApiContext(req)
//	cms, err := s.c.ConfigMapList()
//	if err != nil {
//		return err
//	}
//	apiContext.Write(toConfigMapCollection(cms))
//	return nil
//}
