package label

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

// Label is a plugin that checks if a cluster filter various labels.
type Label struct {
	handle framework.Handle
}

var _ framework.FilterPlugin = &Label{}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *Label) Name() string {
	return names.Label
}

// Filter invoked at the filter extension point.
func (pl *Label) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("label invalid cluster "))
	}
	//
	//netEnvironmentMap, nodeRoleMap, resFormMap, runtimeStateMap, snMap, geolocationMap, providers := cluster.GetHypernodeLabelsMapFromManagedCluster()
	//
	//for _, location := range geoLocations {
	//	if _, exist := geoLocationMap[location]; exist {
	//		return nil
	//	}
	//}
	//
	//if s.labelSelector != nil {
	//	if !s.labelSelector.Matches(labels.Set(node.Labels)) {
	//		return false, nil
	//	}
	//}
	//if s.nodeSelector != nil {
	//	return s.nodeSelector.Match(node)
	//}
	//
	//selector, err = metav1.LabelSelectorAsSelector(daemonSet.Spec.Selector)
	//if err != nil {
	//	// this should not happen if the DaemonSet passed validation
	//	return nil, err
	//}
	//
	//// If a daemonSet with a nil or empty selector creeps in, it should match nothing, not everything.
	//if selector.Empty() || !selector.Matches(labels.Set(pod.Labels)) {
	//	continue
	//}

	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no geoLocation fit for this com. cluster name is %v, component name is %v", cluster.Name, com.Name))

}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &Label{handle: h}, nil
}
