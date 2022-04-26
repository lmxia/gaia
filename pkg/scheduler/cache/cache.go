package cache

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	applisters "github.com/lmxia/gaia/pkg/generated/listers/apps/v1alpha1"
	platformlisters "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"
)

type Cache interface {
	// NumClusters returns the number of clusters in the cache.
	NumClusters() int

	// ListClusters returns the list of ManagedCluster(s).
	ListClusters(labelSelector *metav1.LabelSelector) ([]*clusterapi.ManagedCluster, error)

	// GetCLuster returns the ManagedCluster of the given managed cluster.
	GetCLuster(namespacedName string) (*clusterapi.ManagedCluster, error)

	// SetRBLister set rb lister to watch rb in parent cluster
	SetRBLister(lister applisters.ResourceBindingLister)

	// GetResourceBindings get rb related to description with specific status.
	ListResourceBindings(description *v1alpha1.Description, status string) []*v1alpha1.ResourceBinding

	//SetSelfClusterName set self cluster name
	SetSelfClusterName(name string)

	GetSelfClusterName() string
}

type schedulerCache struct {
	clusterListers        platformlisters.ManagedClusterLister
	resourcebindingLister applisters.ResourceBindingLister
	selfClusterName       string
}

// NumClusters returns the number of clusters in the cache.
func (s *schedulerCache) NumClusters() int {
	clusters, err := s.ListClusters(&metav1.LabelSelector{})
	if err != nil {
		klog.Errorf("failed to list clusters: %v", err)
		return 0
	}
	return len(clusters)
}

// ListClusters returns the list of clusters in the cache.
func (s *schedulerCache) ListClusters(labelSelector *metav1.LabelSelector) ([]*clusterapi.ManagedCluster, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	return s.clusterListers.List(selector)
}

// Get returns the ManagedCluster of the given cluster.
func (s *schedulerCache) GetCLuster(namespacedName string) (*clusterapi.ManagedCluster, error) {
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(namespacedName)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", namespacedName))
		return nil, err
	}

	return s.clusterListers.ManagedClusters(ns).Get(name)
}

func New(clusterListers platformlisters.ManagedClusterLister) Cache {
	return &schedulerCache{
		clusterListers: clusterListers,
	}
}

func (s *schedulerCache) SetRBLister(lister applisters.ResourceBindingLister) {
	s.resourcebindingLister = lister
}

func (s *schedulerCache) SetSelfClusterName(name string) {
	s.selfClusterName = name
}

func (s *schedulerCache) GetSelfClusterName() string {
	return s.selfClusterName
}

func (s *schedulerCache) ListResourceBindings(desc *v1alpha1.Description, status string) []*v1alpha1.ResourceBinding {
	labelSelector := labels.NewSelector()
	requirement, err := labels.NewRequirement(common.GaiaDescriptionLabel, selection.Equals, []string{desc.Name})
	if err != nil {
		return nil
	}
	labelSelector = labelSelector.Add(*requirement)

	if rbs, err := s.resourcebindingLister.List(labelSelector); err != nil {
		return nil
	} else {
		return rbs
	}
}
