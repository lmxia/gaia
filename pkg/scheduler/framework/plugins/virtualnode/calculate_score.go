package virtualnode

import (
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	coreV1 "k8s.io/api/core/v1"
)

func calculateScore(score int64, apps []*v1alpha1.ResourceBindingApps,
	clusterMap map[string]*clusterapi.ManagedCluster,
) int64 {
	if len(clusterMap) == 0 {
		return score
	}
	for _, item := range apps {
		cluster := clusterMap[item.ClusterName]
		if cluster != nil && cluster.GetLabels() != nil {
			_, _, resFormMap, _, _, _, _, _, _, _, _, _, _, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
			if _, exist := resFormMap[common.NodeResourceForm]; exist {
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
	if len(nodeMap) == 0 {
		return score
	}
	for _, item := range apps {
		node := nodeMap[item.ClusterName] // nodeID
		if node != nil {
			if node.GetLabels()[clusterapi.ParsedResFormKey] == common.NodeResourceForm {
				for _, v := range item.Replicas {
					score += int64(v)
				}
				score = calculateScoreVN(score, item.Children, nodeMap)
			}
		}
	}
	return score
}
