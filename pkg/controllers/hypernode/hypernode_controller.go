package hypernode

import (
	"context"
	"fmt"
	"reflect"
	"time"

	clusterv1alpha1 "github.com/lmxia/gaia/pkg/apis/cluster/v1alpha1"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	hyperLister "github.com/lmxia/gaia/pkg/generated/listers/cluster/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog/v2"
)

const (
	controllerAgentName = "hyperNode-controller"

	SuccessSynced         = "Synced"
	MessageResourceSynced = "hyperNodes synced successfully"
)

// Controller is the controller implementation for Hypernode resources
type Controller struct {
	kubeClient      kubernetes.Interface
	gaiaClusterName string

	localGaiaClient      gaiaClientSet.Interface
	localHypernodeLister hyperLister.HypernodeLister
	localHypernodeSynced cache.InformerSynced

	parentGaiaClient                gaiaClientSet.Interface
	parentHypernodeLister           hyperLister.HypernodeLister
	defaultHypernodeInformerFactory gaiaInformers.SharedInformerFactory
	parentHypernodeSynced           cache.InformerSynced

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

func NewController(kubeClient kubernetes.Interface, localGaiaClient gaiaClientSet.Interface,
	localGaiaInformerFactory gaiaInformers.SharedInformerFactory,
) *Controller {
	// Create event broadcaster
	klog.Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartStructuredLogging(0)
	eventBroadcaster.StartRecordingToSink(&v1core.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	c := &Controller{
		kubeClient:           kubeClient,
		localGaiaClient:      localGaiaClient,
		localHypernodeLister: localGaiaInformerFactory.Cluster().V1alpha1().Hypernodes().Lister(),
		localHypernodeSynced: localGaiaInformerFactory.Cluster().V1alpha1().Hypernodes().Informer().HasSynced,
		workqueue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(),
			"parent-hyper-node"),
		recorder: recorder,
	}

	return c
}

// Run will set up the event handlers for types we are interested in, as well
// as syncing informer caches and starting workers. It will block until stopCh
// is closed, at which point it will shut down the workqueue and wait for
// workers to finish processing their current work items.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	klog.Info("Starting Hypernode controller ...")
	defer klog.Info("shutting down Hypernode controller")

	c.defaultHypernodeInformerFactory.Start(stopCh)
	// Wait for the caches to be synced before starting workers
	if !cache.WaitForNamedCacheSync("hyper-node-controller", stopCh, c.localHypernodeSynced, c.parentHypernodeSynced) {
		return
	}

	// Launch workers to process Hypernode resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
}

func (c *Controller) addHypernode(obj interface{}) {
	hyperNode := obj.(*clusterv1alpha1.Hypernode)
	klog.V(5).Infof("adding Hypernode %q", klog.KObj(hyperNode))
	c.enqueue(hyperNode)
}

func (c *Controller) updateHypernode(old, cur interface{}) {
	oldHypernode := old.(*clusterv1alpha1.Hypernode)
	newHypernode := cur.(*clusterv1alpha1.Hypernode)
	// Decide whether discovery has reported a spec change.
	if reflect.DeepEqual(oldHypernode.Spec, newHypernode.Spec) {
		klog.Infof("no updates on the spec of Hypernode %q, skipping syncing", oldHypernode.Name)
	}

	klog.Infof("updating Hypernode %q", klog.KObj(oldHypernode))
	c.enqueue(newHypernode)
}

func (c *Controller) deleteHypernode(obj interface{}) {
	hyperNode, ok := obj.(*clusterv1alpha1.Hypernode)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		hyperNode, ok = tombstone.Obj.(*clusterv1alpha1.Hypernode)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a Hypernode %#v", obj))
			return
		}
	}
	klog.Infof("deleting Hypernode %q", klog.KObj(hyperNode))
	c.enqueue(hyperNode)
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
		// Hypernode resource to be synced.
		if err := c.sync(key); err != nil {
			// Put the item back on the workqueue to handle any transient errors.
			c.workqueue.AddRateLimited(key)
			return fmt.Errorf("error syncing '%s': %s, requeuing", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item, so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		klog.V(5).Infof("successfully synced Hypernode %q", key)
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
func (c *Controller) sync(key string) error {
	// If an error occurs during handling, we'll requeue the item, so we can
	// attempt processing again later. This could have been caused by a
	// temporary network failure, or any other transient reason.

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return err
	}

	klog.V(5).InfoS("Started syncing Hypernode.", "Hypernode", klog.KRef(namespace, name))
	// Get the ClusterRegistrationRequest resource with this name
	hyperNode, err := c.parentHypernodeLister.Hypernodes(namespace).Get(name)
	// The ClusterRegistrationRequest resource may no longer exist, in which case we stop processing.
	switch {
	case apierrors.IsNotFound(err):
		klog.V(2).Infof("Hypernode %q not found, may be it is deleted", key)
		return nil
	case err != nil:
		klog.Errorf("failed to get Hypernode from parent cluster, Hypernode==%q, error==%v", klog.KObj(hyperNode), err)
		return err
	}

	return c.syncHypernode(hyperNode)
}

// syncHandler compares the actual state with the desired, and attempts to
// converge the two. It then updates the Status block of the Hypernode resource
// with the current status of the resource.
func (c *Controller) syncHypernode(hyperNode *clusterv1alpha1.Hypernode) error {
	// create new or update
	parentHyperNodeCopy := hyperNode.DeepCopy()
	localHyperNode, err := c.localHypernodeLister.Hypernodes(hyperNode.Namespace).Get(hyperNode.Name)
	switch {
	case err == nil:
		// update local hyper-node
		localHyperNodeCopy := localHyperNode.DeepCopy()
		update := 0
		if len(parentHyperNodeCopy.Labels) != len(localHyperNodeCopy.Labels) {
			update = 1
		}
		if update == 0 {
			for key := range parentHyperNodeCopy.Labels {
				if parentHyperNodeCopy.Labels[key] != localHyperNodeCopy.Labels[key] {
					klog.InfoS("update hypernode label", "Hypernode", klog.KObj(localHyperNodeCopy),
						"key", key, "value", localHyperNodeCopy.Labels[key])
					update = 1
				}
			}
		}
		if parentHyperNodeCopy.Name != localHyperNodeCopy.Name ||
			parentHyperNodeCopy.Spec.MyAreaName != localHyperNodeCopy.Spec.MyAreaName ||
			parentHyperNodeCopy.Spec.NodeAreaType != localHyperNodeCopy.Spec.NodeAreaType ||
			parentHyperNodeCopy.Spec.SupervisorName != localHyperNodeCopy.Spec.SupervisorName || update == 1 {
			localHyperNodeCopy.Name = parentHyperNodeCopy.Name
			localHyperNodeCopy.Spec.MyAreaName = parentHyperNodeCopy.Spec.MyAreaName
			localHyperNodeCopy.Spec.NodeAreaType = parentHyperNodeCopy.Spec.NodeAreaType
			localHyperNodeCopy.Spec.SupervisorName = parentHyperNodeCopy.Spec.SupervisorName

			localHyperNodeCopy.Labels = parentHyperNodeCopy.Labels

			node, errUpdate := c.localGaiaClient.ClusterV1alpha1().Hypernodes(localHyperNode.Namespace).Update(
				context.TODO(), localHyperNodeCopy, metav1.UpdateOptions{})
			if errUpdate != nil {
				klog.Errorf("failed to update Hypernode, Hypernode==%q, error==%v",
					klog.KObj(parentHyperNodeCopy), err)
			}
			klog.InfoS("successfully update existed hyperNode", "Hypernode", klog.KObj(node),
				"MyAreaName", node.Spec.MyAreaName, "SupervisorName", node.Spec.SupervisorName,
				"LocalClusterName", c.gaiaClusterName)
			return errUpdate
		}
		return nil
	case apierrors.IsNotFound(err):
		// create new hyper-node to local
		if parentHyperNodeCopy.Spec.MyAreaName == c.gaiaClusterName ||
			parentHyperNodeCopy.Spec.SupervisorName == c.gaiaClusterName {
			parentHyperNodeCopy.Kind = ""
			parentHyperNodeCopy.APIVersion = ""
			parentHyperNodeCopy.UID = ""
			parentHyperNodeCopy.ResourceVersion = ""
			node, errCreate := c.localGaiaClient.ClusterV1alpha1().Hypernodes(parentHyperNodeCopy.Namespace).
				Create(context.TODO(), parentHyperNodeCopy, metav1.CreateOptions{})
			if errCreate != nil {
				klog.Errorf("failed to create Hypernode, Hypernode==%q, error==%v", klog.KObj(hyperNode), err)
			}
			klog.InfoS("successfully created new hyperNode", "Hypernode", klog.KObj(node),
				"MyAreaName", node.Spec.MyAreaName, "SupervisorName", node.Spec.SupervisorName,
				"LocalClusterName", c.gaiaClusterName)
			c.recorder.Event(hyperNode, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)
			return errCreate
		} else {
			klog.V(5).InfoS("this Hypernode not belong to this cluster", "Hypernode", klog.KObj(hyperNode),
				"MyAreaName", hyperNode.Spec.MyAreaName, "SupervisorName", hyperNode.Spec.SupervisorName,
				"LocalClusterName", c.gaiaClusterName)
		}
		return nil
	case err != nil:
		klog.Errorf("failed to get Hypernode, Hypernode==%q, error==%v", klog.KObj(hyperNode), err)
		return err
	}

	return nil
}

// enqueue takes a Hypernode resource and converts it into a namespace/name
// string which is then put onto the work queue. This method should *not* be
// passed resources of any type other than Hypernode.
func (c *Controller) enqueue(hyperNode *clusterv1alpha1.Hypernode) {
	key, err := cache.MetaNamespaceKeyFunc(hyperNode)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	c.workqueue.Add(key)
}

func (c *Controller) SetHypernodeController(selfClusterName *string, parentKubeConfig *rest.Config) error {
	if selfClusterName == nil {
		return fmt.Errorf("failed to set selfClusterName for Hypernode Controller")
	}
	c.gaiaClusterName = *selfClusterName
	parentGaiaClient, parentDefaultGaiaInformerFactory := c.SetParentClient(parentKubeConfig)
	if parentGaiaClient != nil {
		parentHypernodeInformer := parentDefaultGaiaInformerFactory.Cluster().V1alpha1().Hypernodes()
		c.parentGaiaClient = parentGaiaClient
		c.parentHypernodeLister = parentHypernodeInformer.Lister()
		c.parentHypernodeSynced = parentHypernodeInformer.Informer().HasSynced
		c.defaultHypernodeInformerFactory = parentDefaultGaiaInformerFactory

		_, err := parentHypernodeInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
			AddFunc:    c.addHypernode,
			UpdateFunc: c.updateHypernode,
			DeleteFunc: c.deleteHypernode,
		})
		if err != nil {
			return err
		}
		return nil
	}
	return fmt.Errorf("failed to set Hypernode Controller for parent cluster")
}

func (c *Controller) SetParentClient(parentKubeConfig *rest.Config) (*gaiaClientSet.Clientset,
	gaiaInformers.SharedInformerFactory,
) {
	if parentKubeConfig != nil {
		parentGaiaClient := gaiaClientSet.NewForConfigOrDie(parentKubeConfig)
		parentDefaultGaiaInformerFactory := gaiaInformers.NewSharedInformerFactoryWithOptions(parentGaiaClient,
			known.DefaultResync, gaiaInformers.WithNamespace(metav1.NamespaceDefault))

		return parentGaiaClient, parentDefaultGaiaInformerFactory
	}
	return nil, nil
}
