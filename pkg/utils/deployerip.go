package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"

	lmmserverless "github.com/SUMMERLm/serverless/api/v1"
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/client-go/dynamic"
	"k8s.io/klog/v2"
)

func ApplyRBWorkloadsIP(ctx context.Context, localGaiaClient, parentGaiaClient *gaiaClientSet.Clientset,
	localDynamicClient dynamic.Interface, rb *appsv1alpha1.ResourceBinding, nodes []*corev1.Node,
	desc *appsv1alpha1.Description, components []appsv1alpha1.Component, discoveryRESTMapper meta.RESTMapper,
	clusterName, promURLPrefix string,
) error {
	var allErrs []error
	var vpcResourceSli vpcResourceSlice
	group2VPC := make(map[string]string)
	descLabels := desc.GetLabels()
	wg := sync.WaitGroup{}
	comToBeApply := components

	// Binding hugeComponent with reference to vpc resources
	group2HugeCom := DescToHugeComponents(desc)
	if len(group2HugeCom) != 0 {
		getVPCForGroup(group2HugeCom, nodes, vpcResourceSli, group2VPC, promURLPrefix)
	}

	errCh := make(chan error, len(comToBeApply))
	for _, com := range comToBeApply {
		com := com
		switch com.Workload.Workloadtype {
		case appsv1alpha1.WorkloadTypeDeployment:
			var unStructures []*unstructured.Unstructured
			var errDep error
			if com.Schedule != (appsv1alpha1.SchedulerConfig{}) {
				unStructures, errDep = AssembledCronDeploymentStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false)
			} else {
				unStructures, errDep = AssembledDeploymentStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false)
			}

			for _, unStructure := range unStructures {
				if errDep != nil || unStructure == nil || unStructure.Object == nil || len(unStructure.GetName()) == 0 {
					continue
				}
				wg.Add(1)
				go func(un *unstructured.Unstructured) {
					defer wg.Done()
					retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, un)
					if retryErr != nil {
						klog.Infof("applyRBWorkloads Deployment name==%q err===%v \n",
							klog.KRef(un.GetNamespace(), un.GetName()).String(), retryErr.Error())
						errCh <- retryErr
						return
					}
				}(unStructure)
			}
		case appsv1alpha1.WorkloadTypeServerless:
			var unStructures []*unstructured.Unstructured
			var errSer error
			if com.Schedule != (appsv1alpha1.SchedulerConfig{}) {
				unStructures, errSer = AssembledCronServerlessStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false)
			} else {
				unStructures, errSer = AssembledServerlessStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false)
			}
			for _, unStructure := range unStructures {
				if errSer != nil || unStructure == nil || unStructure.Object == nil || len(unStructure.GetName()) == 0 {
					continue
				}
				wg.Add(1)
				go func(un *unstructured.Unstructured) {
					defer wg.Done()
					retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, un)
					if retryErr != nil {
						klog.Infof("applyRBWorkloads Serverless name==%q err===%v \n",
							klog.KRef(un.GetNamespace(), un.GetName()), retryErr)
						errCh <- retryErr
						return
					}
				}(unStructure)
			}
		case appsv1alpha1.WorkloadTypeAffinityDaemon:
			unStructure, errDep := AssembledDaemonSetStructure(&com, rb.Spec.RbApps, clusterName, desc.Name,
				group2VPC, descLabels, false)
			if errDep != nil || unStructure == nil || unStructure.Object == nil || len(unStructure.GetName()) == 0 {
				continue
			}
			wg.Add(1)
			go func(un *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, un)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads WorkloadTypeAffinityDaemon name==%s err===%v \n", un.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(unStructure)
		case appsv1alpha1.WorkloadTypeUserApp:
			unStructure, errUserAPP := AssembledUserAppStructure(&com, rb.Spec.RbApps, clusterName, desc.Name, descLabels, false)
			if errUserAPP != nil || unStructure == nil || unStructure.Object == nil || len(unStructure.GetName()) == 0 {
				continue
			}
			wg.Add(1)
			go func(unStructure *unstructured.Unstructured) {
				defer wg.Done()
				retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, unStructure)
				if retryErr != nil {
					klog.Infof("applyRBWorkloads userApp name==%s err===%v \n", unStructure.GetName(), retryErr)
					errCh <- retryErr
					return
				}
			}(unStructure)
		}
	}

	wg.Wait()
	close(errCh)
	for err := range errCh {
		allErrs = append(allErrs, err)
	}

	condition := getDeployCondition(allErrs, clusterName, klog.KObj(rb).String())
	// update status  with retry
	var err error
	if rb.Namespace == known.GaiaPushReservedNamespace {
		err = updateRBStatusWithRetry(ctx, localGaiaClient, rb, condition, clusterName)
	} else {
		err = updateRBStatusWithRetry(ctx, parentGaiaClient, rb, condition, clusterName)
	}
	// no more retries
	// if len(allErrs) > 0 {
	// 	return utilerrors.NewAggregate(allErrs)
	// }

	return err
}

func AssembledCronDeploymentStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool,
) ([]*unstructured.Unstructured, error) {
	var unstructureds []*unstructured.Unstructured
	var allErrs []error
	comCopy := com.DeepCopy()
	depIndex := 0
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) != 0 {
			for _, nodeRB := range rbApp.Children {
				replicas := nodeRB.Replicas[comCopy.Name]
				if replicas > 0 {
					newLabels := addLabels(comCopy, descLabels, descName)
					cron := &appsv1alpha1.CronMaster{
						TypeMeta: metav1.TypeMeta{
							Kind:       "CronMaster",
							APIVersion: "apps.gaia.io/v1alpha1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Labels: newLabels,
						},
					}
					if len(comCopy.Namespace) > 0 {
						cron.Namespace = comCopy.Namespace
					} else {
						cron.Namespace = metav1.NamespaceDefault
					}
					cron.Name = comCopy.Name + "-" + strconv.Itoa(depIndex)

					if !delete {
						cron.Spec = appsv1alpha1.CronMasterSpec{
							Schedule: comCopy.Schedule,
							Resource: appsv1alpha1.ReferenceResource{
								Namespace: comCopy.Namespace,
								Name:      comCopy.Name,
								Kind:      "Deployment",
								Version:   "v1",
								Group:     "apps",
							},
						}
						// construct ReferenceResource.RawData
						dep := &appsv1.Deployment{
							TypeMeta: metav1.TypeMeta{
								Kind:       "Deployment",
								APIVersion: "apps/v1",
							},
							ObjectMeta: metav1.ObjectMeta{
								Labels: newLabels,
							},
						}
						if len(comCopy.Namespace) > 0 {
							dep.Namespace = comCopy.Namespace
						} else {
							dep.Namespace = metav1.NamespaceDefault
						}
						dep.Name = cron.Name
						dep.Spec.Template = comCopy.Module
						nodeAffinity := AddNodeAffinity(comCopy, group2VPC)
						nodeAffinity = AddNodeAffinityIP(nodeAffinity, []string{nodeRB.ClusterName})
						dep.Spec.Template.Spec.Affinity = nodeAffinity
						dep.Spec.Template.Spec.NodeSelector = addNodeSelector(comCopy,
							dep.Spec.Template.Spec.NodeSelector)
						dep.Spec.Replicas = &replicas
						label := getTemLabel(comCopy, newLabels)
						dep.Spec.Template.Labels = label
						dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
						// add  env variables log needed
						dep.Spec.Template.Spec.Containers = addEnvVars(comCopy.Module.Spec.Containers, com.Scc)

						depJs, errDep := json.Marshal(dep)
						if errDep != nil {
							msg := fmt.Sprintf("deployment %q failed to marshal resource: %v",
								klog.KRef(dep.GetNamespace(), dep.GetName()), errDep)
							klog.ErrorDepth(5, msg)
							return nil, errDep
						}
						cron.Spec.Resource.RawData = depJs
					}
					cronUnstructured, err := ObjectConvertToUnstructured(cron)
					unstructureds = append(unstructureds, cronUnstructured)
					allErrs = append(allErrs, err)
					depIndex++
				}
			}
			break
		}
	}
	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		klog.Errorf("AssembledCronDeploymentStructureIP error: %v", err)
		return unstructureds, err
	}

	return unstructureds, nil
}

func AssembledDeploymentStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool,
) ([]*unstructured.Unstructured, error) {
	var unstructureds []*unstructured.Unstructured
	var allErrs []error
	comCopy := com.DeepCopy()
	depIndex := 0
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) != 0 {
			for _, nodeRB := range rbApp.Children {
				replicas := nodeRB.Replicas[comCopy.Name]
				if replicas > 0 {
					newLabels := addLabels(comCopy, descLabels, descName)
					dep := &appsv1.Deployment{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Deployment",
							APIVersion: "apps/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Labels: newLabels,
						},
					}
					if len(comCopy.Namespace) > 0 {
						dep.Namespace = comCopy.Namespace
					} else {
						dep.Namespace = metav1.NamespaceDefault
					}
					dep.Name = comCopy.Name
					if !delete {
						dep.Spec.Template = getPodTemplate(comCopy.Module)
						nodeAffinity := AddNodeAffinity(comCopy, group2VPC)
						nodeAffinity = AddNodeAffinityIP(nodeAffinity, []string{nodeRB.ClusterName})
						dep.Spec.Template.Spec.Affinity = nodeAffinity
						dep.Spec.Template.Spec.NodeSelector = addNodeSelector(comCopy,
							dep.Spec.Template.Spec.NodeSelector)
						dep.Spec.Replicas = &replicas
						label := getTemLabel(comCopy, newLabels)
						dep.Spec.Template.Labels = label
						dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: label}
						// add  env variables log needed
						dep.Spec.Template.Spec.Containers = addEnvVars(comCopy.Module.Spec.Containers, com.Scc)
					}
					depUnstructured, err := ObjectConvertToUnstructured(dep)
					unstructureds = append(unstructureds, depUnstructured)
					allErrs = append(allErrs, err)
					depIndex++
				}
			}
			break
		}
	}

	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		klog.Errorf("AssembledDeploymentStructureIP error: %v", err)
		return unstructureds, err
	}
	return unstructureds, nil
}

func AssembledCronServerlessStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string,
	delete bool,
) ([]*unstructured.Unstructured, error) {
	var unstructureds []*unstructured.Unstructured
	var allErrs []error
	comCopy := com.DeepCopy()
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) != 0 {
			var nodeNames []string
			for _, nodeRB := range rbApp.Children {
				replicas := nodeRB.Replicas[comCopy.Name]
				if replicas > 0 {
					nodeNames = append(nodeNames, nodeRB.ClusterName)
				}
			}
			if len(nodeNames) > 0 {
				newLabels := addLabels(comCopy, descLabels, descName)
				cron := &appsv1alpha1.CronMaster{
					TypeMeta: metav1.TypeMeta{
						Kind:       "CronMaster",
						APIVersion: "apps.gaia.io/v1alpha1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Labels: newLabels,
					},
				}
				if len(comCopy.Namespace) > 0 {
					cron.Namespace = comCopy.Namespace
				} else {
					cron.Namespace = metav1.NamespaceDefault
				}
				cron.Name = comCopy.Name

				if !delete {
					cron.Spec = appsv1alpha1.CronMasterSpec{
						Schedule: comCopy.Schedule,
						Resource: appsv1alpha1.ReferenceResource{
							Namespace: comCopy.Namespace,
							Name:      comCopy.Name,
							Kind:      "Serverless",
							Version:   "v1",
							Group:     "serverless.pml.com.cn",
						},
					}
					// construct ReferenceResource.RawData
					comCopy.Workload.TraitServerless.Foundingmember = rbApp.ChosenOne[comCopy.Name] == 1
					ser := &lmmserverless.Serverless{
						TypeMeta: metav1.TypeMeta{
							Kind:       "Serverless",
							APIVersion: "serverless.pml.com.cn/v1",
						},
						ObjectMeta: metav1.ObjectMeta{
							Labels: newLabels,
						},
					}
					if len(comCopy.Namespace) > 0 {
						ser.Namespace = comCopy.Namespace
					} else {
						ser.Namespace = metav1.NamespaceDefault
					}
					ser.Name = comCopy.Name
					ser.Spec = lmmserverless.ServerlessSpec{
						Namespace:   comCopy.Namespace,
						Name:        comCopy.Name,
						RuntimeType: comCopy.RuntimeType,
						Module:      comCopy.Module,
						Workload: lmmserverless.Workload{
							Workloadtype:    lmmserverless.WorkloadType(comCopy.Workload.Workloadtype),
							TraitServerless: comCopy.Workload.TraitServerless,
						},
					}
					nodeAffinity := AddNodeAffinity(comCopy, group2VPC)
					nodeAffinity = AddNodeAffinityIP(nodeAffinity, nodeNames)
					ser.Spec.Module.Spec.Affinity = nodeAffinity
					ser.Spec.Module.Spec.NodeSelector = addNodeSelector(comCopy,
						ser.Spec.Module.Spec.NodeSelector)
					// add  env variables log needed
					ser.Spec.Module.Spec.Containers = addEnvVars(comCopy.Module.Spec.Containers, com.Scc)
					if ser.Spec.Module.Spec.NodeSelector == nil {
						ser.Spec.Module.Spec.NodeSelector = map[string]string{
							known.HypernodeClusterNodeRole: known.HypernodeClusterNodeRolePublic,
							v1alpha1.ParsedRuntimeStateKey: comCopy.RuntimeType,
						}
					}
					ser.Spec.Module.Labels = getTemLabel(comCopy, newLabels)

					serJs, errSer := json.Marshal(ser)
					if errSer != nil {
						msg := fmt.Sprintf("serverless %q failed to marshal resource: %v",
							klog.KRef(ser.GetNamespace(), ser.GetName()), errSer)
						klog.ErrorDepth(5, msg)
						return nil, errSer
					}
					cron.Spec.Resource.RawData = serJs
				}
				cronUnstructured, err := ObjectConvertToUnstructured(cron)
				unstructureds = append(unstructureds, cronUnstructured)
				allErrs = append(allErrs, err)
			}
			break
		}
	}
	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		klog.Errorf("AssembledCronServerlessStructureIP error: %v", err)
		return unstructureds, err
	}

	return unstructureds, nil
}

func AssembledServerlessStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool,
) ([]*unstructured.Unstructured, error) {
	var unstructureds []*unstructured.Unstructured
	var allErrs []error
	comCopy := com.DeepCopy()
	for _, rbApp := range rbApps {
		if clusterName == rbApp.ClusterName && len(rbApp.Children) != 0 {
			var nodeNames []string
			for _, nodeRB := range rbApp.Children {
				replicas := nodeRB.Replicas[comCopy.Name]
				if replicas > 0 {
					nodeNames = append(nodeNames, nodeRB.ClusterName)
				}
			}
			if len(nodeNames) > 0 {
				newLabels := addLabels(comCopy, descLabels, descName)
				ser := &lmmserverless.Serverless{
					TypeMeta: metav1.TypeMeta{
						Kind:       "Serverless",
						APIVersion: "serverless.pml.com.cn/v1",
					},
					ObjectMeta: metav1.ObjectMeta{
						Labels: newLabels,
					},
				}
				if len(comCopy.Namespace) > 0 {
					ser.Namespace = comCopy.Namespace
				} else {
					ser.Namespace = metav1.NamespaceDefault
				}
				ser.Name = comCopy.Name

				if !delete {
					ser.Spec = lmmserverless.ServerlessSpec{
						Namespace:   comCopy.Namespace,
						Name:        comCopy.Name,
						RuntimeType: comCopy.RuntimeType,
						Module:      comCopy.Module,
						Workload: lmmserverless.Workload{
							Workloadtype:    lmmserverless.WorkloadType(comCopy.Workload.Workloadtype),
							TraitServerless: comCopy.Workload.TraitServerless,
						},
					}
					nodeAffinity := AddNodeAffinity(comCopy, group2VPC)
					nodeAffinity = AddNodeAffinityIP(nodeAffinity, nodeNames)
					ser.Spec.Module.Spec.Affinity = nodeAffinity
					ser.Spec.Module.Spec.NodeSelector = addNodeSelector(comCopy, ser.Spec.Module.Spec.NodeSelector)
					// add  env variables log needed
					ser.Spec.Module.Spec.Containers = addEnvVars(comCopy.Module.Spec.Containers, com.Scc)
					if ser.Spec.Module.Spec.NodeSelector == nil {
						ser.Spec.Module.Spec.NodeSelector = map[string]string{
							known.HypernodeClusterNodeRole: known.HypernodeClusterNodeRolePublic,
							v1alpha1.ParsedRuntimeStateKey: comCopy.RuntimeType,
						}
					}
					ser.Spec.Module.Labels = getTemLabel(comCopy, newLabels)
				}
				serUnstructured, err := ObjectConvertToUnstructured(ser)
				unstructureds = append(unstructureds, serUnstructured)
				allErrs = append(allErrs, err)
			}
			break
		}
	}

	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		if err != nil {
			klog.Errorf("AssembledServerlessStructureIP error: %v", err)
			return unstructureds, err
		}
	}
	return unstructureds, nil
}

func AddNodeAffinityIP(nodeAffinity *corev1.Affinity, nodeNames []string) *corev1.Affinity {
	if len(nodeNames) > 0 {
		nodeSelectorTermSNs := corev1.NodeSelectorTerm{
			MatchExpressions: []corev1.NodeSelectorRequirement{{
				Key:      v1alpha1.ParsedResNameKey, // todo: to resid ==> sn
				Operator: corev1.NodeSelectorOpIn,
				Values:   nodeNames,
			}},
		}
		if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution == nil {
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution = &corev1.NodeSelector{}
		}
		if nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms == nil {
			nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
				make([]corev1.NodeSelectorTerm, 0)
		}
		nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms =
			append(nodeAffinity.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms,
				nodeSelectorTermSNs)
		return nodeAffinity
	}
	return nodeAffinity
}
