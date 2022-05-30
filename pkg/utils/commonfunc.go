package utils

import (
	"context"
	"encoding/json"
	"github.com/lmxia/gaia/pkg/common"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	externalInformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
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

func SetParentClient(localkubeclient *kubernetes.Clientset, localgaiaclient *gaiaClientSet.Clientset) (*gaiaClientSet.Clientset, dynamic.Interface, externalInformers.SharedInformerFactory) {
	parentKubeConfig := NewParentConfig(context.TODO(), localkubeclient, localgaiaclient)
	if parentKubeConfig != nil {
		parentGaiaClient := gaiaClientSet.NewForConfigOrDie(parentKubeConfig)
		parentDynamicClient, _ := dynamic.NewForConfig(parentKubeConfig)
		parentgaiaInformerFactory := externalInformers.NewSharedInformerFactoryWithOptions(parentGaiaClient, known.DefaultResync,
			externalInformers.WithNamespace(known.GaiaRBMergedReservedNamespace))

		return parentGaiaClient, parentDynamicClient, parentgaiaInformerFactory
	}
	return nil, nil, nil
}
