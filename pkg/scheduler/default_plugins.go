package scheduler

import (
	schedulerapis "github.com/lmxia/gaia/pkg/scheduler/apis"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
)

// getDefaultPlugins returns the default set of plugins.
func getDefaultPlugins() *schedulerapis.Plugins {
	return &schedulerapis.Plugins{
		PreFilter: schedulerapis.PluginSet{},
		Filter: schedulerapis.PluginSet{
			Enabled: []schedulerapis.Plugin{
				{Name: names.TaintToleration},
				{Name: names.SpecificResource},
				{Name: names.AffinityDaemon},
				{Name: names.NetEnviroment},
			},
		},
		PostFilter: schedulerapis.PluginSet{},
		PreScore:   schedulerapis.PluginSet{},
		Score: schedulerapis.PluginSet{
			Enabled: []schedulerapis.Plugin{
				{Name: names.CorePriority, Weight: 3},
			},
		},
	}
}
