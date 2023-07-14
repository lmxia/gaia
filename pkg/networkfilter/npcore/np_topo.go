package npcore

import (
	"fmt"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
	"sort"
)

/***********************************************************************************************************************/
/*********************************************global variable***********************************************************/
/***********************************************************************************************************************/
const (
	Field_Domain_Inner_Delay = 4
)

/***********************************************************************************************************************/
/*********************************************data structure************************************************************/
/***********************************************************************************************************************/

type DomainSid struct {
	DomainId   uint32
	SrcNodeSN  string //作为link的src节点
	DstNodeSN  string //作为link的dst节点
	DomainType DomainType
	//DomainVlink ncsnp.DomainVLink //Domain的Delay最小的vlink
}

type DomainSrPath struct {
	DomainSidArray []DomainSid
}

//多条最短路径的多个domainSrPath
type DomainSrPathArray struct {
	DomainSrPathArray []DomainSrPath
}

//连接属性的特定实例的domainPathArray,多条路径
type AppDomainPathArray struct {
	AppConnect        AppConnectAttr //带实例号的AppConnectAttr
	DomainSrPathArray []DomainSrPath //AppConnect特定实例的多条可达的domainSrPath
}

//指定源连接属性的所有实例的domainPathGroup
type ScnIdAppDomainPathGroup struct {
	ScnIdInstance      ScnIdInstance        //特定源scnID实例
	AppDomainPathArray []AppDomainPathArray //特定源scnID实例的AppConnect的多路径domainSrPath(源实例相同，目的实例不相同组成的AppConnect)
}

//连接属性的特定实例的一条domainPath：单条路径
type AppDomainPath struct {
	AppConnect   AppConnectAttr //带实例号的AppConnectAttr
	DomainSrPath DomainSrPath   //特定AppConnect实例的一条domainPath
}

//连接属性的特定实例的domainNamePath:包含fabricName
type AppDomainNamePathArray struct {
	AppConnect          AppConnectAttr
	DomainNamePathArray []DomainSrNamePath //AppConnect特定实例的domainNamePathArray
}

type DomainSrNamePath struct {
	DomainNameList []string
}

/**********************************************************************************************/
/******************************************* API ********************************************/
/**********************************************************************************************/
func (domainSrPath *DomainSrPath) IsSatisfiedNetworkReq(appLinkAttr AppLinkAttr, reverseDelay uint32) bool {
	nputil.TraceInfoBegin("")

	var bCheckSla bool
	var bRttCheck bool
	var bProviderCheck bool

	//Check Sla
	if appLinkAttr.LinkSlaAttr.Mandatory == true {
		bCheckSla = domainSrPath.IsSatisfiedSla(appLinkAttr.LinkSlaAttr.SlaAttr)
		//如果设置的Mandatory属性
		if bCheckSla == false {
			nputil.TraceInfoEnd("False: Sla is not satisfied")
			return false
		}
	}
	//Check RTT
	if appLinkAttr.LinkRttAttr.Mandatory == true {
		bRttCheck = domainSrPath.IsSatisfiedRtt(reverseDelay, appLinkAttr.LinkRttAttr)
		//如果设置的Mandatory属性
		if bRttCheck == false {
			nputil.TraceInfoEnd("False: Sla is not satisfied")
			return false
		}
	}
	//Check Providers
	if appLinkAttr.Providers.Mandatory == true {
		bProviderCheck = domainSrPath.IsSatisfiedProvider(appLinkAttr.Providers)
		//如果设置的Mandatory属性
		if bProviderCheck == false {
			nputil.TraceInfoEnd("False: Provider is not satisfied")
			return false
		}
	}

	nputil.TraceInfoEnd("NetworkRequirment is satisfied, True")
	return true
}

func (domainSrPath *DomainSrPath) IsSatisfiedRtt(reverseDelay uint32, rttAttr AppLinkRttAttr) bool {
	nputil.TraceInfoBegin("")

	_, tmpDelay := domainSrPath.GetDomainSrPathDelay()
	//标识通信有rt指标要求
	if rttAttr.Rtt != 0 && (tmpDelay+reverseDelay) > (uint32)(rttAttr.Rtt) {
		nputil.TraceInfoEnd("False: Rtt is not satisfied")
		return false
	}
	nputil.TraceInfoEnd("RTT is satisfied, True")
	return true

}

func (domainSrPath *DomainSrPath) IsSatisfiedProvider(provider AppLinkProviderAttr) bool {
	nputil.TraceInfoBegin("")

	graph := GraphFindByGraphType(GraphTypeAlgo_Cspf)
	domainInfoPath := graph.GetDomainPathNameWithFaric(*domainSrPath)
	var domainNameList []string
	for _, domainInfo := range domainInfoPath {
		domainNameList = append(domainNameList, domainInfo.DomainName)
	}
	for _, proName := range provider.Providers {
		sort.Strings(domainNameList)
		index := sort.SearchStrings(domainNameList, proName)
		if !(index < len(domainNameList) && domainNameList[index] == proName) {
			nputil.TraceInfoEnd("False: Provider is not satisfied")
			return false
		}
	}

	nputil.TraceInfoEnd("Provider is satisfied, True")
	return true

}

func (domainSrPath *DomainSrPath) IsSatisfiedSla(appSlaAttr AppSlaAttr) bool {
	nputil.TraceInfoBegin("")

	//带宽是否满足SLA, 带宽为0说明不关注带宽属性
	if appSlaAttr.ThroughputValue != 0 {
		bCheck := domainSrPath.isSatisfiedThroughput(appSlaAttr)
		if bCheck == false {
			nputil.TraceInfoEnd("False: throughput is not satisfied")
			return false
		}
	}
	//时延是否满足SLA,因为NBI对外接口是int32值，所以输入最大值是0x7fffffff，最大值认为是不需要关注时延
	if appSlaAttr.DelayValue != 0 &&
		appSlaAttr.DelayValue < 0xffff {
		bCheck := domainSrPath.isSatisfiedDelay(appSlaAttr)
		if bCheck == false {
			nputil.TraceInfoEnd("False: delay is not satisfied")
			return false
		}
	}

	//丢包率是否满足SLA,100认为是最大值，不需要关注丢包率
	if appSlaAttr.LostValue != 0 &&
		appSlaAttr.LostValue < 100 {
		bCheck := domainSrPath.isSatisfiedLost(appSlaAttr)
		if bCheck == false {
			nputil.TraceInfoEnd("False: lostrate is not satisfied")
			return false
		}
	}

	//抖动是否满足SLA
	if appSlaAttr.JitterValue != 0 &&
		appSlaAttr.JitterValue < 0xffff {
		bCheck := domainSrPath.isSatisfiedJitter(appSlaAttr)
		if bCheck == false {
			nputil.TraceInfoEnd("False: jitter is not satisfied")
			return false
		}
	}

	nputil.TraceInfoEnd("Sla is satisfied, True")
	return true
}

//Check and update free bandwidth of baseDomainlink
func (domainSrPath *DomainSrPath) isSatisfiedThroughput(appSlaAttr AppSlaAttr) bool {
	nputil.TraceInfoBegin("")

	// Free-bandwidth of all domainlink should bigger than requirement
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]
		infoString := fmt.Sprintf("srcDomainSid(%+v), dstDomainSid(%+v)", srcDomainSid, dstDomainSid)
		nputil.TraceInfo(infoString)

		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		infoString = fmt.Sprintf("baseDomainLink.BaseDomainLinkDbV(%+v), slaAttr(%+v)", baseDomainLink.BaseDomainLinkDbV, appSlaAttr)
		nputil.TraceInfo(infoString)
		//If free-bandwidth is lower than requirement, return false
		if uint64(appSlaAttr.ThroughputValue) > baseDomainLink.BaseDomainLinkDbV.Sla.FreeThroughputValue {
			nputil.TraceInfoEnd("Throughput is not satisfied!")
			return false
		}
	}
	//对于多条最短路径，用户只会选择一条，所以在初步筛选路径时，带宽不扣减去，给出用户所有可选的路径。
	//在推送给用户可选路径时，再重新扣减带宽
	// Update free-bandwidth for basedomainlink
	/*for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]

		//找domainlink，比较Fabric SLA质量矩阵中值是否满足SLA
		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.SrcNodeSN, dstDomainSid.DomainId, dstDomainSid.DstNodeSN)
		infoString := fmt.Sprintf("Update baseDomainLink free bandwidth: BaseDomainLinkDbV(%+v), slaAttr(%+v)", baseDomainLink.BaseDomainLinkDbV, appSlaAttr)
		nputil.TraceInfo(infoString)
		baseDomainLink.UpdateBaseDomainLinkFreeBandwidth(appSlaAttr.ThroughputValue)
	}*/
	nputil.TraceInfoEnd("Throughput is satisfied!")
	return true
}

//Check and update free bandwidth of baseDomainlink
func (domainSrPath *DomainSrPath) isSatisfiedThroughputAndUpdate(appSlaAttr AppSlaAttr) bool {
	nputil.TraceInfoBegin("")

	// Free-bandwidth of all domainlink should bigger than requirement
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]
		infoString := fmt.Sprintf("srcDomainSid(%+v), dstDomainSid(%+v)", srcDomainSid, dstDomainSid)
		nputil.TraceInfo(infoString)

		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		infoString = fmt.Sprintf("baseDomainLink.BaseDomainLinkDbV(%+v), slaAttr(%+v)", baseDomainLink.BaseDomainLinkDbV, appSlaAttr)
		nputil.TraceInfo(infoString)
		//If free-bandwidth is lower than requirement, return false
		if uint64(appSlaAttr.ThroughputValue) > baseDomainLink.BaseDomainLinkDbV.Sla.FreeThroughputValue {
			nputil.TraceInfoEnd("Throughput is not satisfied!")
			return false
		}
	}

	//If the paths is satisfied and then update free-bandwidth for basedomainlink
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]

		//找domainlink，比较Fabric SLA质量矩阵中值是否满足SLA
		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		infoString := fmt.Sprintf("Update baseDomainLink free bandwidth: BaseDomainLinkDbV(%+v), slaAttr(%+v)", baseDomainLink.BaseDomainLinkDbV, appSlaAttr)
		nputil.TraceInfo(infoString)
		baseDomainLink.UpdateBaseDomainLinkFreeBandwidth(uint64(appSlaAttr.ThroughputValue))
	}
	nputil.TraceInfoEnd("Throughput is satisfied!")
	return true
}

func (domainSrPath *DomainSrPath) GetDomainSrPathDelay() (bool, uint32) {
	nputil.TraceInfoBegin("")

	var totalDelay uint32

	infoString := fmt.Sprintf("domainSrPath is:(%+v)", domainSrPath)
	nputil.TraceInfo(infoString)
	//遍历路径中所有节点，获取节点link的delay值
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]

		/*domainSrPath:
		[{DomainId:4 SrcNodeSN:Node42 DstNodeSN:Node42 DomainType:1}
		{DomainId:2 SrcNodeSN:Node24 DstNodeSN:Node21 DomainType:1}
		{DomainId:1 SrcNodeSN:Node12 DstNodeSN: DomainType:1}]})"*/
		infoString := fmt.Sprintf("SrcDomain is:(%+v),SrcNode is:(%+v), dstDomain is:(%+v), dstNode is:(%+v)", srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		nputil.TraceInfo(infoString)
		//找domainlink，比较Fabric SLA质量矩阵中值是否满足SLA
		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		if baseDomainLink == nil {
			nputil.TraceErrorStringWithStack("baseDomainLink is nil")
			return false, totalDelay
		}

		infoString = fmt.Sprintf("baseDomainLink.BaseDomainLinkDbV(%+v).", baseDomainLink.BaseDomainLinkDbV)
		nputil.TraceInfo(infoString)

		tmpDelay := baseDomainLink.BaseDomainLinkDbV.Sla.DelayValue
		// Field_Domain_Inner_Delay是Field域内的预估时延， tmpDelay 是Field - Field域间的Fabric SLA报的时延
		totalDelay = totalDelay + Field_Domain_Inner_Delay + tmpDelay

		infoString = fmt.Sprintf("totalDelay(%d)", totalDelay)
		nputil.TraceInfo(infoString)
	}

	totalDelay = totalDelay + Field_Domain_Inner_Delay //最后尾域Field域内的预估时延
	infoString = fmt.Sprintf("totalDelay(%d)", totalDelay)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return true, totalDelay
}

func (domainSrPath *DomainSrPath) isSatisfiedDelay(appSlaAttr AppSlaAttr) bool {
	nputil.TraceInfoBegin("")

	var totalDelay uint32

	ret, totalDelay := domainSrPath.GetDomainSrPathDelay()
	if ret == false {
		nputil.TraceErrorStringWithStack("baseDomainLink is nil")
		return false
	}
	if totalDelay > uint32(appSlaAttr.DelayValue) {
		nputil.TraceInfoEnd("Delay is not satisfied!")
		return false
	} else {
		nputil.TraceInfoEnd("Delay is satisfied!")
		return true
	}
}

func (domainSrPath *DomainSrPath) isSatisfiedLost(appSlaAttr AppSlaAttr) bool {
	nputil.TraceInfoBegin("")

	var totalNotLost uint32 = 100

	//遍历路径中所有节点，获取节点link的lost值
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]

		//找domainlink，比较Fabric SLA质量矩阵中值是否满足SLA
		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		if baseDomainLink == nil {
			nputil.TraceErrorStringWithStack("baseDomainLink is nil")
			return false
		}

		infoString := fmt.Sprintf("baseDomainLink.BaseDomainLinkDbV(%+v), slaAttr(%+v)", baseDomainLink.BaseDomainLinkDbV, appSlaAttr)
		nputil.TraceInfo(infoString)

		tmpLost := baseDomainLink.BaseDomainLinkDbV.Sla.LostValue
		totalNotLost = totalNotLost * (100 - tmpLost) / 100

		infoString = fmt.Sprintf("totalLost(%d), tmpLost(%d)", 100-totalNotLost, tmpLost)
		nputil.TraceInfo(infoString)
	}

	if (100 - totalNotLost) > uint32(appSlaAttr.LostValue) {
		nputil.TraceInfoEnd("Lost rate is not satisfied!")
		return false
	} else {
		nputil.TraceInfoEnd("Lost rate is satisfied!")
		return true
	}
}

func (domainSrPath *DomainSrPath) isSatisfiedJitter(appSlaAttr AppSlaAttr) bool {
	nputil.TraceInfoBegin("")

	//遍历路径中所有节点，获取节点link的delay值
	for j := 0; j < len(domainSrPath.DomainSidArray)-1; j++ {
		srcDomainSid := domainSrPath.DomainSidArray[j]
		dstDomainSid := domainSrPath.DomainSidArray[j+1]

		//找domainlink，比较Fabric SLA质量矩阵中值是否满足SLA
		baseDomainLink := BaseDomainLinkFindByNodeSN(srcDomainSid.DomainId, srcDomainSid.DstNodeSN, dstDomainSid.DomainId, dstDomainSid.SrcNodeSN)
		if baseDomainLink == nil {
			nputil.TraceErrorStringWithStack("baseDomainLink is nil")
			return false
		}

		infoString := fmt.Sprintf("baseDomainLink.BaseDomainLinkDbV(%+v), slaAttr(%+v)", baseDomainLink.BaseDomainLinkDbV, appSlaAttr)
		nputil.TraceInfo(infoString)

		tmpJitter := baseDomainLink.BaseDomainLinkDbV.Sla.JitterValue

		if tmpJitter > uint32(appSlaAttr.JitterValue) {
			nputil.TraceInfoEnd("Jitter is not satisfied!")
			return false
		}
	}
	nputil.TraceInfoEnd("Jitter is satisfied!")
	return true

}
