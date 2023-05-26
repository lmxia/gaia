package scheduler

import (
	"context"
	"errors"
	"fmt"
	"github.com/lmxia/gaia/cmd/gaia-scheduler/app/option"
	"github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/metrics"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/client-go/util/retry"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	schedulerserverconfig "github.com/lmxia/gaia/cmd/gaia-scheduler/app/config"
	appsapi "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	platformapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	listner "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/algorithm"
	schedulercache "github.com/lmxia/gaia/pkg/scheduler/cache"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins"
	frameworkruntime "github.com/lmxia/gaia/pkg/scheduler/framework/runtime"
	"github.com/lmxia/gaia/pkg/scheduler/parallelize"
	"github.com/lmxia/gaia/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
)

// These are reasons for a subscription's transition to a condition.
const (
	// ReasonUnschedulable reason in DescriptionScheduled SubscriptionCondition means that the scheduler
	// can't schedule the subscription right now, for example due to insufficient resources in the clusters.
	ReasonUnschedulable = "Unschedulable"

	// SchedulerError is the reason recorded for events when an error occurs during scheduling a subscription.
	SchedulerError = "SchedulerError"
)

type Scheduler struct {
	// just local options
	dynamicClient                  dynamic.Interface
	localGaiaClient                *gaiaClientSet.Clientset
	localSuperConfig               *rest.Config
	localNamespacedInformerFactory gaiainformers.SharedInformerFactory // namespaced
	localGaiaAllFactory            gaiainformers.SharedInformerFactory // all ns
	localDescLister                listner.DescriptionLister
	selfClusterName                string // this cluster name

	// dedicated kubeconfig for accessing parent cluster, which is auto populated by the parent cluster when cluster registration request gets approved
	parentDedicatedKubeConfig   *rest.Config
	parentInformerFactory       gaiainformers.SharedInformerFactory // namespaced
	parentDescriptionLister     listner.DescriptionLister
	parentResourceBindingLister v1alpha1.ResourceBindingLister
	parentGaiaClient            *gaiaClientSet.Clientset

	// clientset for child cluster
	childKubeClientSet kubernetes.Interface

	dedicatedNamespace string `json:"dedicatednamespace,omitempty" protobuf:"bytes,1,opt,name=dedicatedNamespace"`
	Identity           string
	// default in-tree registry
	registry frameworkruntime.Registry

	scheduleAlgorithm algorithm.ScheduleAlgorithm

	// localSchedulingQueue holds description in local namespace to be scheduled
	localSchedulingQueue workqueue.RateLimitingInterface

	// parentSchedulingQueue holds description in parent cluster namespace to be scheduled
	parentSchedulingQueue workqueue.RateLimitingInterface

	// parentSchedulingRetryQueue holds description in parent cluster namespace to be re scheduled
	parentSchedulingRetryQueue workqueue.RateLimitingInterface

	framework framework.Framework

	lockLocal      sync.RWMutex
	lockParent     sync.RWMutex
	lockReschedule sync.RWMutex
}

// NewSchedule returns a new Scheduler.
func NewSchedule(ctx context.Context, childKubeConfigFile string, opts *option.Options) (*schedulerserverconfig.CompletedConfig, *Scheduler, error) {
	if errs := opts.Validate(); len(errs) > 0 {
		return nil, nil, utilerrors.NewAggregate(errs)
	}
	c, err := opts.Config()
	if err != nil {
		return nil, nil, err
	}
	// Get the completed config
	cc := c.Complete()

	hostname, err := os.Hostname()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	identity := hostname + "_" + string(uuid.NewUUID())
	klog.V(4).Infof("current identity lock id %q", identity)

	childKubeConfig, err := utils.LoadsKubeConfig(childKubeConfigFile, 1)
	if err != nil {
		return nil, nil, err
	}
	// create clientset for child cluster
	childKubeClientSet := kubernetes.NewForConfigOrDie(childKubeConfig)
	childGaiaClientSet := gaiaClientSet.NewForConfigOrDie(childKubeConfig)

	localAllGaiaInformerFactory := gaiainformers.NewSharedInformerFactory(childGaiaClientSet, known.DefaultResync)
	localSuperKubeConfig := NewLocalSuperKubeConfig(ctx, childKubeConfig.Host, childKubeClientSet)

	// create event recorder
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: childKubeClientSet.CoreV1().Events(""),
	})
	utilruntime.Must(appsapi.AddToScheme(scheme.Scheme))
	utilruntime.Must(platformapi.AddToScheme(scheme.Scheme))
	recorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "gaia-scheduler"})

	schedulerCache := schedulercache.New(localAllGaiaInformerFactory.Platform().V1alpha1().ManagedClusters().Lister(), childGaiaClientSet)
	dynamicClient, err := dynamic.NewForConfig(localSuperKubeConfig)
	if err != nil {
		return nil, nil, err
	}

	sched := &Scheduler{
		localGaiaClient:     childGaiaClientSet,
		localGaiaAllFactory: localAllGaiaInformerFactory,
		localDescLister:     localAllGaiaInformerFactory.Apps().V1alpha1().Descriptions().Lister(),
		childKubeClientSet:  childKubeClientSet,

		dynamicClient:              dynamicClient,
		registry:                   plugins.NewInTreeRegistry(),
		scheduleAlgorithm:          algorithm.NewGenericScheduler(schedulerCache),
		localSchedulingQueue:       workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter()),
		parentSchedulingQueue:      workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter()),
		parentSchedulingRetryQueue: workqueue.NewRateLimitingQueue(workqueue.DefaultItemBasedRateLimiter()),
	}

	framework, err := frameworkruntime.NewFramework(sched.registry, getDefaultPlugins(),
		frameworkruntime.WithEventRecorder(recorder),
		frameworkruntime.WithInformerFactory(localAllGaiaInformerFactory),
		frameworkruntime.WithClientSet(childGaiaClientSet),
		frameworkruntime.WithKubeConfig(localSuperKubeConfig), // 最高权 kubeconfig
		frameworkruntime.WithParallelism(parallelize.DefaultParallelism),
		frameworkruntime.WithRunAllFilters(false),
	)
	if err != nil {
		return nil, nil, err
	}
	sched.framework = framework
	// local scheduler always exsit
	sched.localNamespacedInformerFactory = gaiainformers.NewSharedInformerFactoryWithOptions(childGaiaClientSet, known.DefaultResync,
		gaiainformers.WithNamespace(known.GaiaReservedNamespace))
	// local event handler
	sched.addLocalAllEventHandlers()

	metrics.Register()

	return &cc, sched, nil
}

func (scheduler *Scheduler) Run(cxt context.Context, cc *schedulerserverconfig.CompletedConfig) {
	klog.Info("starting gaia schedule scheduler ...")
	defer scheduler.localSchedulingQueue.ShutDown()

	// start the leader election code loop
	leaderelection.RunOrDie(context.TODO(), *newLeaderElectionConfigWithDefaultValue(scheduler.Identity, scheduler.childKubeClientSet, leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			// 1. start local gaia generic informers
			scheduler.localGaiaAllFactory.Start(ctx.Done())
			scheduler.localGaiaAllFactory.WaitForCacheSync(ctx.Done())

			scheduler.localNamespacedInformerFactory.Start(ctx.Done())
			scheduler.localNamespacedInformerFactory.WaitForCacheSync(ctx.Done())
			// 2. start local scheduler.
			go func() {
				wait.UntilWithContext(ctx, scheduler.RunLocalScheduler, 0)
			}()

			// metrics
			if cc.SecureServing != nil {
				handler := buildHandlerChain(newMetricsHandler(), cc.Authentication.Authenticator, cc.Authorization.Authorizer)
				klog.Info("Starting gaia-scheduler metrics server...")
				if _, err := cc.SecureServing.Serve(handler, 0, ctx.Done()); err != nil {
					klog.Infof("failed to start metrics server: %v", err)
				}
			}

			// 3. when we get add parentDedicatedKubeConfig add parent desc scheduler and start it.
			scheduler.SetparentDedicatedConfig(ctx)
			scheduler.parentInformerFactory.Start(ctx.Done())
			scheduler.parentInformerFactory.WaitForCacheSync(ctx.Done())
			go func() {
				wait.UntilWithContext(ctx, scheduler.RunParentReScheduler, 0)
			}()
			wait.UntilWithContext(ctx, scheduler.RunParentScheduler, 0)
		},
		OnStoppedLeading: func() {
			klog.Error("leader election got lost")
		},
		OnNewLeader: func(identity string) {
			// we're notified when new leader elected
			if identity == scheduler.Identity {
				// I just got the lock
				return
			}
			klog.Infof("new leader elected: %s", identity)
		},
	},
	))
}

// buildHandlerChain wraps the given handler with the standard filters.
func buildHandlerChain(handler http.Handler, authn authenticator.Request, authz authorizer.Authorizer) http.Handler {
	requestInfoResolver := &apirequest.RequestInfoFactory{}
	failedHandler := genericapifilters.Unauthorized(scheme.Codecs)

	handler = genericapifilters.WithAuthorization(handler, authz, scheme.Codecs)
	handler = genericapifilters.WithAuthentication(handler, authn, failedHandler, nil)
	handler = genericapifilters.WithRequestInfo(handler, requestInfoResolver)
	handler = genericapifilters.WithCacheControl(handler)
	handler = genericfilters.WithHTTPLogging(handler)
	handler = genericfilters.WithPanicRecovery(handler, requestInfoResolver)

	return handler
}

func installMetricHandler(pathRecorderMux *mux.PathRecorderMux) {
	// configz.InstallHandler(pathRecorderMux)
	pathRecorderMux.Handle("/metrics", legacyregistry.HandlerWithReset())

}

// newMetricsHandler builds a metrics server from the config.
func newMetricsHandler() http.Handler {
	pathRecorderMux := mux.NewPathRecorderMux("gaia-scheduler")
	installMetricHandler(pathRecorderMux)
	return pathRecorderMux
}

// newHealthzHandler creates a healthz server from the config, and will also
// embed the metrics handler if the healthz and metrics address configurations
// are the same.
func newHealthzHandler(separateMetrics bool, checks ...healthz.HealthChecker) http.Handler {
	pathRecorderMux := mux.NewPathRecorderMux("gaia-scheduler")
	healthz.InstallHandler(pathRecorderMux, checks...)
	if !separateMetrics {
		installMetricHandler(pathRecorderMux)
	}

	return pathRecorderMux
}

func newLeaderElectionConfigWithDefaultValue(identity string, clientset kubernetes.Interface, callbacks leaderelection.LeaderCallbacks) *leaderelection.LeaderElectionConfig {
	return &leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      common.GaiaSchedulerLeaseName,
				Namespace: common.GaiaSystemNamespace,
			},
			Client: clientset.CoordinationV1(),
			LockConfig: resourcelock.ResourceLockConfig{
				Identity: identity,
			},
		},
		// IMPORTANT: you MUST ensure that any code you have that
		// is protected by the lease must terminate **before**
		// you call cancel. Otherwise, you could have a background
		// loop still running and another process could
		// get elected before your background loop finished, violating
		// the stated goal of the lease.
		ReleaseOnCancel: true,
		LeaseDuration:   common.DefaultLeaseDuration,
		RenewDeadline:   common.DefaultRenewDeadline,
		RetryPeriod:     common.DefaultRetryPeriod,
		Callbacks:       callbacks,
	}
}

func NewLocalSuperKubeConfig(ctx context.Context, apiserverURL string, kubeClient kubernetes.Interface) *rest.Config {
	retryCtx, retryCancel := context.WithTimeout(ctx, known.DefaultRetryPeriod)
	defer retryCancel()

	// get high priority secret.
	secret := utils.GetDeployerCredentials(retryCtx, kubeClient, common.GaiaAppSA)
	var clusterStatusKubeConfig *rest.Config
	if secret != nil {
		var err error
		clusterStatusKubeConfig, err = utils.GenerateKubeConfigFromToken(apiserverURL, string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey], 2)
		if err == nil {
			kubeClient = kubernetes.NewForConfigOrDie(clusterStatusKubeConfig)
		}
	}
	return clusterStatusKubeConfig
}

func (sched *Scheduler) RunLocalScheduler(ctx context.Context) {
	key, shutdown := sched.localSchedulingQueue.Get()
	if shutdown {
		klog.Error("failed to get next unscheduled description from closed queue")
		return
	}
	defer sched.localSchedulingQueue.Done(key)

	// TODO: scheduling
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return
	}

	desc, err := sched.localDescLister.Descriptions(ns).Get(name)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	klog.InfoS("Attempting to schedule description", "description", klog.KObj(desc))

	// Synchronously attempt to find a fit for the description.
	start := time.Now()

	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	scheduleResult, err := sched.scheduleAlgorithm.Schedule(schedulingCycleCtx, sched.framework, nil, desc)
	if err != nil {
		sched.recordSchedulingFailure(desc, err, ReasonUnschedulable)
		var lastError error
		err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
			newDesc, _ := sched.localGaiaClient.AppsV1alpha1().Descriptions(known.GaiaReservedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
			if appsapi.DescriptionPhaseFailure == newDesc.Status.Phase {
				lastError = errors.New("description status phase is already 'Failure', there is no need to update it.")
				return true, nil
			}

			desc.Status.Phase = appsapi.DescriptionPhaseFailure
			// check if failed
			_, lastError := sched.localGaiaClient.AppsV1alpha1().Descriptions(known.GaiaReservedNamespace).UpdateStatus(ctx, desc, metav1.UpdateOptions{})
			if lastError == nil {
				return true, nil
			}
			if apierrors.IsConflict(lastError) {
				newDesc, lastError := sched.localGaiaClient.AppsV1alpha1().Descriptions(known.GaiaReservedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
				if lastError == nil {
					desc = newDesc
				}
			}
			return false, nil
		})
		if err != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ", desc.Namespace, desc.Name, lastError)
		}
		klog.Warningf("scheduler failed %v", err)
		return
	}

	mcls, _ := sched.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if len(mcls.Items) == 0 {
		klog.Warningf("scheduler success but do nothing because there is no child clusters.")
	} else {
		// 1. create rbs in sub children cluster namespace.
		for _, itemCluster := range mcls.Items {
			for rbIndex, itemRb := range scheduleResult.ResourceBindings {
				itemRb.Name = fmt.Sprintf("%s-rs-%d", desc.Name, rbIndex)
				itemRb.Namespace = itemCluster.Namespace
				itemRb.Spec.TotalPeer = getTotal(itemRb.Spec.TotalPeer, len(scheduleResult.ResourceBindings))
				_, err := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(itemCluster.Namespace).
					Create(ctx, itemRb, metav1.CreateOptions{})
				if err != nil {
					klog.Infof("scheduler success, but some rb not created success %v", err)
				}
			}
			// 2. create desc in to child cluster namespace
			newDesc := utils.ConstructDescriptionFromExistOne(desc)
			newDesc.Namespace = itemCluster.Namespace
			_, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(itemCluster.Namespace).Create(ctx, newDesc, metav1.CreateOptions{})
			if err != nil {
				klog.InfoS("scheduler success, but desc not created success in sub child cluster.", err)
			}
		}
		var lastError error
		err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
			newDesc, _ := sched.localGaiaClient.AppsV1alpha1().Descriptions(known.GaiaReservedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
			if appsapi.DescriptionPhaseScheduled == newDesc.Status.Phase {
				lastError = errors.New("description status phase is already 'Scheduled', there is no need to update it.")
				return true, nil
			}

			desc.Status.Phase = appsapi.DescriptionPhaseScheduled
			_, lastError := sched.localGaiaClient.AppsV1alpha1().Descriptions(known.GaiaReservedNamespace).UpdateStatus(ctx, desc, metav1.UpdateOptions{})
			// check if failed
			if lastError == nil {
				return true, nil
			}
			if apierrors.IsConflict(lastError) {
				newDesc, lastError := sched.localGaiaClient.AppsV1alpha1().Descriptions(known.GaiaReservedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
				if lastError == nil {
					desc = newDesc
				}
			}
			return false, nil
		})
		if err != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ", desc.Namespace, desc.Name, lastError)
		}
		metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
		metrics.DescriptionScheduled(sched.framework.ProfileName(), metrics.SinceInSeconds(start))
		klog.Infof("scheduler success %v", scheduleResult)
	}
}

func (sched *Scheduler) RunParentScheduler(ctx context.Context) {
	klog.Info("start to schedule one description...")
	defer klog.Info("finish schedule a description")
	key, shutdown := sched.parentSchedulingQueue.Get()
	if shutdown {
		klog.Error("failed to get next unscheduled description from closed queue")
		return
	}
	defer sched.parentSchedulingQueue.Done(key)

	// TODO: scheduling
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return
	}

	desc, err := sched.parentDescriptionLister.Descriptions(ns).Get(name)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	klog.InfoS("Attempting to schedule description", "description", klog.KObj(desc))

	// Synchronously attempt to find a fit for the description.
	start := time.Now()

	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	labelSelector := labels.NewSelector()
	requirement, _ := labels.NewRequirement(common.GaiaDescriptionLabel, selection.Equals, []string{desc.Name})
	labelSelector = labelSelector.Add(*requirement)
	// get rbs
	rbs, _ := sched.parentResourceBindingLister.List(labelSelector)
	// get mcls
	mcls, _ := sched.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if len(mcls.Items) == 0 {
		// there is no child clusters. no need to schedule just transfer.
		for _, item := range rbs {
			rb := &appsapi.ResourceBinding{}
			rb.Name = item.Name
			rb.Namespace = common.GaiaRSToBeMergedReservedNamespace
			rb.Labels = item.Labels
			rb.Spec = appsapi.ResourceBindingSpec{
				AppID:       desc.Name,
				ParentRB:    item.Spec.ParentRB,
				RbApps:      item.Spec.RbApps,
				TotalPeer:   item.Spec.TotalPeer,
				NetworkPath: item.Spec.NetworkPath,
			}
			_, errCreate := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).
				Create(ctx, rb, metav1.CreateOptions{})
			if errCreate != nil {
				klog.Infof("create rb in local to be merged ns error", errCreate)
			}
		}
		klog.V(3).InfoS("scheduler success just change rb namespace.")
		err := sched.parentGaiaClient.AppsV1alpha1().ResourceBindings(sched.dedicatedNamespace).
			DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
				common.GaiaDescriptionLabel: desc.Name,
			}).String()})
		if err != nil {
			klog.Infof("faild to delete rbs in parent cluster", err)
		}
	} else {
		scheduleResult, err := sched.scheduleAlgorithm.Schedule(schedulingCycleCtx, sched.framework, rbs, desc)
		if err != nil {
			sched.recordParentSchedulingFailure(desc, err, ReasonUnschedulable)
			var lastError error
			err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
				newDesc, _ := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
				if appsapi.DescriptionPhaseFailure == newDesc.Status.Phase {
					lastError = errors.New("description status phase is already 'Failure', there is no need to update it.")
					return true, nil
				}

				desc.Status.Phase = appsapi.DescriptionPhaseFailure
				// check if failed
				_, lastError := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).UpdateStatus(ctx, desc, metav1.UpdateOptions{})
				if lastError == nil {
					return true, nil
				}
				if apierrors.IsConflict(lastError) {
					newDesc, lastError := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
					if lastError == nil {
						desc = newDesc
					}
				}
				return false, nil
			})
			if err != nil {
				klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ", desc.Namespace, desc.Name, lastError)
			}
			klog.Warningf("scheduler failed %v", err)
			return
		}

		for _, itemCluster := range mcls.Items {
			for rbIndex, itemRb := range scheduleResult.ResourceBindings {
				itemRb.Namespace = itemCluster.Namespace
				itemRb.Name = fmt.Sprintf("%s-rs-%d", desc.Name, rbIndex)
				// itemRb.Spec.TotalPeer = len(scheduleResult.ResourceBindings)
				rb, err := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(itemCluster.Namespace).
					Create(ctx, itemRb, metav1.CreateOptions{})
				if err != nil {
					klog.V(3).InfoS("scheduler success, but some rb not created success", rb)
				}
			}
			// 2. create desc in to child cluster namespace
			newDesc := utils.ConstructDescriptionFromExistOne(desc)
			newDesc.Namespace = itemCluster.Namespace
			_, err := sched.localGaiaClient.AppsV1alpha1().Descriptions(itemCluster.Namespace).Create(ctx, newDesc, metav1.CreateOptions{})
			if err != nil {
				klog.V(3).InfoS("scheduler success, but desc not created success in sub child cluster.", err)
			}
		}
	}

	sched.parentGaiaClient.AppsV1alpha1().ResourceBindings(sched.dedicatedNamespace).
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
			common.GaiaDescriptionLabel: desc.Name,
		}).String()})
	var lastError error
	err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
		newDesc, _ := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
		if appsapi.DescriptionPhaseScheduled == newDesc.Status.Phase {
			lastError = errors.New("description status phase is already 'Scheduled', there is no need to update it.")
			return true, nil
		}

		desc.Status.Phase = appsapi.DescriptionPhaseScheduled
		// check if failed
		_, lastError := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).UpdateStatus(ctx, desc, metav1.UpdateOptions{})
		if lastError == nil {
			return true, nil
		}
		if apierrors.IsConflict(lastError) {
			newDesc, lastError := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
			if lastError == nil {
				desc = newDesc
			}
		}
		return false, nil
	})
	if err != nil {
		klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ", desc.Namespace, desc.Name, lastError)
	}
	metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
	metrics.DescriptionScheduled(sched.framework.ProfileName(), metrics.SinceInSeconds(start))
	klog.Info("scheduler success")
}

// RunParentReScheduler run reschedule in agent cluster.
func (sched *Scheduler) RunParentReScheduler(ctx context.Context) {
	klog.Info("start to re schedule one description...")
	defer klog.Info("finish re schedule a description")
	key, shutdown := sched.parentSchedulingRetryQueue.Get()
	if shutdown {
		klog.Error("failed to get next unscheduled description from closed retry queue")
		return
	}
	defer sched.parentSchedulingRetryQueue.Done(key)

	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(key.(string))
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return
	}

	desc, err := sched.parentDescriptionLister.Descriptions(ns).Get(name)
	if err != nil {
		utilruntime.HandleError(err)
		return
	}
	klog.InfoS("Attempting to re schedule description", "description", klog.KObj(desc))

	// Synchronously attempt to find a fit for the description.
	start := time.Now()

	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	labelSelector := labels.NewSelector()
	requirement, _ := labels.NewRequirement(common.GaiaDescriptionLabel, selection.Equals, []string{desc.Name})
	labelSelector = labelSelector.Add(*requirement)
	// get rbs we need only one that is the selected one.
	rbList, _ := sched.parentGaiaClient.AppsV1alpha1().ResourceBindings(known.GaiaRBMergedReservedNamespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	})
	rbs := make([]*appsapi.ResourceBinding, 0)
	rbs = append(rbs, &rbList.Items[0])

	mcls, _ := sched.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(ctx, metav1.ListOptions{})
	if len(mcls.Items) > 0 {
		scheduleResult, err := sched.scheduleAlgorithm.Schedule(schedulingCycleCtx, sched.framework, rbs, desc)
		if err != nil {
			sched.recordParentReSchedulingFailure(desc, err, ReasonUnschedulable)
			return
		}
		localRB, err := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(known.GaiaRBMergedReservedNamespace).List(ctx, metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
		if err != nil {
			sched.recordParentReSchedulingFailure(desc, err, ReasonUnschedulable)
			return
		}
		for _, rbapp := range scheduleResult.ResourceBindings[0].Spec.RbApps {
			rbItemApp := rbapp.DeepCopy()
			if rbItemApp.ClusterName == sched.selfClusterName {
				localRB.Items[0].Spec.RbApps = rbItemApp.Children
				rb, err := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(known.GaiaRBMergedReservedNamespace).
					Update(ctx, &localRB.Items[0], metav1.UpdateOptions{})
				if err != nil {
					klog.InfoS("scheduler success, but rb not update success", rb)
				}
				break
			}
		}
	}

	var lastError error
	err = wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
		newDesc, _ := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
		if appsapi.DescriptionPhaseScheduled == newDesc.Status.Phase {
			lastError = errors.New("description status phase is already 'Scheduled', there is no need to update it.")
			return true, nil
		}

		desc.Status.Phase = appsapi.DescriptionPhaseScheduled
		// check if failed
		_, lastError := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).UpdateStatus(ctx, desc, metav1.UpdateOptions{})
		if lastError == nil {
			return true, nil
		}
		if apierrors.IsConflict(lastError) {
			newDesc, lastError := sched.parentGaiaClient.AppsV1alpha1().Descriptions(sched.dedicatedNamespace).Get(ctx, desc.Name, metav1.GetOptions{})
			if lastError == nil {
				desc = newDesc
			}
		}
		return false, nil
	})
	if err != nil {
		klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ", desc.Namespace, desc.Name, lastError)
	}
	metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
	metrics.DescriptionScheduled(sched.framework.ProfileName(), metrics.SinceInSeconds(start))
	klog.Info("scheduler success")
}

func (sched *Scheduler) SetparentDedicatedConfig(ctx context.Context) {
	// complete your controller loop here
	klog.Info("start set parent DedicatedKubeConfig current cluster as a child cluster...")
	// wait until stopCh is closed or request is approved
	waitingCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	wait.JitterUntilWithContext(waitingCtx, func(ctx context.Context) {
		target, err := sched.localGaiaClient.PlatformV1alpha1().Targets().Get(ctx, common.ParentClusterTargetName, metav1.GetOptions{})
		if err != nil {
			// klog.Errorf("set parentDedicatedKubeConfig failed to get targets: %v wait for next loop", err)
			return
		}
		secret, err := sched.childKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Get(ctx, common.ParentClusterSecretName, metav1.GetOptions{})
		if err != nil {
			// klog.Errorf("set parentDedicatedKubeConfig failed to get secretFromParentCluster: %v", err)
			return
		}
		if err == nil {
			klog.Infof("found existing secretFromParentCluster '%s/%s' that can be used to access parent cluster", common.GaiaSystemNamespace, common.ParentClusterSecretName)

			parentDedicatedKubeConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL, string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey], 2)
			if err == nil {
				sched.parentDedicatedKubeConfig = parentDedicatedKubeConfig
				sched.dedicatedNamespace = string(secret.Data[corev1.ServiceAccountNamespaceKey])
				sched.selfClusterName = secret.Labels[common.ClusterNameLabel]
				sched.parentGaiaClient = gaiaClientSet.NewForConfigOrDie(sched.parentDedicatedKubeConfig)
				sched.parentInformerFactory = gaiainformers.NewSharedInformerFactoryWithOptions(
					sched.parentGaiaClient, known.DefaultResync, gaiainformers.WithNamespace(sched.dedicatedNamespace))
				sched.parentDescriptionLister = sched.parentInformerFactory.Apps().V1alpha1().Descriptions().Lister()
				sched.parentResourceBindingLister = sched.parentInformerFactory.Apps().V1alpha1().ResourceBindings().Lister()
				sched.scheduleAlgorithm.SetSelfClusterName(sched.selfClusterName)
				sched.addParentAllEventHandlers()
			} else {
				klog.Errorf("set parentkubeconfig failed to get sa and secretFromParentCluster: %v", err)
				return
			}
		}
		cancel()
	}, known.DefaultRetryPeriod*4, 0.3, true)

}

func (sched *Scheduler) GetparentDedicatedKubeConfig() *rest.Config {
	// complete your controller loop here
	klog.Info(" get parent DedicatedKubeConfig current cluster as a child cluster...")
	fmt.Printf("sched.parentDedicatedKubeConfig host == %s \n", sched.parentDedicatedKubeConfig.Host)
	return sched.parentDedicatedKubeConfig
}
func (sched *Scheduler) GetDedicatedNamespace() string {
	// complete your controller loop here
	fmt.Printf("sched.GetDedicatedNamespace == %s \n", sched.dedicatedNamespace)
	return sched.dedicatedNamespace
}

// recordSchedulingFailure records an event for the subscription that indicates the
// subscription has failed to schedule. Also, update the subscription condition.
func (sched *Scheduler) recordSchedulingFailure(sub *appsapi.Description, err error, _ string) {
	klog.V(2).InfoS("Unable to schedule subscription; waiting", "subscription", klog.KObj(sub), "err", err)

	msg := truncateMessage(err.Error())
	sched.framework.EventRecorder().Event(sub, corev1.EventTypeWarning, "FailedScheduling", msg)

	// re-added to the queue for re-processing
	sched.localSchedulingQueue.AddRateLimited(klog.KObj(sub).String())
}

// recordSchedulingFailure records an event for the subscription that indicates the
// Description has failed to schedule. Also, update the subscription condition.
func (sched *Scheduler) recordParentSchedulingFailure(sub *appsapi.Description, err error, _ string) {
	klog.V(2).InfoS("Unable to schedule Description; waiting", "Description", klog.KObj(sub), "err", err)

	msg := truncateMessage(err.Error())
	sched.framework.EventRecorder().Event(sub, corev1.EventTypeWarning, "FailedScheduling", msg)
	// re-added to the queue for re-processing
	sched.parentSchedulingQueue.AddRateLimited(klog.KObj(sub).String())
}

// recordParentReSchedulingFailure records an event for the description that indicates the
// Description has failed to re schedule. Also, update the description condition.
func (sched *Scheduler) recordParentReSchedulingFailure(sub *appsapi.Description, err error, reason string) {
	klog.V(2).InfoS("Unable to re schedule Description; waiting", "Description", klog.KObj(sub), "err", err)

	msg := truncateMessage(err.Error())
	sched.framework.EventRecorder().Event(sub, corev1.EventTypeWarning, reason, msg)
	// re-added to the queue for re-processing
	sched.parentSchedulingRetryQueue.AddRateLimited(klog.KObj(sub).String())
}

// addLocalAllEventHandlers is a helper function used in Scheduler
// to add event handlers for various local informers.
func (sched *Scheduler) addLocalAllEventHandlers() {
	sched.localNamespacedInformerFactory.Apps().V1alpha1().Descriptions().Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *appsapi.Description:
				desc := obj.(*appsapi.Description)
				if desc.DeletionTimestamp != nil {
					sched.lockLocal.Lock()
					defer sched.lockLocal.Unlock()
					return false
				}
				if len(desc.Status.Phase) == 0 || desc.Status.Phase == appsapi.DescriptionPhasePending || desc.Status.Phase == appsapi.DescriptionPhaseFailure {
					// TODO: filter scheduler name
					return true
				} else {
					sched.lockParent.Lock()
					defer sched.lockParent.Unlock()
					return false
				}
			case cache.DeletedFinalStateUnknown:
				if _, ok := t.Obj.(*appsapi.Description); ok {
					return true
				}
				utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *Description in %T", obj, sched))
				return false
			default:
				utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", sched, obj))
				return false
			}
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sub := obj.(*appsapi.Description)
				sched.lockLocal.Lock()
				defer sched.lockLocal.Unlock()
				sched.localSchedulingQueue.AddRateLimited(klog.KObj(sub).String())
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldSub := oldObj.(*appsapi.Description)
				newSub := newObj.(*appsapi.Description)

				// Decide whether discovery has reported a spec change.
				if reflect.DeepEqual(oldSub.Spec, newSub.Spec) {
					klog.V(4).Infof("no updates on the spec of Description %s, skipping syncing", klog.KObj(oldSub))
					return
				}

				sched.lockLocal.Lock()
				defer sched.lockLocal.Unlock()
				sched.localSchedulingQueue.AddRateLimited(klog.KObj(newSub).String())
			},
		},
	})

}

// addParentAllEventHandlers is a helper function used in Scheduler
// to add event handlers for various parent informers.
func (sched *Scheduler) addParentAllEventHandlers() {
	sched.parentInformerFactory.Apps().V1alpha1().Descriptions().Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *appsapi.Description:
				desc := obj.(*appsapi.Description)
				if desc.DeletionTimestamp != nil {
					sched.lockParent.Lock()
					defer sched.lockParent.Unlock()
					return false
				}
				if len(desc.Status.Phase) == 0 || desc.Status.Phase == appsapi.DescriptionPhasePending {
					// TODO: filter scheduler name
					return true
				} else {
					sched.lockParent.Lock()
					defer sched.lockParent.Unlock()
					return false
				}

			case cache.DeletedFinalStateUnknown:
				if _, ok := t.Obj.(*appsapi.Description); ok {
					return true
				}
				utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *Description in %T", obj, sched))
				return false
			default:
				utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", sched, obj))
				return false
			}
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sub := obj.(*appsapi.Description)
				sched.lockParent.Lock()
				defer sched.lockParent.Unlock()
				sched.parentSchedulingQueue.AddRateLimited(klog.KObj(sub).String())
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldDesc := oldObj.(*appsapi.Description)
				newDesc := newObj.(*appsapi.Description)

				// Decide whether discovery has reported a spec change.
				if reflect.DeepEqual(oldDesc.Spec, newDesc.Spec) {
					klog.V(4).Infof("no updates on the spec of Description %s, skipping syncing", klog.KObj(oldDesc))
					return
				}
				sched.lockParent.Lock()
				defer sched.lockParent.Unlock()
				sched.parentSchedulingQueue.AddRateLimited(klog.KObj(newDesc).String())
			},
		},
	})

	// config reschedule handlers.
	sched.parentInformerFactory.Apps().V1alpha1().Descriptions().Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			switch t := obj.(type) {
			case *appsapi.Description:
				desc := obj.(*appsapi.Description)
				if desc.DeletionTimestamp != nil {
					sched.lockReschedule.Lock()
					defer sched.lockReschedule.Unlock()
					return false
				}
				if desc.Status.Phase == appsapi.DescriptionPhaseReSchedule {
					return true
				} else {
					sched.lockReschedule.Lock()
					defer sched.lockReschedule.Unlock()
					return false
				}

			case cache.DeletedFinalStateUnknown:
				if _, ok := t.Obj.(*appsapi.Description); ok {
					return true
				}
				utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *Description in %T", obj, sched))
				return false
			default:
				utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", sched, obj))
				return false
			}
		},
		Handler: cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				sub := obj.(*appsapi.Description)
				sched.lockReschedule.Lock()
				defer sched.lockReschedule.Unlock()
				sched.parentSchedulingRetryQueue.AddRateLimited(klog.KObj(sub).String())
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				oldDesc := oldObj.(*appsapi.Description)
				newDesc := newObj.(*appsapi.Description)

				// Decide whether discovery has reported a spec change.
				if reflect.DeepEqual(oldDesc.Spec, newDesc.Spec) {
					klog.V(4).Infof("no updates on the spec of Description %s, skipping syncing", klog.KObj(oldDesc))
					return
				}
				sched.lockReschedule.Lock()
				defer sched.lockReschedule.Unlock()
				sched.parentSchedulingRetryQueue.AddRateLimited(klog.KObj(newDesc).String())
			},
		},
	})
}

// truncateMessage truncates a message if it hits the NoteLengthLimit.
// copied from k8s.io/kubernetes/pkg/scheduler/scheduler.go
func truncateMessage(message string) string {
	max := known.NoteLengthLimit
	if len(message) <= max {
		return message
	}
	suffix := " ..."
	return message[:max-len(suffix)] + suffix
}

func getTotal(spec, lenResult int) int {
	if spec == 0 {
		return lenResult
	}
	return spec
}
