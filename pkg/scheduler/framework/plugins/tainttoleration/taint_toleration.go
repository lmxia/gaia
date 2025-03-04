package tainttoleration

import (
	"context"
	"fmt"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	v1helper "k8s.io/component-helpers/scheduling/corev1"

	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/helper"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

// TaintToleration is a plugin that checks if a subscription tolerates a cluster's taints.
type TaintToleration struct {
	handle framework.Handle
}

var _ framework.FilterPlugin = &TaintToleration{}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *TaintToleration) Name() string {
	return names.TaintToleration
}

// Filter invoked at the filter extension point.
func (pl *TaintToleration) Filter(ctx context.Context, com *v1alpha1.Component,
	cluster *clusterapi.ManagedCluster,
) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("invalid cluster"))
	}

	filterPredicate := func(t *v1.Taint) bool {
		// only interested in NoSchedule and NoExecute taints.
		return t.Effect == v1.TaintEffectNoSchedule || t.Effect == v1.TaintEffectNoExecute
	}

	taint, isUntolerated := v1helper.FindMatchingUntoleratedTaint(cluster.Spec.Taints, com.ClusterTolerations,
		filterPredicate)
	if !isUntolerated {
		return nil
	}

	errReason := fmt.Sprintf("clusters(s) had taint {%s: %s}, that the component not tolerate: "+
		"cluster name is %v, component name is %v",
		taint.Key, taint.Value, cluster.Name, com.Name)
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, errReason)
}

func (pl *TaintToleration) FilterVN(ctx context.Context, com *v1alpha1.Component,
	node *v1.Node,
) *framework.Status {
	if node == nil {
		return framework.AsStatus(fmt.Errorf("invalid node"))
	}

	filterPredicate := func(t *v1.Taint) bool {
		// only interested in NoSchedule and NoExecute taints.
		return t.Effect == v1.TaintEffectNoSchedule || t.Effect == v1.TaintEffectNoExecute
	}

	taint, isUntolerated := v1helper.FindMatchingUntoleratedTaint(node.Spec.Taints, com.ClusterTolerations,
		filterPredicate)
	if !isUntolerated {
		return nil
	}

	errReason := fmt.Sprintf("node had taint {%s: %s}, that the component not tolerate, "+
		"node name is %v, component name is %v",
		taint.Key, taint.Value, node.Name, com.Name)
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, errReason)
}

// NormalizeScore invoked after scoring all clusters.
func (pl *TaintToleration) NormalizeScore(ctx context.Context,
	scores framework.ResourceBindingScoreList,
) *framework.Status {
	return helper.DefaultNormalizeScore(framework.MaxClusterScore, true, scores)
}

// ScoreExtensions of the Score plugin.
func (pl *TaintToleration) ScoreExtensions() framework.ScoreExtensions {
	return pl
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &TaintToleration{handle: h}, nil
}
