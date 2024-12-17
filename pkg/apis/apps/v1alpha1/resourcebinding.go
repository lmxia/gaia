package v1alpha1

import (
	coreV1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=rb,categories=gaia
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="SCHEDULE",type=string,JSONPath=".spec.statusScheduler"
// +kubebuilder:printcolumn:name="BIND",type=string,JSONPath=".status.status"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type ResourceBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              ResourceBindingSpec `json:"spec,omitempty"`
	// +optional
	Status ResourceBindingStatus `json:"status,omitempty"`
}

type StatusScheduler string

const (
	ResourceBindingMerging      StatusScheduler = "merging"
	ResourceBindingmerged       StatusScheduler = "merged"
	ResourceBindingSchedulering StatusScheduler = "schedulering"
	ResourceBindingSelected     StatusScheduler = "selected"
)

type ResourceBindingSpec struct {
	// +optional
	AppID string `json:"appID,omitempty"`
	// +optional
	TotalPeer int `json:"totalpeer,omitempty"`
	// +optional
	NonZeroClusterNum int `json:"nonZeroClusterNum,omitempty"`
	// +optional
	ParentRB string `json:"parentRB,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	RbApps      []*ResourceBindingApps `json:"rbApps,omitempty"`
	NetworkPath [][]byte               `json:"networkPath,omitempty"`
	StoragePath [][]byte               `json:"storagePath,omitempty"`
	FrontendRbs []*FrontendRb          `json:"frontendRbs,omitempty"`

	// +optional
	// +kubebuilder:validation:Enum=merging;merged;schedulering;selected
	StatusScheduler StatusScheduler `json:"statusScheduler,omitempty"`
}

type ResourceBindingApps struct {
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// +optional
	ChosenOne map[string]int32 `json:"theChosenOne"`
	Replicas  map[string]int32 `json:"replicas,omitempty"`
	// +optional
	Children []*ResourceBindingApps `json:"children,omitempty"`
}

type FrontendRb struct {
	// +optional
	Suppliers []*Supplier `json:"suppliers,omitempty"`
}

type Supplier struct {
	// optional
	SupplierName string `json:"supplierName,omitempty"`
	// +optional
	Replicas map[string]int32 `json:"replicas,omitempty"`
}

type StatusRBDeploy string

const (
	ResourceBindingRed   StatusRBDeploy = "Red"
	ResourceBindingGreen StatusRBDeploy = "Green"

	ConditionFail    string = "Failure"
	ConditionSuccess string = "Success"
)

type ResourceBindingStatus struct {
	// +optional
	Status StatusRBDeploy `json:"status,omitempty"`
	// +optional
	WorkloadStatus []ComStatus `json:"workloadStatus,omitempty"`
	// +optional
	Clusters map[string]StatusRBDeploy `json:"clusters,omitempty"`
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`
}

type ComStatus struct {
	// +optional
	ComponentName string       `json:"componentName,omitempty"`
	Clusters      []ComCluster `json:"clusters,omitempty"`
}

type ComCluster struct {
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	DeployMsg   string `json:"deployMsg,omitempty"`
	// map[podName]status
	// PodsStatus map[string]string `json:"podsStatus,omitempty"`
	// map[serviceName]string
	ServiceIP map[string]string `json:"serviceIP,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceBindingList contains a list of ResourceBinding
type ResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceBinding `json:"items"`
}

// PlanMsg 任务下发描述
type PlanMsg struct {
	TaskName string         `json:"taskName"`
	ResId    string         `json:"resId"` //nolint:stylecheck
	Replicas map[string]int `json:"replicas"`

	NetworkDesc []ComNetworkDesc `json:"NetworkDesc,omitempty"`
	StorageDesc []ComStorageDesc `json:"storageDesc,omitempty"`
}

type ComNetworkDesc struct {
	NetType string `json:"netType,omitempty"` // ceni public local
	Fqdn    string `json:"fqdn,omitempty"`
	IPs     OneIPs `json:"ips,omitempty"`
}

type OneIPs struct {
	Protocol string `json:"protocol,omitempty"`
	IP       string `json:"ip,omitempty"`
	Port     string `json:"port,omitempty"`
}

type ComStorageDesc struct {
	Image      ComImage      `json:"image,omitempty"`
	DataVolume ComDataVolume `json:"dataVolume,omitempty"`
}

type ComImage struct {
	ContainerName      string `json:"containerName,omitempty"`
	SourceDataPath     string `json:"sourceDataPath,omitempty"`
	TargetResource     string `json:"targetResource,omitempty"`
	TargetResourcePath string `json:"targetResourcePath,omitempty"`
	ExpectedDuration   string `json:"expectedDuration,omitempty"`
}

// ComDataVolume describe data volume
type ComDataVolume struct {
	// read write
	Mode           string `json:"mode,omitempty"`
	Parameter      string `json:"parameter,omitempty"` // todo 这个参数是啥
	SourceDataPath string `json:"SourceDataPath,omitempty"`
	// 搬运任务目的地址
	TargetResource string `json:"targetResource,omitempty"`
	// 搬运任务目的路径
	TargetResourcePath string `json:"targetResourcePath,omitempty"`

	ExpectedDuration   string `json:"expectedDuration,omitempty"`
	SplitShellLocation string `json:"SplitShellLocation,omitempty"`
	SourceSplitCount   int    `json:"sourceSplitCount,omitempty"`
}

type HyperLabelNetItem struct {
	ComponentName string `json:"componentName"`

	ExposeType string   `json:"exposeType"`
	VNList     []string `json:"vnList,omitempty"` // sn
	// +optional ceniip should be related to vnlist one by one.
	CeniIPList []string `json:"ceniIPList,omitempty"`
	// +optional
	FQDNCENI string `json:"fqdnCeni,omitempty"`
	// +optional
	FQDNPublic string               `json:"fqdnPublic,omitempty"`
	Ports      []coreV1.ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port"`
}
