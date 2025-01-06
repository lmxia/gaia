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

func UpdateReouceBingdingStatus(client *gaiaclientset.Clientset, rb *v1alpha1.ResourceBinding) error {
	klog.V(5).Infof("try to update resourcebinding %q status", rb.Name)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := client.AppsV1alpha1().ResourceBindings(rb.Namespace).UpdateStatus(
			context.TODO(), rb, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}

		hyperlabelNewed, errGet := client.AppsV1alpha1().ResourceBindings(rb.Namespace).Get(context.TODO(),
			rb.Name, metav1.GetOptions{})
		if errGet == nil {
			rb = hyperlabelNewed.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error get updated %q from lister: %v", rb.Name, errGet))
		}
		return err
	})
}
