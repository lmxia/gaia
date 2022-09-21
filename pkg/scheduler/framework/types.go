package framework

import (
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
)

// ClusterInfo is cluster level level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	Cluster *v1alpha1.ManagedCluster
	Total   int64
}

// pod is used to check affinity, TODO.
func (c *ClusterInfo) CalculateCapacity(cpuRequest int64, memRequest int64) int64 {
	freeCpuLeft := c.Cluster.Status.Available.Cpu().MilliValue()
	freeMemLeft := c.Cluster.Status.Available.Memory().Value()
	cpuCapacity := freeCpuLeft / cpuRequest
	memCapacity := freeMemLeft / memRequest
	return minInt64(cpuCapacity, memCapacity)
}

func minInt64(a int64, b int64) int64 {
	if a < b {
		return a
	}
	return b
}
