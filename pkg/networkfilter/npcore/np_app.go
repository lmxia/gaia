package npcore

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/gogo/protobuf/proto"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	ncsnp "github.com/lmxia/gaia/pkg/networkfilter/model"
	"github.com/lmxia/gaia/pkg/networkfilter/nputil"
)

type SlaAttr struct {
	Delay         uint32
	Jitter        uint32
	Loss          uint32
	Bandwidth     uint64
	FreeBandwidth uint64
}

type AppSlaAttr struct {
	DelayValue      int32 `json:"DelayValue,omitempty"`
	LostValue       int32 `json:"LostValue,omitempty"`
	JitterValue     int32 `json:"JitterValue,omitempty"`
	ThroughputValue int64 `json:"ThroughputValue,omitempty"`
}

type UserSla struct {
	Delay     int32 `json:"delay,omitempty"`
	Lost      int32 `json:"lost,omitempty"`
	Jitter    int32 `json:"jitter,omitempty"`
	Bandwidth int64 `json:"bandwidth,omitempty"`
}

type AutoGenerated struct {
	DelayValue      int `json:"DelayValue"`
	LostValue       int `json:"LostValue"`
	JitterValue     int `json:"JitterValue"`
	ThroughputValue int `json:"ThroughputValue"`
}

// 本来可以复用AppSlaAttr，但为了减少与前端界面base64的问题，都是用in32类型，AppSlaAttr使用的模块比较多，他们都不想动。
type QosSlaAttr struct {
	Delay     int32 `json:"delay,omitempty"`
	Lost      int32 `json:"lost,omitempty"`
	Jitter    int32 `json:"jitter,omitempty"`
	Bandwidth int32 `json:"bandwidth,omitempty"`
}

type RttAttr struct {
	Rtt int32 `json:"rtt,omitempty"`
}

type AccelerateAttr struct {
	Accelerate bool `json:"accelerate,omitempty"`
}

type ProviderAttr struct {
	Providers []string `json:"providers,omitempty"`
}

type AppConnectAttrKey struct {
	SrcUrl      string
	DstUrl      string
	SrcID       uint32
	DstID       uint32
	SrcDomainId uint32
	DstDomainId uint32
}

type KVAttribute struct {
	Key   string
	Value string
}

type AppConnectReqKey struct {
	SrcUrl          string        // 源标识
	DstUrl          string        // 目的标识
	SrcScnidKVList  []KVAttribute // 源标识策略KV lists
	DestScnidKVList []KVAttribute // 目的标识策略KV lists
}

type AppConnectAttr struct {
	Key             AppConnectAttrKey
	SlaAttr         AppSlaAttr
	SrcScnidKVList  []KVAttribute
	DestScnidKVList []KVAttribute
	Accelerate      bool // 冷启动加速
	Rtt             int32
}

type SyncAppConnectAttr struct {
	Key             AppConnectAttrKey
	SlaAttr         AppSlaAttr
	SrcScnidKVList  []KVAttribute
	DestScnidKVList []KVAttribute
	Accelerate      bool // 冷启动加速
}

type AppLinkSlaAttr struct {
	SlaAttr   AppSlaAttr
	Mandatory bool // true:Mandatory, flase:BestEffort
}

type AppLinkRttAttr struct {
	Rtt       int32
	Mandatory bool // true:Mandatory, flase:BestEffort
}

type AppLinkProviderAttr struct {
	Providers []string
	Mandatory bool // true:Mandatory, flase:BestEffort
}

type AppLinkAttr struct {
	Key             AppConnectAttrKey
	LinkSlaAttr     AppLinkSlaAttr
	LinkRttAttr     AppLinkRttAttr
	SrcScnidKVList  []KVAttribute
	DestScnidKVList []KVAttribute
	Providers       AppLinkProviderAttr
	Accelerate      bool // 冷启动加速
}

// component的连接请求AppConnectReq
type AppConnectReq struct {
	Key                  AppConnectReqKey
	linkAttr             AppLinkAttr
	ScnIdDomainPathGroup []ScnIdAppDomainPathGroup // 数组的元素为：以特定SCN_ID为源的，且目的实例不在同一个filed的appConnect的：appConnect和其多路径
}

type AppConnect struct {
	AppConnect []AppLinkAttr
}

type APPComponent struct {
	Name              string
	SelfID            []string
	ScnIdInstList     map[string]ScnIdInstanceArray //k:scn_id, v: ScnIdInstance created by app location and replicas
	AppConnectReqList []AppConnectReq               // Component的interSCNID连接请求列表
	AppconnectList    []AppLinkAttr                 // 以本component的标识为source的APPconnect，以实例号区分
	FiledList         map[string]uint32             // Component可以部署的filed: k: filedName, v: replicas
	TotalReplicas     uint32                        // Component所有的副本数目
}

type ScnIdInstance struct {
	ScnId string
	Filed string
	Id    uint32
}

// 指定标识scn_id的实例列表
type ScnIdInstanceArray struct {
	ScnIdInstList []ScnIdInstance
}

type DomainInfo struct {
	DomainName string
	DomainID   uint32
	DomainType uint32
}

// InterCommunication只选取一个副本的DomainSrPathArray
type AppConnectSelectedDomainPath struct {
	AppConnectAttr SyncAppConnectAttr
	DomainInfoPath []DomainInfo
	DomainSrPath   DomainSrPath // 预占时给控制器的DomainPath
}

type AppConnectDomainPathForRb struct {
	AppConnectAttr AppConnectAttr
	DomainPath     []DomainInfo
	DomainSrPath   DomainSrPath
}

// 一种resourcebing的方案
type BindingSelectedDomainPath struct {
	SelectedDomainPath []AppConnectSelectedDomainPath
}

func getComponentReplicas(rbApp *v1alpha1.ResourceBindingApps, componentName string) uint32 {
	nputil.TraceInfoBegin("")

	var replicasVal int32
	if replicas, ok := rbApp.Replicas[componentName]; ok {
		replicasVal = replicas
	} else {
		infoString := fmt.Sprintf("The component (%s) doesn't exist!\n", componentName)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("")
	return uint32(replicasVal)
}

func GetFiledNamebyComponent(rbApp *v1alpha1.ResourceBindingApps, componentName string) (string, bool) {
	nputil.TraceInfoBegin("")

	var ret bool
	var fieldName string
	if _, ok := rbApp.Replicas[componentName]; ok {
		fieldName = rbApp.ClusterName
		ret = true

	} else {
		infoString := fmt.Sprintf("The field(%s) doesn't have the component (%s)!\n", rbApp.ClusterName, componentName)
		nputil.TraceInfo(infoString)
		ret = false
	}
	nputil.TraceInfoEnd("")
	return fieldName, ret
}

// Map SCN_ID to Component name
func SetSelfId2Component(networkReq v1alpha1.NetworkRequirement) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	for _, scn := range networkReq.Spec.WorkloadComponents.Scns {
		for _, selfId := range scn.SelfID {
			local.selfId2Component[selfId] = scn.Name
		}
	}

	infoString := fmt.Sprintf("local.selfId2Component is: (%+v).\n", local.selfId2Component)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return
}

func SetComponentByNetReq(networkReq v1alpha1.NetworkRequirement) {
	nputil.TraceInfoBegin("")

	for _, netCom := range networkReq.Spec.WorkloadComponents.Scns {
		local.ComponentArray[netCom.Name] = APPComponent{
			Name:              netCom.Name,
			SelfID:            netCom.SelfID,
			ScnIdInstList:     make(map[string]ScnIdInstanceArray),
			AppConnectReqList: []AppConnectReq{},
			AppconnectList:    []AppLinkAttr{},
			FiledList:         make(map[string]uint32),
			TotalReplicas:     0,
		}
	}
	// set the connection attributes for components
	SetAppConnectReqList(networkReq)

	for comName, Component := range local.ComponentArray {
		infoString := fmt.Sprintf("After set by networkReq, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("")
	return
}

// set the field location of component and its replicas
func SetComponentByRb(rb v1alpha1.ResourceBinding) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	for _, rbApp := range rb.Spec.RbApps {
		for comName, replicas := range rbApp.Replicas {
			if replicas == 0 {
				continue
			}
			filedName := rbApp.ClusterName
			infoString := fmt.Sprintf("filedName is [%s], local.ComponentArray[%s] is (%+v).", filedName, comName, local.ComponentArray[comName])
			nputil.TraceInfo(infoString)
			// component不存在
			if _, ok := local.ComponentArray[comName]; !ok {
				continue
			}
			appCom := local.ComponentArray[comName]
			filedList := appCom.FiledList
			filedList[filedName] = uint32(replicas)
			appCom.FiledList = filedList
			appCom.TotalReplicas += uint32(replicas)
			local.ComponentArray[comName] = appCom
		}
	}

	for comName, Component := range local.ComponentArray {
		infoString := fmt.Sprintf("After set by rb, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		nputil.TraceInfoAlwaysPrint(infoString)
	}

	nputil.TraceInfoEnd("")
	return
}

func getComponentBySrcScnId(scnId string) string {
	nputil.TraceInfoBegin("")

	ComName := ""
	local := GetCoreLocal()
	if len(local.ComponentArray) == 0 {
		nputil.TraceInfoEnd("")
		return ComName
	}

	for _, component := range local.ComponentArray {
		sort.Strings(component.SelfID)
		index := sort.SearchStrings(component.SelfID, scnId)
		if index < len(component.SelfID) && component.SelfID[index] == scnId {
			ComName = component.Name
		}
	}

	nputil.TraceInfoEnd("")
	return ComName
}

// 解析deployconditions中的mandatory links属性：SLA, RTT, Providers, 加速启动等属性
func ParseDeployCondition(appReq *AppConnectReq, condition v1alpha1.Condition, mandatory bool) {
	nputil.TraceInfoBegin("")

	infoString := fmt.Sprintf("appReq is: (%+v), condition is:  (%+v)\n.", appReq, condition)
	nputil.TraceInfoAlwaysPrint(infoString)

	if condition.Subject.Type == "link" {
		if condition.Object.Type == "sla" {
			sla := new(AppSlaAttr)
			userSla := new(UserSla)
			slaByte := fmt.Sprintf("slaStr is: (%s)", condition.Extent)
			nputil.TraceInfo(slaByte)
			err := json.Unmarshal(condition.Extent, userSla)
			if err != nil {
				info := fmt.Sprintf("slaStr unmarshal is failed!")
				nputil.TraceInfo(info)
				nputil.TraceInfoEnd("")
				return
			} else {
				info := fmt.Sprintf("userSla is: (%+v)", userSla)
				nputil.TraceInfo(info)
				sla.DelayValue = userSla.Delay
				sla.LostValue = userSla.Lost
				sla.JitterValue = userSla.Jitter
				sla.ThroughputValue = userSla.Bandwidth
				appReq.linkAttr.LinkSlaAttr.SlaAttr = *sla
				appReq.linkAttr.LinkSlaAttr.Mandatory = mandatory
				info1 := fmt.Sprintf("LinkSlaAttr is : (%+v).", appReq.linkAttr.LinkSlaAttr)
				nputil.TraceInfoAlwaysPrint(info1)
			}
		} else if condition.Object.Type == "rtt" {
			rtt := new(RttAttr)
			rttByte := fmt.Sprintf("rttStr is: (%s)", condition.Extent)
			nputil.TraceInfo(rttByte)
			err := json.Unmarshal(condition.Extent, rtt)
			if err != nil {
				info := fmt.Sprintf("rttStr unmarshal is failed!")
				nputil.TraceInfo(info)
				nputil.TraceInfoEnd("")
				return
			} else {
				appReq.linkAttr.LinkRttAttr.Rtt = (*rtt).Rtt
				appReq.linkAttr.LinkRttAttr.Mandatory = mandatory
				info := fmt.Sprintf("LinkRttAttr is : (%+v).", appReq.linkAttr.LinkRttAttr)
				nputil.TraceInfoAlwaysPrint(info)
			}
		} else if condition.Object.Type == "label" {
			providers := new([]string)
			providerByte := fmt.Sprintf("providerStr is: (%s)", condition.Extent)
			nputil.TraceInfo(providerByte)
			err := json.Unmarshal(condition.Extent, providers)
			if err != nil {
				info := fmt.Sprintf("providerStr unmarshal is failed!")
				nputil.TraceInfo(info)
				nputil.TraceInfoEnd("")
				return
			} else {
				appReq.linkAttr.Providers.Providers = *providers
				appReq.linkAttr.Providers.Mandatory = mandatory
				info := fmt.Sprintf("Providers is : (%+v).", appReq.linkAttr.Providers)
				nputil.TraceInfoAlwaysPrint(info)
			}
		} else if condition.Object.Type == "accelerate" {
			accelerate := new(AccelerateAttr)
			accelerateByte := fmt.Sprintf("accelerateStr is: (%s)", condition.Extent)
			nputil.TraceInfo(accelerateByte)
			err := json.Unmarshal(condition.Extent, accelerate)
			if err != nil {
				info := fmt.Sprintf("accelerateStr unmarshal is failed!")
				nputil.TraceInfo(info)
				nputil.TraceInfoEnd("")
				return
			} else {
				info := fmt.Sprintf("accelerate is : (%+v).", *accelerate)
				nputil.TraceInfoAlwaysPrint(info)
				appReq.linkAttr.Accelerate = (*accelerate).Accelerate
			}
		} else {
			nputil.TraceInfoEnd("")
			return
		}
	}
	nputil.TraceInfoEnd("")
}

// set the connection attributes requirement for components
func SetAppConnectReqList(networkReq v1alpha1.NetworkRequirement) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()

	// for _, netCom := range networkReq.Spec.NetworkCommunication {
	// 1.解析component和其对应的标识列表ScnIdList
	for _, scn := range networkReq.Spec.WorkloadComponents.Scns {
		comName := scn.Name
		appComponent := local.ComponentArray[comName]
		appComponent.SelfID = scn.SelfID
		local.ComponentArray[comName] = appComponent
	}
	// 2. 解析标识通信links
	for _, link := range networkReq.Spec.WorkloadComponents.Links {
		comName := getComponentBySrcScnId(link.SourceID)
		if comName == "" {
			nputil.TraceInfoEnd("")
			return
		}
		appComponent := local.ComponentArray[comName]
		appReq := new(AppConnectReq)

		// 解析源目的标识
		srcScnID := link.SourceID
		dstScnID := link.DestinationID
		appReq.Key.SrcUrl = srcScnID
		appReq.Key.DstUrl = dstScnID

		// 解析源目的标识的KV lists
		for _, kv := range link.SourceAttributes {
			var srcKv KVAttribute
			srcKv.Key = kv.Key
			srcKv.Value = kv.Value
			appReq.Key.SrcScnidKVList = append(appReq.Key.SrcScnidKVList, srcKv)
			appReq.linkAttr.SrcScnidKVList = append(appReq.linkAttr.SrcScnidKVList, srcKv)
		}
		for _, kv := range link.DestinationAttributes {
			var dstKv KVAttribute
			dstKv.Key = kv.Key
			dstKv.Value = kv.Value
			appReq.Key.DestScnidKVList = append(appReq.Key.DestScnidKVList, dstKv)
			appReq.linkAttr.DestScnidKVList = append(appReq.linkAttr.DestScnidKVList, dstKv)
		}

		// 解析deployconditions中的mandatory links属性：SLA, RTT, Providers, 加速启动等属性
		for _, condition := range networkReq.Spec.Deployconditions.Mandatory {
			if condition.Subject.Name == link.LinkName {
				ParseDeployCondition(appReq, condition, true)
			}
		}
		// 解析deployconditions中的bestEffort links属性：SLA, RTT, Providers, 加速启动等属性
		for _, condition := range networkReq.Spec.Deployconditions.BestEffort {
			if condition.Subject.Name == link.LinkName {
				ParseDeployCondition(appReq, condition, false)
			}
		}
		infoString := fmt.Sprintf("appReq is: (%+v).\n", *appReq)
		nputil.TraceInfoAlwaysPrint(infoString)
		appComponent.AppConnectReqList = append(appComponent.AppConnectReqList, *appReq)
		local.ComponentArray[comName] = appComponent
	}

	for comName, Component := range local.ComponentArray {
		infoString := fmt.Sprintf("local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("")
	return
}

func CreateScnIdInstanceForScnId(comName string, scnID string) ScnIdInstanceArray {
	nputil.TraceInfoBegin("")

	var ScnIdInstList ScnIdInstanceArray
	var intId uint32

	local := GetCoreLocal()
	for filed, replicas := range local.ComponentArray[comName].FiledList {
		for i := 0; i < int(replicas); i++ {
			var scnIdInt ScnIdInstance
			scnIdInt.ScnId = scnID
			scnIdInt.Filed = filed
			scnIdInt.Id = intId
			intId++
			ScnIdInstList.ScnIdInstList = append(ScnIdInstList.ScnIdInstList, scnIdInt)
		}
	}
	infoString := fmt.Sprintf("ScnIdInstList is: (%+v).\n", ScnIdInstList)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return ScnIdInstList
}

func SetScnIdInstanceForComponent(component APPComponent) {
	nputil.TraceInfoBegin("")
	local := GetCoreLocal()

	tmpInstList := make(map[string]ScnIdInstanceArray)
	tmpComp := local.ComponentArray[component.Name]
	for _, scnId := range component.SelfID {
		ScnIdInstList := CreateScnIdInstanceForScnId(component.Name, scnId)
		infoString := fmt.Sprintf("local.ComponentArray[%s] is (%+v).", component.Name, local.ComponentArray[component.Name])
		nputil.TraceInfo(infoString)
		tmpInstList[scnId] = ScnIdInstList
	}
	tmpComp.ScnIdInstList = tmpInstList
	local.ComponentArray[component.Name] = tmpComp

	infoString := fmt.Sprintf("local.ComponentArray[%s] is (%+v) after create ScnIdInstance.\n", component.Name, local.ComponentArray[component.Name])
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return
}

func getAppConnectionByScnId(srcScnId string, dstScnId string) (AppConnectReq, bool) {
	nputil.TraceInfoBegin("")

	var appReq AppConnectReq
	var findFlag bool
	local := GetCoreLocal()
	srcCompName := local.selfId2Component[srcScnId]
	for _, tmpAppReq := range local.ComponentArray[srcCompName].AppConnectReqList {
		if srcScnId == tmpAppReq.Key.SrcUrl && dstScnId == tmpAppReq.Key.DstUrl {
			appReq = tmpAppReq
			findFlag = true
		}
	}

	nputil.TraceInfoEnd("")
	return appReq, findFlag
}

// Set App connect instances for all components
// instances are initialized with src_scnid, srcDomain, instanceId, srcKVList and dstKVlist
func SetScnIdInstances() {
	nputil.TraceInfoBegin("")
	local := GetCoreLocal()
	for _, component := range local.ComponentArray {
		SetScnIdInstanceForComponent(component)
	}

	for comName, Component := range local.ComponentArray {
		infoString := fmt.Sprintf("local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		nputil.TraceInfo(infoString)
	}
	nputil.TraceInfoEnd("")
}

// 因为AppConnectReq中的key中没有实例号，domainId等，根据部署的field和副本数，重构app
// 获取标识通信的连接属性：key, kv, sla, rtt, provider，冷启动加速等
func CreateAppConnectAttrByScnInst(srcInst ScnIdInstance, dstInst ScnIdInstance) *AppLinkAttr {
	nputil.TraceInfoBegin("")

	appLinkAttr := new(AppLinkAttr)

	appLinkAttr.Key.SrcUrl = srcInst.ScnId
	appLinkAttr.Key.DstUrl = dstInst.ScnId
	appLinkAttr.Key.SrcID = srcInst.Id
	appLinkAttr.Key.DstID = dstInst.Id
	srcDomainId, flag := getDomainIDbyName(srcInst.Filed)
	if flag == false {
		infoString := fmt.Sprintf("The source domain (%s) doesn't exist!\n", srcInst.Filed)
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return nil
	}
	dstDomainId, flag := getDomainIDbyName(dstInst.Filed)
	if flag == false {
		infoString := fmt.Sprintf("The component (%s) doesn't exist!\n", dstInst.Filed)
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return nil
	}
	appLinkAttr.Key.SrcDomainId = srcDomainId
	appLinkAttr.Key.DstDomainId = dstDomainId
	AppReq, flag := getAppConnectionByScnId(srcInst.ScnId, dstInst.ScnId)
	if flag == false {
		infoString := fmt.Sprintf("The AppConnectReq cannot find by srcScnID(%+v) and dstInst(%+v)!\n", srcInst, dstInst)
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return nil
	}
	appLinkAttr.LinkSlaAttr = AppReq.linkAttr.LinkSlaAttr
	appLinkAttr.LinkRttAttr = AppReq.linkAttr.LinkRttAttr
	appLinkAttr.Providers = AppReq.linkAttr.Providers
	appLinkAttr.Accelerate = AppReq.linkAttr.Accelerate
	appLinkAttr.SrcScnidKVList = AppReq.linkAttr.SrcScnidKVList
	appLinkAttr.DestScnidKVList = AppReq.linkAttr.DestScnidKVList

	nputil.TraceInfoEnd("")
	return appLinkAttr
}

func PbRbDomainPathsCreate(rbDomainPaths BindingSelectedDomainPath) *ncsnp.BindingSelectedDomainPath {
	nputil.TraceInfoBegin("")

	pbRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
	for _, appConnectDomainPath := range rbDomainPaths.SelectedDomainPath {
		pbAppConnectDomainPath := new(ncsnp.AppConnectSelectedDomainPath)

		infoString4 := fmt.Sprintf("appConnectDomainPath is(%+v)!", appConnectDomainPath)
		nputil.TraceInfo(infoString4)

		pbAppConnectAttr := new(ncsnp.AppConnectAttr)
		key := new(ncsnp.AppConnectKey)
		key.SrcSCNID = appConnectDomainPath.AppConnectAttr.Key.SrcUrl
		key.DstSCNID = appConnectDomainPath.AppConnectAttr.Key.DstUrl
		key.SrcID = appConnectDomainPath.AppConnectAttr.Key.SrcID
		key.DstID = appConnectDomainPath.AppConnectAttr.Key.DstID
		key.SrcDomainId = appConnectDomainPath.AppConnectAttr.Key.SrcDomainId
		key.DstDomainId = appConnectDomainPath.AppConnectAttr.Key.DstDomainId

		pbAppConnectAttr.Key = key
		infoString2 := fmt.Sprintf(" pbAppConnectAttr.Key.SrcID(%+v), pbAppConnectAttr.Key.DstID(%+v),pbAppConnectAttr.Key(%+v)!",
			pbAppConnectAttr.Key.SrcID, pbAppConnectAttr.Key.DstID, pbAppConnectAttr.Key)
		nputil.TraceInfo(infoString2)

		slaAttr := new(ncsnp.AppSlaAttr)
		slaAttr.DelayValue = uint32(appConnectDomainPath.AppConnectAttr.SlaAttr.DelayValue)
		slaAttr.JitterValue = uint32(appConnectDomainPath.AppConnectAttr.SlaAttr.JitterValue)
		slaAttr.ThroughputValue = uint64(appConnectDomainPath.AppConnectAttr.SlaAttr.ThroughputValue)
		slaAttr.LostValue = uint32(appConnectDomainPath.AppConnectAttr.SlaAttr.LostValue)
		pbAppConnectAttr.SlaAttr = slaAttr

		for _, srcKv := range appConnectDomainPath.AppConnectAttr.SrcScnidKVList {
			pSrcKv := new(ncsnp.KVAttribute)
			pSrcKv.Key = srcKv.Key
			pSrcKv.Value = srcKv.Value
			pbAppConnectAttr.SrcScnidKVList = append(pbAppConnectAttr.SrcScnidKVList, pSrcKv)
		}
		for _, dstKv := range appConnectDomainPath.AppConnectAttr.DestScnidKVList {
			pDstKv := new(ncsnp.KVAttribute)
			pDstKv.Key = dstKv.Key
			pDstKv.Value = dstKv.Value
			pbAppConnectAttr.DestScnidKVList = append(pbAppConnectAttr.DestScnidKVList, pDstKv)
		}
		if appConnectDomainPath.AppConnectAttr.Accelerate == true {
			pbAppConnectAttr.Accelerate = appConnectDomainPath.AppConnectAttr.Accelerate
		} else {
			pbAppConnectAttr.Accelerate = false
		}
		infoString1 := fmt.Sprintf("pbAppConnectAttr.Accelerate is (%+v).", pbAppConnectAttr.Accelerate)
		nputil.TraceInfo(infoString1)

		pbAppConnectDomainPath.AppConnect = pbAppConnectAttr
		infoString := fmt.Sprintf("pbAppConnectDomainPath.AppConnect is (%+v)!", pbAppConnectDomainPath.AppConnect)
		nputil.TraceInfo(infoString)

		for _, domainInfoPath := range appConnectDomainPath.DomainInfoPath {
			pbDomainInfo := new(ncsnp.DomainInfo)
			pbDomainInfo.DomainName = domainInfoPath.DomainName
			pbDomainInfo.DomainId = domainInfoPath.DomainID
			pbDomainInfo.DomainType = domainInfoPath.DomainType
			pbAppConnectDomainPath.DomainList = append(pbAppConnectDomainPath.DomainList, pbDomainInfo)
		}
		pbAppConnectDomainPath.Content = []byte{}
		pbRbDomainPaths.SelectedDomainPath = append(pbRbDomainPaths.SelectedDomainPath, pbAppConnectDomainPath)
	}

	for i, rbDomainPath := range pbRbDomainPaths.SelectedDomainPath {
		infoString := fmt.Sprintf("pbRbDomainPaths[%d] is: (%+v).", i, rbDomainPath)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("")
	return pbRbDomainPaths
}

func ClearLocalComConnectArray() {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	for appname := range local.ComponentArray {
		/*for fieldName, _ := range local.ComponentArray[appname].FiledList {
			delete(local.ComponentArray, fieldName)
		}*/
		delete(local.ComponentArray, appname)
	}
	nputil.TraceInfoEnd("")
}

// 从各个连接属性的实例的pathgroup中各选出一个，组合成一组方案返回给resourceBinding
func CombMatrixToDomainPathGroup() [][]AppDomainPath {
	nputil.TraceInfoBegin("")
	local := GetCoreLocal()

	var appConnectDomainPathMatrix [][]AppDomainPath
	var appConnectDomainPathArray []AppDomainPath
	var combPath []AppDomainPath
	var combGroup [][]AppDomainPath
	var index uint32

	// 把所有components的所有连接属性的所有实例的单个domainpath定义为一个矩阵，
	// 每一维度是:特定SrcUrl/DstUrl/srcId/DstId/srcField/dstField的具体实例对应的domainpath数组，且目的实例号不在同一个filed内
	// 矩阵中的每个元素为：带实例号的AppConnect的单条domainPath
	for comName, component := range local.ComponentArray {
		// 指定副本下的AppConnectReqList
		for i, appConnectReqList := range component.AppConnectReqList {
			// AppRequest下所有的副本实例的多条domainPath
			// 清空内容
			infoString := fmt.Sprintf("component[%s].AppConnectReqList[%d] is: (%+v)", comName, i, appConnectReqList)
			nputil.TraceInfo(infoString)
			// scnIdAppDomainPathGroup指定源连接属性的所有实例的domainPathGroup
			for _, scnIdAppDomainPathGroup := range appConnectReqList.ScnIdDomainPathGroup {
				// 带实例号的连接属性的多条domainPath路径为一维数组
				appConnectDomainPathArray = []AppDomainPath{}
				for _, appDomainPathArray := range scnIdAppDomainPathGroup.AppDomainPathArray {
					infoString := fmt.Sprintf("appDomainPathArray is: (%+v)\n\n", appDomainPathArray)
					nputil.TraceInfo(infoString)
					for _, appDomainPath := range appDomainPathArray.DomainSrPathArray {
						appConnectDomainPath := AppDomainPath{}
						appConnectDomainPath.AppConnect = appDomainPathArray.AppConnect
						appConnectDomainPath.DomainSrPath = appDomainPath
						appConnectDomainPathArray = append(appConnectDomainPathArray, appConnectDomainPath)
						infoString := fmt.Sprintf("appConnectDomainPathArray is: (%+v)\n\n", appConnectDomainPathArray)
						nputil.TraceInfo(infoString)
					}
				}
				// 带实例号的AppConnect的domainSrPath matrix
				appConnectDomainPathMatrix = append(appConnectDomainPathMatrix, appConnectDomainPathArray)
			}
		}
	}
	for i, appConnectDomainPathArray := range appConnectDomainPathMatrix {
		infoString := fmt.Sprintf("appConnectDomainPathMatrix[%d], is: (%+v)\n\n", i, appConnectDomainPathArray)
		nputil.TraceInfo(infoString)
	}
	// Number of arrays
	n := len(appConnectDomainPathMatrix)
	if n == 0 {
		infoString := fmt.Sprintf("The InterScnIDDomainPathMatrix is empty!\n")
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return combGroup
	}
	// To keep track of next element in each of the n arrays
	indices := make([]int, n)
	// Initialize with first element's index
	for i := 0; i < n; i++ {
		indices[i] = 0
	}

	for {
		/*if index > DomainPathGroupMaxNum-1 {
			for _, combPath := range combGroup {
				infoString := fmt.Sprintf("The combPath is (%+v)!\n", combPath)
				nputil.TraceInfo(infoString)
			}
			nputil.TraceInfoEnd("")
			return combGroup
		}*/
		// combine domainpath for different interSCNID connections
		combPath = []AppDomainPath{}
		for j := 0; j < n; j++ {
			infoString := fmt.Sprintf("appConnectDomainPathMatrix[%d][indices[%d]] is (%+v)", j, j, appConnectDomainPathMatrix[j][indices[j]])
			nputil.TraceInfo(infoString)
			combPath = append(combPath, appConnectDomainPathMatrix[j][indices[j]])
		}
		combGroup = append(combGroup, combPath)
		index++
		// find the rightmost array that has more elements left after the current element in that array
		next := n - 1
		for next >= 0 && (indices[next]+1 >= len(appConnectDomainPathMatrix[next])) {
			next--
		}
		// no such array is found so no more combinations left
		if next < 0 {
			for i, combPath := range combGroup {
				infoString := fmt.Sprintf("The combPath[%d] is (%+v)!\n", i, combPath)
				nputil.TraceInfo(infoString)
			}
			nputil.TraceInfoEnd("")
			return combGroup
		}
		// if found move to next element in that array
		indices[next]++
		// for all arrays to the right of this array current index again points to first element
		for k := next + 1; k < n; k++ {
			indices[k] = 0
		}
	}
}

// 假设连接属性的源srcScnId的component有5个副本，则每个副本实例在该连接属性都可达，才认为该连接属性有效。
// 每个副本实例只要有一条可达实例就认为该副本的连接属性是有效的：即a1--b1可达就认为有效
// 尽量满足负荷分担，如：a1--b1可达，那a2尝试从b2开始连接，如果到a2---b2……b5都不可达，则再从头计算a2-b1，截至的实例为上次循环的前一个实例b2-1=b1
func CalAppConnectAttrForLink(link v1alpha1.Link) bool {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()

	graph := GraphFindByGraphType(GraphTypeAlgo_Cspf)
	domainGraph := graph.DomainGraphPoint

	srcCom := local.selfId2Component[link.SourceID]                                                  // 源component
	dstCom := local.selfId2Component[link.DestinationID]                                             // 目的component
	srcScnIdInstList := local.ComponentArray[srcCom].ScnIdInstList[link.SourceID].ScnIdInstList      // 源定标识scn_id的实例列表
	dstScnIdInstList := local.ComponentArray[dstCom].ScnIdInstList[link.DestinationID].ScnIdInstList // 目的标识scn_id的实例列表

	//var appDomainPathGroupArray [][]AppDomainPathArray
	/*TBD:为了尽可能地在构造各个实例标识的matrix时负荷分担，目标实例不集中在某几个实例上，构建以某个源标识的appConnect时候，采用折回方式,且目标实例在同一个域看作一个：
	// 实例部署： a1f1   b1f1
	            a2f2   b2f2
	            a3f2   b3f2
	// 构造的组合 a1: a1f1-b1f1 a1f1-b2f2
	//          a2: a2f2-b3f2 a2f2-b1f1  折回:记录上一个a1的可用index为1，从1+1开始计算，如果上一次计算到最后一个，从0开始计算
	//          a3: a3f2-b2f2 a2f2-b1f1
	*/
	ScnIdDomainPathGroupArray := []ScnIdAppDomainPathGroup{}
	for i := 0; i < len(srcScnIdInstList); i++ {
		ScnIdDomainPathGroup := ScnIdAppDomainPathGroup{}
		ScnIdDomainPathGroup.ScnIdInstance = srcScnIdInstList[i]

		for j := 0; j < len(dstScnIdInstList); j++ {
			// 获取标识通信的连接属性：key, kv, sla, rtt, provider，冷启动加速等
			appLinkAttr := CreateAppConnectAttrByScnInst(srcScnIdInstList[i], dstScnIdInstList[j])
			if appLinkAttr == nil {
				continue
			}
			// appConnectAttr用于网络算路后，回填给运营系统
			var appConnectAttr AppConnectAttr
			appConnectAttr.Key = appLinkAttr.Key
			appConnectAttr.SlaAttr = appLinkAttr.LinkSlaAttr.SlaAttr
			appConnectAttr.SrcScnidKVList = appLinkAttr.SrcScnidKVList
			appConnectAttr.DestScnidKVList = appLinkAttr.DestScnidKVList
			appConnectAttr.Accelerate = appLinkAttr.Accelerate
			appConnectAttr.Rtt = appLinkAttr.LinkRttAttr.Rtt

			infoString := fmt.Sprintf("srcScnIdInstList[%d] is: (%+v).\n dstScnIdInstList[%d] is: (%+v).\n appLinkAttr is: (%+v)",
				i, srcScnIdInstList[i], j, dstScnIdInstList[j], appLinkAttr)
			nputil.TraceInfo(infoString)
			dstDuplicate := false

			// 如果同一个filed内的目的实例已经计算过，则不在计算
			for _, tmpAppConnect := range ScnIdDomainPathGroup.AppDomainPathArray {
				if appLinkAttr.Key.SrcDomainId == tmpAppConnect.AppConnect.Key.SrcDomainId &&
					appLinkAttr.Key.DstDomainId == tmpAppConnect.AppConnect.Key.DstDomainId &&
					appLinkAttr.Key.SrcUrl == tmpAppConnect.AppConnect.Key.SrcUrl &&
					appLinkAttr.Key.DstUrl == tmpAppConnect.AppConnect.Key.DstUrl &&
					appConnectAttr.Key.SrcID == tmpAppConnect.AppConnect.Key.SrcID {
					dstDuplicate = true
					break
				}
			}
			if dstDuplicate == true {
				break
			}
			// 源和目的在同一个filed,认为可达，返回所在filed
			if appConnectAttr.Key.SrcDomainId == appConnectAttr.Key.DstDomainId {
				domainSrPath := []DomainSrPath{
					0: {
						DomainSidArray: []DomainSid{
							0: {DomainId: appConnectAttr.Key.SrcDomainId},
						},
					},
				}
				tmpAappDomainPath := AppDomainPathArray{}
				tmpAappDomainPath.DomainSrPathArray = domainSrPath

				tmpAappDomainPath.AppConnect = appConnectAttr
				ScnIdDomainPathGroup.AppDomainPathArray = append(ScnIdDomainPathGroup.AppDomainPathArray, tmpAappDomainPath)
				break
			}

			// 源实例标识和目的实例标识不在同一个field计算domainPath
			// 只计算以时延为权重的domainPath
			domainSrPathArray := graph.AppConnect(domainGraph.DomainLinkKspGraph, KspCalcMaxNum, *appLinkAttr)
			domainSrNamePath := graph.GetDomainPathNameArrayWithFaric(domainSrPathArray)
			infoString = fmt.Sprintf("domainSrNamePath is (%+v)\n", domainSrNamePath)
			nputil.TraceInfo(infoString)
			if len(domainSrPathArray) != 0 {
				appDomainPath := AppDomainPathArray{}
				appDomainPath.DomainSrPathArray = domainSrPathArray
				appDomainPath.AppConnect = appConnectAttr
				// 附加一个具体实例AppConnect的多条路径
				ScnIdDomainPathGroup.AppDomainPathArray = append(ScnIdDomainPathGroup.AppDomainPathArray, appDomainPath)
				infoString = fmt.Sprintf("ScnIdDomainPathGroup.AppDomainPathArray is (%+v)\n", ScnIdDomainPathGroup.AppDomainPathArray)
				nputil.TraceInfo(infoString)
			}
		}

		// 判断该源副本实例是否存在可达路径
		// 当每个副本都的链接属性都可达时，才认为该链接属性可达，只要有一个链接属性不可达，ResourceBinding方案无效
		if len(ScnIdDomainPathGroup.AppDomainPathArray) == 0 {
			infoString := fmt.Sprintf("The interCommunication (%+v) is unavailable!\n", link)
			nputil.TraceInfo(infoString)
			nputil.TraceInfoEnd("false")
			return false
		} else {
			// 该副本实例存在可达路径，将该副本实例的与不同目的实例的AppConnect的domainSrPathArray 存放在相同源标识的local.ComponentArray[srcCom].AppConnectReqList下
			for i, tmpAppReq := range local.ComponentArray[srcCom].AppConnectReqList {
				if link.SourceID == tmpAppReq.Key.SrcUrl && link.DestinationID == tmpAppReq.Key.DstUrl {
					ScnIdDomainPathGroupArray = append(ScnIdDomainPathGroupArray, ScnIdDomainPathGroup)
					local.ComponentArray[srcCom].AppConnectReqList[i].ScnIdDomainPathGroup = ScnIdDomainPathGroupArray
					infoString := fmt.Sprintf("The local.ComponentArray[%s].AppConnectReqList[%d].ScnIdDomainPathGroupArray is (%+v)!\n",
						srcCom, i, ScnIdDomainPathGroupArray)
					nputil.TraceInfo(infoString)
				}
			}
		}
	}

	infoString := fmt.Sprintf("The interCommunication (%+v) is available!\n", link)
	nputil.TraceInfo(infoString)
	nputil.TraceInfoEnd("true")
	return true
}

func CalAppConnectAttrForRb(rb *v1alpha1.ResourceBinding, networkReq v1alpha1.NetworkRequirement) *v1alpha1.ResourceBinding {
	nputil.TraceInfoBegin("")

	graph := GraphFindByGraphType(GraphTypeAlgo_Cspf)

	// set component map
	SetComponentByNetReq(networkReq)
	// Map SCN_ID to Component name
	SetSelfId2Component(networkReq)
	// set the filed location of component and ite replicas
	SetComponentByRb(*rb)
	// Set App connect instances for all components
	SetScnIdInstances()

	// Calc and check domainSrPathArray of all appConnect instances for all interCommunication
	// and store domainSrPathArray in
	var available bool
	available = true
	for _, link := range networkReq.Spec.WorkloadComponents.Links {
		infoString := fmt.Sprintf("The netCom is (%+v)!\n", link)
		nputil.TraceInfo(infoString)
		available = CalAppConnectAttrForLink(link)
		if available == false {
			infoString := fmt.Sprintf("The interComm (%+v) is unavailable!\n", link)
			nputil.TraceInfoAlwaysPrint(infoString)
			nputil.TraceInfoEnd("")
			return nil
		}
	}

	// 选出DomainPathGroupMaxNum组组合方案
	var domainPathCluster [][]AppDomainPath
	domainPathComb := CombMatrixToDomainPathGroup()
	if len(domainPathComb) < DomainPathGroupMaxNum {
		domainPathCluster = domainPathComb
	} else {
		indexList := nputil.GenerateRandomIndex(0, len(domainPathComb)-1, DomainPathGroupMaxNum)
		infoString := fmt.Sprintf("indexList is(%+v)!\n", indexList)
		nputil.TraceInfo(infoString)
		for _, index := range indexList {
			domainPathCluster = append(domainPathCluster, domainPathComb[index])
		}
	}
	for i, combPath := range domainPathCluster {
		infoString := fmt.Sprintf("domainPathCluster[%d] is (%+v)!\n", i, combPath)
		nputil.TraceInfo(infoString)
	}

	if len(domainPathCluster) != 0 {
		var rbDomainPathArray []BindingSelectedDomainPath
		for _, appPathGroup := range domainPathCluster {
			rbSdp := BindingSelectedDomainPath{}
			for i, appDomainPath := range appPathGroup {
				infoString := fmt.Sprintf("appPathGroup[%d] is: (%+v).\n\n", i, appDomainPath)
				nputil.TraceInfo(infoString)
				appSelectedPath := AppConnectSelectedDomainPath{}
				appSelectedPath.AppConnectAttr.Key = appDomainPath.AppConnect.Key
				appSelectedPath.AppConnectAttr.SlaAttr = appDomainPath.AppConnect.SlaAttr
				appSelectedPath.AppConnectAttr.SrcScnidKVList = appDomainPath.AppConnect.SrcScnidKVList
				appSelectedPath.AppConnectAttr.DestScnidKVList = appDomainPath.AppConnect.DestScnidKVList
				appSelectedPath.AppConnectAttr.Accelerate = appDomainPath.AppConnect.Accelerate
				appSelectedPath.DomainSrPath = appDomainPath.DomainSrPath
				appSelectedPath.DomainInfoPath = graph.GetDomainPathNameWithFaric(appDomainPath.DomainSrPath)

				// 如果有rtt要求，计算并同步最短时延的反向路径, rtt在计算正向路径时已经满足，此处不用校验，肯定会满足
				rvrAppSelectedPath := AppConnectSelectedDomainPath{}
				if appDomainPath.AppConnect.Rtt != 0 {
					reverseDomainSrPath := new(DomainSrPath)
					// 获取反向路径的domainsr
					// 源和目的在同一个filed,认为可达，返回所在filed
					if appDomainPath.AppConnect.Key.SrcDomainId == appDomainPath.AppConnect.Key.DstDomainId {
						*reverseDomainSrPath = DomainSrPath{
							DomainSidArray: []DomainSid{
								0: {DomainId: appDomainPath.AppConnect.Key.SrcDomainId},
							},
						}
					} else {
						reverseDomainSrPath = GetReverseDomainPath(graph, appDomainPath.AppConnect)
						if nil == reverseDomainSrPath {
							break
						}
					}
					// 构造反向通信请求
					rvrAppSelectedPath.AppConnectAttr.Key.SrcUrl = appDomainPath.AppConnect.Key.DstUrl
					rvrAppSelectedPath.AppConnectAttr.Key.DstUrl = appDomainPath.AppConnect.Key.SrcUrl
					rvrAppSelectedPath.AppConnectAttr.Key.SrcID = appDomainPath.AppConnect.Key.DstID
					rvrAppSelectedPath.AppConnectAttr.Key.DstID = appDomainPath.AppConnect.Key.SrcID
					rvrAppSelectedPath.AppConnectAttr.Key.SrcDomainId = appDomainPath.AppConnect.Key.DstDomainId
					rvrAppSelectedPath.AppConnectAttr.Key.DstDomainId = appDomainPath.AppConnect.Key.SrcDomainId
					rvrAppSelectedPath.AppConnectAttr.SlaAttr = appDomainPath.AppConnect.SlaAttr
					rvrAppSelectedPath.AppConnectAttr.SrcScnidKVList = appDomainPath.AppConnect.SrcScnidKVList
					rvrAppSelectedPath.AppConnectAttr.DestScnidKVList = appDomainPath.AppConnect.DestScnidKVList
					rvrAppSelectedPath.AppConnectAttr.Accelerate = appDomainPath.AppConnect.Accelerate
					rvrAppSelectedPath.DomainSrPath = *reverseDomainSrPath
					rvrAppSelectedPath.DomainInfoPath = graph.GetDomainPathNameWithFaric(rvrAppSelectedPath.DomainSrPath)
				}
				rbSdp.SelectedDomainPath = append(rbSdp.SelectedDomainPath, appSelectedPath)
				if appDomainPath.AppConnect.Rtt != 0 {
					rbSdp.SelectedDomainPath = append(rbSdp.SelectedDomainPath, rvrAppSelectedPath)
				}
			}
			infoString := fmt.Sprintf("rbSdp : (%+v).\n\n", rbSdp)
			nputil.TraceInfo(infoString)
			if len(rbSdp.SelectedDomainPath) != 0 {
				rbDomainPathArray = append(rbDomainPathArray, rbSdp)
			}
		}
		// 构造message数据
		for i, bindDomainPath := range rbDomainPathArray {
			infoString := fmt.Sprintf("---------bindDomainPath SelectedDomainPath [%d] ---------\n", i)
			nputil.TraceInfo(infoString)
			for _, appDomainPath := range bindDomainPath.SelectedDomainPath {
				infoString := fmt.Sprintf("AppConnectAttr is: [%+v]", appDomainPath.AppConnectAttr)
				nputil.TraceInfo(infoString)
				infoString = fmt.Sprintf("DomainSrPath is: [%+v]", appDomainPath.DomainSrPath)
				nputil.TraceInfo(infoString)
				infoString = fmt.Sprintf("DomainInfoPath is: [%+v]", appDomainPath.DomainInfoPath)
				nputil.TraceInfo(infoString)
			}
			pbRbDomainPaths := PbRbDomainPathsCreate(bindDomainPath)
			content, err := proto.Marshal(pbRbDomainPaths)
			if err != nil {
				nputil.TraceErrorWithStack(err)
				return nil
			}
			infoString = fmt.Sprintf("Proto marshal content bytes is: [%+v]", content)
			nputil.TraceInfo(infoString)
			NpContentBase64 := make([]byte, base64.StdEncoding.EncodedLen(len(content)))
			base64.StdEncoding.Encode(NpContentBase64, content)
			infoString = fmt.Sprintf("Proto marshal content []base64 is: [%+v]", NpContentBase64)
			nputil.TraceInfo(infoString)
			encodeToString := base64.StdEncoding.EncodeToString(NpContentBase64)
			infoString = fmt.Sprintf("Proto marshal content base64 encodeToString is (%+v)", encodeToString)
			nputil.TraceInfo(infoString)

			// Verify the unmarshal action
			dbuf := make([]byte, base64.StdEncoding.DecodedLen(len(NpContentBase64)))
			n, _ := base64.StdEncoding.Decode(dbuf, []byte(NpContentBase64))
			npConentByteConvert := dbuf[:n]
			infoString = fmt.Sprintf("npConentByteConvert bytes is: [%+v]", npConentByteConvert)
			nputil.TraceInfo(infoString)
			TmpRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
			err = proto.Unmarshal(npConentByteConvert, TmpRbDomainPaths)
			if err != nil {
				infoString = fmt.Sprintf("TmpRbDomainPaths Proto unmarshal is failed!")
				nputil.TraceInfo(infoString)
				return nil
			}
			infoString = fmt.Sprintf("The Umarshal BindingSelectedDomainPath[%d] is: [%+v]", i, TmpRbDomainPaths)
			nputil.TraceInfoAlwaysPrint(infoString)
			rb.Spec.NetworkPath = append(rb.Spec.NetworkPath, NpContentBase64)
		}
	}
	nputil.TraceInfoEnd("")
	return rb
}

func networkFilterForRbs(rbs []*v1alpha1.ResourceBinding, networkReq *v1alpha1.NetworkRequirement) []*v1alpha1.ResourceBinding {
	nputil.TraceInfoBegin("")

	var selectedRbs []*v1alpha1.ResourceBinding
	for _, rb := range rbs {
		tmpRb := CalAppConnectAttrForRb(rb, *networkReq)
		if tmpRb != nil {
			selectedRbs = append(selectedRbs, tmpRb)
		} else {
			infoString := fmt.Sprintf("The rb (%+v) is unavailable!\n", rb)
			nputil.TraceInfo(infoString)
			for _, app := range rb.Spec.RbApps {
				infoString := fmt.Sprintf("The app of the unavailable rb is (%+v).\n", *app)
				nputil.TraceInfo(infoString)
			}
		}
		// 一个ResourceBing筛选完路径后，清除暂存的临时数据。
		ClearLocalComConnectArray()
	}
	for i, selectedRb := range selectedRbs {
		infoString := fmt.Sprintf("The selectedRbs[%d] is:  (%+v).\n", i, *selectedRb)
		nputil.TraceInfo(infoString)
	}
	nputil.TraceInfoEnd("")
	return selectedRbs
}
