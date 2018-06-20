package api

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/cnrancher/cube-apiserver/backend"

	"github.com/Sirupsen/logrus"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/cache"
)

func (s *Server) ClusterList(w http.ResponseWriter, req *http.Request) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	flusher, ok := w.(http.Flusher)
	if !ok {
		return errors.New("Streaming not supported")
	}

	impersonateUser := req.Header.Get("Impersonate-User")
	c := backend.NewImpersonateGenerator(KubeConfigLocation, impersonateUser)

	resources, err := c.ClusterResources()
	if err != nil || resources == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read clusters resources")
	}
	componentStatuses, err := s.c.ClusterComponentStatuses()
	if err != nil || componentStatuses == nil {
		return errors.Wrap(err, "RancherCUBE: fail to read clusters component statuses")
	}

	io.WriteString(w, "data: ")
	enc := json.NewEncoder(w)
	enc.Encode(toClusterCollection(resources, componentStatuses))
	io.WriteString(w, "\n\n")
	flusher.Flush()

	eventc := make(chan interface{})
	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	defer func() {
		cancel()
		close(eventc)
		logrus.Debugf("component statuses connection closed")
	}()

	s.c.InformerFactory.Core().V1().ComponentStatuses().Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newStatus := new.(*corev1.ComponentStatus)
			oldStatus := old.(*corev1.ComponentStatus)
			if newStatus.ResourceVersion != oldStatus.ResourceVersion {
				componentStatuses, err := s.c.ClusterComponentStatuses()
				if err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
					eventc <- toClusterCollection(resources, componentStatuses)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
		},
	})

	s.c.PodInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		UpdateFunc: func(old, new interface{}) {
			newPod := new.(*corev1.Pod)
			oldPod := old.(*corev1.Pod)
			if newPod.ResourceVersion != oldPod.ResourceVersion {
				resources, err := c.ClusterResources()
				if err != nil {
					return
				}
				select {
				case <-ctx.Done():
					return
				default:
					eventc <- toClusterCollection(resources, componentStatuses)
				}
			}
		},
		DeleteFunc: func(obj interface{}) {
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
