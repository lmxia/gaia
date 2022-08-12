package geolocation

import (
	"context"
	"fmt"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"

	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"k8s.io/apimachinery/pkg/runtime"
)

// Geolocation is a plugin that checks if a cluster filter geolocation.
type Geolocation struct {
	handle framework.Handle
}

var _ framework.FilterPlugin = &Geolocation{}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *Geolocation) Name() string {
	return names.TaintToleration
}

// Filter invoked at the filter extension point.
func (pl *Geolocation) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("geolocation invalid cluster "))
	}
	if com.SchedulePolicy.GeoLocation == nil {
		return nil
	}

	geoLocations := com.SchedulePolicy.GeoLocation.MatchExpressions[0].Values
	_, _, _, _, _, geoLocationMap, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
	for _, location := range geoLocations {
		if _, exist := geoLocationMap[location]; exist {
			return nil
		}
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no geoLocation fit for this com. cluster name is %v, component name is %v", cluster.Name, com.Name))

}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &Geolocation{handle: h}, nil
}
