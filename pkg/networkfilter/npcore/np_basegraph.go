package npcore

import (
	"errors"
	"fmt"

	ncsnp "github.com/lmxia/gaia/pkg/networkfilter/model"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
	"github.com/timtadh/data-structures/tree/avl"
	"github.com/timtadh/data-structures/types"
)

/***********************************************************************************************************************/
/*********************************************data structure*******************************************************************/
/***********************************************************************************************************************/

type BaseGraph struct {
	BaseDomainGraphPoint *BaseDomainGraph
}

/************************** Domain **************************************/
type BaseDomainGraph struct {
	BaseDomainTree    avl.AvlTree // 树节点结构BaseDomain
	BaseDomainDbArray []BaseDomainDb
}

type DomainKey struct {
	DomainId uint32 `json:"domainId" groups:"db"`
}

type BaseDomain struct {
	BaseDomainDbV        BaseDomainDb
	BaseDomainGraphPoint *BaseDomainGraph

	BaseDomainLinkTree    avl.AvlTree // 树节点结构BaseDomainLink
	BaseDomainLinkDbArray []BaseDomainLinkDb
}

type DomainType uint32

type BaseDomainDb struct {
	Key        DomainKey  `json:"key" groups:"db"`
	Type       DomainType `json:"type" groups:"db"`
	DomainName string     `json:"domainName" groups:"db"`
	IspName    string     `json:"ispName" groups:"db"`
}

type BaseDomainLink struct {
	BaseDomainLinkDbV BaseDomainLinkDb
	BaseDomainPoint   *BaseDomain
}

type BaseDomainLinkDb struct {
	Key               DomainLinkKey `json:"key" groups:"db"`
	IntfName          string        `json:"intfName" groups:"db"` // link使用的物理口信息，用于计算带宽占用等
	Frequency         uint32        `json:"frequency" groups:"db"`
	Delay             uint32        `json:"delay" groups:"db"`
	HistoryDelayArray []uint32      `json:"historyDelayArray" groups:"db"`
	PeerSN            string        `json:"peerSN" groups:"db"`
	Sla               DomainLinkSla
}

type DomainLinkSla struct {
	DelayValue          uint32 `json:"delayValue" groups:"db"`
	LostValue           uint32 `json:"lostValue" groups:"db"`
	JitterValue         uint32 `json:"jitterValue" groups:"db"`
	ThroughputValue     uint64 `json:"throughputValue" groups:"db"`
	FreeThroughputValue uint64 `json:"freeThroughputValue" groups:"db"`
}

type DomainLinkKey struct {
	SrcDomainId    uint32 `json:"srcDomainId" groups:"db"`
	DstDomainId    uint32 `json:"dstDomainId" groups:"db"`
	SrcNodeSN      string `json:"srcNodeSN" groups:"db"`
	DstNodeSN      string `json:"dstNodeSN" groups:"db"`
	AttachId       uint64 `json:"attachId" groups:"db"`
	AttachDomainId uint64 `json:"attachDomainId" groups:"db"`
}

/***********************************************************************************************************************/
/*********************************************global variable*******************************************************************/
/***********************************************************************************************************************/
/***********************************************************************************************************************/
func (domainKey DomainKey) Equals(other types.Equatable) bool {
	if o, ok := other.(DomainKey); ok {
		if domainKey.DomainId != o.DomainId {
			return false
		}
	}
	return true
}

func (domainKey DomainKey) Less(other types.Sortable) bool {
	if o, ok := other.(DomainKey); ok {
		if domainKey.DomainId < o.DomainId {
			return true
		}
	}
	return false
}

func (domainKey DomainKey) Hash() int {
	return int(domainKey.DomainId)
}

func (domainLinkKey DomainLinkKey) Equals(other types.Equatable) bool {
	if o, ok := other.(DomainLinkKey); ok {
		if domainLinkKey.SrcDomainId != o.SrcDomainId {
			return false
		}

		if domainLinkKey.DstDomainId != o.DstDomainId {
			return false
		}

		if domainLinkKey.SrcNodeSN != o.SrcNodeSN {
			return false
		}

		if domainLinkKey.DstNodeSN != o.DstNodeSN {
			return false
		}

		if domainLinkKey.AttachId != o.AttachId {
			return false
		}

		if domainLinkKey.AttachDomainId != o.AttachDomainId {
			return false
		}

	}
	return true
}

func (domainLinkKey DomainLinkKey) Less(other types.Sortable) bool {
	if o, ok := other.(DomainLinkKey); ok {
		if domainLinkKey.AttachId < o.AttachId {
			return true
		} else if domainLinkKey.AttachId > o.AttachId {
			return false
		}

		if domainLinkKey.AttachDomainId < o.AttachDomainId {
			return true
		} else if domainLinkKey.AttachDomainId > o.AttachDomainId {
			return false
		}

		if domainLinkKey.SrcDomainId < o.SrcDomainId {
			return true
		} else if domainLinkKey.SrcDomainId > o.SrcDomainId {
			return false
		}

		if domainLinkKey.DstDomainId < o.DstDomainId {
			return true
		} else if domainLinkKey.DstDomainId > o.DstDomainId {
			return false
		}

		if domainLinkKey.SrcNodeSN < o.SrcNodeSN {
			return true
		} else if domainLinkKey.SrcNodeSN > o.SrcNodeSN {
			return false
		}

		if domainLinkKey.DstNodeSN < o.DstNodeSN {
			return true
		} else if domainLinkKey.DstNodeSN > o.DstNodeSN {
			return false
		}

	}
	return false
}

func (domainLinkKey DomainLinkKey) Hash() int {
	return int(domainLinkKey.AttachId) + int(domainLinkKey.AttachDomainId)
}

/**********************************************************************************************/
/******************************************* API ********************************************/
/**********************************************************************************************/

func baseDomainCreateById(domainId uint32, domainName string, ispName string, domainType DomainType, baseDomainGraph *BaseDomainGraph) *BaseDomain {
	nputil.TraceInfoBegin("")

	baseDomain := new(BaseDomain)
	baseDomain.BaseDomainDbV.Key.DomainId = domainId

	baseDomain.BaseDomainDbV.Type = domainType
	baseDomain.BaseDomainDbV.DomainName = domainName
	baseDomain.BaseDomainDbV.IspName = ispName
	baseDomain.BaseDomainGraphPoint = baseDomainGraph
	nputil.TraceInfoEnd("")
	return baseDomain
}

func (baseDomainGraph *BaseDomainGraph) BaseDomainFindById(domainId uint32) *BaseDomain {
	nputil.TraceInfoBegin("")

	var findKey DomainKey
	findKey.DomainId = domainId

	val, _ := baseDomainGraph.BaseDomainTree.Get(findKey)
	if val == nil {
		nputil.TraceInfoEnd("basedomain not find")
		return nil
	}

	nputil.TraceInfoEnd("basedomain find")
	return val.(*BaseDomain)
}

func (baseGraph *BaseGraph) BaseDomainAdd(domainId uint32, domainName string, ispName string, domainType DomainType) *BaseDomain {
	nputil.TraceInfoBegin("")

	baseDomainGraph := baseGraph.BaseDomainGraphPoint

	addDomain := baseDomainGraph.BaseDomainAdd(domainId, domainName, ispName, domainType)

	nputil.TraceInfoEnd("")
	return addDomain
}

func (baseDomainGraph *BaseDomainGraph) BaseDomainAdd(domainId uint32, domainName string, ispName string, domainType DomainType) *BaseDomain {
	nputil.TraceInfoBegin("")

	// basegraph 中添加 domain
	findDomain := baseDomainGraph.BaseDomainFindById(domainId)
	if findDomain != nil {
		infoString := fmt.Sprintf("domainId(%d) has existed", domainId)
		nputil.TraceInfoEnd(infoString)
		return findDomain
	}

	addDomain := baseDomainCreateById(domainId, domainName, ispName, domainType, baseDomainGraph)
	// 加入树中
	err := baseDomainGraph.BaseDomainTree.Put(addDomain.BaseDomainDbV.Key, addDomain)
	if err != nil {
		rtnErr := errors.New("BaseDomainTree put error")
		nputil.TraceErrorWithStack(rtnErr)
		return addDomain
	}

	// 更新BaseDomainDbArray
	baseDomainGraph.BaseDomainDbArray = []BaseDomainDb{}
	for _, v, next := baseDomainGraph.BaseDomainTree.Iterate()(); next != nil; _, v, next = next() {
		tmpBaseDomain := v.(*BaseDomain)
		baseDomainGraph.BaseDomainDbArray = append(baseDomainGraph.BaseDomainDbArray, tmpBaseDomain.BaseDomainDbV)
	}

	nputil.TraceInfoEnd("")
	return addDomain
}

func (baseDomainGraph *BaseDomainGraph) BaseDomainDelete(domainId uint32) error {
	nputil.TraceInfoBegin("")

	BaseDomain := baseDomainGraph.BaseDomainFindById(domainId)
	if BaseDomain == nil {
		rtnErr := errors.New("baseDomain does not exist")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	// 删掉domain的domainlink
	_ = BaseDomain.BaseDomainlinkAllDelete()

	// 摘除树中
	_, err := baseDomainGraph.BaseDomainTree.Remove(BaseDomain.BaseDomainDbV.Key)
	if err != nil {
		rtnErr := errors.New("BaseDomainTree remove error")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}
	nputil.TraceInfoEnd("")
	return nil
}

func DomainAddForAllGraph(domainID uint32, domainName string, ispName string, domainTypeString string) {
	nputil.TraceInfoBegin("------------------------------------------------------")

	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint

	// Add domain in baseDomainGraph and graph tree
	baseDomain := baseDomainGraph.BaseDomainFindById(domainID)
	if baseDomain == nil {
		domainType := String2DomainType(domainTypeString)

		newBaseDomain := baseDomainGraph.BaseDomainAdd(domainID, domainName, ispName, domainType)
		if newBaseDomain == nil {
			rtnErr := errors.New("DomainAddForAllGraph error")
			nputil.TraceErrorWithStack(rtnErr)
			return
		}
		for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
			graph := v.(*Graph)
			_, _ = graph.DomainAdd(domainID)
		}
	}
	nputil.TraceInfoEnd("------------------------------------------------------")
}

func (baseDomain *BaseDomain) BaseDomainLinkFindByKey(domainLinkKey DomainLinkKey) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	infoString := fmt.Sprintf("baseDomain is (%+v), DomainLinkKey is (%+v)!", baseDomain, domainLinkKey)
	nputil.TraceInfo(infoString)
	val, _ := baseDomain.BaseDomainLinkTree.Get(domainLinkKey)
	if val == nil {
		nputil.TraceInfoEnd("BaseDomainLinkTree not find")
		return nil
	}

	nputil.TraceInfoEnd("BaseDomainLinkTree find")
	return val.(*BaseDomainLink)
}

func (baseDomainGraph *BaseDomainGraph) GetReverseBaseDomainLink(domainLinkKey DomainLinkKey) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	reverseDomain := baseDomainGraph.BaseDomainFindById(domainLinkKey.DstDomainId)
	if reverseDomain == nil {
		nputil.TraceErrorStringWithStack("reverseDomain is nil")
		return nil
	}

	var reverseDomainLinkKey DomainLinkKey
	reverseDomainLinkKey.SrcDomainId = domainLinkKey.DstDomainId
	reverseDomainLinkKey.SrcNodeSN = domainLinkKey.DstNodeSN
	reverseDomainLinkKey.DstDomainId = domainLinkKey.SrcDomainId
	reverseDomainLinkKey.DstNodeSN = domainLinkKey.SrcNodeSN
	reverseDomainLinkKey.AttachId = domainLinkKey.AttachId
	reverseDomainLinkKey.AttachDomainId = domainLinkKey.AttachDomainId

	infoString := fmt.Sprintf("domainLinkKey(%+v) , reverseDomainLinkKey(%+v)", domainLinkKey, reverseDomainLinkKey)
	nputil.TraceInfo(infoString)

	reverseDomainLink := reverseDomain.BaseDomainLinkFindByKey(reverseDomainLinkKey)
	if reverseDomainLink == nil {
		nputil.TraceErrorStringWithStack("reverseDomainLink is nil")
		return nil
	}

	nputil.TraceInfoEnd("")
	return reverseDomainLink
}

func DomainLinkKeyCreateFromCache(domainVlink *ncsnp.DomainVLink) *DomainLinkKey {
	nputil.TraceInfoBegin("")

	infoString := fmt.Sprintf("domainVlink is (%+v)!", domainVlink)
	nputil.TraceInfoAlwaysPrint(infoString)
	domainLinkKey := new(DomainLinkKey)
	domainLinkKey.SrcDomainId = domainVlink.LocalDomainId
	domainLinkKey.SrcNodeSN = domainVlink.LocalNodeSN
	domainLinkKey.DstDomainId = domainVlink.RemoteDomainId
	domainLinkKey.DstNodeSN = domainVlink.RemoteNodeSN
	domainLinkKey.AttachId = 0
	domainLinkKey.AttachDomainId = domainVlink.AttachDomainId

	nputil.TraceInfoEnd("")
	return domainLinkKey
}

func BaseDomainLinkCreateByKey(domainLinkKey DomainLinkKey, domainVlink ncsnp.DomainVLink, baseDomain *BaseDomain) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	baseDomainLink := new(BaseDomainLink)

	baseDomainLink.BaseDomainLinkDbV.Key = domainLinkKey
	baseDomainLink.BaseDomainLinkDbV.IntfName = domainVlink.LocalInterface
	baseDomainLink.BaseDomainLinkDbV.Delay = domainVlink.VLinkSlaAttr.Delay
	sla := new(DomainLinkSla)
	sla.LostValue = domainVlink.VLinkSlaAttr.Loss
	sla.DelayValue = domainVlink.VLinkSlaAttr.Delay
	sla.JitterValue = domainVlink.VLinkSlaAttr.Jitter
	sla.ThroughputValue = domainVlink.VLinkSlaAttr.Bandwidth
	sla.FreeThroughputValue = domainVlink.VLinkSlaAttr.FreeBandwidth
	baseDomainLink.BaseDomainLinkDbV.Sla = *sla
	baseDomainLink.BaseDomainPoint = baseDomain

	nputil.TraceInfoEnd("")
	return baseDomainLink
}

func GetDomainNameByDomainId(domainId uint32) string {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint

	baseDomain := baseDomainGraph.BaseDomainFindById(domainId)
	if baseDomain == nil {
		infoString := fmt.Sprintf("The domain is nil and domainId is (%d)!\n", domainId)
		nputil.TraceInfoEnd(infoString)
		nputil.TraceErrorString("baseDomain is nil.")
		return ""
	}

	nputil.TraceInfoEnd("")
	return baseDomain.BaseDomainDbV.DomainName
}
func GetIspNameByDomainId(domainId uint32) string {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint

	baseDomain := baseDomainGraph.BaseDomainFindById(domainId)
	if baseDomain == nil {
		infoString := fmt.Sprintf("The domain is nil and domainId is (%d)!\n", domainId)
		nputil.TraceInfoEnd(infoString)
		nputil.TraceErrorString("baseDomain is nil.")
		return ""
	}

	nputil.TraceInfoEnd("")
	return baseDomain.BaseDomainDbV.IspName
}

func DomainVLinkCreateByBaseDomainLink(baseDomainLink *BaseDomainLink) *ncsnp.DomainVLink {
	nputil.TraceInfoBegin("")

	pbDomainVLink := new(ncsnp.DomainVLink)

	if baseDomainLink != nil {

		baseDomainLinkKey := baseDomainLink.BaseDomainLinkDbV.Key
		pbDomainVLink.LocalDomainName = GetDomainNameByDomainId(baseDomainLinkKey.SrcDomainId)
		pbDomainVLink.LocalDomainId = baseDomainLinkKey.SrcDomainId
		pbDomainVLink.RemoteDomainName = GetDomainNameByDomainId(baseDomainLinkKey.DstDomainId)
		pbDomainVLink.RemoteDomainId = baseDomainLinkKey.DstDomainId

		pbDomainVLink.LocalNodeSN = baseDomainLinkKey.SrcNodeSN
		pbDomainVLink.RemoteNodeSN = baseDomainLinkKey.DstNodeSN
		pbDomainVLink.LocalInterface = baseDomainLink.BaseDomainLinkDbV.IntfName
		pbDomainVLink.AttachDomainId = baseDomainLinkKey.AttachDomainId

		pbVLinkSla := new(ncsnp.VLinkSla)
		pbVLinkSla.Delay = baseDomainLink.BaseDomainLinkDbV.Sla.DelayValue
		pbVLinkSla.Jitter = baseDomainLink.BaseDomainLinkDbV.Sla.JitterValue
		pbVLinkSla.Loss = baseDomainLink.BaseDomainLinkDbV.Sla.LostValue
		pbVLinkSla.Bandwidth = baseDomainLink.BaseDomainLinkDbV.Sla.ThroughputValue
		pbVLinkSla.FreeBandwidth = baseDomainLink.BaseDomainLinkDbV.Sla.FreeThroughputValue
		pbDomainVLink.VLinkSlaAttr = pbVLinkSla
	}

	nputil.TraceInfoEnd("")
	return pbDomainVLink
}

func (baseDomainLink *BaseDomainLink) UpdateBaseDomainLinkFreeBandwidth(requireBandwidth uint64) bool {
	nputil.TraceInfoBegin("")

	if baseDomainLink.BaseDomainLinkDbV.Sla.FreeThroughputValue < requireBandwidth {
		infoString := fmt.Sprintf("No free bandwidth left: baseDomainLink.BaseDomainLinkDbV.Sla.(%+v) is lower than requireBandwidth(%d).",
			baseDomainLink.BaseDomainLinkDbV.Sla, requireBandwidth)
		nputil.TraceErrorString(infoString)
		return false
	}
	// Update basedomainlink free bandwidth
	baseDomainLink.BaseDomainLinkDbV.Sla.FreeThroughputValue = baseDomainLink.BaseDomainLinkDbV.Sla.FreeThroughputValue - requireBandwidth

	nputil.TraceInfoEnd("")
	return true
}

// Add Basedomainlink and domainlink in graph
func (baseDomain *BaseDomain) BaseDomainLinkAdd(domainLinkKey DomainLinkKey, domainVlink ncsnp.DomainVLink) error {
	nputil.TraceInfoBegin("")

	// 先查找
	findBaseDomainLink := baseDomain.BaseDomainLinkFindByKey(domainLinkKey)
	if findBaseDomainLink != nil {
		infoString := fmt.Sprintf("The domainlink(%+v) has existed", domainLinkKey)
		nputil.TraceInfoEnd(infoString)
		return nil
	}

	// Add Basedomainlink in BaseGraph
	baseDomainLink := BaseDomainLinkCreateByKey(domainLinkKey, domainVlink, baseDomain)
	err := baseDomain.BaseDomainLinkTree.Put(baseDomainLink.BaseDomainLinkDbV.Key, baseDomainLink)
	if err != nil {
		rtnErr := errors.New("baseDomainLinkTree put error")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	// Add domainlink in graph
	local := GetCoreLocal()
	for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
		graph := v.(*Graph)
		domainGraph := graph.DomainGraphPoint
		_, _ = domainGraph.DomainLinkAddByKey(baseDomainLink.BaseDomainLinkDbV.Key)
	}

	nputil.TraceInfoEnd("")
	return nil
}

func (baseDomain *BaseDomain) BaseDomainlinkAllDelete() error {
	nputil.TraceInfoBegin("")

	// 释放domainlink
	for _, v, next := baseDomain.BaseDomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		baseDomainLink := v.(*BaseDomainLink)
		err := baseDomain.BaseDomainlinkDelete(baseDomainLink.BaseDomainLinkDbV.Key)
		if err != nil {
			rtnErr := errors.New("baseDomainlinkAllDelete is failed")
			nputil.TraceErrorWithStack(rtnErr)
			return rtnErr
		}

	}
	nputil.TraceInfoEnd("")
	return nil
}

func (baseDomain *BaseDomain) BaseDomainlinkDelete(domainLinkKey DomainLinkKey) error {
	nputil.TraceInfoBegin("")

	// Find domainlink
	findBaseDomainLink := baseDomain.BaseDomainLinkFindByKey(domainLinkKey)
	if findBaseDomainLink == nil {
		rtnErr := errors.New("baseDomainLink doesn't exist")
		nputil.TraceErrorWithStack(rtnErr)
		return rtnErr
	}

	// Delete domainlink in basegraph
	_, err := baseDomain.BaseDomainLinkTree.Remove(domainLinkKey)
	if err != nil {
		rtnErr := errors.New("BaseDomainLinkTree remove error")
		nputil.TraceErrorWithStack(rtnErr)
		return nil
	}
	nputil.TraceInfoEnd("")
	return nil
}

func DomainLinkKeyCreate(srcDomainId uint32, SrcNodeSN string, dstDomainId uint32, dstNodeSN string, attachId uint64, attachDomainId uint64) *DomainLinkKey {
	nputil.TraceInfoBegin("")

	domainLinkKey := new(DomainLinkKey)
	domainLinkKey.SrcDomainId = srcDomainId
	domainLinkKey.SrcNodeSN = SrcNodeSN
	domainLinkKey.DstDomainId = dstDomainId
	domainLinkKey.DstNodeSN = dstNodeSN
	domainLinkKey.AttachId = attachId
	domainLinkKey.AttachDomainId = attachDomainId

	nputil.TraceInfoEnd("")
	return domainLinkKey
}

func BaseDomainLinkGetBySrc(domainId uint32, nodeSN string, attachId uint64) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	coreLocal := GetCoreLocal()
	baseDomainGraph := coreLocal.BaseGraphPoint.BaseDomainGraphPoint

	baseDomain := baseDomainGraph.BaseDomainFindById(domainId)
	if baseDomain == nil {
		infoString := fmt.Sprintf("domainId(%d) is nil", domainId)
		nputil.TraceErrorStringWithStack(infoString)
		return nil
	}

	// 遍历BaseDomain下所有的 BaseDomainLink，找源node 和 attachid相同的 baseDomainLink
	for _, v, next := baseDomain.BaseDomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		tmpBaseDomainLink := v.(*BaseDomainLink)
		if tmpBaseDomainLink.BaseDomainLinkDbV.Key.SrcNodeSN == nodeSN && tmpBaseDomainLink.BaseDomainLinkDbV.Key.AttachId == attachId {
			nputil.TraceInfoEnd("find")
			return tmpBaseDomainLink
		}
	}

	nputil.TraceInfoEnd("not find")
	return nil
}

func DomainLinkCostGetByDomainLinkKey(domainLinkKey DomainLinkKey) uint32 {
	nputil.TraceInfoBegin("")

	baseDomainGraph := GetCoreLocal().BaseGraphPoint.BaseDomainGraphPoint
	baseDomain := baseDomainGraph.BaseDomainFindById(domainLinkKey.SrcDomainId)
	if baseDomain != nil {
		baseDomainLink := baseDomain.BaseDomainLinkFindByKey(domainLinkKey)
		if baseDomainLink != nil {
			nputil.TraceInfoEnd("")
			return baseDomainLink.BaseDomainLinkDbV.Sla.DelayValue
		}
	}

	nputil.TraceErrorStringWithStack("cost is 0")
	return 0
}

func BaseDomainLinkFindByDstDomainId(srcDomainId uint32, dstDomainId uint32) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	baseDomainGraph := GetCoreLocal().BaseGraphPoint.BaseDomainGraphPoint

	srcBaseDomain := baseDomainGraph.BaseDomainFindById(srcDomainId)
	if srcBaseDomain == nil {
		infoString := fmt.Sprintf("srcBaseDomain(%d) is nil", srcDomainId)
		nputil.TraceErrorStringWithStack(infoString)
		return nil
	}

	for _, v, next := srcBaseDomain.BaseDomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
		tmpBaseDomainLink := v.(*BaseDomainLink)
		if tmpBaseDomainLink.BaseDomainLinkDbV.Key.DstDomainId == dstDomainId {
			nputil.TraceInfoEnd("find")
			return tmpBaseDomainLink
		}
	}

	nputil.TraceInfoEnd("not find")
	return nil
}

func BaseDomainLinkFindByKeyWithoutBaseDomain(domainLinkKey DomainLinkKey) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	baseDomainGraph := GetCoreLocal().BaseGraphPoint.BaseDomainGraphPoint

	srcBaseDomain := baseDomainGraph.BaseDomainFindById(domainLinkKey.SrcDomainId)
	if srcBaseDomain == nil {
		nputil.TraceErrorStringWithStack("srcBaseDomain is nil")
		return nil
	}

	baseDomainLink := srcBaseDomain.BaseDomainLinkFindByKey(domainLinkKey)
	if baseDomainLink == nil {
		nputil.TraceInfoEnd("not find")
		return nil
	}

	nputil.TraceInfoEnd("find")
	return baseDomainLink
}

func (baseDomain *BaseDomain) BaseDomainLinkAddFromCache(domainTopoCache ncsnp.DomainTopoCacheNotify) error {
	nputil.TraceInfoBegin("")

	for i := 0; i < len(domainTopoCache.DomainVLinkArray); i++ {
		domainVlink := domainTopoCache.DomainVLinkArray[i]
		if domainVlink.AttachDomainId == 0 {
			infoString := fmt.Sprintf("The domainVink is not fabric type, domainVlink is (%+v).", domainVlink)
			nputil.TraceInfoEnd(infoString)
			continue
		}
		domainLinkKey := DomainLinkKeyCreateFromCache(domainVlink)

		// Add domainlink in baseDomainGraph and graph
		err := baseDomain.BaseDomainLinkAdd(*domainLinkKey, *domainVlink)
		if err != nil {
			rtnErr := errors.New("baseDomainLinkAddFromCache failed")
			nputil.TraceErrorWithStack(rtnErr)
			return rtnErr
		}

		// Add Fabric domain of the domainlink
		if domainVlink.AttachDomainName != "" {
			infoString := fmt.Sprintf("fabric domainId(%d) is , fabricName is (%s)", domainVlink.AttachDomainId, domainVlink.AttachDomainName)
			nputil.TraceInfoEnd(infoString)
			DomainAddForAllGraph(uint32(domainVlink.AttachDomainId), domainVlink.AttachDomainName, domainVlink.Isp, domainTypeString_Fabric_Internet)
		}
	}

	nputil.TraceInfoEnd("")
	return nil
}

func DomainResourceFreeForCache() {
	nputil.TraceInfoBegin("------------------------------------------------------")

	local := GetCoreLocal()
	baseDomainGraph := local.BaseGraphPoint.BaseDomainGraphPoint

	// Delete domainlink and domain in baseDomainGraph tree and graph tree
	for _, v, next := baseDomainGraph.BaseDomainTree.Iterate()(); next != nil; _, v, next = next() {
		baseDomain := v.(*BaseDomain)
		_ = baseDomainGraph.BaseDomainDelete(baseDomain.BaseDomainDbV.Key.DomainId)

		// Delete domainlink and domain in DomainGraph tree and graph tree
		for _, v, next := local.GraphTree.Iterate()(); next != nil; _, v, next = next() {
			graph := v.(*Graph)
			_ = graph.GraphDomainDelete(baseDomain.BaseDomainDbV.Key.DomainId)
		}
	}

	//_ = findDomain.BaseDomainlinkAllDelete()
	nputil.TraceInfoEnd("------------------------------------------------------")
}

func BaseDomainLinkFindByNodeSN(srcDomainId uint32, srcNodeSN string, dstDomainId uint32, dstNodeSN string) *BaseDomainLink {
	nputil.TraceInfoBegin("")

	baseDomainGraph := GetCoreLocal().BaseGraphPoint.BaseDomainGraphPoint

	srcDomain := baseDomainGraph.BaseDomainFindById(srcDomainId)
	if srcDomain != nil {
		for _, v, next := srcDomain.BaseDomainLinkTree.Iterate()(); next != nil; _, v, next = next() {
			tmpBaseDomainLink := v.(*BaseDomainLink)
			if tmpBaseDomainLink.BaseDomainLinkDbV.Key.SrcDomainId == srcDomainId &&
				tmpBaseDomainLink.BaseDomainLinkDbV.Key.SrcNodeSN == srcNodeSN &&
				tmpBaseDomainLink.BaseDomainLinkDbV.Key.DstDomainId == dstDomainId &&
				tmpBaseDomainLink.BaseDomainLinkDbV.Key.DstNodeSN == dstNodeSN {
				nputil.TraceInfoEnd("find")
				return tmpBaseDomainLink
			}
		}
	}

	nputil.TraceInfoEnd("not find")
	return nil
}
