package cronmaster

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/lmxia/gaia/pkg/utils"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	cacheddiscovery "k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/restmapper"

	appsV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/klog/v2"

	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	applisters "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/dynamic"
	kubeInformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	k8slisters "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

var (
	cronKind       = appsV1alpha1.SchemeGroupVersion.WithKind("CronMaster")
	serverlessKind = "Serverless"
	deploymentKind = "Deployment"
)

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

func NewController(gaiaClient gaiaClientSet.Interface, kubeClient kubernetes.Interface,
	gaiaInformerFactory gaiaInformers.SharedInformerFactory,
	kubeInformerFactory kubeInformers.SharedInformerFactory, localKubeConfig *rest.Config,
) (*Controller, error) {
	localDynamicClient, err := dynamic.NewForConfig(localKubeConfig)
	if err != nil {
		return nil, fmt.Errorf("newCronMasterController: failed to get dynamicClient from local kubeconfig, ERROR: %v", err)
	}
	cronInformer := gaiaInformerFactory.Apps().V1alpha1().CronMasters()
	c := &Controller{
		queue: workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cronmaster"),
		restMapper: restmapper.NewDeferredDiscoveryRESTMapper(cacheddiscovery.NewMemCacheClient(
			kubeClient.Discovery())),
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

	klog.Infof("Starting local cronmaster controller ...")
	defer klog.Infof("Shutting down local cronmaster controller")

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

	requeueAfter, err := c.sync(key.(string))
	switch {
	case err != nil:
		utilruntime.HandleError(fmt.Errorf("error syncing CronMasterController %v, requeuing: %v", key.(string), err))
		c.queue.AddRateLimited(key)
	case requeueAfter != nil:
		c.queue.Forget(key)
		c.queue.AddAfter(key, *requeueAfter)
	}
	return true
}

func (c *Controller) sync(cronKey string) (*time.Duration, error) {
	ns, name, err := cache.SplitMetaNamespaceKey(cronKey)
	if err != nil {
		return nil, err
	}
	klog.InfoS("sync CronMaster ...  ", "CronMaster", klog.KRef(ns, name))

	cron, err := c.cronMasterList.CronMasters(ns).Get(name)
	switch {
	case apierrors.IsNotFound(err):
		// may be cronmaster is deleted, do not need to requeue this key
		klog.V(4).InfoS("cronmaster not found, may be it is deleted", "cronmaster", klog.KRef(ns, name), "error", err)
		return nil, nil
	case err != nil:
		// for other transient apiserver error requeue with exponential backoff
		return nil, err
	}

	resourceRaw, err := c.getResourceRaw(cron)
	if err != nil {
		return nil, err
	}

	_, requeueAfter, err := c.syncCronMaster(cron, resourceRaw)
	if err != nil {
		klog.V(2).InfoS("error reconciling cronmaster", "cronmaster",
			klog.KRef(cron.GetNamespace(), cron.GetName()), "ERROR:", err)
		return nil, err
	}

	if requeueAfter != nil {
		klog.V(4).InfoS("re-queuing cronmaster", "cronmaster",
			klog.KRef(cron.GetNamespace(), cron.GetName()), "requeueAfter", requeueAfter)
		return requeueAfter, nil
	}
	// this marks the key done, currently only happens when the cronmaster is suspended or spec has invalid schedule format
	return nil, nil
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
		nextTime, isStart1 := getNextScheduledTimeAfterNow(newCron.Spec.Schedule, now)
		td := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster",
			klog.KRef(newCron.GetNamespace(), newCron.GetName()), "DateTime", nextTime.String(),
			"isStart", isStart1, "requeueAfter", td)

		if isStart1 {
			newCron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			newCron.Status.NextScheduleAction = appsV1alpha1.Stop
		}
		newCron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(newCron.GetNamespace()).UpdateStatus(
			context.TODO(), newCron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(newCron.GetNamespace(), newCron.GetName()),
				"resourceVersion", newCron.ResourceVersion, "error", err)
			c.enqueueControllerAfter(curr, *td)
			return
		}
		*curr.(*appsV1alpha1.CronMaster) = *updatedCron
		c.enqueueControllerAfter(curr, *td)
		return
	}

	// other parameters changed, requeue this now and if this gets triggered
	// within deadline, sync loop will work on the CJ otherwise updates will be handled
	// during the next schedule
	// TODO: need to handle the change of spec.JobTemplate.metadata.labels explicitly
	//   to cleanup jobs with old labels
	c.enqueueController(curr)
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
func (c *Controller) getResourceRaw(cron *appsV1alpha1.CronMaster) (*unstructured.Unstructured, error) {
	resource := &unstructured.Unstructured{}
	err := resource.UnmarshalJSON(cron.Spec.Resource.RawData)
	if err != nil {
		msg := fmt.Sprintf("failed to unmarshal cron RawData to resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}

	return resource, nil
}

func (c *Controller) syncCronMaster(cron *appsV1alpha1.CronMaster, resource *unstructured.Unstructured,
) (*appsV1alpha1.CronMaster, *time.Duration, error) {
	cron = cron.DeepCopy()
	now := c.now()
	var err error

	// update status
	kind := cron.GetResourceKind()
	if kind == "" || (kind != serverlessKind && kind != deploymentKind) {
		klog.WarningfDepth(2, "kind of cronmaster resource %q error", cron.GetResourceKind())
		return cron, nil, fmt.Errorf("kind of cronmaster resource %q error", cron.GetResourceKind())
	}
	err = c.manageStatus(cron, kind)
	if err != nil {
		return cron, nil, err
	}
	updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
		context.TODO(), cron, metav1.UpdateOptions{})
	if err != nil {
		klog.InfoS("Unable to update status for CronMaster",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"resourceVersion", cron.ResourceVersion, "error", err)
		return cron, nil, err
	}
	*cron = *updatedCron

	if cron.DeletionTimestamp != nil {
		// The CronJob is being deleted.
		// Don't do anything other than updating status.
		return cron, nil, nil
	}

	if cron.Spec.Schedule.CronEnable {
		cron2, td, err := c.handleCron(cron, resource, kind, now)
		return cron2, td, err
	}
	// Timing start not cron
	if cron.Spec.Schedule.StartEnable && !cron.Spec.Schedule.EndEnable {
		cron2, td, err := c.handleOnlyStart(cron, resource, kind, now)
		return cron2, td, err
	}
	// Timing stop not cron
	if !cron.Spec.Schedule.StartEnable && cron.Spec.Schedule.EndEnable {
		cron2, td, err := c.handleOnlyEnd(cron, resource, kind, now)
		return cron2, td, err
	}
	// Timing start before timing stop not cron
	if cron.Spec.Schedule.StartEnable && cron.Spec.Schedule.EndEnable {
		cron2, td, err := c.handleStartAndStop(cron, resource, kind, now)
		return cron2, td, err
	}

	return cron, nil, nil
}

func (c *Controller) getResourcesToBeReconciled(cron *appsV1alpha1.CronMaster, kind string,
) ([]*appsv1.Deployment, []*unstructured.Unstructured, error) {
	switch kind {
	case deploymentKind:
		depList, err := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			return nil, nil, err
		}
		var depsToBeReconciled []*appsv1.Deployment
		for i := range depList.Items {
			// If it has a ControllerRef, that's all that matters.
			controllerRef := metav1.GetControllerOf(&depList.Items[i])
			if controllerRef != nil && controllerRef.Name == cron.Name {
				// this dep is needs to be reconciled
				depsToBeReconciled = append(depsToBeReconciled, &depList.Items[i])
			}
		}
		return depsToBeReconciled, nil, nil

	case serverlessKind:
		gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
		restMapping, err := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), err)
			return nil, nil, err
		}
		serList, errList := c.dynamicClient.Resource(restMapping.Resource).Namespace(cron.GetNamespace()).
			List(context.TODO(), metav1.ListOptions{})
		if errList != nil {
			return nil, nil, errList
		}

		var sersToBeReconciled []*unstructured.Unstructured
		for i := range serList.Items {
			// If it has a ControllerRef, that's all that matters.
			controllerRef := metav1.GetControllerOf(&serList.Items[i])
			if controllerRef != nil && controllerRef.Name == cron.Name {
				// this ser is needs to be reconciled
				sersToBeReconciled = append(sersToBeReconciled, &serList.Items[i])
			}
		}
		return nil, sersToBeReconciled, nil
	}

	return nil, nil, fmt.Errorf("faild list resource of %q", klog.KObj(cron))
}

func (c *Controller) manageStatus(cron *appsV1alpha1.CronMaster, kind string) error {
	childrenResources := make(map[types.UID]bool)
	depList, serList, err := c.getResourcesToBeReconciled(cron, kind)
	if err != nil {
		return err
	}

	if kind == deploymentKind {
		for _, d := range depList {
			childrenResources[d.ObjectMeta.UID] = true
			found := inActiveList(*cron, d.ObjectMeta.UID)
			if !found {
				cronCopy, errGet := c.cronMasterList.CronMasters(cron.GetNamespace()).Get(cron.GetName())
				if errGet != nil {
					return errGet
				}
				if inActiveList(*cronCopy, d.ObjectMeta.UID) {
					cron = cronCopy
					continue
				}
			} else {
				_, status := getFinishedStatus(d)
				// deleteFromActiveList(cron, d.ObjectMeta.UID)
				klog.V(5).InfoS("manage Status",
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
					"Saw existed deployment", klog.KRef(d.GetNamespace(), d.GetName()), "status", status)
				// todo  update cron.Status.LastSuccessfulTime
			}
		}
	}
	if kind == serverlessKind {
		for _, s := range serList {
			childrenResources[s.GetUID()] = true
			found := inActiveList(*cron, s.GetUID())
			if !found {
				cronCopy, errGet := c.cronMasterList.CronMasters(cron.GetNamespace()).Get(cron.GetName())
				if errGet != nil {
					return errGet
				}
				if inActiveList(*cronCopy, s.GetUID()) {
					cron = cronCopy
					continue
				}
			} else {
				// _, status := getFinishedStatus(s)
				// deleteFromActiveList(cron, s.ObjectMeta.UID)
				klog.V(5).InfoS("manage Status",
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
					"Saw existed serverless", klog.KRef(s.GetNamespace(), s.GetName()))
				// todo  update cron.Status.LastSuccessfulTime
			}
		}
	}

	// Remove any resource reference from the active list if the corresponding resource does not exist anymore.
	// Otherwise, the cronmaster may be stuck in active mode forever even though there is no matching
	// resource running.
	for _, j := range cron.Status.Active {
		_, found := childrenResources[j.UID]
		if found {
			continue
		}
		// Explicitly try to get the resource from api-server to avoid a slow watch not able to update
		// the job lister on time, giving an unwanted miss
		switch j.Kind {
		case deploymentKind:
			_, errDep := c.gaiaClient.AppsV1alpha1().CronMasters(j.Namespace).
				Get(context.TODO(), j.Name, metav1.GetOptions{})
			switch {
			case apierrors.IsNotFound(errDep):
				// The resource is actually missing, delete from active list and schedule a new one if within
				// deadline
				klog.V(5).InfoS("deployment is missing, deleting",
					deploymentKind, klog.KRef(j.Namespace, j.Name),
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()))
				deleteFromActiveList(cron, j.UID)
			case errDep != nil:
				return errDep
			}
		case serverlessKind:
			gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
			restMapping, errM := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if errM != nil {
				klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), err)
				return errM
			}
			_, err = c.dynamicClient.Resource(restMapping.Resource).Namespace(j.Namespace).
				Get(context.TODO(), j.Name, metav1.GetOptions{})
			switch {
			case apierrors.IsNotFound(err):
				// The resource is actually missing, delete from active list and schedule a new one if within
				// deadline
				klog.V(5).InfoS("serverless is missing, deleting",
					serverlessKind, klog.KRef(j.Namespace, j.Name),
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()))
				deleteFromActiveList(cron, j.UID)
			case err != nil:
				return err
			}
		}
		// the job is missing in the lister but found in api-server
	}

	return nil
}

// handleOnlyStart means only start not cron
func (c *Controller) handleOnlyStart(cron *appsV1alpha1.CronMaster, resource *unstructured.Unstructured,
	kind string, now time.Time,
) (*appsV1alpha1.CronMaster, *time.Duration, error) {
	depList, serList, err := c.getResourcesToBeReconciled(cron, kind)
	if err != nil {
		klog.Errorf("failed to get resource to reconcile")
		return nil, nil, err
	}
	if (len(cron.Status.Active) > 0 && (len(depList) > 0 || len(serList) > 0)) ||
		cron.Status.NextScheduleAction == appsV1alpha1.Processed {
		klog.InfoS("handleOnlyStart: already handled", "CronMaster",
			klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion)
		return cron, nil, nil
	}

	scheduledTime, isStart, err := getCronNextScheduleTime(cron, now)
	if err != nil || !isStart {
		klog.V(2).InfoS("invalid schedule or not start", "CronMaster",
			klog.KRef(cron.GetNamespace(), cron.GetName()), "schedule", cron.Spec.Schedule,
			"scheduledTime", scheduledTime.String(), "error", err)
		return cron, nil, err
	}

	if scheduledTime == nil {
		klog.V(4).InfoS("No unmet start times", "CronMaster",
			klog.KRef(cron.GetNamespace(), cron.GetName()))
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next start datetime, next ScheduleTime==now %q",
				nextTime.String())
		}
		t := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", t)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			klog.InfoS("invalid schedule lead to no start",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"next ScheduleTime", nextTime.String())
			return cron, nil, fmt.Errorf("invalid schedule lead to no start, CronMaster==%q next ScheduleTime==%q",
				klog.KRef(cron.GetNamespace(), cron.GetName()), nextTime.String())
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", err)
			return cron, nil, err
		}
		*cron = *updatedCron
		return cron, t, nil
	}

	if len(cron.Status.Active) > 0 && cron.Status.LastScheduleTime.Equal(&metav1.Time{Time: *scheduledTime}) {
		klog.V(4).InfoS("Not handle cronmaster because the scheduled time is already processed",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "schedule", scheduledTime)
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next start datetime, next ScheduleTime==now %q",
				nextTime.String())
		}
		t := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", t)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			klog.InfoS("invalid schedule lead to not start",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"next ScheduleTime", nextTime.String())
			return cron, nil, fmt.Errorf("invalid schedule lead to no start, CronMaster==%q  next ScheduleTime==%q",
				klog.KRef(cron.GetNamespace(), cron.GetName()), nextTime.String())
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", err)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		*cron = *updatedCron
		return cron, t, nil
	}

	// create or update resources for a cronmaster
	switch kind {
	case deploymentKind:
		resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
		retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
		if retryErr != nil {
			klog.InfoS("failed apply Deployment",
				deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
				"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
			return cron, nil, retryErr
		}
		klog.InfoS("apply deployment successfully",
			deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
			"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))

		dep2, err := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).
			Get(context.TODO(), cron.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), err)
			return cron, nil, err
		}
		deleteFromActiveList(cron, dep2.GetUID())
		cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
			Kind:            dep2.Kind,
			APIVersion:      dep2.APIVersion,
			Name:            dep2.Name,
			Namespace:       dep2.Namespace,
			UID:             dep2.UID,
			ResourceVersion: dep2.ResourceVersion,
		})
		cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
		cron.Status.NextScheduleAction = appsV1alpha1.Processed
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", err)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		*cron = *updatedCron

	case serverlessKind:
		gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
		restMapping, err := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), err)
			return cron, nil, err
		}
		_, err = c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
				retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
				if retryErr != nil {
					klog.InfoS("failed apply Serverless",
						serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
						"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
					return cron, nil, retryErr
				}
				klog.InfoS("apply serverless successfully",
					serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
					"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))
			} else {
				klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err)
				return cron, nil, err
			}
		} else {
			break
		}

		// update status
		serU, err := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
		if err != nil {
			klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v",
				klog.KObj(cron), err)
			return cron, nil, err
		}
		deleteFromActiveList(cron, serU.GetUID())
		cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
			Kind:            serU.GetKind(),
			APIVersion:      serU.GetAPIVersion(),
			Name:            serU.GetName(),
			Namespace:       serU.GetNamespace(),
			UID:             serU.GetUID(),
			ResourceVersion: serU.GetResourceVersion(),
		})
		cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
		cron.Status.NextScheduleAction = appsV1alpha1.Processed
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", err)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		*cron = *updatedCron
	}

	return cron, nil, nil
}

func (c *Controller) handleOnlyEnd(cron *appsV1alpha1.CronMaster, resource *unstructured.Unstructured,
	kind string, now time.Time,
) (*appsV1alpha1.CronMaster, *time.Duration, error) {
	if cron.Status.NextScheduleAction == appsV1alpha1.Processed {
		klog.InfoS("handleOnlyEnd: already handled",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"resourceVersion", cron.ResourceVersion)
		return cron, nil, nil
	}
	depList, serList, err := c.getResourcesToBeReconciled(cron, kind)
	if err != nil {
		klog.Errorf("failed to get resource to reconcile")
		return nil, nil, err
	}
	if len(cron.Status.Active) == 0 && (len(depList) == 0 && len(serList) == 0) {
		// create or update resources for a cronmaster
		switch kind {
		case deploymentKind:
			resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
			retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
			if retryErr != nil {
				klog.InfoS("failed apply deployment",
					deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
					"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
				return cron, nil, retryErr
			}
			klog.InfoS("apply deployment successfully",
				deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
				"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))

			dep2, errGet := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).
				Get(context.TODO(), cron.GetName(), metav1.GetOptions{})
			if errGet != nil {
				klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), errGet)
				return cron, nil, errGet
			}
			deleteFromActiveList(cron, dep2.GetUID())
			cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
				Kind:            dep2.Kind,
				APIVersion:      dep2.APIVersion,
				Name:            dep2.Name,
				Namespace:       dep2.Namespace,
				UID:             dep2.UID,
				ResourceVersion: dep2.ResourceVersion,
			})
			cron.Status.LastScheduleTime = &metav1.Time{Time: now}
			updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
				UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
			if errUS != nil {
				klog.InfoS("Unable to update status for CronMaster",
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
					"resourceVersion", cron.ResourceVersion, "error", errUS)
				return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
					klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
			}
			*cron = *updatedCron

		case serverlessKind:
			gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
			restMapping, errM := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if errM != nil {
				klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), errM)
				return cron, nil, errM
			}
			_, err = c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
				Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
					retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
					if retryErr != nil {
						klog.InfoS("failed apply serverless",
							serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
							"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
						return cron, nil, retryErr
					}
					klog.InfoS("apply serverless successfully",
						serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
						"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))
				} else {
					klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err)
					return cron, nil, err
				}
			} else {
				klog.InfoS("handleOnlyEnd: serverless already exists does not need to create",
					serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
					"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))
				break
			}

			// update status
			serU, err := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
				Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
			if err != nil {
				klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err)
				return cron, nil, err
			}
			deleteFromActiveList(cron, serU.GetUID())
			cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
				Kind:            serU.GetKind(),
				APIVersion:      serU.GetAPIVersion(),
				Name:            serU.GetName(),
				Namespace:       serU.GetNamespace(),
				UID:             serU.GetUID(),
				ResourceVersion: serU.GetResourceVersion(),
			})
			cron.Status.LastScheduleTime = &metav1.Time{Time: now}
			updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
				UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
			if err != nil {
				klog.InfoS("Unable to update status for CronMaster", "CronMaster",
					klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "error", err)
				return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
					klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
			}
			*cron = *updatedCron
		}
	}

	scheduledTime, isStart, err := getCronNextScheduleTime(cron, now)
	if err != nil || isStart {
		klog.V(2).InfoS("invalid schedule or not stop",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"schedule", cron.Spec.Schedule, "scheduledTime", scheduledTime.String(), "error", err)
		return cron, nil, err
	}

	if scheduledTime == nil {
		klog.V(4).InfoS("No unmet stop times", "CronMaster",
			klog.KRef(cron.GetNamespace(), cron.GetName()))
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next stop datetime, next ScheduleTime==now %q", nextTime.String())
		}
		t := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", t)

		if !isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Stop
		} else {
			klog.InfoS("invalid schedule lead to not stop",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"next ScheduleTime", nextTime.String(), "error", err)
			return cron, nil, fmt.Errorf("failed to get next stop datetime, next ScheduleTime==now %q", nextTime.String())
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, err2 := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if err2 != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", err2)
			return cron, nil, err2
		}
		*cron = *updatedCron
		return cron, t, nil
	}

	if len(cron.Status.Active) > 0 && cron.Status.LastScheduleTime.Equal(&metav1.Time{Time: *scheduledTime}) {
		klog.V(4).InfoS("not handle cronmaster because the scheduled time is already processed",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "schedule", scheduledTime)
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next stop datetime, next ScheduleTime==now %q", nextTime.String())
		}
		t := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", t)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Stop
		} else {
			klog.InfoS("invalid schedule lead to no stop",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"next ScheduleTime", nextTime.String(), "error", err)
			return cron, nil, nil
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if errUS != nil {
			klog.InfoS("Unable to update status for CronMaster", "CronMaster",
				klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "error", errUS)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
		}
		*cron = *updatedCron
		return cron, t, nil
	}

	// delete resources
	updated := false
	switch kind {
	case deploymentKind:
		dep2, err2 := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).
			Get(context.TODO(), cron.GetName(), metav1.GetOptions{})
		if err2 != nil {
			if apierrors.IsNotFound(err2) {
				klog.InfoS("handleOnlyEnd: stop deployment, but not exists",
					deploymentKind, klog.KObj(cron), "CronMaster", klog.KObj(cron), "error", err2)
				break
			}
			klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster==%q, error==%v", klog.KObj(cron), err)
			return cron, nil, err2
		}
		retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
		if retryErr != nil {
			klog.Infof("delete Deployment %q of cronmaster %q error==%v \n",
				klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
			return cron, nil, retryErr
		}
		deleteFromActiveList(cron, dep2.GetUID())
		klog.Infof("deleted deployment %q of cronmaster %q \n",
			klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron))
		updated = true

	case serverlessKind:
		gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
		restMapping, err := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		if err != nil {
			klog.Errorf("syncCronMaster: failed to get restMapping for Serverless==%q, error==%v", klog.KObj(cron), err)
			return cron, nil, err
		}
		serU, err2 := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
		if err2 != nil {
			if apierrors.IsNotFound(err2) {
				klog.InfoS("syncCronMaster: stop serverless, but not exists",
					serverlessKind, klog.KObj(cron), "CronMaster", klog.KObj(cron), "error", err2)
				break
			}
			klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err2)
			return cron, nil, err2
		}
		retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
		if retryErr != nil {
			klog.Infof("delete Serverless %q of cronmaster %q err2==%v \n",
				klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
			return cron, nil, retryErr
		}
		deleteFromActiveList(cron, serU.GetUID())
		klog.Infof("deleted serverless %q of cronmaster %q \n",
			klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron))
		updated = true
	}

	// update status
	if updated {
		cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
		cron.Status.NextScheduleAction = appsV1alpha1.Processed
		updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if err != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "error", err)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, err)
		}
		*cron = *updatedCron
	}

	return cron, nil, nil
}

func (c *Controller) handleStartAndStop(cron *appsV1alpha1.CronMaster, resource *unstructured.Unstructured, kind string, now time.Time) (*appsV1alpha1.CronMaster, *time.Duration, error) {
	if cron.Status.NextScheduleAction == appsV1alpha1.Processed {
		klog.InfoS("handleStartAndStop: already handled",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"resourceVersion", cron.ResourceVersion)
		return cron, nil, nil
	}

	scheduledTime, isStart, err := getCronNextScheduleTime(cron, now)
	if err != nil {
		klog.V(2).InfoS("invalid schedule",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"schedule", cron.Spec.Schedule, "scheduledTime", scheduledTime.String(), "error", err)
		return cron, nil, err
	}
	if scheduledTime == nil {
		klog.V(4).InfoS("No unmet start times", "CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()))
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next schedule datetime, next ScheduleTime==now %q", nextTime.String())
		}
		t := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", t)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			cron.Status.NextScheduleAction = appsV1alpha1.Stop
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if errUS != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", errUS)
			return cron, nil, errUS
		}
		return updatedCron, t, nil
	}

	if cron.Status.LastScheduleTime.Equal(&metav1.Time{Time: *scheduledTime}) {
		klog.V(4).InfoS("Not handle cronmaster because the scheduled time is already processed",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "schedule", scheduledTime)
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next schedule datetime, next ScheduleTime==now %q", nextTime.String())
		}
		t := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", t)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			cron.Status.NextScheduleAction = appsV1alpha1.Stop
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
			UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
		if errUS != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", errUS)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
		}
		*cron = *updatedCron
		return cron, t, nil
	}

	// create or update resources for a cronmaster
	if isStart {
		updated := false
		switch kind {
		case deploymentKind:
			_, errGet := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).
				Get(context.TODO(), cron.GetName(), metav1.GetOptions{})
			if errGet != nil {
				if apierrors.IsNotFound(errGet) {
					resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
					retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
					if retryErr != nil {
						klog.InfoS("failed apply Deployment",
							deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
							"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
						return cron, nil, retryErr
					}
					klog.InfoS("apply deployment successfully",
						deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
						"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))

					dep2, err2 := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).
						Get(context.TODO(), cron.GetName(), metav1.GetOptions{})
					if err2 != nil {
						klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), err2)
						return cron, nil, err2
					}
					deleteFromActiveList(cron, dep2.GetUID())
					cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
						Kind:            dep2.Kind,
						APIVersion:      dep2.APIVersion,
						Name:            dep2.Name,
						Namespace:       dep2.Namespace,
						UID:             dep2.UID,
						ResourceVersion: dep2.ResourceVersion,
					})
					updated = true
				} else {
					klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), errGet)
				}
			}
		case serverlessKind:
			gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
			restMapping, errM := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if errM != nil {
				klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), errM)
				return cron, nil, errM
			}
			_, err = c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
				Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
					retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
					if retryErr != nil {
						klog.InfoS("failed apply Serverless",
							serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
							"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
						return cron, nil, retryErr
					}
					klog.InfoS("apply serverless successfully",
						serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
						"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))

					// update status
					serU, err2 := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
						Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
					if err2 != nil {
						klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster==%q, error==%v", klog.KObj(cron), err2)
						return cron, nil, err2
					}
					deleteFromActiveList(cron, serU.GetUID())
					cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
						Kind:            serU.GetKind(),
						APIVersion:      serU.GetAPIVersion(),
						Name:            serU.GetName(),
						Namespace:       serU.GetNamespace(),
						UID:             serU.GetUID(),
						ResourceVersion: serU.GetResourceVersion(),
					})
					updated = true
				} else {
					klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err)
				}
			}
		}

		if updated {
			cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
			updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).
				UpdateStatus(context.TODO(), cron, metav1.UpdateOptions{})
			if errUS != nil {
				klog.InfoS("Unable to update status for CronMaster",
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
					"resourceVersion", cron.ResourceVersion, "error", errUS)
				return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
					klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
			}
			*cron = *updatedCron
		}
	} else {
		// stop delete resource and update status
		updated := false
		switch kind {
		case deploymentKind:
			dep2, err2 := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).Get(
				context.TODO(), cron.GetName(), metav1.GetOptions{})
			if err2 != nil {
				if apierrors.IsNotFound(err2) {
					klog.InfoS("handleStarStop: stop deployment, but not exists",
						deploymentKind, klog.KObj(cron), "CronMaster", klog.KObj(cron), "error", err2)
					break
				}
				klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), err2)
				return cron, nil, err2
			}
			retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
			if retryErr != nil {
				klog.Infof("delete Deployment %q of cronmaster %q err2==%v \n",
					klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
				return cron, nil, retryErr
			}
			deleteFromActiveList(cron, dep2.GetUID())
			klog.Infof("deleted deployment %q of cronmaster %q \n",
				klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron))
			updated = true

		case serverlessKind:
			gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
			restMapping, errM := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if errM != nil {
				klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), errM)
				return cron, nil, errM
			}
			serU, err2 := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).Get(
				context.TODO(), resource.GetName(), metav1.GetOptions{})
			if err2 != nil {
				if apierrors.IsNotFound(err2) {
					klog.InfoS("handleStarStop: stop serverless, but not exists",
						serverlessKind, klog.KObj(cron), "CronMaster", klog.KObj(cron), "error", err2)
					break
				}
				klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err2)
				return cron, nil, err2
			}
			retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
			if retryErr != nil {
				klog.Infof("delete Serverless %q of cronmaster %q error==%v \n",
					klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
				return cron, nil, retryErr
			}
			deleteFromActiveList(cron, serU.GetUID())
			klog.Infof("deleted serverless %q of cronmaster %q \n",
				klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron))
			updated = true
		}

		if updated {
			cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
			cron.Status.NextScheduleAction = appsV1alpha1.Processed
			updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
				context.TODO(), cron, metav1.UpdateOptions{})
			if errUS != nil {
				klog.InfoS("Unable to update status for CronMaster",
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
					"resourceVersion", cron.ResourceVersion, "error", errUS)
				return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
					klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
			}
			*cron = *updatedCron
			return cron, nil, nil
		}
	}

	nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
	if nextTime.Equal(now) {
		return cron, nil, fmt.Errorf("failed to get next schedule datetime, next ScheduleTime==now %q", nextTime.String())
	}
	td := nextScheduledTimeDuration(nextTime, now)
	klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
		"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", td)

	switch {
	case isStart1 && cron.Status.NextScheduleAction == "":
		cron.Status.NextScheduleAction = appsV1alpha1.Start
	case !isStart1 && cron.Status.NextScheduleAction == "start":
		cron.Status.NextScheduleAction = appsV1alpha1.Stop
	default:
		klog.InfoS("timing start before timing stop: failed to get correct nextScheduledTimeDuration",
			"cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion)
		return cron, nil, fmt.Errorf("timing start before timing stop: failed to get correct nextScheduledTimeDuration,"+
			" cronmaster: %q(rv = %s)", klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion)
	}
	cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
	updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
		context.TODO(), cron, metav1.UpdateOptions{})
	if err != nil {
		klog.InfoS("Unable to update status for CronMaster",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"resourceVersion", cron.ResourceVersion, "error", err)
		return cron, nil, err
	}
	return updatedCron, td, nil
}

func (c *Controller) handleCron(cron *appsV1alpha1.CronMaster, resource *unstructured.Unstructured, kind string, now time.Time) (*appsV1alpha1.CronMaster, *time.Duration, error) {
	scheduledTime, isStart, err := getCronNextScheduleTime(cron, now)
	if err != nil {
		klog.V(2).InfoS("invalid schedule", "CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"schedule", cron.Spec.Schedule, "scheduledTime", scheduledTime.String(), "error", err)
		return cron, nil, err
	}
	if scheduledTime == nil {
		klog.V(4).InfoS("No unmet start times", "CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()))
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next schedule datetime, next ScheduleTime==now %q", nextTime.String())
		}
		td := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", td)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			cron.Status.NextScheduleAction = appsV1alpha1.Stop
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
			context.TODO(), cron, metav1.UpdateOptions{})
		if errUS != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", errUS)
			return cron, nil, errUS
		}
		return updatedCron, td, nil
	}

	if cron.Status.LastScheduleTime.Equal(&metav1.Time{Time: *scheduledTime}) {
		klog.V(4).InfoS("Not handle cronmaster because the scheduled time is already processed",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "schedule", scheduledTime)
		nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
		if nextTime.Equal(now) {
			return cron, nil, fmt.Errorf("failed to get next schedule datetime, next ScheduleTime==now %q", nextTime.String())
		}
		td := nextScheduledTimeDuration(nextTime, now)
		klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", td)

		if isStart1 {
			cron.Status.NextScheduleAction = appsV1alpha1.Start
		} else {
			cron.Status.NextScheduleAction = appsV1alpha1.Stop
		}
		cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
		updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
			context.TODO(), cron, metav1.UpdateOptions{})
		if errUS != nil {
			klog.InfoS("Unable to update status for CronMaster",
				"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
				"resourceVersion", cron.ResourceVersion, "error", errUS)
			return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
				klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
		}
		*cron = *updatedCron
		return cron, td, nil
	}

	// create or update resources for a cronmaster
	if isStart {
		switch kind {
		case deploymentKind:
			depExist, errGet := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).Get(
				context.TODO(), cron.GetName(), metav1.GetOptions{})
			if errGet != nil {
				if apierrors.IsNotFound(errGet) {
					resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
					retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
					if retryErr != nil {
						klog.InfoS("failed apply Deployment",
							deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
							"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
						return cron, nil, retryErr
					}
					klog.InfoS("apply Deployment successful",
						deploymentKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
						"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))
				} else {
					klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), errGet)
					return cron, nil, errGet
				}
			} else {
				deployResource := &appsv1.Deployment{}
				if err = utils.UnstructuredConvertToStruct(resource, deployResource); err != nil {
					return cron, nil, err
				}
				replicas := *deployResource.Spec.Replicas + *depExist.Spec.Replicas
				depCopy := depExist.DeepCopy()
				depCopy.Spec.Replicas = &(replicas)
				_, err = c.kubeClient.AppsV1().Deployments(depCopy.GetNamespace()).Update(
					context.TODO(), depCopy, metav1.UpdateOptions{})
				if err != nil {
					klog.WarningDepth(2, fmt.Sprintf("syncCronMaster: failed to update replicas of "+
						"existing Deployment %q, error==%v", klog.KObj(cron), err))
					return cron, nil, err
				}
				klog.InfoS("cron start updated replicas of deployment",
					"from replicas", *depExist.Spec.Replicas, "to replicas", replicas,
					deploymentKind, klog.KRef(cron.GetNamespace(), cron.GetName()),
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()))
			}

			dep2, err2 := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).Get(
				context.TODO(), cron.GetName(), metav1.GetOptions{})
			if err2 != nil {
				klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), err2)
				return cron, nil, err2
			}
			deleteFromActiveList(cron, dep2.GetUID())
			cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
				Kind:            dep2.Kind,
				APIVersion:      dep2.APIVersion,
				Name:            dep2.Name,
				Namespace:       dep2.Namespace,
				UID:             dep2.UID,
				ResourceVersion: dep2.ResourceVersion,
			})
			cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
			updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
				context.TODO(), cron, metav1.UpdateOptions{})
			if errUS != nil {
				klog.InfoS("Unable to update status for CronMaster",
					"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
					"resourceVersion", cron.ResourceVersion, "error", errUS)
				return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
					klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
			}
			*cron = *updatedCron
		case serverlessKind:
			updated := false
			gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
			restMapping, errM := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if errM != nil {
				klog.Errorf("syncCronMaster: failed to get restMapping for 'Serverless' %q, error==%v", klog.KObj(cron), errM)
				return cron, nil, errM
			}
			_, err = c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).Get(
				context.TODO(), resource.GetName(), metav1.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					resource.SetOwnerReferences([]metav1.OwnerReference{*metav1.NewControllerRef(cron, cronKind)})
					retryErr := utils.ApplyResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
					if retryErr != nil {
						klog.InfoS("failed apply Serverless",
							serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
							"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()), "error", retryErr)
						return cron, nil, retryErr
					}
					klog.InfoS("apply serverless successful",
						serverlessKind, klog.KRef(resource.GetNamespace(), resource.GetName()),
						"CronMaster", klog.KRef(resource.GetNamespace(), resource.GetName()))
					updated = true
				} else {
					klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), err)
				}
			}
			// else Serverless自调谐

			// update status
			if updated {
				serU, errGet := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).Get(
					context.TODO(), resource.GetName(), metav1.GetOptions{})
				if errGet != nil {
					klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster %q, error==%v", klog.KObj(cron), errGet)
					return cron, nil, errGet
				}
				deleteFromActiveList(cron, serU.GetUID())
				cron.Status.Active = append(cron.Status.Active, v1.ObjectReference{
					Kind:            serU.GetKind(),
					APIVersion:      serU.GetAPIVersion(),
					Name:            serU.GetName(),
					Namespace:       serU.GetNamespace(),
					UID:             serU.GetUID(),
					ResourceVersion: serU.GetResourceVersion(),
				})
				cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
				updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
					context.TODO(), cron, metav1.UpdateOptions{})
				if errUS != nil {
					klog.InfoS("Unable to update status for CronMaster", "CronMaster",
						klog.KRef(cron.GetNamespace(), cron.GetName()), "resourceVersion", cron.ResourceVersion, "error", errUS)
					return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
						klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
				}
				*cron = *updatedCron
			}
		}
	} else {
		updated := false
		// stop delete resource and update status
		switch kind {
		case deploymentKind:
			dep2, err2 := c.kubeClient.AppsV1().Deployments(cron.GetNamespace()).Get(
				context.TODO(), cron.GetName(), metav1.GetOptions{})
			if err2 != nil {
				if apierrors.IsNotFound(err2) {
					klog.InfoS("syncCronMaster: stop deployment, but not exists",
						deploymentKind, klog.KObj(cron), "CronMaster", klog.KObj(cron), "err3", err2)
					break
				}
				klog.Errorf("syncCronMaster: failed to get Deployment of cronmaster %q, error==%v", klog.KObj(cron), err2)
				return cron, nil, err2
			}
			retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
			if retryErr != nil {
				klog.Infof("delete Deployment %q of cronmaster %q err3==%v \n",
					klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
				return cron, nil, retryErr
			}
			deleteFromActiveList(cron, dep2.GetUID())
			klog.Infof("deleted Deployment %q of cronmaster %q \n",
				klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron))
			updated = true
		case serverlessKind:
			gvk := schema.GroupVersionKind{Group: "serverless.pml.com.cn", Version: "v1", Kind: serverlessKind}
			restMapping, err3 := c.restMapper.RESTMapping(gvk.GroupKind(), gvk.Version)
			if err3 != nil {
				klog.Errorf("syncCronMaster: failed to get restMapping for Serverless==%q, error==%v", klog.KObj(cron), err3)
				return cron, nil, err3
			}
			serU, err2 := c.dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).Get(
				context.TODO(), resource.GetName(), metav1.GetOptions{})
			if err2 != nil {
				if apierrors.IsNotFound(err2) {
					klog.InfoS("syncCronMaster: stop serverless, but not exists",
						serverlessKind, klog.KObj(cron), "CronMaster", klog.KObj(cron), "err3", err2)
					break
				}
				klog.Errorf("syncCronMaster: failed to get Serverless of cronmaster==%q, error==%v", klog.KObj(cron), err2)
				return cron, nil, err2
			}
			retryErr := utils.DeleteResourceWithRetry(context.TODO(), c.dynamicClient, c.restMapper, resource)
			if retryErr != nil {
				klog.Infof("delete Serverless %q of cronmaster %q err2==%v \n",
					klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron), retryErr)
				return cron, nil, retryErr
			}
			deleteFromActiveList(cron, serU.GetUID())
			klog.Infof("deleted Serverless %q of cronmaster %q \n",
				klog.KRef(resource.GetNamespace(), resource.GetName()), klog.KObj(cron))
			updated = true
		}
		if updated {
			cron.Status.LastScheduleTime = &metav1.Time{Time: *scheduledTime}
			updatedCron, errUS := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
				context.TODO(), cron, metav1.UpdateOptions{})
			if errUS != nil {
				klog.InfoS("Unable to update status for CronMaster", "CronMaster", klog.KRef(cron.GetNamespace(),
					cron.GetName()), "resourceVersion", cron.ResourceVersion, "error", errUS)
				return cron, nil, fmt.Errorf("unable to update status for %s (rv = %s): %v",
					klog.KRef(cron.GetNamespace(), cron.GetName()), cron.ResourceVersion, errUS)
			}
			*cron = *updatedCron
		}
	}

	// get and update NextScheduleAction
	nextTime, isStart1 := getNextScheduledTimeAfterNow(cron.Spec.Schedule, now)
	if nextTime.Equal(now) {
		return cron, nil, fmt.Errorf("failed to get next schedule datetime, next ScheduleTime==now %q", nextTime.String())
	}
	td := nextScheduledTimeDuration(nextTime, now)
	klog.InfoS("next schedule", "cronmaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
		"DateTime", nextTime.String(), "isStart", isStart1, "requeueAfter", td)

	if isStart1 {
		cron.Status.NextScheduleAction = appsV1alpha1.Start
	} else {
		cron.Status.NextScheduleAction = appsV1alpha1.Stop
	}
	cron.Status.NextScheduleDateTime = &metav1.Time{Time: nextTime.Local()}
	updatedCron, err := c.gaiaClient.AppsV1alpha1().CronMasters(cron.GetNamespace()).UpdateStatus(
		context.TODO(), cron, metav1.UpdateOptions{})
	if err != nil {
		klog.InfoS("Unable to update status for CronMaster",
			"CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()),
			"resourceVersion", cron.ResourceVersion, "error", err)
		return cron, nil, err
	}
	*cron = *updatedCron

	return cron, td, nil
}
