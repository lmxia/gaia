package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	rest "k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/component-base/metrics/legacyregistry"
	"k8s.io/klog/v2"

	schedulerserverconfig "github.com/lmxia/gaia/cmd/gaia-scheduler/app/config"
	"github.com/lmxia/gaia/cmd/gaia-scheduler/app/option"
	appsapi "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	platformapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	listner "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/algorithm"
	schedulercache "github.com/lmxia/gaia/pkg/scheduler/cache"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins"
	frameworkruntime "github.com/lmxia/gaia/pkg/scheduler/framework/runtime"
	"github.com/lmxia/gaia/pkg/scheduler/metrics"
	"github.com/lmxia/gaia/pkg/scheduler/parallelize"
	"github.com/lmxia/gaia/pkg/utils"
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
	localGaiaClient                *gaiaClientSet.Clientset
	localNamespacedInformerFactory gaiainformers.SharedInformerFactory // namespaced
	localGaiaAllFactory            gaiainformers.SharedInformerFactory // all ns
	localDescLister                listner.DescriptionLister
	selfClusterName                string // this cluster name

	// dedicated kubeconfig for accessing parent cluster, which is auto populated by the parent
	// cluster when cluster registration request gets approved
	parentDedicatedKubeConfig   *rest.Config
	parentInformerFactory       gaiainformers.SharedInformerFactory // namespaced
	parentDescriptionLister     listner.DescriptionLister
	parentResourceBindingLister listner.ResourceBindingLister
	parentGaiaClient            *gaiaClientSet.Clientset

	// clientset for child cluster
	childKubeClientSet kubernetes.Interface

	dedicatedNamespace string
	Identity           string
	// default in-tree registry
	registry frameworkruntime.Registry

	scheduleAlgorithm algorithm.ScheduleAlgorithm

	// localSchedulingQueue holds description in local namespace to be scheduled
	localSchedulingQueue workqueue.RateLimitingInterface

	// parentSchedulingQueue holds description in parent cluster namespace to be scheduled
	parentSchedulingQueue workqueue.RateLimitingInterface

	// parentSchedulingRetryQueue holds description in parent cluster namespace to be rescheduled
	parentSchedulingRetryQueue workqueue.RateLimitingInterface

	framework framework.Framework

	lockLocal      sync.RWMutex
	lockParent     sync.RWMutex
	lockReschedule sync.RWMutex
}

// NewSchedule returns a new Scheduler.
func NewSchedule(ctx context.Context, childKubeConfigFile string, opts *option.Options) (
	*schedulerserverconfig.CompletedConfig, *Scheduler, error) {
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

	localAllGaiaInformerFactory := gaiainformers.NewSharedInformerFactory(childGaiaClientSet, common.DefaultResync)
	localSuperKubeConfig := NewLocalSuperKubeConfig(ctx, childKubeConfig.Host, childKubeClientSet)

	// create event recorder
	broadcaster := record.NewBroadcaster()
	broadcaster.StartRecordingToSink(&v1core.EventSinkImpl{
		Interface: childKubeClientSet.CoreV1().Events(""),
	})
	utilruntime.Must(appsapi.AddToScheme(scheme.Scheme))
	utilruntime.Must(platformapi.AddToScheme(scheme.Scheme))
	recorder := broadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: "gaia-scheduler"})

	schedulerCache := schedulercache.New(localAllGaiaInformerFactory.Platform().V1alpha1().ManagedClusters().Lister(),
		childGaiaClientSet)
	if err != nil {
		return nil, nil, err
	}

	sched := &Scheduler{
		localGaiaClient:     childGaiaClientSet,
		localGaiaAllFactory: localAllGaiaInformerFactory,
		localDescLister:     localAllGaiaInformerFactory.Apps().V1alpha1().Descriptions().Lister(),
		childKubeClientSet:  childKubeClientSet,

		// dynamicClient:              dynamicClient,
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
	sched.localNamespacedInformerFactory = gaiainformers.NewSharedInformerFactoryWithOptions(childGaiaClientSet,
		common.DefaultResync, gaiainformers.WithNamespace(common.GaiaReservedNamespace))
	// local event handler
	sched.addLocalAllEventHandlers()

	metrics.Register()

	return &cc, sched, nil
}

func (sched *Scheduler) Run(cxt context.Context, cc *schedulerserverconfig.CompletedConfig) {
	klog.Info("starting gaia schedule scheduler ...")
	defer sched.localSchedulingQueue.ShutDown()

	// start the leader election code loop
	leaderelection.RunOrDie(context.TODO(), *newLeaderElectionConfigWithDefaultValue(sched.Identity,
		sched.childKubeClientSet, leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// 1. start local gaia generic informers
				sched.localGaiaAllFactory.Start(ctx.Done())
				sched.localGaiaAllFactory.WaitForCacheSync(ctx.Done())

				// 2. start local scheduler.
				go func() {
					sched.localNamespacedInformerFactory.Start(ctx.Done())
					sched.localNamespacedInformerFactory.WaitForCacheSync(ctx.Done())
					wait.UntilWithContext(ctx, sched.RunLocalScheduler, 0)
				}()

				// metrics
				if cc.SecureServing != nil {
					handler := buildHandlerChain(newMetricsHandler(), cc.Authentication.Authenticator,
						cc.Authorization.Authorizer)
					klog.Info("Starting gaia-scheduler metrics server...")
					if _, err := cc.SecureServing.Serve(handler, 0, ctx.Done()); err != nil {
						klog.Infof("failed to start metrics server: %v", err)
					}
				}

				// 3. when we get add parentDedicatedKubeConfig add parent desc scheduler and start it.
				sched.SetparentDedicatedConfig(ctx)
				sched.parentInformerFactory.Start(ctx.Done())
				sched.parentInformerFactory.WaitForCacheSync(ctx.Done())
				go func() {
					wait.UntilWithContext(ctx, sched.RunParentReScheduler, 0)
				}()
				wait.UntilWithContext(ctx, sched.RunParentScheduler, 0)
			},
			OnStoppedLeading: func() {
				klog.Error("leader election got lost")
			},
			OnNewLeader: func(identity string) {
				// we're notified when new leader elected
				if identity == sched.Identity {
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

func newLeaderElectionConfigWithDefaultValue(identity string, clientset kubernetes.Interface,
	callbacks leaderelection.LeaderCallbacks) *leaderelection.LeaderElectionConfig {
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
	retryCtx, retryCancel := context.WithTimeout(ctx, common.DefaultRetryPeriod)
	defer retryCancel()

	// get high priority secret.
	secret := utils.GetDeployerCredentials(retryCtx, kubeClient, common.GaiaAppSA)
	var clusterStatusKubeConfig *rest.Config
	if secret != nil {
		var err error
		clusterStatusKubeConfig, err = utils.GenerateKubeConfigFromToken(apiserverURL,
			string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey], 2)
		if err != nil {
			klog.Info("can't get local super Auth")
			return nil
			// kubeClient = kubernetes.NewForConfigOrDie(clusterStatusKubeConfig)
		}
	}
	return clusterStatusKubeConfig
}

func (sched *Scheduler) RunLocalScheduler(ctx context.Context) {
	var err error
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
		sched.localSchedulingQueue.AddRateLimited(klog.KRef(ns, name).String())
		utilruntime.HandleError(err)
		return
	}
	klog.InfoS("Attempting to schedule description", "description", klog.KObj(desc))

	// Synchronously attempt to find a fit for the description.
	start := time.Now()

	schedulingCycleCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// schedule frontend apps, only one supplier
	frontendRbs, errS := sched.scheduleFrontend(desc)
	if errS != nil {
		sched.recordSchedulingFailure(desc, errS, ReasonUnschedulable)
		desc.Status.Phase = appsapi.DescriptionPhaseFailure
		desc.Status.Reason = truncateMessage(errS.Error())
		err = utils.UpdateDescriptionStatus(sched.localGaiaClient, desc)
		if err != nil {
			klog.Errorf("failed to update status of description's status phase: %v/%v, err is %v",
				desc.Namespace, desc.Name, err)
		}
		klog.Errorf("scheduler failed %v", errS)
		return
	}
	if len(desc.Spec.WorkloadComponents) == 0 {
		errF := sched.createFrontRBLocal(desc, frontendRbs)
		if errF != nil {
			return
		} else {
			metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
			metrics.DescriptionScheduled(sched.framework.ProfileName(), metrics.SinceInSeconds(start))
			sched.localSchedulingQueue.Forget(key)
			klog.Infof("scheduler success, Description=%q", desc.Name)
		}
		return
	}

	scheduleResult, err := sched.scheduleAlgorithm.Schedule(schedulingCycleCtx, sched.framework, nil, desc)
	if err != nil {
		sched.recordSchedulingFailure(desc, err, ReasonUnschedulable)
		desc.Status.Phase = appsapi.DescriptionPhaseFailure
		desc.Status.Reason = truncateMessage(err.Error())
		err = utils.UpdateDescriptionStatus(sched.localGaiaClient, desc)
		if err != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ",
				desc.Namespace, desc.Name, err)
		}
		klog.Warningf("scheduler failed %v", err)
		return
	}
	if len(frontendRbs) != 0 {
		for _, itemRb := range scheduleResult.ResourceBindings {
			itemRb.Spec.FrontendRbs = frontendRbs
		}
	}

	mcls, _ := sched.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(ctx,
		metav1.ListOptions{})
	if len(mcls.Items) == 0 {
		klog.Warningf("scheduler success but do nothing because there is no child clusters.")
	} else {
		// 1. create rbs in sub children cluster namespace.
		for _, itemCluster := range mcls.Items {
			skipDescCreate := true
			for rbIndex, itemRb := range scheduleResult.ResourceBindings {
				if has := hasReplicasInCluster(itemRb.Spec.RbApps, itemCluster.Name); has {
					skipDescCreate = false
				} else {
					continue
				}

				itemRb.Name = fmt.Sprintf("%s-rs-%d", desc.Name, rbIndex)
				itemRb.Namespace = itemCluster.Namespace
				itemRb.Spec.TotalPeer = getTotal(itemRb.Spec.TotalPeer, len(scheduleResult.ResourceBindings))
				itemRb.Spec.NonZeroClusterNum = countNonZeroClusterNumforRB(itemRb)
				_, err2 := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(itemCluster.Namespace).
					Create(ctx, itemRb, metav1.CreateOptions{})
				if err2 != nil {
					klog.Infof("scheduler success, but some rb not created success %v", err2)
				} else {
					klog.InfoS("successfully created rb", "Description", desc.GetName(),
						"ResourceBinding", klog.KRef(itemRb.GetNamespace(), itemRb.GetName()))
				}
			}
			// 2. create desc in to child cluster namespace
			if skipDescCreate {
				continue
			}
			newDesc := utils.ConstructDescriptionFromExistOne(desc)
			newDesc.Namespace = itemCluster.Namespace
			_, err2 := sched.localGaiaClient.AppsV1alpha1().Descriptions(itemCluster.Namespace).Create(ctx, newDesc,
				metav1.CreateOptions{})
			if err2 != nil {
				klog.InfoS("scheduler success, but desc not created success in sub child cluster.", err2)
			}
		}
		desc.Status.Phase = appsapi.DescriptionPhaseScheduled
		err = utils.UpdateDescriptionStatus(sched.localGaiaClient, desc)
		if err != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ",
				desc.Namespace, desc.Name, err)
		}
		metrics.SchedulingAlgorithmLatency.Observe(metrics.SinceInSeconds(start))
		metrics.DescriptionScheduled(sched.framework.ProfileName(), metrics.SinceInSeconds(start))
		sched.localSchedulingQueue.Forget(key)
		klog.Infof("scheduler success %v", scheduleResult)
	}
}

func (sched *Scheduler) RunParentScheduler(ctx context.Context) {
	klog.Info("start to schedule one description from parent cluster...")
	defer klog.Info("finish schedule a description from parent cluster.")
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
	mcls, _ := sched.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(ctx,
		metav1.ListOptions{})
	if len(mcls.Items) == 0 {
		// there is no child clusters. no need to schedule just transfer.
		transferRB(sched.parentGaiaClient, sched.localGaiaClient, sched.dedicatedNamespace,
			common.GaiaRSToBeMergedReservedNamespace, desc.Name, rbs, ctx)
		klog.Info("scheduler success just change rb namespace.")
	} else {
		scheduleResult, err2 := sched.scheduleAlgorithm.Schedule(schedulingCycleCtx, sched.framework, rbs, desc)
		if err2 != nil {
			sched.recordParentSchedulingFailure(desc, err2, ReasonUnschedulable)
			desc.Status.Phase = appsapi.DescriptionPhaseFailure
			desc.Status.Reason = truncateMessage(err2.Error())
			err2 = utils.UpdateDescriptionStatus(sched.parentGaiaClient, desc)
			if err2 != nil {
				klog.Info("failed to update status of description's status phase: %v/%v, "+
					"err is ", desc.Namespace, desc.Name, err2)
			}
			klog.Warningf("scheduler failed %v", err2)
			return
		}

		// only one time.
		transferRB(nil, sched.localGaiaClient, sched.dedicatedNamespace,
			common.GaiaRSToBeMergedReservedNamespace, desc.Name, scheduleResult.ResourceBindings, ctx)
		for _, itemCluster := range mcls.Items {
			skipDescCreate := true
			for _, itemRb := range scheduleResult.ResourceBindings {
				if has := hasReplicasInCluster(itemRb.Spec.RbApps, itemCluster.Name); has {
					skipDescCreate = false
					break
				}
			}
			if skipDescCreate {
				continue
			}
			// 2. create desc in to child cluster namespace
			newDesc := utils.ConstructDescriptionFromExistOne(desc)
			newDesc.Namespace = itemCluster.Namespace
			_, err3 := sched.localGaiaClient.AppsV1alpha1().Descriptions(itemCluster.Namespace).Create(ctx,
				newDesc, metav1.CreateOptions{})
			if err3 != nil {
				klog.InfoS("scheduler success, but desc not created success in sub child cluster.", err3)
			}
		}
	}

	err = sched.parentGaiaClient.AppsV1alpha1().ResourceBindings(sched.dedicatedNamespace).
		DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				common.GaiaDescriptionLabel: desc.Name,
			}).String()})
	klog.Info("i have tried to delete rbs in parent cluster")
	if err != nil {
		klog.Errorf("failed to delete rbs in parent namespace %s, related to desc %s, err: %v", desc.Namespace,
			desc.Name, err)
	}

	desc.Status.Phase = appsapi.DescriptionPhaseScheduled
	err = utils.UpdateDescriptionStatus(sched.parentGaiaClient, desc)
	if err != nil {
		klog.Errorf("failed to update status of description's status phase: %v/%v, err is %v", desc.Namespace,
			desc.Name, err)
	}
	sched.parentSchedulingQueue.Forget(key)
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
	rbList, _ := sched.parentGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).List(ctx,
		metav1.ListOptions{
			LabelSelector: labelSelector.String(),
		})
	rbs := make([]*appsapi.ResourceBinding, 0)
	rbs = append(rbs, &rbList.Items[0])

	mcls, _ := sched.localGaiaClient.PlatformV1alpha1().ManagedClusters(corev1.NamespaceAll).List(ctx,
		metav1.ListOptions{})
	if len(mcls.Items) > 0 {
		scheduleResult, err2 := sched.scheduleAlgorithm.Schedule(schedulingCycleCtx, sched.framework, rbs, desc)
		if err2 != nil {
			sched.recordParentReSchedulingFailure(desc, err2, ReasonUnschedulable)
			return
		}
		localRB, err2 := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).
			List(ctx, metav1.ListOptions{
				LabelSelector: labelSelector.String(),
			})
		if err2 != nil {
			sched.recordParentReSchedulingFailure(desc, err2, ReasonUnschedulable)
			return
		}
		for _, rbapp := range scheduleResult.ResourceBindings[0].Spec.RbApps {
			rbItemApp := rbapp.DeepCopy()
			if rbItemApp.ClusterName == sched.selfClusterName {
				localRB.Items[0].Spec.RbApps = rbItemApp.Children
				rb, err3 := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).
					Update(ctx, &localRB.Items[0], metav1.UpdateOptions{})
				if err3 != nil {
					klog.InfoS("scheduler success, but rb not update success", rb)
				}
				break
			}
		}
	}

	desc.Status.Phase = appsapi.DescriptionPhaseScheduled
	err = utils.UpdateDescriptionStatus(sched.parentGaiaClient, desc)
	if err != nil {
		klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ",
			desc.Namespace, desc.Name, err)
	}
	sched.parentSchedulingRetryQueue.Forget(key)
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
		target, err := sched.localGaiaClient.PlatformV1alpha1().Targets().Get(ctx, common.ParentClusterTargetName,
			metav1.GetOptions{})
		if err != nil {
			// klog.Errorf("set parentDedicatedKubeConfig failed to get targets: %v wait for next loop", err)
			return
		}
		secret, err := sched.childKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Get(ctx,
			common.ParentClusterSecretName, metav1.GetOptions{})
		if err != nil {
			// klog.Errorf("set parentDedicatedKubeConfig failed to get secretFromParentCluster: %v", err)
			return
		}
		if err == nil {
			klog.Infof("found existing secretFromParentCluster '%s/%s' that can be used to access parent "+
				"cluster", common.GaiaSystemNamespace, common.ParentClusterSecretName)

			parentDedicatedKubeConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL,
				string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey],
				2)
			if err == nil {
				sched.parentDedicatedKubeConfig = parentDedicatedKubeConfig
				sched.dedicatedNamespace = string(secret.Data[corev1.ServiceAccountNamespaceKey])
				sched.selfClusterName = secret.Labels[common.ClusterNameLabel]
				sched.parentGaiaClient = gaiaClientSet.NewForConfigOrDie(sched.parentDedicatedKubeConfig)
				sched.parentInformerFactory = gaiainformers.NewSharedInformerFactoryWithOptions(
					sched.parentGaiaClient, common.DefaultResync, gaiainformers.WithNamespace(sched.dedicatedNamespace))
				sched.parentDescriptionLister = sched.parentInformerFactory.Apps().V1alpha1().Descriptions().Lister()
				sched.parentResourceBindingLister = sched.parentInformerFactory.Apps().V1alpha1().
					ResourceBindings().Lister()
				sched.scheduleAlgorithm.SetSelfClusterName(sched.selfClusterName)
				sched.addParentAllEventHandlers()
			} else {
				klog.Errorf("set parentkubeconfig failed to get sa and secretFromParentCluster: %v", err)
				return
			}
		}
		cancel()
	}, common.DefaultRetryPeriod*4, 0.3, true)
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
	klog.V(2).InfoS("Unable to schedule subscription; waiting", "subscription",
		klog.KObj(sub), "err", err)

	msg := truncateMessage(err.Error())
	sched.framework.EventRecorder().Event(sub, corev1.EventTypeWarning, "FailedScheduling", msg)

	// re-added to the queue for re-processing
	sched.localSchedulingQueue.AddRateLimited(klog.KObj(sub).String())
}

// recordSchedulingFailure records an event for the subscription that indicates the
// Description has failed to schedule. Also, update the subscription condition.
func (sched *Scheduler) recordParentSchedulingFailure(sub *appsapi.Description, err error, _ string) {
	klog.V(2).InfoS("Unable to schedule Description; waiting", "Description", klog.KObj(sub),
		"err", err)

	msg := truncateMessage(err.Error())
	sched.framework.EventRecorder().Event(sub, corev1.EventTypeWarning, "FailedScheduling", msg)
	// re-added to the queue for re-processing
	sched.parentSchedulingQueue.AddRateLimited(klog.KObj(sub).String())
}

// recordParentReSchedulingFailure records an event for the description that indicates the
// Description has failed to re schedule. Also, update the description condition.
func (sched *Scheduler) recordParentReSchedulingFailure(sub *appsapi.Description, err error, reason string) {
	klog.V(2).InfoS("Unable to re schedule Description; waiting", "Description",
		klog.KObj(sub), "err", err)

	msg := truncateMessage(err.Error())
	sched.framework.EventRecorder().Event(sub, corev1.EventTypeWarning, reason, msg)
	// re-added to the queue for re-processing
	sched.parentSchedulingRetryQueue.AddRateLimited(klog.KObj(sub).String())
}

// addLocalAllEventHandlers is a helper function used in Scheduler
// to add event handlers for various local informers.
func (sched *Scheduler) addLocalAllEventHandlers() {
	sched.localNamespacedInformerFactory.Apps().V1alpha1().Descriptions().Informer().
		AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *appsapi.Description:
					desc := obj.(*appsapi.Description)
					if desc.DeletionTimestamp != nil {
						sched.lockLocal.Lock()
						defer sched.lockLocal.Unlock()
						sched.localSchedulingQueue.Done(klog.KObj(desc).String())
						sched.localSchedulingQueue.Forget(klog.KObj(desc).String())
						return false
					}
					if len(desc.Status.Phase) == 0 || desc.Status.Phase == appsapi.DescriptionPhasePending ||
						desc.Status.Phase == appsapi.DescriptionPhaseFailure {
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
					utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *Description in %T",
						obj, sched))
					return false
				default:
					utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", sched, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					desc := obj.(*appsapi.Description)
					sched.lockLocal.Lock()
					defer sched.lockLocal.Unlock()
					sched.localSchedulingQueue.AddRateLimited(klog.KObj(desc).String())
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					oldSub := oldObj.(*appsapi.Description)
					newSub := newObj.(*appsapi.Description)

					// Decide whether discovery has reported a spec change.
					// if reflect.DeepEqual(oldSub.Spec, newSub.Spec) {
					if true {
						klog.V(4).Infof("no updates on the spec of Description %s, skipping syncing",
							klog.KObj(oldSub))
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
	sched.parentInformerFactory.Apps().V1alpha1().Descriptions().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *appsapi.Description:
					desc := obj.(*appsapi.Description)
					if desc.DeletionTimestamp != nil {
						sched.lockParent.Lock()
						defer sched.lockParent.Unlock()
						sched.parentSchedulingQueue.Done(klog.KObj(desc).String())
						sched.parentSchedulingQueue.Forget(klog.KObj(desc).String())
						return false
					}
					if len(desc.Status.Phase) == 0 || desc.Status.Phase == appsapi.DescriptionPhasePending ||
						desc.Status.Phase == appsapi.DescriptionPhaseFailure {
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
					utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *Description in %T",
						obj, sched))
					return false
				default:
					utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", sched, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					desc := obj.(*appsapi.Description)
					sched.lockParent.Lock()
					defer sched.lockParent.Unlock()
					sched.parentSchedulingQueue.AddRateLimited(klog.KObj(desc).String())
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					oldDesc := oldObj.(*appsapi.Description)
					newDesc := newObj.(*appsapi.Description)

					// Decide whether discovery has reported a spec change.
					if reflect.DeepEqual(oldDesc.Spec, newDesc.Spec) {
						klog.V(4).Infof("no updates on the spec of Description %s, skipping syncing",
							klog.KObj(oldDesc))
						return
					}
					sched.lockParent.Lock()
					defer sched.lockParent.Unlock()
					sched.parentSchedulingQueue.AddRateLimited(klog.KObj(newDesc).String())
				},
			},
		})

	// config reschedule handlers.
	sched.parentInformerFactory.Apps().V1alpha1().Descriptions().Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: func(obj interface{}) bool {
				switch t := obj.(type) {
				case *appsapi.Description:
					desc := obj.(*appsapi.Description)
					if desc.DeletionTimestamp != nil {
						sched.lockReschedule.Lock()
						defer sched.lockReschedule.Unlock()
						sched.parentSchedulingRetryQueue.Forget(klog.KObj(desc).String())
						sched.parentSchedulingRetryQueue.Done(klog.KObj(desc).String())
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
					utilruntime.HandleError(fmt.Errorf("unable to convert object %T to *Description in %T",
						obj, sched))
					return false
				default:
					utilruntime.HandleError(fmt.Errorf("unable to handle object in %T: %T", sched, obj))
					return false
				}
			},
			Handler: cache.ResourceEventHandlerFuncs{
				AddFunc: func(obj interface{}) {
					desc := obj.(*appsapi.Description)
					sched.lockReschedule.Lock()
					defer sched.lockReschedule.Unlock()
					sched.parentSchedulingRetryQueue.AddRateLimited(klog.KObj(desc).String())
				},
				UpdateFunc: func(oldObj, newObj interface{}) {
					oldDesc := oldObj.(*appsapi.Description)
					newDesc := newObj.(*appsapi.Description)

					// Decide whether discovery has reported a spec change.
					if reflect.DeepEqual(oldDesc.Spec, newDesc.Spec) {
						klog.V(4).Infof("no updates on the spec of Description %s, skipping syncing",
							klog.KObj(oldDesc))
						return
					}
					sched.lockReschedule.Lock()
					defer sched.lockReschedule.Unlock()
					sched.parentSchedulingRetryQueue.AddRateLimited(klog.KObj(newDesc).String())
				},
			},
		})
}

func (sched *Scheduler) createFrontRBLocal(desc *appsapi.Description, rbs []*appsapi.FrontendRb) error {
	descName := desc.Name
	frontRB := &appsapi.ResourceBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("%s-rs", descName),
			Namespace: common.GaiaRSToBeMergedReservedNamespace,
			Labels: map[string]string{
				common.GaiaDescriptionLabel:            descName,
				common.OriginDescriptionNameLabel:      descName,
				common.OriginDescriptionNamespaceLabel: common.GaiaReservedNamespace,
				common.OriginDescriptionUIDLabel:       string(desc.GetUID()),
				common.UserNameLabel:                   desc.GetLabels()[common.UserNameLabel],
			},
		},
		Spec: appsapi.ResourceBindingSpec{
			AppID:           desc.Name,
			FrontendRbs:     rbs,
			StatusScheduler: appsapi.ResourceBindingMerging,
		},
	}
	frontRB.Kind = "ResourceBinding"
	frontRB.APIVersion = "apps.gaia.io/v1alpha1"

	_, err := sched.localGaiaClient.AppsV1alpha1().ResourceBindings(common.GaiaRSToBeMergedReservedNamespace).
		Create(context.TODO(), frontRB, metav1.CreateOptions{})
	if err != nil {
		klog.Errorf("frontendAPP scheduler success, but rb not created success %v", err)
		// update desc status
		sched.recordSchedulingFailure(desc, err, ReasonUnschedulable)
		desc.Status.Phase = appsapi.DescriptionPhaseFailure
		desc.Status.Reason = truncateMessage(err.Error())
		err2 := utils.UpdateDescriptionStatus(sched.localGaiaClient, desc)
		if err2 != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ",
				desc.Namespace, desc.Name, err2)
			return err2
		}
		return err
	} else {
		klog.InfoS("successfully created rb", "Description", descName,
			"ResourceBinding", klog.KObj(frontRB))

		desc.Status.Phase = appsapi.DescriptionPhaseScheduled
		err2 := utils.UpdateDescriptionStatus(sched.localGaiaClient, desc)
		if err2 != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, error==%v",
				desc.Namespace, desc.Name, err2)
			return err2
		}
	}

	return nil
}

func (sched *Scheduler) scheduleFrontend(desc *appsapi.Description) ([]*appsapi.FrontendRb, error) {
	frontendRbs := make([]*appsapi.FrontendRb, 0)
	var supplierName string
	if desc.Spec.IsFrontEnd {
		cdnSuppliers, err2 := sched.localGaiaClient.AppsV1alpha1().CdnSuppliers(common.GaiaFrontendNamespace).
			List(context.TODO(), metav1.ListOptions{})
		if err2 != nil {
			klog.Errorf("failed to get cdn supplier, error==%v", err2)
			return nil, err2
		}
		suppliers, errL := sched.localGaiaClient.AppsV1alpha1().CdnSuppliers(common.GaiaFrontendNamespace).List(
			context.TODO(), metav1.ListOptions{})
		if errL != nil || len(suppliers.Items) == 0 {
			klog.Errorf("failed to list CdnSuppliers or no CdnSupplier existed, Description=%q",
				klog.KObj(desc))
			return nil, fmt.Errorf("failed to list CdnSuppliers or no CdnSupplier existed, error=%v", errL)
		} else {
			// 仅获取第一个supplier
			supplierName = suppliers.Items[0].Name
			// for _, supplier := range suppliers.Items {
			// 	supplierName = supplier.Name
			// }
		}

		frontendRb := appsapi.FrontendRb{
			Suppliers: make([]*appsapi.Supplier, 0),
		}
		if len(cdnSuppliers.Items) == 0 {
			klog.Errorf("no cdn supplier, error==%v", err2)
			return nil, fmt.Errorf("no cdn supplier in cluster")
		} else if len(desc.Spec.FrontendComponents) != 0 {
			fCom2Rep := make(map[string]int32)
			// 暂时仅支持 多frontComponent, 单个supplier
			for _, fc := range desc.Spec.FrontendComponents {
				fCom2Rep[fc.ComponentName] = 1
			}
			supplier := appsapi.Supplier{
				SupplierName: supplierName,
				Replicas:     fCom2Rep,
			}
			frontendRb.Suppliers = append(frontendRb.Suppliers, &supplier)
		}
		frontendRbs = append(frontendRbs, &frontendRb)
	}
	return frontendRbs, nil
}

// truncateMessage truncates a message if it hits the NoteLengthLimit.
// copied from k8s.io/kubernetes/pkg/scheduler/scheduler.go
func truncateMessage(message string) string {
	max := common.NoteLengthLimit
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

// transfer from src to dst
func transferRB(srcClient, dstClient *gaiaClientSet.Clientset, srcNS, dstNS, descName string,
	rbs []*appsapi.ResourceBinding, ctx context.Context) {
	for _, item := range rbs {
		rb := &appsapi.ResourceBinding{}
		rb.Name = item.Name
		rb.Namespace = dstNS
		rb.Labels = item.Labels
		rb.Spec = appsapi.ResourceBindingSpec{
			AppID:             item.Spec.AppID,
			ParentRB:          item.Spec.ParentRB,
			FrontendRbs:       item.Spec.FrontendRbs,
			RbApps:            item.Spec.RbApps,
			TotalPeer:         item.Spec.TotalPeer,
			NonZeroClusterNum: item.Spec.NonZeroClusterNum,
			NetworkPath:       item.Spec.NetworkPath,
		}
		_, errCreate := dstClient.AppsV1alpha1().ResourceBindings(dstNS).
			Create(ctx, rb, metav1.CreateOptions{})
		if errCreate != nil {
			klog.Infof("create rb in local to be merged ns error", errCreate)
		} else {
			klog.InfoS("successfully created rb", "Description", descName, "ResourceBinding",
				klog.KRef(rb.GetNamespace(), rb.GetName()))
		}
	}
	if srcClient != nil {
		err := srcClient.AppsV1alpha1().ResourceBindings(srcNS).
			DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(
				labels.Set{
					common.GaiaDescriptionLabel: descName,
				}).String()})
		klog.Info("i have try to delete rbs in parent cluster")
		if err != nil {
			klog.Infof("faild to delete rbs in parent cluster", err)
		}
	}
}

// hasReplicasInCluster check if there is replicas in the cluster, return true if there is.
func hasReplicasInCluster(bindingApps []*appsapi.ResourceBindingApps, clusterName string) bool {
	for _, rbApp := range bindingApps {
		if rbApp.ClusterName == clusterName {
			for _, v := range rbApp.Replicas {
				if v > 0 {
					return true
				}
			}
			// maybe we can early stop once confirm false.
			return false
		}
		if rbApp.Children != nil && len(rbApp.Children) > 0 {
			if subContain := hasReplicasInCluster(rbApp.Children, clusterName); subContain {
				return true
			}
		}
	}
	return false
}

func countNonZeroClusterNumforRB(binding *appsapi.ResourceBinding) int {
	nonZeroCount := 0
	for _, rbApp := range binding.Spec.RbApps {
		for _, v := range rbApp.Replicas {
			if v > 0 {
				nonZeroCount += 1
				break
			}
		}

		if rbApp.Children != nil && len(rbApp.Children) > 0 {
			nonZeroCount = 0
			for _, child := range rbApp.Children {
				for _, v := range child.Replicas {
					if v > 0 {
						nonZeroCount += 1
						break
					}
				}
			}
		}
	}
	return nonZeroCount
}
