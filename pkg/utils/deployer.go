package utils

import (
	"context"
	"fmt"
	"strings"
	"sync"

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
	serveringv1 "knative.dev/serving/pkg/apis/serving/v1"
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
			lastError = fmt.Errorf("please check whether the advertised apiserver of current child cluster is accessible. %v", err)
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

func GetDescrition(ctx context.Context, dynamicClient dynamic.Interface, restMapper meta.RESTMapper, name, desNs string) (*appsv1alpha1.Description, error) {
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

func OffloadRBWorkloads(ctx context.Context, desc *appsv1alpha1.Description, parentGaiaclient *gaiaClientSet.Clientset, localdynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, rb *appsv1alpha1.ResourceBinding, clusterName string) error {
	var allErrs []error
	var err error
	wg := sync.WaitGroup{}
	comToBeDeleted := desc.Spec.Components
	errCh := make(chan error, len(comToBeDeleted))
	for _, com := range comToBeDeleted {
		switch com.Workload.Workloadtype {
		case appsv1alpha1.WorkloadTypeDeployment:
			depunstructured, deperr := AssembledDeploymentStructure(&com, rb.Spec.RbApps, clusterName, true)
			if deperr != nil {
				msg := fmt.Sprintf("Deployment offloadRBWorkloads failed to unmarshal resource: %v", err)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, deperr)
				continue
			} else if depunstructured == nil {
				continue
			}
			wg.Add(1)
			go func(depunstructured *unstructured.Unstructured) {
				defer wg.Done()
				klog.V(5).Infof("deleting %s %s defined in ResourceBinding %s", depunstructured.GetKind(),
					klog.KObj(depunstructured), klog.KObj(rb))
				err2 := DeleteResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, depunstructured)
				if err2 != nil {
					klog.Infof("offloadRBWorkloads Deployment name==%s err===%v \n", depunstructured.GetName(), err2)
					errCh <- err2
				}
			}(depunstructured)
		case appsv1alpha1.WorkloadTypeKnative:
			knativeUn, errservice := AssembledKnativeStructure(&com, rb.Spec.RbApps, clusterName, true)
			if errservice != nil {
				msg := fmt.Sprintf("Knative applyRBWorkloads failed to unmarshal resource: %v", errservice)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, errservice)
				continue
			} else if knativeUn == nil {
				continue
			}
			wg.Add(1)
			go func(knativeUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := DeleteResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, knativeUn)
				if retryErr != nil {
					klog.Infof("offloadRBWorkloads Knative name==%s err===%v \n", knativeUn.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(knativeUn)
		case appsv1alpha1.WorkloadTypeServerless:
			SerUn, errservice := AssembledServerlessStructure(com, rb.Spec.RbApps, clusterName, true)
			if errservice != nil {
				msg := fmt.Sprintf("Serverless applyRBWorkloads failed to unmarshal resource: %v", errservice)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, errservice)
				continue
			} else if SerUn == nil {
				continue
			}
			wg.Add(1)
			go func(SerUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := DeleteResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, SerUn)
				if retryErr != nil {
					klog.Infof("offloadRBWorkloads Serverless name==%s err===%v \n", SerUn.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(SerUn)
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
	} else {
		klog.V(5).Infof("Description %s is deleted successfully", klog.KObj(desc))
		rbCopy := rb.DeepCopy()
		rbCopy.Finalizers = RemoveString(rbCopy.Finalizers, known.AppFinalizer)
		_, err = parentGaiaclient.AppsV1alpha1().ResourceBindings(rbCopy.Namespace).Update(context.TODO(), rbCopy, metav1.UpdateOptions{})
		if err != nil {
			klog.WarningDepth(4, fmt.Sprintf("failed to remove finalizer %s from Description %s: %v", known.AppFinalizer, klog.KObj(rbCopy), err))
		}
	}
	return err
}

func ApplyRBWorkloads(ctx context.Context, desc *appsv1alpha1.Description, parentGaiaclient *gaiaClientSet.Clientset, localdynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, rb *appsv1alpha1.ResourceBinding, clusterName string) error {
	var allErrs []error

	wg := sync.WaitGroup{}
	comToBeApply := desc.Spec.Components
	errCh := make(chan error, len(comToBeApply))
	for _, com := range comToBeApply {
		switch com.Workload.Workloadtype {
		case appsv1alpha1.WorkloadTypeDeployment:
			depUn, errdep := AssembledDeploymentStructure(&com, rb.Spec.RbApps, clusterName, false)
			if errdep != nil {
				msg := fmt.Sprintf("Deployment applyRBWorkloads failed to unmarshal resource: %v", errdep)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, errdep)
				continue
			} else if depUn == nil {
				continue
			}
			wg.Add(1)
			go func(depUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, depUn)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads Deployment name==%s err===%v \n", depUn.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(depUn)
		case appsv1alpha1.WorkloadTypeKnative:
			SerUn, errservice := AssembledKnativeStructure(&com, rb.Spec.RbApps, clusterName, false)
			if errservice != nil {
				msg := fmt.Sprintf("Knative applyRBWorkloads failed to unmarshal resource: %v", errservice)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, errservice)
				continue
			} else if SerUn == nil {
				continue
			}
			wg.Add(1)
			go func(SerUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, SerUn)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads Knative name==%s err===%v \n", SerUn.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(SerUn)
		case appsv1alpha1.WorkloadTypeServerless:
			SerUn, errservice := AssembledServerlessStructure(com, rb.Spec.RbApps, clusterName, false)
			if errservice != nil {
				msg := fmt.Sprintf("Serverless applyRBWorkloads failed to unmarshal resource: %v", errservice)
				klog.ErrorDepth(5, msg)
				allErrs = append(allErrs, errservice)
				continue
			} else if SerUn == nil {
				continue
			}
			wg.Add(1)
			go func(SerUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localdynamicClient, discoveryRESTMapper, SerUn)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads Serverless name==%s err===%v \n", SerUn.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(SerUn)
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
	_, err := parentGaiaclient.AppsV1alpha1().ResourceBindings(rb.Namespace).UpdateStatus(context.TODO(), rb, metav1.UpdateOptions{})

	if len(allErrs) > 0 {
		return utilerrors.NewAggregate(allErrs)
	}
	return err
}

func ApplyResourceBinding(ctx context.Context, localdynamicClient dynamic.Interface, discoveryRESTMapper meta.RESTMapper, rb *appsv1alpha1.ResourceBinding, clusterName string) error {
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

func AssembledDeploymentStructure(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName string, delete bool) (*unstructured.Unstructured, error) {
	depUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			dep := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				}}
			if len(com.Namespace) > 0 {
				dep.Namespace = com.Namespace
			} else {
				dep.Namespace = metav1.NamespaceDefault
			}
			dep.Name = com.Name
			nodeAffinity := &corev1.Affinity{
				NodeAffinity: &corev1.NodeAffinity{
					PreferredDuringSchedulingIgnoredDuringExecution: []corev1.PreferredSchedulingTerm{
						{
							Preference: corev1.NodeSelectorTerm{
								MatchExpressions: []corev1.NodeSelectorRequirement{
									{
										Key:      known.SpecificNodeLabelsKeyVirtualnode,
										Operator: corev1.NodeSelectorOpIn,
										Values:   []string{"pool", "Pool"},
									},
								},
							},
						},
					},
				},
			}
			dep.Spec.Template.Spec.Affinity = nodeAffinity
			if com.SchedulePolicy.SpecificResource != nil {
				if com.SchedulePolicy.SpecificResource.MatchLabels != nil {
					dep.Spec.Template.Spec.NodeSelector = com.SchedulePolicy.SpecificResource.MatchLabels
				}
			}
			
			if !delete {
				//dep.ClusterName = clusterName
				dep.Spec.Template = com.Module
				replicas := rbApp.Replicas[com.Name]
				if replicas <= 0 {
					return nil, fmt.Errorf("rbApps cannot get replicas, deployment name==%s, have nil Replicas \n", dep.Name)
				}
				dep.Spec.Replicas = &replicas
				selector := &metav1.LabelSelector{MatchLabels: com.Module.Labels}
				dep.Spec.Selector = selector
				depUnstructured, err = ObjectConvertToUnstructured(dep)
			} else {
				depUnstructured, err = ObjectConvertToUnstructured(dep)
			}
		}
	}

	if depUnstructured == nil || depUnstructured.Object == nil {
		fmt.Printf("deploy cluster  %s donnot need to create rb and its component.name==%s namespace=%s \n", clusterName, com.Name, com.Namespace)
		return nil, nil
	}
	if err != nil {
		msg := fmt.Sprintf("failed to unmarshal resource: %v", err)
		klog.ErrorDepth(5, msg)
		return nil, err
	}
	if len(depUnstructured.GetName()) <= 0 {
		err = fmt.Errorf("failed to get deployment description component %s replicas from rb labels:%v", com.Name, rbApps)
		klog.ErrorDepth(5, err)
		return nil, err
	}
	return depUnstructured, nil
}

func AssembledKnativeStructure(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName string, delete bool) (*unstructured.Unstructured, error) {
	serviceUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			replicas := rbApp.Replicas[com.Name]
			if replicas > 0 {
				ser := &serveringv1.Service{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Service",
						APIVersion: "serving.knative.dev/v1",
					}}

				if len(com.Namespace) > 0 {
					ser.Namespace = com.Namespace
				} else {
					ser.Namespace = metav1.NamespaceDefault
				}
				ser.Name = com.Name
				if !delete {
					ser.Spec.Traffic = com.Serverless.Traffic
					ser.Spec.Template = serveringv1.RevisionTemplateSpec{
						ObjectMeta: com.Module.ObjectMeta,
						Spec: serveringv1.RevisionSpec{
							PodSpec:              com.Module.Spec,
							ContainerConcurrency: com.Serverless.Revision.ContainerConcurrency,
							TimeoutSeconds:       com.Serverless.Revision.TimeoutSeconds,
						},
					}
					serviceUnstructured, err = ObjectConvertToUnstructured(ser)
				} else {
					serviceUnstructured, err = ObjectConvertToUnstructured(ser)
				}
				if err != nil {
					msg := fmt.Sprintf("failed to unmarshal resource: %v", err)
					klog.ErrorDepth(5, msg)
					return nil, err
				}

			}
		}
	}
	if serviceUnstructured == nil || serviceUnstructured.Object == nil {
		fmt.Printf("knative cluster  %s donnot need to create rb and its component.name==%s namespace=%s \n", clusterName, com.Name, com.Namespace)
		return nil, nil
	}
	if len(serviceUnstructured.GetName()) <= 0 {
		err = fmt.Errorf("failed to get serverless description component %s replicas from rb labels:%v", com.Name, rbApps)
		klog.ErrorDepth(5, err)
		return nil, err
	}
	return serviceUnstructured, nil
}

func AssembledServerlessStructure(com appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps, clusterName string, delete bool) (*unstructured.Unstructured, error) {
	serlessUnstructured := &unstructured.Unstructured{}
	var err error
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) == 0 {
			replicas := rbApp.Replicas[com.Name]
			if replicas > 0 {
				ser := &appsv1alpha1.Serverless{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Serverless",
						APIVersion: "apps.gaia.io/v1alpha1",
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
		}
	}
	if serlessUnstructured == nil || serlessUnstructured.Object == nil {
		fmt.Printf("serverless cluster %s donnot need to create rb and its component.name==%s namespace=%s \n", clusterName, com.Name, com.Namespace)
		return nil, nil
	}
	if len(serlessUnstructured.GetName()) <= 0 {
		err = fmt.Errorf("failed to get serverless description component %s replicas from rb labels:%v", com.Name, rbApps)
		klog.ErrorDepth(5, err)
		return nil, err
	}
	return serlessUnstructured, nil
}

func ConstructDescriptionFromExistOne(old *appsv1alpha1.Description) *appsv1alpha1.Description {
	newOne := &appsv1alpha1.Description{
		ObjectMeta: metav1.ObjectMeta{
			Name:       old.Name,
			Finalizers: old.Finalizers,
		},
		Spec: old.Spec,
	}
	return newOne
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

func OffloadWorkloadsByDescription(ctx context.Context, dynamicClient dynamic.Interface,
	discoveryRESTMapper meta.RESTMapper, desc *appsv1alpha1.Description) error {
	var allErrs []error
	wg := sync.WaitGroup{}
	errCh := make(chan error, len(desc.Spec.Components))
	for _, com := range desc.Spec.Components {
		if com.Workload.Workloadtype == appsv1alpha1.WorkloadTypeDeployment {
			dep := &appsv1.Deployment{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Deployment",
					APIVersion: "apps/v1",
				},
			}
			dep.Namespace = com.Namespace
			dep.Name = com.Name
			depUnstructured, errun := ObjectConvertToUnstructured(dep)
			if errun != nil {
				allErrs = append(allErrs, errun)
				msg := fmt.Sprintf("failed to unmarshal resource: %v", errun)
				klog.ErrorDepth(5, msg)
			} else {
				wg.Add(1)
				go func(depUnstructured *unstructured.Unstructured) {
					defer wg.Done()
					klog.V(5).Infof("deleting %s %s defined in deploy %s", depUnstructured.GetKind(),
						klog.KObj(depUnstructured), klog.KObj(desc))
					err := DeleteResourceWithRetry(ctx, dynamicClient, discoveryRESTMapper, depUnstructured)
					if err != nil {
						errCh <- err
					}
				}(depUnstructured)
			}
		} else if com.Workload.Workloadtype == appsv1alpha1.WorkloadTypeKnative {
			ser := &serveringv1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "serving.knative.dev/v1",
				}}
			ser.Namespace = com.Namespace
			ser.Name = com.Name
			SerUn, err := ObjectConvertToUnstructured(ser)
			if err != nil {
				msg := fmt.Sprintf("failed to unmarshal resource: %v", err)
				klog.ErrorDepth(5, msg)
				return err
			}
			wg.Add(1)
			go func(SerUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, dynamicClient, discoveryRESTMapper, SerUn)
				if retryErr != nil {
					errCh <- retryErr
					return
				}
			}(SerUn)
		} else if com.Workload.Workloadtype == appsv1alpha1.WorkloadTypeServerless {
			ser := &serveringv1.Service{
				TypeMeta: metav1.TypeMeta{
					Kind:       "Service",
					APIVersion: "serving.knative.dev/v1",
				}}
			ser.Namespace = com.Namespace
			ser.Name = com.Name
			SerUn, err := ObjectConvertToUnstructured(ser)
			if err != nil {
				msg := fmt.Sprintf("failed to unmarshal resource: %v", err)
				klog.ErrorDepth(5, msg)
				return err
			}
			wg.Add(1)
			go func(SerUn *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, dynamicClient, discoveryRESTMapper, SerUn)
				if retryErr != nil {
					errCh <- retryErr
					return
				}
			}(SerUn)
		}
		wg.Wait()
	}
	// collect errors
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}
	nsErr := fmt.Errorf("create descrption components deploy %s error===.%v \n", desc.Name, allErrs)
	return nsErr
}

func OffloadResourceBindingByDescription(ctx context.Context, dynamicClient dynamic.Interface,
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
