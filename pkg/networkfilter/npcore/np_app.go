package npcore

import (
	"encoding/base64"
	"fmt"
	"github.com/golang/protobuf/proto"
	"github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	ncsnp "github.com/lmxia/gaia/pkg/networkfilter/model"
	"gonum.org/v1/gonum/graph/simple"

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
	DelayValue      uint32
	LostValue       uint32
	JitterValue     uint32
	ThroughputValue uint64
}

type AppConnectAttrKey struct {
	SrcUrl      string
	DstUrl      string
	SrcID       uint32
	DstID       uint32
	SrcDomainId uint32
	DstDomainId uint32
}

type AppConnectReqKey struct {
	SrcUrl string
	DstUrl string
}

type AppConnectAttr struct {
	Key     AppConnectAttrKey
	SlaAttr AppSlaAttr
}

//component的连接请求AppConnectReq
type AppConnectReq struct {
	Key                  AppConnectReqKey
	SlaAttr              AppSlaAttr
	ScnIdDomainPathGroup []ScnIdAppDomainPathGroup //数组的元素为：以特定SCN_ID为源的，且目的实例不在同一个filed的appConnect的：appConnect和其多路径
}

type AppConnect struct {
	AppConnect []AppConnectAttr
}

type APPComponent struct {
	Name              string
	SelfID            []string
	ScnIdInstList     map[string]ScnIdInstanceArray //k:scn_id, v: ScnIdInstance created by app location and replicas
	AppConnectReqList []AppConnectReq               //Component的interSCNID连接请求列表
	AppconnectList    []AppConnectAttr              //以本component的标识为source的APPconnect，以实例号区分
	FiledList         map[string]uint32             //Component可以部署的filed: k: filedName, v: replicas
	TotalReplicas     uint32                        //Component所有的副本数目
}

type ScnIdInstance struct {
	ScnId string
	Filed string
	Id    uint32
}

//指定标识scn_id的实例列表
type ScnIdInstanceArray struct {
	ScnIdInstList []ScnIdInstance
}

type DomainInfo struct {
	DomainName string
	DomainID   uint32
	DomainType uint32
}

//InterCommunication只选取一个副本的DomainSrPathArray
type AppConnectSelectedDomainPath struct {
	AppConnectAttr AppConnectAttr
	DomainInfoPath []DomainInfo
	DomainSrPath   DomainSrPath //预占时给控制器的DomainPath
}

type AppConnectDomainPathForRb struct {
	AppConnectAttr AppConnectAttr
	DomainPath     []DomainInfo
	DomainSrPath   DomainSrPath
}

//一种resourcebing的方案
type BindingSelectedDomainPath struct {
	SelectedDomainPath []AppConnectSelectedDomainPath
}

func getComponentReplicas(rbApp *v1alpha1.ResourceBindingApps, componentName string) uint32 {
	nputil.TraceInfoBegin("")

	var replicasVal int32
	if replicas, ok := rbApp.Replicas[componentName]; ok {
		fmt.Printf("The replicas of component (%s) is (%d)\n", componentName, replicas)
		replicasVal = replicas

	} else {
		fmt.Printf("The component (%s) doesn't exist!\n", componentName)
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
		fmt.Printf("The field(%s) doesn't have the component (%s)!\n", rbApp.ClusterName, componentName)
		infoString := fmt.Sprintf("The field(%s) doesn't have the component (%s)!\n", rbApp.ClusterName, componentName)
		nputil.TraceInfo(infoString)
		ret = false
	}
	nputil.TraceInfoEnd("")
	return fieldName, ret
}

//Map SCN_ID to Component name
func SetSelfId2Component(networkReq v1alpha1.NetworkRequirement) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()
	for _, netCom := range networkReq.Spec.NetworkCommunication {
		for _, selfId := range netCom.SelfID {
			local.selfId2Component[selfId] = netCom.Name
		}
	}
	fmt.Printf("local.selfId2Component is: (%+v).\n", local.selfId2Component)
	infoString := fmt.Sprintf("local.selfId2Component is: (%+v).\n", local.selfId2Component)
	nputil.TraceInfo(infoString)

	nputil.TraceInfoEnd("")
	return
}

func SetComponentByNetReq(networkReq v1alpha1.NetworkRequirement) {

	nputil.TraceInfoBegin("")

	for _, netCom := range networkReq.Spec.NetworkCommunication {
		local.ComponentArray[netCom.Name] = APPComponent{
			Name:              netCom.Name,
			SelfID:            netCom.SelfID,
			ScnIdInstList:     make(map[string]ScnIdInstanceArray),
			AppConnectReqList: []AppConnectReq{},
			AppconnectList:    []AppConnectAttr{},
			FiledList:         make(map[string]uint32),
			TotalReplicas:     0,
		}
	}
	//set the connection attributes for components
	SetAppConnectReqList(networkReq)

	for comName, Component := range local.ComponentArray {
		fmt.Printf("After set by networkReq, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
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
			fmt.Printf("filedName is [%s], local.ComponentArray[%s] is (%+v).\n", filedName, comName, local.ComponentArray[comName])
			infoString := fmt.Sprintf("filedName is [%s], local.ComponentArray[%s] is (%+v).", filedName, comName, local.ComponentArray[comName])
			nputil.TraceInfo(infoString)
			//component不存在
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
		fmt.Printf("After set by rb, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		infoString := fmt.Sprintf("After set by rb, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		nputil.TraceInfo(infoString)
	}

	nputil.TraceInfoEnd("")
	return
}

//set the connection attributes requirement for components
func SetAppConnectReqList(networkReq v1alpha1.NetworkRequirement) {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()

	for _, netCom := range networkReq.Spec.NetworkCommunication {
		comName := netCom.Name
		appComponent := local.ComponentArray[comName]
		appComponent.SelfID = netCom.SelfID
		for _, interSCNID := range netCom.InterSCNID {
			appReq := new(AppConnectReq)
			srcScnID := interSCNID.Source.Id
			dstScnID := interSCNID.Destination.Id
			appReq.Key.SrcUrl = srcScnID
			appReq.Key.DstUrl = dstScnID
			appReq.SlaAttr.DelayValue = uint32(interSCNID.Sla.Delay)
			appReq.SlaAttr.LostValue = uint32(interSCNID.Sla.Lost)
			appReq.SlaAttr.JitterValue = uint32(interSCNID.Sla.Jitter)
			appReq.SlaAttr.ThroughputValue = uint64(interSCNID.Sla.Bandwidth)
			appComponent.AppConnectReqList = append(appComponent.AppConnectReqList, *appReq)
		}
		local.ComponentArray[comName] = appComponent
	}

	for comName, Component := range local.ComponentArray {
		fmt.Printf("After set AppConnectReqList, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
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
	fmt.Printf("ScnIdInstList is: (%+v).\n", ScnIdInstList)
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
		fmt.Printf("local.ComponentArray[%s] is (%+v).\n", component.Name, local.ComponentArray[component.Name])
		tmpInstList[scnId] = ScnIdInstList
	}
	tmpComp.ScnIdInstList = tmpInstList
	local.ComponentArray[component.Name] = tmpComp

	fmt.Printf("local.ComponentArray[%s] is (%+v) after create ScnIdInstance.\n", component.Name, local.ComponentArray[component.Name])
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
// instances are initialized with src_scnid, srcDomain, instanceId
func SetScnIdInstances() {
	nputil.TraceInfoBegin("")
	local := GetCoreLocal()
	for _, component := range local.ComponentArray {
		SetScnIdInstanceForComponent(component)
	}

	for comName, Component := range local.ComponentArray {
		fmt.Printf("After set ScnIdInstances, the local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		infoString := fmt.Sprintf("local.ComponentArray[%s] is: (%+v).\n", comName, Component)
		nputil.TraceInfo(infoString)
	}
	nputil.TraceInfoEnd("")
}

func CreateAppConnectAttrByScnInst(srcInst ScnIdInstance, dstInst ScnIdInstance) *AppConnectAttr {
	nputil.TraceInfoBegin("")

	appConnect := new(AppConnectAttr)

	appConnect.Key.SrcUrl = srcInst.ScnId
	appConnect.Key.DstUrl = dstInst.ScnId
	appConnect.Key.SrcID = srcInst.Id
	appConnect.Key.DstID = dstInst.Id
	srcDomainId, flag := getDomainIDbyName(srcInst.Filed)
	if flag == false {
		fmt.Printf("The source domain (%s) doesn't exist!\n", srcInst.Filed)
		infoString := fmt.Sprintf("The source domain (%s) doesn't exist!\n", srcInst.Filed)
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return nil
	}
	dstDomainId, flag := getDomainIDbyName(dstInst.Filed)
	if flag == false {
		fmt.Printf("The source domain (%s) doesn't exist!\n", dstInst.Filed)
		infoString := fmt.Sprintf("The component (%s) doesn't exist!\n", dstInst.Filed)
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return nil
	}
	appConnect.Key.SrcDomainId = srcDomainId
	appConnect.Key.DstDomainId = dstDomainId
	AppReq, flag := getAppConnectionByScnId(srcInst.ScnId, dstInst.ScnId)
	if flag == false {
		fmt.Printf("The AppConnectReq cannot find by srcScnID(%+v) and dstInst(%+v)!\n", srcInst, dstInst)
		infoString := fmt.Sprintf("The AppConnectReq cannot find by srcScnID(%+v) and dstInst(%+v)!\n", srcInst, dstInst)
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return nil
	}
	appConnect.SlaAttr = AppReq.SlaAttr

	nputil.TraceInfoEnd("")
	return appConnect
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
		slaAttr.DelayValue = appConnectDomainPath.AppConnectAttr.SlaAttr.DelayValue
		slaAttr.LostValue = appConnectDomainPath.AppConnectAttr.SlaAttr.LostValue
		slaAttr.JitterValue = appConnectDomainPath.AppConnectAttr.SlaAttr.JitterValue
		slaAttr.ThroughputValue = appConnectDomainPath.AppConnectAttr.SlaAttr.ThroughputValue
		pbAppConnectAttr.SlaAttr = slaAttr

		pbAppConnectDomainPath.AppConnect = pbAppConnectAttr
		infoString := fmt.Sprintf("pbAppConnectDomainPath.AppConnect(%+v)!", pbAppConnectDomainPath.AppConnect)
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
	nputil.TraceInfoEnd("")
	for i, rbDomainPaths := range pbRbDomainPaths.SelectedDomainPath {
		infoString := fmt.Sprintf("pbRbDomainPaths[%d] is: (%+v).", i, rbDomainPaths)
		nputil.TraceInfo(infoString)
	}
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

//从各个连接属性的实例的pathgroup中各选出一个，组合成一组方案返回给resourceBinding
func CombMatrixToDomainPathGroup() [][]AppDomainPath {
	nputil.TraceInfoBegin("")
	local := GetCoreLocal()

	var appConnectDomainPathMatrix [][]AppDomainPath
	var appConnectDomainPathArray []AppDomainPath
	var combPath []AppDomainPath
	var combGroup = make([][]AppDomainPath, DomainPathGroupMaxNum)
	var index uint32

	//把所有components的所有连接属性的所有实例的单个domainpath定义为一个矩阵，
	//每一维度是:特定SrcUrl/DstUrl/srcId/DstId/srcField/dstField的具体实例对应的domainpath数组，且目的实例号不在同一个filed内
	//矩阵中的每个元素为：带实例号的AppConnect的单条domainPath
	for comName, component := range local.ComponentArray {
		//指定副本下的AppConnectReqList
		for i, appConnectReqList := range component.AppConnectReqList {
			//AppRequest下所有的副本实例的多条domainPath
			//清空内容
			fmt.Printf("component[%s].AppConnectReqList[%d] is: (%+v)\n", comName, i, appConnectReqList)
			//scnIdAppDomainPathGroup指定源连接属性的所有实例的domainPathGroup
			for _, scnIdAppDomainPathGroup := range appConnectReqList.ScnIdDomainPathGroup {
				//带实例号的连接属性的多条domainPath路径为一维数组
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
				//带实例号的AppConnect的domainSrPath matrix
				appConnectDomainPathMatrix = append(appConnectDomainPathMatrix, appConnectDomainPathArray)
			}
		}
	}
	for i, appConnectDomainPathArray := range appConnectDomainPathMatrix {
		fmt.Printf("The appConnectDomainPathMatrix[%d], is: (%+v)\n\n", i, appConnectDomainPathArray)
		infoString := fmt.Sprintf("appConnectDomainPathMatrix[%d], is: (%+v)\n\n", i, appConnectDomainPathArray)
		nputil.TraceInfo(infoString)
	}
	// Number of arrays
	n := len(appConnectDomainPathMatrix)
	if n == 0 {
		fmt.Printf("The InterScnIDDomainPathMatrix is empty!\n")
		infoString := fmt.Sprintf("The InterScnIDDomainPathMatrix is empty!\n")
		nputil.TraceInfo(infoString)
		nputil.TraceInfoEnd("")
		return combGroup
	}
	// To keep track of next element in each of the n arrays
	var indices = make([]int, n)
	// Initialize with first element's index
	for i := 0; i < n; i++ {
		indices[i] = 0
	}

	for {
		if index > DomainPathGroupMaxNum-1 {
			for _, combPath := range combGroup {
				fmt.Printf("The combPath is (%+v)!\n", combPath)
				infoString := fmt.Sprintf("The combPath is (%+v)!\n", combPath)
				nputil.TraceInfo(infoString)
			}
			nputil.TraceInfoEnd("")
			return combGroup
		}
		// combine domainpath for different interSCNID connections
		combPath = []AppDomainPath{}
		for j := 0; j < n; j++ {
			fmt.Printf("appConnectDomainPathMatrix[%d][indices[%d]] is (%+v)\n", j, j, appConnectDomainPathMatrix[j][indices[j]])
			combPath = append(combPath, appConnectDomainPathMatrix[j][indices[j]])
		}
		combGroup[index] = combPath
		index++
		// find the rightmost array that has more elements left after the current element in that array
		next := n - 1
		for next >= 0 && (indices[next]+1 >= len(appConnectDomainPathMatrix[next])) {
			next--
		}
		// no such array is found so no more combinations left
		if next < 0 {
			for _, combPath := range combGroup {
				fmt.Printf("The combPath is (%+v)!\n", combPath)
				infoString := fmt.Sprintf("The combPath is (%+v)!\n", combPath)
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

//假设连接属性的源srcScnId的component有5个副本，则每个副本实例在该连接属性都可达，才认为该连接属性有效。
//每个副本实例只要有一条可达实例就认为该副本的连接属性是有效的：即a1--b1可达就认为有效
//尽量满足负荷分担，如：a1--b1可达，那a2尝试从b2开始连接，如果到a2---b2……b5都不可达，则再从头计算a2-b1，截至的实例为上次循环的前一个实例b2-1=b1
func CalAppConnectAttrForInterSCNID(interSCNID v1alpha1.InterSCNID) bool {
	nputil.TraceInfoBegin("")

	local := GetCoreLocal()

	graph := GraphFindByGraphType(GraphTypeAlgo_Cspf)
	domainGraph := graph.DomainGraphPoint

	srcCom := local.selfId2Component[interSCNID.Source.Id]                                                  //源component
	dstCom := local.selfId2Component[interSCNID.Destination.Id]                                             //目的component
	srcScnIdInstList := local.ComponentArray[srcCom].ScnIdInstList[interSCNID.Source.Id].ScnIdInstList      //源定标识scn_id的实例列表
	dstScnIdInstList := local.ComponentArray[dstCom].ScnIdInstList[interSCNID.Destination.Id].ScnIdInstList //目的标识scn_id的实例列表

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
			appConnectAttr := CreateAppConnectAttrByScnInst(srcScnIdInstList[i], dstScnIdInstList[j])
			if appConnectAttr == nil {
				continue
			}
			fmt.Printf("srcScnIdInstList[%d] is: (%+v).\n dstScnIdInstList[%d] is: (%+v).\n appConnectAttr is: (%+v)",
				i, srcScnIdInstList[i], j, dstScnIdInstList[j], appConnectAttr)
			infoString := fmt.Sprintf("srcScnIdInstList[%d] is: (%+v).\n dstScnIdInstList[%d] is: (%+v).\n appConnectAttr is: (%+v)",
				i, srcScnIdInstList[i], j, dstScnIdInstList[j], appConnectAttr)
			nputil.TraceInfo(infoString)
			dstDuplicate := false

			//如果同一个filed内的目的实例已经计算过，则不在计算
			for _, tmpAppConnect := range ScnIdDomainPathGroup.AppDomainPathArray {
				if appConnectAttr.Key.SrcDomainId == tmpAppConnect.AppConnect.Key.SrcDomainId &&
					appConnectAttr.Key.DstDomainId == tmpAppConnect.AppConnect.Key.DstDomainId &&
					appConnectAttr.Key.SrcUrl == tmpAppConnect.AppConnect.Key.SrcUrl &&
					appConnectAttr.Key.DstUrl == tmpAppConnect.AppConnect.Key.DstUrl &&
					appConnectAttr.Key.SrcID == tmpAppConnect.AppConnect.Key.SrcID {
					dstDuplicate = true
					break
				}
			}
			if dstDuplicate == true {
				break
			}

			//源和目的在同一个filed,认为可达，返回所在filed
			if appConnectAttr.Key.SrcDomainId == appConnectAttr.Key.DstDomainId {
				domainSrPath := []DomainSrPath{
					0: DomainSrPath{
						DomainSidArray: []DomainSid{
							0: {DomainId: appConnectAttr.Key.SrcDomainId},
						},
					},
				}
				tmpAappDomainPath := AppDomainPathArray{}
				tmpAappDomainPath.DomainSrPathArray = domainSrPath
				tmpAappDomainPath.AppConnect = *appConnectAttr
				ScnIdDomainPathGroup.AppDomainPathArray = append(ScnIdDomainPathGroup.AppDomainPathArray, tmpAappDomainPath)
				break
			}

			//源实例标识和目的实例标识不在同一个field计算domainPath
			srcDomainSpfID, _ := getDomainSpfID(appConnectAttr.Key.SrcDomainId)
			dstDomainSpfID, _ := getDomainSpfID(appConnectAttr.Key.DstDomainId)
			query := simple.Edge{F: simple.Node(srcDomainSpfID), T: simple.Node(dstDomainSpfID)}
			//只计算以时延为权重的domainPath
			domainSrPathArray := graph.AppConnect(domainGraph.DomainLinkKspGraph, KspCalcMaxNum, query, *appConnectAttr)
			domainSrNamePath := graph.GetDomainPathNameArrayWithFaric(domainSrPathArray)
			fmt.Printf("domainSrNamePath is (%+v)\n", domainSrNamePath)
			infoString = fmt.Sprintf("domainSrNamePath is (%+v)\n", domainSrNamePath)
			nputil.TraceInfo(infoString)
			if len(domainSrPathArray) != 0 {
				fmt.Printf("AppConnect (%+v) domainSrPath are:%+v\n", *appConnectAttr, domainSrPathArray)
				appDomainPath := AppDomainPathArray{}
				appDomainPath.DomainSrPathArray = domainSrPathArray
				appDomainPath.AppConnect = *appConnectAttr
				//附加一个具体实例AppConnect的多条路径
				ScnIdDomainPathGroup.AppDomainPathArray = append(ScnIdDomainPathGroup.AppDomainPathArray, appDomainPath)
				fmt.Printf("ScnIdDomainPathGroup.AppDomainPathArray is (%+v)\n", ScnIdDomainPathGroup.AppDomainPathArray)
				infoString = fmt.Sprintf("ScnIdDomainPathGroup.AppDomainPathArray is (%+v)\n", ScnIdDomainPathGroup.AppDomainPathArray)
				nputil.TraceInfo(infoString)
			}
		}

		//判断该源副本实例是否存在可达路径
		//当每个副本都的链接属性都可达时，才认为该链接属性可达，只要有一个链接属性不可达，ResourceBinding方案无效
		if len(ScnIdDomainPathGroup.AppDomainPathArray) == 0 {
			fmt.Printf("The interCommunication (%+v) is unavailable!\n", interSCNID)
			infoString := fmt.Sprintf("The interCommunication (%+v) is unavailable!\n", interSCNID)
			nputil.TraceInfo(infoString)
			nputil.TraceInfoEnd("false")
			return false
		} else {
			//该副本实例存在可达路径，将该副本实例的与不同目的实例的AppConnect的domainSrPathArray 存放在相同源标识的local.ComponentArray[srcCom].AppConnectReqList下
			for i, tmpAppReq := range local.ComponentArray[srcCom].AppConnectReqList {
				if interSCNID.Source.Id == tmpAppReq.Key.SrcUrl && interSCNID.Destination.Id == tmpAppReq.Key.DstUrl {
					ScnIdDomainPathGroupArray = append(ScnIdDomainPathGroupArray, ScnIdDomainPathGroup)
					local.ComponentArray[srcCom].AppConnectReqList[i].ScnIdDomainPathGroup = ScnIdDomainPathGroupArray
					fmt.Printf("The local.ComponentArray[%s].AppConnectReqList[%d].ScnIdDomainPathGroupArray is (%+v)!\n",
						srcCom, i, ScnIdDomainPathGroupArray)
					infoString := fmt.Sprintf("The local.ComponentArray[%s].AppConnectReqList[%d].ScnIdDomainPathGroupArray is (%+v)!\n",
						srcCom, i, ScnIdDomainPathGroupArray)
					nputil.TraceInfo(infoString)
				}
			}
		}
	}

	infoString := fmt.Sprintf("The interCommunication (%+v) is available!\n", interSCNID)
	nputil.TraceInfo(infoString)
	nputil.TraceInfoEnd("true")
	return true
}

func CalAppConnectAttrForRb(rb *v1alpha1.ResourceBinding, networkReq v1alpha1.NetworkRequirement) *v1alpha1.ResourceBinding {
	nputil.TraceInfoBegin("")

	graph := GraphFindByGraphType(GraphTypeAlgo_Cspf)
	//set component map
	SetComponentByNetReq(networkReq)
	//Map SCN_ID to Component name
	SetSelfId2Component(networkReq)
	// set the filed location of component and ite replicas
	SetComponentByRb(*rb)
	// Set App connect instances for all components
	SetScnIdInstances()

	//Calc and check domainSrPathArray of all appConnect instances for all interCommunication
	// and store domainSrPathArray in
	var available bool
	available = true
	for _, netCom := range networkReq.Spec.NetworkCommunication {
		infoString := fmt.Sprintf("The netCom is (%+v)!\n", netCom)
		nputil.TraceInfo(infoString)
		for _, interSCNID := range netCom.InterSCNID {
			infoString := fmt.Sprintf("The netCom.InterSCNID is (%+v)!\n", interSCNID)
			nputil.TraceInfo(infoString)
			available = CalAppConnectAttrForInterSCNID(interSCNID)
			if available == false {
				infoString := fmt.Sprintf("The interComm (%+v) is unavailable!\n", interSCNID)
				nputil.TraceInfo(infoString)
				nputil.TraceInfoEnd("")
				return nil
			}
		}
	}

	//选出DomainPathGroupMaxNum组组合方案
	domainPathCluster := CombMatrixToDomainPathGroup()
	if len(domainPathCluster) != 0 {
		var rbDomainPathArray []BindingSelectedDomainPath
		for _, appPathGroup := range domainPathCluster {
			var rbSdp = BindingSelectedDomainPath{}
			for i, appDomainPath := range appPathGroup {
				infoString := fmt.Sprintf("appPathGroup[%d] is: (%+v).\n\n", i, appDomainPath)
				nputil.TraceInfo(infoString)
				var appSelectedPath = AppConnectSelectedDomainPath{}
				appSelectedPath.AppConnectAttr = appDomainPath.AppConnect
				appSelectedPath.DomainSrPath = appDomainPath.DomainSrPath
				appSelectedPath.DomainInfoPath = graph.GetDomainPathNameWithFaric(appDomainPath.DomainSrPath)
				rbSdp.SelectedDomainPath = append(rbSdp.SelectedDomainPath, appSelectedPath)
			}
			infoString := fmt.Sprintf("rbSdp : (%+v).\n\n", rbSdp)
			nputil.TraceInfo(infoString)
			if len(rbSdp.SelectedDomainPath) != 0 {
				rbDomainPathArray = append(rbDomainPathArray, rbSdp)
			}
		}
		//构造message数据
		for i, bindDomainPath := range rbDomainPathArray {
			fmt.Printf("---------bindDomainPath SelectedDomainPath [%d] ---------\n", i)
			infoString := fmt.Sprintf("---------bindDomainPath SelectedDomainPath [%d] ---------\n", i)
			nputil.TraceInfo(infoString)
			for _, appDomainPath := range bindDomainPath.SelectedDomainPath {
				fmt.Printf("AppConnectAttr is: [%+v]\n\n", appDomainPath.AppConnectAttr)
				fmt.Printf("DomainSrPath is: [%+v]\n\n", appDomainPath.DomainSrPath)
				fmt.Printf("DomainInfoPath is: [%+v]\n\n", appDomainPath.DomainInfoPath)
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
			infoString = fmt.Sprintf("Proto marshal content base64 is: [%+v], stringbase64 is (%+v)", NpContentBase64, string(NpContentBase64))
			nputil.TraceInfo(infoString)
			rb.Spec.NetworkPath = append(rb.Spec.NetworkPath, NpContentBase64)

			//test
			dbuf := make([]byte, base64.StdEncoding.DecodedLen(len(NpContentBase64)))
			n, _ := base64.StdEncoding.Decode(dbuf, []byte(NpContentBase64))
			npConentByteConvert := dbuf[:n]
			infoString = fmt.Sprintf("npConentByteConvert bytes is: [%+v]", npConentByteConvert)
			nputil.TraceInfo(infoString)

			TmpRbDomainPaths := new(ncsnp.BindingSelectedDomainPath)
			err = proto.Unmarshal(content, TmpRbDomainPaths)
			infoString = fmt.Sprintf("TmpRbDomainPaths is: [%+v]", TmpRbDomainPaths)
			nputil.TraceInfo(infoString)
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
		//一个ResourceBing筛选完路径后，清除暂存的临时数据。
		ClearLocalComConnectArray()
	}
	for i, selectedRb := range selectedRbs {
		infoString := fmt.Sprintf("The selectedRbs[%d] is:  (%+v).\n", i, *selectedRb)
		nputil.TraceInfo(infoString)
	}
	nputil.TraceInfoEnd("")
	return selectedRbs
}
