package npcore

import (
	"errors"
	"fmt"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
	kspGraph "gonum.org/v1/gonum/graph"
	"gonum.org/v1/gonum/graph/simple"
	"math"

	"github.com/timtadh/data-structures/tree/avl"
	"github.com/timtadh/data-structures/types"
)

/***********************************************************************************************************************/
/*********************************************data structure*******************************************************************/
/***********************************************************************************************************************/

type Graph struct {
	GraphDbV         GraphDb
	DomainGraphPoint *DomainGraph
}

type GraphDb struct {
	Key GraphKey `json:"key" groups:"db"`
}

type GraphKey struct {
	GraphType uint64 `json:"graphType" groups:"db"`
}

type WeightedEdge struct {
	SrcSpfId int64
	DstSpfId int64
	Weight   float64
}

type DomainGraph struct {
	DomainTree      avl.AvlTree //树节点结构Domain
	DomainKeyArray  []DomainKey //这里只存KEY即可，domain的属性都保存在basegraph中，这里不需要重复保存
	SpfID2DomainKey []DomainKey //使用SPF lib库的临时数据结构，保存节点和lib库中节点编号的映射关系
	GraphPoint      *Graph      //父节点指针

	DomainKeyRevision     int64
	DomainLinkKeyRevision int64

	DomainEdgeArry         []WeightedEdge
	DomainWeightedEdgeArry []simple.WeightedEdge
	DomainLinkKspGraph     *DomainLinkKspGraph
}

type Domain struct {
	Key              DomainKey
	DomainGraphPoint *DomainGraph
	SpfLibID         int //使用SPF lib库的临时数据结构，保存节点当前在lib库中的序号

	DomainLinkTree     avl.AvlTree //树节点结构DomainLink
	DomainLinkKeyArray []DomainLinkKey
	//DomainLinkKeyRevision int64  //配置恢复外层需要使用，所以放在外层

}

type DomainLink struct {
	Key         DomainLinkKey
	DomainPoint *Domain
}

/***********************************************************************************************************************/
/*********************************************global variable*******************************************************************/
/***********************************************************************************************************************/
type GraphTypeAlgo byte

const (
	GraphTypeAlgo_Invalid GraphTypeAlgo = 0
	GraphTypeAlgo_Cspf    GraphTypeAlgo = 1
)

func (graphKey GraphKey) Equals(other types.Equatable) bool {
	if o, ok := other.(GraphKey); ok {
		if graphKey.GraphType != o.GraphType {
			return false
		}
	}
	return true
}

func (graphKey GraphKey) Less(other types.Sortable) bool {
	if o, ok := other.(GraphKey); ok {
		if graphKey.GraphType < o.GraphType {
			return true
		}
	}
	return false
}

func (graphKey GraphKey) Hash() int {
	return int(graphKey.GraphType)
}

/**********************************************************************************************/
/******************************************* API ********************************************/
/**********************************************************************************************/
func GetDomainLinkByDomainId(srcDomainId uint32, dstDomainId uint32, domainGraph *DomainGraph) *DomainLink {
	nputil.TraceInfoBegin("")

	srcDomain := domainGraph.DomainFindById(srcDomainId)
	if srcDomain == nil {
		nputil.TraceInfoEnd("srcDomain is nil")
		return nil
	}

	//遍历domain中domainlink，找满足条件的
	for _, v, next := srcDomain.DomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		tmpDomainLink := v.(*DomainLink)
		if tmpDomainLink.Key.SrcDomainId == srcDomainId &&
			tmpDomainLink.Key.DstDomainId == dstDomainId {
			nputil.TraceInfoEnd("find")
			return tmpDomainLink
		}
	}

	nputil.TraceInfoEnd("not find")
	return nil
}

func GetMiniDalyDomainLink(srcDomainId uint32, dstDomainId uint32, domainGraph *DomainGraph) (*DomainLink, uint32) {
	nputil.TraceInfoBegin("")

	infoString := fmt.Sprintf(" srcDomainId is (%+v), dstDomainId is ((%+v)", srcDomainId, dstDomainId)
	nputil.TraceInfo(infoString)
	srcDomain := domainGraph.DomainFindById(srcDomainId)
	if srcDomain == nil {
		nputil.TraceInfoEnd("srcDomain is nil")
		return nil, 0
	}
	var MinDomainlinkDelay uint32 = 0xffffffff
	domainLink := new(DomainLink)
	//遍历domain中domainlink，找满足条件的
	for _, v, next := srcDomain.DomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		tmpDomainLink := v.(*DomainLink)
		if tmpDomainLink.Key.SrcDomainId == srcDomainId &&
			tmpDomainLink.Key.DstDomainId == dstDomainId {
			tmDelay := DomainLinkCostGetByDomainLinkKey(tmpDomainLink.Key)
			if MinDomainlinkDelay > tmDelay {
				MinDomainlinkDelay = tmDelay
				domainLink = tmpDomainLink
			}
		}
	}

	infoString = fmt.Sprintf(" domainLink is (%+v), MinDomainlinkDelay is ((%+v)", domainLink, MinDomainlinkDelay)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return domainLink, MinDomainlinkDelay
}

func domainLinkCreateByKey(domainLinkKey DomainLinkKey, domain *Domain) *DomainLink {
	nputil.TraceInfoBegin("")

	domainLink := new(DomainLink)
	domainLink.Key = domainLinkKey

	domainLink.DomainPoint = domain

	nputil.TraceInfoEnd("")
	return domainLink
}

func GraphCreateByKey(graphTypeAlgo GraphTypeAlgo) *Graph {
	nputil.TraceInfoBegin("")

	if graphTypeAlgo == GraphTypeAlgo_Invalid {
		nputil.TraceInfoEnd("graphTypeAlgo invalid")
		return nil
	} else {
		graph := new(Graph)
		graph.GraphDbV.Key.GraphType = graphTypeCreate(graphTypeAlgo)
		nputil.TraceInfoEnd("")
		return graph
	}

}

func GraphFindByGraphType(graphTypeAlgo GraphTypeAlgo) *Graph {
	nputil.TraceInfoBegin("")

	var graphKey GraphKey
	graphKey.GraphType = uint64(graphTypeAlgo)

	local := GetCoreLocal()
	val, _ := local.GraphTree.Get(graphKey)
	if val == nil {
		nputil.TraceInfoEnd("GraphTree not find")
		return nil
	}

	nputil.TraceInfoEnd("find")
	return val.(*Graph)
}

func graphTypeCreate(graphTypeAlgo GraphTypeAlgo) uint64 {
	nputil.TraceInfoBegin("")

	var grahpType uint64

	//TBD: 前面6个预留字节先全部填0，后期看场景扩展
	grahpType = uint64(graphTypeAlgo)

	infoString := fmt.Sprintf("grahpType(%d)", grahpType)
	nputil.TraceInfoEnd(infoString)
	return grahpType
}

func (graph *Graph) DomainAdd(domainId uint32) (*Domain, error) {
	nputil.TraceInfoBegin("")

	domainGraph := graph.DomainGraphPoint

	addDomain, rtnErr := domainGraph.DomainAdd(domainId)

	nputil.TraceInfoEnd("")
	return addDomain, rtnErr
}

func domainCreateById(domainId uint32, domainGraph *DomainGraph) *Domain {
	nputil.TraceInfoBegin("")

	domain := new(Domain)
	domain.Key.DomainId = domainId

	domain.DomainGraphPoint = domainGraph

	nputil.TraceInfoEnd("")
	return domain
}

func getDomainIDbyName(domainName string) (uint32, bool) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint
	for _, v, next := baseDomainGraph.BaseDomainTree.Iterate()(); next != nil; _, v, next = next() {
		tmpBaseDomain := v.(*BaseDomain)
		if tmpBaseDomain.BaseDomainDbV.DomainName == domainName {
			return tmpBaseDomain.BaseDomainDbV.Key.DomainId, true
		}
	}
	nputil.TraceInfoEnd("")
	return 0, false
}

func getDomainSpfID(domainId uint32) (int, bool) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	//Get domainSpfID in graph by domainid
	for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
		graph := v.(*Graph)
		domainGraph := graph.DomainGraphPoint
		findDomain := domainGraph.DomainFindById(domainId)
		if findDomain == nil {
			infoString := fmt.Sprintf("domainId(%d) doesn't existed!", domainId)
			nputil.TraceInfoEnd(infoString)
			return 0, false
		} else {
			nputil.TraceInfoEnd("")
			return findDomain.SpfLibID, true
		}
	}
	nputil.TraceInfoEnd("")
	return 0, false
}

func (domainGraph *DomainGraph) DomainAdd(domainId uint32) (*Domain, error) {
	nputil.TraceInfoBegin("")

	//graph 中添加 domain
	findDomain := domainGraph.DomainFindById(domainId)
	if findDomain != nil {
		infoString := fmt.Sprintf("domainId(%d) has existed", domainId)
		nputil.TraceInfoEnd(infoString)
		return findDomain, nil
	}

	addDomain := domainCreateById(domainId, domainGraph)
	//加入树中
	err := domainGraph.DomainTree.Put(addDomain.Key, addDomain)
	if err != nil {
		rtnErr := errors.New("DomainTree put error")
		nputil.TraceErrorWithStack(rtnErr)
		return addDomain, rtnErr
	}
	domainGraph.DomainKeyArray = []DomainKey{}

	//更新DomainKeyArray
	for _, v, next := domainGraph.DomainTree.Iterate()(); next != nil; _, v, next = next() {
		tmpDomain := v.(*Domain)
		domainGraph.DomainKeyArray = append(domainGraph.DomainKeyArray, tmpDomain.Key)
	}
	nputil.TraceInfoEnd("")
	return addDomain, nil
}

func (domain *Domain) DomainLinkKeyArrayUpdate() error {
	nputil.TraceInfoBegin("")

	domain.DomainLinkKeyArray = []DomainLinkKey{}

	for _, v, next := domain.DomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		tmpDomainLink := v.(*DomainLink)
		domain.DomainLinkKeyArray = append(domain.DomainLinkKeyArray, tmpDomainLink.Key)
	}

	nputil.TraceInfoEnd("")
	return nil
}

func (domain *Domain) DomainLinkAddByKey(domainLinkKey DomainLinkKey) (*DomainLink, error) {
	nputil.TraceInfoBegin("")

	//先查找
	findDomainLink := domain.DomainLinkFindByKey(domainLinkKey)
	if findDomainLink != nil {
		return findDomainLink, nil
	}

	//1 创建baseNodeLink控制块
	domainLink := domainLinkCreateByKey(domainLinkKey, domain)
	infoString := fmt.Sprintf("domainLinkKey is (%+v), domainLink is (%+v)\n", domainLinkKey, domainLink)
	nputil.TraceInfo(infoString)
	//2 挂树，写DB
	err := domain.DomainLinkTree.Put(domainLink.Key, domainLink)
	if err != nil {
		rtnErr := errors.New("domainLinkTree put error")
		nputil.TraceErrorWithStack(rtnErr)
		return domainLink, nil
	}
	err = domain.DomainLinkKeyArrayUpdate()
	if err != nil {
		nputil.TraceError(err)
	}

	nputil.TraceInfoEnd("")
	return domainLink, nil
}

func (domain *Domain) DomainLinkFindByKey(domainLinkKey DomainLinkKey) *DomainLink {
	nputil.TraceInfoBegin("")

	val, _ := domain.DomainLinkTree.Get(domainLinkKey)
	if val == nil {
		nputil.TraceInfoEnd("DomainLink not find")
		return nil
	}

	nputil.TraceInfoEnd("DomainLink find")
	return val.(*DomainLink)
}

func (domain *Domain) DomainLinkDeleteByKey(domainLinkKey DomainLinkKey) error {
	nputil.TraceInfoBegin("")

	findDomainLink := domain.DomainLinkFindByKey(domainLinkKey)
	if findDomainLink == nil {
		rtnErr := errors.New("the domainlink does'nt exist")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	_, err := domain.DomainLinkTree.Remove(domainLinkKey)
	if err != nil {
		rtnErr := errors.New("remove DomainLink failed")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	err = domain.DomainLinkKeyArrayUpdate()
	if err != nil {
		nputil.TraceError(err)
	}
	nputil.TraceInfoEnd("")
	return nil
}

func (domain *Domain) DomainlinkAllDelete() error {
	nputil.TraceInfoBegin("")

	//释放domainlink
	for _, v, next := domain.DomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		baseDomainLink := v.(*DomainLink)
		err := domain.DomainLinkDeleteByKey(baseDomainLink.Key)
		if err != nil {
			rtnErr := errors.New("domainlinkAllDelete failed")
			nputil.TraceErrorWithStack(rtnErr)
			return rtnErr
		}
	}
	nputil.TraceInfoEnd("")
	return nil
}

func (graph *Graph) GraphDomainDelete(domainId uint32) error {
	nputil.TraceInfoBegin("")

	domainGraph := graph.DomainGraphPoint
	findDomain := domainGraph.DomainFindById(domainId)
	if findDomain == nil {
		rtnErr := errors.New("domain doesn't exist")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	//删掉domain的domainlink
	_ = findDomain.DomainlinkAllDelete()

	//摘除树中
	_, err := domainGraph.DomainTree.Remove(findDomain.Key)
	if err != nil {
		rtnErr := errors.New("BaseDomainTree remove error")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}
	nputil.TraceInfoEnd("")
	return nil
}

func (domainGraph *DomainGraph) DomainLinkAddByKey(domainLinkKey DomainLinkKey) (*DomainLink, error) {
	nputil.TraceInfoBegin("")

	domain := domainGraph.DomainFindById(domainLinkKey.SrcDomainId)
	if domain == nil {
		domain, _ = domainGraph.DomainAdd(domainLinkKey.SrcDomainId)
		if domain == nil {
			rtnErr := errors.New("domain add error")
			nputil.TraceErrorWithStack(rtnErr)
			return nil, rtnErr
		}
	}

	domainLink, rtnErr := domain.DomainLinkAddByKey(domainLinkKey)
	infoString := fmt.Sprintf("domain is (%+v), domainlink is  (%+v)", *domain, *domainLink)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return domainLink, rtnErr
}

func (domainGraph *DomainGraph) DomainLinkDeleteByKey(domainLinkKey DomainLinkKey) error {
	nputil.TraceInfoBegin("")

	domain := domainGraph.DomainFindById(domainLinkKey.SrcDomainId)
	if domain == nil {
		rtnErr := errors.New("the Domain doesn't exist")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	rtn := domain.DomainLinkDeleteByKey(domainLinkKey)

	nputil.TraceInfoEnd("")
	return rtn
}

func (domainGraph *DomainGraph) DomainFindById(domainId uint32) *Domain {
	nputil.TraceInfoBegin("")

	var findKey DomainKey
	findKey.DomainId = domainId

	val, _ := domainGraph.DomainTree.Get(findKey)
	if val == nil {
		nputil.TraceInfoEnd("Domain not find")
		return nil
	}

	nputil.TraceInfoEnd("Domain find")
	return val.(*Domain)
}

func (graph *Graph) AppConnect(domainLinkKspGraph *DomainLinkKspGraph, spfCalcMaxNum int, query simple.Edge, appConnectAttr AppConnectAttr) []DomainSrPath {
	nputil.TraceInfoBegin("")

	domainGraph := graph.DomainGraphPoint
	domain := domainGraph.DomainFindById(appConnectAttr.Key.SrcDomainId)
	domainSrPathArray := domain.SpfCalcDomainPathForAppConnect(domainLinkKspGraph, spfCalcMaxNum, query, appConnectAttr.SlaAttr)
	nputil.TraceInfoBegin("")

	return domainSrPathArray
}

func (domainGraph *DomainGraph) MapDomainKey2SpfID() error {
	nputil.TraceInfoBegin("")

	var libID = 0
	domainGraph.SpfID2DomainKey = []DomainKey{}
	for _, v, next := domainGraph.DomainTree.Iterate()(); next != nil; _, v, next = next() {
		tmpDomain := v.(*Domain)
		tmpDomain.SpfLibID = libID
		domainGraph.SpfID2DomainKey = append(domainGraph.SpfID2DomainKey, tmpDomain.Key)
		libID++
	}

	infoString := fmt.Sprintf("domainGraph.SpfID2DomainKey is (%+v)\n", domainGraph.SpfID2DomainKey)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return nil
}

func (domainGraph *DomainGraph) SpfGraphEdgeCreateForSla() error {
	nputil.TraceInfoBegin("")

	domainGraph.SpfID2DomainKey = []DomainKey{}

	//Map domain to spfID
	domainGraph.MapDomainKey2SpfID()

	//Build edge for domainGraph
	for _, v, next := domainGraph.DomainTree.Iterate()(); next != nil; _, v, next = next() {
		tmpDomain := v.(*Domain)
		for i := 0; i < len(tmpDomain.DomainLinkKeyArray); i++ {
			domainLinkKey := tmpDomain.DomainLinkKeyArray[i]
			infoString := fmt.Sprintf("domainLinkKey is (%+v)", domainLinkKey)
			nputil.TraceInfo(infoString)
			baseDomainLink := BaseDomainLinkFindByKeyWithoutBaseDomain(domainLinkKey)
			if baseDomainLink == nil {
				nputil.TraceErrorStringWithStack("baseDomainLink is nil")
				continue
			}
			err := domainGraph.SetGraphEdge(domainLinkKey)
			if err != nil {
				rtnErr := errors.New("SetGraphEdge failed")
				nputil.TraceErrorWithStack(rtnErr)
				continue
			}
		}
	}
	for i, domainEdge := range domainGraph.DomainEdgeArry {
		infoString := fmt.Sprintf("DomainEdge[%d] is (%+v)", i, domainEdge)
		nputil.TraceInfo(infoString)
	}
	//Build WeightedEdge of domainGraph to calc ksp
	for _, domainEdge := range domainGraph.DomainEdgeArry {
		edge := simple.WeightedEdge{
			F: simple.Node(domainEdge.SrcSpfId),
			T: simple.Node(domainEdge.DstSpfId),
			W: domainEdge.Weight,
		}
		domainGraph.DomainWeightedEdgeArry = append(domainGraph.DomainWeightedEdgeArry, edge)
	}
	for i, domainWeightedEdge := range domainGraph.DomainWeightedEdgeArry {
		infoString := fmt.Sprintf("domainWeightedEdge[%d] is (%+v)\n", i, domainWeightedEdge)
		nputil.TraceInfo(infoString)
	}
	nputil.TraceInfoEnd("")
	return nil
}

func (domainGraph *DomainGraph) SetGraphEdge(domainLinkKey DomainLinkKey) error {

	nputil.TraceInfoBegin("")

	infoString := fmt.Sprintf("domainLinkKey detail is :(%+v)", domainLinkKey)
	nputil.TraceInfo(infoString)
	srcDomain := domainGraph.DomainFindById(domainLinkKey.SrcDomainId)
	if srcDomain == nil {
		rtnErr := errors.New("error: srcDomain cannot be found")
		nputil.TraceInfo("Error: srcDomain cannot be found!")
		return rtnErr
	}
	dstDomain := domainGraph.DomainFindById(domainLinkKey.DstDomainId)
	if dstDomain == nil {
		rtnErr := errors.New("error: dstDomain cannot be found")
		nputil.TraceInfo("Error: dstDomain cannot be found!")
		return rtnErr
	}

	linkCost := DomainLinkCostGetByDomainLinkKey(domainLinkKey)

	var domainEdge WeightedEdge
	domainEdge.SrcSpfId = int64(srcDomain.SpfLibID)
	domainEdge.DstSpfId = int64(dstDomain.SpfLibID)
	domainEdge.Weight = float64(linkCost)

	findEdge := 0
	//两个domain之间可能会有多条domainlink,以weight最小domainlink作为两个domain之间的link
	for i, tmpDomainEdge := range domainGraph.DomainEdgeArry {
		if (domainEdge.SrcSpfId == tmpDomainEdge.SrcSpfId) && (domainEdge.DstSpfId == tmpDomainEdge.DstSpfId) {
			if domainEdge.Weight < tmpDomainEdge.Weight {
				domainGraph.DomainEdgeArry[i].Weight = domainEdge.Weight
			}
			findEdge++
		}
	}
	if findEdge == 0 {
		domainGraph.DomainEdgeArry = append(domainGraph.DomainEdgeArry, domainEdge)
	}

	nputil.TraceInfoEnd("")
	return nil
}

func (domainGraph *DomainGraph) CreateDomainLinkKspGraph() {
	nputil.TraceInfoBegin("")

	domainLinkKspGraph := NewDomainLinkKspGraph()
	domainLinkKspGraph.graph = func() kspGraph.WeightedEdgeAdder { return simple.NewWeightedDirectedGraph(0, math.Inf(1)) }
	domainLinkKspGraph.edges = domainGraph.DomainWeightedEdgeArry
	domainGraph.DomainLinkKspGraph = domainLinkKspGraph

	nputil.TraceInfoEnd("")
}

//Build DomainLinkKspGraph for all Graph
func BuildDomainLinkKspGraphAll() {
	nputil.TraceInfoBegin("")
	local := GetCoreLocal()
	for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
		graph := v.(*Graph)
		domainGraph := graph.DomainGraphPoint
		domainGraph.CreateDomainLinkKspGraph()
	}
	nputil.TraceInfoEnd("")
}

func (domainGraph *DomainGraph) DomainSrPathCreateByKspPath(path PathAttr) *DomainSrPath {
	nputil.TraceInfoBegin("")

	//baseDomainGraph := GetCoreLocal().BaseGraphPoint.BaseDomainGraphPoint
	domainSrPath := new(DomainSrPath)

	if len(path.PathIds) == 0 {
		nputil.TraceInfoEnd("No shortest path!")
		return nil
	}
	infoString := fmt.Sprintf("DomainSrPathCreateByKspPath path is (%+v)", path)
	nputil.TraceInfo(infoString)

	domainSrPath.DomainSidArray = make([]DomainSid, len(path.PathIds))
	for j := 0; j < len(path.PathIds); j++ {

		domainKey := domainGraph.SpfID2DomainKey[path.PathIds[j]]
		infoString := fmt.Sprintf(" DomainSrPathCreateByKspPath domainKey is : (%+v)", domainKey)
		nputil.TraceInfo(infoString)
		domainSrPath.DomainSidArray[j].DomainId = domainKey.DomainId
		domainSrPath.DomainSidArray[j].DomainType = String2DomainType(domainTypeString_Field)
	}
	for j := 0; j < len(path.PathIds); j++ {
		if j > 0 {
			srcDomainID := domainSrPath.DomainSidArray[j-1].DomainId
			destDomainID := domainSrPath.DomainSidArray[j].DomainId
			LastDomainLink, _ := GetMiniDalyDomainLink(srcDomainID, destDomainID, domainGraph)
			domainSrPath.DomainSidArray[j].DstNodeSN = LastDomainLink.Key.DstNodeSN
		}
		if j < (len(path.PathIds) - 1) {
			srcDomainID := domainSrPath.DomainSidArray[j].DomainId
			destDomainID := domainSrPath.DomainSidArray[j+1].DomainId
			NextDomainLink, _ := GetMiniDalyDomainLink(srcDomainID, destDomainID, domainGraph)
			domainSrPath.DomainSidArray[j].SrcNodeSN = NextDomainLink.Key.SrcNodeSN
			//baseDomain := baseDomainGraph.BaseDomainFindById(NextDomainLink.Key.SrcDomainId)
			//baseDomainLink := baseDomain.BaseDomainLinkFindByKey(NextDomainLink.Key)
			//domainVlink := DomainVLinkCreateByBaseDomainLink(baseDomainLink)
			//domainSrPath.DomainSidArray[j].DomainVlink = *domainVlink
		}
	}
	//等前面for循环，赋值好以后，单独处理头节点的Hub
	domainLink, _ := GetMiniDalyDomainLink(domainSrPath.DomainSidArray[0].DomainId, domainSrPath.DomainSidArray[1].DomainId, domainGraph)
	domainSrPath.DomainSidArray[0].SrcNodeSN = domainLink.Key.SrcNodeSN

	infoString = fmt.Sprintf("domainSrPath is %+v", domainSrPath)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return domainSrPath
}

func (domain *Domain) SpfCalcDomainPathForAppConnect(domainLinkKspGraph *DomainLinkKspGraph, spfCalcMaxNum int, query simple.Edge, appSlaAttr AppSlaAttr) []DomainSrPath {
	nputil.TraceInfoBegin("")

	domainGraph := domain.DomainGraphPoint
	bestPathGroups := KspCalcDomainPath(domainLinkKspGraph, spfCalcMaxNum, query)
	if len(bestPathGroups) == 0 {
		nputil.TraceInfoEnd("No shortest path!")
		retDomainSrPathArray := []DomainSrPath{}
		return retDomainSrPathArray
	}
	var domainSrPathArray []DomainSrPath
	for _, bestPath := range bestPathGroups {
		domainSrPath := domainGraph.DomainSrPathCreateByKspPath(bestPath)
		if domainSrPath.IsSatisfiedSla(appSlaAttr) == true {
			domainSrPathArray = append(domainSrPathArray, *domainSrPath)
		}
	}
	if len(domainSrPathArray) == 0 {
		infoString := fmt.Sprintf("Calc Domain path for AppConnect is not satisified.")
		nputil.TraceInfo(infoString)
	}
	infoString := fmt.Sprintf("SpfCalcDomainPathForAppConnect: domainSrPathArray is (%+v)!\n", domainSrPathArray)
	nputil.TraceInfo(infoString)
	nputil.TraceInfoEnd("")
	return domainSrPathArray
}

func (graph *Graph) GetDomainPathNameWithFaric(domainSrPath DomainSrPath) []DomainInfo {
	nputil.TraceInfoBegin("")

	domainGraph := graph.DomainGraphPoint

	infoString := fmt.Sprintf("domainSrPath is (%+v)", domainSrPath)
	nputil.TraceInfo(infoString)
	var domainSrNamePath []DomainInfo
	var domaininfo = DomainInfo{}
	//在同一个域内
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		domaininfo = DomainInfo{}
		filedName := GetDomainNameByDomainId(domainSrPath.DomainSidArray[j].DomainId)
		domaininfo.DomainName = filedName
		domaininfo.DomainID = domainSrPath.DomainSidArray[j].DomainId
		domaininfo.DomainType = uint32(String2DomainType(domainTypeString_Field))
		domainSrNamePath = append(domainSrNamePath, domaininfo)

		srcDomainID := domainSrPath.DomainSidArray[j].DomainId
		destDomainID := domainSrPath.DomainSidArray[j+1].DomainId
		LastDomainLink, _ := GetMiniDalyDomainLink(srcDomainID, destDomainID, domainGraph)
		if LastDomainLink.Key.AttachDomainId != 0 {
			fabricName := GetDomainNameByDomainId(uint32(LastDomainLink.Key.AttachDomainId))
			domaininfo.DomainName = fabricName
			domaininfo.DomainID = uint32(LastDomainLink.Key.AttachDomainId)
			domaininfo.DomainType = uint32(String2DomainType(domainTypeString_Fabric_Internet))
			domainSrNamePath = append(domainSrNamePath, domaininfo)
		}
	}
	//最后一个域或者同一个越
	lastFiledName := GetDomainNameByDomainId(domainSrPath.DomainSidArray[len(domainSrPath.DomainSidArray)-1].DomainId)
	domaininfo.DomainName = lastFiledName
	domaininfo.DomainID = domainSrPath.DomainSidArray[len(domainSrPath.DomainSidArray)-1].DomainId
	domaininfo.DomainType = uint32(String2DomainType(domainTypeString_Field))
	domainSrNamePath = append(domainSrNamePath, domaininfo)

	infoString = fmt.Sprintf("domainSrPathWithFabricArray is (%+v)\n", domainSrNamePath)
	nputil.TraceInfo(infoString)
	nputil.TraceInfoEnd("")
	return domainSrNamePath
}

func (graph *Graph) GetDomainPathNameArrayWithFaric(domainSrPathArray []DomainSrPath) []DomainSrNamePath {
	nputil.TraceInfoBegin("")

	var domainSrPathWithFabricArray []DomainSrNamePath
	domainGraph := graph.DomainGraphPoint
	for _, domainSrPath := range domainSrPathArray {
		infoString := fmt.Sprintf("domainSrPath.DomainSidArray is (%+v).", domainSrPath.DomainSidArray)
		nputil.TraceInfo(infoString)
		var domainSrNamePath = DomainSrNamePath{}
		for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
			filedName := GetDomainNameByDomainId(domainSrPath.DomainSidArray[j].DomainId)
			domainSrNamePath.DomainNameList = append(domainSrNamePath.DomainNameList, filedName)
			srcDomainID := domainSrPath.DomainSidArray[j].DomainId
			destDomainID := domainSrPath.DomainSidArray[j+1].DomainId
			LastDomainLink, _ := GetMiniDalyDomainLink(srcDomainID, destDomainID, domainGraph)
			if LastDomainLink.Key.AttachDomainId != 0 {
				fabricName := GetDomainNameByDomainId(uint32(LastDomainLink.Key.AttachDomainId))
				domainSrNamePath.DomainNameList = append(domainSrNamePath.DomainNameList, fabricName)
			}
		}
		lastFiledName := GetDomainNameByDomainId(domainSrPath.DomainSidArray[len(domainSrPath.DomainSidArray)-1].DomainId)
		domainSrNamePath.DomainNameList = append(domainSrNamePath.DomainNameList, lastFiledName)
		domainSrPathWithFabricArray = append(domainSrPathWithFabricArray, domainSrNamePath)
	}
	infoString := fmt.Sprintf("domainSrPathWithFabricArray is (%+v).", domainSrPathWithFabricArray)
	nputil.TraceInfo(infoString)
	nputil.TraceInfoEnd("")
	return domainSrPathWithFabricArray
}
