// This file was copied from k8s.io/kubernetes/pkg/scheduler/generic_scheduler.go and modified

package algorithm

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"gonum.org/v1/gonum/mat"
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog/v2"
	utiltrace "k8s.io/utils/trace"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/networkfilter/npcore"
	schedulerapis "github.com/lmxia/gaia/pkg/scheduler/apis"
	schedulercache "github.com/lmxia/gaia/pkg/scheduler/cache"
	framework2 "github.com/lmxia/gaia/pkg/scheduler/framework"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/runtime"
	"github.com/lmxia/gaia/pkg/scheduler/metrics"
	"github.com/lmxia/gaia/pkg/scheduler/parallelize"
	"github.com/lmxia/gaia/pkg/utils"
)

// ErrNoClustersAvailable is used to describe the error that no clusters available to schedule subscriptions.
var ErrNoClustersAvailable = fmt.Errorf("no clusters available to schedule subscriptions")

type genericScheduler struct {
	cache                       schedulercache.Cache
	percentageOfClustersToScore int32
}

func (g *genericScheduler) SetSelfClusterName(name string) {
	g.cache.SetSelfClusterName(name)
}

func (g *genericScheduler) Schedule(ctx context.Context, fwk framework.Framework, rbs []*v1alpha1.ResourceBinding,
	desc *v1alpha1.Description,
) (result ScheduleResult, err error) {
	trace := utiltrace.New("Scheduling", utiltrace.Field{Key: "namespace", Value: desc.Namespace},
		utiltrace.Field{Key: "name", Value: desc.Name})
	defer trace.LogIfLong(100 * time.Millisecond)

	rbsResultFinal := make([]*v1alpha1.ResourceBinding, 0)
	maxRBNumberString := os.Getenv("MaxRBNumber")
	maxRBNumber, errC := strconv.Atoi(maxRBNumberString)
	if errC != nil {
		maxRBNumber = 2
		klog.V(5).Info("MaxRBNumber set default value: 2")
	}

	// 1. get backup clusters.
	if g.cache.NumClusters() == 0 {
		return result, ErrNoClustersAvailable
	}
	allClusters, _ := g.cache.ListClusters(&metav1.LabelSelector{})

	// format desc to components
	components, comLocation, affinity := utils.DescToComponents(desc)

	group2HugeCom := utils.DescToHugeComponents(desc)
	klog.V(5).Infof("Components are %+v", components)
	klog.V(5).Infof("comLocation is %+v,affinity is %v", comLocation, affinity) // 临时占用

	numComponent := len(components)
	if numComponent == 0 {
		return result, errors.New("the desc is empty, we can't handle this case")
	}
	// 2, means allspread, one spread, 2 spread.
	allResultGlobal := make([][]mat.Matrix, 3)
	for i := 0; i < 3; i++ {
		allResultGlobal[i] = make([]mat.Matrix, numComponent)
	}
	// has parent
	// first dim means resource binding, second means component, third means spread level.
	allResultWithRB := make([][][]mat.Matrix, len(rbs))
	if desc.Namespace != common.GaiaReservedNamespace {
		// desc has resource binding
		for i := 0; i < len(rbs); i++ {
			allResultWithRB[i] = make([][]mat.Matrix, 3)
			for j := 0; j < 3; j++ {
				allResultWithRB[i][j] = make([]mat.Matrix, numComponent)
			}
		}
	}

	for i, comm := range components {
		affinityDest := affinity[comLocation[comm.Name]]
		var componentMat mat.Matrix
		var feasibleClusters []*framework2.ClusterInfo
		var diagnosis framework.Diagnosis

		// spread level info: full level, 2 level, 1 level
		// spreadLevels := []int64{int64(len(feasibleClusters)), 2, 1}
		// todo only 1 as default spread level
		spreadLevels := []int64{1}

		if affinityDest != i {
			// this  is an affinity component
			klog.V(5).Infof("There is no need to filter for affinity component:%s", comm.Name)

			// get the affinityDest component filter Plan
			if desc.Namespace == common.GaiaReservedNamespace {
				for j := range spreadLevels {
					if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeDeployment {
						// componentMat := makeDeployPlans(allPlan, int64(comm.Workload.TraitDeployment.Replicas), int64(comm.Dispersion))
						// affinity logic
						// components[i] is affinity with component[affinityDest]
						componentMat = GetAffinityComPlanForDeployment(GetResultWithoutRB(allResultGlobal, j, affinityDest),
							int64(comm.Workload.TraitDeployment.Replicas), false)
						allResultGlobal[j][i] = componentMat
					} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeServerless {
						componentMat = GetAffinityComPlanForServerless(GetResultWithoutRB(allResultGlobal, j, affinityDest))
						allResultGlobal[j][i] = componentMat
					}
				}
			} else {
				for j, rb := range rbs {
					replicas := getComponentClusterTotal(rb.Spec.RbApps, g.cache.GetSelfClusterName(), comm.Name)
					for k := range spreadLevels {
						if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeDeployment {
							// affinity logic
							// components[i] is affinity with component[affinityDest]
							componentMat = GetAffinityComPlanForDeployment(GetResultWithRB(allResultWithRB, j, k, affinityDest),
								replicas, false)
							allResultWithRB[j][k][i] = componentMat
						} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeServerless {
							componentMat = GetAffinityComPlanForServerless(GetResultWithRB(allResultWithRB, j, k, affinityDest))
							allResultWithRB[j][k][i] = componentMat
						}
					}
				}
			}
			continue
		}

		// NO.1 pre filter
		if comm.GroupName != "" {
			hugeComm := group2HugeCom[comm.GroupName]
			commonHugeComponent := utils.HugeComponentToCommanComponent(hugeComm)
			feasibleClusters, diagnosis, err = g.findClustersThatFitComponent(ctx, fwk, commonHugeComponent)
		} else {
			feasibleClusters, diagnosis, _ = g.findClustersThatFitComponent(ctx, fwk, &components[i])
		}

		if desc.Namespace == common.GaiaReservedNamespace {
			klog.V(5).Infof("component:%v feasibleClusters is %+v", comm.Name, feasibleClusters)
			if len(feasibleClusters) == 0 {
				return result, &framework.FitError{
					Description:    desc,
					NumAllClusters: g.cache.NumClusters(),
					Diagnosis:      diagnosis,
				}
			}
		}

		allPlan := nomalizeClusters(feasibleClusters, allClusters)
		// desc come from reserved namespace, that means no resource bindings
		if desc.Namespace == common.GaiaReservedNamespace {
			for j := range spreadLevels {
				switch comm.Workload.Workloadtype {
				case v1alpha1.WorkloadTypeDeployment:
					if comm.GroupName != "" {
						componentMat := makeUniqeDeployPlans(allPlan, int64(1), 1)
						var m mat.Dense
						m.Scale(float64(comm.Workload.TraitDeployment.Replicas), componentMat)
						allResultGlobal[j][i] = &m
					} else {
						//  不再随机了，遍历所有可行解，对范围做均衡。
						allResultGlobal[j][i] = makeUniqeDeployPlans(allPlan,
							int64(comm.Workload.TraitDeployment.Replicas), spreadLevels[j])
					}
				case v1alpha1.WorkloadTypeServerless:
					allResultGlobal[j][i] = makeServelessPlan(allPlan, 1)
				case v1alpha1.WorkloadTypeAffinityDaemon:
					allResultGlobal[j][i] = makeServelessPlan(allPlan, 1)
				case v1alpha1.WorkloadTypeUserApp:
					allResultGlobal[j][i] = makeUserAPPPlan(allPlan)
				}
			}
		} else {
			for j, rb := range rbs {
				replicas := getComponentClusterTotal(rb.Spec.RbApps, g.cache.GetSelfClusterName(), comm.Name)
				if replicas != 0 && len(feasibleClusters) == 0 {
					klog.V(5).Infof("component:%v feasibleClusters is %+v", comm.Name, feasibleClusters)
					return result, &framework.FitError{
						Description:    desc,
						NumAllClusters: g.cache.NumClusters(),
						Diagnosis:      diagnosis,
					}
				}
				for k := range spreadLevels {
					switch comm.Workload.Workloadtype {
					case v1alpha1.WorkloadTypeDeployment:
						if comm.GroupName != "" && replicas != 0 {
							var m mat.Dense
							m.Scale(float64(comm.Workload.TraitDeployment.Replicas),
								makeUniqeDeployPlans(allPlan, int64(1), 1))
							allResultWithRB[j][k][i] = &m
						} else {
							//  不再随机了，遍历所有可行解，对范围做均衡。
							allResultWithRB[j][k][i] = makeUniqeDeployPlans(allPlan, replicas, spreadLevels[k])
						}
					case v1alpha1.WorkloadTypeServerless:
						allResultWithRB[j][k][i] = makeServelessPlan(allPlan, replicas)
					case v1alpha1.WorkloadTypeAffinityDaemon:
						allResultWithRB[j][k][i] = makeServelessPlan(allPlan, replicas)
					case v1alpha1.WorkloadTypeUserApp:
						allResultWithRB[j][k][i] = makeUserAPPPlan(allPlan)
					}
				}
			}
		}
	}

	// NO.2 first we should spawn rbs.
	if desc.Namespace == common.GaiaReservedNamespace {
		// all 5
		rbsResultFinal = spawnResourceBindings(allResultGlobal, allClusters, desc, components, affinity)
		// 1. add networkFilter only if we can get nwr
		if nwr, err2 := g.cache.GetNetworkRequirement(desc); err2 == nil {
			networkInfoMap := g.getTopologyInfoMap()
			klog.Infof("Log: networkInfoMap is %v", networkInfoMap)
			klog.Infof("resource binding before net filter %v", rbsResultFinal)
			rbsResultFinal = npcore.NetworkFilter(rbsResultFinal, nwr, networkInfoMap)
			klog.Infof("resource binding after net filter %v", rbsResultFinal)
			if len(rbsResultFinal) == 0 {
				return result, errors.New("network filter can't find path for current rbs")
			}
		}
		if len(rbsResultFinal) > maxRBNumber {
			// score plugins.
			priorityList, scoreError := prioritizeResourcebindings(ctx, fwk, desc, allClusters, rbsResultFinal)
			if scoreError != nil {
				klog.Warningf("score plugin run error %v", scoreError)
			}
			// select maxRBNumber
			rbsResultFinal, err = g.selectResourceBindings(priorityList, rbsResultFinal, maxRBNumber)
		}
	} else {
		rbIndex := 0
		for i, rbOld := range rbs {
			rbsResult := make([]*v1alpha1.ResourceBinding, 0)
			rbForrb := spawnResourceBindings(allResultWithRB[i], allClusters, desc, components, affinity)
			totalPeer := getTotalPeer(len(rbForrb), maxRBNumber)
			for j := range rbForrb {
				subRBApps := make([]*v1alpha1.ResourceBindingApps, 0)
				for _, rbapp := range rbOld.Spec.RbApps {
					rbItemApp := rbapp.DeepCopy()
					subRBApps = append(subRBApps, rbItemApp)
					if rbItemApp.ClusterName == g.cache.GetSelfClusterName() {
						rbItemApp.Children = rbForrb[j].Spec.RbApps
						// 第二次修改掉chosen one 的矩阵
						for _, rbApp := range rbItemApp.Children {
							for key, v := range rbItemApp.ChosenOne {
								// 当前field 没有在该component上选中
								if v == 0 {
									rbApp.ChosenOne[key] = 0
								}
							}
						}
					}
				}

				rbLabels := rbOld.GetLabels()
				rbLabels[common.ParentRBNameLabel] = rbOld.Name
				rbNew := &v1alpha1.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   fmt.Sprintf("%s-%d", rbOld.Name, rbIndex),
						Labels: rbLabels,
					},
					Spec: v1alpha1.ResourceBindingSpec{
						AppID:       desc.Name,
						ParentRB:    rbOld.Name,
						FrontendRbs: rbOld.Spec.FrontendRbs,
						RbApps:      subRBApps,
						TotalPeer:   totalPeer,
						NetworkPath: rbOld.Spec.NetworkPath,
					},
				}
				rbNew.Spec.NonZeroClusterNum = utils.CountNonZeroClusterNumForRB(rbNew)
				rbNew.Kind = common.RBKind
				rbNew.APIVersion = common.GaiaAPIVersion
				rbIndex += 1
				rbsResult = append(rbsResult, rbNew)
			}
			if len(rbsResult) > maxRBNumber {
				// score plugins.
				priorityList, scoreError := prioritizeResourcebindings(ctx, fwk, desc, allClusters, rbsResult)
				if scoreError != nil {
					klog.Warningf("score pulgin run error %v", scoreError)
				}
				// select prioritize
				rbsResult, err = g.selectResourceBindings(priorityList, rbsResult, maxRBNumber)
			}
			rbsResultFinal = append(rbsResultFinal, rbsResult...)
		}
	}

	return ScheduleResult{
		ResourceBindings: rbsResultFinal,
	}, err
}

func (g *genericScheduler) getTopologyInfoMap() map[string]clusterapi.Topo {
	networkInfoMap := make(map[string]clusterapi.Topo, 0)
	clusters, _ := g.cache.ListClusters(&metav1.LabelSelector{})
	for _, cluster := range clusters {
		networkInfoMap[cluster.GetName()] = cluster.Status.TopologyInfo
	}
	return networkInfoMap
}

func prioritizeResourcebindings(ctx context.Context, fwk framework.Framework, desc *v1alpha1.Description,
	clusters []*clusterapi.ManagedCluster, rbs []*v1alpha1.ResourceBinding,
) (framework.ResourceBindingScoreList, error) {
	if !fwk.HasScorePlugins() {
		result := make(framework.ResourceBindingScoreList, 0, len(rbs))
		for i := range rbs {
			result = append(result, framework.ResourceBindingScore{
				Index: i,
				Score: 1,
			})
		}
		return result, nil
	}

	scoresMap, scoreStatus := fwk.RunScorePlugins(ctx, desc, rbs, clusters)
	if !scoreStatus.IsSuccess() {
		return nil, scoreStatus.AsError()
	}

	// Summarize all scores.
	result := make(framework.ResourceBindingScoreList, 0, len(rbs))

	for i := range rbs {
		result = append(result, framework.ResourceBindingScore{Index: i, Score: 0})
		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}
	return result, nil
}

func prioritizeResourcebindingsVN(ctx context.Context, fwk framework.Framework, desc *v1alpha1.Description,
	nodes []*coreV1.Node, rbs []*v1alpha1.ResourceBinding,
) (framework.ResourceBindingScoreList, error) {
	if !fwk.HasScorePlugins() {
		result := make(framework.ResourceBindingScoreList, 0, len(rbs))
		for i := range rbs {
			result = append(result, framework.ResourceBindingScore{
				Index: i,
				Score: 1,
			})
		}
		return result, nil
	}

	scoresMap, scoreStatus := fwk.RunScorePluginsVN(ctx, desc, rbs, nodes)
	if !scoreStatus.IsSuccess() {
		return nil, scoreStatus.AsError()
	}

	// Summarize all scores.
	result := make(framework.ResourceBindingScoreList, 0, len(rbs))

	for i := range rbs {
		result = append(result, framework.ResourceBindingScore{Index: i, Score: 0})
		for j := range scoresMap {
			result[i].Score += scoresMap[j][i].Score
		}
	}
	return result, nil
}

func nomalizeClusters(feasibleClusters []*framework2.ClusterInfo, allClusters []*clusterapi.ManagedCluster,
) []*framework2.ClusterInfo {
	indexCluster := make(map[string]*framework2.ClusterInfo)
	result := make([]*framework2.ClusterInfo, len(allClusters))
	for _, feasibleCluster := range feasibleClusters {
		indexCluster[feasibleCluster.Cluster.Name] = feasibleCluster
	}

	for i, cluster := range allClusters {
		if k, ok := indexCluster[cluster.Name]; ok {
			result[i] = k
		} else {
			clusterInfo := &framework2.ClusterInfo{
				Cluster: cluster,
				Total:   0,
			}
			result[i] = clusterInfo
		}
	}
	return result
}

// selectResourceBindings takes a prioritized list of rbs and then picks a fraction of clusters
// in a reservoir sampling manner from the clusters that had the highest score.
func (g *genericScheduler) selectResourceBindings(rbScoreList framework.ResourceBindingScoreList,
	result []*v1alpha1.ResourceBinding, maxRBNumber int,
) ([]*v1alpha1.ResourceBinding, error) {
	if len(rbScoreList) == 0 {
		return nil, fmt.Errorf("empty rbScoreList")
	}
	sort.Sort(rbScoreList)

	// Top best maxRBNumber rbs.
	selected := make([]*v1alpha1.ResourceBinding, 0)

	for i := range rbScoreList {
		if i < maxRBNumber {
			selected = append(selected, result[rbScoreList[i].Index])
		}
	}

	return selected, nil
}

// Filters the clusters to find the ones that fit the subscription based on the framework filter plugins.
func (g *genericScheduler) findClustersThatFitComponent(ctx context.Context, fwk framework.Framework,
	comm *v1alpha1.Component,
) ([]*framework2.ClusterInfo, framework.Diagnosis, error) {
	diagnosis := framework.Diagnosis{
		ClusterToStatusMap:   make(framework.ClusterToStatusMap),
		UnschedulablePlugins: sets.NewString(),
	}

	var allClusters []*clusterapi.ManagedCluster
	mergedSelector := &metav1.LabelSelector{}
	clusters, err := g.cache.ListClusters(mergedSelector)
	if err != nil {
		return nil, diagnosis, err
	}
	allClusters = append(allClusters, clusters...)

	allClusters = normalizedClusters(allClusters)
	// Return immediately if no clusters match the cluster affinity.
	if len(allClusters) == 0 {
		return nil, diagnosis, nil
	}

	// Run "prefilter" plugins. ALL PASS FOR NOW. we don't know how to use it.
	s := fwk.RunPreFilterPlugins(ctx, comm)
	if !s.IsSuccess() {
		if !s.IsUnschedulable() {
			return nil, diagnosis, s.AsError()
		}
		// All clusters will have the same status. Some non trivial refactoring is
		// needed to avoid this copy.
		for _, n := range allClusters {
			diagnosis.ClusterToStatusMap[klog.KObj(n).String()] = s
		}
		// Status satisfying IsUnschedulable() gets injected into diagnosis.UnschedulablePlugins.
		diagnosis.UnschedulablePlugins.Insert(s.FailedPlugin())
		return nil, diagnosis, nil
	}

	feasibleClusters, err := g.findClustersThatPassFilters(ctx, fwk, comm, diagnosis, allClusters)
	if err != nil || len(feasibleClusters) == 0 {
		return nil, diagnosis, err
	}

	// aggregate all container resource
	non0CPU, non0MEM, _ := utils.CalculateResource(comm.Module)
	gpuReqMap := utils.CalculateGPU(comm.Module)
	result, _ := scheduleWorkload(non0CPU, non0MEM, gpuReqMap, feasibleClusters, diagnosis, comm.Name)

	return result, diagnosis, nil
}

// findClustersThatPassFilters finds the clusters that fit the filter plugins.
func (g *genericScheduler) findClustersThatPassFilters(ctx context.Context, fwk framework.Framework,
	com *v1alpha1.Component, diagnosis framework.Diagnosis, clusters []*clusterapi.ManagedCluster,
) ([]*clusterapi.ManagedCluster, error) {
	if !fwk.HasFilterPlugins() {
		return clusters, nil
	}

	errCh := parallelize.NewErrorChannel()
	var statusesLock sync.Mutex
	var feasibleClustersLen int32
	feasibleClusters := make([]*clusterapi.ManagedCluster, len(clusters))

	ctx, cancel := context.WithCancel(ctx)
	checkCluster := func(i int) {
		cluster := clusters[i]

		status := fwk.RunFilterPlugins(ctx, com, cluster).Merge()
		if status.Code() == framework.Error {
			errCh.SendErrorWithCancel(status.AsError(), cancel)
			return
		}
		if status.IsSuccess() {
			length := atomic.AddInt32(&feasibleClustersLen, 1)
			feasibleClusters[length-1] = cluster
		} else {
			statusesLock.Lock()
			diagnosis.ClusterToStatusMap[klog.KObj(cluster).String()] = status
			diagnosis.UnschedulablePlugins.Insert(status.FailedPlugin())
			statusesLock.Unlock()
		}
	}

	beginCheckCluster := time.Now()
	statusCode := framework.Success
	defer func() {
		// We record Filter extension point latency here instead of in framework.go because framework.RunFilterPlugins
		// function is called for each cluster, whereas we want to
		// have an overall latency for all clusters per scheduling cycle.
		metrics.FrameworkExtensionPointDuration.WithLabelValues(runtime.Filter, statusCode.String(),
			fwk.ProfileName()).Observe(metrics.SinceInSeconds(beginCheckCluster))
	}()

	// Stops searching for more clusters once the configured number of feasible clusters
	// are found.
	fwk.Parallelizer().Until(ctx, len(clusters), checkCluster)

	if err := errCh.ReceiveError(); err != nil {
		statusCode = framework.Error
		return nil, err
	}
	feasibleClusters = feasibleClusters[:feasibleClustersLen]
	return feasibleClusters, nil
}

// NewGenericScheduler creates a genericScheduler object.
func NewGenericScheduler(cache schedulercache.Cache) ScheduleAlgorithm {
	return &genericScheduler{
		cache:                       cache,
		percentageOfClustersToScore: schedulerapis.DefaultPercentageOfClustersToScore,
	}
}

// normalizedClusters will remove duplicate clusters. Deleting clusters will be removed as well.
func normalizedClusters(clusters []*clusterapi.ManagedCluster) []*clusterapi.ManagedCluster {
	allKeys := make(map[string]bool)
	var uniqueClusters []*clusterapi.ManagedCluster
	for _, cluster := range clusters {
		if _, ok := allKeys[klog.KObj(cluster).String()]; !ok {
			if cluster.DeletionTimestamp != nil {
				continue
			}
			uniqueClusters = append(uniqueClusters, cluster)
			allKeys[klog.KObj(cluster).String()] = true
		}
	}
	return uniqueClusters
}

func normalizeNodes(feasibleNodes []*framework2.NodeInfo, allNodes []*coreV1.Node,
) []*framework2.NodeInfo {
	indexNode := make(map[string]*framework2.NodeInfo)
	result := make([]*framework2.NodeInfo, len(allNodes))
	for _, feasibleNode := range feasibleNodes {
		indexNode[feasibleNode.Node.Name] = feasibleNode
	}

	for i, node := range allNodes {
		if k, ok := indexNode[node.Name]; ok {
			result[i] = k
		} else {
			nodeInfo := &framework2.NodeInfo{
				Node:  node,
				Total: 0,
			}
			result[i] = nodeInfo
		}
	}
	return result
}

// normalizedClusters will remove duplicate clusters. Deleting clusters will be removed as well.
func normalizedNodes(nodes []*coreV1.Node) []*coreV1.Node {
	allKeys := make(map[string]bool)
	var uniqueNodes []*coreV1.Node
	for _, node := range nodes {
		if _, ok := allKeys[klog.KObj(node).String()]; !ok {
			if node.DeletionTimestamp != nil {
				continue
			}
			uniqueNodes = append(uniqueNodes, node)
			allKeys[klog.KObj(node).String()] = true
		}
	}
	return uniqueNodes
}

func getTotalPeer(rbsResultNum, threshold int) int {
	if rbsResultNum < threshold {
		return rbsResultNum
	}
	return threshold
}

// Filters the clusters to find the ones that fit the subscription based on the framework filter plugins.
func (g *genericScheduler) findNodesThatFitComponent(ctx context.Context, fwk framework.Framework,
	comm *v1alpha1.Component,
) ([]*framework2.NodeInfo, framework.Diagnosis, error) {
	diagnosis := framework.Diagnosis{
		ClusterToStatusMap:   make(framework.ClusterToStatusMap),
		UnschedulablePlugins: sets.NewString(),
	}

	var allNodes []*coreV1.Node
	mergedSelector := &metav1.LabelSelector{}
	nodes, err := g.cache.ListNodes(mergedSelector)
	if err != nil {
		return nil, diagnosis, err
	}
	allNodes = append(allNodes, nodes...)

	allNodes = normalizedNodes(allNodes)
	// Return immediately if no nodes match the cluster affinity.
	if len(allNodes) == 0 {
		return nil, diagnosis, nil
	}

	// Run "prefilter" plugins. ALL PASS FOR NOW. we don't know how to use it.
	s := fwk.RunPreFilterPlugins(ctx, comm)
	if !s.IsSuccess() {
		if !s.IsUnschedulable() {
			return nil, diagnosis, s.AsError()
		}
		// All nodes will have the same status. Some non trivial refactoring is
		// needed to avoid this copy.
		for _, n := range allNodes {
			diagnosis.ClusterToStatusMap[klog.KObj(n).String()] = s
		}
		// Status satisfying IsUnschedulable() gets injected into diagnosis.UnschedulablePlugins.
		diagnosis.UnschedulablePlugins.Insert(s.FailedPlugin())
		return nil, diagnosis, nil
	}

	feasibleNodes, err := g.findNodesThatPassFilters(ctx, fwk, comm, diagnosis, allNodes)
	if err != nil || len(feasibleNodes) == 0 {
		return nil, diagnosis, err
	}

	// aggregate all container resource
	non0CPU, non0MEM, _ := utils.CalculateResource(comm.Module)
	result, _ := scheduleWorkloadVNode(non0CPU, non0MEM, feasibleNodes, diagnosis, comm.Name)

	return result, diagnosis, nil
}

// findClustersThatPassFilters finds the clusters that fit the filter plugins.
func (g *genericScheduler) findNodesThatPassFilters(ctx context.Context, fwk framework.Framework,
	com *v1alpha1.Component, diagnosis framework.Diagnosis, nodes []*coreV1.Node,
) ([]*coreV1.Node, error) {
	if !fwk.HasFilterPlugins() {
		return nodes, nil
	}

	errCh := parallelize.NewErrorChannel()
	var statusesLock sync.Mutex
	var feasibleNodesLen int32
	feasibleNodes := make([]*coreV1.Node, len(nodes))

	ctx, cancel := context.WithCancel(ctx)
	checkCluster := func(i int) {
		node := nodes[i]

		status := fwk.RunFilterPluginsVN(ctx, com, node).Merge()
		if status.Code() == framework.Error {
			errCh.SendErrorWithCancel(status.AsError(), cancel)
			return
		}
		if status.IsSuccess() {
			length := atomic.AddInt32(&feasibleNodesLen, 1)
			feasibleNodes[length-1] = node
		} else {
			statusesLock.Lock()
			diagnosis.ClusterToStatusMap[klog.KObj(node).String()] = status
			diagnosis.UnschedulablePlugins.Insert(status.FailedPlugin())
			statusesLock.Unlock()
		}
	}

	beginCheckCluster := time.Now()
	statusCode := framework.Success
	defer func() {
		// We record Filter extension point latency here instead of in framework.go because framework.RunFilterPlugins
		// function is called for each cluster, whereas we want to
		// have an overall latency for all nodes per scheduling cycle.
		metrics.FrameworkExtensionPointDuration.WithLabelValues(runtime.Filter, statusCode.String(),
			fwk.ProfileName()).Observe(metrics.SinceInSeconds(beginCheckCluster))
	}()

	// Stops searching for more nodes once the configured number of feasible nodes
	// are found.
	fwk.Parallelizer().Until(ctx, len(nodes), checkCluster)

	if err := errCh.ReceiveError(); err != nil {
		statusCode = framework.Error
		return nil, err
	}
	feasibleNodes = feasibleNodes[:feasibleNodesLen]
	return feasibleNodes, nil
}

func (g *genericScheduler) ScheduleVN(ctx context.Context, fwk framework.Framework, rbs []*v1alpha1.ResourceBinding,
	desc *v1alpha1.Description,
) (result ScheduleResult, err error) {
	trace := utiltrace.New("Scheduling", utiltrace.Field{Key: "namespace", Value: desc.Namespace},
		utiltrace.Field{Key: "name", Value: desc.Name})
	defer trace.LogIfLong(100 * time.Millisecond)

	if desc.Namespace == common.GaiaReservedNamespace {
		return result, fmt.Errorf("can not schedule desc==%q to vnode, namespace error", klog.KObj(desc))
	}
	rbsResultFinal := make([]*v1alpha1.ResourceBinding, 0)
	maxRBNumberString := os.Getenv("MaxRBNumber")
	maxRBNumber, errC := strconv.Atoi(maxRBNumberString)
	if errC != nil {
		maxRBNumber = 2
		klog.V(5).Info("MaxRBNumber set default value: 2")
	}

	allNodes, _ := g.cache.ListNodes(&metav1.LabelSelector{})

	// format desc to components
	components, comLocation, affinity := utils.DescToComponents(desc)

	group2HugeCom := utils.DescToHugeComponents(desc)
	klog.V(5).Infof("Components are %+v", components)
	klog.V(5).Infof("comLocation is %+v,affinity is %v", comLocation, affinity) // 临时占用

	numComponent := len(components)
	if numComponent == 0 {
		return result, errors.New("the desc is empty, we can't handle this case")
	}
	// 2, means allspread, one spread, 2 spread.
	allResultGlobal := make([][]mat.Matrix, 3)
	for i := 0; i < 3; i++ {
		allResultGlobal[i] = make([]mat.Matrix, numComponent)
	}
	// has parent
	// first dim means resource binding, second means component, third means spread level.
	allResultWithRB := make([][][]mat.Matrix, len(rbs))
	if desc.Namespace != common.GaiaReservedNamespace {
		// desc has resource binding
		for i := 0; i < len(rbs); i++ {
			allResultWithRB[i] = make([][]mat.Matrix, 3)
			for j := 0; j < 3; j++ {
				allResultWithRB[i][j] = make([]mat.Matrix, numComponent)
			}
		}
	}

	for i, comm := range components {
		affinityDest := affinity[comLocation[comm.Name]]
		var componentMat mat.Matrix
		var feasibleNodes []*framework2.NodeInfo
		var diagnosis framework.Diagnosis

		// spread level info: full level, 2 level, 1 level
		// spreadLevels := []int64{int64(len(feasibleNodes)), 2, 1}
		// todo only 1 as default spread level
		spreadLevels := []int64{1}

		if affinityDest != i {
			// this  is an affinity component
			klog.V(5).Infof("There is no need to filter for affinity component:%s", comm.Name)

			// get the affinityDest component filter Plan
			for j, rb := range rbs {
				replicas := getComponentClusterTotal(rb.Spec.RbApps, g.cache.GetSelfClusterName(), comm.Name)
				for k := range spreadLevels {
					if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeDeployment {
						// affinity logic
						// components[i] is affinity with component[affinityDest]
						componentMat = GetAffinityComPlanForDeployment(GetResultWithRB(allResultWithRB, j, k, affinityDest),
							replicas, false)
						allResultWithRB[j][k][i] = componentMat
					} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeServerless {
						componentMat = GetAffinityComPlanForServerless(GetResultWithRB(allResultWithRB, j, k, affinityDest))
						allResultWithRB[j][k][i] = componentMat
					}
				}
			}
			continue
		}

		// NO.1 pre filter
		if comm.GroupName != "" {
			hugeComm := group2HugeCom[comm.GroupName]
			commonHugeComponent := utils.HugeComponentToCommanComponent(hugeComm)
			feasibleNodes, diagnosis, err = g.findNodesThatFitComponent(ctx, fwk, commonHugeComponent)
		} else {
			feasibleNodes, diagnosis, _ = g.findNodesThatFitComponent(ctx, fwk, &components[i])
		}

		allPlan := normalizeNodes(feasibleNodes, allNodes)
		// desc come from reserved namespace, that means no resource bindings
		for j, rb := range rbs {
			replicas := getComponentClusterTotal(rb.Spec.RbApps, g.cache.GetSelfClusterName(), comm.Name)
			if replicas != 0 && len(feasibleNodes) == 0 {
				klog.Warningf("component:%v feasibleNodes is %+v", comm.Name, feasibleNodes)
				return result, &framework.FitError{
					Description:    desc,
					NumAllClusters: g.cache.NumNodes(),
					Diagnosis:      diagnosis,
				}
			}
			for k := range spreadLevels {
				switch comm.Workload.Workloadtype {
				case v1alpha1.WorkloadTypeDeployment:
					if comm.GroupName != "" && replicas != 0 {
						var m mat.Dense
						m.Scale(float64(comm.Workload.TraitDeployment.Replicas),
							makeUniqueDeployPlansVN(allPlan, int64(1), 1))
						allResultWithRB[j][k][i] = &m
					} else {
						//  不再随机了，遍历所有可行解，对范围做均衡。
						allResultWithRB[j][k][i] = makeUniqueDeployPlansVN(allPlan, replicas, spreadLevels[k])
					}
				case v1alpha1.WorkloadTypeServerless:
					allResultWithRB[j][k][i] = makeServelessPlanVN(allPlan, replicas)
				case v1alpha1.WorkloadTypeAffinityDaemon:
					allResultWithRB[j][k][i] = makeServelessPlanVN(allPlan, replicas)
				case v1alpha1.WorkloadTypeUserApp:
					allResultWithRB[j][k][i] = makeUserAPPPlanVN(allPlan)
				}
			}
		}
	}

	// NO.2 first we should spawn rbs.
	rbIndex := 0
	for i, rbOld := range rbs {
		rbsResult := make([]*v1alpha1.ResourceBinding, 0)
		rbForrb := spawnResourceBindingsVN(allResultWithRB[i], allNodes, desc, components, affinity)
		for j := range rbForrb {
			newRBApps := make([]*v1alpha1.ResourceBindingApps, 0)
			for _, fRBApp := range rbOld.Spec.RbApps {
				fItemRBApp := fRBApp.DeepCopy()
				newRBApps = append(newRBApps, fItemRBApp)
				for _, cRBApp := range fItemRBApp.Children {
					if cRBApp.ClusterName == g.cache.GetSelfClusterName() {
						cRBApp.Children = rbForrb[j].Spec.RbApps
						// 第二次修改掉chosen one 的矩阵
						for _, rbApp := range cRBApp.Children {
							for key, v := range cRBApp.ChosenOne {
								// 当前field 没有在该component上选中
								if v == 0 {
									rbApp.ChosenOne[key] = 0
								}
							}
						}
					}
				}
			}
			rbLabels := rbOld.GetLabels()
			rbLabels[common.ParentRBNameLabel] = rbOld.Name
			rbNew := &v1alpha1.ResourceBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:   fmt.Sprintf("%s-%d", rbOld.Name, rbIndex),
					Labels: rbLabels,
				},
				Spec: v1alpha1.ResourceBindingSpec{
					AppID:       desc.Name,
					ParentRB:    rbOld.Name,
					FrontendRbs: rbOld.Spec.FrontendRbs,
					RbApps:      newRBApps,
					TotalPeer:   getTotalPeer(len(rbForrb), maxRBNumber),
					NetworkPath: rbOld.Spec.NetworkPath,
				},
			}
			rbNew.Spec.NonZeroClusterNum = utils.CountNonZeroClusterNumForRB(rbNew)
			rbNew.Kind = common.RBKind
			rbNew.APIVersion = common.GaiaAPIVersion
			rbIndex += 1
			rbsResult = append(rbsResult, rbNew)
		}
		if len(rbsResult) > maxRBNumber {
			// score plugins.
			priorityList, scoreError := prioritizeResourcebindingsVN(ctx, fwk, desc, allNodes, rbsResult)
			if scoreError != nil {
				klog.Warningf("score pulgin run error %v", scoreError)
			}
			// select prioritize
			rbsResult, err = g.selectResourceBindings(priorityList, rbsResult, maxRBNumber)
		}
		rbsResultFinal = append(rbsResultFinal, rbsResult...)
	}

	return ScheduleResult{
		ResourceBindings: rbsResultFinal,
	}, err
}
