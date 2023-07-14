package cronmaster

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/lmxia/gaia/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"

	appsV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	applisters "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	kubeInformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8slisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var cronKind = appsV1alpha1.SchemeGroupVersion.WithKind("CronMaster")

// Controller is a controller for cronMaster.
// It is local cluster controller
type Controller struct {
	queue workqueue.RateLimitingInterface
	// recorder record.EventRecorder

	dynamicClient dynamic.Interface
	gaiaClient    gaiaClientSet.Interface
	kubeClient    kubernetes.Interface
	restMapper    *restmapper.DeferredDiscoveryRESTMapper

	cronMasterList applisters.CronMasterLister
	// dLister can list/get deployments from the shared informer's store
	deployLister k8slisters.DeploymentLister

	cronMasterListSynced cache.InformerSynced
	deployListerSynced   cache.InformerSynced

	// now is a function that returns current time, done to facilitate unit tests
	now func() time.Time
}

func NewController(gaiaClient gaiaClientSet.Interface, kubeClient kubernetes.Interface, gaiaInformerFactory gaiaInformers.SharedInformerFactory, kubeInformerFactory kubeInformers.SharedInformerFactory, localKubeConfig *rest.Config) (*Controller, error) {

	localDynamicClient, err := dynamic.NewForConfig(localKubeConfig)
	if err != nil {
		return nil, fmt.Errorf("newCronMasterController: failed to get dynamicClient from local kubeconfig, ERROR: %v", err)
	}
	cronInformer := gaiaInformerFactory.Apps().V1alpha1().CronMasters()
	c := &Controller{
		queue:                workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cronmaster"),
		restMapper:           restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(kubeClient.Discovery())),
		dynamicClient:        localDynamicClient,
		gaiaClient:           gaiaClient,
		kubeClient:           kubeClient,
		cronMasterList:       cronInformer.Lister(),
		deployLister:         kubeInformerFactory.Apps().V1().Deployments().Lister(),
		cronMasterListSynced: cronInformer.Informer().HasSynced,
		deployListerSynced:   kubeInformerFactory.Apps().V1().Deployments().Informer().HasSynced,
		now:                  time.Now,
	}

	// cronMaster events handler
	cronInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			c.enqueueController(obj)
		},
		UpdateFunc: c.updateCronMaster,
		DeleteFunc: func(obj interface{}) {
			c.enqueueController(obj)
		},
	})

	return c, nil
}

// Run starts the main goroutine responsible for watching and syncing cronmasters.
func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer utilruntime.HandleCrash()
	defer c.queue.ShutDown()

	klog.Infof("Starting cronmaster controller ...")
	defer klog.Infof("Shutting down cronmaster controller")

	if !cache.WaitForNamedCacheSync("cronmaster", stopCh, c.deployListerSynced, c.cronMasterListSynced) {
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
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	isAlreadyHandled, requeueAfter, err := c.sync(key.(string))
	switch {
	case isAlreadyHandled == true:
		c.queue.Forget(key)
	case err != nil:
		utilruntime.HandleError(fmt.Errorf("error syncing CronMasterController %v, requeuing: %v", key.(string), err))
		c.queue.AddRateLimited(key)
	case requeueAfter != nil:
		c.queue.Forget(key)
		c.queue.AddAfter(key, *requeueAfter)
	}
	return true
}

func (c *Controller) sync(cronKey string) (bool, *time.Duration, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(cronKey)
	if err != nil {
		return false, nil, err
	}

	cron, err := c.cronMasterList.CronMasters(ns).Get(name)
	switch {
	case errors.IsNotFound(err):
		// may be cronmaster is deleted, do not need to requeue this key
		klog.V(4).InfoS("cronmaster not found, may be it is deleted", "cronmaster", klog.KRef(ns, name), "err", err)
		return false, nil, nil
	case err != nil:
		// for other transient apiserver error requeue with exponential backoff
		return false, nil, err
	}

	resourceToBeReconciled, err := c.getResourceToBeReconciled(cron)
	if err != nil {
		return false, nil, err
	}

	_, isAlreadyHandled, requeueAfter, err := c.syncCronMaster(cron, resourceToBeReconciled)
	if err != nil {
		klog.V(2).InfoS("error reconciling cronmaster", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "ERROR:", err)
		return isAlreadyHandled, nil, err
	}

	if requeueAfter != nil {
		klog.V(4).InfoS("re-queuing cronmaster", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "requeueAfter", requeueAfter)
		return isAlreadyHandled, requeueAfter, nil
	}
	// this marks the key done, currently only happens when the cronmaster is suspended or spec has invalid schedule format
	return isAlreadyHandled, nil, nil
}

// updateCronMaster re-queues the cronmaster for next scheduled time if there is a
// change in spec.schedule otherwise it re-queues it now
func (c *Controller) updateCronMaster(old interface{}, curr interface{}) {
	oldCron, okOld := old.(*appsV1alpha1.CronMaster)
	newCron, okNew := curr.(*appsV1alpha1.CronMaster)

	if !okOld || !okNew {
		// typecasting of one failed, handle this better, may be log entry
		return
	}
	// if the change in schedule results in next requeue having to be sooner than it already was,
	// it will be handled here by the queue. If the next requeue is further than previous schedule,
	// the sync loop will essentially be a no-op for the already queued key with old schedule.
	if !reflect.DeepEqual(oldCron.Spec.Schedule, newCron.Spec.Schedule) {
		// schedule changed, change the requeue time
		// get and update NextScheduleAction
		now := c.now()
		td, isStart, err := nextScheduledTimeDuration(newCron.Spec.Schedule, now)
		if err != nil {
			klog.V(2).InfoS("invalid schedule", "cronmaster", klog.KRef(newCron.GetNamespace(), newCron.GetName()), "err", err)
			return
		}
		if isStart {
			newCron.Spec.NextScheduleAction = appsV1alpha1.Start
		} else {
			newCron.Spec.NextScheduleAction = appsV1alpha1.Stop
		}

		c.enqueueControllerAfter(curr, *td)
		return
	}
	klog.V(4).Infof("no updates on the spec of cronmaster %q, skipping syncing", oldCron.Name)

	if oldCron.Spec.NextScheduleAction != newCron.Spec.NextScheduleAction {
		klog.V(2).InfoS("only update 'NextScheduleAction'", "cronmaster", klog.KRef(newCron.GetNamespace(), newCron.GetName()))
		return
	}
	// other parameters changed, requeue this now and if this gets triggered
	// within deadline, sync loop will work on the CJ otherwise updates will be handled
	// during the next schedule
	c.enqueueController(curr)
}

func (c *Controller) deleteCronMaster(obj interface{}) {
	cron, ok := obj.(*appsV1alpha1.CronMaster)
	if !ok {
		tombstone, ok := obj.(cache.DeletedFinalStateUnknown)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("couldn't get object from tombstone %#v", obj))
			return
		}
		cron, ok = tombstone.Obj.(*appsV1alpha1.CronMaster)
		if !ok {
			utilruntime.HandleError(fmt.Errorf("tombstone contained object that is not a CronMaster %#v", obj))
			return
		}
	}
	klog.V(4).Infof("deleting CronMaster %q", klog.KObj(cron))
	c.enqueueController(cron)
}

func (c *Controller) enqueueController(obj interface{}) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}

	c.queue.Add(key)
}

func (c *Controller) enqueueControllerAfter(obj interface{}, t time.Duration) {
	key, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("couldn't get key for object %+v: %v", obj, err))
		return
	}

	c.queue.AddAfter(key, t)
}

// getResourceToBeReconciled returns resources from cronmaster
func (c *Controller) getResourceToBeReconciled(cron *appsV1alpha1.CronMaster) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(cron.Spec.Resource.RawData)
	if err != nil {
		msg := fmt.Sprintf("failed to unmarshal cron RawData to resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}

	return resource, nil
}

func (c *Controller) syncCronMaster(cron *appsV1alpha1.CronMaster, resource *unstructured.Unstructured) (*appsV1alpha1.CronMaster, bool, *time.Duration, error) {
	klog.V(2).InfoS("syncCronMaster: handle CronMaster ... ", "CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()))
	cron = cron.DeepCopy()
	now := c.now()
	var err error
	// update status

	// todo delete resource
	if cron.DeletionTimestamp != nil {
		// The CronJob is being deleted.
		// Don't do anything other than updating status.
		return cron, false, nil, nil
	}

	// handle current action start or stop
	action := cron.Spec.NextScheduleAction
	if action != "" {
		kind := cron.GetResourceKind()
		if kind == "" || (kind != "Serverless" && kind != "Deployment") {
			klog.WarningfDepth(2, "kind of cronmaster resource %q error", cron.GetResourceKind())
			return cron, false, nil, fmt.Errorf("kind of cronmaster resource %q error", cron.GetResourceKind())
		}

		if kind == "Deployment" {
			// start deploy
			if action == "start" {
				depExist, err := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).Get(context.TODO(), cron.GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
						retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
						if retryErr != nil {
							klog.Infof("apply Deployment %q of cronmaster %q err==%v \n", klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
						}
					} else {
						klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error == %v", klog.KObj(cron), err)
					}
				} else {
					deployResource := &appsv1.Deployment{}
					if err = utils.UnstructuredConvertToStruct(resource, deployResource); err != nil {
						return cron, false, nil, err
					}
					replicas := *deployResource.Spec.Replicas + *depExist.Spec.Replicas
					depCopy := depExist.DeepCopy()
					depCopy.Spec.Replicas = &(replicas)
					_, err = c.kubeClient.AppsV1().Deployments(depCopy.GetNamespace()).Update(context.TODO(), depCopy, metav1.UpdateOptions{})
					if err != nil {
						klog.WarningDepth(2,
							fmt.Sprintf("syncCronMaster: failed to update replicas of existing Deployment %q, error==%v", klog.KObj(cron), err))
					}
				}
			}
			// stop 直接删除deploy
			if action == "stop" {
				retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
				if retryErr != nil {
					klog.Infof("delete Deployment %q of cronmaster %q err==%v \n", klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
				}
			}
		}

		if kind == "Serverless" {
			if action == "start" {
				var kind = schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: "Serverless"}
				restMapping, err := c.restMapper.RESTMapping(kind.GroupKind(), kind.Version)
				if err != nil {
					klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error == %v", klog.KObj(cron), err)
					return cron, false, nil, err
				}
				_, err = c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
				if err != nil {
					if apierrors.IsNotFound(err) {
						resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
						retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
						if retryErr != nil {
							klog.Infof("apply Serverless %q of cronmaster %q, error==%v \n", klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
						}
					} else {
						klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err)
					}
				}
				// else Serverless自调谐
			}
		}
		// stop 直接删除deploy
		if action == "stop" {
			retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
			if retryErr != nil {
				klog.Infof("delete Serverless %q of cronmaster %q err==%v \n", klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
			}
		}
	}

	// cron is enabled
	if cron.Spec.Schedule.CronEnable {
		// get and update NextScheduleAction
		td, isStart, err := nextScheduledTimeDuration(cron.Spec.Schedule, now)
		if err != nil {
			klog.V(2).InfoS("invalid schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "err", err)
			return cron, false, nil, nil
		}
		if isStart {
			cron.Spec.NextScheduleAction = appsV1alpha1.Start
		} else {
			cron.Spec.NextScheduleAction = appsV1alpha1.Stop
		}
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).Update(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("unable to update nextScheduleAction", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "err", err)
			return cron, false, nil, fmt.Errorf("unable to update nextScheduleAction for %s (rv = %s): %v", klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		return updatedCron, false, td, nil
	}
	// Timing start not cron
	if cron.Spec.Schedule.StartEnable && !cron.Spec.Schedule.EndEnable {
		if cron.Spec.NextScheduleAction != "" {
			klog.InfoS("already handled", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion)
			return cron, true, nil, fmt.Errorf("already handled cronmaster %s ", klog.KRef(cron.GetNamespace(), cron.GetName()))
		}
		td, _, err := nextScheduledTimeDuration(cron.Spec.Schedule, now)
		if err != nil {
			klog.V(2).InfoS("invalid schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "err", err)
			return cron, false, nil, nil
		}
		cron.Spec.NextScheduleAction = appsV1alpha1.Start
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).Update(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update nextScheduleAction", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "err", err)
			return cron, false, nil, fmt.Errorf("unable to update nextScheduleAction for %s (rv = %s): %v", klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		return updatedCron, false, td, nil
	}
	// Timing stop not cron
	if !cron.Spec.Schedule.StartEnable && cron.Spec.Schedule.EndEnable {
		if cron.Spec.NextScheduleAction != "" {
			klog.InfoS("already handled", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion)
			return cron, true, nil, fmt.Errorf("already handled cronmaster %s ", klog.KRef(cron.GetNamespace(), cron.GetName()))
		}
		// create resource
		resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
		retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
		if retryErr != nil {
			klog.Infof("apply Deployment %q of cronmaster %q err==%v \n", klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
		}

		td, _, err := nextScheduledTimeDuration(cron.Spec.Schedule, now)
		if err != nil {
			klog.V(2).InfoS("invalid schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "err", err)
			return cron, false, nil, nil
		}
		cron.Spec.NextScheduleAction = appsV1alpha1.Stop
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).Update(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update nextScheduleAction", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "err", err)
			return cron, false, nil, fmt.Errorf("unable to update nextScheduleAction for %s (rv = %s): %v", klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		return updatedCron, false, td, nil
	}
	// Timing start before timing stop not cron
	if cron.Spec.Schedule.StartEnable && cron.Spec.Schedule.EndEnable {
		if cron.Spec.NextScheduleAction != "stop" {
			klog.InfoS("already handled", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion)
			return cron, true, nil, fmt.Errorf("already handled cronmaster %s ", klog.KRef(cron.GetNamespace(), cron.GetName()))
		}

		td, isStart, err := nextScheduledTimeDuration(cron.Spec.Schedule, now)
		if err != nil {
			klog.V(2).InfoS("invalid schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "err", err)
			return cron, false, nil, nil
		}
		if isStart && cron.Spec.NextScheduleAction != "" {
			cron.Spec.NextScheduleAction = appsV1alpha1.Start
		} else if !isStart && cron.Spec.NextScheduleAction != "start" {
			cron.Spec.NextScheduleAction = appsV1alpha1.Stop
		} else {
			klog.InfoS("timing start before timing stop: failed to get correct nextScheduledTimeDuration", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion)
			return cron, false, nil, fmt.Errorf("timing start before timing stop: failed to get correct nextScheduledTimeDuration, cronmaster: %q(rv = %s)", klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion)
		}
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).Update(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update nextScheduleAction", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "err", err)
			return cron, false, nil, fmt.Errorf("unable to update nextScheduleAction for %s (rv = %s): %v", klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		return updatedCron, false, td, nil
	}

	return cron, false, nil, err
}
