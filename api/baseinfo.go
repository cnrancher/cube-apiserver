package api

import (
	"net/http"

	"github.com/pkg/errors"
	"github.com/rancher/go-rancher/api"
)

func (s *Server) BaseInfoGet(w http.ResponseWriter, req *http.Request) error {
	apiContext := api.GetApiContext(req)

	baseInfo, err := s.c.BaseInfoGet()
	if err != nil {
		return errors.Wrap(err, "failed to read base info")
	}
	apiContext.Write(toBaseInfo(baseInfo))
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
