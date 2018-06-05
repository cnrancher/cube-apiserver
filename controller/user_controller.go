package controller

import (
	"fmt"
	"time"

	userv1alpha1 "github.com/cnrancher/cube-apiserver/k8s/pkg/apis/cube/v1alpha1"
	userclientset "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned"
	userscheme "github.com/cnrancher/cube-apiserver/k8s/pkg/client/clientset/versioned/scheme"
	userinformers "github.com/cnrancher/cube-apiserver/k8s/pkg/client/informers/externalversions"
	userlisters "github.com/cnrancher/cube-apiserver/k8s/pkg/client/listers/cube/v1alpha1"
	"github.com/cnrancher/cube-apiserver/util"

	"github.com/golang/glog"
	"github.com/pkg/errors"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	rbacv1lister "k8s.io/client-go/listers/rbac/v1"
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

	principalLister   userlisters.PrincipalLister
	principalInformer cache.SharedIndexInformer
	principalSynced   cache.InformerSynced

	crbLister   rbacv1lister.ClusterRoleBindingLister
	crbInformer cache.SharedIndexInformer
	crbSynced   cache.InformerSynced

	crLister   rbacv1lister.ClusterRoleLister
	crInformer cache.SharedIndexInformer
	crSynced   cache.InformerSynced

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

	// obtain references to shared index informers for the Principle and User
	// types.
	userInformer := userInformerFactory.Cube().V1alpha1().Users()

	// add index for userInformer
	userIndexers := map[string]cache.IndexFunc{
		UserByPrincipalIndex: UserByPrincipal,
		UserByUsernameIndex:  UserByUsername,
		UserSearchIndex:      UserSearchIndexer,
	}

	if err := userInformer.Informer().AddIndexers(userIndexers); err != nil {
		return nil
	}

	// obtain references to shared index informers for the Principle and User
	// types.
	principalInformer := userInformerFactory.Cube().V1alpha1().Principals()

	// add index for userInformer
	principalIndexers := map[string]cache.IndexFunc{
		PrincipalByIdIndex: PrincipalById,
	}

	if err := principalInformer.Informer().AddIndexers(principalIndexers); err != nil {
		return nil
	}

	// obtain references to shared index informers for the ClusterRoleBinding
	// types.
	crbInformer := informerFactory.Rbac().V1().ClusterRoleBindings()

	// add index for crbInformer
	crbIndexers := map[string]cache.IndexFunc{
		ClusterRoleBindingByNameIndex: ClusterRoleBindingByName,
	}

	if err := crbInformer.Informer().AddIndexers(crbIndexers); err != nil {
		return nil
	}

	// obtain references to shared index informers for the ClusterRole
	// types.
	crInformer := informerFactory.Rbac().V1().ClusterRoles()

	// add index for crInformer
	crIndexers := map[string]cache.IndexFunc{
		ClusterRoleByNameIndex: ClusterRoleByName,
	}

	if err := crInformer.Informer().AddIndexers(crIndexers); err != nil {
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
		clientset:         clientset,
		userclientset:     userclientset,
		userLister:        userInformer.Lister(),
		userInformer:      userInformer.Informer(),
		userSynced:        userInformer.Informer().HasSynced,
		principalLister:   principalInformer.Lister(),
		principalInformer: principalInformer.Informer(),
		principalSynced:   principalInformer.Informer().HasSynced,
		crbLister:         crbInformer.Lister(),
		crbInformer:       crbInformer.Informer(),
		crbSynced:         crbInformer.Informer().HasSynced,
		crLister:          crInformer.Lister(),
		crInformer:        crInformer.Informer(),
		crSynced:          crInformer.Informer().HasSynced,
		workqueue:         workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Users"),
		recorder:          recorder,
	}
	glog.Info("Setting up event handlers")

	// Set up an event handler for when User resources change
	userInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueUser,
		UpdateFunc: func(old, new interface{}) {
			controller.enqueueUser(new)
		},
	})

	// Set up an event handler for when Principal resources change
	principalInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newPrincipal := new.(*userv1alpha1.Principal)
			oldPrincipal := old.(*userv1alpha1.Principal)
			if newPrincipal.ResourceVersion == oldPrincipal.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	// Set up an event handler for when ClusterRoleBinding resources change
	crbInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newCrb := new.(*rbacv1.ClusterRoleBinding)
			oldCrb := old.(*rbacv1.ClusterRoleBinding)
			if newCrb.ResourceVersion == oldCrb.ResourceVersion {
				// Periodic resync will send update events for all known Deployments.
				// Two different versions of the same Deployment will always have different RVs.
				return
			}
			controller.handleObject(new)
		},
		DeleteFunc: controller.handleObject,
	})

	// Set up an event handler for when ClusterRole resources change
	crInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.handleObject,
		UpdateFunc: func(old, new interface{}) {
			newCr := new.(*rbacv1.ClusterRole)
			oldCr := old.(*rbacv1.ClusterRole)
			if newCr.ResourceVersion == oldCr.ResourceVersion {
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
func (c *UserController) Run(threadiness int, stopCh <-chan struct{}) error {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting User controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.userSynced, c.principalSynced, c.crbSynced, c.crSynced); !ok {
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
		runtime.HandleError(fmt.Errorf("%s: user name must be specified", key))
		return nil
	}

	principalLogic := ToPrincipal("user", user.DisplayName, user.Username, user.Namespace, GetLocalPrincipalID(user), nil)

	var principal *userv1alpha1.Principal
	principals, _ := c.principalInformer.GetIndexer().ByIndex(PrincipalByIdIndex, principalLogic.Name)
	if len(principals) <= 0 {
		principal, err = c.createPrincipal(user, &principalLogic)
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}
	if len(principals) > 1 {
		return errors.Errorf("can't find unique principal for user %v", userName)
	}
	if len(principals) == 1 {
		u := principals[0].(*userv1alpha1.Principal)
		principal = u.DeepCopy()
	}

	// If the Principal is not controlled by this User resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(principal, user) {
		msg := fmt.Sprintf(UserMessageResourceExists, principal)
		c.recorder.Event(user, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// If this number of the replicas on the User resource is specified, and the
	// number does not equal the current desired replicas on the Principal, we
	// should update the Principal resource.
	if !matchPrincipalId(user.PrincipalIDs, principal.Name) {
		glog.V(4).Infof("User %s principalIds not contain principal id: %s", user.Name, principal.Name)
		principal, err = c.updatePrincipal(user, principal)
	}

	// If an error occurs during Update, we'll requeue the item so we can
	// attempt processing again later. THis could have been caused by a
	// temporary network failure, or any other transient reason.
	if err != nil {
		return err
	}

	var clusterRole *rbacv1.ClusterRole
	clusterRoles, _ := c.crInformer.GetIndexer().ByIndex(ClusterRoleByNameIndex, userName)
	if len(clusterRoles) <= 0 {
		clusterRole, err = c.generateClusterRole("admin", user)
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}
	if len(clusterRoles) > 1 {
		return errors.Errorf("can't find unique clusterRoles for user %v", userName)
	}
	if len(clusterRoles) == 1 {
		u := clusterRoles[0].(*rbacv1.ClusterRole)
		clusterRole = u.DeepCopy()
	}

	// If the ClusterRole is not controlled by this User resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(clusterRole, user) {
		msg := fmt.Sprintf(UserMessageResourceExists, clusterRole)
		c.recorder.Event(user, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	var clusterRoleBinding *rbacv1.ClusterRoleBinding
	clusterRoleBindings, _ := c.crbInformer.GetIndexer().ByIndex(ClusterRoleBindingByNameIndex, userName)
	if len(clusterRoleBindings) <= 0 {
		clusterRoleBinding, err = c.generateClusterRoleBinding(user)
		// If an error occurs during Get/Create, we'll requeue the item so we can
		// attempt processing again later. This could have been caused by a
		// temporary network failure, or any other transient reason.
		if err != nil {
			return err
		}
	}
	if len(clusterRoleBindings) > 1 {
		return errors.Errorf("can't find unique clusterRoleBindings for user %v", userName)
	}
	if len(clusterRoleBindings) == 1 {
		u := clusterRoleBindings[0].(*rbacv1.ClusterRoleBinding)
		clusterRoleBinding = u.DeepCopy()
	}

	// If the ClusterRoleBinding is not controlled by this User resource, we should log
	// a warning to the event recorder and ret
	if !metav1.IsControlledBy(clusterRoleBinding, user) {
		msg := fmt.Sprintf(UserMessageResourceExists, clusterRoleBinding)
		c.recorder.Event(user, corev1.EventTypeWarning, ErrResourceExists, msg)
		return fmt.Errorf(msg)
	}

	// Finally, we update the status block of the User resource to reflect the
	// current state of the world
	err = c.updateUser(user, principal)
	if err != nil {
		return err
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

// handleObject will take any resource implementing metav1.Object and attempt
// to find the User resource that 'owns' it. It does this by looking at the
// objects metadata.ownerReferences field for an appropriate OwnerReference.
// It then enqueues that User resource to be processed. If the object does not
// have an appropriate OwnerReference, it will simply be skipped.
func (c *UserController) handleObject(obj interface{}) {
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
		// If this object is not owned by a User, we should not do anything more
		// with it.
		if ownerRef.Kind != "User" {
			return
		}

		user, err := c.userLister.Users(object.GetNamespace()).Get(ownerRef.Name)
		if err != nil {
			glog.V(4).Infof("ignoring orphaned object '%s' of user '%s'", object.GetSelfLink(), ownerRef.Name)
			return
		}

		c.enqueueUser(user)
		return
	}
}

func (c *UserController) updateUser(user *userv1alpha1.User, principal *userv1alpha1.Principal) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance
	userCopy := user.DeepCopy()
	found := -1
	for index, id := range userCopy.PrincipalIDs {
		if id == principal.Name {
			found = index
		}
	}

	if found == -1 {
		userCopy.PrincipalIDs = append(userCopy.PrincipalIDs, principal.Name)
	}

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Infrastructure resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	_, err := c.userclientset.CubeV1alpha1().Users(user.Namespace).Update(userCopy)
	return err
}

// createPrincipal will create principal associate to user
func (c *UserController) createPrincipal(user *userv1alpha1.User, principalLogic *userv1alpha1.Principal) (*userv1alpha1.Principal, error) {
	err := c.ensureNamespaceExists()
	if err == nil || k8serrors.IsAlreadyExists(err) {
		principalLogic.OwnerReferences = []metav1.OwnerReference{
			*metav1.NewControllerRef(user, schema.GroupVersionKind{
				Group:   userv1alpha1.SchemeGroupVersion.Group,
				Version: userv1alpha1.SchemeGroupVersion.Version,
				Kind:    "User",
			}),
		}

		principal, err := c.userclientset.CubeV1alpha1().Principals(user.Namespace).Create(principalLogic)
		if err != nil && !k8serrors.IsAlreadyExists(err) {
			return nil, err
		}
		return principal, nil
	}

	return nil, err
}

// updatePrincipal will update principal associate to user
func (c *UserController) updatePrincipal(user *userv1alpha1.User, principalLogic *userv1alpha1.Principal) (*userv1alpha1.Principal, error) {
	principalLogic.OwnerReferences = []metav1.OwnerReference{
		*metav1.NewControllerRef(user, schema.GroupVersionKind{
			Group:   userv1alpha1.SchemeGroupVersion.Group,
			Version: userv1alpha1.SchemeGroupVersion.Version,
			Kind:    "User",
		}),
	}
	principalLogic.Namespace = user.Namespace
	principal, err := c.userclientset.CubeV1alpha1().Principals(user.Namespace).Update(principalLogic)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return nil, err
	}

	return principal, nil
}

func (c *UserController) ensureNamespaceExists() error {
	_, err := c.clientset.CoreV1().Namespaces().Create(&corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: UserNamespace,
		},
	})
	return err
}

// generateClusterRole is used to generate cluster scope role for user
// when add user, impersonate user need to update this workload
// temporary only support all resources and verbs
// TODO: it needs to be judged by the user role
func (c *UserController) generateClusterRole(role string, user *userv1alpha1.User) (*rbacv1.ClusterRole, error) {
	_, err := c.clientset.RbacV1().ClusterRoles().Get(user.Name, util.GetOptions)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		cubeAdmin := &rbacv1.ClusterRole{
			ObjectMeta: metav1.ObjectMeta{
				Name: user.Name,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(user, schema.GroupVersionKind{
						Group:   userv1alpha1.SchemeGroupVersion.Group,
						Version: userv1alpha1.SchemeGroupVersion.Version,
						Kind:    "User",
					}),
				},
			},
			Rules: []rbacv1.PolicyRule{
				{
					APIGroups:     []string{""},
					Resources:     []string{"users"},
					Verbs:         []string{"impersonate"},
					ResourceNames: []string{user.Name},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"groups"},
					Verbs:         []string{"impersonate"},
					ResourceNames: []string{"admin", "edit", "view"},
				},
				{
					APIGroups:     []string{"authentication.k8s.io"},
					Resources:     []string{"userextras/scopes"},
					Verbs:         []string{"impersonate"},
					ResourceNames: []string{""},
				},
				{
					APIGroups:     []string{""},
					Resources:     []string{"serviceaccounts"},
					Verbs:         []string{"impersonate"},
					ResourceNames: []string{""},
				},
			},
		}
		if role == "admin" {
			cubeAdmin.Rules = append(cubeAdmin.Rules, rbacv1.PolicyRule{
				APIGroups: []string{""},
				Resources: util.AdminResources,
				Verbs:     util.AdminVerbs,
			})
		}
		clusterRole, err := c.clientset.RbacV1().ClusterRoles().Create(cubeAdmin)
		if err == nil || k8serrors.IsAlreadyExists(err) {
			return clusterRole, nil
		}
	}

	return nil, err
}

// generateClusterRoleBinding is used to generate cluster scope roleBinding for user
// when add user, impersonate user need to update this workload
func (c *UserController) generateClusterRoleBinding(user *userv1alpha1.User) (*rbacv1.ClusterRoleBinding, error) {
	_, err := c.clientset.RbacV1().ClusterRoleBindings().Get(user.Name, util.GetOptions)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		cubeAdmin := &rbacv1.ClusterRoleBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name: user.Name,
				OwnerReferences: []metav1.OwnerReference{
					*metav1.NewControllerRef(user, schema.GroupVersionKind{
						Group:   userv1alpha1.SchemeGroupVersion.Group,
						Version: userv1alpha1.SchemeGroupVersion.Version,
						Kind:    "User",
					}),
				},
			},
			RoleRef: rbacv1.RoleRef{
				Kind:     "ClusterRole",
				APIGroup: "rbac.authorization.k8s.io",
				Name:     user.Name,
			},
			Subjects: []rbacv1.Subject{
				{
					Kind:     "User",
					APIGroup: "rbac.authorization.k8s.io",
					Name:     user.Name,
				},
			},
		}

		clusterRoleBinding, err := c.clientset.RbacV1().ClusterRoleBindings().Create(cubeAdmin)
		if err == nil || k8serrors.IsAlreadyExists(err) {
			return clusterRoleBinding, nil
		}
	}
	return nil, err
}
