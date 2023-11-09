package utils

import (
	"context"
	"fmt"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"
)

func UpdateDescriptionStatus(gaiaClient gaiaclientset.Interface, desc *v1alpha1.Description) error {
	klog.Infof("try to update Description %q status", desc.Name)
	status := desc.Status

	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := gaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).UpdateStatus(context.TODO(), desc, metav1.UpdateOptions{})
		if err == nil {
			// TODO
			return nil
		}

		updated, err2 := gaiaClient.AppsV1alpha1().Descriptions(desc.Namespace).Get(context.TODO(), desc.Name, metav1.GetOptions{})
		if err2 == nil {
			// make a copy, so we don't mutate the shared cache
			desc = updated.DeepCopy()
			desc.Status = status
		} else {
			utilruntime.HandleError(fmt.Errorf("error getting updated Description %q: %v", desc.Name, err2))
		}
		return err2
	})
}
