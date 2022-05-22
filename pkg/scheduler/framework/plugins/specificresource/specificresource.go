package specificresource

import (
	"context"
	"fmt"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	platformv1 "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ framework.FilterPlugin = &SpecificResource{}

// AffinityDaemon is a plugin that checks if a commponent fit a cluster's sn.
type SpecificResource struct {
	handle framework.Handle
}

func (s SpecificResource) Name() string {
	return names.SpecificResource
}

func (s SpecificResource) Filter(ctx context.Context, com *v1alpha1.Component, cluster *platformv1.ManagedCluster) *framework.Status {
	sns := com.SchedulePolicy.SpecificResource.MatchExpressions[0].Values
	_, _, _, _, snMap := cluster.GetHypernodeLabelsMapFromManagedCluster()
	for _, sn := range sns {
		if _, exist := snMap[sn]; exist {
			return nil
		}
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("clusters(s) had no specific sns "))
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &SpecificResource{handle: h}, nil
}
