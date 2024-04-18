/*
Copyright 2021 The Clusternet Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package resourcebindingmerger

import (
	"context"
	"fmt"
	"reflect"
	"time"

	appv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	rbsInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions/apps/v1alpha1"
	rbsListers "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"

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
var controllerKind = appv1alpha1.SchemeGroupVersion.WithKind("ResourceBindingMerger")

type SyncHandlerFunc func(*appv1alpha1.ResourceBinding) error

// Controller is a controller that merge the ResourceBinding from children clusters
type Controller struct {
	clusternetClient gaiaClientSet.Interface

	rbsLister rbsListers.ResourceBindingLister
	rbsSynced cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	SyncHandler SyncHandlerFunc
}

// NewController creates and initializes a new ResourceBindingMergerController
func NewController(clusternetClient gaiaClientSet.Interface,
	rbsInformer rbsInformers.ResourceBindingInformer, syncHandler SyncHandlerFunc,
) (*Controller, error) {
	if syncHandler == nil {
		return nil, fmt.Errorf("syncHandler must be set")
	}

	c := &Controller{
		clusternetClient: clusternetClient,
		rbsLister:        rbsInformer.Lister(),
		rbsSynced:        rbsInformer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),
			"resource-bindings-to-merge"),
		SyncHandler: syncHandler,
	}

	// Manage the addition/update of cluster registration requests
	rbsInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addRB,
		UpdateFunc: c.updateRB,
		DeleteFunc: c.deleteRB,
	})

	return c, nil
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shutdown the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("starting resource-bindings-merger controller...")
	defer klog.Info("shutting down resource-bindings-merger controller")

	// Wait for the caches to be synced before starting workers
	if !cache.WaitForNamedCacheSync("resource-bindings-merger-controller", stopCh, c.rbsSynced) {
		return
	}

	klog.V(2).Infof("starting %d worker threads", workers)
	// Launch workers to process ResourceBinding resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) addRB(obj interface{}) {
	rb := obj.(*appv1alpha1.ResourceBinding)
	klog.V(4).Infof("adding ResourceBinding %q", klog.KObj(rb))
	c.enqueue(rb)
}

func (c *Controller) updateRB(old, cur interface{}) {
	oldRB := old.(*appv1alpha1.ResourceBinding)
	newRB := cur.(*appv1alpha1.ResourceBinding)

	// Decide whether discovery has reported a spec change.
	if reflect.DeepEqual(oldRB.Spec, newRB.Spec) {
		klog.V(4).Infof("no updates on the spec of ResourceBinding %q, skipping syncing", oldRB.Name)
		return
	}

	klog.V(4).Infof("updating ResourceBinding %q", klog.KObj(oldRB))
	c.enqueue(newRB)
}

func (c *Controller) deleteRB(obj interface{}) {
	rb, ok := obj.(*appv1alpha1.ResourceBinding)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		rb, ok = tombstone.Obj.(*appv1alpha1.ResourceBinding)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ResourceBinding %#v", obj))
			return
		}
	}
	klog.V(4).Infof("deleting ResourceBinding %q", klog.KObj(rb))
	c.enqueue(rb)
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
		// ResourceBinding resource to be synced.
		if err := c.syncHandler(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("successfully synced ResourceBinding %q", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ResourceBinding resource
// with the current status of the resource.
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

	klog.V(4).Infof("start processing ResourceBinding %q", key)
	// Get the ResourceBinding resource with this name
	rb, err := c.rbsLister.ResourceBindings(namespace).Get(name)
	// The ResourceBinding resource may no longer exist, in which case we stop processing.
	if errors.IsNotFound(err) {
		klog.V(2).Infof("ResourceBinding %q has been deleted", key)
		return nil
	}
	if err != nil {
		return err
	}

	if len(rb.Kind) == 0 {
		rb.Kind = controllerKind.Kind
	}
	if len(rb.APIVersion) == 0 {
		rb.APIVersion = controllerKind.Version
	}

	return c.SyncHandler(rb)
}

func (c *Controller) UpdateRBStatus(rb *appv1alpha1.ResourceBinding, status *appv1alpha1.ResourceBindingStatus) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		rb.Status = *status
		_, err := c.clusternetClient.AppsV1alpha1().ResourceBindings(rb.Namespace).UpdateStatus(context.TODO(),
			rb, metav1.UpdateOptions{})
		if err == nil {
			klog.V(4).Infof("successfully update status of ResourceBinding %q to %q", rb.Name, status.Status)
			return nil
		}

		if updated, err2 := c.rbsLister.ResourceBindings(rb.Namespace).Get(rb.Name); err2 == nil {
			// make a copy so we don't mutate the shared cache
			rb = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated ResourceBinding %q from lister: %v", rb.Name, err2))
		}
		return err
	})
}

// enqueue takes a ResourceBinding resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ResourceBinding.
func (c *Controller) enqueue(rb *appv1alpha1.ResourceBinding) {
	key, err := cache.MetaNamespaceKeyFunc(rb)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}
