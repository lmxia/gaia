package frontend

import (
	"fmt"
	"reflect"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	vhostClient "github.com/SUMMERLm/vhost/pkg/generated/clientset/versioned"
	vhostinformers "github.com/SUMMERLm/vhost/pkg/generated/informers/externalversions"
	appsV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	applisters "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
)

// Controller is a controller for frontend.
// It is local cluster controller
type Controller struct {
	workqueue          workqueue.RateLimitingInterface
	dynamicClient      dynamic.Interface
	gaiaClient         gaiaClientSet.Interface
	frontendLister     applisters.FrontendLister
	cdnSupplierLister  applisters.CdnSupplierLister
	frontendListSynced cache.InformerSynced
	vhostClient        vhostClient.Interface
}

func NewController(gaiaClient gaiaClientSet.Interface, gaiaInformerFactory gaiaInformers.SharedInformerFactory, vhostClient vhostClient.Interface, vhostInformerFactory vhostinformers.SharedInformerFactory) (*Controller, error) {
	frontendInformer := gaiaInformerFactory.Apps().V1alpha1().Frontends()
	cdnSupplierInformer := gaiaInformerFactory.Apps().V1alpha1().CdnSuppliers()
	c := &Controller{
		workqueue:          workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "Frontend"),
		gaiaClient:         gaiaClient,
		vhostClient:        vhostClient,
		frontendLister:     frontendInformer.Lister(),
		cdnSupplierLister:  cdnSupplierInformer.Lister(),
		frontendListSynced: frontendInformer.Informer().HasSynced,
	}
	// frontend events handler
	frontendInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addFrontend,
		UpdateFunc: c.updateFrontend,
		DeleteFunc: c.enqueueForDelete,
	})
	return c, nil
}

// Run starts the main goroutine responsible for watching and syncing Frontends.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Infof("Starting frontend cdn accelerate controller ...")
	defer klog.Infof("Shutting down frontend cdn accelerate controller")

	if !cache.WaitForNamedCacheSync("Frontend", stopCh, c.frontendListSynced) {
		return
	}

	for i := 0; i < workers; i++ {
		go wait.Until(c.worker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Controller) processNextWorkItem() bool {
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
			utilruntime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// ClusterRegistrationRequest resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("successfully synced frontend accelerate %q", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to sync.
func (c *Controller) syncHandler(key string) error {
	// If an error occurs during handling, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}
	klog.V(4).Infof("start processing frontend accelerate %q", key)
	//Get the ClusterRegistrationRequest resource with this name
	frontend, err := c.frontendLister.Frontends(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
	}
	frontendDel := frontend.DeletionTimestamp.IsZero()
	if !frontendDel {
		err = c.cdnAccelerateRecycle(frontend)
		if err != nil {
			klog.Errorf("Failed to recycle cdn of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
		err = c.dnsAccelerateRecycle(frontend)
		if err != nil {
			klog.Errorf("Failed to recycle dns of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
		err = c.frontendManageRecycle(frontend)
		if err != nil {
			klog.Errorf("Failed to recycle vhost of 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
		return nil
	}

	//judge cdn  state
	cdnstate, cdnStatus, err := c.cdnAccelerateState(frontend)
	if err != nil {
		klog.Errorf("Failed to get  'Frontend state and status' %s, error == %v", frontend.Name, err)
		return err
	}
	klog.V(5).Infof("Cdn state is: %s  and cdn status is: %s ", cdnstate, cdnStatus)
	//domain accelerate manage
	switch cdnstate {
	case common.FrontendAliyunCdnNoExist:
		err = c.cdnAccelerateCreate(frontend, cdnStatus)
		if err != nil {
			klog.Errorf("Failed to create 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
	case common.FrontendAliyunCdnExist:
		err = c.cdnAccelerateUpdate(frontend, cdnStatus)
		if err != nil {
			klog.Errorf("Failed to update 'Frontend' %q, error == %v", frontend.Name, err)
			return err
		}
	}
	err = c.frontendManage(frontend)
	if err != nil {
		klog.Errorf("Failed to new 'Frontend vhost' %q, error == %v", frontend.Name, err)
		return err
	}
	return nil
}

// addFrontend re-queues the Frontend for next scheduled time if there is a
// change in spec.schedule otherwise it re-queues it now
func (c *Controller) addFrontend(obj interface{}) {
	frontend := obj.(*appsV1alpha1.Frontend)
	klog.V(4).Infof("adding Frontend %q", frontend)
	c.enqueue(frontend)
}

// updateFrontend re-queues the Frontend for next scheduled time if there is a
// change in spec otherwise it re-queues it now
func (c *Controller) updateFrontend(old, cur interface{}) {
	oldFrontend := old.(*appsV1alpha1.Frontend)
	newFrontend := cur.(*appsV1alpha1.Frontend)

	// Decide whether discovery has reported a spec change.
	if reflect.DeepEqual(oldFrontend.DeletionTimestamp, newFrontend.DeletionTimestamp) && reflect.DeepEqual(oldFrontend.Spec, newFrontend.Spec) {
		klog.V(4).Infof("no updates on the spec of Frontend %q, skipping syncing", oldFrontend.Name)
		return
	}

	klog.V(4).Infof("updating Frontend %q", oldFrontend.Name)
	c.enqueue(newFrontend)
}

// enqueue takes a Frontend resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Frontend.
func (c *Controller) enqueue(front *appsV1alpha1.Frontend) {
	key, err := cache.MetaNamespaceKeyFunc(front)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) enqueueForDelete(obj interface{}) {
	var key string
	var err error
	key, err = cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	//requeue
	c.workqueue.AddRateLimited(key)
}
