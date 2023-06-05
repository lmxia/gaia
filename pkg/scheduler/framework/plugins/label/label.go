package label

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/selection"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	gaiaLabels "github.com/lmxia/gaia/pkg/scheduler/framework/plugins/label/labels"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
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

	netEnvironmentMap, nodeRoleMap, resFormMap, runtimeStateMap, snMap, geolocationMap, providersMap := cluster.GetHypernodeLabelsMapFromManagedCluster()
	clusterLabels := make(map[string][]string, 0)

	// no.1
	netEnvironments := make([]string, 0, len(netEnvironmentMap))
	for k := range netEnvironmentMap {
		netEnvironments = append(netEnvironments, k)
	}
	clusterLabels["net-environment"] = netEnvironments
	// no.2
	nodeRoles := make([]string, 0, len(nodeRoleMap))
	for k := range nodeRoleMap {
		nodeRoles = append(nodeRoles, k)
	}
	clusterLabels["node-role"] = nodeRoles
	// no.3
	resForms := make([]string, 0, len(resFormMap))
	for k := range resFormMap {
		resForms = append(resForms, k)
	}
	clusterLabels["res-form"] = resForms
	// no.4
	runtimeStates := make([]string, 0, len(runtimeStateMap))
	for k := range runtimeStateMap {
		runtimeStates = append(runtimeStates, k)
	}
	clusterLabels["runtime-state"] = runtimeStates
	// no.5
	sns := make([]string, 0, len(snMap))
	for k := range snMap {
		sns = append(sns, k)
	}
	clusterLabels["sn"] = sns
	// no.6
	geolocations := make([]string, 0, len(geolocationMap))
	for k := range geolocationMap {
		geolocations = append(geolocations, k)
	}
	clusterLabels["geo-location"] = geolocations
	// no.7
	providers := make([]string, 0, len(providersMap))
	for k := range providersMap {
		providers = append(providers, k)
	}
	clusterLabels["supplier-name"] = providers

	gaiaSelector, err := LabelSelectorAsSelector(com.SchedulePolicy.Level[v1alpha1.SchedulePolicyMandatory])
	if err != nil {
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("can't change gaia schedule policy to label selector %v, component name is %v", com.SchedulePolicy.Level[v1alpha1.SchedulePolicyMandatory], err))
	}
	if gaiaSelector.Matches(gaiaLabels.Set(clusterLabels)) {
		return nil
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no geoLocation fit for this com. cluster name is %v, component name is %v", cluster.Name, com.Name))
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
