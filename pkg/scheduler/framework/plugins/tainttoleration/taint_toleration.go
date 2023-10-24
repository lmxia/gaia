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
func (pl *TaintToleration) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("invalid cluster"))
	}

	filterPredicate := func(t *v1.Taint) bool {
		// only interested in NoSchedule and NoExecute taints.
		return t.Effect == v1.TaintEffectNoSchedule || t.Effect == v1.TaintEffectNoExecute
	}

	taint, isUntolerated := v1helper.FindMatchingUntoleratedTaint(cluster.Spec.Taints, com.ClusterTolerations, filterPredicate)
	if !isUntolerated {
		return nil
	}

	errReason := fmt.Sprintf("clusters(s) had taint {%s: %s}, that the component didn't tolerate. cluster name is %v, component name is %v",
		taint.Key, taint.Value, cluster.Name, com.Name)
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, errReason)
}

// getAllTolerationEffectPreferNoSchedule gets the list of all Tolerations with Effect PreferNoSchedule or with no effect.
// copied from k8s.io/kubernetes/pkg/scheduler/framework/plugins/tainttoleration/taint_toleration.go
func getAllTolerationPreferNoSchedule(tolerations []v1.Toleration) (tolerationList []v1.Toleration) {
	for _, toleration := range tolerations {
		// Empty effect means all effects which includes PreferNoSchedule, so we need to collect it as well.
		if len(toleration.Effect) == 0 || toleration.Effect == v1.TaintEffectPreferNoSchedule {
			tolerationList = append(tolerationList, toleration)
		}
	}
	return
}

// CountIntolerableTaintsPreferNoSchedule gives the count of intolerable taints of a subscription with effect PreferNoSchedule
// copied from k8s.io/kubernetes/pkg/scheduler/framework/plugins/tainttoleration/taint_toleration.go
func countIntolerableTaintsPreferNoSchedule(taints []v1.Taint, tolerations []v1.Toleration) (intolerableTaints int) {
	for _, taint := range taints {
		// check only on taints that have effect PreferNoSchedule
		if taint.Effect != v1.TaintEffectPreferNoSchedule {
			continue
		}

		if !v1helper.TolerationsTolerateTaint(tolerations, &taint) {
			intolerableTaints++
		}
	}
	return
}

// NormalizeScore invoked after scoring all clusters.
func (pl *TaintToleration) NormalizeScore(ctx context.Context, scores framework.ResourceBindingScoreList) *framework.Status {
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
