package framework

import (
	"gaia.io/gaia/pkg/apis/platform/v1alpha1"
	v1 "k8s.io/api/core/v1"
)


// ClusterInfo is cluster level level aggregated information.
type ClusterInfo struct {
	// Overall cluster information.
	Cluster *v1alpha1.ManagedCluster
}

// pod is used to check affinity, TODO.
func (c *ClusterInfo) CalculateCapacity(cpuRequest int64, memRequest int64, _ *v1.Pod) int64 {
	freeCpuLeft := c.Cluster.Status.Allocatable.Cpu().MilliValue()
	freeMemLeft := c.Cluster.Status.Allocatable.Memory().Value()
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
