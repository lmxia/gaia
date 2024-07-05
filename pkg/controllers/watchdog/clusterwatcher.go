package watchdog

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	"github.com/lmxia/gaia/pkg/common"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	listerv1alpha1 "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"
)

type ClusterWatcher struct {
	// 存储上次更新时间
	lastUpdateTimes      map[string]metav1.Time
	clusterSynced        cache.InformerSynced
	managedClusterLister listerv1alpha1.ManagedClusterLister
	sync.Mutex
	gaiaClient gaiaclientset.Interface
}

func NewClusterWatcher(gaiaFactory externalversions.SharedInformerFactory,
	client gaiaclientset.Interface) *ClusterWatcher {
	clusterWatcher := &ClusterWatcher{
		managedClusterLister: gaiaFactory.Platform().V1alpha1().ManagedClusters().Lister(),
		gaiaClient:           client,
		clusterSynced:        gaiaFactory.Platform().V1alpha1().ManagedClusters().Informer().HasSynced,
		lastUpdateTimes:      make(map[string]metav1.Time),
	}

	return clusterWatcher
}

func (c *ClusterWatcher) Run(stopCh <-chan struct{}) error {
	// 定期检查 ManagedClusters 对象
	ticker := time.NewTicker(common.WatcherCheckPeriod)
	defer ticker.Stop()
	// Wait for the caches to be synced before starting workers
	if ok := cache.WaitForCacheSync(stopCh, c.clusterSynced); !ok {
		return fmt.Errorf("failed to wait for caches to sync")
	}
	for {
		select {
		case <-ticker.C:
			c.checkAndMarkNotReady(c.managedClusterLister, c.gaiaClient)
		case <-stopCh:
			return nil
		}
	}
}

func (c *ClusterWatcher) checkAndMarkNotReady(lister listerv1alpha1.ManagedClusterLister,
	client gaiaclientset.Interface) {
	clusterList, err := lister.List(labels.Everything())
	if err != nil {
		klog.Infof("Error watching ManagedCluster: %v\n", err)
		return
	}

	for _, cluster := range clusterList {
		clusterName := cluster.GetName()
		updateTime := cluster.Status.LastObservedTime

		// 保存当前的 LastUpdateTime
		c.Lock()
		if prevUpdateTime, found := c.lastUpdateTimes[clusterName]; found {
			if prevUpdateTime.Time.Before(updateTime.Time) {
				c.lastUpdateTimes[clusterName] = updateTime
			}
		} else {
			c.lastUpdateTimes[clusterName] = updateTime
		}
		c.Unlock()
		// 检查是否超时
		if time.Since(c.lastUpdateTimes[clusterName].Time) > 30*time.Second && !cluster.Status.Livez {
			fmt.Printf("Cluster %s has not been updated for more than 30 seconds, marking as NotReady\n",
				clusterName)

			// 更新状态为 NotReady
			cluster.Status.Livez = false
			_, err := client.PlatformV1alpha1().ManagedClusters(cluster.Namespace).UpdateStatus(context.TODO(),
				cluster, metav1.UpdateOptions{})
			if err != nil {
				fmt.Printf("Error updating status for cluster %s: %v\n", clusterName, err)
			}
		}
	}
}
