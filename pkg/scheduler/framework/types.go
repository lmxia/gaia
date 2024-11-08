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
func (c *ClusterInfo) CalculateCapacity(cpuRequest int64, memRequest int64) int64 {
	freeCPULeft := c.Cluster.Status.Available.Cpu().MilliValue()
	freeMemLeft := c.Cluster.Status.Available.Memory().Value()
	cpuCapacity := freeCPULeft / cpuRequest
	memCapacity := freeMemLeft / memRequest
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
	freeCPULeft, freeMemLeft := utils.GetNodeResFromProm(n.Node, promURLPrefix)

	cpuCapacity := freeCPULeft.MilliValue() / cpuRequest
	memCapacity := freeMemLeft.Value() / memRequest
	return minInt64(cpuCapacity, memCapacity)
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
