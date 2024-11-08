package algorithm

import (
	"context"

	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	framework "github.com/lmxia/gaia/pkg/scheduler/framework/interfaces"
)

// ScheduleAlgorithm is an interface implemented by things that know how to schedule resources to target
// managed clusters.
type ScheduleAlgorithm interface {
	Schedule(context.Context, framework.Framework, []*v1alpha1.ResourceBinding, *v1alpha1.Description,
	) (scheduleResult ScheduleResult, err error)
	ScheduleVN(context.Context, framework.Framework, []*v1alpha1.ResourceBinding, *v1alpha1.Description,
	) (scheduleResult ScheduleResult, err error)
	SetSelfClusterName(name string)
}

// ScheduleResult represents the result of one description scheduled.
type ScheduleResult struct {
	// the final rbs.
	ResourceBindings []*v1alpha1.ResourceBinding
}
