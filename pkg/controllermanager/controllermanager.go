package controllermanager

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	genericapifilters "k8s.io/apiserver/pkg/endpoints/filters"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	genericfilters "k8s.io/apiserver/pkg/server/filters"
	"k8s.io/apiserver/pkg/server/healthz"
	"k8s.io/component-base/metrics/legacyregistry"

	hypernodeclientset "github.com/SUMMERLm/hyperNodes/pkg/generated/clientset/versioned"

	gaiaconfig "github.com/lmxia/gaia/cmd/gaia-controllers/app/config"
	"github.com/lmxia/gaia/cmd/gaia-controllers/app/option"
	platformv1alpha1 "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllermanager/approver"
	"github.com/lmxia/gaia/pkg/controllermanager/metrics"
	"github.com/lmxia/gaia/pkg/controllers/apps/resourcebinding"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	"github.com/lmxia/gaia/pkg/utils"
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

	"github.com/lmxia/gaia/pkg/controllers/apps/cronmaster"
	"k8s.io/apiserver/pkg/authentication/authenticator"
	"k8s.io/apiserver/pkg/authorization/authorizer"
	"k8s.io/apiserver/pkg/server/mux"
	"k8s.io/client-go/kubernetes/scheme"
)

type ControllerManager struct {
	ctx context.Context

	// Identity is the unique string identifying a lease holder across
	// all participants in an election.
	Identity string
	// kubernetes clustername to generate gaiaName and clsrrName
	ClusterHostName string
	// ClusterID denotes current child cluster id
	ClusterID *types.UID

	// clientset for child cluster
	localKubeClientSet kubernetes.Interface
	localGaiaClientSet *gaiaclientset.Clientset

	// dedicated kubeconfig for accessing parent cluster, which is auto populated by the parent cluster
	// when cluster registration request gets approved
	parentKubeConfig *rest.Config
	// dedicated namespace in parent cluster for current child cluster
	DedicatedNamespace *string

	// report cluster status
	statusManager  *Manager
	crrApprover    *approver.CRRApprover
	rbController   *resourcebinding.RBController
	rbMerger       *resourcebinding.RBMerger
	cronController *cronmaster.Controller

	gaiaInformerFactory gaiainformers.SharedInformerFactory
	kubeInformerFactory kubeinformers.SharedInformerFactory
	triggerFunc         func(metav1.Object)
}

// NewAgent returns a new Agent.
func NewControllerManager(ctx context.Context, childKubeConfigFile, clusterHostName, networkBindUrl string, managedCluster *platformv1alpha1.ManagedClusterOptions, opts *option.Options) (*gaiaconfig.CompletedConfig, *ControllerManager, error) {
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

	localKubeConfig, err := utils.LoadsKubeConfig(childKubeConfigFile, 1)
	if err != nil {
		return nil, nil, err
	}

	if len(managedCluster.ManagedClusterSource) <= 0 {
		managedCluster.ManagedClusterSource = known.ManagedClusterSourceFromInformer
	}

	if managedCluster.ManagedClusterSource == known.ManagedClusterSourceFromPrometheus && len(managedCluster.PrometheusMonitorUrlPrefix) <= 0 {
		managedCluster.PrometheusMonitorUrlPrefix = known.PrometheusUrlPrefix
	}

	if len(managedCluster.TopoSyncBaseUrl) <= 0 {
		managedCluster.TopoSyncBaseUrl = known.TopoSyncBaseUrl
	}

	// create clientset for child cluster
	localKubeClientSet := kubernetes.NewForConfigOrDie(localKubeConfig)
	localGaiaClientSet := gaiaclientset.NewForConfigOrDie(localKubeConfig)
	hypernodeClientSet := hypernodeclientset.NewForConfigOrDie(localKubeConfig)

	localKubeInformerFactory := kubeinformers.NewSharedInformerFactory(localKubeClientSet, known.DefaultResync)
	localGaiaInformerFactory := gaiainformers.NewSharedInformerFactory(localGaiaClientSet, known.DefaultResync)

	approver, apprerr := approver.NewCRRApprover(localKubeClientSet, localGaiaClientSet, localKubeConfig, localGaiaInformerFactory, localKubeInformerFactory)
	if apprerr != nil {
		klog.Error(apprerr)
	}

	rbController, rberr := resourcebinding.NewRBController(localKubeClientSet, localGaiaClientSet, localGaiaInformerFactory, localKubeConfig, networkBindUrl)
	if rberr != nil {
		klog.Error(rberr)
	}
	statusManager := NewStatusManager(ctx, localKubeConfig.Host, clusterHostName, managedCluster, localKubeClientSet, localGaiaClientSet, hypernodeClientSet)
	rbMerger, err := resourcebinding.NewRBMerger(localKubeClientSet, localGaiaClientSet)
	if err != nil {
		klog.Error(err)
	}
	cronController, cronErr := cronmaster.NewController(localGaiaClientSet, localKubeClientSet, localGaiaInformerFactory, localKubeInformerFactory, localKubeConfig)
	if cronErr != nil {
		klog.Error(rberr)
	}
	agent := &ControllerManager{
		ctx:                 ctx,
		Identity:            identity,
		ClusterHostName:     clusterHostName,
		localKubeClientSet:  localKubeClientSet,
		localGaiaClientSet:  localGaiaClientSet,
		gaiaInformerFactory: localGaiaInformerFactory,
		kubeInformerFactory: localKubeInformerFactory,
		crrApprover:         approver,
		rbController:        rbController,
		rbMerger:            rbMerger,
		statusManager:       statusManager,
		cronController:      cronController,
	}

	metrics.Register()

	return &cc, agent, nil
}

func (controller *ControllerManager) Run(cc *gaiaconfig.CompletedConfig) {
	klog.Info("starting gaia controller ...")

	// start the leader election code loop
	leaderelection.RunOrDie(controller.ctx, *newLeaderElectionConfigWithDefaultValue(controller.Identity, controller.localKubeClientSet,
		leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				// 1. start generic informers
				klog.Info("start generic informers...")
				controller.gaiaInformerFactory.Start(ctx.Done())
				controller.kubeInformerFactory.Start(ctx.Done())

				// 2. start Approve
				go func() {
					klog.Info("start 2. start Approve ...")
					controller.crrApprover.Run(common.DefaultThreadiness, ctx.Done())
				}()

				// 3. start rbMerger
				go func() {
					klog.Info("start 3. start rbMerger Controller...")
					controller.rbMerger.RunToLocalResourceBindingMerger(common.DefaultThreadiness, ctx.Done())
				}()

				// 4. wait for target to join into a parent cluster.
				go func() {
					klog.Info("start 4. wait for target to join into a parent cluster")
					// 4.1 will wait, need period report only when register successfully.
					controller.registerSelfCluster(ctx)

					// 4.2 report periodly.
					go wait.UntilWithContext(ctx, func(ctx context.Context) {
						controller.statusManager.Run(ctx, controller.parentKubeConfig, controller.DedicatedNamespace, controller.ClusterID)
					}, time.Duration(0))
				}()

				// 5. start local description
				go func() {
					controller.rbController.SetLocalDescriptionController()
					klog.Info("start 5. start local description informers...")
					controller.rbController.RunLocalDescription(common.DefaultThreadiness, ctx.Done())
				}()

				// 6. start parent ResourceBinding and Description
				go func() {
					// set parent config
					controller.rbController.SetParentRBController()
					klog.Info("start 6. start parent ResourceBinding and Description informers...")
					controller.rbController.RunParentResourceBindingAndDescription(common.DefaultThreadiness, ctx.Done())
				}()

				// 7. start rbMerger
				go func() {
					controller.rbMerger.SetParentRBController()
					klog.Info("start 7. start rbMerger Controller...")
					controller.rbMerger.RunToParentResourceBindingMerger(common.DefaultThreadiness, ctx.Done())
				}()
				// 8. start cronmaster controller
				go func() {
					klog.Info("start 8. start cronmaster controller...")
					controller.cronController.Run(common.DefaultThreadiness, ctx.Done())
				}()

				// metrics
				if cc.SecureServing != nil {
					handler := buildHandlerChain(newMetricsHandler(), cc.Authentication.Authenticator, cc.Authorization.Authorizer)
					klog.Info("Starting gaia-controllers metrics server...")
					if _, err := cc.SecureServing.Serve(handler, 0, ctx.Done()); err != nil {
						klog.Infof("failed to start metrics server: %v", err)
					}
				}

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
				Name:      common.GaiaControllerLeaseName,
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
			clusterID, err := controller.getClusterID(ctx, controller.localKubeClientSet)
			if err != nil {
				return
			}
			klog.Infof("current cluster id is %q", clusterID)
			controller.ClusterID = &clusterID
		}

		target, err := controller.localGaiaClientSet.PlatformV1alpha1().Targets().Get(ctx, common.ParentClusterTargetName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get targets: %v wait for next loop", err)
			return
		}

		// get parent cluster kubeconfig
		if tryToUseSecret {
			secret, err := controller.localKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Get(ctx,
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
						controller.parentKubeConfig = parentDedicatedKubeConfig
					}
				}
			}
		}

		// bootstrap cluster registration
		if err := controller.bootstrapClusterRegistrationIfNeeded(ctx, target); err != nil {
			klog.Error(err)
			klog.Warning("something went wrong when using existing parent cluster credentials, switch to use bootstrap token instead")
			tryToUseSecret = false
			controller.parentKubeConfig = nil
			return
		}

		// Cancel the context on success
		cancel()
	}, common.DefaultRetryPeriod, 0.3, true)
}

func (controller *ControllerManager) getClusterID(ctx context.Context, childClientSet kubernetes.Interface) (types.UID, error) {
	lease, err := childClientSet.CoordinationV1().Leases(common.GaiaSystemNamespace).Get(ctx, common.GaiaControllerLeaseName, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("unable to retrieve %s/%s Lease object: %v", common.GaiaSystemNamespace, common.GaiaControllerLeaseName, err)
		return "", err
	}
	return lease.UID, nil
}

func (controller *ControllerManager) bootstrapClusterRegistrationIfNeeded(ctx context.Context, target *platformv1alpha1.Target) error {
	klog.Infof("try to bootstrap cluster registration if needed")

	clientConfig, err := controller.getBootstrapKubeConfigForParentCluster(target)
	if err != nil {
		return err
	}
	// create ClusterRegistrationRequest
	client := gaiaclientset.NewForConfigOrDie(clientConfig)
	clusterNamePrefix := generateClusterNamePrefix(target.Spec.ClusterName, controller.ClusterHostName, common.NamePrefixForGaiaObjects)
	crr, err := client.PlatformV1alpha1().ClusterRegistrationRequests().Create(ctx,
		newClusterRegistrationRequest(*controller.ClusterID, clusterNamePrefix, generateClusterName(clusterNamePrefix),
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

func (controller *ControllerManager) getBootstrapKubeConfigForParentCluster(target *platformv1alpha1.Target) (*rest.Config, error) {
	if controller.parentKubeConfig != nil {
		return controller.parentKubeConfig, nil
	}

	// get bootstrap kubeconfig from token
	clientConfig, err := utils.GenerateKubeConfigFromToken(target.Spec.ParentURL, target.Spec.BootstrapToken, nil, 1)
	if err != nil {
		return nil, fmt.Errorf("error while creating kubeconfig: %v", err)
	}

	return clientConfig, nil
}

func (controller *ControllerManager) waitingForApproval(ctx context.Context, client gaiaclientset.Interface, target *platformv1alpha1.Target) error {
	var crr *platformv1alpha1.ClusterRegistrationRequest
	var err error

	// wait until stopCh is closed or request is approved
	waitingCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var clusterName string
	wait.JitterUntilWithContext(waitingCtx, func(ctx context.Context) {
		crrName := generateClusterRegistrationRequestName(*controller.ClusterID, generateClusterNamePrefix(target.Spec.ClusterName, controller.ClusterHostName, common.NamePrefixForGaiaObjects))

		crr, err = client.PlatformV1alpha1().ClusterRegistrationRequests().Get(ctx, crrName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("failed to get ClusterRegistrationRequest %s: %v", crrName, err)
			return
		}
		if name, ok := crr.Labels[known.ClusterNameLabel]; ok {
			clusterName = name
			klog.V(5).Infof("found existing cluster name %q, reuse it", name)
		}

		if crr.Status.Result != nil && *crr.Status.Result == platformv1alpha1.RequestApproved {
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
	controller.parentKubeConfig = parentDedicatedKubeConfig
	controller.DedicatedNamespace = utilpointer.StringPtr(crr.Status.DedicatedNamespace)

	// once the request gets approved
	// store auto-populated credentials to Secret "parent-cluster" in "gaia-system" namespace
	go controller.storeParentClusterCredentials(crr, clusterName, target)

	return nil
}

func (controller *ControllerManager) storeParentClusterCredentials(crr *platformv1alpha1.ClusterRegistrationRequest, clusterName string, target *platformv1alpha1.Target) {
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
		_, err := controller.localKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Create(ctx, secret, metav1.CreateOptions{})
		if err == nil {
			klog.V(5).Infof("successfully store parent cluster credentials")
			cancel()
			return
		}

		if apierrors.IsAlreadyExists(err) {
			klog.V(5).Infof("found existed parent cluster credentials, will try to update if needed")
			_, err = controller.localKubeClientSet.CoreV1().Secrets(common.GaiaSystemNamespace).Update(ctx, secret, metav1.UpdateOptions{})
			if err == nil {
				cancel()
				return
			}
		}
		klog.ErrorDepth(5, fmt.Sprintf("failed to store parent cluster credentials: %v", err))
	}, common.DefaultRetryPeriod, 0.4, true)
}

func newClusterRegistrationRequest(clusterID types.UID, clusterNamePrefix, clusterName, clusterLabels string) *platformv1alpha1.ClusterRegistrationRequest {
	return &platformv1alpha1.ClusterRegistrationRequest{
		ObjectMeta: metav1.ObjectMeta{
			Name: generateClusterRegistrationRequestName(clusterID, clusterNamePrefix),
			Labels: map[string]string{
				common.ClusterRegisteredByLabel: common.SubCluster,
				common.ClusterIDLabel:           string(clusterID),
				common.ClusterNameLabel:         clusterName,
			},
		},
		Spec: platformv1alpha1.ClusterRegistrationRequestSpec{
			ClusterID:         clusterID,
			ClusterNamePrefix: fmt.Sprintf("%s-", clusterNamePrefix),
			ClusterName:       clusterName,
			ClusterLabels:     parseClusterLabels(clusterLabels),
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

func generateClusterNamePrefix(targetClusterName, optsClusterName, clusterNamePrefix string) string {
	if len(targetClusterName) != 0 {
		return targetClusterName
	}
	if len(optsClusterName) != 0 {
		return optsClusterName
	}
	return clusterNamePrefix
}

func generateClusterRegistrationRequestName(clusterID types.UID, clusterNamePrefix string) string {
	if len(clusterNamePrefix) != 0 {
		return fmt.Sprintf("%s-%s", clusterNamePrefix, string(clusterID))
	}
	return fmt.Sprintf("%s%s", common.NamePrefixForGaiaObjects, string(clusterID))
}

func generateClusterName(clusterNamePrefix string) string {
	if len(clusterNamePrefix) != 0 {
		return clusterNamePrefix
	}

	clusterName := fmt.Sprintf("%s%s", common.NamePrefixForGaiaObjects, utilrand.String(common.DefaultRandomUIDLength))
	klog.V(4).Infof("generate a random string %q as cluster name for later use", clusterName)
	return clusterName
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
	pathRecorderMux := mux.NewPathRecorderMux("gaia-controller-manager")
	installMetricHandler(pathRecorderMux)
	return pathRecorderMux
}

// newHealthzHandler creates a healthz server from the config, and will also
// embed the metrics handler if the healthz and metrics address configurations
// are the same.
func newHealthzHandler(separateMetrics bool, checks ...healthz.HealthChecker) http.Handler {
	pathRecorderMux := mux.NewPathRecorderMux("gaia-controller-manager")
	healthz.InstallHandler(pathRecorderMux, checks...)
	if !separateMetrics {
		installMetricHandler(pathRecorderMux)
	}

	return pathRecorderMux
}
