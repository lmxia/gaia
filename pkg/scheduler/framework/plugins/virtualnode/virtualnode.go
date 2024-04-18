package virtualnode

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/helper"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

// TaintToleration is a plugin that checks if a subscription tolerates a cluster's taints.
type VirtualNode struct {
	handle framework.Handle
}

var _ framework.ScorePlugin = &VirtualNode{}

// Name returns name of the plugin. It is used in logs, etc.
func (v *VirtualNode) Name() string {
	return names.VirtualNode
}

// NormalizeScore invoked after scoring all clusters.
func (v *VirtualNode) NormalizeScore(ctx context.Context, scores framework.ResourceBindingScoreList) *framework.Status {
	return helper.DefaultNormalizeScore(framework.MaxClusterScore, true, scores)
}

func (v *VirtualNode) Score(ctx context.Context, _ *v1alpha1.Description, rb *v1alpha1.ResourceBinding,
	clusters []*clusterapi.ManagedCluster) (int64, *framework.Status) {
	clusterMap := make(map[string]*clusterapi.ManagedCluster, 0)
	for _, cluster := range clusters {
		clusterMap[cluster.Name] = cluster
	}

	return calculateScore(0, rb.Spec.RbApps, clusterMap), nil
}

// ScoreExtensions of the Score plugin.
func (v *VirtualNode) ScoreExtensions() framework.ScoreExtensions {
	return v
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &VirtualNode{handle: h}, nil
}
