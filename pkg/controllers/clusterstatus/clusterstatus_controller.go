package clusterstatus

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lmxia/gaia/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	corev1lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	utilpointer "k8s.io/utils/pointer"

	hypernodeclientset "github.com/SUMMERLm/hyperNodes/pkg/generated/clientset/versioned"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	known "github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/clusterstatus/toposync"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	gaialister "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"
	"github.com/prometheus/common/model"
)

// Controller is a controller that collects cluster status
type Controller struct {
	kubeClient             kubernetes.Interface
	lock                   *sync.Mutex
	clusterStatus          *clusterapi.ManagedClusterStatus
	collectingPeriod       time.Duration
	heartbeatFrequency     time.Duration
	apiserverURL           string
	managedClusterSource   string
	promURLPrefix          string
	topoSyncBaseURL        string
	useHypernodeController bool
	mclsLister             gaialister.ManagedClusterLister
	nodeLister             corev1lister.NodeLister
	hypernodeClient        *hypernodeclientset.Clientset
	nodeSynced             cache.InformerSynced
	podLister              corev1lister.PodLister
	podSynced              cache.InformerSynced
	clusterName            string
}

func NewController(ctx context.Context, apiserverURL, clusterName string,
	managedCluster *clusterapi.ManagedClusterOptions, kubeClient kubernetes.Interface,
	gaiaClient *gaiaclientset.Clientset, hypernodeClient *hypernodeclientset.Clientset, collectingPeriod time.Duration,
	heartbeatFrequency time.Duration,
) *Controller {
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, known.DefaultResync)
	// add informers
	kubeInformerFactory.Core().V1().Nodes().Informer()
	kubeInformerFactory.Core().V1().Pods().Informer()
	kubeInformerFactory.Start(ctx.Done())

	gaiaInformerFactory := gaiainformers.NewSharedInformerFactory(gaiaClient, known.DefaultResync)
	gaiaInformerFactory.Platform().V1alpha1().ManagedClusters().Informer()
	gaiaInformerFactory.Start(ctx.Done())

	return &Controller{
		kubeClient:             kubeClient,
		lock:                   &sync.Mutex{},
		collectingPeriod:       collectingPeriod,
		heartbeatFrequency:     heartbeatFrequency,
		apiserverURL:           apiserverURL,
		managedClusterSource:   managedCluster.ManagedClusterSource,
		promURLPrefix:          managedCluster.PrometheusMonitorURLPrefix,
		topoSyncBaseURL:        managedCluster.TopoSyncBaseURL,
		mclsLister:             gaiaInformerFactory.Platform().V1alpha1().ManagedClusters().Lister(),
		nodeLister:             kubeInformerFactory.Core().V1().Nodes().Lister(),
		hypernodeClient:        hypernodeClient,
		useHypernodeController: managedCluster.UseHypernodeController,
		nodeSynced:             kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		podLister:              kubeInformerFactory.Core().V1().Pods().Lister(),
		podSynced:              kubeInformerFactory.Core().V1().Pods().Informer().HasSynced,
		clusterName:            clusterName,
	}
}

func (c *Controller) Run(ctx context.Context) {
	if !cache.WaitForNamedCacheSync("cluster-status-controller", ctx.Done(),
		c.podSynced,
		c.nodeSynced,
	) {
		return
	}

	wait.UntilWithContext(ctx, c.collectingClusterStatus, c.collectingPeriod)
}

func (c *Controller) collectingClusterStatus(ctx context.Context) {
	klog.V(7).Info("collecting cluster status...")
	clusterVersion, err := c.getKubernetesVersion(ctx)
	if err != nil {
		klog.Warningf("failed to collect kubernetes version: %v", err)
	}

	clusters, err := c.mclsLister.List(labels.Everything())
	if err != nil {
		klog.Warningf("failed to list clusters: %v", err)
	}

	var nodeStatistics clusterapi.NodeStatistics
	var capacity, allocatable, available corev1.ResourceList
	var topoInfo clusterapi.Topo
	var nodesAllRes map[string]clusterapi.NodeResources
	if len(clusters) == 0 {
		klog.V(7).Info("no joined clusters, collecting cluster resources...")
		nodes, err2 := c.nodeLister.List(labels.Everything())
		if err2 != nil {
			klog.Warningf("failed to list nodes: %v", err2)
		}

		nodeStatistics = getNodeStatistics(nodes)
		if c.managedClusterSource == known.ManagedClusterSourceFromInformer {
			capacity, allocatable, available = getNodeResource(nodes)
			nodesAllRes = getEachNodeResourceFromPrometheus("", nodes)
		} else if c.managedClusterSource == known.ManagedClusterSourceFromPrometheus {
			capacity, allocatable, available = getNodeResourceFromPrometheus(c.promURLPrefix)
			nodesAllRes = getEachNodeResourceFromPrometheus(c.promURLPrefix, nodes)
		}
	} else {
		klog.V(5).Info("collecting ManagedCluster status...")

		nodeStatistics = getManagedClusterNodeStatistics(clusters)
		capacity, allocatable, available = getManagedClusterResource(clusters)
		nodesAllRes = getManagedClusterAllResource(clusters)

		selfClusterName, _, errClusterName := utils.GetLocalClusterName(c.kubeClient.(*kubernetes.Clientset))
		if errClusterName != nil {
			klog.Warningf("failed to get self clusterName from secret: %v", errClusterName)
			selfClusterName = c.clusterName
		}
		topoInfo = getTopoInfo(ctx, selfClusterName, c.topoSyncBaseURL)
	}

	clusterCIDR, err := c.discoverClusterCIDR()
	if err != nil {
		klog.Warningf("failed to discover cluster CIDR: %v", err)
	}

	serviceCIDR, err := c.discoverServiceCIDR()
	if err != nil {
		klog.V(2).ErrorS(err, "failed to discover service CIDR")
	}

	var status clusterapi.ManagedClusterStatus
	status.KubernetesVersion = clusterVersion.GitVersion
	status.Platform = clusterVersion.Platform
	status.APIServerURL = c.apiserverURL
	status.Healthz = c.getHealthStatus(ctx, "/healthz")
	status.Livez = c.getHealthStatus(ctx, "/livez")
	status.Readyz = c.getHealthStatus(ctx, "/readyz")
	status.ClusterCIDR = clusterCIDR
	status.ServiceCIDR = serviceCIDR
	status.NodeStatistics = nodeStatistics
	status.Allocatable = allocatable
	status.Capacity = capacity
	status.Available = available
	status.NodesResources = nodesAllRes
	status.HeartbeatFrequencySeconds = utilpointer.Int64(int64(c.heartbeatFrequency.Seconds()))
	status.Conditions = []metav1.Condition{c.getCondition(status)}
	status.TopologyInfo = topoInfo
	c.setClusterStatus(status)
}

func (c *Controller) setClusterStatus(status clusterapi.ManagedClusterStatus) {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.clusterStatus == nil {
		c.clusterStatus = new(clusterapi.ManagedClusterStatus)
	}

	c.clusterStatus = &status
	c.clusterStatus.LastObservedTime = metav1.Now()
	klog.V(7).Infof("current cluster status is %#v", status)
}

func (c *Controller) GetClusterStatus() *clusterapi.ManagedClusterStatus {
	c.lock.Lock()
	defer c.lock.Unlock()

	if c.clusterStatus == nil {
		return nil
	}

	return c.clusterStatus.DeepCopy()
}

func (c *Controller) getKubernetesVersion(_ context.Context) (*version.Info, error) {
	return c.kubeClient.Discovery().ServerVersion()
}

func (c *Controller) getHealthStatus(ctx context.Context, path string) bool {
	var statusCode int
	c.kubeClient.Discovery().RESTClient().Get().AbsPath(path).Do(ctx).StatusCode(&statusCode)
	return statusCode == http.StatusOK
}

// getTopoInfo returns the topology information according to toposync api
func getTopoInfo(ctx context.Context, clusterName, topoSyncBaseURL string) (topoInfo clusterapi.Topo) {
	hyperTopoSync := clusterapi.Fields{
		Field: []string{clusterName},
	}
	topos, body, err := toposync.NewAPIClient(toposync.NewConfiguration(topoSyncBaseURL)).
		TopoSyncAPI.TopoSync(ctx, hyperTopoSync)
	if body != nil {
		defer body.Body.Close()
	}
	if err != nil {
		klog.Warningf("failed to get network topology info: %v", err)
		return topoInfo
	}
	topoInfo = topos.Topo[0]
	return topoInfo
}

func (c *Controller) getCondition(status clusterapi.ManagedClusterStatus) metav1.Condition {
	if status.Livez && status.Readyz {
		return metav1.Condition{
			Type:               clusterapi.ClusterReady,
			Status:             metav1.ConditionTrue,
			LastTransitionTime: metav1.Now(),
			Reason:             "ManagedClusterReady",
			Message:            "managed cluster is ready.",
		}
	}

	return metav1.Condition{
		Type:               clusterapi.ClusterReady,
		Status:             metav1.ConditionFalse,
		LastTransitionTime: metav1.Now(),
		Reason:             "ManagedClusterNotReady",
		Message:            "managed cluster is not ready.",
	}
}

// getNodeStatistics returns the NodeStatistics in the cluster
// get nodes num in different conditions
func getNodeStatistics(nodes []*corev1.Node) (nodeStatistics clusterapi.NodeStatistics) {
	for _, node := range nodes {
		flag, condition := getNodeCondition(&node.Status, corev1.NodeReady)
		if flag == -1 {
			nodeStatistics.LostNodes += 1
			continue
		}

		switch condition.Status {
		case corev1.ConditionTrue:
			nodeStatistics.ReadyNodes += 1
		case corev1.ConditionFalse:
			nodeStatistics.NotReadyNodes += 1
		case corev1.ConditionUnknown:
			nodeStatistics.UnknownNodes += 1
		}
	}
	return
}

// getManagedClusterNodeStatistics returns the sum of the ManagedClusters' NodeStatistics in the cluster
func getManagedClusterNodeStatistics(clusters []*clusterapi.ManagedCluster) (nodeStatistics clusterapi.NodeStatistics) {
	nodeStatistics = clusterapi.NodeStatistics{}
	for _, cluster := range clusters {
		nodeStatistics.LostNodes += cluster.Status.NodeStatistics.LostNodes
		nodeStatistics.ReadyNodes += cluster.Status.NodeStatistics.ReadyNodes
		nodeStatistics.NotReadyNodes += cluster.Status.NodeStatistics.NotReadyNodes
		nodeStatistics.UnknownNodes += cluster.Status.NodeStatistics.UnknownNodes
	}
	return
}

// getNodeLabels returns the specified node labels in the cluster
func getNodeLabels(nodes []*corev1.Node) (nodeLabels map[string]string) {
	nodeLabels = make(map[string]string)

	for _, node := range nodes {
		// get worker nodes' labels whose "NodeRole" is not "System"
		if value, ok := node.GetLabels()[clusterapi.ParsedNodeRoleKey]; ok && value != "System" {
			nodeLabels = parseNodeLabels(nodeLabels, node.GetLabels())
		}
	}
	return nodeLabels
}

// parseNodeLabels returns the nodeLabels that belong to specific string list.
func parseNodeLabels(nodeLabels, inLabels map[string]string) map[string]string {
	sn := inLabels[clusterapi.ParsedSNKey]
	for labelKey, labelValue := range inLabels {
		if len(labelValue) > 0 {
			if strings.HasPrefix(labelKey, clusterapi.ParsedGPUCountKey) {
				if val, ok := nodeLabels[labelKey]; ok {
					oriNum, err := strconv.Atoi(labelValue)
					valNum, err2 := strconv.Atoi(val)
					if err != nil || err2 != nil {
						klog.Errorf("failed to get gpuCount, gpu-product==%q, Error==%v+%v", labelKey, err, err2)
					} else {
						nodeLabels[labelKey] = strconv.Itoa(oriNum + valNum)
					}
				} else {
					nodeLabels[labelKey] = labelValue
				}
			}
			if labelKey == clusterapi.ParsedNetEnvironmentKey || labelKey == clusterapi.ParsedNodeRoleKey ||
				labelKey == clusterapi.ParsedResFormKey || labelKey == clusterapi.ParsedRuntimeStateKey {
				if _, ok := nodeLabels[labelKey]; ok {
					existedLabelValueArray := strings.Split(nodeLabels[labelKey], "__")
					if !utils.ContainsString(existedLabelValueArray, labelValue) {
						nodeLabels[labelKey] = nodeLabels[labelKey] + "__" + labelValue
					}
				} else {
					nodeLabels[labelKey] = labelValue
				}
			} else if utils.ContainsString(clusterapi.ParsedHypernodeLabelKeyList, labelKey) {
				nodeLabels[labelKey+"__"+sn] = labelValue
			}
		}
	}
	return nodeLabels
}

// parseHypernodeLabels returns the HypernodeLabels that belong to specific string list.
func parseHypernodeLabels(nodeLabels, inLabels map[string]string) map[string]string {
	sn := inLabels[clusterapi.SNKey]
	for labelKey, labelValue := range inLabels {
		if len(labelValue) > 0 {
			if labelKey == clusterapi.NetEnvironmentKey || labelKey == clusterapi.NodeRoleKey ||
				labelKey == clusterapi.ResFormKey || labelKey == clusterapi.RuntimeStateKey {
				if _, ok := nodeLabels[clusterapi.HypernodeLabelKeyToStandardLabelKey[labelKey]]; ok {
					existedLabelValueArray := strings.Split(
						nodeLabels[clusterapi.HypernodeLabelKeyToStandardLabelKey[labelKey]], "__")
					if !utils.ContainsString(existedLabelValueArray, labelValue) {
						nodeLabels[clusterapi.HypernodeLabelKeyToStandardLabelKey[labelKey]] =
							nodeLabels[clusterapi.HypernodeLabelKeyToStandardLabelKey[labelKey]] + "__" + labelValue
					}
				} else {
					nodeLabels[clusterapi.HypernodeLabelKeyToStandardLabelKey[labelKey]] = labelValue
				}
			} else if utils.ContainsString(clusterapi.HypernodeLabelKeyList, labelKey) {
				nodeLabels[clusterapi.HypernodeLabelKeyToStandardLabelKey[labelKey]+"__"+sn] = labelValue
			}
		}
	}
	return nodeLabels
}

// getClusterLabels returns the specified node labels from its sub clusters
func getClusterLabels(clusters []*clusterapi.ManagedCluster) (nodeLabels map[string]string) {
	nodeLabels = make(map[string]string)
	for _, cluster := range clusters {
		for labelKey, labelValue := range cluster.GetLabels() {
			if len(labelValue) > 0 {
				if strings.HasPrefix(labelKey, clusterapi.ParsedGPUCountKey) {
					if val, ok := nodeLabels[labelKey]; ok {
						oriNum, err := strconv.Atoi(labelValue)
						valNum, err2 := strconv.Atoi(val)
						if err != nil || err2 != nil {
							klog.Errorf("failed to get gpuCount, gpu-product==%q, Error==%v+%v", labelKey, err, err2)
						} else {
							nodeLabels[labelKey] = strconv.Itoa(oriNum + valNum)
						}
					}
				}
				if labelKey == clusterapi.ParsedNetEnvironmentKey || labelKey == clusterapi.ParsedNodeRoleKey ||
					labelKey == clusterapi.ParsedResFormKey || labelKey == clusterapi.ParsedRuntimeStateKey {
					if _, ok := nodeLabels[labelKey]; ok {
						existedLabelValueArray := strings.Split(nodeLabels[labelKey], "__")
						if !utils.ContainsString(existedLabelValueArray, labelValue) {
							nodeLabels[labelKey] = nodeLabels[labelKey] + "__" + labelValue
						}
					} else {
						nodeLabels[labelKey] = labelValue
					}
				} else if strings.HasPrefix(labelKey, known.SpecificNodeLabelsKeyPrefix) {
					nodeLabels[labelKey] = labelValue
				}
			}
		}
	}
	return nodeLabels
}

// GetManagedClusterLabels returns the specified node labels for the clusters
func (c *Controller) GetManagedClusterLabels() (nodeLabels map[string]string) {
	nodeLabels = make(map[string]string)
	if c.useHypernodeController {
		// discard
		hypernodeList, err := c.hypernodeClient.ClusterV1alpha1().Hypernodes(metav1.NamespaceDefault).
			List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Warningf("failed to list hypernodes: %v", err)
		}

		for _, hypernode := range hypernodeList.Items {
			// get hypernodes' labels that are in Cluster level
			// only the worker node labels whose "NodeRole" is not "System"
			if value, ok := hypernode.GetLabels()[clusterapi.NodeRoleKey]; ok && value != "System" {
				if hypernode.Spec.NodeAreaType == known.ClusterLayer {
					nodeLabels = parseHypernodeLabels(nodeLabels, hypernode.GetLabels())
				}
			}
		}
	} else {
		// get clusters
		clusters, err := c.mclsLister.List(labels.Everything())
		if err != nil {
			klog.Warningf("failed to list clusters: %v", err)
		}
		if len(clusters) == 0 {
			// get nodes
			nodes, err := c.nodeLister.List(labels.Everything())
			if err != nil {
				klog.Warningf("failed to list nodes: %v", err)
				return nodeLabels
			}
			readyNodes := make([]*corev1.Node, 0, len(nodes))
			for _, node := range nodes {
				if utils.IsNodeReady(node) {
					readyNodes = append(readyNodes, node)
				}
			}
			nodeLabels = getNodeLabels(readyNodes)
		} else {
			nodeLabels = getClusterLabels(clusters)
		}
	}
	return nodeLabels
}

// discoverServiceCIDR returns the service CIDR for the cluster.
func (c *Controller) discoverServiceCIDR() (string, error) {
	return findPodIPRange(c.nodeLister, c.podLister)
}

// discoverClusterCIDR returns the cluster CIDR for the cluster.
func (c *Controller) discoverClusterCIDR() (string, error) {
	return findClusterIPRange(c.podLister)
}

// get node capacity and allocatable resource
func getNodeResource(nodes []*corev1.Node) (capacity, allocatable, available corev1.ResourceList) {
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem resource.Quantity
	capacity, allocatable, available = make(map[corev1.ResourceName]resource.Quantity),
		make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)

	for _, node := range nodes {
		capacityCPU.Add(*node.Status.Capacity.Cpu())
		capacityMem.Add(*node.Status.Capacity.Memory())
		allocatableCPU.Add(*node.Status.Allocatable.Cpu())
		allocatableMem.Add(*node.Status.Allocatable.Memory())
	}

	capacity[corev1.ResourceCPU] = capacityCPU
	capacity[corev1.ResourceMemory] = capacityMem
	allocatable[corev1.ResourceCPU] = allocatableCPU
	allocatable[corev1.ResourceMemory] = allocatableMem
	available[corev1.ResourceCPU] = allocatableCPU
	available[corev1.ResourceMemory] = allocatableMem

	return
}

// getManagedClusterResource gets the node capacity of all managedClusters and their allocatable resources
func getManagedClusterResource(clusters []*clusterapi.ManagedCluster) (capacity, allocatable,
	available corev1.ResourceList,
) {
	capacity, allocatable, available = make(map[corev1.ResourceName]resource.Quantity),
		make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem, availableCPU, availableMem resource.Quantity
	for _, cluster := range clusters {
		capacityCPU.Add(cluster.Status.Capacity[corev1.ResourceCPU])
		capacityMem.Add(cluster.Status.Capacity[corev1.ResourceMemory])
		allocatableCPU.Add(cluster.Status.Allocatable[corev1.ResourceCPU])
		allocatableMem.Add(cluster.Status.Allocatable[corev1.ResourceMemory])
		availableCPU.Add(cluster.Status.Available[corev1.ResourceCPU])
		availableMem.Add(cluster.Status.Available[corev1.ResourceMemory])
	}
	capacity[corev1.ResourceCPU] = capacityCPU
	capacity[corev1.ResourceMemory] = capacityMem
	allocatable[corev1.ResourceCPU] = allocatableCPU
	allocatable[corev1.ResourceMemory] = allocatableMem
	available[corev1.ResourceCPU] = availableCPU
	available[corev1.ResourceMemory] = availableMem
	return
}

func getManagedClusterAllResource(clusters []*clusterapi.ManagedCluster) map[string]clusterapi.NodeResources {
	allNodeResourcesMap := make(map[string]clusterapi.NodeResources)
	for _, cluster := range clusters {
		for k, v := range cluster.Status.NodesResources {
			allNodeResourcesMap[k] = v
		}
		// maps.Copy(allNodeResourcesMap, cluster.Status.NodesResources)
	}

	return allNodeResourcesMap
}

// getNodeCondition returns the specified condition from node's status
// Copied from k8s.io/kubernetes/pkg/controller/util/node/controller_utils.go and make some modifications
func getNodeCondition(status *corev1.NodeStatus, conditionType corev1.NodeConditionType) (int, *corev1.NodeCondition) {
	if status == nil {
		return -1, nil
	}
	for i := range status.Conditions {
		if status.Conditions[i].Type == conditionType {
			return i, &status.Conditions[i]
		}
	}
	return -1, nil
}

// getNodeResourceFromPrometheus returns the cpu and memory resources from Prometheus in the cluster
func getNodeResourceFromPrometheus(promPreURL string) (capacity, allocatable, available corev1.ResourceList) {
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem, availableCPU, availableMem resource.Quantity
	capacity, allocatable, available = make(map[corev1.ResourceName]resource.Quantity),
		make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)
	var valueList [6]string

	QueryMetricSet, ClusterMetricList, err := utils.InitConfig(known.MetricConfigMapAbsFilePath)
	if err == nil && len(ClusterMetricList) == 6 {
		for index, metric := range ClusterMetricList[:6] {
			result, err2 := utils.GetDataFromPrometheus(promPreURL, QueryMetricSet[metric])
			if err2 == nil {
				if len(result.(model.Vector)) == 0 {
					klog.Warningf("Query from prometheus successfully, but the result is a null array.")
					valueList[index] = "0"
				} else {
					valueList[index] = result.(model.Vector)[0].Value.String()
				}
			} else {
				valueList[index] = "0"
			}
		}
	} else {
		if err != nil {
			klog.Warningf("Wrong metrics, err: %v", err)
		} else {
			klog.Warningf("The length of ClusterMetricList is %v, it is less then 6. Wrong metrics %v",
				len(ClusterMetricList), QueryMetricSet)
		}
		valueList = [6]string{"0", "0", "0", "0", "0", "0"}
	}

	capacityCPU.Add(resource.MustParse(valueList[0]))
	capacityMem.Add(resource.MustParse(valueList[1] + "Ki"))
	allocatableCPU.Add(resource.MustParse(valueList[2]))
	allocatableMem.Add(resource.MustParse(valueList[3] + "Ki"))
	availableCPU.Add(resource.MustParse(getSubStringWithSpecifiedDecimalPlace(valueList[4], 3)))
	availableMem.Add(resource.MustParse(valueList[5] + "Ki"))

	capacity[corev1.ResourceCPU] = capacityCPU
	capacity[corev1.ResourceMemory] = capacityMem
	allocatable[corev1.ResourceCPU] = allocatableCPU
	allocatable[corev1.ResourceMemory] = allocatableMem
	available[corev1.ResourceCPU] = availableCPU
	available[corev1.ResourceMemory] = availableMem

	return capacity, allocatable, available
}

// getEachNodeResourceFromPrometheus returns the cpu, gpu, memory resources from Prometheus in the cluster
func getEachNodeResourceFromPrometheus(promPreURL string, nodes []*corev1.Node) map[string]clusterapi.NodeResources {
	nodeResourcesMap := make(map[string]clusterapi.NodeResources)

	// 筛选节点
	filteredNodes := make([]*corev1.Node, 0, len(nodes))
	for _, node := range nodes {
		sn := utils.GetNodeSNID(node)
		if sn == "" || !utils.IsNodeReady(node) || utils.IsSystemNode(node) {
			continue
		}
		filteredNodes = append(filteredNodes, node)
	}

	if promPreURL == "" {
		var nodeResource clusterapi.NodeResources
		for _, node := range filteredNodes {
			sn := utils.GetNodeSNID(node)
			nodeResourcesMap[sn] = nodeResource
		}
		klog.Errorf("cat not getEachNodeResourceFromPrometheus, mcSource of gaia should be prometheus")
		return nodeResourcesMap
	}

	QueryMetricSet, metricKeyList, err := utils.InitConfig(known.MetricConfigMapNodeAbsFilePath)
	if err != nil || len(metricKeyList) < 5 {
		klog.Warningf("Wrong metrics config or The length of ClusterMetricList not 4, err: %v", err)
		return nil
	}

	for _, node := range filteredNodes {
		var nodeResource clusterapi.NodeResources
		var valueList [4]string
		nodeResource.ClusterName = utils.GetNodeClusterName(node)
		sn := utils.GetNodeSNID(node)
		for index, metric := range metricKeyList[:4] {
			queryStr := QueryMetricSet[metric]
			queryStr = strings.ReplaceAll(queryStr, "XXX", sn)
			result, err2 := utils.GetDataFromPrometheus(promPreURL, queryStr)
			if err2 == nil {
				if len(result.(model.Vector)) == 0 {
					klog.V(2).Infof("Query %s from prometheus successfully, but the result is a null array, nodeName==%q",
						metric, node.GetName())
					valueList[index] = "0"
				} else {
					valueList[index] = result.(model.Vector)[0].Value.String()
				}
			} else {
				valueList[index] = "0"
			}
		}

		// todo: Judge whether it is an entity node
		nodeResource.Capacity, nodeResource.Available, nodeResource.AvailableGPU =
			make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity),
			make(map[string]int)
		nodeResource.Capacity[corev1.ResourceCPU] = resource.MustParse(valueList[0])
		nodeResource.Capacity[corev1.ResourceMemory] = resource.MustParse(valueList[1] + "Ki")
		nodeResource.Available[corev1.ResourceCPU] = resource.MustParse(getSubStringWithSpecifiedDecimalPlace(
			valueList[2], 3))
		nodeResource.Available[corev1.ResourceMemory] = resource.MustParse(valueList[3] + "Ki")

		// get gpu resource
		gpuMap := make(map[string]int)
		for labelKey := range node.GetLabels() {
			if strings.HasPrefix(labelKey, clusterapi.ParsedGPUCountKey) {
				gpuProduct := labelKey[len(clusterapi.ParsedGPUCountKey):]
				if len(gpuProduct) > 0 {
					queryStr := QueryMetricSet[metricKeyList[4]]
					queryStr = strings.ReplaceAll(queryStr, "XXX", sn)
					queryStr = strings.ReplaceAll(queryStr, "GPU_PRODUCT", gpuProduct)
					result, err2 := utils.GetDataFromPrometheus(promPreURL, queryStr)
					if err2 == nil {
						if len(result.(model.Vector)) == 0 {
							klog.Warningf("Query from prometheus successfully, but the result is a null array.")
							gpuMap[gpuProduct] = 0
						} else {
							countStr := result.(model.Vector)[0].Value.String()
							count, errC := strconv.Atoi(countStr)
							if errC != nil {
								klog.Errorf("failed to get gpuCount, gpu-product==%q, Error==%v", gpuProduct, errC)
							} else {
								klog.V(2).Infof("node==%q get GPU:gpuCount==%q:%d",
									node.GetName(), gpuProduct, count)
								gpuMap[gpuProduct] = count
							}
						}
					} else {
						gpuMap[gpuProduct] = 0
					}
				}
			}
		}
		nodeResource.AvailableGPU = gpuMap
		nodeResourcesMap[sn] = nodeResource
	}

	return nodeResourcesMap
}

// getSubStringWithSpecifiedDecimalPlace returns a sub string based on the specified number of decimal places
func getSubStringWithSpecifiedDecimalPlace(inputString string, m int) string {
	if inputString == "" {
		return ""
	}
	if m >= len(inputString) {
		return inputString
	}
	newString := strings.Split(inputString, ".")
	if len(newString) < 2 || m >= len(newString[1]) {
		return inputString
	}
	return newString[0] + "." + newString[1][:m]
}

// DescComMap declares a map whose keys are the components' names and the values are null structs
type DescComMap map[string]struct{}

// DescNameMap declares a map whose keys are the descriptions' names and the values are the DescComMap
type DescNameMap map[string]DescComMap

// GetDescNameFromAbnormalPod returns a map whose keys are the descriptions' name
// Those descriptions' pods are abnormal according to the specified metrics
func (c *Controller) GetDescNameFromAbnormalPod() (descNameMap DescNameMap) {
	descNameMap = make(DescNameMap)
	// get metricPsql from the config map file
	metricMap, _, err := utils.InitConfig(known.ServiceMaintenanceConfigMapAbsFilePath)
	if err != nil {
		klog.Warningf("Wrong metrics, err: %v", err)
		return descNameMap
	}

	for _, metricPsql := range metricMap {
		pendingLatencyResult, err := utils.GetDataFromPrometheus(c.promURLPrefix, metricPsql)
		if err != nil {
			klog.Warningf("Query failed from prometheus, err is %v. The metric is %v", err, metricPsql)
			return descNameMap
		}
		resultList := pendingLatencyResult.(model.Vector)
		if len(resultList) > 0 {
			for _, result := range resultList {
				podNamespace := result.Metric["destination_namespace"]
				podName := result.Metric["destination_pod"]
				podUID := result.Metric["destination_pod_uid"]
				podDescName := result.Metric["destination_pod_description_name"]
				podComName := result.Metric["destination_pod_component_name"]
				klog.V(5).InfoS("pod is abnormal according to the metric: ", "podNamespace",
					podNamespace, "podName", podName, "podUID", podUID, "metricPsql", metricPsql)
				if comMap, ok := descNameMap[string(podDescName)]; ok {
					if _, exist := comMap[string(podComName)]; !exist {
						comMap[string(podComName)] = struct{}{}
					}
				} else {
					descNameMap[string(podDescName)] = make(map[string]struct{})
				}
			}
		} else {
			klog.V(5).Infof("the query result of metricPsql(%v) is a null array.", metricPsql)
		}
	}
	return descNameMap
}

// IsParentCluster return whether it is a parent cluster
func (c *Controller) IsParentCluster() (bool, error) {
	clusters, err := c.mclsLister.List(labels.Everything())
	if err != nil {
		klog.Warningf("Failed to list clusters: %v, "+
			"therefore, the cluster cannot confirm whether it is a parent cluster.", err)
		return false, err
	}
	if len(clusters) > 0 {
		return true, nil
	}
	return false, nil
}
