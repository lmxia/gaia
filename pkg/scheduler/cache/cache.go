package cache

import (
	"context"
	"fmt"

	gaiaClientSet "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	coreV1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	platformlisters "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"
)

type Cache interface {
	// NumClusters returns the number of clusters in the cache.
	NumClusters() int

	// ListClusters returns the list of ManagedCluster(s).
	ListClusters(labelSelector *metav1.LabelSelector) ([]*clusterapi.ManagedCluster, error)

	// GetCLuster returns the ManagedCluster of the given managed cluster.
	GetCLuster(namespacedName string) (*clusterapi.ManagedCluster, error)

	// NumNodes returns the number of nodes in the cache.
	NumNodes() int

	// ListNodes returns the list of nodes.
	ListNodes(labelSelector *metav1.LabelSelector) ([]*coreV1.Node, error)

	// GetNode returns the Node of the given managed cluster.
	GetNode(name string) (*coreV1.Node, error)

	GetNetworkRequirement(description *v1alpha1.Description) (*v1alpha1.NetworkRequirement, error)

	// SetSelfClusterName set self cluster name
	SetSelfClusterName(name string)

	GetSelfClusterName() string
}

type schedulerCache struct {
	clusterListers  platformlisters.ManagedClusterLister
	nodeLister      corev1lister.NodeLister
	localGaiaClient *gaiaClientSet.Clientset
	localKubeClient *kubernetes.Clientset
	selfClusterName string
}

func (s *schedulerCache) NumNodes() int {
	nodes, err := s.ListNodes(&metav1.LabelSelector{})
	if err != nil {
		klog.Errorf("failed to list nodes: %v", err)
		return 0
	}
	return len(nodes)
}

func (s *schedulerCache) ListNodes(labelSelector *metav1.LabelSelector) ([]*coreV1.Node, error) {
	selector, err := metav1.LabelSelectorAsSelector(labelSelector)
	if err != nil {
		return nil, err
	}
	return s.nodeLister.List(selector)
}

func (s *schedulerCache) GetNode(name string) (*coreV1.Node, error) {
	return s.nodeLister.Get(name)
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

// GetCLuster returns the ManagedCluster of the given cluster.
func (s *schedulerCache) GetCLuster(namespacedName string) (*clusterapi.ManagedCluster, error) {
	// Convert the namespace/name string into a distinct namespace and name
	ns, name, err := cache.SplitMetaNamespaceKey(namespacedName)
	if err != nil {
		utilruntime.HandleError(fmt.Errorf("invalid resource key: %s", namespacedName))
		return nil, err
	}

	return s.clusterListers.ManagedClusters(ns).Get(name)
}

func New(clusterListers platformlisters.ManagedClusterLister, nodeLister corev1lister.NodeLister,
	localGaiaClient *gaiaClientSet.Clientset, localKubeClient *kubernetes.Clientset,
) Cache {
	return &schedulerCache{
		clusterListers:  clusterListers,
		nodeLister:      nodeLister,
		localGaiaClient: localGaiaClient,
		localKubeClient: localKubeClient,
	}
}

func (s *schedulerCache) SetSelfClusterName(name string) {
	s.selfClusterName = name
}

func (s *schedulerCache) GetSelfClusterName() string {
	return s.selfClusterName
}

func (s *schedulerCache) GetNetworkRequirement(desc *v1alpha1.Description) (*v1alpha1.NetworkRequirement, error) {
	nwr, err := s.localGaiaClient.AppsV1alpha1().NetworkRequirements(desc.Namespace).Get(context.TODO(),
		desc.Name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return nwr, nil
}
