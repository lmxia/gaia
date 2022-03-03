package scheduler

import (
	"sync"

	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/scheduler/framework"
	corev1 "k8s.io/api/core/v1"
)

type schedulerCache struct {
	stop <-chan struct{}
	// This mutex guards all fields within this cache struct.
	mu       sync.RWMutex
	clusters map[string]*framework.ClusterInfo
}

func newSchedulerCache() *schedulerCache {
	return &schedulerCache{
		clusters: make(map[string]*framework.ClusterInfo),
	}
}

func (cache *schedulerCache) AddCluster(cluster *v1alpha1.ManagedCluster) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	c, ok := cache.clusters[cluster.Name]
	if !ok {
		c = &framework.ClusterInfo{
			Cluster: cluster,
		}
		cache.clusters[cluster.Name] = c
	}
	return nil
}

func (cache *schedulerCache) UpdateCluster(oldCluster, newCluster *v1alpha1.ManagedCluster) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	c := &framework.ClusterInfo{
		Cluster: newCluster,
	}
	cache.clusters[newCluster.Name] = c

	return nil
}

func (cache *schedulerCache) RemoveCluster(cluster *v1alpha1.ManagedCluster) error {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	delete(cache.clusters, cluster.Name)
	return nil
}

func (cache *schedulerCache) GetClusters() []string {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	clusterNames := make([]string, 0)
	for cluster := range cache.clusters {
		clusterNames = append(clusterNames, cluster)
	}
	return clusterNames
}

func (cache *schedulerCache) GetCluster(clusterName string) *v1alpha1.ManagedCluster {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// no time to check.
	return cache.clusters[clusterName].Cluster
}

func (cache *schedulerCache) GetClusterCapacity(cluster string, cpu int64, mem int64, pod *corev1.Pod) int64 {
	cache.mu.Lock()
	defer cache.mu.Unlock()
	if info, ok := cache.clusters[cluster]; ok {
		return info.CalculateCapacity(cpu, mem, pod)
	} else {
		return 0
	}
}
