package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"

	lmmserverless "github.com/SUMMERLm/serverless/api/v1"
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	servicev1alpha1 "github.com/lmxia/gaia/pkg/apis/service/v1alpha1"
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

	hyperNets := getDecodeNetworkPath(rb.Spec.NetworkPath)
	hyperDataVolumes := getDecodeStoragePath(rb.Spec.StoragePath)
	errCh := make(chan error, len(comToBeApply))
	for _, com := range comToBeApply {
		com := com
		switch com.Workload.Workloadtype {
		case appsv1alpha1.WorkloadTypeDeployment:
			var unStructures []*unstructured.Unstructured
			var errDep error
			if com.Schedule != (appsv1alpha1.SchedulerConfig{}) {
				unStructures, errDep = AssembledCronDeploymentStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false, nodes, hyperNets, hyperDataVolumes)
			} else {
				unStructures, errDep = AssembledDeploymentStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false, nodes, hyperNets, hyperDataVolumes)
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
					group2VPC, descLabels, false, nodes, hyperNets, hyperDataVolumes)
			} else {
				unStructures, errSer = AssembledServerlessStructureIP(&com, rb.Spec.RbApps, clusterName, desc.Name,
					group2VPC, descLabels, false, nodes, hyperNets, hyperDataVolumes)
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
		}
	}
	if len(hyperNets) != 0 {
		hyperlabeUn, err := AssembleHyperLabelService(hyperNets, descLabels, desc.Name)
		if err != nil {
			klog.Errorf("failed to assemble hyperlabel service, err: %v", err)
			return err
		}
		wg.Add(1)
		go func(un *unstructured.Unstructured) {
			defer wg.Done()
			retryErr := ApplyResourceWithRetry(ctx, localDynamicClient, discoveryRESTMapper, un)
			if retryErr != nil {
				klog.Infof("applyRBWorkloads HyperLabel name==%q err===%v \n",
					klog.KRef(un.GetNamespace(), un.GetName()), retryErr)
				errCh <- retryErr
				return
			}
		}(hyperlabeUn)
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

func getDecodeStoragePath(storages [][]byte) []appsv1alpha1.ComDataVolume {
	var hyperDataVolumes []appsv1alpha1.ComDataVolume
	if len(storages) == 0 {
		return hyperDataVolumes
	}
	for _, storageByte := range storages {
		klog.V(5).Infof("storageByte: %v", storageByte)
		var comStorage appsv1alpha1.ComDataVolume
		err := json.Unmarshal(storageByte, &comStorage)
		if err != nil {
			klog.Errorf("failed to unmarshal storagePath to json, storagePath==%q, error==%v", storageByte, err)
			return nil
		}
		klog.Infof("decode storagePath to storageVolume %+v", comStorage)
		hyperDataVolumes = append(hyperDataVolumes, comStorage)
	}
	return hyperDataVolumes
}

func getDecodeNetworkPath(path [][]byte) []servicev1alpha1.HyperLabelItem {
	var hyperLabelNets []servicev1alpha1.HyperLabelItem
	if len(path) == 0 {
		return hyperLabelNets
	}
	for _, oneNetPath := range path {
		var LabelNet servicev1alpha1.HyperLabelItem
		err := json.Unmarshal(oneNetPath, &LabelNet)
		if err != nil {
			klog.Errorf("failed to unmarshal netowrkPath to json, networkPath==%q, error==%v", oneNetPath, err)
			return nil
		}
		klog.Infof("decode networkPath to hyperLabel: %+v", LabelNet)
		hyperLabelNets = append(hyperLabelNets, LabelNet)
	}
	return hyperLabelNets
}

func AssembleHyperLabelService(hyperNets []servicev1alpha1.HyperLabelItem, descLabels map[string]string,
	descName string,
) (*unstructured.Unstructured, error) {
	var hyperLabelItems []servicev1alpha1.HyperLabelItem
	for _, netLabel := range hyperNets {
		hyperLabelItems = append(hyperLabelItems, servicev1alpha1.HyperLabelItem{
			ExposeType:    netLabel.ExposeType,
			VNList:        netLabel.VNList,
			CeniIPList:    netLabel.CeniIPList,
			ComponentName: netLabel.ComponentName,
			Namespace:     netLabel.Namespace,
			FQDNCENI:      netLabel.FQDNCENI,
			FQDNPublic:    netLabel.FQDNPublic,
			Ports:         netLabel.Ports,
		})
	}

	hyperLabel := servicev1alpha1.HyperLabel{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HyperLabel",
			APIVersion: "service.gaia.io/v1alpha1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Labels: descLabels,
		},
		Spec: hyperLabelItems,
	}
	hyperLabel.Name = descName
	hyperLabel.Namespace = known.GaiaRBMergedReservedNamespace

	return ObjectConvertToUnstructured(&hyperLabel)
}

func AssembledCronDeploymentStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool, nodes []*corev1.Node,
	hyperNets []servicev1alpha1.HyperLabelItem, hyperDataVolumes []appsv1alpha1.ComDataVolume,
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
					newLabels["apps.gaia.io/component-deploy-name"] = cron.Name

					if !delete {
						for _, comVolume := range hyperDataVolumes {
							if comVolume.ComponentName == comCopy.Name {
								comCopy.Module = InsertVolume(comCopy.Module, comVolume)
							}
						}
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
						nodeAffinity := AddNodeAffinity(comCopy, group2VPC, nodes)
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
			if len(unstructureds) > 0 && len(hyperNets) > 0 {
				for index := range hyperNets {
					if hyperNets[index].ComponentName == comCopy.Name {
						hyperNets[index].Namespace = comCopy.Namespace
					}
				}
			}
			break
		}
	}
	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		if err != nil {
			klog.Errorf("AssembledCronDeploymentStructureIP error: %v", err)
			return unstructureds, err
		}
	}

	return unstructureds, nil
}

func AssembledDeploymentStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool, nodes []*corev1.Node,
	hyperNets []servicev1alpha1.HyperLabelItem, hyperDataVolumes []appsv1alpha1.ComDataVolume,
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
					dep.Name = comCopy.Name + "-" + strconv.Itoa(depIndex)
					newLabels["apps.gaia.io/component-deploy-name"] = dep.Name

					if !delete {
						for _, comVolume := range hyperDataVolumes {
							if comVolume.ComponentName == comCopy.Name {
								comCopy.Module = InsertVolume(comCopy.Module, comVolume)
							}
						}
						dep.Spec.Template = getPodTemplate(comCopy.Module)
						nodeAffinity := AddNodeAffinity(comCopy, group2VPC, nodes)
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
			if len(unstructureds) > 0 && len(hyperNets) > 0 {
				for index := range hyperNets {
					if hyperNets[index].ComponentName == comCopy.Name {
						hyperNets[index].Namespace = comCopy.Namespace
					}
				}
			}
			break
		}
	}

	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		if err != nil {
			klog.Errorf("AssembledDeploymentStructureIP error: %v", err)
			return unstructureds, err
		}
	}
	return unstructureds, nil
}

func AssembledCronServerlessStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool, nodes []*corev1.Node,
	hyperNets []servicev1alpha1.HyperLabelItem, hyperDataVolumes []appsv1alpha1.ComDataVolume,
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
					for _, comVolume := range hyperDataVolumes {
						if comVolume.ComponentName == comCopy.Name {
							comCopy.Module = InsertVolume(comCopy.Module, comVolume)
						}
					}
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
					nodeAffinity := AddNodeAffinity(comCopy, group2VPC, nodes)
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
			if len(unstructureds) > 0 && len(hyperNets) > 0 {
				for index := range hyperNets {
					if hyperNets[index].ComponentName == comCopy.Name {
						hyperNets[index].Namespace = comCopy.Namespace
					}
				}
			}
			break
		}
	}
	if len(allErrs) > 0 {
		err := utilerrors.NewAggregate(allErrs)
		if err != nil {
			klog.Errorf("AssembledCronServerlessStructureIP error: %v", err)
			return unstructureds, err
		}
	}

	return unstructureds, nil
}

func AssembledServerlessStructureIP(com *appsv1alpha1.Component, rbApps []*appsv1alpha1.ResourceBindingApps,
	clusterName, descName string, group2VPC, descLabels map[string]string, delete bool, nodes []*corev1.Node,
	hyperNets []servicev1alpha1.HyperLabelItem, hyperDataVolumes []appsv1alpha1.ComDataVolume,
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
					for _, comVolume := range hyperDataVolumes {
						if comVolume.ComponentName == comCopy.Name {
							comCopy.Module = InsertVolume(comCopy.Module, comVolume)
						}
					}
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
					nodeAffinity := AddNodeAffinity(comCopy, group2VPC, nodes)
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
			if len(unstructureds) > 0 && len(hyperNets) > 0 {
				for index := range hyperNets {
					if hyperNets[index].ComponentName == comCopy.Name {
						hyperNets[index].Namespace = comCopy.Namespace
					}
				}
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
				Key:      v1alpha1.ParsedSNKey,
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

func InsertVolume(module corev1.PodTemplateSpec, comVolume appsv1alpha1.ComDataVolume) corev1.PodTemplateSpec {
	var hyperVolumes []corev1.Volume
	for _, hyperVolume := range comVolume.Volumes {
		if strings.ToLower(hyperVolume.Type) == "nfs" {
			volume := corev1.Volume{
				Name: hyperVolume.Name,
				VolumeSource: corev1.VolumeSource{
					NFS: &corev1.NFSVolumeSource{
						Server:   hyperVolume.Server,
						Path:     hyperVolume.Path,
						ReadOnly: hyperVolume.ReadOnly,
					},
				},
			}
			hyperVolumes = append(hyperVolumes, volume)
		} else if strings.ToLower(hyperVolume.Type) == "object-storage" {
			for index := range module.Spec.Containers {
				if module.Spec.Containers[index].Name == hyperVolume.Name {
					module.Spec.Containers[index].Env = append(module.Spec.Containers[index].Env, []corev1.EnvVar{
						{
							Name:  "ACCESS_KEY_ID",
							Value: hyperVolume.AccessKeyID,
						},
						{
							Name:  "ACCESS_KEY_SECRET",
							Value: hyperVolume.AccessKeySecret,
						},
						{
							Name:  "BUCKET_NAME",
							Value: hyperVolume.BucketName,
						},
						{
							Name:  "ENDPOINT",
							Value: hyperVolume.EndPoint,
						},
					}...)
				}
			}
		}
	}

	module.Spec.Volumes = append(module.Spec.Volumes, hyperVolumes...)
	return module
}
