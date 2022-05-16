package main

import (
	"fmt"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/networkfilter/npcore"
)

func main() {
	fmt.Printf("Hello,Main func!\n")
	//logx.NewLogger()
	//npcore.Register()
	rbs, networkRequirement := npcore.SetRbsAndNetworksRequirment()
	var networkInfoMap map[string]clusterapi.Topo
	networkInfoMap = make(map[string]clusterapi.Topo)
	npcore.NetworkFilter(rbs, networkRequirement, networkInfoMap)
	//npcore.NetworkFilter_old()

}
