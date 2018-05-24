package controller

import (
	infralisters "github.com/rancher/rancher-cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"
	"k8s.io/client-go/kubernetes"
	listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	infraclientset "k8s.io/sample-controller/pkg/client/clientset/versioned"
)

type InfraController struct {
	clientset         kubernetes.Interface
	infraclientset    infraclientset.Interface
	deploymentsLister listers.DeploymentLister
	deploymentsSynced cache.InformerSynced
	infrasLister      infralisters.InfrastructureLister
	infrasSynced      cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface
	// recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	recorder record.EventRecorder
}
