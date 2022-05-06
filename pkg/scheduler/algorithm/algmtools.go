package algorithm

import (
	"errors"
	"fmt"
	"github.com/lmxia/gaia/pkg/common"
	"math"
	"math/rand"
	"reflect"
	"sort"
	"time"

	appv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/framework"
	"gonum.org/v1/gonum/mat"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	// DefaultMilliCPURequest defines default milli cpu request number.
	DefaultMilliCPURequest int64 = 100 // 0.1 core
	// DefaultMemoryRequest defines default memory request size.
	DefaultMemoryRequest int64 = 200 * 1024 * 1024 // 200 MB
)

func scheduleWorkload(cpu int64, mem int64, clusters []*v1alpha1.ManagedCluster) ([]*framework.ClusterInfo, int64) {
	result := make([]*framework.ClusterInfo, len(clusters))
	total := int64(0)
	for i, cluster := range clusters {
		clusterInfo := &framework.ClusterInfo{
			Cluster: cluster,
		}
		clusterCapacity := clusterInfo.CalculateCapacity(cpu, mem)
		clusterInfo.Total = clusterCapacity
		result[i] = clusterInfo
	}
	return result, total
}

// spawn a brank new resourcebindings on multi spread level.
func spawnResourceBindings(ins [][]mat.Matrix, allClusters []*v1alpha1.ManagedCluster, desc *appv1alpha1.Description) []*appv1alpha1.ResourceBinding {
	result := make([]*appv1alpha1.ResourceBinding, 0)
	matResult := make([]mat.Matrix, 0)
	rbIndex := 0
	for _, items := range ins {
		indexMatrix, err := makeResourceBindingMatrix(items)
		if err != nil {
			// leave this matrix alone.
			continue
		}
		for _, item := range indexMatrix {
			rb := &appv1alpha1.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name: fmt.Sprintf("%s-rs-%d", desc.Name, rbIndex),
					Labels: map[string]string{
						common.GaiaDescriptionLabel: desc.Name,
					},
				},
				Spec: appv1alpha1.ResourceBindingSpec{
					AppID:  desc.Name,
					RbApps: spawnResourceBindingApps(item, allClusters, desc),
				},
			}
			rb.Kind = "ResourceBinding"
			rb.APIVersion = "platform.gaia.io/v1alpha1"
			if checkContainMatrix(matResult, item) {
				continue
			}
			rbIndex += 1
			matResult = append(matResult, item)
			result = append(result, rb)
			// TODO only return top 10 rbs. not so random
			// TODO change me!!!
			if rbIndex == 5 {
				return result
			}
		}
	}
	return result
}

// make a matrix to a rbApps struct.
func spawnResourceBindingApps(mat mat.Matrix, allClusters []*v1alpha1.ManagedCluster, desc *appv1alpha1.Description) []*appv1alpha1.ResourceBindingApps {
	matR, matC := mat.Dims()
	rbapps := make([]*appv1alpha1.ResourceBindingApps, len(allClusters))
	for i, item := range allClusters {
		rbapps[i] = &appv1alpha1.ResourceBindingApps{
			ClusterName: item.Name,
			Replicas:    make(map[string]int32),
		}
	}

	for i := 0; i < matR; i++ {
		for j := 0; j < matC; j++ {
			rbapps[j].Replicas[desc.Spec.Components[i].Name] = int32(mat.At(i, j))
		}
	}
	return rbapps
}

// makeServelessPlan return serverless plan
func makeServelessPlan(capability []*framework.ClusterInfo, replicas int64) mat.Matrix {
	result := mat.NewDense(1, len(capability), nil)
	if replicas > 0  {
		numResult := make([]float64, len(capability))
		for i, item := range capability {
			if item.Total > 0 {
				numResult[i] = 1
			} else {
				numResult[i] = 0
			}
		}
		result.SetRow(0, numResult)
	} else {
		result.Zero()
	}
	return result
}


// makeComponentPlans make plans for specific component
func makeComponentPlans(capability []*framework.ClusterInfo, componentTotal, spreadOver int64) mat.Matrix {
	if componentTotal == 0 {
		result := mat.NewDense(1, len(capability), nil)
		result.Zero()
		return result
	}
	// 1. check param
	if int(spreadOver) > len(capability) {
		return nil
	} else if int(spreadOver) == len(capability) {
		result := mat.NewDense(1, len(capability), nil)
		result.SetRow(0, plan(capability, componentTotal))
		return result
	} else {
		result := mat.NewDense(2, len(capability), nil)
		result.Zero()
		// 2. random 2
		i := 0
		for i < 2 {
			randomClusterInfo, err := GenerateRandomClusterInfos(capability, int(spreadOver))
			availableCountForThisPlan := int64(0)
			if randomClusterInfo != nil {
				for _, item := range randomClusterInfo {
					if item != nil {
						availableCountForThisPlan += item.Total
					}
				}
			}
			// we get a unavailable plan
			if err != nil || availableCountForThisPlan < componentTotal {
				continue
			} else {
				r := plan(randomClusterInfo, componentTotal)
				result.SetRow(i, r)
				i += 1
			}
		}
		return result
	}
}

// make matrix from component matrix on same level, 1, n, full, return one schedule result.
func makeResourceBindingMatrix(in []mat.Matrix) ([]mat.Matrix, error) {
	totalCount := 1
	for i := 0; i < len(in); i++ {
		if in[i] == nil {
			return nil, errors.New("can't produce logic matrix")
		}
		aR, _ := in[i].Dims()
		totalCount = totalCount * aR
	}

	// only one component.
	result := make([]mat.Matrix, totalCount)
	aR, aC := in[0].Dims()
	for i := 0; i < aR; i++ {
		row := mat.DenseCopyOf(in[0]).RawRowView(i)
		dens := mat.NewDense(1, aC, nil)
		dens.SetRow(0, row)
		result[i] = dens
	}
	if len(in) == 1 {
		return result, nil
	}

	for i := 1; i < len(in); i++ {
		bR, _ := in[i].Dims()
		stride := 0
		tempResult := make([]mat.Matrix, len(result))
		reflect.Copy(reflect.ValueOf(tempResult), reflect.ValueOf(result))
		for j := 0; j < bR; j++ {
			row := mat.DenseCopyOf(in[i]).RawRowView(j)
			for index, item := range tempResult {
				// not initialed so we stop
				if item == nil {
					stride = index
					break
				}
				tempMatBeforeGrow := mat.DenseCopyOf(item).Grow(1, 0)
				tempRowCount, _ := tempMatBeforeGrow.Dims()
				tempMatGrow := mat.DenseCopyOf(tempMatBeforeGrow)
				tempMatGrow.SetRow(tempRowCount-1, row)
				result[j*stride+index] = tempMatGrow
			}
		}
	}

	return result, nil
}

// constructResourceBinding
 func getComponentClusterTotal(rbApps []*appv1alpha1.ResourceBindingApps, clusterName, componentName string) int64 {
	for _, rbApp := range rbApps {
		if rbApp.ClusterName == clusterName {
			return int64(rbApp.Replicas[componentName])
		}
	}

	for _, rbApp := range rbApps {
		if len(rbApp.Children) > 0 {
			return getComponentClusterTotal(rbApp.Children, clusterName, componentName)
		}
	}

	return 0
}

func calculateResource(templateSpec corev1.PodTemplateSpec) (non0CPU int64, non0Mem int64, pod *corev1.Pod) {

	pod = &corev1.Pod{
		ObjectMeta: templateSpec.ObjectMeta,
		Spec:       templateSpec.Spec,
	}

	for _, c := range templateSpec.Spec.Containers {
		non0CPUReq, non0MemReq := GetNonzeroRequests(&c.Resources.Requests)
		non0CPU += non0CPUReq
		non0Mem += non0MemReq
		// No non-zero resources for GPUs or opaque resources.
	}

	// If Overhead is being utilized, add to the total requests for the pod
	if templateSpec.Spec.Overhead != nil {
		if _, found := templateSpec.Spec.Overhead[corev1.ResourceCPU]; found {
			non0CPU += templateSpec.Spec.Overhead.Cpu().MilliValue()
		}

		if _, found := templateSpec.Spec.Overhead[corev1.ResourceMemory]; found {
			non0Mem += templateSpec.Spec.Overhead.Memory().Value()
		}
	}

	return
}

// plan https://www.jianshu.com/p/12b89147993c
func plan(weight []*framework.ClusterInfo, request int64) []float64 {
	sum := int64(0)
	percent := make([]float64, 0)
	firstAssign := make([]float64, 0)
	finalAssign := make([]float64, 0)
	targetF := make([]float64, 0)
	totalFirstAssign := float64(0)

	for _, weight := range weight {
		if weight != nil {
			sum += weight.Total
		}
	}

	for _, weight := range weight {
		currentTotal := int64(0)
		if weight != nil {
			currentTotal = weight.Total
		}
		currentPercent := float64(currentTotal) / float64(sum)
		currentAssign := currentPercent * float64(request)
		currentFinalAssign := math.Floor(currentAssign)
		totalFirstAssign += currentFinalAssign
		percent = append(percent, currentPercent)
		firstAssign = append(firstAssign, float64(request)*currentPercent)
		if currentTotal != 0 {
			targetF = append(targetF, float64(sum)/float64(request)*(1+(currentAssign-math.Floor(currentAssign))/math.Floor(currentAssign)))
		} else {
			targetF = append(targetF, float64(0))
		}

		finalAssign = append(finalAssign, currentFinalAssign)
	}
	//f=np.sum(arr)/n*(1+(arg-np.floor(arg))/np.floor(arg))
	unassginLeft := request - int64(totalFirstAssign)
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
}

func GenerateRandomClusterInfos(capacities []*framework.ClusterInfo, count int) ([]*framework.ClusterInfo, error) {
	nonZeroCount := 0
	for _, item := range capacities {
		if item.Total != 0 {
			nonZeroCount += 1
		}
	}
	if nonZeroCount < count {
		return nil, errors.New("math: can't allocate desired count into clusters")
	} else if nonZeroCount == count {
		return capacities, nil
	} else {
		nums := make(map[int]struct{}, 0)
		randomCapacity := make([]*framework.ClusterInfo, len(capacities))
		r := rand.New(rand.NewSource(time.Now().UnixNano()))
		for len(nums) < count {
			num := r.Intn(len(capacities))
			if _, ok := nums[num]; ok {
				// we have chosen this cluster.
				continue
			} else {
				if capacities[num].Total > 0 {
					nums[num] = struct{}{}
				} else {
					continue
				}
			}
		}

		for i, item := range capacities {
			if _, ok := nums[i]; ok {
				randomCapacity[i] = item
			} else {
				randomCapacity[i] = nil
			}
		}

		return randomCapacity, nil
	}
}

// if mat in matrices
func checkContainMatrix(matrices []mat.Matrix, matix mat.Matrix) bool {
	for _, item := range matrices {
		if mat.Equal(item, matix) {
			return true
		}
	}
	return false
}
