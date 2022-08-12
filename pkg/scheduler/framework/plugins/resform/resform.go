package resform

import (
	"context"
	"fmt"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"k8s.io/apimachinery/pkg/runtime"
)

var _ framework.FilterPlugin = &ResForm{}

// AffinityDaemon is a plugin that checks if a commponent fit a cluster's sn.
type ResForm struct {
	handle framework.Handle
}

func (r ResForm) Name() string {
	return names.NetEnviroment
}

func (r ResForm) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("resform invalid cluster "))
	}
	_, _, resFormMap, _, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
	if len(resFormMap) > 0 {
		return nil
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no resform fit for this com. cluster name is %v, component name is %v", cluster.Name, com.Name))
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &ResForm{handle: h}, nil
}
