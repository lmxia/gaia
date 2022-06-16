package affinitydaemon

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

var _ framework.FilterPlugin = &AffinityDaemon{}

// AffinityDaemon is a plugin that checks if a commponent fit a cluster's sn.
type AffinityDaemon struct {
	handle framework.Handle
}

func (a AffinityDaemon) Name() string {
	return names.AffinityDaemon
}

func (a AffinityDaemon) Filter(ctx context.Context, com *v1alpha1.Component, cluster *clusterapi.ManagedCluster) *framework.Status {
	if cluster == nil {
		return framework.AsStatus(fmt.Errorf("invalid cluster"))
	}

	if com.Workload.Workloadtype == v1alpha1.WorkloadTypeAffinityDaemon {
		_, _, _, _, snMap, _, _ := cluster.GetHypernodeLabelsMapFromManagedCluster()
		for _, s := range com.Workload.TraitAffinityDaemon.SNS {
			if _, exist := snMap[s]; exist {
				return nil
			}
		}
	} else {
		return nil
	}

	errReason := fmt.Sprintf("this cluster {%s}, has no sn {%s}", cluster.Name, com.Workload.TraitAffinityDaemon.SNS)
	return framework.NewStatus(framework.UnschedulableAndUnresolvable, errReason)
}

// New initializes a new plugin and returns it.
func New(_ runtime.Object, h framework.Handle) (framework.Plugin, error) {
	return &AffinityDaemon{handle: h}, nil
}
