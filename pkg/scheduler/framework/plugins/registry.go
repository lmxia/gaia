package plugins

import (
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/corenetworkpriority"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/tainttoleration"
	"github.com/lmxia/gaia/pkg/scheduler/framework/runtime"
)

// NewInTreeRegistry builds the registry with all the in-tree plugins.
func NewInTreeRegistry() runtime.Registry {
	return runtime.Registry{
		names.TaintToleration: tainttoleration.New,
		names.CorePriority: corenetworkpriority.New,
	}
}
