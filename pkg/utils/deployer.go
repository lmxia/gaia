package utils

import (
	"bytes"
	"context"
	"fmt"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"net/http"
	"strings"
	"sync"

	"encoding/json"
	lmmserverless "github.com/SUMMERLm/serverless/api/v1"
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/retry"
	"k8s.io/klog/v2"

	known "github.com/lmxia/gaia/pkg/common"
)

func DeleteResourceWithRetry(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, resource *unstructured.Unstructured) error {
	deletePropagationBackground := metav1.DeletePropagationBackground

	var lastError error
	err := wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
		restMapping, err := restMapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
		if err != nil {
			lastError = fmt.Errorf("please check whether the advertised apiserver of current child cluster is accessible. %v", err)
			return false, nil
		}

		lastError = dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Delete(context.TODO(), resource.GetName(), metav1.DeleteOptions{PropagationPolicy: &deletePropagationBackground})
		if lastError == nil || (lastError != nil && apierrors.IsNotFound(lastError)) {
			return true, nil
		}
		return false, nil
	})
	if err == nil {
		return nil
	}
	return lastError
}

// copied from k8s.io/apimachinery/pkg/apis/meta/v1/unstructured
func getNestedString(obj map[string]interface{}, fields ...string) string {
	val, found, err := unstructured.NestedString(obj, fields...)
	if !found || err != nil {
		return ""
	}
	return val
}

// copied from k8s.io/apimachinery/pkg/apis/meta/v1/unstructured
// and modified
func setNestedField(u *unstructured.Unstructured, value interface{}, fields ...string) {
	if u.Object == nil {
		u.Object = make(map[string]interface{})
	}
	err := unstructured.SetNestedField(u.Object, value, fields...)
	if err != nil {
		klog.Warningf("failed to set nested field: %v", err)
	}
}

// getStatusCause returns the named cause from the provided error if it exists and
// the error is of the type APIStatus. Otherwise it returns false.
func getStatusCause(err error) ([]metav1.StatusCause, bool) {
	apierr, ok := err.(apierrors.APIStatus)
	if !ok || apierr == nil || apierr.Status().Details == nil {
		return nil, false
	}
	return apierr.Status().Details.Causes, true
}

func GetDeployerCredentials(ctx context.Context, childKubeClientSet kubernetes.Interface, sa string) *corev1.Secret {
	var secret *corev1.Secret
	localCtx, cancel := context.WithCancel(ctx)

	klog.V(4).Infof("get ServiceAccount %s/%s", known.GaiaSystemNamespace, sa)
	wait.JitterUntilWithContext(localCtx, func(ctx context.Context) {
		sa, err := childKubeClientSet.CoreV1().ServiceAccounts(known.GaiaSystemNamespace).Get(ctx, sa, metav1.GetOptions{})
		if err != nil {
			klog.ErrorDepth(5, fmt.Errorf("failed to get ServiceAccount %s/%s: %v", known.GaiaSystemNamespace, sa, err))
			return
		}

		if len(sa.Secrets) == 0 {
			klog.ErrorDepth(5, fmt.Errorf("no secrets found in ServiceAccount %s/%s", known.GaiaSystemNamespace, sa))
			return
		}

		secret, err = childKubeClientSet.CoreV1().Secrets(known.GaiaSystemNamespace).Get(ctx, sa.Secrets[0].Name, metav1.GetOptions{})
		if err != nil {
			klog.ErrorDepth(5, fmt.Errorf("failed to get Secret %s/%s: %v", known.GaiaSystemNamespace, sa.Secrets[0].Name, err))
			return
		}

		cancel()
	}, known.DefaultRetryPeriod, 0.4, true)

	klog.V(4).Info("successfully get credentials populated for deployer")
	return secret
}

func ApplyResourceWithRetry(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, resource *unstructured.Unstructured) error {
	// set UID as empty
	resource.SetUID("")

	var lastError error
	err := wait.ExponentialBackoffWithContext(ctx, retry.DefaultBackoff, func() (bool, error) {
		restMapping, err := restMapper.RESTMapping(resource.GroupVersionKind().GroupKind(), resource.GroupVersionKind().Version)
		if err != nil {
			lastError = fmt.Errorf("error===%v \n", err)
			return false, nil
		}

		if len(resource.GetNamespace()) > 0 {
			lastError = CreatNSIdNeed(dynamicClient, restMapper, resource.GetNamespace())
			if lastError != nil && !apierrors.IsAlreadyExists(lastError) {
				err = fmt.Errorf("create  ns %s error===.%v \n", lastError, err)
				return false, nil
			}
		}

		_, lastError = dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Create(context.TODO(), resource, metav1.CreateOptions{})
		if lastError == nil {
			return true, nil
		}
		if !apierrors.IsAlreadyExists(lastError) {
			return false, nil
		}

		curObj, err := dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Get(context.TODO(), resource.GetName(), metav1.GetOptions{})
		if err != nil {
			lastError = err
			return false, nil
		} else {
			lastError = nil
		}

		// try to update resource
		_, lastError = dynamicClient.Resource(restMapping.Resource).Namespace(resource.GetNamespace()).
			Update(context.TODO(), resource, metav1.UpdateOptions{})
		if lastError == nil {
			return true, nil
		}
		statusCauses, ok := getStatusCause(lastError)
		if !ok {
			lastError = fmt.Errorf("failed to get StatusCause for %s %s", resource.GetKind(), klog.KObj(resource))
			return false, nil
		}
		resourceCopy := resource.DeepCopy()
		for _, cause := range statusCauses {
			if cause.Type != metav1.CauseTypeFieldValueInvalid {
				continue
			}
			// apply immutable value
			fields := strings.Split(cause.Field, ".")
			setNestedField(resourceCopy, getNestedString(curObj.Object, fields...), fields...)
		}
		// update with immutable values applied
		_, lastError = dynamicClient.Resource(restMapping.Resource).Namespace(resourceCopy.GetNamespace()).
			Update(context.TODO(), resourceCopy, metav1.UpdateOptions{})
		if lastError == nil {
			return true, nil
		}
		return false, nil
	})

	if err == nil {
		return nil
	}
	return lastError
}

func GetDescription(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, name, desNs string) (*appsv1alpha1.Description, error) {
	var descriptionsKind = schema.GroupVersionKind{Group: "apps.gaia.io", Version: "v1alpha1", Kind: "Description"}
	restMapping, err := restMapper.RESTMapping(descriptionsKind.GroupKind(), descriptionsKind.Version)
	if err != nil {
		klog.Errorf("cannot get its descrito %v", err)
		return nil, err
	}
	curObj, queryErr := dynamicClient.Resource(restMapping.Resource).Namespace(desNs).Get(ctx, name, metav1.GetOptions{})
	if queryErr != nil && !apierrors.IsNotFound(queryErr) {
		return nil, queryErr
	}
	des := &appsv1alpha1.Description{}
	if err = UnstructuredConvertToStruct(curObj, des); err != nil {
		return nil, err
	}
	return des, nil
}

func GetNetworkRequirement(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, name, desNs string) (*appsv1alpha1.NetworkRequirement, error) {
	var kind = schema.GroupVersionKind{Group: "apps.gaia.io", Version: "v1alpha1", Kind: "NetworkRequirement"}
	restMapping, err := restMapper.RESTMapping(kind.GroupKind(), kind.Version)
	if err != nil {
		klog.Errorf("cannot get networkRequirement= %v", err)
		return nil, err
	}
	curObj, queryErr := dynamicClient.Resource(restMapping.Resource).Namespace(desNs).Get(ctx, name, metav1.GetOptions{})
	if queryErr != nil && !apierrors.IsNotFound(queryErr) {
		return nil, queryErr
	}
	nwr := &appsv1alpha1.NetworkRequirement{}
	if err = UnstructuredConvertToStruct(curObj, nwr); err != nil {
		return nil, err
	}
	return nwr, nil
}

func OffloadRBWorkloads(ctx context.Context, desc *appsv1alpha1.Description, parentGaiaclient *gaiaClientSet.Clientset, localDynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, rb *appsv1alpha1.ResourceBinding, clusterName string) error {
	var allErrs []error
	var err error
	wg := sync.WaitGroup{}
	comToBeDeleted := desc.Spec.Components
	errCh := make(chan error, len(comToBeDeleted))
	for _, com := range comToBeDeleted {
		switch com.Workload.Workloadtype {
		case appsv1alpha1.WorkloadTypeDeployment:
			depunstructured, deperr := AssembledDeploymentStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, true)
			if deperr != nil || depunstructured == nil {
				continue
			}
			wg.Add(1)
			go func(depunstructured *unstructured.Unstructured) {
				defer wg.Done()
				klog.V(5).Infof("deleting %s %s defined in ResourceBinding %s", depunstructured.GetKind(),
					klog.KObj(depunstructured), klog.KObj(rb))
				err2 := DeleteResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, depunstructured)
				if err2 != nil {
					klog.Infof("offloadRBWorkloads Deployment name==%s err===%v \n", depunstructured.GetName(), err2)
					errCh <- err2
				}
			}(depunstructured)
		case appsv1alpha1.WorkloadTypeServerless:
			SerUn, errservice := AssembledServerlessStructure(com, rb.Spec.RbApps, clusterName, desc.Name, true)
			if errservice != nil || SerUn == nil {
				continue
			}
			wg.Add(1)
			go func(SerUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := DeleteResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, SerUn)
				if retryErr != nil {
					klog.Infof("offloadRBWorkloads Serverless name==%s err===%v \n", SerUn.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(SerUn)
		case appsv1alpha1.WorkloadTypeAffinityDaemon:
			depunstructured, deperr := AssembledDeamonsetStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, true)
			if deperr != nil || depunstructured == nil {
				continue
			}
			wg.Add(1)
			go func(depunstructured *unstructured.Unstructured) {
				defer wg.Done()
				klog.V(5).Infof(" workloadTypeAffinityDaemon deleting %s %s defined in ResourceBinding %s", depunstructured.GetKind(),
					klog.KObj(depunstructured), klog.KObj(rb))
				err2 := DeleteResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, depunstructured)
				if err2 != nil {
					klog.Infof("offloadRBWorkloads WorkloadTypeAffinityDaemon name==%s err===%v \n", depunstructured.GetName(), err2)
					errCh <- err2
				}
			}(depunstructured)
		case appsv1alpha1.WorkloadTypeUserApp:
			depunstructured, deperr := AssembledUserAppStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, true)
			if deperr != nil || depunstructured == nil {
				continue
			}
			wg.Add(1)
			go func(depunstructured *unstructured.Unstructured) {
				defer wg.Done()
				klog.V(5).Infof(" userApp deleting %s %s defined in ResourceBinding %s", depunstructured.GetKind(),
					klog.KObj(depunstructured), klog.KObj(rb))
				err2 := DeleteResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, depunstructured)
				if err2 != nil {
					klog.Infof("offloadRBWorkloads userApp name==%s err===%v \n", depunstructured.GetName(), err2)
					errCh <- err2
				}
			}(depunstructured)
		}
		wg.Wait()
	}

	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}

	err = utilerrors.NewAggregate(allErrs)
	if err != nil {
		msg := fmt.Sprintf("failed to deleting Description %s: %v", klog.KObj(desc), err)
		klog.ErrorDepth(5, msg)
	}
	return err
}

func ApplyRBWorkloads(ctx context.Context, desc *appsv1alpha1.Description, parentGaiaClient *gaiaClientSet.Clientset, localDynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, rb *appsv1alpha1.ResourceBinding, clusterName string) error {
	var allErrs []error

	wg := sync.WaitGroup{}
	comToBeApply := desc.Spec.Components
	errCh := make(chan error, len(comToBeApply))
	for _, com := range comToBeApply {
		switch com.Workload.Workloadtype {
		case appsv1alpha1.WorkloadTypeDeployment:
			unStructure, errDep := AssembledDeploymentStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, false)
			if errDep != nil || unStructure == nil || unStructure.Object == nil || len(unStructure.GetName()) == 0 {
				continue
			}
			wg.Add(1)
			go func(unstructure *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, unstructure)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads Deployment name==%s err===%v \n", unstructure.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(unStructure)
		case appsv1alpha1.WorkloadTypeServerless:
			unstructure, errServiceless := AssembledServerlessStructure(com, rb.Spec.RbApps, clusterName, desc.Name, false)
			if errServiceless != nil || unstructure == nil || unstructure.Object == nil || len(unstructure.GetName()) == 0 {
				continue
			}
			wg.Add(1)
			go func(unstructure *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, unstructure)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads Serverless name==%s err===%v \n", unstructure.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(unstructure)
		case appsv1alpha1.WorkloadTypeAffinityDaemon:
			unstructure, errdep := AssembledDeamonsetStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, false)
			if errdep != nil || unstructure == nil || unstructure.Object == nil || len(unstructure.GetName()) == 0 {
				continue
			}
			wg.Add(1)
			go func(unstructure *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, unstructure)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads WorkloadTypeAffinityDaemon name==%s err===%v \n", unstructure.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(unstructure)
		case appsv1alpha1.WorkloadTypeUserApp:
			unstructure, errUserapp := AssembledUserAppStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, false)
			if errUserapp != nil || unstructure == nil || unstructure.Object == nil || len(unstructure.GetName()) == 0 {
				continue
			}
			wg.Add(1)
			go func(unstructure *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, unstructure)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads userApp name==%s err===%v \n", unstructure.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(unstructure)
		}
		wg.Wait()
	}

	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}

	var statusScheduler string
	var reason string
	if len(allErrs) > 0 {
		statusScheduler = known.ResourceBindingRed
		reason = utilerrors.NewAggregate(allErrs).Error()

		msg := fmt.Sprintf("failed to ResourceBindings deploying  %s: %s", klog.KObj(rb), reason)
		klog.ErrorDepth(5, msg)
	} else {
		statusScheduler = known.ResourceBindingGreen
		reason = ""

		msg := fmt.Sprintf("ResourceBindings %s is deployed successfully", klog.KObj(rb))
		klog.V(5).Info(msg)
	}

	// update status
	rb.Status.Status = statusScheduler
	rb.Status.Reason = reason
	_, err := parentGaiaClient.AppsV1alpha1().ResourceBindings(rb.Namespace).UpdateStatus(context.TODO(), rb, metav1.UpdateOptions{})

	if len(allErrs) > 0 {
		return utilerrors.NewAggregate(allErrs)
	}
	return err
}

func ApplyResourceBinding(ctx context.Context, localdynamicClient dynamic.Interface, discoveryRESTMapper meta.RESTMapper,
	rb *appsv1alpha1.ResourceBinding, clusterName, descriptionName, networkBindUrl string, nwr *appsv1alpha1.NetworkRequirement) error {
	var allErrs []error
	var err error
	errCh := make(chan error, len(rb.Spec.RbApps))
	wg := sync.WaitGroup{}
	for _, rbApp := range rb.Spec.RbApps {
		if rbApp.ClusterName == clusterName {
			newRB := &appsv1alpha1.ResourceBinding{
				TypeMeta: metav1.TypeMeta{
					Kind:       "ResourceBinding",
					APIVersion: "apps.gaia.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Name:   rb.Name,
					Labels: rb.Labels,
					Finalizers: []string{
						known.AppFinalizer,
					},
				},
				Spec: appsv1alpha1.ResourceBindingSpec{
					AppID:           rb.Spec.AppID,
					StatusScheduler: rb.Spec.StatusScheduler,
					TotalPeer:       rb.Spec.TotalPeer,
					ParentRB:        rb.Name,
					RbApps:          rbApp.Children,
					NetworkPath:     rb.Spec.NetworkPath,
				},
			}
			if len(rb.Namespace) > 0 {
				newRB.Namespace = rb.Namespace
			}
			rbUnstructured, errdep := ObjectConvertToUnstructured(newRB)
			if errdep != nil || rbUnstructured == nil {
				msg := fmt.Sprintf("apply RB in field %s failed to unmarshal resource: %v", newRB.ClusterName, errdep)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, errdep)
				continue
			}
			wg.Add(1)
			go func(rbUnstructured *unstructured.Unstructured) {
				defer wg.Done()
				err = ApplyResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, rbUnstructured)
				if err != nil {
					errCh <- err
					return
				}
			}(rbUnstructured)
			wg.Wait()

			if len(newRB.Spec.NetworkPath) > 0 && len(networkBindUrl) > 0 && nwr != nil {
				if NeedBindNetworkInCluster(rb.Spec.RbApps, clusterName, nwr) {
					postRequest(networkBindUrl, descriptionName, newRB.Spec.NetworkPath[0])
				}
			}
		}
	}

	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}
	if len(allErrs) > 0 {
		return utilerrors.NewAggregate(allErrs)
	}
	return err
}

type NetworkScheme struct {

	// whether resource reserved or not
	IsResouceReserved bool `json:"isResouceReserved,omitempty"`

	// network scheme path
	Path []byte `json:"path,omitempty"`

	// buleprint ID
	BuleprintID string `json:"buleprintID,omitempty"`
}

func postRequest(url, descriptionName string, path []byte) {
	networkScheme := NetworkScheme{
		IsResouceReserved: false,
		Path:              path,
		BuleprintID:       descriptionName,
	}
	data, jsonerr := json.Marshal(networkScheme)
	if jsonerr != nil {
		klog.Errorf("request  post new request, error=%v \n", jsonerr)
	}
	request, err := http.NewRequest("POST", url, bytes.NewBuffer(data))
	if err != nil {
		klog.Errorf("request  post new request, error=%v \n", err)
	}
	request.Header.Add("Content-Type", "application/json")
	request.Header.Add("cache-control", "no-cache")
	resp, resperr := http.DefaultClient.Do(request)
	if resperr != nil {
		klog.Errorf("post do sent, error====%v\n", resperr)
	}
	defer resp.Body.Close()
}

func AssembledDeamonsetStructure(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName, descName string, delete bool) (*unstructured.Unstructured, error) {
	depUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			replicas := rbApp.Replicas[com.Name]
			if replicas > 0 {
				ds := &appsv1.DaemonSet{
					TypeMeta: metav1.TypeMeta{
						Kind:       "DaemonSet",
						APIVersion: "apps/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							known.GaiaDescriptionLabel: descName,
							known.GaiaComponentLabel:   com.Name,
						},
					}}
				if len(com.Namespace) > 0 {
					ds.Namespace = com.Namespace
				} else {
					ds.Namespace = metav1.NamespaceDefault
				}
				ds.Name = com.Name

				if !delete {
					ds.Spec.Template = com.Module
					nodeAffinity := AddNodeAffinity(com)
					ds.Spec.Template.Spec.Affinity = nodeAffinity

					label := ds.GetLabels()
					ds.Spec.Template.Labels = label
					ds.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
					depUnstructured, err = ObjectConvertToUnstructured(ds)
				} else {
					depUnstructured, err = ObjectConvertToUnstructured(ds)
				}
			}
			break
		}
	}

	if err != nil {
		msg := fmt.Sprintf("deamonset failed to unmarshal resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}
	return depUnstructured, nil
}

func AssembledUserAppStructure(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName, descName string, delete bool) (*unstructured.Unstructured, error) {
	depUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			replicas := rbApp.Replicas[com.Name]
			if replicas == 0 {
				return nil, nil
			}
			userAPP := &appsv1alpha1.UserAPP{
				TypeMeta: metav1.TypeMeta{
					Kind:       "UserAPP",
					APIVersion: "apps.gaia.io/v1alpha1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						known.GaiaDescriptionLabel: descName,
						known.GaiaComponentLabel:   com.Name,
					},
				},
			}
			userAPP.Name = com.Name
			if !delete {
				userAPP.Spec.Module = com.Module
				userAPP.Spec.SN = com.Workload.TraitUserAPP.SN
				label := userAPP.GetLabels()
				userAPP.Spec.Module.Labels = label
				depUnstructured, err = ObjectConvertToUnstructured(userAPP)
			} else {
				depUnstructured, err = ObjectConvertToUnstructured(userAPP)
			}
			break
		}
	}

	if err != nil {
		msg := fmt.Sprintf("userAPP failed to unmarshal resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}
	return depUnstructured, nil
}

func AddNodeAffinity(com *appsv1alpha1.Component) *corev1.Affinity {
	nodeAffinity := &corev1.Affinity{
		NodeAffinity: &corev1.NodeAffinity{
			PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{{
				Weight: 2,
				Preference: corev1.NodeSelectorTerm{
					MatchExpressions: []corev1.NodeSelectorRequirement{{
						Key:      v1alpha1.ParsedResFormKey,
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"pool", "Pool"},
					}},
				},
			}},
		},
	}

	if com.Workload.Workloadtype == appsv1alpha1.WorkloadTypeAffinityDaemon {
		if len(com.Workload.TraitAffinityDaemon.SNS) > 0 {
			nodeSelectorTermSNs := corev1.NodeSelectorTerm{
				MatchExpressions: []corev1.NodeSelectorRequirement{{
					Key:      v1alpha1.ParsedSNKey,
					Operator: corev1.NodeSelectorOpIn,
					Values:   com.Workload.TraitAffinityDaemon.SNS,
				}},
			}
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = make([]corev1.NodeSelectorTerm, 0)
			}
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
				append(nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, nodeSelectorTermSNs)
			return nodeAffinity
		}
	}

	if com.SchedulePolicy.Provider != nil && com.SchedulePolicy.Provider.MatchExpressions != nil {
		providers := setNodeSelectorTerms(com.SchedulePolicy.Provider.MatchExpressions)
		if len(providers) > 0 {
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{}
			}
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
				append(nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, providers...)
		}
	}

	if com.SchedulePolicy.NetEnvironment != nil && com.SchedulePolicy.NetEnvironment.MatchExpressions != nil {
		netEnvironments := setNodeSelectorTerms(com.SchedulePolicy.NetEnvironment.MatchExpressions)
		if len(netEnvironments) > 0 {
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{}
			}
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
				append(nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, netEnvironments...)
		}
	}

	if com.SchedulePolicy.SpecificResource != nil && com.SchedulePolicy.SpecificResource.MatchExpressions != nil {
		specificResources := setNodeSelectorTerms(com.SchedulePolicy.SpecificResource.MatchExpressions)
		if len(specificResources) > 0 {
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{}
			}
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
				append(nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, specificResources...)
		}
	}

	if com.SchedulePolicy.GeoLocation != nil && com.SchedulePolicy.GeoLocation.MatchExpressions != nil {
		geoLocations := setNodeSelectorTerms(com.SchedulePolicy.GeoLocation.MatchExpressions)
		if len(geoLocations) > 0 {
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
			}
			if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
				nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms = []corev1.NodeSelectorTerm{}
			}
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
				append(nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms, geoLocations...)
		}
	}
	return nodeAffinity
}
func AssembledDeploymentStructure(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName, descName string, delete bool) (*unstructured.Unstructured, error) {
	depUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			replicas := rbApp.Replicas[com.Name]
			if replicas <= 0 {
				return nil, fmt.Errorf("deployment name==%s, have zero replicas", com.Name)
			}
			dep := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						known.GaiaDescriptionLabel: descName,
						known.GaiaComponentLabel:   com.Name,
					},
				}}
			if len(com.Namespace) > 0 {
				dep.Namespace = com.Namespace
			} else {
				dep.Namespace = metav1.NamespaceDefault
			}
			dep.Name = com.Name

			if !delete {
				dep.Spec.Template = com.Module
				nodeAffinity := AddNodeAffinity(com)
				dep.Spec.Template.Spec.Affinity = nodeAffinity
				if dep.Spec.Template.Spec.NodeSelector == nil {
					dep.Spec.Template.Spec.NodeSelector = map[string]string{
						v1alpha1.ParsedRuntimeStateKey: com.RuntimeType,
					}
				}

				dep.Spec.Replicas = &replicas
				label := dep.GetLabels()
				dep.Spec.Template.Labels = label
				dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
				depUnstructured, err = ObjectConvertToUnstructured(dep)
			} else {
				depUnstructured, err = ObjectConvertToUnstructured(dep)
			}
			break
		}
	}

	if err != nil {
		msg := fmt.Sprintf("deployment failed to unmarshal resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}

	return depUnstructured, nil
}

func setMatchExpressions(matchExpressions []metav1.LabelSelectorRequirement) []corev1.NodeSelectorRequirement {
	nsRequirements := []corev1.NodeSelectorRequirement{}
	for _, expression := range matchExpressions {
		express := corev1.NodeSelectorRequirement{
			Key:      known.SpecificNodeLabelsKeyPrefix + expression.Key,
			Operator: corev1.NodeSelectorOperator(expression.Operator),
			Values:   expression.Values,
		}
		nsRequirements = append(nsRequirements, express)
	}
	return nsRequirements
}

func setNodeSelectorTerms(matchExpressions []metav1.LabelSelectorRequirement) []corev1.NodeSelectorTerm {
	nsRequirements := setMatchExpressions(matchExpressions)
	nodeSelectorTerms := []corev1.NodeSelectorTerm{}
	nodeSelectorTerm := corev1.NodeSelectorTerm{
		MatchExpressions: nsRequirements,
	}
	if len(nsRequirements) > 0 {
		nodeSelectorTerms = append(nodeSelectorTerms, nodeSelectorTerm)

	}
	return nodeSelectorTerms
}

func AssembledServerlessStructure(com appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName, descName string, delete bool) (*unstructured.Unstructured, error) {
	serlessUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			replicas := rbApp.Replicas[com.Name]
			if replicas > 0 {
				ser := &lmmserverless.Serverless{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Serverless",
						APIVersion: "serverless.pml.com.cn/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Labels: map[string]string{
							known.GaiaDescriptionLabel: descName,
							known.GaiaComponentLabel:   com.Name,
						},
					},
				}
				if len(com.Namespace) > 0 {
					ser.Namespace = com.Namespace
				} else {
					ser.Namespace = metav1.NamespaceDefault
				}
				ser.Name = com.Name

				if !delete {
					ser.Spec = com
					nodeAffinity := AddNodeAffinity(&com)
					ser.Spec.Module.Spec.Affinity = nodeAffinity
					if ser.Spec.Module.Spec.NodeSelector == nil {
						ser.Spec.Module.Spec.NodeSelector = map[string]string{
							known.HypernodeClusterNodeRole: known.HypernodeClusterNodeRolePublic,
							v1alpha1.ParsedRuntimeStateKey: com.RuntimeType,
						}
					}
					ser.Spec.Module.Labels = ser.GetLabels()
					serlessUnstructured, err = ObjectConvertToUnstructured(ser)
				} else {
					serlessUnstructured, err = ObjectConvertToUnstructured(ser)
				}
				if err != nil {
					msg := fmt.Sprintf("failed to unmarshal resource: %v", err)
					klog.ErrorDepth(5, msg)
					return nil, err
				}
			}
			break
		}
	}
	if err != nil {
		msg := fmt.Sprintf("serverless failed to unmarshal resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}
	return serlessUnstructured, nil
}

func ConstructDescriptionFromExistOne(old *appsv1alpha1.Description) *appsv1alpha1.Description {
	newOne := &appsv1alpha1.Description{
		ObjectMeta: metav1.ObjectMeta{
			Name:       old.Name,
			Finalizers: old.Finalizers,
			Labels:     fillDescriptionLabels(old),
		},
		Spec: old.Spec,
	}
	return newOne
}

func fillDescriptionLabels(desc *appsv1alpha1.Description) map[string]string {
	newLabels := make(map[string]string)
	oldLabels := desc.GetLabels()
	if len(oldLabels) != 0 {
		for key, value := range oldLabels {
			newLabels[key] = value
		}
	}
	if desc.Namespace == known.GaiaReservedNamespace {
		newLabels[known.OriginDescriptionNameLabel] = desc.Name
		newLabels[known.OriginDescriptionNamespaceLabel] = desc.Namespace
		newLabels[known.OriginDescriptionUIDLabel] = string(desc.UID)
	}
	return newLabels
}

func CreatNSIdNeed(dynamicClient dynamic.Interface, restMapper meta.RESTMapper, namespace string) error {
	var err error
	if len(namespace) > 0 {
		ns := &corev1.Namespace{
			TypeMeta: metav1.TypeMeta{
				Kind:       "Namespace",
				APIVersion: "v1",
			},
			ObjectMeta: metav1.ObjectMeta{
				Name: namespace,
			},
		}
		var nsKind = schema.GroupVersionKind{Group: "", Version: "v1", Kind: "Namespace"}
		restNS, _ := restMapper.RESTMapping(nsKind.GroupKind(), nsKind.Version)
		nsUnstructured, errns := ObjectConvertToUnstructured(ns)
		if errns != nil {
			err := fmt.Errorf("convert ns %s error===.%v \n", namespace, err)
			return err
		}
		_, err := dynamicClient.Resource(restNS.Resource).Create(context.TODO(), nsUnstructured, metav1.CreateOptions{})
		if err != nil && !apierrors.IsAlreadyExists(err) {
			nsErr := fmt.Errorf("create  ns %s error===.%v \n", namespace, err)
			return nsErr
		}
	}
	return nil
}

// OffloadResourceBindingByDescription by ssider
func OffloadResourceByDescription(ctx context.Context, dynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, desc *appsv1alpha1.Description) error {

	var descriptionsKind = schema.GroupVersionKind{Group: "apps.gaia.io", Version: "v1alpha1", Kind: "ResourceBinding"}
	restMapping, err := discoveryRESTMapper.RESTMapping(descriptionsKind.GroupKind(), descriptionsKind.Version)
	if err != nil {
		klog.Errorf("cannot get desc name=%s its descrito %v", desc.Name, err)
		return err
	}

	klog.V(5).Infof("deleting description %s  defined in deploy %s", desc.Name, klog.KObj(desc))
	err = dynamicClient.Resource(restMapping.Resource).Namespace(desc.Namespace).DeleteCollection(ctx, metav1.DeleteOptions{}, metav1.ListOptions{LabelSelector: labels.SelectorFromSet(labels.Set{
		known.GaiaDescriptionLabel: desc.Name}).String()})
	if err != nil && !apierrors.IsNotFound(err) {
		klog.Errorf("deleting description %s  defined in deploy %s", desc.Name, klog.KObj(desc))
		return err
	}
	return err
}

func OffloadDescription(ctx context.Context, dynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, desc *appsv1alpha1.Description) error {

	descUnstructured, _ := ObjectConvertToUnstructured(desc)
	return DeleteResource(ctx, dynamicClient, discoveryRESTMapper, descUnstructured)
}

func DeleteResource(ctx context.Context, dynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, resource *unstructured.Unstructured) error {
	wg := sync.WaitGroup{}
	var err error
	wg.Add(1)
	go func(resource *unstructured.Unstructured) {
		defer wg.Done()
		klog.V(5).Infof("resource deleting %s %s defined in resource %s", resource.GetKind(),
			resource.GetName(), klog.KObj(resource))
		err = DeleteResourceWithRetry(ctx, dynamicClient, discoveryRESTMapper, resource)
		if err != nil {
			klog.Errorf("error resource deleting %s %s defined in resource %s", resource.GetKind(),
				resource.GetName(), klog.KObj(resource))
			return
		}
	}(resource)
	wg.Wait()
	return err
}

func ApplyResource(ctx context.Context, dynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, resource *unstructured.Unstructured) error {
	wg := sync.WaitGroup{}
	var err error
	wg.Add(1)
	go func(resource *unstructured.Unstructured) {
		defer wg.Done()
		klog.V(5).Infof("resource apply %s %s defined in resource %s", resource.GetKind(),
			resource.GetName(), klog.KObj(resource))
		err = ApplyResourceWithRetry(ctx, dynamicClient, discoveryRESTMapper, resource)
		if err != nil {
			klog.Errorf("error resource apply %s %s defined in resource %s", resource.GetKind(),
				resource.GetName(), klog.KObj(resource))
			return
		}
	}(resource)
	wg.Wait()
	return err
}

// check if we should binding network path.
func NeedBindNetworkInCluster(rbApps []*appsv1alpha1.ResourceBindingApps, clusterName string, networkReq *appsv1alpha1.NetworkRequirement) bool {
	var compToClusterMap map[string]int32
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName {
			compToClusterMap = rbApp.Replicas
		}
	}
	idToComponentMap := make(map[string]string, 0)
	for _, netCommu := range networkReq.Spec.NetworkCommunication {
		for _, selfID := range netCommu.SelfID {
			idToComponentMap[selfID] = netCommu.Name
		}
	}

	for _, netCommu := range networkReq.Spec.NetworkCommunication {
		if len(netCommu.InterSCNID) > 0 {
			for _, internalSCN := range netCommu.InterSCNID {
				comName := idToComponentMap[internalSCN.Source.Id]
				if compToClusterMap[comName] > 0 {
					return true
				}
			}
		}
	}
	return false
}
