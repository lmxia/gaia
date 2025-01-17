package corenetworkpriority

import (
	"context"

	coreV1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/helper"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

// TaintToleration is a plugin that checks if a subscription tolerates a cluster's taints.
type CoreNetworkPriority struct {
	handle framework.Handle
}

var _ framework.ScorePlugin = &CoreNetworkPriority{}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *CoreNetworkPriority) Name() string {
	return names.CorePriority
}

// NormalizeScore invoked after scoring all clusters.
func (pl *CoreNetworkPriority) NormalizeScore(ctx context.Context,
	scores framework.ResourceBindingScoreList,
) *framework.Status {
	return helper.DefaultNormalizeScore(framework.MaxClusterScore, true, scores)
}

func (pl *CoreNetworkPriority) Score(ctx context.Context, _ *v1alpha1.Description, rb *v1alpha1.ResourceBinding,
	clusters []*clusterapi.ManagedCluster,
) (int64, *framework.Status) {
	clusterMap := make(map[string]*clusterapi.ManagedCluster, 0)
	for _, cluster := range clusters {
		clusterMap[cluster.Name] = cluster
	}

	return calculateScore(0, rb.Spec.RbApps, clusterMap), nil
}

func (pl *CoreNetworkPriority) ScoreVN(ctx context.Context, _ *v1alpha1.Description, rb *v1alpha1.ResourceBinding,
	nodes []*coreV1.Node,
) (int64, *framework.Status) {
	nodeMap := make(map[string]*coreV1.Node, 0)
	for _, node := range nodes {
		nodeMap[node.Name] = node
	}

	return calculateScoreVN(0, rb.Spec.RbApps, nodeMap), nil
}

// ScoreExtensions of the Score plugin.
func (pl *CoreNetworkPriority) ScoreExtensions() framework.ScoreExtensions {
	return pl
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &CoreNetworkPriority{handle: h}, nil
}
