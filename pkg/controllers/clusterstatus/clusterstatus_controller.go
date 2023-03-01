package clusterstatus

import (
	"context"
	"github.com/lmxia/gaia/pkg/utils"
	"net/http"
	"strings"
	"sync"
	"time"

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
	hypernodelister "github.com/SUMMERLm/hyperNodes/pkg/generated/listers/cluster/v1alpha1"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	known "github.com/lmxia/gaia/pkg/common"
	"github.com/lmxia/gaia/pkg/controllers/clusterstatus/toposync"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	gaialister "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"
	"github.com/prometheus/client_golang/api"
	prometheusv1 "github.com/prometheus/client_golang/api/prometheus/v1"
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
	promUrlPrefix          string
	topoSyncBaseUrl        string
	appPusherEnabled       bool
	useSocket              bool
	useHypernodeController bool
	mclsLister             gaialister.ManagedClusterLister
	nodeLister             corev1lister.NodeLister
	hypernodeLister        hypernodelister.HypernodeLister
	hypernodeClient        *hypernodeclientset.Clientset
	nodeSynced             cache.InformerSynced
	podLister              corev1lister.PodLister
	podSynced              cache.InformerSynced
	clusterName            string
}

func NewController(ctx context.Context, apiserverURL, clusterName string, managedCluster *clusterapi.ManagedClusterOptions, kubeClient kubernetes.Interface, gaiaClient *gaiaclientset.Clientset, hypernodeClient *hypernodeclientset.Clientset, collectingPeriod time.Duration, heartbeatFrequency time.Duration) *Controller {
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
		promUrlPrefix:          managedCluster.PrometheusMonitorUrlPrefix,
		topoSyncBaseUrl:        managedCluster.TopoSyncBaseUrl,
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
	if len(clusters) == 0 {
		klog.V(7).Info("no joined clusters, collecting cluster resources...")
		nodes, err := c.nodeLister.List(labels.Everything())
		if err != nil {
			klog.Warningf("failed to list nodes: %v", err)
		}

		nodeStatistics = getNodeStatistics(nodes)
		if c.managedClusterSource == known.ManagedClusterSourceFromInformer {
			capacity, allocatable, available = getNodeResource(nodes)
		} else if c.managedClusterSource == known.ManagedClusterSourceFromPrometheus {
			capacity, allocatable, available = getNodeResourceFromPrometheus(c.promUrlPrefix)
		}
	} else {
		klog.V(7).Info("collecting ManagedCluster status...")

		nodeStatistics = getManagedClusterNodeStatistics(clusters)
		capacity, allocatable, available = getManagedClusterResource(clusters)

		selfClusterName, _, errClusterName := utils.GetLocalClusterName(c.kubeClient.(*kubernetes.Clientset))
		if errClusterName != nil {
			klog.Warningf("failed to get self clusterName from secret: %v", errClusterName)
			selfClusterName = c.clusterName
		}
		topoInfo = getTopoInfo(ctx, selfClusterName, c.topoSyncBaseUrl)
	}

	clusterCIDR, err := c.discoverClusterCIDR()
	if err != nil {
		klog.Warningf("failed to discover cluster CIDR: %v", err)
	}

	serviceCIDR, err := c.discoverServiceCIDR()
	if err != nil {
		klog.Warningf("failed to discover service CIDR: %v", err)
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
	status.HeartbeatFrequencySeconds = utilpointer.Int64Ptr(int64(c.heartbeatFrequency.Seconds()))
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
func getTopoInfo(ctx context.Context, clusterName, topoSyncBaseUrl string) (topoInfo clusterapi.Topo) {
	hyperTopoSync := clusterapi.Fields{
		Field: []string{clusterName},
	}
	topos, _, err := toposync.NewAPIClient(toposync.NewConfiguration(topoSyncBaseUrl)).TopoSyncApi.TopoSync(ctx, hyperTopoSync)
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
			nodeLabels = parseNodeLabels(nodeLabels, node.GetLabels(), node.GetName())
		}
	}
	return nodeLabels
}

// parseNodeLabels returns the nodeLabels that belong to specific string list.
func parseNodeLabels(nodeLabels, inLabels map[string]string, nodeName string) map[string]string {
	for labelKey, labelValue := range inLabels {
		if len(labelValue) > 0 {
			if labelKey == clusterapi.ParsedSNKey || labelKey == clusterapi.ParsedGeoLocationKey || labelKey == clusterapi.ParsedProviderKey {
				nodeLabels[labelKey+"__"+nodeName] = labelValue
			} else if utils.ContainsString(clusterapi.ParsedHypernodeLableKeyList, labelKey) {
				nodeLabels[labelKey] = labelValue
			}
		}
	}
	return nodeLabels
}

// parseHypernodeLabels returns the HypernodeLabels that belong to specific string list.
func parseHypernodeLabels(nodeLabels, inLabels map[string]string, nodeName string) map[string]string {
	for labelKey, labelValue := range inLabels {
		if len(labelValue) > 0 {
			if labelKey == clusterapi.SNKey || labelKey == clusterapi.GeoLocationKey || labelKey == clusterapi.NetworkEnvKey {
				nodeLabels[clusterapi.HypernodeLableKeyToStandardLabelKey[labelKey]+"__"+nodeName] = labelValue
			} else if utils.ContainsString(clusterapi.HypernodeLableKeyList, labelKey) {
				if _, ok := nodeLabels[clusterapi.HypernodeLableKeyToStandardLabelKey[labelKey]]; ok {
					existedLabelValueArray := strings.Split(nodeLabels[clusterapi.HypernodeLableKeyToStandardLabelKey[labelKey]], "__")
					if !utils.ContainsString(existedLabelValueArray, labelValue) {
						nodeLabels[clusterapi.HypernodeLableKeyToStandardLabelKey[labelKey]] = nodeLabels[clusterapi.HypernodeLableKeyToStandardLabelKey[labelKey]] + "__" + labelValue
					}
				} else {
					nodeLabels[clusterapi.HypernodeLableKeyToStandardLabelKey[labelKey]] = labelValue
				}
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
				if strings.HasPrefix(labelKey, clusterapi.ParsedSNKey) || strings.HasPrefix(labelKey, clusterapi.ParsedGeoLocationKey) || strings.HasPrefix(labelKey, clusterapi.ParsedProviderKey) {
					nodeLabels[labelKey] = labelValue
				} else if utils.ContainsString(clusterapi.ParsedHypernodeLableKeyList, labelKey) {
					if _, ok := nodeLabels[labelKey]; ok {
						existedLabelValueArray := strings.Split(nodeLabels[labelKey], "__")
						if !utils.ContainsString(existedLabelValueArray, labelValue) {
							nodeLabels[labelKey] = nodeLabels[labelKey] + "__" + labelValue
						}
					} else {
						nodeLabels[labelKey] = labelValue
					}
				}
			}
		}
	}
	return
}

// GetManagedClusterLabels returns the specified node labels for the clusters
func (c *Controller) GetManagedClusterLabels() (nodeLabels map[string]string) {
	nodeLabels = make(map[string]string)
	if c.useHypernodeController {
		hypernodeList, err := c.hypernodeClient.ClusterV1alpha1().Hypernodes(metav1.NamespaceDefault).List(context.TODO(), metav1.ListOptions{})
		if err != nil {
			klog.Warningf("failed to list hypernodes: %v", err)
		}

		for _, hypernode := range hypernodeList.Items {
			// get hypernodes' labels that are in Cluster level
			// only the worker node labels whose "NodeRole" is not "System"
			if value, ok := hypernode.GetLabels()[clusterapi.NodeRoleKey]; ok && value != "System" {
				if hypernode.Spec.NodeAreaType == "cluster" {
					nodeLabels = parseHypernodeLabels(nodeLabels, hypernode.GetLabels(), hypernode.GetName())
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
				nodeLabels = getNodeLabels(nodes)
			}
			nodeLabels = getNodeLabels(nodes)
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
func getNodeResource(nodes []*corev1.Node) (Capacity, Allocatable, Available corev1.ResourceList) {
	var capacityCpu, capacityMem, allocatableCpu, allocatableMem resource.Quantity
	Capacity, Allocatable, Available = make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)

	for _, node := range nodes {
		capacityCpu.Add(*node.Status.Capacity.Cpu())
		capacityMem.Add(*node.Status.Capacity.Memory())
		allocatableCpu.Add(*node.Status.Allocatable.Cpu())
		allocatableMem.Add(*node.Status.Allocatable.Memory())
	}

	Capacity[corev1.ResourceCPU] = capacityCpu
	Capacity[corev1.ResourceMemory] = capacityMem
	Allocatable[corev1.ResourceCPU] = allocatableCpu
	Allocatable[corev1.ResourceMemory] = allocatableMem
	Available[corev1.ResourceCPU] = allocatableCpu
	Available[corev1.ResourceMemory] = allocatableMem

	return
}

// getManagedClusterResource gets the node capacity of all managedClusters and their allocatable resources
func getManagedClusterResource(clusters []*clusterapi.ManagedCluster) (Capacity, Allocatable, Available corev1.ResourceList) {
	Capacity, Allocatable, Available = make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem, availableCPU, availableMem resource.Quantity
	for _, cluster := range clusters {
		capacityCPU.Add(cluster.Status.Capacity[corev1.ResourceCPU])
		capacityMem.Add(cluster.Status.Capacity[corev1.ResourceMemory])
		allocatableCPU.Add(cluster.Status.Allocatable[corev1.ResourceCPU])
		allocatableMem.Add(cluster.Status.Allocatable[corev1.ResourceMemory])
		availableCPU.Add(cluster.Status.Available[corev1.ResourceCPU])
		availableMem.Add(cluster.Status.Available[corev1.ResourceMemory])
	}
	Capacity[corev1.ResourceCPU] = capacityCPU
	Capacity[corev1.ResourceMemory] = capacityMem
	Allocatable[corev1.ResourceCPU] = allocatableCPU
	Allocatable[corev1.ResourceMemory] = allocatableMem
	Available[corev1.ResourceCPU] = availableCPU
	Available[corev1.ResourceMemory] = availableMem
	return
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

// getDataFromPrometheus returns the result from Prometheus according to the specified metric in the cluster
func getDataFromPrometheus(promPreUrl, metric string) (model.Value, error) {
	client, err := api.NewClient(api.Config{
		Address: promPreUrl,
	})
	if err != nil {
		klog.Warningf("Error creating client: %v", err)
		return nil, err
	}

	v1api := prometheusv1.NewAPI(client)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, warnings, err := v1api.Query(ctx, metric, time.Now())
	if err != nil {
		klog.Warningf("Error querying Prometheus: %v", err)
		return nil, err
	}
	if len(warnings) > 0 {
		klog.Warningf("Warnings: %v\n", warnings)
	}

	return result, nil
}

// getNodeResourceFromPrometheus returns the cpu and memory resources from Prometheus in the cluster
func getNodeResourceFromPrometheus(promPreUrl string) (Capacity, Allocatable, Available corev1.ResourceList) {
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem, availableCPU, availableMem resource.Quantity
	Capacity, Allocatable, Available = make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)
	var valueList [6]string

	QueryMetricSet, ClusterMetricList, err := utils.InitConfig(known.MetricConfigMapAbsFilePath)
	if err == nil && len(ClusterMetricList) == 6 {
		for index, metric := range ClusterMetricList[:6] {
			result, err := getDataFromPrometheus(promPreUrl, QueryMetricSet[metric])
			if err == nil {
				if len(result.(model.Vector)) <= 0 {
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
			klog.Warningf("The length of ClusterMetricList is %v, it is less then 6. Wrong metrics %v", len(ClusterMetricList), QueryMetricSet)
		}
		valueList = [6]string{"0", "0", "0", "0", "0", "0"}
	}

	capacityCPU.Add(resource.MustParse(valueList[0]))
	capacityMem.Add(resource.MustParse(valueList[1] + "Ki"))
	allocatableCPU.Add(resource.MustParse(valueList[2]))
	allocatableMem.Add(resource.MustParse(valueList[3] + "Ki"))
	availableCPU.Add(resource.MustParse(getSubStringWithSpecifiedDecimalPlace(valueList[4], 3)))
	availableMem.Add(resource.MustParse(valueList[5] + "Ki"))

	Capacity[corev1.ResourceCPU] = capacityCPU
	Capacity[corev1.ResourceMemory] = capacityMem
	Allocatable[corev1.ResourceCPU] = allocatableCPU
	Allocatable[corev1.ResourceMemory] = allocatableMem
	Available[corev1.ResourceCPU] = availableCPU
	Available[corev1.ResourceMemory] = availableMem

	return
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
		return
	}

	for _, metricPsql := range metricMap {
		pendingLatencyResult, err := getDataFromPrometheus(c.promUrlPrefix, metricPsql)
		if err != nil {
			klog.Warningf("Query failed from prometheus, err is %v. The metric is %v", err, metricPsql)
			return
		}
		resultList := pendingLatencyResult.(model.Vector)
		if len(resultList) > 0 {
			for _, result := range resultList {
				podNamespace := result.Metric["destination_namespace"]
				podName := result.Metric["destination_pod"]
				podUid := result.Metric["destination_pod_uid"]
				podDescName := result.Metric["destination_pod_description_name"]
				podComName := result.Metric["destination_pod_component_name"]
				klog.V(5).InfoS("pod is abnormal according to the metric: ", "podNamespace", podNamespace, "podName", podName, "podUid", podUid, "metricPsql", metricPsql)
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
	return
}

// IsParentCluster return whether it is a parent cluster
func (c *Controller) IsParentCluster() (bool, error) {
	clusters, err := c.mclsLister.List(labels.Everything())
	if err != nil {
		klog.Warningf("Failed to list clusters: %v, therefore, the cluster cannot confirm whether it is a parent cluster.", err)
		return false, err
	}
	if len(clusters) > 0 {
		return true, nil
	}
	return false, nil
}
