package scheduler

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"math"
	"sort"
)

const (
	// DefaultMilliCPURequest defines default milli cpu request number.
	DefaultMilliCPURequest int64 = 100 // 0.1 core
	// DefaultMemoryRequest defines default memory request size.
	DefaultMemoryRequest int64 = 200 * 1024 * 1024 // 200 MB
)
func (sched *Scheduler) ScheduleDeploymentOverClusters(deployment *v1.Deployment, clusters []string) (map[string]int64, int64, error) {
	non0CPU, non0MEM, pod := calculateResource(deployment)
	result, total := sched.scheduleWorkload(non0CPU, non0MEM, clusters, pod)
	scheduleResult := make(map[string]int64, 0)
	remainingReplicas := int64(*deployment.Spec.Replicas)
	scheduleTotal := int64(0)

	if remainingReplicas < total {
		for i, replicas := range plan(result, int64(*deployment.Spec.Replicas)) {
			scheduleResult[clusters[i]] = replicas
			scheduleTotal += replicas
		}
	} else {
		for i, replicas := range result {
			scheduleResult[clusters[i]] = replicas
			scheduleTotal += replicas
		}
	}
	return scheduleResult, scheduleTotal, nil
}

func (sched *Scheduler) scheduleWorkload(cpu int64, mem int64, clusterNames []string, pod *corev1.Pod) ([]int64, int64) {
	result := make([]int64, 0)
	total := int64(0)
	for _, cluster := range clusterNames {
		clusterCapacity := sched.schedulerCache.GetClusterCapacity(cluster, cpu, mem, pod)
		result = append(result, clusterCapacity)
		total += clusterCapacity
	}
	return result, total
}

func calculateResource(dep *v1.Deployment) (non0CPU int64, non0Mem int64, pod *corev1.Pod) {
	deploySpec := dep.Spec

	pod = &corev1.Pod{
		ObjectMeta: deploySpec.Template.ObjectMeta,
		Spec:       deploySpec.Template.Spec,
	}

	for _, c := range deploySpec.Template.Spec.Containers {
		non0CPUReq, non0MemReq := GetNonzeroRequests(&c.Resources.Requests)
		non0CPU += non0CPUReq
		non0Mem += non0MemReq
		// No non-zero resources for GPUs or opaque resources.
	}

	// If Overhead is being utilized, add to the total requests for the pod
	if deploySpec.Template.Spec.Overhead != nil {
		if _, found := deploySpec.Template.Spec.Overhead[corev1.ResourceCPU]; found {
			non0CPU += deploySpec.Template.Spec.Overhead.Cpu().MilliValue()
		}

		if _, found := deploySpec.Template.Spec.Overhead[corev1.ResourceMemory]; found {
			non0Mem += deploySpec.Template.Spec.Overhead.Memory().Value()
		}
	}

	return
}

// plan https://www.jianshu.com/p/12b89147993c
func plan(weight []int64, request int64) []int64 {
	sum := int64(0)
	percent := make([]float64, 0)
	firstAssign := make([]float64, 0)
	finalAssign := make([]int64, 0)
	targetF := make([]float64, 0)
	totalFirstAssign := int64(0)

	for _, weight := range weight {
		sum += weight
	}

	for _, weight := range weight {
		currentPercent := float64(weight) / float64(sum)
		currentAssign := currentPercent * float64(request)
		currentFinalAssign := int64(math.Floor(currentAssign))
		totalFirstAssign += currentFinalAssign
		percent = append(percent, currentPercent)
		firstAssign = append(firstAssign, float64(request)*currentPercent)
		targetF = append(targetF, float64(sum)/float64(request)*(1+(currentAssign-math.Floor(currentAssign))/math.Floor(currentAssign)))
		finalAssign = append(finalAssign, currentFinalAssign)
	}
	//f=np.sum(arr)/n*(1+(arg-np.floor(arg))/np.floor(arg))
	unassginLeft := request - totalFirstAssign
	ord := ArgsortNew(targetF)
	for i := 0; i < int(unassginLeft); i++ {
		finalAssign[ord[len(ord)-i-1]] += 1
	}
	return finalAssign
}


// argsort, like in Numpy, it returns an array of indexes into an array. Note
// that the gonum version of argsort reorders the original array and returns
// indexes to reconstruct the original order.
type argsort struct {
	s    []float64 // Points to orignal array but does NOT alter it.
	inds []int     // Indexes to be returned.
}

func (a argsort) Len() int {
	return len(a.s)
}

func (a argsort) Less(i, j int) bool {
	return a.s[a.inds[i]] < a.s[a.inds[j]]
}

func (a argsort) Swap(i, j int) {
	a.inds[i], a.inds[j] = a.inds[j], a.inds[i]
}

// ArgsortNew allocates and returns an array of indexes into the source float
// array.
func ArgsortNew(src []float64) []int {
	inds := make([]int, len(src))
	for i := range src {
		inds[i] = i
	}
	Argsort(src, inds)
	return inds
}

// Argsort alters a caller-allocated array of indexes into the source float
// array. The indexes must already have values 0...n-1.
func Argsort(src []float64, inds []int) {
	if len(src) != len(inds) {
		panic("floats: length of inds does not match length of slice")
	}
	a := argsort{s: src, inds: inds}
	sort.Sort(a)
}

// GetNonzeroRequests returns the default cpu and memory resource request if none is found or
// what is provided on the request.
func GetNonzeroRequests(requests *corev1.ResourceList) (int64, int64) {
	return GetNonzeroRequestForResource(corev1.ResourceCPU, requests),
		GetNonzeroRequestForResource(corev1.ResourceMemory, requests)
}

// GetNonzeroRequestForResource returns the default resource request if none is found or
// what is provided on the request.
func GetNonzeroRequestForResource(resource corev1.ResourceName, requests *corev1.ResourceList) int64 {
	switch resource {
	case corev1.ResourceCPU:
		// Override if un-set, but not if explicitly set to zero
		if _, found := (*requests)[corev1.ResourceCPU]; !found {
			return DefaultMilliCPURequest
		}
		return requests.Cpu().MilliValue()
	case corev1.ResourceMemory:
		// Override if un-set, but not if explicitly set to zero
		if _, found := (*requests)[corev1.ResourceMemory]; !found {
			return DefaultMemoryRequest
		}
		return requests.Memory().Value()
	case corev1.ResourceEphemeralStorage:
		quantity, found := (*requests)[corev1.ResourceEphemeralStorage]
		if !found {
			return 0
		}
		return quantity.Value()
	default:
		return 0
	}
	return 0
}
