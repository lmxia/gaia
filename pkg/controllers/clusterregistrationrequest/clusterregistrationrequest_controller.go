// copied and modified from clusternet

package clusterregistrationrequest

import (
	"context"
	"fmt"
	"reflect"
	"time"

	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	crrsInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions/platform/v1alpha1"
	crrsListers "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/retry"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = clusterapi.SchemeGroupVersion.WithKind("ClusterRegistrationRequest")

type SyncHandlerFunc func(*clusterapi.ClusterRegistrationRequest) error

// Controller is a controller that handle edge cluster registration requests
type Controller struct {
	gaiaClient gaiaClientSet.Interface

	crrsLister crrsListers.ClusterRegistrationRequestLister
	crrsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.TypedRateLimitingInterface[string]

	SyncHandler SyncHandlerFunc
}

// NewController creates and initializes a new Controller
func NewController(gaiaClient gaiaClientSet.Interface,
	crrsInformer crrsInformers.ClusterRegistrationRequestInformer, syncHandler SyncHandlerFunc,
) (*Controller, error) {
	if syncHandler == nil {
		return nil, fmt.Errorf("syncHandler must be set")
	}

	c := &Controller{
		gaiaClient: gaiaClient,
		crrsLister: crrsInformer.Lister(),
		crrsSynced: crrsInformer.Informer().HasSynced,
		workqueue: workqueue.NewTypedRateLimitingQueueWithConfig(
			workqueue.DefaultTypedControllerRateLimiter[string](),
			workqueue.TypedRateLimitingQueueConfig[string]{Name: "cluster-registration-requests"},
		),
		SyncHandler: syncHandler,
	}

	// Manage the addition/update of cluster registration requests
	_, err := crrsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addCRR,
		UpdateFunc: c.updateCRR,
		DeleteFunc: c.deleteCRR,
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("starting cluster-registration-requests controller...")
	defer klog.Info("shutting down cluster-registration-requests controller")

	// Wait for the caches to be synced before starting workers
	if !cache.WaitForNamedCacheSync("cluster-registration-request-controller", stopCh, c.crrsSynced) {
		return
	}

	klog.V(2).Infof("starting %d worker threads", workers)
	// Launch workers to process ClusterRegistrationRequest resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) addCRR(obj interface{}) {
	crr := obj.(*clusterapi.ClusterRegistrationRequest)
	klog.V(4).Infof("adding ClusterRegistrationRequest %q", klog.KObj(crr))
	c.enqueue(crr)
}

func (c *Controller) updateCRR(old, cur interface{}) {
	oldCrr := old.(*clusterapi.ClusterRegistrationRequest)
	newCrr := cur.(*clusterapi.ClusterRegistrationRequest)

	// Decide whether discovery has reported a spec change.
	if reflect.DeepEqual(oldCrr.Spec, newCrr.Spec) {
		klog.V(4).Infof("no updates on the spec of ClusterRegistrationRequest %q, skipping syncing", oldCrr.Name)
		return
	}

	klog.V(4).Infof("updating ClusterRegistrationRequest %q", klog.KObj(oldCrr))
	c.enqueue(newCrr)
}

func (c *Controller) deleteCRR(obj interface{}) {
	crr, ok := obj.(*clusterapi.ClusterRegistrationRequest)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		crr, ok = tombstone.Obj.(*clusterapi.ClusterRegistrationRequest)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ClusterRegistrationRequest %#v", obj))
			return
		}
	}
	klog.V(4).Infof("deleting ClusterRegistrationRequest %q", klog.KObj(crr))
	c.enqueue(crr)
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj string) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		// Run the syncHandler, passing it the namespace/name string of the
		// ClusterRegistrationRequest resource to be synced.
		if err := c.syncHandler(obj); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(obj)
			return fmt.Errorf("error syncing '%s': %s, requeuing", obj, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("successfully synced ClusterRegistrationRequest %q", obj)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ClusterRegistrationRequest resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// If an error occurs during handling, we'll requeue the item so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.

	// Convert the namespace/name string into a distinct namespace and name
	_, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	klog.V(4).Infof("start processing ClusterRegistrationRequest %q", key)
	// Get the ClusterRegistrationRequest resource with this name
	crr, err := c.crrsLister.Get(name)
	// The ClusterRegistrationRequest resource may no longer exist, in which case we stop processing.
	if errors.IsNotFound(err) {
		klog.V(2).Infof("ClusterRegistrationRequest %q has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	if len(crr.Kind) == 0 {
		crr.Kind = controllerKind.Kind
	}
	if len(crr.APIVersion) == 0 {
		crr.APIVersion = controllerKind.Version
	}

	return c.SyncHandler(crr)
}

func (c *Controller) UpdateCRRStatus(crr *clusterapi.ClusterRegistrationRequest,
	status *clusterapi.ClusterRegistrationRequestStatus,
) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance

	klog.Infof("try to update ClusterRegistrationRequest %q status", crr.Name)
	if status.Result != nil && *status.Result != clusterapi.RequestApproved {
		if status.ErrorMessage == "" {
			return fmt.Errorf("for non approved requests, must set an error message")
		}

		// remove sensitive data if the request is not approved
		status.DedicatedToken = nil
		status.ManagedClusterName = ""
		status.DedicatedNamespace = ""
		status.CACertificate = nil
	}

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		crr.Status = *status
		_, err := c.gaiaClient.PlatformV1alpha1().ClusterRegistrationRequests().UpdateStatus(context.TODO(),
			crr, metav1.UpdateOptions{})
		if err == nil {
			klog.V(4).Infof("successfully update status of ClusterRegistrationRequest %q to %q",
				crr.Name, *status.Result)
			return nil
		}

		if updated, crrErr := c.crrsLister.Get(crr.Name); crrErr == nil {
			// make a copy so we don't mutate the shared cache
			crr = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated ClusterRegistrationRequest %q"+
				" from lister: %v", crr.Name, crrErr))
		}
		return err
	})
}

// enqueue takes a ClusterRegistrationRequest resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ClusterRegistrationRequest.
func (c *Controller) enqueue(crr *clusterapi.ClusterRegistrationRequest) {
	key, err := cache.MetaNamespaceKeyFunc(crr)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
