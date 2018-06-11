package controller

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	infrav1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	infraclientset "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned"
	infrascheme "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned/scheme"
	infrainformers "github.com/cnrancher/cube-apiserver/k8s/pkg/client/informers/externalversions"
	infralisters "github.com/cnrancher/cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"
	"github.com/pkg/errors"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	listers "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	listerscorev1 "k8s.io/client-go/listers/core/v1"
	"k8s.io/api/extensions/v1beta1"
)

type InfraController struct {
	clientset         kubernetes.Interface
	infraclientset    infraclientset.Interface
	deploymentsLister listers.DeploymentLister
	deploymentsSynced cache.InformerSynced
	infraLister       infralisters.InfrastructureLister
	infraSynced       cache.InformerSynced
	serviceLister     listerscorev1.ServiceLister
	serviceSynced     cache.InformerSynced

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

func NewInfraController(
	clientset kubernetes.Interface,
	infraclientset infraclientset.Interface,
	informerFactory informers.SharedInformerFactory,
	infraInformerFactory infrainformers.SharedInformerFactory) *InfraController {

	// obtain references to shared index informers for the Deployment and Infrastructure
	// types.
	deploymentInformer := informerFactory.Apps().V1().Deployments()
	infraInformer := infraInformerFactory.Cube().V1alpha1().Infrastructures()
	serviceInformer := informerFactory.Core().V1().Services()

	// Create event broadcaster
	infrascheme.AddToScheme(scheme.Scheme)
	logrus.Infof("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(logrus.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: InfraControllerAgentName})

	controller := &InfraController{
		clientset:         clientset,
		infraclientset:    infraclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		infraLister:       infraInformer.Lister(),
		infraSynced:       infraInformer.Informer().HasSynced,
		serviceLister:     serviceInformer.Lister(),
		serviceSynced:     serviceInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Infrastructures"),
		recorder:          recorder,
	}
	logrus.Infof("Setting up event handlers")

	// Set up an event handler for when Infrastructure resources change
	infraInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueInfra,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueInfra(new)
		},
	})

	// Set up an event handler for when Deployment resources change. This
	// handler will lookup the owner of the given Deployment, and if it is
	// owned by a Infrastructure resource will enqueue that Infrastructure resource for
	// processing. This way, we don't need to implement custom logic for
	// handling Deployment resources. More info on this pattern:
	// https://github.com/kubernetes/community/blob/8cafef897a22026d42f5e5bb3f104febe7e29830/contributors/devel/controllers.md
	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newDepl := new.(*appsv1.Deployment)
			oldDepl := old.(*appsv1.Deployment)
			if newDepl.ResourceVersion == oldDepl.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	return controller

}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *InfraController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	logrus.Infof("Starting Infrastructure controller")

	// Wait for the caches to be synced before starting workers
	logrus.Infof("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.infraSynced, c.serviceSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	logrus.Infof("Starting workers")
	// Launch four workers to process Infrastructure resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	logrus.Infof("Started workers")
	<-stopCh
	logrus.Infof("Shutting down workers")

	return nil
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *InfraController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *InfraController) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// Infrastructure resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		logrus.Infof("Successfully synced '%s'", key)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Infrastructure resource
// with the current status of the resource.
func (c *InfraController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the Infrastructure resource with this namespace/name
	infra, err := c.infraLister.Infrastructures(namespace).Get(name)
	if err != nil {
		// The Infrastructure resource may no longer exist, in which case we stop
		// processing.
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("infrastructure '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	deploymentName := infra.Name
	if deploymentName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	// Get the deployment with the name specified in Infrastructure.spec
	deployment, err := c.deploymentsLister.Deployments(infra.Namespace).Get(deploymentName)
	// If the resource doesn't exist, we'll create it
	if k8serrors.IsNotFound(err) {
		deployment, err = c.bundleCreate(infra)
	}

	// If an error occurs during Get/Create, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// If the Deployment is not controlled by this Infrastructure resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(deployment, infra) {
		msg := fmt.Sprintf(InfraMessageResourceExists, deployment.Name)
		c.recorder.Event(infra, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the Infrastructure resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	if infra.Spec.Replicas != nil && *infra.Spec.Replicas != *deployment.Spec.Replicas {
		logrus.Infof("Infrastructure %s replicas: %d, deployment replicas: %d", name, *infra.Spec.Replicas, *deployment.Spec.Replicas)
		deployment, err = c.bundleUpdate(infra)
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	// Finally, we update the status block of the Infrastructure resource to reflect the
	// current state of the world
	err = c.updateInfra(infra, deployment)
	if err != nil {
		return err
	}

	c.recorder.Event(infra, corev1.EventTypeNormal, SuccessSynced, InfraMessageResourceSynced)
	return nil
}

func (c *InfraController) updateInfra(infra *infrav1alpha1.Infrastructure, deployment *appsv1.Deployment) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	infraCopy := infra.DeepCopy()
	infraCopy.Status.AvailableReplicas = deployment.Status.AvailableReplicas

	// Detect Infrastructure Service
	isHealthy, serviceErr := c.detectService(infraCopy)

	if isHealthy && infraCopy.Status.AvailableReplicas > 0 {
		infraCopy.Status.State = "Healthy"
	} else {
		infraCopy.Status.State = "UnHealthy"
		if serviceErr != nil {
			infraCopy.Status.Message = serviceErr.Error()
		}
	}

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Infrastructure resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := c.infraclientset.CubeV1alpha1().Infrastructures(infra.Namespace).Update(infraCopy)
	return err
}

// enqueueInfra takes a Infrastructure resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Infrastructure.
func (c *InfraController) enqueueInfra(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

// handleObject will take any resource implementing metav1.Object and attempt
// to find the Infrastructure resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that Infrastructure resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *InfraController) handleObject(obj interface{}) {
	var object metav1.Object
	var ok bool
	if object, ok = obj.(metav1.Object); !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object, invalid type"))
			return
		}
		object, ok = tombstone.Obj.(metav1.Object)
		if !ok {
			runtime.HandleError(fmt.Errorf("error decoding object tombstone, invalid type"))
			return
		}
		logrus.Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	logrus.Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Infrastructure, we should not do anything more
		// with it.
		if ownerRef.Kind != "Infrastructure" {
			return
		}

		infra, err := c.infraLister.Infrastructures(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			logrus.Infof("ignoring orphaned object '%s' of infrastructure '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueInfra(infra)
		return
	}
}

// bundleCreate will create all Infrastructure needed such as(Service, Rbac, Pv, Pvc, Deployment, etc...)
// currently infrastructure types of support include (Dashboard, Longhorn, RancherVM)
func (c *InfraController) bundleCreate(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	switch infra.Spec.InfraKind {
	case "Dashboard":
		return c.createDashboard(infra)
	case "Longhorn":
		return c.createLonghorn(infra)
	case "RancherVM":
		logrus.Infof("=====zzzz=====infra===%s", infra.Name)
		return c.createRancherVM(infra)
	}
	return nil, errors.New("error bundle create: infrastructure type " + infra.Spec.InfraKind + " is invalid")
}

// bundleUpdate will update all Infrastructure needed such as(Service, Rbac, Pv, Pvc, Deployment, etc...)
// currently infrastructure types of support include (Dashboard, Longhorn, RancherVM)
func (c *InfraController) bundleUpdate(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	switch infra.Spec.InfraKind {
	case "Dashboard":
		return c.updateDashboard(infra)
	case "Longhorn":
		return c.updateLonghorn(infra)
	case "RancherVM":
		logrus.Infof("=====yyyy=====infra===%s", infra.Name)
		return c.updateRancherVM(infra)
	}
	return nil, errors.New("error bundle update: infrastructure type " + infra.Spec.InfraKind + " is invalid")
}

// detectService will check the Infrastructure service weather healthy or not.
func (c *InfraController) detectService(infra *infrav1alpha1.Infrastructure) (bool, error) {
	serviceName := ""
	namespace := ""

	switch infra.Spec.InfraKind {
	case "Dashboard":
		serviceName = "kubernetes-dashboard"
		namespace = InfrastructureNamespace
	case "Longhorn":
		serviceName = "longhorn-frontend"
		namespace = LonghornNamespace
	case "RancherVM":
		serviceName = "ranchervm-frontend"
		namespace = RancherVMNamespace
	default:
		return false, errors.New("error detect service: infrastructure type " + infra.Spec.InfraKind + " is invalid")
	}

	_, err := c.serviceLister.Services(namespace).Get(serviceName)

	if err != nil {
		logrus.Infof("=====jjjjjjjj=====infra===%s", infra.Name)
		return false, err
	}

	return true, nil
}

func (c *InfraController) ensureNamespaceExists(namespace string) error {
	_, err := c.clientset.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
	return err
}

func (c *InfraController) createDashboard(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	err := c.ensureNamespaceExists(InfrastructureNamespace)
	if err == nil || k8serrors.IsAlreadyExists(err) {
		// create dashboard secret
		_, err = c.clientset.CoreV1().Secrets(InfrastructureNamespace).Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-certs",
				Namespace: InfrastructureNamespace,
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Type: corev1.SecretTypeOpaque,
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create dashboard serviceAccount
		_, err := c.clientset.CoreV1().ServiceAccounts(InfrastructureNamespace).Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: InfrastructureNamespace,
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create dashboard role
		_, err = c.clientset.RbacV1().Roles(InfrastructureNamespace).Create(&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-minimal",
				Namespace: InfrastructureNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{""},
					Resources: []string{"secrets"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"configmaps"},
					Verbs:     []string{"create"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"secrets"},
					ResourceNames: []string{"kubernetes-dashboard-key-holder", "kubernetes-dashboard-certs"},
					Verbs:         []string{"get", "update", "delete"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"configmaps"},
					ResourceNames: []string{"kubernetes-dashboard-settings"},
					Verbs:         []string{"get", "update"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"services"},
					ResourceNames: []string{"heapster"},
					Verbs:         []string{"proxy"},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"services/proxy"},
					ResourceNames: []string{"heapster", "http:heapster:", "https:heapster:"},
					Verbs:         []string{"get"},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create dashboard roleBinding
		_, err = c.clientset.RbacV1().RoleBindings(InfrastructureNamespace).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-minimal",
				Namespace: InfrastructureNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "Role",
				Name:     "kubernetes-dashboard-minimal",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "kubernetes-dashboard",
					Namespace: InfrastructureNamespace,
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		//kubectl create clusterrolebinding add-on-cluster-admin \
		//--clusterrole=cluster-admin \
		//--serviceaccount=kube-system:default

		//RoleRef: rbacv1.RoleRef{
		//	APIGroup: "rbac.authorization.k8s.io",
		//	Kind:     "ClusterRole",
		//	Name:     "longhorn-role",
		//},
		//	Subjects: []rbacv1.Subject{
		//		{
		//			Kind:      "ServiceAccount",
		//			Name:      "longhorn-service-account",
		//			Namespace: LonghornNamespace,
		//		},
		//	},

		// create dashboard clusterRoleBinding
		//_, err = c.clientset.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
		//	ObjectMeta: metav1.ObjectMeta{
		//		Name: "kubernetes-dashboard-admin",
		//		OwnerReferences: []metav1.OwnerReference{
		//			*metav1.NewControllerRef(infra, schema.GroupVersionKind{
		//				Group:   infrav1alpha1.SchemeGroupVersion.Group,
		//				Version: infrav1alpha1.SchemeGroupVersion.Version,
		//				Kind:    "Infrastructure",
		//			}),
		//		},
		//	},
		//	RoleRef: rbacv1.RoleRef{
		//		APIGroup: "rbac.authorization.k8s.io",
		//		Kind:     "ClusterRole",
		//		Name:     "cluster-admin",
		//	},
		//	Subjects: []rbacv1.Subject{
		//		{
		//			Kind:      "ServiceAccount",
		//			Name:      "default",
		//			Namespace: InfrastructureNamespace,
		//		},
		//	},
		//})
		//if err != nil && !k8serrors.IsAlreadyExists(err) {
		//	return nil, err
		//}

		// create dashboard service
		_, err = c.clientset.CoreV1().Services(InfrastructureNamespace).Create(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Namespace: InfrastructureNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       9090,
						TargetPort: intstr.FromInt(9090),
					},
				},
				Selector: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Type: "NodePort",
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create dashboard deployment
		historyLimit := int32(1)
		deployment, err := c.clientset.AppsV1().Deployments(InfrastructureNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Namespace: InfrastructureNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             infra.Spec.Replicas,
				RevisionHistoryLimit: &historyLimit,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"k8s-app": "kubernetes-dashboard",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"k8s-app": "kubernetes-dashboard",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "kubernetes-dashboard",
								Image: "k8s.gcr.io/kubernetes-dashboard-amd64:v1.8.3",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 9090,
										Protocol:      "TCP",
									},
								},
								Args: []string{
									"--enable-insecure-login=true",
									"--insecure-bind-address=0.0.0.0",
									"--insecure-port=9090",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "kubernetes-dashboard-certs",
										MountPath: "/certs",
									},
									{
										Name:      "tmp-volume",
										MountPath: "/tmp",
									},
								},
								LivenessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Scheme: "HTTP",
											Path:   "/",
											Port:   intstr.FromInt(9090),
										},
									},
									InitialDelaySeconds: 30,
									TimeoutSeconds:      30,
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "kubernetes-dashboard-certs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "kubernetes-dashboard-certs",
									},
								},
							},
							{
								Name: "tmp-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						ServiceAccountName: "kubernetes-dashboard",
						Tolerations: []corev1.Toleration{
							{
								Key:    "node-role.kubernetes.io/master",
								Effect: "NoSchedule",
							},
						},
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		return deployment, nil
	}

	return nil, err
}

func (c *InfraController) createLonghorn(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	err := c.ensureNamespaceExists(LonghornNamespace)
	if err == nil || k8serrors.IsAlreadyExists(err) {

		ownerReferences := []metav1.OwnerReference{
			*metav1.NewControllerRef(infra, schema.GroupVersionKind{
				Group:   infrav1alpha1.SchemeGroupVersion.Group,
				Version: infrav1alpha1.SchemeGroupVersion.Version,
				Kind:    "Infrastructure",
			}),
		}

		ns, err := c.clientset.CoreV1().Namespaces().Get(LonghornNamespace, metav1.GetOptions{})
		lnNs := ns.DeepCopy()
		lnNs.OwnerReferences = ownerReferences
		c.clientset.CoreV1().Namespaces().Update(lnNs)

		// create longhorn serviceAccount
		_, err = c.clientset.CoreV1().ServiceAccounts(LonghornNamespace).Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "longhorn-service-account",
				Namespace: LonghornNamespace,

				OwnerReferences: ownerReferences,
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn clusterRole
		_, err = c.clientset.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longhorn-role",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{""},
					Resources: []string{"pods", "events", "persistentvolumes", "persistentvolumeclaims", "nodes", "proxy/nodes"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"apps"},
					Resources: []string{"daemonsets"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"batch"},
					Resources: []string{"jobs", "cronjobs"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"storage.k8s.io"},
					Resources: []string{"storageclasses"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"longhorn.rancher.io"},
					Resources: []string{"nodes"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"longhorn.rancher.io"},
					Resources: []string{"volumes"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"longhorn.rancher.io"},
					Resources: []string{"engines"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"longhorn.rancher.io"},
					Resources: []string{"replicas"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"longhorn.rancher.io"},
					Resources: []string{"settings"},
					Verbs:     []string{"*"},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn clusterRoleBinding
		_, err = c.clientset.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longhorn-bind",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "longhorn-role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "longhorn-service-account",
					Namespace: LonghornNamespace,
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn daemonset
		privileged := true
		_, err = c.clientset.ExtensionsV1beta1().DaemonSets(LonghornNamespace).Create(&v1beta1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longhorn-manager",
				Labels: map[string]string{
					"app": "longhorn-manager",
				},
				Namespace: LonghornNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: v1beta1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "longhorn-manager",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "longhorn-manager",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "longhorn-manager",
								Image:           "rancher/longhorn-manager:b7f1b01",
								ImagePullPolicy: "Always",
								SecurityContext: &corev1.SecurityContext{
									Privileged: &privileged,
								},
								Command: []string{
									"longhorn-manager",
									"-d",
									"daemon",
									"--engine-image",
									"rancher/longhorn-engine:91aa784",
									"--manager-image",
									"rancher/longhorn-manager:b7f1b01",
									"--service-account",
									"longhorn-service-account",
								},
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 9500,
									},
								},

								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "dev",
										MountPath: "/host/dev/",
									},
									{
										Name:      "proc",
										MountPath: "/host/proc/",
									},
									{
										Name:      "varrun",
										MountPath: "/var/run/",
									},
									{
										Name:      "longhorn",
										MountPath: "/var/lib/rancher/longhorn/",
									},
								},
								Env: []corev1.EnvVar{
									{
										Name: "POD_NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
										},
									},
									{
										Name: "POD_IP",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "status.podIP"},
										},
									},
									{
										Name: "NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
										},
									},
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "dev",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/dev/",
									},
								},
							},
							{
								Name: "proc",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/proc/",
									},
								},
							},
							{
								Name: "varrun",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/run/",
									},
								},
							},
							{
								Name: "longhorn",
								VolumeSource: corev1.VolumeSource{
									HostPath: &corev1.HostPathVolumeSource{
										Path: "/var/lib/rancher/longhorn/",
									},
								},
							},
						},
						ServiceAccountName: "longhorn-service-account",
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn backend service
		_, err = c.clientset.CoreV1().Services(LonghornNamespace).Create(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longhorn-backend",
				Labels: map[string]string{
					"app": "longhorn-manager",
				},
				Namespace: LonghornNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       9500,
						TargetPort: intstr.FromInt(9500),
					},
				},
				Selector: map[string]string{
					"app": "longhorn-manager",
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn ui service
		_, err = c.clientset.CoreV1().Services(LonghornNamespace).Create(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longhorn-frontend",
				Labels: map[string]string{
					"app": "longhorn-ui",
				},
				Namespace: LonghornNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Port:       9091, //80,
						TargetPort: intstr.FromInt(8000),
					},
				},
				Selector: map[string]string{
					"app": "longhorn-ui",
				},
				Type: "NodePort",
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn driver deployment
		_, err = c.clientset.AppsV1().Deployments(LonghornNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "longhorn-flexvolume-driver-deployer",
				Namespace: LonghornNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: infra.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "longhorn-flexvolume-driver-deployer",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "longhorn-flexvolume-driver-deployer",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "longhorn-flexvolume-driver-deployer",
								Image:           "rancher/longhorn-manager:fabeb53",
								ImagePullPolicy: "Always",

								Command: []string{
									"longhorn-manager",
									"-d",
									"deploy-flexvolume-driver",
									"--manager-image",
									"rancher/longhorn-manager:fabeb53",
								},

								Env: []corev1.EnvVar{
									{
										Name: "POD_NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
										},
									},
									{
										Name: "NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
										},
									},
									{
										Name:  "FLEXVOLUME_DIR",
										Value: "",
									},
								},
							},
						},
						ServiceAccountName: "longhorn-service-account",
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		//var replicas int32 = 1
		//replicas := int32(1)
		// create longhorn ui deployment
		deployment, err := c.clientset.AppsV1().Deployments(LonghornNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "longhorn-ui",
				Labels: map[string]string{
					"app": "longhorn-ui",
				},
				Namespace: LonghornNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: /*&replicas*/ infra.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "longhorn-ui",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "longhorn-ui",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "longhorn-ui",
								Image: "rancher/longhorn-ui:6d74dfc",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 8000,
									},
								},
								Env: []corev1.EnvVar{
									{
										Name:  "LONGHORN_MANAGER_IP",
										Value: "http://longhorn-backend:9500",
									},
								},
							},
						},
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		return deployment, nil
	}

	return nil, err
}

func (c *InfraController) createRancherVM(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	err := c.ensureNamespaceExists(RancherVMNamespace)
	if err == nil || k8serrors.IsAlreadyExists(err) {

		// create rancherVM serviceAccount
		_, err := c.clientset.CoreV1().ServiceAccounts(RancherVMNamespace).Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ranchervm-service-account",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create rancherVM clusterRole
		_, err = c.clientset.RbacV1().ClusterRoles().Create(&rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ranchervm-role",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups: []string{"apiextensions.k8s.io"},
					Resources: []string{"customresourcedefinitions"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"", "batch"},
					Resources: []string{"pods", "nodes", "services", "jobs", "endpoints"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"vm.rancher.com"},
					Resources: []string{"virtualmachines"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"vm.rancher.com"},
					Resources: []string{"arptables"},
					Verbs:     []string{"*"},
				},
				{
					APIGroups: []string{"vm.rancher.com"},
					Resources: []string{"credentials"},
					Verbs:     []string{"*"},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create longhorn clusterRoleBinding
		_, err = c.clientset.RbacV1().ClusterRoleBindings().Create(&rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: "ranchervm-bind",
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			RoleRef: rbacv1.RoleRef{
				APIGroup: "rbac.authorization.k8s.io",
				Kind:     "ClusterRole",
				Name:     "ranchervm-role",
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:      "ServiceAccount",
					Name:      "ranchervm-service-account",
					Namespace: RancherVMNamespace,
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create ip controller daemonset
		_, err = c.clientset.ExtensionsV1beta1().DaemonSets(RancherVMNamespace).Create(&v1beta1.DaemonSet{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ip-controller",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: v1beta1.DaemonSetSpec{
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "ip-controller",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "ip-controller",
						},
					},
					Spec: corev1.PodSpec{
						HostNetwork: true,
						Containers: []corev1.Container{
							{
								Name:            "ip-controller",
								Image:           "rancher/vm",
								ImagePullPolicy: "Always",
								Command: []string{
									"sh",
									"-c",
								},
								Args: []string{
									"exec /ranchervm -ip -nodename ${MY_NODE_NAME} -v 3",
								},
								Env: []corev1.EnvVar{
									{
										Name: "MY_NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
										},
									},
								},
							},
							{
								Name:            "arp-scanner",
								Image:           "rancher/vm",
								ImagePullPolicy: "Always",
								Command: []string{
									"bash",
									"-c",
								},
								Args: []string{
									`while true; do iface=br0; inet_addr=$(ifconfig $iface | grep "inet " | awk '{print $2}' | cut -d':' -f2);inet_mask=$(ifconfig $iface | grep "inet " | awk '{print $4}' | cut -d':' -f2);ips="$(arp-scan -interface=$iface $inet_addr:$inet_mask | awk -F'\t' '$2 ~ /[0-9a-f][0-9a-f]:/ {print $1}')";for ip in $ips; do ping -c 1 -t 1 $ip &>/dev/null & done; sleep 5; done`,
								},
							},
						},
						ServiceAccountName: "ranchervm-service-account",
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create ranchervm backend service
		_, err = c.clientset.CoreV1().Services(RancherVMNamespace).Create(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ranchervm-backend",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "api",
						Port: 9500,
						//TargetPort: intstr.FromInt(9090),
					},
				},
				Selector: map[string]string{
					"app": "ranchervm-backend",
				},
				Type: "ClusterIP",
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create ranchervm frontend service
		_, err = c.clientset.CoreV1().Services(RancherVMNamespace).Create(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ranchervm-frontend",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: corev1.ServiceSpec{
				Ports: []corev1.ServicePort{
					{
						Name: "ui",
						Port: 8000,
						//TargetPort: intstr.FromInt(9090),
					},
				},
				Selector: map[string]string{
					"app": "ranchervm-frontend",
				},
				Type: "NodePort",
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create ranchervm controller deployment
		//controllerReplica := int32(2)
		_, err = c.clientset.AppsV1().Deployments(RancherVMNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ranchervm-controller",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: infra.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "ranchervm-controller",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "ranchervm-controller",
						},
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: "In",
													Values: []string{
														"ranchervm-controller",
													},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "ranchervm-controller",
								Image:           "rancher/vm",
								ImagePullPolicy: "Always",
								Args: []string{
									"--vm",
									"--bridge-iface=ens33",
									"--v=3",
								},
							},
						},
						ServiceAccountName: "ranchervm-service-account",
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create ranchervm backend deployment
		_, err = c.clientset.AppsV1().Deployments(RancherVMNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ranchervm-backend",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: infra.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "ranchervm-backend",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "ranchervm-backend",
						},
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: "In",
													Values: []string{
														"ranchervm-backend",
													},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "ranchervm-backend",
								Image:           "rancher/vm",
								ImagePullPolicy: "Always",
								Args: []string{
									"--backend",
								},
							},
						},
						ServiceAccountName: "ranchervm-service-account",
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create ranchervm frontend deployment
		deployment, err := c.clientset.AppsV1().Deployments(RancherVMNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "ranchervm-frontend",
				Namespace: RancherVMNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: infra.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "ranchervm-frontend",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "ranchervm-frontend",
						},
					},
					Spec: corev1.PodSpec{
						Affinity: &corev1.Affinity{
							PodAntiAffinity: &corev1.PodAntiAffinity{
								RequiredDuringSchedulingIgnoredDuringExecution: []corev1.PodAffinityTerm{
									{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: "In",
													Values: []string{
														"ranchervm-frontend",
													},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
						Containers: []corev1.Container{
							{
								Name:            "ranchervm-frontend",
								Image:           "rancher/vm-frontend",
								ImagePullPolicy: "Always",
								Env: []corev1.EnvVar{
									{
										Name:  "LONGHORN_MANAGER_IP",
										Value: "http://ranchervm-backend:9500",
									},
								},
							},
						},
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		return deployment, nil
	}

	return nil, err
}

func (c *InfraController) updateDashboard(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	err := c.ensureNamespaceExists(InfrastructureNamespace)
	if err == nil || k8serrors.IsAlreadyExists(err) {
		// update dashboard deployment
		historyLimit := int32(1)
		deployment, err := c.clientset.AppsV1().Deployments(InfrastructureNamespace).Update(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Namespace: InfrastructureNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas:             infra.Spec.Replicas,
				RevisionHistoryLimit: &historyLimit,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"k8s-app": "kubernetes-dashboard",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"k8s-app": "kubernetes-dashboard",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "kubernetes-dashboard",
								Image: "k8s.gcr.io/kubernetes-dashboard-amd64:v1.8.3",
								Ports: []corev1.ContainerPort{
									{
										ContainerPort: 9090,
										Protocol:      "TCP",
									},
								},
								Args: []string{
									"--enable-insecure-login=true",
									"--insecure-bind-address=0.0.0.0",
									"--insecure-port=9090",
								},
								VolumeMounts: []corev1.VolumeMount{
									{
										Name:      "kubernetes-dashboard-certs",
										MountPath: "/certs",
									},
									{
										Name:      "tmp-volume",
										MountPath: "/tmp",
									},
								},
								LivenessProbe: &corev1.Probe{
									Handler: corev1.Handler{
										HTTPGet: &corev1.HTTPGetAction{
											Scheme: "HTTP",
											Path:   "/",
											Port:   intstr.FromInt(9090),
										},
									},
									InitialDelaySeconds: 30,
									TimeoutSeconds:      30,
								},
							},
						},
						Volumes: []corev1.Volume{
							{
								Name: "kubernetes-dashboard-certs",
								VolumeSource: corev1.VolumeSource{
									Secret: &corev1.SecretVolumeSource{
										SecretName: "kubernetes-dashboard-certs",
									},
								},
							},
							{
								Name: "tmp-volume",
								VolumeSource: corev1.VolumeSource{
									EmptyDir: &corev1.EmptyDirVolumeSource{},
								},
							},
						},
						ServiceAccountName: "kubernetes-dashboard",
						Tolerations: []corev1.Toleration{
							{
								Key:    "node-role.kubernetes.io/master",
								Effect: "NoSchedule",
							},
						},
					},
				},
			},
		})
		if err != nil {
			return nil, err
		}

		return deployment, nil
	}

	return nil, err
}

func (c *InfraController) updateLonghorn(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	err := c.ensureNamespaceExists(LonghornNamespace)
	if err == nil || k8serrors.IsAlreadyExists(err) {
		// create longhorn driver deployment
		deployment, err := c.clientset.AppsV1().Deployments(LonghornNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "longhorn-flexvolume-driver-deployer",
				Namespace: LonghornNamespace,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(infra, schema.GroupVersionKind{
						Group:   infrav1alpha1.SchemeGroupVersion.Group,
						Version: infrav1alpha1.SchemeGroupVersion.Version,
						Kind:    "Infrastructure",
					}),
				},
			},
			Spec: appsv1.DeploymentSpec{
				Replicas: infra.Spec.Replicas,
				Selector: &metav1.LabelSelector{
					MatchLabels: map[string]string{
						"app": "longhorn-flexvolume-driver-deployer",
					},
				},
				Template: corev1.PodTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							"app": "longhorn-flexvolume-driver-deployer",
						},
					},
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:            "longhorn-flexvolume-driver-deployer",
								Image:           "rancher/longhorn-manager:fabeb53",
								ImagePullPolicy: "Always",

								Command: []string{
									"longhorn-manager",
									"-d",
									"deploy-flexvolume-driver",
									"--manager-image",
									"rancher/longhorn-manager:fabeb53",
								},

								Env: []corev1.EnvVar{
									{
										Name: "POD_NAMESPACE",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "metadata.namespace"},
										},
									},
									{
										Name: "NODE_NAME",
										ValueFrom: &corev1.EnvVarSource{
											FieldRef: &corev1.ObjectFieldSelector{FieldPath: "spec.nodeName"},
										},
									},
									{
										Name:  "FLEXVOLUME_DIR",
										Value: "",
									},
								},
							},
						},
						ServiceAccountName: "longhorn-service-account",
					},
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		return deployment, nil
	}

	return nil, err
}

func (c *InfraController) updateRancherVM(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	return nil, nil
}
