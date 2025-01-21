package framework

import (
	"os"

	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/utils"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/klog/v2"
)

// ClusterInfo is cluster level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	Cluster *v1alpha1.ManagedCluster
	Total   int64
}

// CalculateCapacity  pod is used to check affinity, TODO.
func (c *ClusterInfo) CalculateCapacity(cpuRequest int64, memRequest int64, gpuReqMap map[string]int) int64 {
	freeCPULeft := c.Cluster.Status.Available.Cpu().MilliValue()
	freeMemLeft := c.Cluster.Status.Available.Memory().Value()
	cpuCapacity := freeCPULeft / cpuRequest
	memCapacity := freeMemLeft / memRequest

	if len(gpuReqMap) != 0 {
		var gpuCapacityS []int
		for _, nodeRes := range c.Cluster.Status.NodesResources {
			var allGPUS []int
			for gpu, count := range gpuReqMap {
				gpuCap := nodeRes.AvailableGPU[gpu] / count
				allGPUS = append(allGPUS, gpuCap)
			}
			allGPUCap := utils.MinInSliceUsingSort(allGPUS)
			gpuCapacityS = append(gpuCapacityS, allGPUCap)
		}
		gpuCapacity := utils.MaxInSliceUsingSort(gpuCapacityS)

		return minInt64(int64(gpuCapacity), minInt64(cpuCapacity, memCapacity))
	}

	return minInt64(cpuCapacity, memCapacity)
}

// NodeInfo is node level aggregated information.
type NodeInfo struct {
	// Overall cluster information.
	Node  *coreV1.Node
	Total int64
}

func (n *NodeInfo) CalculateCapacity(cpuRequest int64, memRequest int64) int64 {
	promURLPrefix := os.Getenv("PromURLPrefix")
	if promURLPrefix == "" {
		klog.Errorf("failed to get PromURLPrefix from env, Calculate node Capacity error")
		return 0
	}
	freeCPULeft, freeMemLeft := utils.GetEachNodeResourceFromPrometheus(n.Node, promURLPrefix)
	cpuCapacity := freeCPULeft.MilliValue() / cpuRequest
	memCapacity := freeMemLeft.Value() / memRequest

	// todo get gpuCapacity

	return minInt64(cpuCapacity, memCapacity)
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
