package utils

import (
	"encoding/json"
	"strings"

	lmmserverless "github.com/SUMMERLm/serverless/api/v1"
	appsv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

// DescToComponents reflect a description to Components
func DescToComponents(desc *appsv1alpha1.Description) (components []appsv1alpha1.Component, comLocation map[string]int, affinity []int) {
	comLocation = make(map[string]int)
	components = make([]appsv1alpha1.Component, len(desc.Spec.WorkloadComponents))

	// 1. workloadComponents
	for i, comn := range desc.Spec.WorkloadComponents {
		comLocation[comn.ComponentName] = i

		components[i] = appsv1alpha1.Component{
			Namespace:   comn.Namespace,
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
	ParseExpectedPerformance(desc.Spec.ExpectedPerformance, components, comLocation)

	// 4. Some transformations according to the different workload types.
	// For sn, transfer it to the specified TraitXxx.
	for _, comn := range components {
		if comn.Workload.Workloadtype == appsv1alpha1.WorkloadTypeUserApp && comn.SchedulePolicy.Level != nil {
			// use appsv1alpha1.SchedulePolicyMandatory for default
			for _, exp := range comn.SchedulePolicy.Level[appsv1alpha1.SchedulePolicyMandatory].MatchExpressions {
				if exp.Key == "sn" { // 还要加上sn.Operator的参考 TODO
					comn.Workload.TraitUserAPP.SN = exp.Values[0]
					break
				}
			}
			klog.V(5).Infof("%s's Workload.TraitUserAPP is %+v", comn.Name, comn.Workload.TraitUserAPP)
		} else if comn.Workload.Workloadtype == appsv1alpha1.WorkloadTypeAffinityDaemon && comn.SchedulePolicy.Level != nil {
			// use appsv1alpha1.SchedulePolicyMandatory for default
			for _, exp := range comn.SchedulePolicy.Level[appsv1alpha1.SchedulePolicyMandatory].MatchExpressions {
				if exp.Key == "sn" { // 还要加上sn.Operator的参考 TODO
					comn.Workload.TraitAffinityDaemon.SNS = exp.Values
					break
				}
			}
			klog.V(5).Infof("%s's Workload.TraitAffinityDaemon is %+v", comn.Name, comn.Workload.TraitAffinityDaemon)
		} else {
			// for log view
			if comn.Workload.TraitServerless != nil {
				klog.V(5).Infof("%s's 'Workload.TraitServerless is %+v", comn.Name, comn.Workload.TraitServerless)
			} else if comn.Workload.TraitDeployment != nil {
				klog.V(5).Infof("%s's Workload.TraitDeployment is %+v", comn.Name, comn.Workload.TraitDeployment)
			}
		}
	}

	return components, comLocation, affinity
}

// GetWorkloadType get the standard workload type according to the workloadType of description
// Currently, only four types, deployment, serverless, userAPP and affinityDaemon, are supported.
// So workloadType parameter can only be xxx-user-xxx and xxx-system-service, the former is userAPP and the latter
// may be serverless, affinityDaemon or deployment.
func GetWorkloadType(workloadType string) (workload appsv1alpha1.Workload) {
	workloadArray := strings.Split(workloadType, "-")
	for i, value := range workloadArray {
		workloadArray[i] = strings.ToLower(value)
	}

	if ContainsString(workloadArray, "task") {
		klog.V(5).Info("This case wouldn't be existed. That should be constrained by the UI")
	} else if ContainsString(workloadArray, "user") {
		// xxx-user-xxx
		workload = appsv1alpha1.Workload{
			Workloadtype: appsv1alpha1.WorkloadTypeUserApp,
			TraitUserAPP: &appsv1alpha1.TraitUserAPP{},
		}
	} else {
		workload = appsv1alpha1.Workload{}
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
		klog.V(5).Infof("after unmarshal,extent is %v", extentStr)
		req := metav1.LabelSelectorRequirement{
			Key:      condition.Object.Name,
			Operator: metav1.LabelSelectorOperator(condition.Relation),
			Values:   extentStr,
		}
		spLevel.MatchExpressions = append(spLevel.MatchExpressions, req)
	}
	return spLevel

}

// ParseExpectedPerformance parse desc.Spec.ExpectedPerformance
func ParseExpectedPerformance(ep appsv1alpha1.ExpectedPerformance, components []appsv1alpha1.Component, comLocation map[string]int) {

	// get boundaries map
	boundaryMap := ParseBoundaries(ep.Boundaries, components, comLocation)
	// parse HPA or VPA  and then get the threshold
	ParseMaintenance(ep.Maintenance, boundaryMap, components, comLocation)
}

// ParseBoundaries returns a map that the key is the boundary name and the value is the boundary itself
func ParseBoundaries(boundary appsv1alpha1.Boundaries, components []appsv1alpha1.Component, comLocation map[string]int) map[string]appsv1alpha1.Boundary {

	// expectedPerformance.Boundaries.Inner
	boundaryMap := make(map[string]appsv1alpha1.Boundary)
	klog.V(6).Infof("start to parse expectedPerformance.Boundaries.Inner:")
	for _, inner := range boundary.Inner {
		klog.V(5).Infof("inner is %+v", inner)
		index := comLocation[inner.Subject]
		if "replicas" == inner.Type {
			var data int32
			_ = json.Unmarshal(inner.Value, &data)
			klog.V(5).Infof("%v's replicas is %v", inner.Subject, data)
			if components[index].Workload.TraitDeployment == nil {
				// TraitDeployment init
				components[index].Workload.Workloadtype = appsv1alpha1.WorkloadTypeDeployment
				components[index].Workload.TraitDeployment = &appsv1alpha1.TraitDeployment{
					Replicas: data,
				}
				klog.V(5).Infof("%s:components[%d].Workload.TraitDeployment.Replicas is %v", inner.Subject, index, components[index].Workload.TraitDeployment.Replicas)
			}

		} else if "daemonset" == inner.Type {
			var data bool
			_ = json.Unmarshal(inner.Value, &data)
			if data {
				// TraitAffinityDaemon init
				components[index].Workload.Workloadtype = appsv1alpha1.WorkloadTypeAffinityDaemon
				components[index].Workload.TraitAffinityDaemon = &appsv1alpha1.TraitAffinityDaemon{}
			} else {
				continue
			}
		} else {
			// serverless
			var data int32
			_ = json.Unmarshal(inner.Value, &data)

			if components[index].Workload.TraitServerless == nil {
				// TraitServerless init
				components[index].Workload.Workloadtype = appsv1alpha1.WorkloadTypeServerless
				components[index].Workload.TraitServerless = &lmmserverless.TraitServerless{}
			}
			if "maxReplicas" == inner.Type {
				components[index].Workload.TraitServerless.MaxReplicas = data
				klog.V(5).Infof("%s:components[%d].Workload.TraitServerless.MaxReplicas is %v", inner.Subject, index, components[index].Workload.TraitServerless.MaxReplicas)
			} else if "maxQPS" == inner.Type {
				components[index].Workload.TraitServerless.MaxQPS = data
				klog.V(5).Infof("%s:components[%d].Workload.TraitServerless.MaxQPS is %v", inner.Subject, index, components[index].Workload.TraitServerless.MaxQPS)
			} else {
				// serverless threshold fields: cpuMax, cpuMin, memMax, memMin, qpsMax, qpsMin
				boundaryMap[inner.Name] = inner
			}
		}
	}

	// expectedPerformance.Boundaries.Inter
	klog.V(6).Info("start to parse expectedPerformance.Boundaries.Inter:")
	for range boundary.Inter {
		// TODO
	}

	// expectedPerformance.Boundaries.Extra
	klog.V(6).Info("start to parse expectedPerformance.Boundaries.Extra:")
	for range boundary.Extra {
		// TODO
	}
	return boundaryMap
}

// ParseMaintenance reflect the boundaries and maintenance's trigger to serverless' threshold.
func ParseMaintenance(maintenance appsv1alpha1.Maintenance, boundaryMap map[string]appsv1alpha1.Boundary, components []appsv1alpha1.Component, comLocation map[string]int) {

	// 1. expectedPerformance.Maintenance.HPA
	threshold := make([]map[string]int32, len(components))
	for i := range threshold {
		threshold[i] = make(map[string]int32)
	}
	klog.V(5).Info("start to parse expectedPerformance.Maintenance.HPA:")
	for _, hpa := range maintenance.HPA {
		index := comLocation[hpa.Subject]

		var value, step int32
		_ = json.Unmarshal(hpa.Strategy.Value, &step)
		if components[index].Workload.TraitServerless.MaxQPS != 0 {
			components[index].Workload.TraitServerless.QpsStep = step
		} else if components[index].Workload.TraitServerless.MaxReplicas != 0 {
			components[index].Workload.TraitServerless.ResplicasStep = step
		} else {
			components[index].Workload.TraitServerless.ResplicasStep = 1 // default: 1
		}
		if "increase" == hpa.Strategy.Type {
			// 扩 或 最大值
			boundaryList := strings.Split(hpa.Trigger, "||")
			for _, bn := range boundaryList {
				bn = strings.TrimSpace(bn)
				_ = json.Unmarshal(boundaryMap[bn].Value, &value)
				switch boundaryMap[bn].Type {
				case "cpu":
					threshold[index]["cpuMax"] = value
				case "mem":
					threshold[index]["memMax"] = value
				case "QPS":
					threshold[index]["qpsMax"] = value
				}
			}

		} else if "decrease" == hpa.Strategy.Type {
			// 缩 且 最小值
			boundaryList := strings.Split(hpa.Trigger, "&&")

			for _, bn := range boundaryList {
				bn = strings.TrimSpace(bn)
				_ = json.Unmarshal(boundaryMap[bn].Value, &value)
				switch boundaryMap[bn].Type {
				case "cpu":
					threshold[index]["cpuMin"] = value
				case "mem":
					threshold[index]["memMin"] = value
				case "QPS":
					threshold[index]["qpsMin"] = value
				}
			}
		}
	}

	// convert the threshold that type is map[string]int32 to string in order to get the serverless threshold string
	for i, td := range threshold {
		if len(td) != 0 {
			tdByte, _ := json.Marshal(td)
			klog.V(5).Infof("string(tdByte) %s:td is %s", components[i].Name, string(tdByte))
			components[i].Workload.TraitServerless.Threshold = string(tdByte)
			klog.V(5).Infof("%s:component[%d] TraitServerless is %+v", components[i].Name, i, components[i].Workload.TraitServerless)
		}
	}

	// 2. expectedPerformance.Maintenance.VPA
	klog.V(6).Info("start to parse expectedPerformance.Maintenance.VPA:")
	for _, vpa := range maintenance.VPA {
		index := comLocation[vpa.Subject]
		components[index].Workload.Workloadtype = appsv1alpha1.WorkloadTypeServerless
		// TODO
	}
}
