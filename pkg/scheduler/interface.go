package scheduler

import (
	"github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

type Cache interface {
	// AddNode adds overall information about node.
	// It returns a clone of added ClusterInfo object.
	AddCluster(cluster *v1alpha1.ManagedCluster) error

	// UpdateNode updates overall information about node.
	// It returns a clone of updated ClusterInfo object.
	UpdateCluster(oldNode, newNode *v1alpha1.ManagedCluster) error

	// RemoveNode removes overall information about node.
	RemoveCluster(node *v1alpha1.ManagedCluster) error

	// GetClusters returns a list of all clusters.
	GetClusters() []string

	GetClusterCapacity(cluster string, cpu int64, mem int64, pod *corev1.Pod) int64

	GetCluster(clusterName string) *v1alpha1.ManagedCluster

}