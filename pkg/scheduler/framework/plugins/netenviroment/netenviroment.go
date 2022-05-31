package netenviroment

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

var _ framework.FilterPlugin = &NetEnviroment{}

// AffinityDaemon is a plugin that checks if a commponent fit a cluster's sn.
type NetEnviroment struct {
	handle framework.Handle
}

func (n NetEnviroment) Name() string {
	return names.NetEnviroment
}

func (n NetEnviroment) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if com.SchedulePolicy.NetEnvironment == nil {
		return nil
	}

	netEnvironments := com.SchedulePolicy.NetEnvironment.MatchExpressions[0].Values
	netEnviromentMap, _, _, _, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
	for _, netEnviroment := range netEnvironments {
		if _, exist := netEnviromentMap[netEnviroment]; exist {
			return nil
		}
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no netenviroment fit for this com."))
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &NetEnviroment{handle: h}, nil
}
