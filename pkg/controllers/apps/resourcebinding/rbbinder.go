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

package resourcebinding

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"

	appsV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/apps/description"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	informers "github.com/lmxia/gaia/pkg/generated/informers/externalversions/apps/v1alpha1"
	appListerV1alpha1 "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/algorithm"
	"github.com/lmxia/gaia/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	utilErrors "k8s.io/apimachinery/pkg/util/errors"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	kubeInformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var deletePropagationBackground = metav1.DeletePropagationBackground

// controllerKind contains the schema.GroupVersionKind for this controller type.
var controllerKind = appsV1alpha1.SchemeGroupVersion.WithKind("ResourceBinding")

type SyncHandlerFunc func(binding *appsV1alpha1.ResourceBinding) error

// Binder defines configuration for ResourceBinding requests
type Binder struct {
	networkBindURL string
	promURLPrefix  string

	localGaiaInformerFactory gaiaInformers.SharedInformerFactory
	// localReservedInformerFactory gaiaInformers.SharedInformerFactory
	localKubeClient     *kubernetes.Clientset
	localNodeLister     corev1lister.NodeLister
	localNodeSynced     cache.InformerSynced
	localDescLister     appListerV1alpha1.DescriptionLister
	localDescSynced     cache.InformerSynced
	localRBsLister      appListerV1alpha1.ResourceBindingLister
	localRBsSynced      cache.InformerSynced
	localFrontLister    appListerV1alpha1.FrontendLister
	localFrontSynced    cache.InformerSynced
	localGaiaClient     *gaiaClientSet.Clientset
	localDynamicClient  dynamic.Interface
	localDescController *description.Controller
	localRBController   *Controller

	parentMergedGaiaInformerFactory    gaiaInformers.SharedInformerFactory
	parentDedicatedGaiaInformerFactory gaiaInformers.SharedInformerFactory
	parentDynamicClient                dynamic.Interface
	parentGaiaClient                   *gaiaClientSet.Clientset
	parentDescController               *description.Controller
	parentRBController                 *Controller

	restMapper *restmapper.DeferredDiscoveryRESTMapper
}

// Controller is a controller that handle ResourceBinding requests
type Controller struct {
	// local gaia-push-reserved namespace
	GaiaClient gaiaClientSet.Interface
	Lister     appListerV1alpha1.ResourceBindingLister
	Synced     cache.InformerSynced

	// workqueue is a rate limited work queue. This is used to queue work to be
	// processed instead of performing it as soon as a change happens. This
	// means we can ensure we only process a fixed amount of resources at a
	// time, and makes it easy to ensure we are never processing the same item
	// simultaneously in two different workers.
	workqueue workqueue.RateLimitingInterface

	SyncHandler SyncHandlerFunc
}

// NewBinder returns a new Controller for ResourceBindings and Descriptions.
func NewBinder(localKubeClient *kubernetes.Clientset, localGaiaClient *gaiaClientSet.Clientset,
	kubeInformerFactory kubeInformers.SharedInformerFactory, localGaiaInformerFactory gaiaInformers.SharedInformerFactory,
	localKubeConfig *rest.Config, networkBindURL, prometheusMonitorURLPrefix string,
) (*Binder, error) {
	localDynamicClient, err := dynamic.NewForConfig(localKubeConfig)

	rbController := &Binder{
		networkBindURL:           networkBindURL,
		promURLPrefix:            prometheusMonitorURLPrefix,
		localKubeClient:          localKubeClient,
		localNodeLister:          kubeInformerFactory.Core().V1().Nodes().Lister(),
		localNodeSynced:          kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		localGaiaClient:          localGaiaClient,
		localDynamicClient:       localDynamicClient,
		localGaiaInformerFactory: localGaiaInformerFactory,
		localDescLister:          localGaiaInformerFactory.Apps().V1alpha1().Descriptions().Lister(),
		localDescSynced:          localGaiaInformerFactory.Apps().V1alpha1().Descriptions().Informer().HasSynced,
		localRBsLister:           localGaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Lister(),
		localRBsSynced:           localGaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Informer().HasSynced,
		localFrontLister:         localGaiaInformerFactory.Apps().V1alpha1().Frontends().Lister(),
		localFrontSynced:         localGaiaInformerFactory.Apps().V1alpha1().Frontends().Informer().HasSynced,
		restMapper: restmapper.NewDeferredDiscoveryRESTMapper(
			cacheddiscovery.NewMemCacheClient(localKubeClient.Discovery())),
	}
	if err != nil {
		return nil, err
	}

	return rbController, nil
}

// NewController return resource-binding Controller
func NewController(gaiaClient gaiaClientSet.Interface, informer informers.ResourceBindingInformer,
	syncHandler SyncHandlerFunc,
) (*Controller, error) {
	if syncHandler == nil {
		return nil, fmt.Errorf("syncHandler must be set")
	}

	c := &Controller{
		GaiaClient: gaiaClient,
		Lister:     informer.Lister(),
		Synced:     informer.Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(
			workqueue.NewItemExponentialFailureRateLimiter(time.Second, 100*time.Second),
			"resource-binding-requests"),
		SyncHandler: syncHandler,
	}

	// Manage the addition/update of self cluster's ResourceBinding requests in gaia-push-reserved namespace
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.addRB,
		UpdateFunc: c.updateRB,
		DeleteFunc: c.deleteRB,
	})

	return c, nil
}

func (c *Controller) addRB(obj interface{}) {
	rb := obj.(*appsV1alpha1.ResourceBinding)
	klog.V(4).Infof("adding resource-binding %q", klog.KObj(rb))
	c.enqueue(rb)
}

func (c *Controller) updateRB(old, cur interface{}) {
	oldRB := old.(*appsV1alpha1.ResourceBinding)
	newRB := cur.(*appsV1alpha1.ResourceBinding)

	if newRB.DeletionTimestamp != nil {
		c.enqueue(newRB)
		return
	}
	// Decide whether discovery has reported a spec change.
	if reflect.DeepEqual(oldRB.Spec, newRB.Spec) {
		klog.V(4).Infof("no updates on the spec of resource-binding %q, skipping syncing", oldRB.Name)
		return
	}

	klog.V(4).Infof("updating resource-binding %q, re-enqueuing", klog.KObj(oldRB))
	c.enqueue(newRB)
}

func (c *Controller) deleteRB(obj interface{}) {
	binding, ok := obj.(*appsV1alpha1.ResourceBinding)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		binding, ok = tombstone.Obj.(*appsV1alpha1.ResourceBinding)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a ResourceBinding %#v", obj))
			return
		}
	}
	klog.V(4).Infof("deleting ResourceBinding %q", klog.KObj(binding))
	c.enqueue(binding)
}

// enqueue takes a ResourceBinding resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ResourceBinding.
func (c *Controller) enqueue(rb *appsV1alpha1.ResourceBinding) {
	key, err := cache.MetaNamespaceKeyFunc(rb)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

// UpdateResourceBindingStatus enqueue takes a ResourceBinding resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than ResourceBinding.
func (c *Controller) UpdateResourceBindingStatus(rb *appsV1alpha1.ResourceBinding,
	status *appsV1alpha1.ResourceBindingStatus,
) error {
	// NEVER modify objects from the store. It's a read-only, local cache.
	// You can use DeepCopy() to make a deep copy of original object and modify this copy
	// Or create a copy manually for better performance

	klog.V(5).Infof("try to update ResourceBinding %q status", rb.Name)

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		rb.Status = *status
		_, err := c.GaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).UpdateStatus(
			context.TODO(), rb, metav1.UpdateOptions{})
		if err == nil {
			// TODO
			return nil
		}

		updated, err := c.Lister.ResourceBindings(rb.Namespace).Get(rb.Name)
		if err == nil {
			// make a copy, so we don't mutate the shared cache
			rb = updated.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated ResourceBinding %q from lister: %v", rb.Name, err))
		}
		return err
	})
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the ResourceBinding resource
// with the current status of the resource.
func (c *Controller) syncHandler(key string) error {
	// If an error occurs during handling, we'll requeue the item, so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	// Get the ResourceBinding resource with this name
	binding, err := c.Lister.ResourceBindings(namespace).Get(name)
	// The ResourceBinding resource may no longer exist, in which case we stop processing.
	if err != nil {
		// The Foo resource may no longer exist, in which case we stop
		// processing.
		if apierrors.IsNotFound(err) {
			klog.Errorf("resourcebinding '%s' in work queue no longer exists", key)
			utilruntime.HandleError(err)
			return nil
		}
		return err
	}

	if len(binding.Kind) == 0 {
		binding.Kind = controllerKind.Kind
	}
	if len(binding.APIVersion) == 0 {
		binding.APIVersion = controllerKind.Version
	}

	return c.SyncHandler(binding)
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

	// We wrap this block in a func, so we can defer c.workqueue.Done.
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
		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.Infof("successfully synced resourcebinding %q", key)
		return nil
	}(obj)
	if err != nil {
		utilruntime.HandleError(err)
		return true
	}

	return true
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	klog.Info("starting resourceBinding-requests controller...")
	defer klog.Info("shutting down resourceBinding-requests controller")

	// Wait for the caches to be synced before starting workers
	if !cache.WaitForNamedCacheSync("cluster-ResourceBinding-request-controller", stopCh, c.Synced) {
		return
	}

	klog.V(2).Infof("starting %d worker threads", workers)
	// Launch workers to process ResourceBinding resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
}

func (c *Binder) RunParentBinder(workers int, stopCh <-chan struct{}) {
	klog.Info("starting parent binder...")
	defer klog.Info("shutting parent binder...")
	c.parentMergedGaiaInformerFactory.Start(stopCh)
	c.parentDedicatedGaiaInformerFactory.Start(stopCh)
	if !cache.WaitForNamedCacheSync("parentResourceBinding-controller", stopCh,
		c.localDescSynced, c.localRBsSynced, c.localNodeSynced) {
		return
	}

	go c.parentRBController.Run(workers, stopCh)
	go c.parentDescController.Run(workers, stopCh)

	<-stopCh
}

func (c *Binder) RunLocalBinder(workers int, stopCh <-chan struct{}) {
	klog.Info("starting local binder... ")
	defer klog.Info("shutting local binder...")
	c.localGaiaInformerFactory.Start(stopCh)
	if !cache.WaitForNamedCacheSync("local-Description-Controller", stopCh,
		c.localDescSynced, c.localRBsSynced, c.localNodeSynced, c.localFrontSynced) {
		return
	}

	go c.localDescController.Run(workers, stopCh)
	go c.localRBController.Run(workers, stopCh)

	<-stopCh
}

// handle description in gaia-reserved and gaia-push-reserved namespace
func (c *Binder) handleLocalDescription(desc *appsV1alpha1.Description) error {
	klog.V(5).Infof("handleLocalDescription: handle local description %q ...", klog.KObj(desc))
	// delete gaia-reserved description
	if desc.DeletionTimestamp != nil {
		if desc.Namespace == common.GaiaPushReservedNamespace {
			return c.removeDescFinalizer(desc)
		}
		if desc.Namespace == common.GaiaReservedNamespace {
			var allErrs []error
			descName := desc.GetName()
			descNS := desc.GetNamespace()
			descUID := desc.GetUID()
			errCh := make(chan error, 3)
			wg := sync.WaitGroup{}
			wg.Add(3)
			go func(name, namespace string, uid types.UID) {
				defer wg.Done()
				err := c.offloadLocalResourceBindingsByDescription(name, namespace, uid)
				if err != nil {
					errCh <- err
				}
			}(descName, descNS, descUID)
			go func(name, namespace string) {
				defer wg.Done()
				err := c.offloadLocalNetworkRequirement(name, namespace)
				if err != nil {
					errCh <- err
				}
			}(descName, descNS)
			go func(name, namespace string, uid types.UID) {
				defer wg.Done()
				err := c.offloadLocalDescriptions(name, namespace, string(uid))
				if err != nil {
					errCh <- err
				}
			}(descName, descNS, descUID)

			wg.Wait()
			close(errCh)

			for err := range errCh {
				allErrs = append(allErrs, err)
			}
			if len(allErrs) > 0 {
				err := utilErrors.NewAggregate(allErrs)
				return err
			}
		}

		return c.removeDescFinalizer(desc)
	}

	return nil
}

func (c *Binder) removeDescFinalizer(desc *appsV1alpha1.Description) error {
	descCopy := desc.DeepCopy()
	descCopy.Finalizers = utils.RemoveString(descCopy.Finalizers, common.AppFinalizer)
	_, err := c.localGaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).Update(context.TODO(), descCopy,
		metav1.UpdateOptions{})
	if err != nil {
		klog.WarningDepth(2, fmt.Sprintf("handleLocalDescription: failed to remove finalizer "+
			"%s from Descriptions %s: %v", common.AppFinalizer, klog.KObj(desc), err))
	}
	return err
}

func (c *Binder) handleParentDescription(desc *appsV1alpha1.Description) error {
	klog.V(5).Infof("handleParentDescription: handle Description %q ...", klog.KObj(desc))
	if desc.DeletionTimestamp != nil {
		descLabels := desc.GetLabels()
		descName := descLabels[common.OriginDescriptionNameLabel]
		descNS := descLabels[common.OriginDescriptionNamespaceLabel]
		descUID := descLabels[common.OriginDescriptionUIDLabel]
		// delete local descriptions
		if err := c.offloadLocalDescriptions(descName, descNS, descUID); err != nil {
			return err
		}

		descCopy := desc.DeepCopy()
		descCopy.Finalizers = utils.RemoveString(descCopy.Finalizers, common.AppFinalizer)
		_, err := c.parentGaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).Update(context.TODO(), descCopy,
			metav1.UpdateOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				return nil
			}
			klog.WarningDepth(4, fmt.Sprintf("handleParentDescription: failed to remove finalizer "+
				"%s from parent Descriptions %s: %v", common.AppFinalizer, klog.KObj(desc), err))
		}
		return err
	}

	return nil
}

func (c *Binder) handleLocalResourceBinding(rb *appsV1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handleLocalResourceBinding: handle resourceBinding %q ...", klog.KObj(rb))
	if rb.Spec.StatusScheduler == appsV1alpha1.ResourceBindingSelected {
		var descNs string
		var err error
		if !utils.ContainsString(rb.Finalizers, common.AppFinalizer) && rb.DeletionTimestamp == nil {
			err = c.updateSelectedRB(rb, "localSelfCluster")
			if err != nil {
				return err
			}
		}
		if rb.Namespace == common.GaiaPushReservedNamespace {
			descNs = common.GaiaPushReservedNamespace
		} else {
			descNs = common.GaiaReservedNamespace
		}
		descriptionName := rb.GetLabels()[common.GaiaDescriptionLabel]

		if rb.DeletionTimestamp == nil {
			if rb.Namespace == common.GaiaRBMergedReservedNamespace && len(rb.Spec.FrontendRbs) != 0 {
				// deploy frontend app
				errFront := c.deployFrontendAPP(rb, descriptionName)
				if errFront != nil {
					return errFront
				}
			}
		} else {
			if rb.Namespace == common.GaiaPushReservedNamespace {
				err := utils.OffloadRBWorkloads(context.TODO(), c.localDynamicClient, c.restMapper, rb.GetLabels())
				if err != nil {
					return fmt.Errorf("fail to offload workloads of parent ResourceBinding %q: %v ",
						rb.Name, err)
				}

				rbCopy := rb.DeepCopy()
				rbCopy.Finalizers = utils.RemoveString(rbCopy.Finalizers, common.AppFinalizer)
				if _, err := c.localGaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).Update(context.TODO(),
					rbCopy, metav1.UpdateOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						return nil
					}
					klog.WarningDepth(4, fmt.Sprintf("failed to remove finalizer %s"+
						" from ResourceBinding %s: %v",
						common.AppFinalizer, klog.KObj(rb), err))
					return err
				}
			} else if len(rb.Spec.FrontendRbs) != 0 {
				descUID := rb.GetLabels()[common.OriginDescriptionUIDLabel]
				err := c.offloadLocalFrontEnd(descriptionName, descNs, descUID)
				if err != nil {
					return err
				}

				rb.Spec.FrontendRbs = nil
				rbCopy := rb.DeepCopy()
				if len(rbCopy.Spec.RbApps) == 0 {
					rbCopy.Finalizers = utils.RemoveString(rbCopy.Finalizers, common.AppFinalizer)
				}
				if _, err := c.localGaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).Update(context.TODO(),
					rbCopy, metav1.UpdateOptions{}); err != nil {
					if apierrors.IsNotFound(err) {
						return err
					}
					klog.WarningDepth(4, fmt.Sprintf("failed to remove finalizer %s"+
						" from ResourceBinding %s: %v",
						common.AppFinalizer, klog.KObj(rb), err))
					return err
				}
				klog.V(5).Infof("offload local frontend app of Description==%q", descriptionName)
			}
		}
	}
	return nil
}

func (c *Binder) handleParentResourceBinding(rb *appsV1alpha1.ResourceBinding) error {
	klog.V(5).Infof("handleParentResourceBinding: handle resourceBinding %q ...", klog.KObj(rb))
	if len(rb.Spec.RbApps) == 0 {
		return nil
	}
	if rb.Spec.StatusScheduler == appsV1alpha1.ResourceBindingSelected {
		var clusterName, descNs string
		var err error
		clusterName, descNs, err = utils.GetLocalClusterName(c.localKubeClient)
		if err != nil {
			klog.Errorf("handleParentResourceBinding: failed to get local clusterName From secret: %v", err)
		}
		if len(clusterName) == 0 {
			return fmt.Errorf("handleParentResourceBinding: local clusterName is nil")
		}
		descriptionName := rb.GetLabels()[common.GaiaDescriptionLabel]

		if rb.Labels[common.NetPlanLabel] == common.IPNetPlan {
			return c.handleParentRBIP(rb, descNs, descriptionName, clusterName)
		} else {
			return c.handleParentRBID(rb, descNs, descriptionName, clusterName)
		}
	}

	return nil
}
func (c *Binder) handleParentRBIP(rb *appsV1alpha1.ResourceBinding, descNs, descName, clusterName string) error {
	createRB := false
	if len(rb.Spec.RbApps) > 0 {
		for _, rba := range rb.Spec.RbApps {
			for _, rbb := range rba.Children {
				if len(rbb.Children) > 0 {
					createRB = true
					break
				}
			}
		}
	}

	if rb.DeletionTimestamp == nil {
		if createRB {
			nwr, newErr := utils.GetNetworkRequirement(context.TODO(), c.parentDynamicClient, c.restMapper, descName,
				common.GaiaReservedNamespace)
			if newErr != nil {
				klog.Infof("nwr error===%v \n", newErr)
				nwr = nil
			}
			return utils.ApplyResourceBinding(context.TODO(), c.localDynamicClient, c.restMapper, rb, clusterName,
				descName, c.networkBindURL, nwr)
		} else {
			desc, errDesc := c.parentGaiaClient.AppsV1alpha1().Descriptions(descNs).Get(context.TODO(), descName,
				metav1.GetOptions{})
			if errDesc != nil {
				klog.Errorf("handleParentResourceBinding: failed get parent Description %s/%s, error==%v",
					descNs, descName, errDesc)
				return errDesc
			}

			nodes, err := c.localNodeLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("handleParentResourceBinding: failed to list nodes, Description=%s/%s, error==%v",
					descNs, descName, err)
				return err
			}
			// format desc to components
			components, _, _ := utils.DescToComponents(desc)
			// need schedule across clusters
			err = utils.ApplyRBWorkloadsIP(context.TODO(), c.localGaiaClient, c.parentGaiaClient,
				c.localDynamicClient, rb, nodes, desc, components,
				c.restMapper, clusterName, c.promURLPrefix)
			if err != nil {
				return fmt.Errorf("handle ParentResourceBinding local apply workloads(components) failed")
			}
		}
	} else {
		if createRB {
			err := c.offloadLocalResourceBindingsByRB(rb)
			if err != nil {
				return fmt.Errorf("fail to offload local ResourceBinding by parent ResourceBinding %q: %v ",
					rb.Name, err)
			}
			if len(rb.Spec.NetworkPath) > 0 && len(c.networkBindURL) > 0 {
				klog.V(2).Infof("networkBindURL is %q", c.networkBindURL)
				utils.PostNetworkRequest(c.networkBindURL, descName, "delete", rb.Spec.NetworkPath[0])
			}
		} else {
			err := utils.OffloadRBWorkloads(context.TODO(), c.localDynamicClient, c.restMapper, rb.GetLabels())
			if err != nil {
				return fmt.Errorf("fail to offload workloads of parent ResourceBinding %q: %v ",
					rb.Name, err)
			}
		}

		// delete rb
		if rb.Namespace == common.GaiaRBMergedReservedNamespace {
			// remove RbApp of this cluster/field
			var newRbApps []*appsV1alpha1.ResourceBindingApps
			for _, rbApp := range rb.Spec.RbApps {
				if rbApp.ClusterName == clusterName {
					continue
				}
				newRbApps = append(newRbApps, rbApp)
			}
			rb.Spec.RbApps = newRbApps
			rbCopy := rb.DeepCopy()
			if len(rbCopy.Spec.RbApps) == 0 && len(rbCopy.Spec.FrontendRbs) == 0 {
				// remove finalizers
				rbCopy.Finalizers = utils.RemoveString(rbCopy.Finalizers, common.AppFinalizer)
			}
			if _, err := c.parentGaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).Update(
				context.TODO(), rbCopy, metav1.UpdateOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				klog.WarningDepth(4, fmt.Sprintf("failed to remove finalizer %s "+
					"from ResourceBinding %s: %v",
					common.AppFinalizer, klog.KObj(rb), err))
				return err
			}
		}
	}
	return nil
}
func (c *Binder) handleParentRBID(rb *appsV1alpha1.ResourceBinding, descNs, descName, clusterName string) error {
	createRB := false
	if len(rb.Spec.RbApps) > 0 {
		for _, rba := range rb.Spec.RbApps {
			if len(rba.Children) > 0 {
				createRB = true
				break
			}
		}
	}

	if rb.DeletionTimestamp == nil {
		if createRB {
			nwr, newErr := utils.GetNetworkRequirement(context.TODO(), c.parentDynamicClient, c.restMapper, descName,
				common.GaiaReservedNamespace)
			if newErr != nil {
				klog.Infof("nwr error===%v \n", newErr)
				nwr = nil
			}
			return utils.ApplyResourceBinding(context.TODO(), c.localDynamicClient, c.restMapper, rb, clusterName,
				descName, c.networkBindURL, nwr)
		} else {
			desc, errDesc := c.parentGaiaClient.AppsV1alpha1().Descriptions(descNs).Get(context.TODO(), descName,
				metav1.GetOptions{})
			if errDesc != nil {
				klog.Errorf("handleParentResourceBinding: failed get parent Description %s/%s, error==%v",
					descNs, descName, errDesc)
				return errDesc
			}

			nodes, err := c.localNodeLister.List(labels.Everything())
			if err != nil {
				klog.Errorf("handleParentResourceBinding: failed to list nodes, Description=%s/%s, error==%v",
					descNs, descName, err)
				return err
			}
			// format desc to components
			components, _, _ := utils.DescToComponents(desc)
			// need schedule across clusters
			err = utils.ApplyRBWorkloads(context.TODO(), c.localGaiaClient, c.parentGaiaClient,
				c.localDynamicClient, rb, nodes, desc, components,
				c.restMapper, clusterName, c.promURLPrefix)
			if err != nil {
				return fmt.Errorf("handle ParentResourceBinding local apply workloads(components) failed")
			}
		}
	} else {
		if createRB {
			err := c.offloadLocalResourceBindingsByRB(rb)
			if err != nil {
				return fmt.Errorf("fail to offload local ResourceBinding by parent ResourceBinding %q: %v ",
					rb.Name, err)
			}
			if len(rb.Spec.NetworkPath) > 0 && len(c.networkBindURL) > 0 {
				klog.V(2).Infof("networkBindURL is %q", c.networkBindURL)
				utils.PostNetworkRequest(c.networkBindURL, descName, "delete", rb.Spec.NetworkPath[0])
			}
		} else {
			err := utils.OffloadRBWorkloads(context.TODO(), c.localDynamicClient, c.restMapper, rb.GetLabels())
			if err != nil {
				return fmt.Errorf("fail to offload workloads of parent ResourceBinding %q: %v ",
					rb.Name, err)
			}
		}

		// delete rb
		if rb.Namespace == common.GaiaRBMergedReservedNamespace {
			// remove RbApp of this cluster/field
			var newRbApps []*appsV1alpha1.ResourceBindingApps
			for _, rbApp := range rb.Spec.RbApps {
				if rbApp.ClusterName == clusterName {
					continue
				}
				newRbApps = append(newRbApps, rbApp)
			}
			rb.Spec.RbApps = newRbApps
			rbCopy := rb.DeepCopy()
			if len(rbCopy.Spec.RbApps) == 0 && len(rbCopy.Spec.FrontendRbs) == 0 {
				// remove finalizers
				rbCopy.Finalizers = utils.RemoveString(rbCopy.Finalizers, common.AppFinalizer)
			}
			if _, err := c.parentGaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).Update(
				context.TODO(), rbCopy, metav1.UpdateOptions{}); err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}
				klog.WarningDepth(4, fmt.Sprintf("failed to remove finalizer %s "+
					"from ResourceBinding %s: %v",
					common.AppFinalizer, klog.KObj(rb), err))
				return err
			}
		}
	}
	return nil
}

func getRBCondition(clusterName string) metav1.Condition {
	return metav1.Condition{
		Type:               clusterName,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "null",
		Message:            "the number of rb.Spec.NetworkPath is not one",
	}
}

func updateRBStatusWithRetry(ctx context.Context, gaiaClient *gaiaClientSet.Clientset,
	rb *appsV1alpha1.ResourceBinding, condition metav1.Condition, clusterName string,
) error {
	rb = rb.DeepCopy()
	err := wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
		rb.Status.Conditions = append(rb.Status.Conditions, condition)
		if rb.Status.Clusters == nil {
			rb.Status.Clusters = make(map[string]appsV1alpha1.StatusRBDeploy)
		}
		if condition.Status == metav1.ConditionFalse {
			rb.Status.Clusters[clusterName] = appsV1alpha1.ResourceBindingRed
		} else {
			rb.Status.Clusters[clusterName] = appsV1alpha1.ResourceBindingGreen
		}
		for _, status := range rb.Status.Clusters {
			if status == appsV1alpha1.ResourceBindingRed {
				rb.Status.Status = appsV1alpha1.ResourceBindingRed
				break
			}
			rb.Status.Status = appsV1alpha1.ResourceBindingGreen
		}
		_, err := gaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).UpdateStatus(context.TODO(),
			rb, metav1.UpdateOptions{})
		// check if failed
		if err == nil {
			return true, nil
		}

		newRB, err2 := gaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).Get(context.TODO(),
			rb.Name, metav1.GetOptions{})
		if err2 == nil {
			rb = newRB.DeepCopy()
		}

		return false, nil
	})
	if err != nil {
		klog.Errorf("failed to update status of resourcebinding %q, err: %v",
			klog.KRef(rb.Namespace, rb.Name), err)
	}

	return err
}

func (c *Binder) SetParentBinderController(dedicatedNamespace *string) (*Binder, error) {
	// todo: 是否需要判断 parent nil
	parentGaiaClient, parentDynamicClient, parentMergedGaiaInformerFactory := utils.SetParentClient(c.localKubeClient,
		c.localGaiaClient)
	c.parentGaiaClient = parentGaiaClient
	c.parentDynamicClient = parentDynamicClient
	c.parentMergedGaiaInformerFactory = parentMergedGaiaInformerFactory
	rbController, rbErr := NewController(parentGaiaClient,
		parentMergedGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(), c.handleParentResourceBinding)
	if rbErr != nil {
		klog.Errorf("failed to set RBBindController, rbErr == %v", rbErr)
		return nil, rbErr
	}
	c.parentRBController = rbController

	// parentDescController
	if dedicatedNamespace != nil {
		dedicatedGaiaInformerFactory := gaiaInformers.NewSharedInformerFactoryWithOptions(parentGaiaClient,
			common.DefaultResync, gaiaInformers.WithNamespace(*dedicatedNamespace))
		descController, descErr := description.NewController(parentGaiaClient,
			dedicatedGaiaInformerFactory.Apps().V1alpha1().Descriptions(),
			c.localGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(), c.handleParentDescription)
		if descErr != nil {
			klog.Errorf(" local handleParentDescription SetRBBindController descErr == %v", descErr)
			return nil, descErr
		}
		c.parentDescController = descController
		c.parentDedicatedGaiaInformerFactory = dedicatedGaiaInformerFactory
	} else {
		return c, fmt.Errorf("SetRBBindController: dedicatedNamespace is nil")
	}

	return c, nil
}

func (c *Binder) SetLocalBinderController() (*Binder, error) {
	rbController, rbErr := NewController(c.localGaiaClient,
		c.localGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(), c.handleLocalResourceBinding)
	if rbErr != nil {
		klog.Errorf("failed to set RBBindController, rbErr == %v", rbErr)
		return nil, rbErr
	}
	c.localRBController = rbController

	// set local description controller
	descController, err := description.NewController(c.localGaiaClient,
		c.localGaiaInformerFactory.Apps().V1alpha1().Descriptions(),
		c.localGaiaInformerFactory.Apps().V1alpha1().ResourceBindings(), c.handleLocalDescription)
	if err != nil {
		klog.Errorf("handleLocalDescription: local SetLocalBinderController err == %v", err)
		return nil, err
	}
	c.localDescController = descController

	return c, nil
}

func (c *Binder) offloadLocalFrontEnd(descName, descNs, descUID string) error {
	klog.V(5).InfoS("Start deleting local Frontend ...", "Description", klog.KRef(descNs, descName))
	frontEnds, err := c.localFrontLister.Frontends(common.GaiaFrontendNamespace).List(labels.SelectorFromSet(labels.Set{
		common.OriginDescriptionNameLabel:      descName,
		common.OriginDescriptionNamespaceLabel: descNs,
		common.OriginDescriptionUIDLabel:       descUID,
	}))
	if err != nil {
		return err
	}
	var allErrs []error
	for _, frontEnd := range frontEnds {
		if frontEnd.DeletionTimestamp != nil {
			continue
		}
		if err = c.deleteFrontEnd(context.TODO(), klog.KObj(frontEnd).String()); err != nil {
			klog.ErrorDepth(5, err)
			allErrs = append(allErrs, err)
		}
	}
	if frontEnds != nil || len(allErrs) > 0 {
		return fmt.Errorf("waiting for local Frontends of Description %q getting deleted", klog.KRef(descNs, descName))
	}

	return nil
}

func (c *Binder) offloadLocalDescriptions(descName, descNS, uid string) error {
	klog.V(5).Infof("Start deleting local Descriptions derived by %s", klog.KRef(descNS, descName))
	var derivedDescs []*appsV1alpha1.Description
	var allErrs []error
	var err error
	if len(uid) != 0 {
		derivedDescs, err = c.localDescLister.List(labels.SelectorFromSet(labels.Set{
			common.OriginDescriptionNameLabel:      descName,
			common.OriginDescriptionNamespaceLabel: descNS,
			common.OriginDescriptionUIDLabel:       uid,
		}))
	} else {
		derivedDescs, err = c.localDescLister.List(labels.SelectorFromSet(labels.Set{
			common.OriginDescriptionNameLabel:      descName,
			common.OriginDescriptionNamespaceLabel: descNS,
		}))
	}
	if err != nil {
		return err
	}

	for _, deleteDesc := range derivedDescs {
		if deleteDesc.DeletionTimestamp != nil {
			continue
		}
		if err = c.deleteDescription(context.TODO(), klog.KObj(deleteDesc).String()); err != nil {
			klog.ErrorDepth(5, err)
			allErrs = append(allErrs, err)
			continue
		}
	}
	if derivedDescs != nil || len(allErrs) > 0 {
		return fmt.Errorf("waiting for local Descriptions belongs to Description %s getting deleted",
			klog.KRef(descNS, descName))
	}

	return nil
}

func (c *Binder) deleteDescription(ctx context.Context, namespacedKey string) error {
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(namespacedKey)
	if err != nil {
		return err
	}

	err = c.localGaiaClient.AppsV1alpha1().Descriptions(ns).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deletePropagationBackground,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Binder) offloadLocalResourceBindingsByDescription(descName, descNS string, uid types.UID) error {
	klog.V(5).InfoS("Start deleting local ResourceBindings ...", "Description",
		klog.KRef(descNS, descName))
	// offload gaia-merged && gaia-to-be-merged  Rbs
	derivedRBs, err := c.localRBsLister.List(labels.SelectorFromSet(labels.Set{
		common.OriginDescriptionNameLabel:      descName,
		common.OriginDescriptionNamespaceLabel: descNS,
		common.OriginDescriptionUIDLabel:       string(uid),
	}))
	if err != nil {
		return err
	}
	var allErrs []error
	for _, deleteRB := range derivedRBs {
		if deleteRB.DeletionTimestamp != nil {
			continue
		}

		if err = c.deleteResourceBinding(context.TODO(), klog.KObj(deleteRB).String()); err != nil {
			klog.ErrorDepth(5, err)
			allErrs = append(allErrs, err)
			continue
		}
	}
	// Wait for deletion to end
	if derivedRBs != nil || len(allErrs) > 0 {
		return fmt.Errorf("waiting for local ResourceBings of Description %q getting deleted",
			klog.KRef(descNS, descName))
	}

	return nil
}

func (c *Binder) deleteFrontEnd(ctx context.Context, namespacedKey string) error {
	ns, name, err := cache.SplitMetaNamespaceKey(namespacedKey)
	if err != nil {
		return err
	}

	err = c.localGaiaClient.AppsV1alpha1().Frontends(ns).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deletePropagationBackground,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Binder) deleteResourceBinding(ctx context.Context, namespacedKey string) error {
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(namespacedKey)
	if err != nil {
		return err
	}

	err = c.localGaiaClient.AppsV1alpha1().ResourceBindings(ns).Delete(ctx, name, metav1.DeleteOptions{
		PropagationPolicy: &deletePropagationBackground,
	})
	if err != nil && apierrors.IsNotFound(err) {
		return nil
	}
	return err
}

func (c *Binder) offloadLocalResourceBindingsByRB(rb *appsV1alpha1.ResourceBinding) error {
	// offload gaia-merged && gaia-to-be-merged  Rbs
	derivedRBs, err := c.localRBsLister.List(labels.SelectorFromSet(labels.Set{
		common.OriginDescriptionNameLabel:      rb.GetLabels()[common.OriginDescriptionNameLabel],
		common.OriginDescriptionNamespaceLabel: rb.GetLabels()[common.OriginDescriptionNamespaceLabel],
		common.OriginDescriptionUIDLabel:       rb.GetLabels()[common.OriginDescriptionUIDLabel],
	}))
	if err != nil {
		return err
	}
	var allErrs []error
	klog.Infof("Start deleting local ResourceBindings derived by parent ResourceBinding %s",
		klog.KObj(rb).String())
	for _, deleteRB := range derivedRBs {
		if deleteRB.DeletionTimestamp != nil {
			continue
		}
		if err = c.deleteResourceBinding(context.TODO(), klog.KObj(deleteRB).String()); err != nil {
			klog.ErrorDepth(5, err)
			allErrs = append(allErrs, err)
			continue
		}
	}
	if derivedRBs != nil || len(allErrs) > 0 {
		return fmt.Errorf("waiting for local ResourceBings belongs to parent ResourceBinding %s getting deleted",
			klog.KObj(rb))
	}

	return nil
}

func (c *Binder) offloadLocalNetworkRequirement(descName, descNS string) error {
	klog.V(5).InfoS("Start deleting local NetworkRequirement ...", "Description",
		klog.KRef(descNS, descName))
	nwr, err := c.localGaiaClient.AppsV1alpha1().NetworkRequirements(common.GaiaReservedNamespace).Get(context.TODO(),
		descName, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		}
		klog.InfoS("failed to get network requirement of Description", "NetworkRequirement",
			klog.KRef(descNS, descName))
		return err
	}

	errDel := c.localGaiaClient.AppsV1alpha1().NetworkRequirements(common.GaiaReservedNamespace).Delete(context.TODO(),
		nwr.GetName(), metav1.DeleteOptions{
			PropagationPolicy: &deletePropagationBackground,
		})
	if errDel != nil {
		klog.Warningf("failed to delete NetworkRequirements of Description %q, error: %v",
			klog.KRef(descNS, descName), err)
		return errDel
	}
	return nil
}

func (c *Binder) updateSelectedRB(rb *appsV1alpha1.ResourceBinding, clusterName string) error {
	if len(rb.Spec.NetworkPath) > 1 {
		klog.Errorf("the number of rb.Spec.NetworkPath is not one, ResourceBinding: %s", klog.KObj(rb))
		condition := getRBCondition(clusterName)
		err2 := updateRBStatusWithRetry(context.TODO(), c.localGaiaClient, rb, condition, clusterName)
		return err2
	}

	rbLabels := rb.GetLabels()
	rb.Finalizers = append(rb.Finalizers, common.AppFinalizer)
	rbLabels[common.StatusScheduler] = string(appsV1alpha1.ResourceBindingSelected)
	_, err := c.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).
		Update(context.TODO(), rb, metav1.UpdateOptions{})
	switch {
	case apierrors.IsConflict(err):
		klog.V(5).InfoS("Delete rbs not selected: update rb conflict for add label and inject finalizer",
			"ResourceBinding", klog.KObj(rb), "err", err)
		return fmt.Errorf("delete rbs not selected: update resourcebinding %q conflict "+
			"for add label and inject finalizer", klog.KObj(rb))
	case err != nil:
		klog.Warning("Delete rbs not selected: failed to inject finalizer %s "+
			"and update ResourceBinding %s: %v", common.AppFinalizer, klog.KObj(rb), err)
		return err
	}
	// delete unselected rbs
	descName := rbLabels[common.OriginDescriptionNameLabel]
	klog.V(4).Infof("Delete unselected ResourceBindings in namespace %q from Description %q.",
		common.GaiaRBMergedReservedNamespace, descName)
	err = c.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).
		DeleteCollection(context.TODO(), metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				common.OriginDescriptionNameLabel:      descName,
				common.OriginDescriptionNamespaceLabel: rbLabels[common.OriginDescriptionNamespaceLabel],
				common.OriginDescriptionUIDLabel:       rbLabels[common.OriginDescriptionUIDLabel],
				common.StatusScheduler:                 string(appsV1alpha1.ResourceBindingmerged),
			}).String(),
		})
	if err != nil {
		klog.Errorf("failed to delete merged rbs in parent cluster %q namespace, err: %v",
			common.GaiaRBMergedReservedNamespace, err)
		return err
	}
	return nil
}

func (c *Binder) deployFrontendAPP(rb *appsV1alpha1.ResourceBinding, descName string) error {
	if len(rb.Spec.FrontendRbs) != 0 {
		desc, errDesc := c.localGaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).
			Get(context.TODO(), descName, metav1.GetOptions{})
		if errDesc != nil {
			klog.Errorf("deploy FrontendApp: failed get parent Description %q, error==%v",
				klog.KRef(common.GaiaReservedNamespace, descName), errDesc)
			return errDesc
		}
		var allErrs []error
		for _, com := range desc.Spec.FrontendComponents {
			if len(com.Cdn) == 0 {
				return fmt.Errorf("failed to create frontend, FrontEndAPP.cdn is nil, Description==%q, FrontendAPP==%q",
					klog.KObj(desc), com.ComponentName)
			}

			suppliers := rb.Spec.FrontendRbs[0].Suppliers
			for _, sup := range suppliers {
				if sup.SupplierName == com.Cdn[0].Supplier && sup.Replicas[com.ComponentName] > 0 {
					label := algorithm.FillRBLabels(desc)
					label[common.GaiaDescriptionLabel] = desc.Name
					label[common.GaiaComponentLabel] = com.ComponentName
					frontEnd := &appsV1alpha1.Frontend{
						TypeMeta: metav1.TypeMeta{
							APIVersion: "apps.gaia.io/v1alpha1",
							Kind:       "Frontend",
						},
						ObjectMeta: metav1.ObjectMeta{
							Name:      com.FQDN,
							Namespace: common.GaiaFrontendNamespace,
							Labels:    label,
							Finalizers: []string{
								common.FrontendAliyunFinalizers,
							},
						},
						Spec: appsV1alpha1.FrontendSpec{
							DomainName:    com.DomainName,
							CdnAccelerate: com.CdnAccelerate,
							Cdn:           com.Cdn,
						},
					}

					_, err := c.localGaiaClient.AppsV1alpha1().Frontends(frontEnd.Namespace).Create(context.TODO(), frontEnd,
						metav1.CreateOptions{})
					if err != nil && !apierrors.IsAlreadyExists(err) {
						klog.ErrorS(err, "failed to create Frontend", "Frontend", klog.KObj(frontEnd),
							"Description", klog.KObj(desc))
						allErrs = append(allErrs, err)
					} else {
						klog.V(5).InfoS("successfully created Frontend", "Frontend", klog.KObj(frontEnd),
							"Description", klog.KObj(desc))
					}
				}
			}
		}
		if len(allErrs) != 0 {
			return utilErrors.NewAggregate(allErrs)
		}

		return nil
	}

	return fmt.Errorf("the components of frontendAPP is nil, Description==%q",
		klog.KRef(common.GaiaReservedNamespace, descName))
}
