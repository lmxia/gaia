package utils

import (
	"encoding/json"
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
	"strings"
)

// DescToComponents reflect a description to Components
func DescToComponents(desc *appsv1alpha1.Description) (components []appsv1alpha1.Component, comLocation map[string]int, affinity []int) {
	comLocation = make(map[string]int)
	components = make([]appsv1alpha1.Component, len(desc.Spec.WorkloadComponents))

	// 1. workloadComponents
	for i, comn := range desc.Spec.WorkloadComponents {
		comLocation[comn.ComponentName] = i

		components[i] = appsv1alpha1.Component{
			Namespace:   desc.Namespace,
			Name:        comn.ComponentName,
			Preoccupy:   comn.Preoccupy,
			Schedule:    comn.Schedule,
			RuntimeType: string(comn.Sandbox),
			Module:      comn.Module,
			Workload:    GetWorkloadType(comn.WorkloadType),
			SchedulePolicy: appsv1alpha1.SchedulePolicy{
				Level: make(map[appsv1alpha1.SchedulePolicyLevel]*metav1.LabelSelector),
			},
		}
	}

	// 2. deploymentConditions
	// 存储亲和关系的component之间的索引，初始化时，对应位置填写自身的索引值
	affinity = make([]int, len(components))
	for i := range affinity {
		affinity[i] = i
	}

	ParseDeploymentCondition(desc.Spec.DeploymentCondition, components, comLocation, affinity)

	// 3. expectedPerformance
	// TODO

	return components, comLocation, affinity
}

// GetWorkloadType get the standard workload type according to the workloadType of description
func GetWorkloadType(workloadType string) (workload appsv1alpha1.Workload) {
	workloadArray := strings.Split(workloadType, "-")
	for i, value := range workloadArray {
		workloadArray[i] = strings.ToLower(value)
	}

	if ContainsString(workloadArray, "user") && ContainsString(workloadArray, "task") {
		klog.V(5).Info("This case wouldn't be existed. That should be constrained by the UI")
	}
	if ContainsString(workloadArray, "user") {
		// xxx-user-xxx
		workload = appsv1alpha1.Workload{
			Workloadtype: appsv1alpha1.WorkloadTypeUserApp,
			TraitUserAPP: &appsv1alpha1.TraitUserAPP{
				SN: "",
			},
		}
	} else if ContainsString(workloadArray, "task") {
		// xxx-xxx-task
		workload = appsv1alpha1.Workload{
			Workloadtype: appsv1alpha1.WorkloadTypeTask,
			TraitTask: &appsv1alpha1.TraitTask{
				Completions: 1,
			},
		}
	} else if ContainsString(workloadArray, "system") && ContainsString(workloadArray, "service") {
		if ContainsString(workloadArray, "stateful") {
			// stateful-system-service
			workload = appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeStatefulSet,
				TraitStatefulSet: &appsv1alpha1.TraitStatefulSet{
					Replicas: 1,
				},
			}
		} else {
			// Init stateless-system-service to deployment.
			// Change it to affinitydaemon or serverless later according to some others conditions.
			workload = appsv1alpha1.Workload{
				Workloadtype: appsv1alpha1.WorkloadTypeDeployment,
			}
		}
	}

	return workload
}

// ParseDeploymentCondition parse desc.Spec.DeploymentCondition
func ParseDeploymentCondition(deploymentCondition appsv1alpha1.DeploymentCondition, components []appsv1alpha1.Component, comLocation map[string]int, affinity []int) {
	// mandatory
	ParseCondition(deploymentCondition.Mandatory, appsv1alpha1.SchedulePolicyMandatory, components, comLocation, affinity)
	// bestEffort
	ParseCondition(deploymentCondition.BestEffort, appsv1alpha1.SchedulePolicyBestEffort, components, comLocation, affinity)
}

// ParseCondition parse desc.Spec.DeploymentCondition.Mandatory or desc.Spec.DeploymentCondition.BestEffort
func ParseCondition(deploymentConditions []appsv1alpha1.Condition, spl appsv1alpha1.SchedulePolicyLevel, components []appsv1alpha1.Component, comLocation map[string]int, affinity []int) {
	for _, condition := range deploymentConditions {
		klog.V(5).Info("parse DeploymentConditions")
		index := comLocation[condition.Subject.Name]

		// when affinity, change the affinity index array
		if "Affinity" == condition.Relation && "component" == condition.Object.Type && "component" == condition.Subject.Type {
			// component对应的索引位置的值用亲和对象的索引值表示
			affinity[index] = comLocation[condition.Object.Name]
		} else {
			if _, ok := components[index].SchedulePolicy.Level[spl]; !ok {
				components[index].SchedulePolicy.Level[spl] = &metav1.LabelSelector{
					MatchExpressions: nil,
				}
			}
			components[index].SchedulePolicy.Level[spl] = SchedulePolicyReflect(condition, components[index].SchedulePolicy.Level[spl])
		}
	}
}

// SchedulePolicyReflect reflect the condition to MatchExpressions according to the different schedule policy
func SchedulePolicyReflect(condition appsv1alpha1.Condition, spLevel *metav1.LabelSelector) *metav1.LabelSelector {
	var extentStr []string
	if "label" == condition.Object.Type && "component" == condition.Subject.Type {
		klog.V(5).Infof("%v: its condition is %v", condition.Subject.Name, condition.Extent)
		_ = json.Unmarshal(condition.Extent, &extentStr)
		klog.V(5).Infof("after ummarshal,extent is %v", extentStr)
		req := metav1.LabelSelectorRequirement{
			Key:      condition.Object.Name,
			Operator: metav1.LabelSelectorOperator(condition.Relation),
			Values:   extentStr,
		}
		spLevel.MatchExpressions = append(spLevel.MatchExpressions, req)
	}
	return spLevel

}
