package scheduler

import (
	"context"
	"fmt"
	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	"github.com/lmxia/gaia/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/uuid"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	"os"
)

// Agent defines configuration for clusternet-agent
type ControllerManager struct {
	ctx context.Context

	// Identity is the unique string identifying a lease holder across
	// all participants in an election.
	Identity string

	// ClusterID denotes current child cluster id
	ClusterID *types.UID

	// clientset for child cluster
	childKubeClientSet kubernetes.Interface
	childGaiaClientSet *gaiaclientset.Clientset

	// dedicated kubeconfig for accessing parent cluster, which is auto populated by the parent cluster
	// when cluster registration request gets approved
	parentDedicatedKubeConfig *rest.Config
	// dedicated namespace in parent cluster for current child cluster
	DedicatedNamespace *string

	gaiaInformerFactory gaiainformers.SharedInformerFactory
	kubeInformerFactory kubeinformers.SharedInformerFactory
	scheduler           *Scheduler
	triggerFunc         func(metav1.Object)
}

// NewAgent returns a new Agent.
func NewScheduleControllerManager(ctx context.Context, childKubeConfigFile string) (*ControllerManager, error) {
	hostname, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("unable to get hostname: %v", err)
	}

	// add a uniquifier so that two processes on the same host don't accidentally both become active
	identity := hostname + "_" + string(uuid.NewUUID())
	klog.V(4).Infof("current identity lock id %q", identity)

	childKubeConfig, err := utils.LoadsKubeConfig(childKubeConfigFile, 1)
	if err != nil {
		return nil, err
	}
	// create clientset for child cluster
	childKubeClientSet := kubernetes.NewForConfigOrDie(childKubeConfig)
	childGaiaClientSet := gaiaclientset.NewForConfigOrDie(childKubeConfig)

	selfKubeInformerFactory := kubeinformers.NewSharedInformerFactory(childKubeClientSet, known.DefaultResync)
	selfGaiaInformerFactory := gaiainformers.NewSharedInformerFactory(childGaiaClientSet, known.DefaultResync)
	localSuperKubeConfig := NewLocalSuperKubeConfig(ctx, childKubeConfig.Host, childKubeClientSet)
	scheduler, err := New(childGaiaClientSet, selfGaiaInformerFactory, localSuperKubeConfig, childKubeClientSet)

	schedulerAgent := &ControllerManager{
		ctx:                 ctx,
		Identity:            identity,
		childKubeClientSet:  childKubeClientSet,
		childGaiaClientSet:  childGaiaClientSet,
		gaiaInformerFactory: selfGaiaInformerFactory,
		kubeInformerFactory: selfKubeInformerFactory,
		scheduler:           scheduler,
	}
	return schedulerAgent, nil
}

func (controller *ControllerManager) Run() {
	klog.Info("starting gaia schedule controller ...")

	// start the leader election code loop
	leaderelection.RunOrDie(controller.ctx, *newLeaderElectionConfigWithDefaultValue(controller.Identity, controller.childKubeClientSet, leaderelection.LeaderCallbacks{
		OnStartedLeading: func(ctx context.Context) {
			// 1. start generic informers
			controller.gaiaInformerFactory.Start(ctx.Done())
			controller.kubeInformerFactory.Start(ctx.Done())

			// 2. start local scheduler.
			go func() {
				controller.scheduler.RunLocalScheduler(known.DefaultThreadiness, ctx.Done())
			}()

			// 3. when we get add parentDedicatedKubeConfig add parent desc controller and start it.
			go func() {
				controller.scheduler.SetparentDedicatedKubeConfig(ctx)
				controller.scheduler.SetParentDescController(gaiaclientset.NewForConfigOrDie(controller.scheduler.GetparentDedicatedKubeConfig()), controller.scheduler.GetDedicatedNamespace())
				controller.scheduler.RunParentScheduler(known.DefaultThreadiness, ctx.Done())
			}()
		},
		OnStoppedLeading: func() {
			klog.Error("leader election got lost")
		},
		OnNewLeader: func(identity string) {
			// we're notified when new leader elected
			if identity == controller.Identity {
				// I just got the lock
				return
			}
			klog.Infof("new leader elected: %s", identity)
		},
	},
	))
}

func newLeaderElectionConfigWithDefaultValue(identity string, clientset kubernetes.Interface, callbacks leaderelection.LeaderCallbacks) *leaderelection.LeaderElectionConfig {
	return &leaderelection.LeaderElectionConfig{
		Lock: &resourcelock.LeaseLock{
			LeaseMeta: metav1.ObjectMeta{
				Name:      common.SelfClusterLeaseName,
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
