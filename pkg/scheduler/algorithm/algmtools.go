package algorithm

import (
	"crypto/rand"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"reflect"
	"sort"
	"strconv"

	"gonum.org/v1/gonum/mat"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	appv1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/scheduler/framework"
	"github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
)

func scheduleWorkload(cpu int64, mem int64, clusters []*v1alpha1.ManagedCluster, diagnosis interfaces.Diagnosis,
) ([]*framework.ClusterInfo, int64) {
	result := make([]*framework.ClusterInfo, 0)
	total := int64(0)
	for _, cluster := range clusters {
		clusterInfo := &framework.ClusterInfo{
			Cluster: cluster,
		}
		clusterCapacity := clusterInfo.CalculateCapacity(cpu, mem)
		if clusterCapacity == 0 {
			diagnosis.UnschedulablePlugins.Insert("filtered cluster ", cluster.Name, " has resource limit")
			continue
		}
		clusterInfo.Total = clusterCapacity
		total += clusterCapacity
		result = append(result, clusterInfo)
	}
	return result, total
}

// spawn a brank new resourcebindings on multi spread level.
func spawnResourceBindings(ins [][]mat.Matrix, allClusters []*v1alpha1.ManagedCluster, desc *appv1alpha1.Description,
	components []appv1alpha1.Component, affinity []int,
) []*appv1alpha1.ResourceBinding {
	result := make([]*appv1alpha1.ResourceBinding, 0)
	matResult := make([]mat.Matrix, 0)
	rbIndex := 0
	rbLabels := FillRBLabels(desc)
	rbLabels[common.GaiaDescriptionLabel] = desc.Name
	for _, items := range ins {
		indexMatrix, err := makeResourceBindingMatrix(items)
		if err != nil {
			// leave this matrix alone.
			continue
		}
		for _, item := range indexMatrix {
			// check whether every item fit the affinity condition
			if !checkAffinityCondition(item, affinity) {
				continue
			}

			if !checkAGroupValid(item, components) {
				continue
			}

			rb := &appv1alpha1.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("%s-rs-%d", desc.Name, rbIndex),
					Labels: rbLabels,
				},
				Spec: appv1alpha1.ResourceBindingSpec{
					AppID:  desc.Name,
					RbApps: spawnResourceBindingApps(item, allClusters, components),
				},
			}
			rb.Kind = "ResourceBinding"
			rb.APIVersion = "apps.gaia.io/v1alpha1"
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

func FillRBLabels(desc *appv1alpha1.Description) map[string]string {
	newLabels := make(map[string]string)
	oldLabels := desc.GetLabels()
	if len(oldLabels) != 0 {
		for key, value := range oldLabels {
			newLabels[key] = value
		}
	}
	if desc.Namespace == common.GaiaReservedNamespace {
		newLabels[common.OriginDescriptionNameLabel] = desc.Name
		newLabels[common.OriginDescriptionNamespaceLabel] = desc.Namespace
		newLabels[common.OriginDescriptionUIDLabel] = string(desc.UID)
	}
	return newLabels
}

func GetResultWithoutRB(result [][]mat.Matrix, levelIndex, comIndex int) mat.Matrix {
	originMat := result[levelIndex][comIndex]
	ar, ac := originMat.Dims()
	got := mat.NewDense(ar, ac, nil)
	got.Copy(originMat)
	return got
}

func GetResultWithRB(result [][][]mat.Matrix, rbIndex, levelIndex, comIndex int) mat.Matrix {
	originMat := result[rbIndex][levelIndex][comIndex]
	ar, ac := originMat.Dims()
	got := mat.NewDense(ar, ac, nil)
	got.Copy(originMat)
	return got
}

// GetAffinityComPlanForDeployment return a new mat.Matrix according to
// the destination component when that is dispersion or serverless
func GetAffinityComPlanForDeployment(m mat.Matrix, replicas int64, dispersion bool) mat.Matrix {
	if dispersion {
		return GetAffinityComPlan(m, replicas)
	}

	rows, cols := m.Dims()
	rowNums := make([]int, rows)
	// 计算每一行非零位
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			if m.At(i, j) != 0 {
				rowNums[i]++
			}
		}
	}

	newResult := mat.NewDense(rows, cols, nil)

	for i := 0; i < rows; i++ {
		if rowNums[i] == 0 {
			newResult.SetRow(i, mat.DenseCopyOf(m).RawRowView(i))
		} else if rowNums[i] >= 1 {
			// 目的component有跨域或者是serverless
			for k := 0; k < cols; k++ {
				// 取一个不为零的位置置为replicas
				if m.At(i, k) != 0 {
					data := make([]float64, cols)
					data[k] = float64(replicas)
					newResult.SetRow(i, data)
					break
				}
			}
		}
	}

	return newResult
}

// GetAffinityComPlan return a mat.Matrix according to the new replicas
func GetAffinityComPlan(result mat.Matrix, replicas int64) mat.Matrix {
	// 按比例缩放实现
	rows, cols := result.Dims()

	// 计算每一行的总和
	rowSums := make([]float64, rows)
	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			rowSums[i] += result.At(i, j)
		}
	}

	// 创建新的结果矩阵
	newResult := mat.NewDense(rows, cols, nil)

	// 缩放每一行的值并四舍五入为整数
	for i := 0; i < rows; i++ {
		if rowSums[i] == 0 {
			newResult.SetRow(i, mat.DenseCopyOf(result).RawRowView(i))
			klog.Infof("-----newResult is %+v", newResult)
			continue
		}

		scaleFactor := float64(replicas) / rowSums[i]
		rowSum := float64(0)
		for j := 0; j < cols; j++ {
			newValue := result.At(i, j) * scaleFactor
			roundedValue := int64(newValue + 0.5) // 四舍五入为最接近的整数
			newResult.Set(i, j, float64(roundedValue))
			rowSum += float64(roundedValue)
		}

		// 如果行的总和不等于 replicas，则需要进行微调
		diff := replicas - int64(rowSum)
		for diff != 0 {
			if diff > 0 {
				// 将剩余的差值依次分配到每个元素中
				for j := 0; j < cols; j++ {
					newValue := newResult.At(i, j) + 1
					newResult.Set(i, j, newValue)
					diff--
					if diff == 0 {
						break
					}
				}
			} else {
				// 将剩余的差值依次从每个元素中减去
				for j := 0; j < cols; j++ {
					newValue := newResult.At(i, j) - 1
					newResult.Set(i, j, newValue)
					diff++
					if diff == 0 {
						break
					}
				}
			}
		}
	}
	return newResult
}

// GetAffinityComPlanForServerless return a mat.Matrix according to the destination component
func GetAffinityComPlanForServerless(m mat.Matrix) mat.Matrix {
	rows, cols := m.Dims()
	newData := make([]float64, rows*cols)

	for i := 0; i < rows; i++ {
		for j := 0; j < cols; j++ {
			val := m.At(i, j)
			if val != 0 {
				// set 1 at the place whose value is not equal 0
				newData[i*cols+j] = 1
			}
		}
	}
	return mat.NewDense(rows, cols, newData)
}

// make a matrix to a rbApps struct.
func spawnResourceBindingApps(mat mat.Matrix, allClusters []*v1alpha1.ManagedCluster,
	components []appv1alpha1.Component,
) []*appv1alpha1.ResourceBindingApps {
	matR, matC := mat.Dims()
	chosenMat := chosenOneInArrow(mat, allClusters)
	rbapps := make([]*appv1alpha1.ResourceBindingApps, len(allClusters))
	for i, item := range allClusters {
		rbapps[i] = &appv1alpha1.ResourceBindingApps{
			ClusterName: item.Name,
			Replicas:    make(map[string]int32),
			ChosenOne:   make(map[string]int32),
		}
	}

	for i := 0; i < matR; i++ {
		for j := 0; j < matC; j++ {
			rbapps[j].Replicas[components[i].Name] = int32(mat.At(i, j))
			rbapps[j].ChosenOne[components[i].Name] = int32(chosenMat.At(i, j))
		}
	}
	return rbapps
}

// makeUserAPPPlan return userapp plan, only one replicas needed.
func makeUserAPPPlan(capability []*framework.ClusterInfo) mat.Matrix {
	result := mat.NewDense(1, len(capability), nil)
	numResult := make([]float64, len(capability))
	success := false
	for i, item := range capability {
		if item.Total > 0 {
			numResult[i] = 1
			success = true
			break
		}
	}
	if success {
		result.SetRow(0, numResult)
	} else {
		result.Zero()
	}
	return result
}

// makeServelessPlan return serverless plan
func makeServelessPlan(capability []*framework.ClusterInfo, replicas int64) mat.Matrix {
	result := mat.NewDense(1, len(capability), nil)
	if replicas > 0 {
		numResult := make([]float64, len(capability))
		for i, item := range capability {
			// resource is enough so, this check is unnecessary.
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

// makeDeployPlans make plans for specific component
func makeDeployPlans(capability []*framework.ClusterInfo, componentTotal, spreadOver int64) mat.Matrix {
	if componentTotal == 0 {
		result := mat.NewDense(1, len(capability), nil)
		result.Zero()
		return result
	}
	// 1. check param,
	if int(spreadOver) >= len(capability) {
		result := mat.NewDense(1, len(capability), nil)
		result.SetRow(0, plan(capability, componentTotal))
		return result
	} else {
		result := mat.NewDense(2, len(capability), nil)
		result.Zero()
		// 2. random 2 	may be duplicate but have no way to avoid.
		i := 0
		for i < 2 {
			randomClusterInfo, err := GenerateRandomClusterInfos(capability, int(spreadOver))
			availableCountForThisPlan := int64(0)
			if err == nil {
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

// makeDeployPlans make plans for specific component
func makeUniqeDeployPlans(capability []*framework.ClusterInfo, componentTotal, spreadOver int64) mat.Matrix {
	// 稍显突兀
	maxRBNumberString := os.Getenv("MaxRBNumber")
	maxRBNumber, err := strconv.Atoi(maxRBNumberString)
	if err != nil {
		maxRBNumber = 2
	}

	if componentTotal == 0 {
		result := mat.NewDense(1, len(capability), nil)
		result.Zero()
		return result
	}
	// 1. check param,
	if int(spreadOver) >= len(capability) {
		result := mat.NewDense(1, len(capability), nil)
		result.SetRow(0, plan(capability, componentTotal))
		return result
	} else {
		temResult := make([][]float64, 0)
		// 2. get 2 plans
		allCombile := GetClusterCombos(capability, int(spreadOver))
		planNums := 0
		for _, clusterPlan := range allCombile {
			availableCountForThisPlan := int64(0)
			randomCapacity := make([]*framework.ClusterInfo, len(capability))
			for i, item := range capability {
				if ifItemInTheArray(clusterPlan, item) {
					randomCapacity[i] = item
					availableCountForThisPlan += item.Total
				} else {
					randomCapacity[i] = nil
				}
			}
			// we get a unavailable plan
			if availableCountForThisPlan < componentTotal {
				continue
			} else {
				r := plan(randomCapacity, componentTotal)
				temResult = append(temResult, r)
				planNums += 1
			}
			if planNums >= maxRBNumber {
				break
			}
		}
		result := mat.NewDense(len(temResult), len(capability), nil)
		for i, item := range temResult {
			result.SetRow(i, item)
		}
		return result
	}
}

func ifItemInTheArray(clusters []*framework.ClusterInfo, cluster *framework.ClusterInfo) bool {
	for _, item := range clusters {
		if item == cluster {
			return true
		}
	}
	return false
}

// make matrix from component matrix on same level, 1, n, full, return one schedule result.
func makeResourceBindingMatrix(in []mat.Matrix) ([]mat.Matrix, error) {
	totalCount := 1
	for i := 0; i < len(in); i++ {
		if in[i] == nil {
			return nil, errors.New("can't produce logic matrix")
		}
		aR, _ := in[i].Dims()
		totalCount *= aR
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

// plan https://www.jianshu.com/p/12b89147993c
func plan(weight []*framework.ClusterInfo, request int64) []float64 {
	sum := int64(0)
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
		if currentTotal != 0 {
			targetF = append(targetF, float64(sum)/float64(request)*(1+
				(currentAssign-math.Floor(currentAssign))/math.Floor(currentAssign)))
		} else {
			targetF = append(targetF, float64(0))
		}

		finalAssign = append(finalAssign, currentFinalAssign)
	}
	// f=np.sum(arr)/n*(1+(arg-np.floor(arg))/np.floor(arg))
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

func GenerateRandomClusterInfos(capacities []*framework.ClusterInfo, count int) ([]*framework.ClusterInfo, error) {
	nonZeroCount := 0
	for _, item := range capacities {
		if item.Total != 0 {
			nonZeroCount += 1
		}
	}
	switch {
	case nonZeroCount < count:
		return nil, errors.New("math: can't allocate desired count into clusters")
	case nonZeroCount == count:
		return capacities, nil
	default:
		nums := make(map[int]struct{}, 0)
		randomCapacity := make([]*framework.ClusterInfo, len(capacities))
		for len(nums) < count {
			n, err := rand.Int(rand.Reader, big.NewInt(int64(len(capacities))))
			if err != nil {
				klog.Info("can't rand.int, continue")
				continue
			}
			num := int(n.Int64())
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

func GetClusterCombos(set []*framework.ClusterInfo, depth int) [][]*framework.ClusterInfo {
	initPrefix := make([]*framework.ClusterInfo, 0)
	return GetCombosHelper(set, depth, 0, initPrefix, [][]*framework.ClusterInfo{})
}

func GetCombosHelper(set []*framework.ClusterInfo, depth int, start int, prefix []*framework.ClusterInfo,
	accum [][]*framework.ClusterInfo,
) [][]*framework.ClusterInfo {
	if depth == 0 {
		return append(accum, prefix)
	} else {
		for i := start; i <= len(set)-depth; i++ {
			if set[i].Total != 0 {
				accum = GetCombosHelper(set, depth-1, i+1, append(prefix, set[i]), accum)
			}
		}
		return accum
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

func randowChoseOneGreateThanZero(in []float64, clusters []*v1alpha1.ManagedCluster) []float64 {
	indexArrow := make([]int, 0)
	truePositiveIndices := []int{}
	// init zero dense
	denseOut := mat.NewDense(1, len(in), nil)
	denseOut.Zero()
	for i, item := range in {
		if item > 0 {
			indexArrow = append(indexArrow, i)
			if val, ok := clusters[i].GetLabels()[common.YuanLaoClusterLabel]; !ok || val != "no" {
				truePositiveIndices = append(truePositiveIndices, i)
			}
		}
	}
	if len(indexArrow) == 0 {
		return denseOut.RawRowView(0)
	}

	// 存在, 即没有污点，又符合标签的集群
	if len(truePositiveIndices) > 0 {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(truePositiveIndices))))
		if err != nil {
			klog.Info("random error...")
		}
		indexChosen := truePositiveIndices[n.Int64()]
		denseOut.Set(0, indexChosen, 1)
	} else {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(indexArrow))))
		if err != nil {
			klog.Info("random error...")
		}
		indexChosen := indexArrow[n.Int64()]
		denseOut.Set(0, indexChosen, 1)
	}
	return denseOut.RawRowView(0)
}

// chosen one
func chosenOneInArrow(in mat.Matrix, clusters []*v1alpha1.ManagedCluster) mat.Matrix {
	aR, aC := in.Dims()
	out := mat.NewDense(aR, aC, nil)
	for i := 0; i < aR; i++ {
		row := mat.DenseCopyOf(in).RawRowView(i)
		dense := randowChoseOneGreateThanZero(row, clusters)
		out.SetRow(i, dense)
	}
	return out
}

// checkAffinityCondition return whether one mat.Matrix fit the affinity conditions
func checkAffinityCondition(m mat.Matrix, affinity []int) bool {
	for i, j := range affinity {
		if i != j {
			if !checkMatrix(m, i, j) {
				return false
			}
		}
	}
	return true
}

// checkAGroupValid return whether one mat.Matrix valid the group conditions
func checkAGroupValid(m mat.Matrix, components []appv1alpha1.Component) bool {
	groupName2Index := make(map[string]int, 0)
	for _, component := range components {
		if component.GroupName != "" {
			groupName2Index[component.GroupName] = -1
		}
	}
	row, col := m.Dims()
	for r := 0; r < row; r++ {
		// 在某个组里
		if components[r].GroupName != "" {
			for c := 0; c < col; c++ {
				replica := m.At(r, c)
				if replica > 0 {
					// 没有赋值过
					if groupName2Index[components[r].GroupName] == -1 {
						groupName2Index[components[r].GroupName] = c
					} else if groupName2Index[components[r].GroupName] != c {
						// 过去有过赋值
						return false
					}
					break
				}
			}
		} else {
			continue
		}
	}

	return true
}

func checkMatrix(m mat.Matrix, i, j int) bool {
	_, ac := m.Dims()

	for k := 0; k < ac; k++ {
		if checkSpread(mat.DenseCopyOf(m).RawRowView(j)) {
			if m.At(i, k) != 0 && m.At(j, k) == 0 {
				return false
			}
		} else if (m.At(i, k) != 0 && m.At(j, k) == 0) || (m.At(i, k) == 0 && m.At(j, k) != 0) {
			return false
		}
	}
	return true
}

func checkSpread(a []float64) bool {
	num := 0
	for _, v := range a {
		if v != 0 {
			num++
		}
	}
	return num > 1
}
