package backend

import (
	"time"

	infracs "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned"
	cubeinformers "github.com/cnrancher/cube-apiserver/k8s/pkg/client/informers/externalversions"
	"github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	infomerv1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/client/informers/externalversions/cube/v1alpha1"

	"github.com/Sirupsen/logrus"
	apics "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/informers/core/v1"
)

var (
	clientGenerator *ClientGenerator
)

type ClientGenerator struct {
	CubeImages          *v1alpha1.InfraImages
	Clientset           kubernetes.Clientset
	Apiclientset        apics.Clientset
	Infraclientset      infracs.Clientset
	InformerFactory     informers.SharedInformerFactory
	CubeInformerFactory cubeinformers.SharedInformerFactory
	serviceLister       listerscorev1.ServiceLister
	serviceSynced       cache.InformerSynced
	InfraInformer       infomerv1alpha1.InfrastructureInformer
	PodInformer         v1.PodInformer
}

func NewClientGenerator(kubeConfig string, images *v1alpha1.InfraImages) *ClientGenerator {
	if clientGenerator == nil {
		var config *rest.Config
		var err error

		if kubeConfig == "" {
			config, err = rest.InClusterConfig()
			if err != nil {
				logrus.Fatalf("RancherCUBE: generate config failed: %v", err)
			}
		}

		config, err = clientcmd.BuildConfigFromFlags("", kubeConfig)
		if err != nil {
			logrus.Fatalf("RancherCUBE: generate config failed: %v", err)
		}

		clientset, err := kubernetes.NewForConfig(config)
		if err != nil {
			logrus.Fatalf("RancherCUBE: generate clientset failed: %v", err)
		}

		apiclientset, err := apics.NewForConfig(config)
		if err != nil {
			logrus.Fatalf("RancherCUBE: generate extensions clientset failed: %v", err)
		}

		infraclientset, err := infracs.NewForConfig(config)
		if err != nil {
			logrus.Fatalf("RancherCUBE: generate infra clientset failed: %v", err)
		}

		informerFactory := informers.NewSharedInformerFactory(clientset, time.Second*30)
		infraInformerFactory := cubeinformers.NewSharedInformerFactory(infraclientset, time.Second*30)
		infraInformer := infraInformerFactory.Cube().V1alpha1().Infrastructures()
		podInformer := informerFactory.Core().V1().Pods()
		serviceInformer := informerFactory.Core().V1().Services()

		clientGenerator = &ClientGenerator{
			CubeImages:          images,
			Clientset:           *clientset,
			Apiclientset:        *apiclientset,
			Infraclientset:      *infraclientset,
			InformerFactory:     informerFactory,
			CubeInformerFactory: infraInformerFactory,
			serviceLister:       serviceInformer.Lister(),
			serviceSynced:       serviceInformer.Informer().HasSynced,
			InfraInformer:       infraInformer,
			PodInformer:         podInformer,
		}
	}

	return clientGenerator
}
