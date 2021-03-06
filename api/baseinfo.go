package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	"k8s.io/client-go/tools/cache"
)

func (s *Server) BaseInfoGet(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("Streaming not supported")
	}

	baseInfo, err := s.c.BaseInfoGet()
	if err != nil {
		return err
	}
	io.WriteString(w, "data: ")
	enc := json.NewEncoder(w)
	enc.Encode(toBaseInfo(baseInfo))
	io.WriteString(w, "\n\n")
	flusher.Flush()

	eventc := make(chan interface{})
	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	defer func() {
		cancel()
		close(eventc)
		logrus.Debugf("Base information connection closed")
	}()

	s.c.InfraInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newInfra := new.(*v1alpha1.Infrastructure)
			oldInfra := old.(*v1alpha1.Infrastructure)
			if newInfra.Status.State != oldInfra.Status.State {
				baseInfo, err := s.c.BaseInfoGet()
				if err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
					eventc <- toBaseInfo(baseInfo)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
			baseInfo, err := s.c.BaseInfoGet()
			if err != nil {
				return
			}
			select {
			case <-ctx.Done():
				return
			default:
				eventc <- toBaseInfo(baseInfo)
			}
		},
	})

	for {
		select {
		case <-w.(http.CloseNotifier).CloseNotify():
			return nil
		case <-ctx.Done():
			return nil
		case <-time.After(time.Second * 30):
			io.WriteString(w, ": ping\n\n")
			flusher.Flush()
		case buf, ok := <-eventc:
			if ok {
				io.WriteString(w, "data: ")
				enc := json.NewEncoder(w)
				enc.Encode(buf)
				io.WriteString(w, "\n\n")
				flusher.Flush()
			}
		}
	}
}
