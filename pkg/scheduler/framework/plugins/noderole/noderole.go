package noderole

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

var _ framework.FilterPlugin = &NodeRole{}

// NodeRole is a plugin that checks if a commponent fit a cluster's sn.
type NodeRole struct {
	handle framework.Handle
}

func (n NodeRole) Name() string {
	return names.NetEnviroment
}

func (n NodeRole) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	_, nodeRoleMap, _, _, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
	if len(nodeRoleMap) > 0 {
		return nil
	}
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, fmt.Sprintf("there is no noderole fit for this com."))
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &NodeRole{handle: h}, nil
}
