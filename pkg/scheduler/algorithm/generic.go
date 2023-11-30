// This file was copied from k8s.io/kubernetes/pkg/scheduler/generic_scheduler.go and modified

package algorithm

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"gonum.org/v1/gonum/mat"
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
	nextStartClusterIndex       int
}

func (g *genericScheduler) SetSelfClusterName(name string) {
	g.cache.SetSelfClusterName(name)
}

// Schedule
func (g *genericScheduler) Schedule(ctx context.Context, fwk framework.Framework, rbs []*v1alpha1.ResourceBinding, desc *v1alpha1.Description) (result ScheduleResult, err error) {
	trace := utiltrace.New("Scheduling", utiltrace.Field{Key: "namespace", Value: desc.Namespace}, utiltrace.Field{Key: "name", Value: desc.Name})
	defer trace.LogIfLong(100 * time.Millisecond)

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
						componentMat = GetAffinityComPlanForDeployment(GetResultWithoutRB(allResultGlobal, j, affinityDest), int64(comm.Workload.TraitDeployment.Replicas), false)
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
							componentMat = GetAffinityComPlanForDeployment(GetResultWithRB(allResultWithRB, j, k, affinityDest), replicas, false)
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
			feasibleClusters, diagnosis, _ = g.findClustersThatFitComponent(ctx, fwk, &comm)
		}

		if desc.Namespace == common.GaiaReservedNamespace {
			if len(feasibleClusters) == 0 {
				return result, &framework.FitError{
					Description:    desc,
					NumAllClusters: g.cache.NumClusters(),
					Diagnosis:      diagnosis,
				}
			}
			klog.V(5).Infof("component:%v feasibleClusters is %+v", comm.Name, feasibleClusters)
		}
		allPlan := nomalizeClusters(feasibleClusters, allClusters)
		// desc come from reserved namespace, that means no resource bindings
		if desc.Namespace == common.GaiaReservedNamespace {
			for j := range spreadLevels {
				if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeDeployment {
					if comm.GroupName != "" {
						componentMat := makeUniqeDeployPlans(allPlan, int64(1), 1)
						var m mat.Dense
						m.Scale(float64(comm.Workload.TraitDeployment.Replicas), componentMat)
						allResultGlobal[j][i] = &m
					} else {
						// componentMat := makeDeployPlans(allPlan, int64(comm.Workload.TraitDeployment.Replicas), int64(comm.Dispersion))
						componentMat := makeDeployPlans(allPlan, int64(comm.Workload.TraitDeployment.Replicas), 1)
						allResultGlobal[j][i] = componentMat
					}
				} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeServerless {
					componentMat := makeServelessPlan(allPlan, 1)
					allResultGlobal[j][i] = componentMat
				} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeAffinityDaemon {
					// same as serverless.
					componentMat := makeServelessPlan(allPlan, 1)
					allResultGlobal[j][i] = componentMat
				} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeUserApp {
					componentMat, _ := makeUserAPPPlan(allPlan)
					allResultGlobal[j][i] = componentMat
				}
			}
		} else {
			for j, rb := range rbs {
				replicas := getComponentClusterTotal(rb.Spec.RbApps, g.cache.GetSelfClusterName(), comm.Name)
				for k := range spreadLevels {
					if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeDeployment {
						if comm.GroupName != "" && replicas != 0 {
							componentMat := makeUniqeDeployPlans(allPlan, int64(1), 1)
							var m mat.Dense
							m.Scale(float64(comm.Workload.TraitDeployment.Replicas), componentMat)
							allResultWithRB[j][k][i] = &m
						} else {
							componentMat := makeDeployPlans(allPlan, replicas, spreadLevels[k])
							allResultWithRB[j][k][i] = componentMat
						}
					} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeServerless {
						componentMat := makeServelessPlan(allPlan, replicas)
						allResultWithRB[j][k][i] = componentMat
					} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeAffinityDaemon {
						// same as serverless.
						componentMat := makeServelessPlan(allPlan, replicas)
						allResultWithRB[j][k][i] = componentMat
					} else if comm.Workload.Workloadtype == v1alpha1.WorkloadTypeUserApp {
						componentMat, _ := makeUserAPPPlan(allPlan)
						allResultWithRB[j][k][i] = componentMat
					}
				}
			}
		}
	}

	rbsResultFinal := make([]*v1alpha1.ResourceBinding, 0)

	// NO.2 first we should spawn rbs.
	if desc.Namespace == common.GaiaReservedNamespace {
		// all 5
		rbsResultFinal = spawnResourceBindings(allResultGlobal, allClusters, desc, components, affinity)
		// 1. add networkFilter only if we can get nwr
		if nwr, err := g.cache.GetNetworkRequirement(desc); err == nil {
			networkInfoMap := g.getTopologyInfoMap()
			klog.Infof("Log: networkInfoMap is %v", networkInfoMap)
			klog.Infof("resource binding before net filter %v", rbsResultFinal)
			rbsResultFinal = npcore.NetworkFilter(rbsResultFinal, nwr, networkInfoMap)
			klog.Infof("resource binding after net filter %v", rbsResultFinal)
			if len(rbsResultFinal) == 0 {
				return result, errors.New("network filter can't find path for current rbs")
			}
		}
		if len(rbsResultFinal) > common.DefaultResouceBindingNumber {
			// score plugins.
			priorityList, scoreError := prioritizeResourcebindings(ctx, fwk, desc, allClusters, rbsResultFinal)
			if scoreError != nil {
				klog.Warningf("score plugin run error %v", scoreError)
			}
			// select 2
			rbsResultFinal, err = g.selectResourceBindings(priorityList, rbsResultFinal)
		}
	} else {
		rbIndex := 0
		for i, rbOld := range rbs {
			rbsResult := make([]*v1alpha1.ResourceBinding, 0)
			rbForrb := spawnResourceBindings(allResultWithRB[i], allClusters, desc, components, affinity)
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
				rbLabels := rbForrb[j].GetLabels()
				rbLabels[common.TotalPeerOfParentRB] = fmt.Sprintf("%d", rbOld.Spec.TotalPeer)
				rbNew := &v1alpha1.ResourceBinding{
					ObjectMeta: metav1.ObjectMeta{
						Name:   fmt.Sprintf("%s-%d", rbOld.Name, rbIndex),
						Labels: rbLabels,
					},
					Spec: v1alpha1.ResourceBindingSpec{
						AppID:             desc.Name,
						NonZeroClusterNum: rbOld.Spec.NonZeroClusterNum,
						ParentRB:          rbOld.Name,
						RbApps:            subRBApps,
						TotalPeer:         getTotalPeer(len(rbForrb), common.DefaultResouceBindingNumber),
						NetworkPath:       rbOld.Spec.NetworkPath,
					},
				}
				rbNew.Kind = "ResourceBinding"
				rbNew.APIVersion = "apps.gaia.io/v1alpha1"
				rbIndex += 1
				rbsResult = append(rbsResult, rbNew)
			}
			if len(rbsResult) > common.DefaultResouceBindingNumber {
				// score plugins.
				priorityList, scoreError := prioritizeResourcebindings(ctx, fwk, desc, allClusters, rbsResult)
				if scoreError != nil {
					klog.Warningf("score pulgin run error %v", scoreError)
				}
				// select prioritize
				rbsResult, err = g.selectResourceBindings(priorityList, rbsResult)
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

func nomalizeClusters(feasibleClusters []*framework2.ClusterInfo, allClusters []*clusterapi.ManagedCluster) []*framework2.ClusterInfo {
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
func (g *genericScheduler) selectResourceBindings(rbScoreList framework.ResourceBindingScoreList, result []*v1alpha1.ResourceBinding) ([]*v1alpha1.ResourceBinding, error) {
	if len(rbScoreList) == 0 {
		return nil, fmt.Errorf("empty rbScoreList")
	}
	sort.Sort(rbScoreList)

	// Top best 2 rbs.
	selected := []*v1alpha1.ResourceBinding{
		0: result[rbScoreList[0].Index],
		1: result[rbScoreList[1].Index],
	}
	return selected, nil
}

// Filters the clusters to find the ones that fit the subscription based on the framework filter plugins.
func (g *genericScheduler) findClustersThatFitComponent(ctx context.Context, fwk framework.Framework, comm *v1alpha1.Component) ([]*framework2.ClusterInfo, framework.Diagnosis, error) {
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
	result, _ := scheduleWorkload(non0CPU, non0MEM, feasibleClusters)

	return result, diagnosis, nil
}

// findClustersThatPassFilters finds the clusters that fit the filter plugins.
func (g *genericScheduler) findClustersThatPassFilters(ctx context.Context, fwk framework.Framework,
	com *v1alpha1.Component, diagnosis framework.Diagnosis,
	clusters []*clusterapi.ManagedCluster,
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
		// function is called for each cluster, whereas we want to have an overall latency for all clusters per scheduling cycle.
		metrics.FrameworkExtensionPointDuration.WithLabelValues(runtime.Filter, statusCode.String(), fwk.ProfileName()).Observe(metrics.SinceInSeconds(beginCheckCluster))
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

func getTotalPeer(rbsResultNum, threshold int) int {
	if rbsResultNum < threshold {
		return rbsResultNum
	}
	return threshold
}
