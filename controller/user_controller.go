package controller

import (
	"fmt"
	"time"

	userv1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	userclientset "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned"
	userscheme "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned/scheme"
	userinformers "github.com/cnrancher/cube-apiserver/k8s/pkg/client/informers/externalversions"
	userlisters "github.com/cnrancher/cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

type UserController struct {
	clientset     kubernetes.Interface
	userclientset userclientset.Interface

	userLister   userlisters.UserLister
	userInformer cache.SharedIndexInformer
	userSynced   cache.InformerSynced

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

func NewUserController(clientset kubernetes.Interface,
	userclientset userclientset.Interface,
	informerFactory informers.SharedInformerFactory,
	userInformerFactory userinformers.SharedInformerFactory) *UserController {

	// obtain references to shared index informers for the Deployment and User
	// types.
	userInformer := userInformerFactory.Cube().V1alpha1().Users()

	// add index for userInformer
	userIndexers := map[string]cache.IndexFunc{
		UserByPrincipalIndex: userByPrincipal,
		UserByUsernameIndex:  userByUsername,
		UserSearchIndex:      userSearchIndexer,
	}

	if err := userInformer.Informer().AddIndexers(userIndexers); err != nil {
		return nil
	}

	// Create event broadcaster
	userscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: clientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: UserControllerAgentName})

	controller := &UserController{
		clientset:     clientset,
		userclientset: userclientset,
		userLister:    userInformer.Lister(),
		userInformer:  userInformer.Informer(),
		userSynced:    userInformer.Informer().HasSynced,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Users"),
		recorder:      recorder,
	}
	glog.Info("Setting up event handlers")

	// Set up an event handler for when User resources change
	userInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueUser,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueUser(new)
		},
	})

	return controller
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *UserController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting User controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.userSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}

	glog.Info("Starting workers")
	// Launch four workers to process User resources
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
func (c *UserController) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *UserController) processNextWorkItem() bool {
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
		// User resource to be synced.
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
// converge the two. It then updates the Status block of the User resource
// with the current status of the resource.
func (c *UserController) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the User resource with this namespace/name
	user, err := c.userLister.Users(namespace).Get(name)
	if err != nil {
		// The User resource may no longer exist, in which case we stop
		// processing.
		if k8serrors.IsNotFound(err) {
			runtime.HandleError(fmt.Errorf("user '%s' in work queue no longer exists", key))
			return nil
		}

		return err
	}

	userName := user.Name
	if userName == "" {
		// We choose to absorb the error here as the worker would requeue the
		// resource otherwise. Instead, the next time the resource is updated
		// the resource will be queued again.
		runtime.HandleError(fmt.Errorf("%s: deployment name must be specified", key))
		return nil
	}

	c.recorder.Event(user, corev1.EventTypeNormal, SuccessSynced, UserMessageResourceSynced)
	return nil
}

// enqueueUser takes a User resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than User.
func (c *UserController) enqueueUser(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func userByPrincipal(obj interface{}) ([]string, error) {
	u, ok := obj.(*userv1alpha1.User)
	if !ok {
		return []string{}, nil
	}

	return u.PrincipalIDs, nil
}

func userByUsername(obj interface{}) ([]string, error) {
	user, ok := obj.(*userv1alpha1.User)
	if !ok {
		return []string{}, nil
	}
	return []string{user.Username}, nil
}

func userSearchIndexer(obj interface{}) ([]string, error) {
	user, ok := obj.(*userv1alpha1.User)
	if !ok {
		return []string{}, nil
	}
	var fieldIndexes []string

	fieldIndexes = append(fieldIndexes, indexField(user.Username, minOf(len(user.Username), searchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(user.DisplayName, minOf(len(user.DisplayName), searchIndexDefaultLen))...)
	fieldIndexes = append(fieldIndexes, indexField(user.ObjectMeta.Name, minOf(len(user.ObjectMeta.Name), searchIndexDefaultLen))...)

	return fieldIndexes, nil
}

func minOf(length int, defaultLen int) int {
	if length < defaultLen {
		return length
	}
	return defaultLen
}

func indexField(field string, maxindex int) []string {
	var fieldIndexes []string
	for i := 2; i <= maxindex; i++ {
		fieldIndexes = append(fieldIndexes, field[0:i])
	}
	return fieldIndexes
}
