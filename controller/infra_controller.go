package controller

import (
	"errors"
	"fmt"
	"github.com/golang/glog"
	"time"

	infrav1alpha1 "github.com/rancher/rancher-cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	infraclientset "github.com/rancher/rancher-cube-apiserver/k8s/pkg/client/clientset/versioned"
	infrascheme "github.com/rancher/rancher-cube-apiserver/k8s/pkg/client/clientset/versioned/scheme"
	infrainformers "github.com/rancher/rancher-cube-apiserver/k8s/pkg/client/informers/externalversions"
	infralisters "github.com/rancher/rancher-cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"
	"github.com/rancher/rancher-cube-apiserver/util"
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
)

const controllerAgentName = "infra-controller"
const infrastructureNamespace = "kube-system"

const (
	SuccessSynced         = "Synced"
	ErrResourceExists     = "ErrResourceExists"
	MessageResourceExists = "Resource %q already exists and is not managed by Infrastructure"
	MessageResourceSynced = "Infrastructure synced successfully"
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

func NewController(
	clientset kubernetes.Interface,
	infraclientset infraclientset.Interface,
	informerFactory informers.SharedInformerFactory,
	infraInformerFactory infrainformers.SharedInformerFactory) *InfraController {

	// obtain references to shared index informers for the Deployment and Infrastructure
	// types.
	deploymentInformer := informerFactory.Apps().V1().Deployments()
	infraInformer := infraInformerFactory.Cube().V1alpha1().Infrastructures()

	// Create event broadcaster
	infrascheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &InfraController{
		clientset:         clientset,
		infraclientset:    infraclientset,
		deploymentsLister: deploymentInformer.Lister(),
		deploymentsSynced: deploymentInformer.Informer().HasSynced,
		infrasLister:      infraInformer.Lister(),
		infrasSynced:      infraInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Infrastructures"),
		recorder:          recorder,
	}
	glog.Info("Setting up event handlers")

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
	glog.Info("Starting Infrastructure controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced, c.infrasSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch four workers to process Infrastructure resources
	for i := 0; i < threadiness; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")

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
		glog.Infof("Successfully synced '%s'", key)
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
	infra, err := c.infrasLister.Infrastructures(namespace).Get(name)
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
		//deployment, err = c.clientset.AppsV1().Deployments(infra.Namespace).Create(newDeployment(infra))
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
		msg := fmt.Sprintf(MessageResourceExists, deployment.Name)
		c.recorder.Event(infra, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the Infrastructure resource is specified, and the
	// number does not equal the current desired replicas on the Deployment, we
	// should update the Deployment resource.
	if infra.Spec.Replicas != nil && *infra.Spec.Replicas != *deployment.Spec.Replicas {
		glog.V(4).Infof("Infrastructure %s replicas: %d, deployment replicas: %d", name, *infra.Spec.Replicas, *deployment.Spec.Replicas)
		//deployment, err = c.clientset.AppsV1().Deployments(infra.Namespace).Update(newDeployment(infra))
		deployment, err = c.bundleCreate(infra)
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

	c.recorder.Event(infra, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
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
		glog.V(4).Infof("Recovered deleted object '%s' from tombstone", object.GetName())
	}
	glog.V(4).Infof("Processing object: %s", object.GetName())
	if ownerRef := metav1.GetControllerOf(object); ownerRef != nil {
		// If this object is not owned by a Infrastructure, we should not do anything more
		// with it.
		if ownerRef.Kind != "Infrastructure" {
			return
		}

		infra, err := c.infrasLister.Infrastructures(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of infrastructure '%s'", object.GetSelfLink(), ownerRef.Name)
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
		return c.createLoghorn(infra)
	case "RancherVM":
		return c.createRancherVM(infra)
	}
	return nil, errors.New("error bundle create: infrastructure type " + infra.Spec.InfraKind + " is invalid")
}

// detectService will check the Infrastructure service weather healthy or not.
func (c *InfraController) detectService(infra *infrav1alpha1.Infrastructure) (bool, error) {
	serviceName := ""

	switch infra.Spec.InfraKind {
	case "Dashboard":
		serviceName = "kubernetes-dashboard"
	case "Longhorn":
		// TODO: need change to the real name
		serviceName = "longhorn"
	case "RancherVM":
		// TODO: need change to the real name
		serviceName = "rancher-vm"
	default:
		return false, errors.New("error detect service: infrastructure type " + infra.Spec.InfraKind + " is invalid")
	}

	// TODO: need change to serviceInformer
	_, err := c.clientset.CoreV1().Services(infrastructureNamespace).Get(serviceName, util.GetOptions)
	if err != nil {
		return false, err
	}

	return true, nil
}

func (c *InfraController) ensureNamespaceExists() error {
	_, err := c.clientset.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: infrastructureNamespace,
		},
	})
	return err
}

func (c *InfraController) createDashboard(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	err := c.ensureNamespaceExists()
	if err == nil || k8serrors.IsAlreadyExists(err) {
		// create dashboard secret
		_, err = c.clientset.CoreV1().Secrets(infrastructureNamespace).Create(&corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-certs",
				Namespace: infrastructureNamespace,
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
		_, err := c.clientset.CoreV1().ServiceAccounts(infrastructureNamespace).Create(&corev1.ServiceAccount{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard",
				Namespace: infrastructureNamespace,
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
		_, err = c.clientset.RbacV1().Roles(infrastructureNamespace).Create(&rbacv1.Role{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-minimal",
				Namespace: infrastructureNamespace,
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
		_, err = c.clientset.RbacV1().RoleBindings(infrastructureNamespace).Create(&rbacv1.RoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kubernetes-dashboard-minimal",
				Namespace: infrastructureNamespace,
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
					Namespace: infrastructureNamespace,
				},
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create dashboard service
		_, err = c.clientset.CoreV1().Services(infrastructureNamespace).Create(&corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Namespace: infrastructureNamespace,
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
			},
		})
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}

		// create dashboard deployment
		historyLimit := int32(1)
		deployment, err := c.clientset.AppsV1().Deployments(infrastructureNamespace).Create(&appsv1.Deployment{
			ObjectMeta: metav1.ObjectMeta{
				Name: "kubernetes-dashboard",
				Labels: map[string]string{
					"k8s-app": "kubernetes-dashboard",
				},
				Namespace: infrastructureNamespace,
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

func (c *InfraController) createLoghorn(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	return nil, nil
}

func (c *InfraController) createRancherVM(infra *infrav1alpha1.Infrastructure) (*appsv1.Deployment, error) {
	return nil, nil
}
