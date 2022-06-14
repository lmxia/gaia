package npcore

import (
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/networkfilter/logx"
	ncsnp "github.com/lmxia/gaia/pkg/networkfilter/model"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
	"github.com/timtadh/data-structures/tree/avl"
	//"k8s.io/apimachinery/pkg/labels"
)

/***********************************************************************************************************************/
/*********************************************data structure*******************************************************************/
/***********************************************************************************************************************/

type Local struct {
	BaseGraphPoint  *BaseGraph
	GraphTree       avl.AvlTree
	GraphDbArray    []GraphDb
	GraphDbRevision int64

	LocalDomainId   uint32
	LocalDomainType DomainType

	selfId2Component map[string]string       //k:string蓝图component中的selfID,v: SCNID_C1_1
	ComponentArray   map[string]APPComponent //k:蓝图componentName v:component attr

	DomainLinkKspGraphPoint *DomainLinkKspGraph
}

/***********************************************************************************************************************/
/*********************************************global variable*******************************************************************/
/***********************************************************************************************************************/

var (
	local *Local
)

const (
	KspCalcMaxNum         = 5
	DomainPathGroupMaxNum = 5
)

const (
	domainTypeString_Field           = "Field"
	domainTypeString_Fabric_Internet = "Fabric_Internet"
	domainTypeString_Fabric_Mpls     = "Fabric_Mpls"
	domainTypeString_Fabric_Otn      = "Fabric_Otn"
	domainTypeString_Fabric_Sdwan    = "Fabric_Sdwan"
)

const (
	DomainType_Invalid         DomainType = 0
	DomainType_Field           DomainType = 1
	DomainType_Fabric_Internet DomainType = 2
	DomainType_Fabric_MPLS     DomainType = 3
	DomainType_Fabric_OTN      DomainType = 4
	DomainType_Fabric_SDWAN    DomainType = 5
)

/***********************************************************************************************************************/
/*********************************************module*******************************************************************/
/***********************************************************************************************************************/
func (local *Local) Name() string {
	return "ncs-core"
}

func (local *Local) Start() {
	nputil.TraceInfoBegin("********************************************************")

	nputil.TraceInfoEnd("********************************************************")
}

func (local *Local) Enable() bool {
	return true
}

func (local *Local) GrShutdown() {
	nputil.TraceInfoBegin("********************************************************")

	nputil.TraceInfoEnd("********************************************************")
}

func newLocal() *Local {
	var local = new(Local)
	return local
}

func (local *Local) init() {
	nputil.TraceInfoBegin("")

	//Initialize the structure of Local
	local.LocalStructInit()
	//Initialize the graph in the Local area
	local.BaseGraphCreate()
	local.GraphCreate()

	nputil.TraceInfoEnd("")
}

func (local *Local) BaseGraphCreate() {
	nputil.TraceInfoBegin("")

	baseGraph := new(BaseGraph)
	baseGraph.BaseDomainGraphPoint = new(BaseDomainGraph)
	baseGraph.BaseDomainGraphPoint.BaseDomainTree = avl.AvlTree{}

	local.BaseGraphPoint = baseGraph

	nputil.TraceInfoEnd("")
	return
}

func (local *Local) GraphCreate() {
	nputil.TraceInfoBegin("")

	//TBD:当前只支持一种CSPF算法的Graph，其他类型Graph带补充
	graph := GraphCreateByKey(GraphTypeAlgo_Cspf)
	if graph == nil {
		nputil.TraceErrorString("graph is nil")
		return
	}

	graph.DomainGraphPoint = new(DomainGraph)
	graph.DomainGraphPoint.GraphPoint = graph
	graph.DomainGraphPoint.DomainTree = avl.AvlTree{}

	//创建的graph保存
	err := local.GraphTree.Put(graph.GraphDbV.Key, graph)
	if err != nil {
		rtnErr := errors.New("GraphTree put error")
		nputil.TraceErrorWithStack(rtnErr)
		return
	}
	local.GraphDbArray = append(local.GraphDbArray, graph.GraphDbV)

	nputil.TraceInfoEnd("")
	return
}

func (local *Local) LocalStructInit() {
	nputil.TraceInfoBegin("")

	local.ComponentArray = make(map[string]APPComponent)
	local.selfId2Component = make(map[string]string)

	nputil.TraceInfoEnd("")
	return
}

func Register() {
	local = newLocal()
	local.init()
}

func GetCoreLocal() *Local {
	return local
}

func GetLocalDomainId() uint32 {
	return local.LocalDomainId
}

func String2DomainType(domainTypeString string) DomainType {
	nputil.TraceInfoBegin("")

	if domainTypeString == domainTypeString_Field {
		nputil.TraceInfoEnd("DomainType_Field")
		return DomainType_Field
	} else if domainTypeString == domainTypeString_Fabric_Internet {
		nputil.TraceInfoEnd("DomainType_Fabric_Internet")
		return DomainType_Fabric_Internet
	} else if domainTypeString == domainTypeString_Fabric_Mpls {
		nputil.TraceInfoEnd("DomainType_Fabric_MPLS")
		return DomainType_Fabric_MPLS
	} else if domainTypeString == domainTypeString_Fabric_Otn {
		nputil.TraceInfoEnd("DomainType_Fabric_OTN")
		return DomainType_Fabric_OTN
	} else if domainTypeString == domainTypeString_Fabric_Sdwan {
		nputil.TraceInfoEnd("DomainType_Fabric_SDWAN")
		return DomainType_Fabric_SDWAN
	}

	nputil.TraceInfoEnd("DomainType_Invalid")
	return DomainType_Invalid
}

/* Add domainvLink topo from Schedule Cache */
func DomainLinkTopoAddFromScache(topoContents map[string][]byte) {
	nputil.TraceInfoBegin("------------------------------------------------------")
	if topoContents == nil {
		return
	}

	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint

	for filedName, topoMsg := range topoContents {
		domainTopoCache := new(ncsnp.DomainTopoCacheNotify)
		err := proto.Unmarshal(topoMsg, domainTopoCache)
		if err != nil {
			nputil.TraceError(err)
			return
		}
		infoString := fmt.Sprintf("Domain(%s)'s domainTopoCache is:(%+v)", filedName, *domainTopoCache)
		nputil.TraceInfo(infoString)

		//Add domain in baseDomainGraph and graph tree
		baseDomain := baseDomainGraph.BaseDomainFindById(domainTopoCache.LocalDomainId)
		if baseDomain == nil {
			domainType := String2DomainType(domainTypeString_Field)
			newBaseDomain := baseDomainGraph.BaseDomainAdd(domainTopoCache.LocalDomainId, domainTopoCache.LocalDomainName, domainType)
			if err != nil {
				rtnErr := errors.New("BaseDomainAdd error")
				nputil.TraceErrorWithStack(rtnErr)
				return
			}
			baseDomain = newBaseDomain
			for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
				graph := v.(*Graph)
				_, _ = graph.DomainAdd(domainTopoCache.LocalDomainId)
			}
		}
		// Add domainlink in baseDomainGraph and graph
		if baseDomain != nil {
			_ = baseDomain.BaseDomainLinkAddFromCache(*domainTopoCache)
		}
	}
	nputil.TraceInfoEnd("------------------------------------------------------")
}

/* Add domainvLink topo from Schedule Cache */
func DomainLinkAddForScache(domainTopoCache ncsnp.DomainTopoCacheNotify) {
	nputil.TraceInfoBegin("------------------------------------------------------")

	infoString := fmt.Sprintf("domainTopoCache(%+v)", domainTopoCache)
	nputil.TraceInfo(infoString)

	//Add domain in baseDomainGraph and graph tree
	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint
	DomainAddForAllGraph(domainTopoCache.LocalDomainId, domainTopoCache.LocalDomainName, domainTypeString_Field)

	// Add domainlink in baseDomainGraph and graph
	baseDomain := baseDomainGraph.BaseDomainFindById(domainTopoCache.LocalDomainId)
	if baseDomain != nil {
		_ = baseDomain.BaseDomainLinkAddFromCache(domainTopoCache)
	}
	nputil.TraceInfoEnd("------------------------------------------------------")
}

func buildSpfGraphEdge() {
	nputil.TraceInfoBegin("------------------------------------------------------")

	local := GetCoreLocal()
	for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
		graph := v.(*Graph)
		domainGraph := graph.DomainGraphPoint
		_ = domainGraph.SpfGraphEdgeCreateForSla()

		for i, domainWeightedEdge := range domainGraph.DomainWeightedEdgeArry {
			infoString := fmt.Sprintf("domainGraph.DomainWeightedEdge[%d] is (%+v).\n", i, domainWeightedEdge)
			nputil.TraceInfo(infoString)
		}
	}
	nputil.TraceInfoEnd("------------------------------------------------------")
}

func NetworkFilter(rbs []*v1alpha1.ResourceBinding, networkReq *v1alpha1.NetworkRequirement, networkInfoMap map[string]clusterapi.Topo) []*v1alpha1.ResourceBinding {

	logx.NewLogger()
	Register()
	nputil.TraceInfoBegin("------------------------------------------------------")

	//1. get network TopoInfo from schedule cache
	var topoContents map[string][]byte
	topoContents = make(map[string][]byte)
	for _, topoInfo := range networkInfoMap {
		infoString := fmt.Sprintf("Field(%s)'s topoContents string is:(%+v)", topoInfo.Field, topoInfo.Content)
		nputil.TraceInfo(infoString)
		byteArray, _ := base64.StdEncoding.DecodeString(topoInfo.Content)
		topoContents[topoInfo.Field] = byteArray
		infoString = fmt.Sprintf("Field(%s)'s topoContents byteArray is:(%+v)", topoInfo.Field, topoContents[topoInfo.Field])
		nputil.TraceInfo(infoString)
	}
	//Add DomainLink topo from cache
	DomainLinkTopoAddFromScache(topoContents)
	//Build KSP Spf edge for KSP graph
	buildSpfGraphEdge()
	//Build DomainLinkKspGraph for all Graph
	BuildDomainLinkKspGraphAll()
	//Filter and select domainPath for resourceBindings
	rbsSelected := networkFilterForRbs(rbs, networkReq)

	nputil.TraceInfoEnd("------------------------------------------------------")
	return rbsSelected
}
