package appstatus

import (
	"context"
	"encoding/json"
	"time"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiaInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	Listers "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
)

type StatusController struct {
	collectingPeriod metav1.Duration
	localGaiaClient  *gaiaClientSet.Clientset

	DescController *DescController
	RBController   *RBController
}

type RBController struct {
	collectingPeriod time.Duration
	kubeClient       *kubernetes.Clientset
	gaiaClient       *gaiaClientSet.Clientset
	parentGaiaClient *gaiaClientSet.Clientset

	rbsLister Listers.ResourceBindingLister
	rbsSynced cache.InformerSynced
}

type DescController struct {
	collectingPeriod time.Duration
	kubeClient       *kubernetes.Clientset
	gaiaClient       *gaiaClientSet.Clientset
	parentGaiaClient *gaiaClientSet.Clientset

	descLister Listers.DescriptionLister
	descSynced cache.InformerSynced
}

func NewStatusController(localKubeClient *kubernetes.Clientset, localGaiaClient *gaiaClientSet.Clientset,
	localGaiaInformerFactory gaiaInformers.SharedInformerFactory,
) *StatusController {
	return &StatusController{
		collectingPeriod: metav1.Duration{Duration: common.DefaultAppStatusCollectFrequency},
		localGaiaClient:  localGaiaClient,

		DescController: NewDescController(localKubeClient, localGaiaClient, localGaiaInformerFactory),
		RBController:   NewRBController(localKubeClient, localGaiaClient, localGaiaInformerFactory),
	}
}

func NewRBController(kubeClient *kubernetes.Clientset, gaiaClient *gaiaClientSet.Clientset,
	gaiaInformerFactory gaiaInformers.SharedInformerFactory,
) *RBController {
	return &RBController{
		collectingPeriod: common.DefaultAppStatusCollectFrequency,
		kubeClient:       kubeClient,
		gaiaClient:       gaiaClient,
		rbsLister:        gaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Lister(),
		rbsSynced:        gaiaInformerFactory.Apps().V1alpha1().ResourceBindings().Informer().HasSynced,
	}
}

func NewDescController(kubeClient *kubernetes.Clientset, gaiaClient *gaiaClientSet.Clientset,
	gaiaInformerFactory gaiaInformers.SharedInformerFactory,
) *DescController {
	return &DescController{
		collectingPeriod: common.DefaultAppStatusCollectFrequency,
		kubeClient:       kubeClient,
		gaiaClient:       gaiaClient,
		descLister:       gaiaInformerFactory.Apps().V1alpha1().Descriptions().Lister(),
		descSynced:       gaiaInformerFactory.Apps().V1alpha1().Descriptions().Informer().HasSynced,
	}
}

func (s *StatusController) Run(ctx context.Context, parentKubeConfig *rest.Config, clusterLevel string) {
	klog.Infof("starting app-status-controller to report status...")
	parentGaiaClient, _ := utils.SetParentClientAndInformer(parentKubeConfig)

	if clusterLevel != common.GlobalLayer {
		go s.RBController.Run(ctx, parentGaiaClient)
	}
	go s.DescController.Run(ctx, parentGaiaClient, clusterLevel)
}

func (r *RBController) Run(ctx context.Context, parentGaiaClient *gaiaClientSet.Clientset) {
	klog.Infof("starting resource-binding-status-controller to report status...")
	if parentGaiaClient == nil {
		klog.Errorf("resource-binding-status-controller failed to get parent gaia client")
		return
	}
	r.parentGaiaClient = parentGaiaClient
	if !cache.WaitForCacheSync(ctx.Done(), r.rbsSynced) {
		return
	}

	wait.UntilWithContext(ctx, r.collectResourceBindingStatus, r.collectingPeriod)
}

func (r *DescController) Run(ctx context.Context, parentGaiaClient *gaiaClientSet.Clientset, clusterLevel string) {
	klog.Infof("starting description-status-controller to report status...")
	if !cache.WaitForCacheSync(ctx.Done(), r.descSynced) {
		return
	}

	if clusterLevel == common.FieldLayer {
		if parentGaiaClient == nil {
			klog.Errorf("Failed to get parent gaia client")
			return
		}
		// todo  parent desc synced
		r.parentGaiaClient = parentGaiaClient
		wait.UntilWithContext(ctx, r.collectFieldDescriptionStatus, r.collectingPeriod)
	}

	if clusterLevel == common.GlobalLayer {
		wait.UntilWithContext(ctx, r.collectGlobalDescriptionStatus, r.collectingPeriod)
	}
}

func (r *DescController) collectFieldDescriptionStatus(ctx context.Context) {
	klog.V(5).Infof("start to collect field description status")
	_, dedicatedNs, err := utils.GetLocalClusterName(r.kubeClient)
	if err != nil {
		klog.Errorf("Failed to get local cluster name: %v", err)
		return
	}
	parentDescs, err := r.parentGaiaClient.AppsV1alpha1().Descriptions(dedicatedNs).List(ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to list parent descriptions used to synchronize the local description status: %v", err)
		return
	}

	for _, parentDesc := range parentDescs.Items {
		parentDescCopy := parentDesc.DeepCopy()
		descLabels := parentDescCopy.GetLabels()
		localDescs, err := r.descLister.List(labels.SelectorFromSet(labels.Set{
			common.OriginDescriptionNameLabel:      descLabels[common.OriginDescriptionNameLabel],
			common.OriginDescriptionNamespaceLabel: descLabels[common.OriginDescriptionNamespaceLabel],
			common.OriginDescriptionUIDLabel:       descLabels[common.OriginDescriptionUIDLabel],
		}))
		if err != nil {
			klog.Errorf("Failed to list all local derivative dridescriptions of parent description %s/%s, error: %v",
				dedicatedNs, parentDescCopy.GetName(), err)
			return
		}

		var status v1alpha1.DescriptionStatus
		for _, desc := range localDescs {
			switch desc.Status.Phase {
			case v1alpha1.DescriptionPhaseScheduled:
				continue
			case "":
				status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() + " is scheduling; "
			case v1alpha1.DescriptionPhaseFailure:
				status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() +
					" is failed, reason: " + desc.Status.Reason + "; "
			case v1alpha1.DescriptionPhasePending:
				status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() +
					" is pending, reason: " + desc.Status.Reason + "; "
			case v1alpha1.DescriptionPhaseReSchedule:
				status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() + " is rescheduling; "
			}
		}
		status.Phase = parentDescCopy.Status.Phase

		// Update the parent description status with local description status
		parentDescCopy.Status = status
		err = utils.UpdateDescriptionStatus(r.parentGaiaClient, parentDescCopy)
		if err != nil {
			klog.Errorf("Failed to update parent description status: %v", err)
		}
	}
	klog.V(5).Infof("end to update collected field description status")
}

func (r *DescController) collectGlobalDescriptionStatus(ctx context.Context) {
	klog.V(5).Infof("start to collect global description status")

	reservedDescs, err := r.gaiaClient.AppsV1alpha1().Descriptions(common.GaiaReservedNamespace).List(
		ctx, metav1.ListOptions{})
	if err != nil {
		klog.Errorf("Failed to list gaia-reserved descriptions used to synchronize the local description status: %v", err)
		return
	}

	for _, parentDesc := range reservedDescs.Items {
		parentDescCopy := parentDesc.DeepCopy()
		localDescs, err := r.descLister.List(labels.SelectorFromSet(labels.Set{
			common.OriginDescriptionNameLabel:      parentDescCopy.GetName(),
			common.OriginDescriptionNamespaceLabel: common.GaiaReservedNamespace,
			common.OriginDescriptionUIDLabel:       string(parentDescCopy.GetUID()),
		}))
		if err != nil {
			klog.Errorf("Failed to list all local dridescriptions of parent description %s/%s, error: %v",
				common.GaiaReservedNamespace, parentDescCopy.GetNamespace(), err)
			return
		}
		var status v1alpha1.DescriptionStatus
		if len(localDescs) == 0 {
			continue
		} else {
			for _, desc := range localDescs {
				switch desc.Status.Phase {
				case v1alpha1.DescriptionPhaseScheduled:
					if len(desc.Status.Reason) == 0 {
						continue
					}
					status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() +
						" Sub-cluster scheduling failed, Details :" + desc.Status.Reason + "; "
				case "":
					status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() + " is scheduling; "
				case v1alpha1.DescriptionPhaseFailure:
					status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() +
						" is failed, reason: " + desc.Status.Reason + "; "
				case v1alpha1.DescriptionPhasePending:
					status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() +
						" is pending, reason: " + desc.Status.Reason + "; "
				case v1alpha1.DescriptionPhaseReSchedule:
					status.Reason = status.Reason + desc.GetNamespace() + "/" + desc.GetName() + " is rescheduling; "
				}
			}
		}

		status.Phase = parentDescCopy.Status.Phase
		// Update the parent description status with local description status
		parentDescCopy.Status = status
		err = utils.UpdateDescriptionStatus(r.gaiaClient, parentDescCopy)
		if err != nil {
			klog.Errorf("Failed to update parent description status: %v", err)
		}
	}
	klog.V(5).Infof("end to update collected global description status")
}

func (r *RBController) collectResourceBindingStatus(ctx context.Context) {
	klog.V(5).Infof("start to collect resource binding status")
	rbs, err := r.rbsLister.ResourceBindings(common.GaiaRBMergedReservedNamespace).List(labels.Everything())
	if err != nil {
		klog.Errorf("Failed to list resource bindings: %v", err)
		return
	}

	for _, rb := range rbs {
		parentRB, err := r.parentGaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).Get(ctx,
			rb.Name, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("Failed to get parent resource binding: %s, error==%v", rb.GetName(), err)
			continue
		}

		hyperlabelInfo, err := getNewServiceIPInfo(rb.Status.ServiceIPInfo, parentRB.Status.ServiceIPInfo)
		if err != nil {
			klog.Errorf("Failed to get new service ip info: %v", err)
		}
		parentRB.Status.ServiceIPInfo = hyperlabelInfo
		err = utils.UpdateReouceBingdingStatus(r.parentGaiaClient, parentRB)
		if err != nil {
			klog.Errorf("Failed to update parent resource binding status: %v", err)
		}
	}
	klog.V(5).Infof("end to update collected resource binding status")
}

func getNewServiceIPInfo(localInfo, parentInfo []byte) ([]byte, error) {
	if len(parentInfo) == 0 {
		return localInfo, nil
	}

	var newInfo []byte
	localLabelInfoMap, parentLabelInfoMap := make(map[string]map[string]string), make(map[string]map[string]string)
	err := json.Unmarshal(localInfo, &localLabelInfoMap)
	if err != nil {
		klog.Errorf("Failed to unmarshal hyperlabel info: %v", err)
		return nil, err
	}
	err = json.Unmarshal(parentInfo, &parentLabelInfoMap)
	if err != nil {
		klog.Errorf("Failed to unmarshal parent hyperlabel info: %v", err)
		return nil, err
	}

	for comName, resIPs := range localLabelInfoMap {
		if _, ok := parentLabelInfoMap[comName]; ok {
			for resID, ip := range resIPs {
				parentLabelInfoMap[comName][resID] = ip
			}
		} else {
			parentLabelInfoMap[comName] = resIPs
		}
	}
	newInfo, err = json.Marshal(parentLabelInfoMap)
	if err != nil {
		klog.Errorf("Failed to marshal new parent hyperlabel info: %v", err)
	}
	return newInfo, err
}
