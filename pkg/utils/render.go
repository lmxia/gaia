package utils

import (
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"k8s.io/klog/v2"
	"strings"
)

// DescToComponents reflect a description to Components
func DescToComponents(desc *appsv1alpha1.Description) (components []appsv1alpha1.Component, comLocation map[string]int) {
	comLocation = make(map[string]int)
	components = make([]appsv1alpha1.Component, len(desc.Spec.WorkloadComponents))

	// 1. workloadComponents
	for i, comn := range desc.Spec.WorkloadComponents {
		comLocation[comn.ComponentName] = i

		components[i] = appsv1alpha1.Component{
			Namespace:      desc.Namespace,
			Name:           comn.ComponentName,
			Preoccupy:      comn.Preoccupy,
			Schedule:       comn.Schedule,
			RuntimeType:    string(comn.Sandbox),
			Module:         comn.Module,
			Workload:       GetWorkloadType(comn.WorkloadType),
			SchedulePolicy: appsv1alpha1.SchedulePolicy{},
		}
	}

	// 2. deploymentConditions
	// TODO

	// 3. expectedPerformance
	// TODO

	return components, comLocation
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
