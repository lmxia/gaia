// copied and modified from clusternet.
package controllermanager

import (
	"context"
	"errors"
	hypernodeclientset "github.com/SUMMERLm/hyperNodes/pkg/generated/clientset/versioned"
	appsapi "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/features"
	"k8s.io/klog/v2"
	"os"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/util/retry"

	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/clusterstatus"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	"github.com/lmxia/gaia/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type Manager struct {
	// statusReportFrequency is the frequency at which the agent reports current cluster's status
	statusReportFrequency metav1.Duration

	clusterStatusController *clusterstatus.Controller

	managedCluster       *clusterapi.ManagedCluster
	localSuperKubeConfig *rest.Config
}

func NewStatusManager(ctx context.Context, apiserverURL, clusterName string, managedCluster *clusterapi.ManagedClusterOptions, kubeClient kubernetes.Interface, gaiaClient *gaiaclientset.Clientset, hypernodeClient *hypernodeclientset.Clientset) *Manager {
	retryCtx, retryCancel := context.WithTimeout(ctx, known.DefaultRetryPeriod)
	defer retryCancel()

	// get high priority secret.
	secret := utils.GetDeployerCredentials(retryCtx, kubeClient, common.GaiaAppSA)
	var clusterStatusKubeConfig *rest.Config
	if secret != nil {
		var err error
		clusterStatusKubeConfig, err = utils.GenerateKubeConfigFromToken(apiserverURL,
			string(secret.Data[corev1.ServiceAccountTokenKey]), secret.Data[corev1.ServiceAccountRootCAKey], 2)
		if err == nil {
			kubeClient = kubernetes.NewForConfigOrDie(clusterStatusKubeConfig)
			gaiaClient = gaiaclientset.NewForConfigOrDie(clusterStatusKubeConfig)
			hypernodeClient = hypernodeclientset.NewForConfigOrDie(clusterStatusKubeConfig)
		}
	}

	return &Manager{
		statusReportFrequency: metav1.Duration{Duration: common.DefaultClusterStatusCollectFrequency},
		clusterStatusController: clusterstatus.NewController(ctx, apiserverURL, clusterName, managedCluster,
			kubeClient, gaiaClient, hypernodeClient, common.DefaultClusterStatusCollectFrequency, common.DefaultClusterStatusReportFrequency),
		localSuperKubeConfig: clusterStatusKubeConfig,
	}
}

func (mgr *Manager) Run(ctx context.Context, parentDedicatedKubeConfig *rest.Config, dedicatedNamespace *string, clusterID *types.UID) {
	klog.Infof("starting status manager to report heartbeats...")
	go mgr.clusterStatusController.Run(ctx)
	// used to handle parent resource
	client := gaiaclientset.NewForConfigOrDie(parentDedicatedKubeConfig)
	wait.UntilWithContext(ctx, func(ctx context.Context) {
		if dedicatedNamespace == nil {
			klog.Error("unexpected nil dedicatedNamespace")
			// in case a race condition here
			os.Exit(1)
			return
		}
		if clusterID == nil {
			klog.Error("unexpected nil clusterID")
			// in case a race condition here
			os.Exit(1)
			return
		}
		mgr.updateClusterStatus(ctx, *dedicatedNamespace, string(*clusterID), client, retry.DefaultBackoff)
	}, mgr.statusReportFrequency.Duration)
}

func (mgr *Manager) updateClusterStatus(ctx context.Context, namespace, clusterID string, client gaiaclientset.Interface, backoff wait.Backoff) {

	if features.DefaultMutableFeatureGate.Enabled(features.AbnormalScheduler) {
		if isParent, errGetCluster := mgr.clusterStatusController.IsParentCluster(); errGetCluster == nil {
			if isParent {
				// monitor abnormal pods and modify their descriptions' status to rescheduled
				mgr.modifyDescStatusForAbnormalPods(ctx, namespace, client)
			} else {
				klog.Infof("This cluster is not a parent cluster. ")
			}
		}
	}

	if mgr.managedCluster == nil {
		managedClusters, err := client.PlatformV1alpha1().ManagedClusters(namespace).List(ctx, metav1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set{
				known.ClusterIDLabel: clusterID,
			}).String(),
		})
		if err != nil {
			klog.Errorf("failed to list ManagedCluster in namespace %s: %v", namespace, err)
			return
		}

		if len(managedClusters.Items) > 0 {
			if len(managedClusters.Items) > 1 {
				klog.Warningf("found multiple ManagedCluster for cluster %s in namespace %s !!!", clusterID, namespace)
			}
			mgr.managedCluster = new(clusterapi.ManagedCluster)
			*mgr.managedCluster = managedClusters.Items[0]
		} else {
			klog.Warningf("unable to get a matching ManagedCluster for cluster %s, will retry later", clusterID)
			return
		}
	}

	// in case the network is not stable, retry with backoff
	var lastError, updateMCError error
	var mcls *clusterapi.ManagedCluster
	err := wait.ExponentialBackoffWithContext(ctx, backoff, func() (bool, error) {
		status := mgr.clusterStatusController.GetClusterStatus()
		if status == nil {
			lastError = errors.New("cluster status is not ready, will retry later")
			return false, nil
		}
		mgr.managedCluster.SetLabels(mgr.getNewManagedClusterLabels())
		mcls, updateMCError = client.PlatformV1alpha1().ManagedClusters(namespace).Update(ctx, mgr.managedCluster, metav1.UpdateOptions{})
		if updateMCError == nil {
			mgr.managedCluster = mcls
		} else {
			klog.Warning("failed to update labels of ManagedCluster: %v", updateMCError)
		}

		mgr.managedCluster.Status = *status
		mcls, lastError = client.PlatformV1alpha1().ManagedClusters(namespace).UpdateStatus(ctx, mgr.managedCluster, metav1.UpdateOptions{})
		if lastError == nil {
			mgr.managedCluster = mcls
			return true, nil
		}
		if apierrors.IsConflict(lastError) {
			mcls, lastError = client.PlatformV1alpha1().ManagedClusters(namespace).Get(ctx, mgr.managedCluster.Name, metav1.GetOptions{})
			if lastError == nil {
				mgr.managedCluster = mcls
			}
		}
		return false, nil
	})
	if err != nil {
		klog.WarningDepth(2, "failed to update status of ManagedCluster: %v", lastError)
	}
}

// getNewManagedClusterLabels return managedCluster labels
// for the labels begin with known.SpecificNodeLabelsKeyPrefix, will use the newly acquired node labels
func (mgr *Manager) getNewManagedClusterLabels() map[string]string {
	managedClusterLabels := mgr.managedCluster.GetLabels()
	for k, _ := range managedClusterLabels {
		if strings.HasPrefix(k, known.SpecificNodeLabelsKeyPrefix) {
			delete(managedClusterLabels, k)
		}
	}
	return labels.Merge(managedClusterLabels, mgr.clusterStatusController.GetManagedClusterLabels())
}

func (mgr *Manager) modifyDescStatusForAbnormalPods(ctx context.Context, namespace string, client gaiaclientset.Interface) {
	descNameMap := mgr.clusterStatusController.GetDescNameFromAbnormalPod()
	if len(descNameMap) == 0 {
		klog.Infof("There is no description that needs to be updated.")
		return
	}
	for descName, _ := range descNameMap {
		desc, _ := client.AppsV1alpha1().Descriptions(namespace).Get(ctx, descName, metav1.GetOptions{})
		klog.Infof("Update the the status phase of the desc(%v/%v)to %v", namespace, descName, appsapi.DescriptionPhaseReSchedule)
		var lastError error
		err := wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
			newDesc, _ := client.AppsV1alpha1().Descriptions(namespace).Get(ctx, desc.Name, metav1.GetOptions{})
			if appsapi.DescriptionPhaseReSchedule == newDesc.Status.Phase {
				lastError = errors.New("description status phase is already 'ReeSchedule', there is no need to update it.")
				return true, nil
			}

			desc.Status.Phase = appsapi.DescriptionPhaseReSchedule
			// check if failed
			_, lastError := client.AppsV1alpha1().Descriptions(namespace).UpdateStatus(ctx, desc, metav1.UpdateOptions{})
			if lastError == nil {
				return true, nil
			}
			if apierrors.IsConflict(lastError) {
				newDesc, lastError := client.AppsV1alpha1().Descriptions(namespace).Get(ctx, desc.Name, metav1.GetOptions{})
				if lastError == nil {
					desc = newDesc
				}
			}
			return false, nil
		})
		if err != nil {
			klog.WarningDepth(2, "failed to update status of description's status phase: %v/%v, err is ", desc.Namespace, desc.Name, lastError)
		}
		klog.V(5).Infof("Update the the status phase of the desc(%v) to %s successfully.",
			descName, appsapi.DescriptionPhaseReSchedule)

	}
	return
}
