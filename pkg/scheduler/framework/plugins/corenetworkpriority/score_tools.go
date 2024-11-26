package corenetworkpriority

import (
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	coreV1 "k8s.io/api/core/v1"
)

func calculateScore(score int64, apps []*v1alpha1.ResourceBindingApps,
	clusterMap map[string]*clusterapi.ManagedCluster,
) int64 {
	for _, item := range apps {
		cluster := clusterMap[item.ClusterName]
		if cluster != nil && cluster.GetLabels() != nil {
			netenviroments, _, _, _, _, _, _, _, _, _, _, _, _, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
			if _, exist := netenviroments[common.NetworkLocationCore]; exist {
				for _, v := range item.Replicas {
					score += int64(v)
				}
				score = calculateScore(score, item.Children, clusterMap)
			}
		}
	}
	return score
}

func calculateScoreVN(score int64, apps []*v1alpha1.ResourceBindingApps,
	nodeMap map[string]*coreV1.Node,
) int64 {
	for _, item := range apps {
		node := nodeMap[item.ClusterName]
		if node != nil {
			if node.GetLabels()[clusterapi.NetEnvironmentKey] == common.NetworkLocationCore {
				for _, v := range item.Replicas {
					score += int64(v)
				}
				score = calculateScoreVN(score, item.Children, nodeMap)
			}
		}
	}
	return score
}
