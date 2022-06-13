package plugins

import (
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/affinitydaemon"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/corenetworkpriority"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/geolocation"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/names"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/netenviroment"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/runtimetype"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/specificresource"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/supplier"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/tainttoleration"
	"github.com/lmxia/gaia/pkg/scheduler/framework/plugins/virtualnode"
	"github.com/lmxia/gaia/pkg/scheduler/framework/runtime"
)

// NewInTreeRegistry builds the registry with all the in-tree plugins.
func NewInTreeRegistry() runtime.Registry {
	return runtime.Registry{
		names.TaintToleration:  tainttoleration.New,
		names.CorePriority:     corenetworkpriority.New,
		names.AffinityDaemon:   affinitydaemon.New,
		names.SpecificResource: specificresource.New,
		names.NetEnviroment:    netenviroment.New,
		names.Geolocation:      geolocation.New,
		names.SupplierName:     supplier.New,
		//names.ResForm:          resform.New,
		names.RuntimeType: runtimetype.New,
		//names.NodeRole:         noderole.New,
		names.VirtualNode: virtualnode.New,
	}
}
