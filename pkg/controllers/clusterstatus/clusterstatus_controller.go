package clusterstatus

import (
	"context"
	"encoding/json"
	"io/ioutil"
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

	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	known "github.com/lmxia/gaia/pkg/common"
	gaiaclientset "github.com/lmxia/gaia/pkg/generated/clientset/versioned"
	gaiainformers "github.com/lmxia/gaia/pkg/generated/informers/externalversions"
	gaialister "github.com/lmxia/gaia/pkg/generated/listers/platform/v1alpha1"
)

// Controller is a controller that collects cluster status
type Controller struct {
	kubeClient           kubernetes.Interface
	lock                 *sync.Mutex
	clusterStatus        *clusterapi.ManagedClusterStatus
	collectingPeriod     time.Duration
	heartbeatFrequency   time.Duration
	apiserverURL         string
	managedClusterSource string
	promUrlPrefix        string
	appPusherEnabled     bool
	useSocket            bool
	mclsLister           gaialister.ManagedClusterLister
	nodeLister           corev1lister.NodeLister
	nodeSynced           cache.InformerSynced
	podLister            corev1lister.PodLister
	podSynced            cache.InformerSynced
}

func NewController(ctx context.Context, apiserverURL, managedClusterSource, promUrlPrefix string, kubeClient kubernetes.Interface, gaiaClient *gaiaclientset.Clientset, collectingPeriod time.Duration, heartbeatFrequency time.Duration) *Controller {
	kubeInformerFactory := informers.NewSharedInformerFactory(kubeClient, known.DefaultResync)
	// add informers
	kubeInformerFactory.Core().V1().Nodes().Informer()
	kubeInformerFactory.Core().V1().Pods().Informer()
	kubeInformerFactory.Start(ctx.Done())

	gaiaInformerFactory := gaiainformers.NewSharedInformerFactory(gaiaClient, known.DefaultResync)
	gaiaInformerFactory.Platform().V1alpha1().ManagedClusters().Informer()
	gaiaInformerFactory.Start(ctx.Done())
	return &Controller{
		kubeClient:           kubeClient,
		lock:                 &sync.Mutex{},
		collectingPeriod:     collectingPeriod,
		heartbeatFrequency:   heartbeatFrequency,
		apiserverURL:         apiserverURL,
		managedClusterSource: managedClusterSource,
		promUrlPrefix:        promUrlPrefix,
		mclsLister:           gaiaInformerFactory.Platform().V1alpha1().ManagedClusters().Lister(),
		nodeLister:           kubeInformerFactory.Core().V1().Nodes().Lister(),
		nodeSynced:           kubeInformerFactory.Core().V1().Nodes().Informer().HasSynced,
		podLister:            kubeInformerFactory.Core().V1().Pods().Lister(),
		podSynced:            kubeInformerFactory.Core().V1().Pods().Informer().HasSynced,
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
	var capacity, allocatable corev1.ResourceList
	if len(clusters) == 0 {
		klog.V(7).Info("no joined clusters, collecting cluster resources...")
		nodes, err := c.nodeLister.List(labels.Everything())
		if err != nil {
			klog.Warningf("failed to list nodes: %v", err)
		}

		nodeStatistics = getNodeStatistics(nodes)

		if c.managedClusterSource == "informer" {
			capacity, allocatable = getNodeResource(nodes)
		} else if c.managedClusterSource == "prometheus" {
			capacity, allocatable = getNodeResourceFromPrometheus(c.promUrlPrefix)
		}
	} else {
		klog.V(7).Info("collecting ManagedCluster status...")

		nodeStatistics = getManagedClusterNodeStatistics(clusters)
		capacity, allocatable = getManagedClusterResource(clusters)

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
	status.HeartbeatFrequencySeconds = utilpointer.Int64Ptr(int64(c.heartbeatFrequency.Seconds()))
	status.Conditions = []metav1.Condition{c.getCondition(status)}
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

// discoverServiceCIDR returns the service CIDR for the cluster.
func (c *Controller) discoverServiceCIDR() (string, error) {
	return findPodIPRange(c.nodeLister, c.podLister)
}

// discoverClusterCIDR returns the cluster CIDR for the cluster.
func (c *Controller) discoverClusterCIDR() (string, error) {
	return findClusterIPRange(c.podLister)
}

// get node capacity and allocatable resource
func getNodeResource(nodes []*corev1.Node) (Capacity, Allocatable corev1.ResourceList) {
	var capacityCpu, capacityMem, allocatableCpu, allocatableMem resource.Quantity
	Capacity, Allocatable = make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)

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

	return
}

// getManagedClusterResource gets the node capacity of all managedClusters and their allocatable resources
func getManagedClusterResource(clusters []*clusterapi.ManagedCluster) (Capacity, Allocatable corev1.ResourceList) {
	Capacity, Allocatable = make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem resource.Quantity
	for _, cluster := range clusters {
		capacityCPU.Add(cluster.Status.Capacity[corev1.ResourceCPU])
		capacityMem.Add(cluster.Status.Capacity[corev1.ResourceMemory])
		allocatableCPU.Add(cluster.Status.Allocatable[corev1.ResourceCPU])
		allocatableMem.Add(cluster.Status.Allocatable[corev1.ResourceMemory])
	}
	Capacity[corev1.ResourceCPU] = capacityCPU
	Capacity[corev1.ResourceMemory] = capacityMem
	Allocatable[corev1.ResourceCPU] = allocatableCPU
	Allocatable[corev1.ResourceMemory] = allocatableMem
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
func getDataFromPrometheus(promPreUrl, metric string) string {
	var build strings.Builder
	build.WriteString(promPreUrl)
	build.WriteString(metric)
	url := build.String()

	resp, err := http.Get(url)
	if err != nil {
		klog.Warningf("failed to get the request: %s%s with err: %v", promPreUrl, metric, err)
		return "0"
	}

	defer resp.Body.Close()
	result := PrometheusQueryResponse{}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		klog.Warningf("failed to read resp.Body with err: %v", err)
		return "0"
	}

	json.Unmarshal(body, &result)

	return result.Data.Result[0].Value[1]
}

// getNodeResourceFromPrometheus returns the cpu and memory resources from Prometheus in the cluster
func getNodeResourceFromPrometheus(promPreUrl string) (Capacity, Allocatable corev1.ResourceList) {
	var capacityCPU, capacityMem, allocatableCPU, allocatableMem resource.Quantity
	Capacity, Allocatable = make(map[corev1.ResourceName]resource.Quantity), make(map[corev1.ResourceName]resource.Quantity)
	var valueList [4]string

	for index, metric := range ClusterMetricList[:4] {
		valueList[index] = getDataFromPrometheus(promPreUrl, QueryMetricSet[metric])
	}

	capacityCPU.Add(resource.MustParse(valueList[0]))
	capacityMem.Add(resource.MustParse(valueList[1] + "Ki"))
	allocatableCPU.Add(resource.MustParse(valueList[2]))
	allocatableMem.Add(resource.MustParse(valueList[3] + "Ki"))

	Capacity[corev1.ResourceCPU] = capacityCPU
	Capacity[corev1.ResourceMemory] = capacityMem
	Allocatable[corev1.ResourceCPU] = allocatableCPU
	Allocatable[corev1.ResourceMemory] = allocatableMem

	return
}

var (
	ClusterCPUCapacity    = "ClusterCPUCapacity"
	ClusterMemCapacity    = "ClusterMemCapacity"
	ClusterCPUAllocatable = "ClusterCPUAllocatable"
	ClusterMemAllocatable = "ClusterMemAllocatable"

	ClusterMetricList = []string{ClusterCPUCapacity, ClusterMemCapacity, ClusterCPUAllocatable, ClusterMemAllocatable}

	QueryMetricSet = map[string]string{
		ClusterCPUCapacity:    `sum(kube_node_status_capacity{resource="cpu"})`,
		ClusterMemCapacity:    `sum(kube_node_status_capacity{resource="memory"})/1024`,
		ClusterCPUAllocatable: `sum(kube_node_status_allocatable{resource="cpu"})`,
		ClusterMemAllocatable: `sum(kube_node_status_allocatable{resource="memory"})/1024`,
	}
)
