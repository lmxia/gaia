package supplier

import (
	"context"
	"fmt"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	platformv1 "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ framework.FilterPlugin = &SupplierName{}

// SupplierName is a plugin that checks if a commponent fit a cluster's sn.
type SupplierName struct {
	handle framework.Handle
}

func (s SupplierName) Name() string {
	return names.SpecificResource
}

func (s SupplierName) Filter(ctx context.Context, com *v1alpha1.Component, cluster *platformv1.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("supplier name invalid cluster "))
	}
	if com.SchedulePolicy.Provider == nil {
		return nil
	}
	providers := com.SchedulePolicy.Provider.MatchExpressions[0].Values
	_, _, _, _, _, _, providerMap := cluster.GetHypernodeLabelsMapFromManagedCluster()
	for _, provider := range providers {
		if _, exist := providerMap[provider]; exist {
			return nil
		}
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("clusters(s) had no supplier name sns "))
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &SupplierName{handle: h}, nil
}
