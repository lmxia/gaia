package label

import (
	"context"
	"fmt"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/helper"
	gaiaLabels "github.com/lmxia/gaia/pkg/scheduler/framework/plugins/label/labels"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"
)

// ClusterAffinity is a plugin that checks if a cluster filter various labels.
type ClusterAffinity struct {
	handle framework.Handle
}

var _ framework.FilterPlugin = &ClusterAffinity{}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *ClusterAffinity) Name() string {
	return names.Label
}

// Filter invoked at the filter extension point.
func (pl *ClusterAffinity) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("label invalid cluster "))
	}
	if com.SchedulePolicy.Level[v1alpha1.SchedulePolicyMandatory] == nil {
		// The component doesn't have any scheduling conditions.
		return nil
	}
	clusterLabels := cluster.GetGaiaLabels()
	gaiaSelector, err := LabelSelectorAsSelector(com.SchedulePolicy.Level[v1alpha1.SchedulePolicyMandatory])
	if err != nil {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("can't change gaia schedule policy to label selector %v, component name is %v", com.SchedulePolicy.Level[v1alpha1.SchedulePolicyMandatory], err))
	}
	if gaiaSelector.Matches(gaiaLabels.Set(clusterLabels)) {
		return nil
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no label fit for this com. cluster name is %v, component name is %v, clusterLabels is %+v, gaiaSelector is %+v", cluster.Name, com.Name, clusterLabels, gaiaSelector))
}

// NormalizeScore invoked after scoring all clusters.
func (pl *ClusterAffinity) NormalizeScore(ctx context.Context, scores framework.ResourceBindingScoreList) *framework.Status {
	return helper.DefaultNormalizeScore(framework.MaxClusterScore, true, scores)
}

func (pl *ClusterAffinity) Score(ctx context.Context, _ *v1alpha1.Description, rb *v1alpha1.ResourceBinding, clusters []*clusterapi.ManagedCluster) (int64, *framework.Status) {
	clusterMap := make(map[string]*clusterapi.ManagedCluster, 0)
	for _, cluster := range clusters {
		clusterMap[cluster.Name] = cluster
	}

	return calculateScore(0, rb.Spec.RbApps, clusterMap), nil
}

// ScoreExtensions of the Score plugin.
func (pl *ClusterAffinity) ScoreExtensions() framework.ScoreExtensions {
	return pl
}

func calculateScore(score int64, apps []*v1alpha1.ResourceBindingApps, clusterMap map[string]*clusterapi.ManagedCluster) int64 {
	for _, item := range apps {
		cluster := clusterMap[item.ClusterName]
		if cluster != nil && cluster.GetLabels() != nil {
			netenviroments, _, _, _, _, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
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

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &ClusterAffinity{handle: h}, nil
}

// LabelSelectorAsSelector converts the LabelSelector api type into a struct that implements
// labels.Selector

func LabelSelectorAsSelector(ps *metav1.LabelSelector) (gaiaLabels.Selector, error) {
	if ps == nil {
		return gaiaLabels.Nothing(), nil
	}
	if len(ps.MatchLabels)+len(ps.MatchExpressions) == 0 {
		return gaiaLabels.Everything(), nil
	}
	requirements := make([]gaiaLabels.Requirement, 0, len(ps.MatchLabels)+len(ps.MatchExpressions))
	for k, v := range ps.MatchLabels {
		r, err := gaiaLabels.NewRequirement(k, selection.Equals, []string{v})
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, *r)
	}
	for _, expr := range ps.MatchExpressions {
		var op selection.Operator
		switch expr.Operator {
		case metav1.LabelSelectorOpIn:
			op = selection.In
		case metav1.LabelSelectorOpNotIn:
			op = selection.NotIn
		case metav1.LabelSelectorOpExists:
			op = selection.Exists
		case metav1.LabelSelectorOpDoesNotExist:
			op = selection.DoesNotExist
		default:
			return nil, fmt.Errorf("%q is not a valid pod selector operator", expr.Operator)
		}
		r, err := gaiaLabels.NewRequirement(expr.Key, op, append([]string(nil), expr.Values...))
		if err != nil {
			return nil, err
		}
		requirements = append(requirements, *r)
	}
	selector := gaiaLabels.NewGaiaSelector()
	selector = selector.Add(requirements...)
	return selector, nil
}
