package appstatus

import (
	"context"
	"encoding/json"
	"time"

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
	// parentGaiaClient *gaiaClientSet.Clientset

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

func (s *StatusController) Run(ctx context.Context, parentKubeConfig *rest.Config) {
	klog.Infof("starting app-status-controller to report status...")
	parentGaiaClient, _ := utils.SetParentClientAndInformer(parentKubeConfig)

	go s.RBController.Run(ctx, parentGaiaClient)
}

func (r *RBController) Run(ctx context.Context, parentGaiaClient *gaiaClientSet.Clientset) {
	klog.Infof("starting resource-binding-status-controller to report status...")
	if parentGaiaClient == nil {
		klog.Errorf("Failed to get parent gaia client")
		return
	}
	r.parentGaiaClient = parentGaiaClient
	if !cache.WaitForCacheSync(ctx.Done(), r.rbsSynced) {
		return
	}

	wait.UntilWithContext(ctx, r.collectResourceBindingStatus, r.collectingPeriod)
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
