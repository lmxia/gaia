package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/lmxia/gaia/pkg/apis/service/v1alpha1"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	v1alpha2 "github.com/lmxia/gaia/pkg/generated/listers/service/v1alpha1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	pkgruntime "k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	"github.com/lmxia/gaia/pkg/common"
	"github.com/mattbaird/jsonpatch"
)

func SetCreatedByLabel(obj *metav1.ObjectMeta) {
	if obj.Labels == nil {
		obj.Labels = map[string]string{}
	}
	obj.Labels[common.ServiceCreatedByLabel] = common.CletName
}

// LetItGoOn only accept LoadBalancer type service in public serverless cloud.
func LetItGoOn(service *v1.Service) bool {
	if ShoulIgnoreThisResource(service.Namespace) {
		return false
	}
	if len(service.Labels) > 0 && service.Spec.Type == v1.ServiceTypeLoadBalancer {
		_, exitManagedbyExist := service.Labels[common.ServiceManagedByLabel]
		if !exitManagedbyExist {
			return false
		}
		if owner, ownerExist := service.Labels[common.ServiceOwnerByLabel]; ownerExist {
			return owner != common.ServicelessOwner
		} else {
			return true
		}
	}
	return false
}

func ShoulIgnoreThisResource(namespace string) bool {
	// need to ignore begins with.
	excludedPrefixes := []string{
		common.GaiaRelatedNamespaces,
		common.KubeSystemRelatedNamespaces,
		common.MonitorNamespaces,
	}

	for _, prefix := range excludedPrefixes {
		if strings.HasPrefix(namespace, prefix) {
			return true
		}
	}

	return false
}

// ApplyServiceWithRetry create or update existed service.
func ApplyServiceWithRetry(k8sClient kubernetes.Interface, service *v1.Service) error {
	return retry.RetryOnConflict(retry.DefaultBackoff, func() error {
		var lastError error
		_, lastError = k8sClient.CoreV1().Services(service.GetNamespace()).
			Create(context.TODO(), service, metav1.CreateOptions{})
		if lastError == nil {
			return nil
		}
		if !errors.IsAlreadyExists(lastError) {
			return lastError
		}

		// 一定会有变化的话，否则不会走到这里。
		_, lastError = k8sClient.CoreV1().Services(service.GetNamespace()).
			Update(context.TODO(), service, metav1.UpdateOptions{})
		if lastError == nil {
			return nil
		}

		updated, err2 := k8sClient.CoreV1().Services(service.GetNamespace()).Get(
			context.TODO(), service.Name, metav1.GetOptions{})
		if err2 == nil {
			// make a copy, so we don't mutate the shared cache
			service = updated.DeepCopy()
		}

		return lastError
	})
}

func RemoveNonexistentService(targetClient kubernetes.Interface, srcServiceMap map[string]bool,
	desList []*v1.Service,
) error {
	for _, item := range desList {
		if !srcServiceMap[item.Name] {
			if err := targetClient.CoreV1().Services(item.Namespace).Delete(context.TODO(),
				item.Name, metav1.DeleteOptions{}); err != nil {
				utilruntime.HandleError(fmt.Errorf("the service"+
					" '%s/%s' deleted failed", item.Namespace, item.Name))
				return err
			}
		}
	}
	return nil
}

func UpdateHyperLabelStatus(client *gaiaclientset.Clientset, hyperLabelLister v1alpha2.HyperLabelLister,
	hyperlabel *v1alpha1.HyperLabel) error {
	klog.V(5).Infof("try to update hyperlabel %q status", hyperlabel.Name)
	return retry.RetryOnConflict(retry.DefaultRetry, func() error {
		_, err := client.ServiceV1alpha1().HyperLabels(hyperlabel.Namespace).UpdateStatus(
			context.TODO(), hyperlabel, metav1.UpdateOptions{})
		if err == nil {
			return nil
		}

		hyperlabelNewed, errGet := hyperLabelLister.HyperLabels(hyperlabel.Namespace).Get(hyperlabel.Name)
		if errGet == nil {
			hyperlabel = hyperlabelNewed.DeepCopy()
		} else {
			utilruntime.HandleError(fmt.Errorf("error get updated %q from lister: %v", hyperlabel.Name, errGet))
		}
		return err
	})
}

func UpdateResourceBindingHyperLabel(client *gaiaclientset.Clientset, hyperlabel *v1alpha1.HyperLabel) error {
	rb, err := client.AppsV1alpha1().ResourceBindings(common.GaiaRBMergedReservedNamespace).Get(context.TODO(),
		hyperlabel.Name, metav1.GetOptions{})
	if err != nil {
		klog.Errorf("failed to get rb: %v", err)
		return err
	}
	ipInfo, err := json.Marshal(hyperlabel.Status.PublicIPInfo)
	if err != nil {
		klog.Errorf("failed to marshal ipInfo: %v", err)
		return err
	}
	rb.Status.ServiceIPInfo = ipInfo
	err = UpdateReouceBingdingStatus(client, rb)
	if err != nil {
		klog.Errorf("failed to update rb status: %v", err)
		return err
	}

	return nil
}

// current is deployed resource, modified is changed resource.
// ignoreAdd is true if you want to ignore add action.
// The function will return the bool value to indicate whether to sync back the current object.
func ResourceNeedResync(current pkgruntime.Object, modified pkgruntime.Object, ignoreAdd bool) bool {
	currentBytes, err := json.Marshal(current)
	if err != nil {
		klog.ErrorDepth(5, fmt.Sprintf("Error marshal json: %v", err))
		return false
	}

	modifiedBytes, err := json.Marshal(modified)
	if err != nil {
		klog.ErrorDepth(5, fmt.Sprintf("Error marshal json: %v", err))
		return false
	}

	patch, err := jsonpatch.CreatePatch(currentBytes, modifiedBytes)
	if err != nil {
		klog.ErrorDepth(5, fmt.Sprintf("Error creating JSON patch: %v", err))
		return false
	}
	for _, operation := range patch {
		// filter ignored paths
		if shouldPatchBeIgnored(operation) {
			continue
		}

		switch operation.Operation {
		case "add":
			if ignoreAdd {
				continue
			} else {
				return true
			}
		case "remove", "replace":
			return true
		default:
			// skip other operations, like "copy", "move" and "test"
			continue
		}
	}

	return false
}

// shouldPatchBeIgnored used to decide if this patch operation should be ignored.
func shouldPatchBeIgnored(operation jsonpatch.JsonPatchOperation) bool {
	// some fields need to be ignore like meta.selfLink, meta.resourceVersion.
	if ContainsString(fieldsToBeIgnored(), operation.Path) {
		return true
	}
	// some sections like status section need to be ignored.
	if ContainsPrefix(sectionToBeIgnored(), operation.Path) {
		return true
	}

	return false
}

func sectionToBeIgnored() []string {
	return []string{
		common.SectionStatus,
	}
}

func fieldsToBeIgnored() []string {
	return []string{
		common.MetaGeneration,
		common.CreationTimestamp,
		common.ManagedFields,
		common.MetaUID,
		common.MetaSelflink,
		common.MetaResourceVersion,
	}
}

func GetLoadbalancerIP(service *v1.Service) (string, []string) {
	var ips []string
	for _, ing := range service.Status.LoadBalancer.Ingress {
		ips = append(ips, ing.IP)
	}
	// 一定会有的，不需要判断
	return service.GetLabels()[common.ServiceManagedByLabel], ips
}
