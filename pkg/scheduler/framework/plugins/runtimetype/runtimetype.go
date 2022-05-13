package runtimetype

import (
	"context"
	"fmt"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"k8s.io/apimachinery/pkg/runtime"
)

// RuntimeType is a plugin that checks if a subscription tolerates a cluster's taints.
type RuntimeType struct {
	handle framework.Handle
}

var _ framework.FilterPlugin = &RuntimeType{}

// Name returns name of the plugin. It is used in logs, etc.
func (pl *RuntimeType) Name() string {
	return names.RuntimeType
}

// Filter invoked at the filter extension point.
func (pl *RuntimeType) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("invalid cluster"))
	}

	if !fit(com, cluster) {
		errReason := fmt.Sprintf("cluster of designated RuntimeType %q not found", com.RuntimeType)
		return framework.NewStatus(framework.UnschedulableAndUnresolvable, errReason)
	}

	return nil
}

func fit(com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) bool {
	_, _, _, runtimeStateMap, _, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
	_, ok := runtimeStateMap[com.RuntimeType]
	return len(com.RuntimeType) == 0 || ok
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &RuntimeType{handle: h}, nil
}
