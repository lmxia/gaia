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
	lastUpdateTimes      map[string]OfflineCount
	clusterSynced        cache.InformerSynced
	managedClusterLister listerv1alpha1.ManagedClusterLister
	sync.Mutex
	gaiaClient gaiaclientset.Interface
}

type OfflineCount struct {
	lastUpdateTime metav1.Time
	offlineCount   int64
}

func NewClusterWatcher(gaiaFactory externalversions.SharedInformerFactory,
	client gaiaclientset.Interface,
) *ClusterWatcher {
	clusterWatcher := &ClusterWatcher{
		managedClusterLister: gaiaFactory.Platform().V1alpha1().ManagedClusters().Lister(),
		gaiaClient:           client,
		clusterSynced:        gaiaFactory.Platform().V1alpha1().ManagedClusters().Informer().HasSynced,
		lastUpdateTimes:      make(map[string]OfflineCount),
	}

	return clusterWatcher
}

func (c *ClusterWatcher) Run(stopCh <-chan struct{}) error {
	klog.Infof("Starting ClusterWatcher")
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
		if !cluster.Status.Livez {
			continue
		}

		clusterName := cluster.GetName()
		updateTime := cluster.Status.LastObservedTime

		// 检查是否超时  保存当前的 LastUpdateTime
		c.Lock()
		if prevUpdateTime, found := c.lastUpdateTimes[clusterName]; found {
			if prevUpdateTime.lastUpdateTime.Time.Before(updateTime.Time) {
				c.lastUpdateTimes[clusterName] = OfflineCount{
					lastUpdateTime: updateTime,
					offlineCount:   0,
				}
			} else {
				c.lastUpdateTimes[clusterName] = OfflineCount{
					lastUpdateTime: updateTime,
					offlineCount:   prevUpdateTime.offlineCount + 1,
				}
			}
		} else {
			c.lastUpdateTimes[clusterName] = OfflineCount{
				lastUpdateTime: updateTime,
				offlineCount:   0,
			}
		}
		c.Unlock()

		klog.V(5).Infof("Cluster %s last update time: %v, currentTime: %v, offlineCount: %d\n",
			clusterName, updateTime, time.Now(), c.lastUpdateTimes[clusterName].offlineCount)
		// 检查是否超时
		if c.lastUpdateTimes[clusterName].offlineCount >= 10 && cluster.Status.Livez {
			klog.Warningf("Cluster %s has not been updated for more than 5 minutes, marking as NotReady\n",
				clusterName)

			// 更新状态为 NotReady
			cluster.Status.Livez = false
			_, err := client.PlatformV1alpha1().ManagedClusters(cluster.Namespace).UpdateStatus(context.TODO(),
				cluster, metav1.UpdateOptions{})
			if err != nil {
				klog.Errorf("Error updating status for cluster %s: %v\n", clusterName, err)
			}
		}
	}
}
