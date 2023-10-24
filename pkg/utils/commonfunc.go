package utils

import (
	"context"
	"encoding/json"

	"k8s.io/api/core/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	externalInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
)

const (
	// DefaultMilliCPURequest defines default milli cpu request number.
	DefaultMilliCPURequest int64 = 100 // 0.1 core
	// DefaultMemoryRequest defines default memory request size.
	DefaultMemoryRequest int64 = 200 * 1024 * 1024 // 200 MB
)

func UnstructuredConvertToStruct(structObj interface{}, des interface{}) error {
	var data []byte
	var err error
	if data, err = json.Marshal(structObj); err != nil {
		return err
	}
	err = json.Unmarshal(data, des)
	return err
}

func ObjectConvertToUnstructured(object runtime.Object) (*unstructured.Unstructured, error) {
	var raw []byte
	var err error
	if raw, err = json.Marshal(object); err != nil {
		return nil, err
	}
	unstructed := &unstructured.Unstructured{}
	err = unstructed.UnmarshalJSON(raw)
	if err != nil {
		return nil, err
	}
	return unstructed, nil
}

func GetLocalClusterName(localkubeclient *kubernetes.Clientset) (string, string, error) {
	var clusterName string
	secret, err := localkubeclient.CoreV1().Secrets(known.GaiaSystemNamespace).Get(context.TODO(), known.ParentClusterSecretName, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("failed to get clustername  From secret,error==: %v", err)
		return clusterName, "", err
	}
	parentNs := string(secret.Data[v1.ServiceAccountNamespaceKey])
	if len(secret.Labels) > 0 {
		clusterName = secret.Labels[known.ClusterNameLabel]
	}
	if len(clusterName) <= 0 || len(parentNs) <= 0 {
		klog.Errorf("failed to get clustername  From secret labels. ")
		return clusterName, parentNs, err
	}

	return clusterName, parentNs, nil
}

func NewParentConfig(ctx context.Context, kubeclient *kubernetes.Clientset, gaiaclient *gaiaClientSet.Clientset) *rest.Config {
	var parentKubeConfig *rest.Config
	// wait until stopCh is closed or request is approved
	waitingCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	wait.JitterUntilWithContext(waitingCtx, func(ctx context.Context) {
		target, err := gaiaclient.PlatformV1alpha1().Targets().Get(ctx, common.ParentClusterTargetName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("set parentkubeconfig failed to get targets: %v wait for next loop", err)
			return
		}
		secret, err := kubeclient.CoreV1().Secrets(common.GaiaSystemNamespace).Get(ctx, common.ParentClusterSecretName, metav1.GetOptions{})
		if err != nil {
			klog.Errorf("set parentkubeconfig failed to get secretFromParentCluster: %v", err)
			return
		}
		if err == nil {
			klog.Infof("found existing secretFromParentCluster '%s/%s' that can be used to access parent cluster", common.GaiaSystemNamespace, common.ParentClusterSecretName)
			parentKubeConfig, err = GenerateKubeConfigFromToken(target.Spec.ParentURL, string(secret.Data[v1.ServiceAccountTokenKey]), secret.Data[v1.ServiceAccountRootCAKey], 2)
			if err != nil {
				klog.Errorf("set parentkubeconfig failed to get sa and secretFromParentCluster: %v", err)
				return
			}
		}
		cancel()
	}, common.DefaultRetryPeriod*4, 0.3, true)

	return parentKubeConfig
}

func SetParentClient(localKubeClient *kubernetes.Clientset, localGaiaClient *gaiaClientSet.Clientset) (*gaiaClientSet.Clientset, dynamic.Interface, externalInformers.SharedInformerFactory) {
	parentKubeConfig := NewParentConfig(context.TODO(), localKubeClient, localGaiaClient)
	if parentKubeConfig != nil {
		parentGaiaClient := gaiaClientSet.NewForConfigOrDie(parentKubeConfig)
		parentDynamicClient, _ := dynamic.NewForConfig(parentKubeConfig)
		parentMergedGaiaInformerFactory := externalInformers.NewSharedInformerFactoryWithOptions(parentGaiaClient, known.DefaultResync,
			externalInformers.WithNamespace(known.GaiaRBMergedReservedNamespace))

		return parentGaiaClient, parentDynamicClient, parentMergedGaiaInformerFactory
	}
	return nil, nil, nil
}

func CalculateResource(templateSpec corev1.PodTemplateSpec) (non0CPU int64, non0Mem int64, pod *corev1.Pod) {
	pod = &corev1.Pod{
		ObjectMeta: templateSpec.ObjectMeta,
		Spec:       templateSpec.Spec,
	}

	for _, c := range templateSpec.Spec.Containers {
		non0CPUReq, non0MemReq := GetNonzeroRequests(&c.Resources.Requests)
		non0CPU += non0CPUReq
		non0Mem += non0MemReq
		// No non-zero resources for GPUs or opaque resources.
	}

	// If Overhead is being utilized, add to the total requests for the pod
	if templateSpec.Spec.Overhead != nil {
		if _, found := templateSpec.Spec.Overhead[corev1.ResourceCPU]; found {
			non0CPU += templateSpec.Spec.Overhead.Cpu().MilliValue()
		}

		if _, found := templateSpec.Spec.Overhead[corev1.ResourceMemory]; found {
			non0Mem += templateSpec.Spec.Overhead.Memory().Value()
		}
	}

	return
}

// GetNonzeroRequests returns the default cpu and memory resource request if none is found or
// what is provided on the request.
func GetNonzeroRequests(requests *corev1.ResourceList) (int64, int64) {
	return GetNonzeroRequestForResource(corev1.ResourceCPU, requests),
		GetNonzeroRequestForResource(corev1.ResourceMemory, requests)
}

// GetNonzeroRequestForResource returns the default resource request if none is found or
// what is provided on the request.
func GetNonzeroRequestForResource(resource corev1.ResourceName, requests *corev1.ResourceList) int64 {
	switch resource {
	case corev1.ResourceCPU:
		// Override if un-set, but not if explicitly set to zero
		if _, found := (*requests)[corev1.ResourceCPU]; !found {
			return DefaultMilliCPURequest
		}
		return requests.Cpu().MilliValue()
	case corev1.ResourceMemory:
		// Override if un-set, but not if explicitly set to zero
		if _, found := (*requests)[corev1.ResourceMemory]; !found {
			return DefaultMemoryRequest
		}
		return requests.Memory().Value()
	case corev1.ResourceEphemeralStorage:
		quantity, found := (*requests)[corev1.ResourceEphemeralStorage]
		if !found {
			return 0
		}
		return quantity.Value()
	default:
		return 0
	}
}
