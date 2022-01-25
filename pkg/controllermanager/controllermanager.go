package controllermanager

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/apimachinery/pkg/util/wait"
	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
	utilpointer "k8s.io/utils/pointer"

	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllermanager/approver"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	"github.com/lmxia/gaia/pkg/scheduler"
	"github.com/lmxia/gaia/pkg/utils"
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

	// report cluster status
	statusManager       *Manager
	crrApprover         *approver.CRRApprover
	gaiaInformerFactory gaiainformers.SharedInformerFactory
	kubeInformerFactory kubeinformers.SharedInformerFactory
	scheduler           *scheduler.Scheduler
	triggerFunc         func(metav1.Object)
}

// NewAgent returns a new Agent.
func NewControllerManager(ctx context.Context, childKubeConfigFile string) (*ControllerManager, error) {
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
	approver, err := approver.NewCRRApprover(childKubeClientSet, childGaiaClientSet, selfGaiaInformerFactory,
		selfKubeInformerFactory)
	if err != nil {
		klog.Error(err)
	}
	statusManager := NewStatusManager(ctx, childKubeConfig.Host, childKubeClientSet)
	scheduler, err := scheduler.New(childGaiaClientSet, selfGaiaInformerFactory, statusManager.localSuperKubeConfig)

	agent := &ControllerManager{
		ctx:                 ctx,
		Identity:            identity,
		childKubeClientSet:  childKubeClientSet,
		childGaiaClientSet:  childGaiaClientSet,
		gaiaInformerFactory: selfGaiaInformerFactory,
		kubeInformerFactory: selfKubeInformerFactory,
		crrApprover:         approver,
		scheduler:           scheduler,
		statusManager: statusManager,
	}
	return agent, nil
}

func (controller *ControllerManager) Run() {
	klog.Info("starting gaia controller ...")

	// start the leader election code loop
	leaderelection.RunOrDie(controller.ctx, *newLeaderElectionConfigWithDefaultValue(controller.Identity, controller.childKubeClientSet,
		leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				//1. start generic informers
				controller.gaiaInformerFactory.Start(ctx.Done())
				controller.kubeInformerFactory.Start(ctx.Done())

				//2. start Approve
				go func() {
					controller.crrApprover.Run(common.DefaultThreadiness, ctx.Done())
				}()

				//3. start local scheduler.
				go func() {
					controller.scheduler.RunLocalScheduler(known.DefaultThreadiness, ctx.Done())
				}()

				// 4. wait for target to join into a parent cluster.
				go func() {
					// 5. will wait, need period report only when register successfully.
					controller.registerSelfCluster(ctx)
					// 6. report periodly.
					go wait.UntilWithContext(ctx, func(ctx context.Context) {
						controller.statusManager.Run(ctx, controller.parentDedicatedKubeConfig, controller.DedicatedNamespace, controller.ClusterID)
					}, time.Duration(0))
					// 7. when we get add parentDedicatedKubeConfig add parent desc controller and start it.
					go func() {
						controller.scheduler.SetParentDescController(gaiaclientset.NewForConfigOrDie(controller.parentDedicatedKubeConfig),
							*controller.DedicatedNamespace)
						controller.scheduler.RunParentScheduler(known.DefaultThreadiness, ctx.Done())
					}()
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

// registerSelfCluster begins registering. It starts registering and blocked until the context is done.
func (controller *ControllerManager) registerSelfCluster(ctx context.Context) {
	// complete your controller loop here
	klog.Info("start registering current cluster as a child cluster...")

	tryToUseSecret := true

	registerCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	wait.JitterUntilWithContext(registerCtx, func(ctx context.Context) {
		// get cluster unique id
		if controller.ClusterID == nil {
			klog.Infof("retrieving cluster id")
			clusterID, err := controller.getClusterID(ctx, controller.childKubeClientSet)
			if err != nil {
				return
			}
			klog.Infof("current cluster id is %q", clusterID)
			controller.ClusterID = &clusterID
		}

		target, err := controller.childGaiaClientSet.PlatformV1alpha1().Targets().Get(ctx, common.ParentClusterTargetName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get targets: %v wait for next loop", err)
			return
		}

		// get parent cluster kubeconfig
		if tryToUseSecret {
			secret, err := controller.childKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Get(ctx,
				common.ParentClusterSecretName, metav1.GetOptions{})
			if err != nil && !apierrors.IsNotFound(err) {
				klog.Errorf("failed to get secretFromParentCluster: %v", err)
				return
			}
			if err == nil {
				klog.Infof("found existing secretFromParentCluster '%s/%s' that can be used to access parent cluster",
					common.GaiaSystemNamespace, common.ParentClusterSecretName)

				if string(secret.Data[common.ClusterAPIServerURLKey]) != target.Spec.ParentURL {
					klog.Warningf("the parent url got changed from %q to %q", secret.Data[known.ClusterAPIServerURLKey], target.Spec.ParentURL)
					klog.Warningf("will try to re-register current cluster")
				} else {
					parentDedicatedKubeConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL,
						string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey], 2)
					if err == nil {
						controller.parentDedicatedKubeConfig = parentDedicatedKubeConfig
					}
				}
			}
		}

		// bootstrap cluster registration
		if err := controller.bootstrapClusterRegistrationIfNeeded(ctx, target); err != nil {
			klog.Error(err)
			klog.Warning("something went wrong when using existing parent cluster credentials, switch to use bootstrap token instead")
			tryToUseSecret = false
			controller.parentDedicatedKubeConfig = nil
			return
		}

		// Cancel the context on success
		cancel()
	}, common.DefaultRetryPeriod, 0.3, true)
}

func (controller *ControllerManager) getClusterID(ctx context.Context, childClientSet kubernetes.Interface) (types.UID, error) {
	lease, err := childClientSet.CoordinationV1().Leases(common.GaiaSystemNamespace).Get(ctx, common.SelfClusterLeaseName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to retrieve %s/%s Lease object: %v", common.GaiaSystemNamespace, common.SelfClusterLeaseName, err)
		return "", err
	}
	return lease.UID, nil
}

func (controller *ControllerManager) bootstrapClusterRegistrationIfNeeded(ctx context.Context, target *clusterapi.Target) error {
	klog.Infof("try to bootstrap cluster registration if needed")

	clientConfig, err := controller.getBootstrapKubeConfigForParentCluster(target)
	if err != nil {
		return err
	}
	// create ClusterRegistrationRequest
	client := gaiaclientset.NewForConfigOrDie(clientConfig)
	crr, err := client.PlatformV1alpha1().ClusterRegistrationRequests().Create(ctx,
		newClusterRegistrationRequest(*controller.ClusterID, generateClusterName("", common.NamePrefixForGaiaObjects),
			""),
		metav1.CreateOptions{})

	if err != nil {
		if !apierrors.IsAlreadyExists(err) {
			return fmt.Errorf("failed to create ClusterRegistrationRequest: %v", err)
		}
		klog.Infof("a ClusterRegistrationRequest has already been created for cluster %q", *controller.ClusterID)
	} else {
		klog.Infof("successfully create ClusterRegistrationRequest %q", klog.KObj(crr))
	}

	// wait until stopCh is closed or request is approved
	err = controller.waitingForApproval(ctx, client, target)

	return err
}

func (controller *ControllerManager) getBootstrapKubeConfigForParentCluster(target *clusterapi.Target) (*rest.Config, error) {
	if controller.parentDedicatedKubeConfig != nil {
		return controller.parentDedicatedKubeConfig, nil
	}

	// get bootstrap kubeconfig from token
	clientConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL, target.Spec.BootstrapToken, nil, 1)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig: %v", err)
	}

	return clientConfig, nil
}

func (controller *ControllerManager) waitingForApproval(ctx context.Context, client gaiaclientset.Interface, target *clusterapi.Target) error {
	var crr *clusterapi.ClusterRegistrationRequest
	var err error

	// wait until stopCh is closed or request is approved
	waitingCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var clusterName string
	wait.JitterUntilWithContext(waitingCtx, func(ctx context.Context) {
		crrName := generateClusterRegistrationRequestName(*controller.ClusterID)

		crr, err = client.PlatformV1alpha1().ClusterRegistrationRequests().Get(ctx, crrName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get ClusterRegistrationRequest %s: %v", crrName, err)
			return
		}
		if name, ok := crr.Labels[known.ClusterNameLabel]; ok {
			clusterName = name
			klog.V(5).Infof("found existing cluster name %q, reuse it", name)
		}

		if crr.Status.Result != nil && *crr.Status.Result == clusterapi.RequestApproved {
			klog.Infof("the registration request for cluster %q gets approved", *controller.ClusterID)
			// cancel on success
			cancel()
			return
		}

		klog.V(4).Infof("the registration request for cluster %q (%q) is still waiting for approval...",
			*controller.ClusterID, clusterName)
	}, known.DefaultRetryPeriod, 0.4, true)

	parentDedicatedKubeConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL,
		string(crr.Status.DedicatedToken), crr.Status.CACertificate, 2)
	if err != nil {
		return err
	}
	controller.parentDedicatedKubeConfig = parentDedicatedKubeConfig
	controller.DedicatedNamespace = utilpointer.StringPtr(crr.Status.DedicatedNamespace)

	// once the request gets approved
	// store auto-populated credentials to Secret "parent-cluster" in "clusternet-system" namespace
	go controller.storeParentClusterCredentials(crr, clusterName, target)

	return nil
}

func (controller *ControllerManager) storeParentClusterCredentials(crr *clusterapi.ClusterRegistrationRequest, clusterName string, target *clusterapi.Target) {
	klog.V(4).Infof("store parent cluster credentials to secret for later use")
	secretCtx, cancel := context.WithCancel(controller.ctx)
	defer cancel()
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name: known.ParentClusterSecretName,
			Labels: map[string]string{
				common.ClusterBootstrappingLabel: common.CredentialsAuto,
				common.ClusterIDLabel:            string(*controller.ClusterID),
				common.ClusterNameLabel:          clusterName,
			},
		},
		Data: map[string][]byte{
			corev1.ServiceAccountRootCAKey:    crr.Status.CACertificate,
			corev1.ServiceAccountTokenKey:     crr.Status.DedicatedToken,
			corev1.ServiceAccountNamespaceKey: []byte(crr.Status.DedicatedNamespace),
			common.ClusterAPIServerURLKey:     []byte(target.Spec.ParentURL),
		},
	}

	wait.JitterUntilWithContext(secretCtx, func(ctx context.Context) {
		_, err := controller.childKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Create(ctx, secret, metav1.CreateOptions{})
		if err == nil {
			klog.V(5).Infof("successfully store parent cluster credentials")
			cancel()
			return
		}

		if apierrors.IsAlreadyExists(err) {
			klog.V(5).Infof("found existed parent cluster credentials, will try to update if needed")
			_, err = controller.childKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Update(ctx, secret, metav1.UpdateOptions{})
			if err == nil {
				cancel()
				return
			}
		}
		klog.ErrorDepth(5, fmt.Sprintf("failed to store parent cluster credentials: %v", err))
	}, common.DefaultRetryPeriod, 0.4, true)
}

func newClusterRegistrationRequest(clusterID types.UID, clusterName, clusterLabels string) *clusterapi.ClusterRegistrationRequest {
	return &clusterapi.ClusterRegistrationRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: generateClusterRegistrationRequestName(clusterID),
			Labels: map[string]string{
				common.ClusterRegisteredByLabel: common.SubCluster,
				common.ClusterIDLabel:           string(clusterID),
				common.ClusterNameLabel:         clusterName,
			},
		},
		Spec: clusterapi.ClusterRegistrationRequestSpec{
			ClusterID:     clusterID,
			ClusterName:   clusterName,
			ClusterLabels: parseClusterLabels(clusterLabels),
		},
	}
}

func parseClusterLabels(clusterLabels string) map[string]string {
	clusterLabelsMap := make(map[string]string)
	clusterLabelsArray := strings.Split(clusterLabels, ",")
	for _, labelString := range clusterLabelsArray {
		labelArray := strings.Split(labelString, "=")
		if len(labelArray) != 2 {
			klog.Warningf("invalid cluster label %s", labelString)
			continue
		}
		clusterLabelsMap[labelArray[0]] = labelArray[1]
	}
	return clusterLabelsMap
}

func generateClusterRegistrationRequestName(clusterID types.UID) string {
	return fmt.Sprintf("%s%s", common.NamePrefixForGaiaObjects, string(clusterID))
}

func generateClusterName(clusterName, clusterNamePrefix string) string {
	if len(clusterName) == 0 {
		clusterName = fmt.Sprintf("%s%s", clusterNamePrefix, utilrand.String(common.DefaultRandomUIDLength))
		klog.V(4).Infof("generate a random string %q as cluster name for later use", clusterName)
	}
	return clusterName
}
