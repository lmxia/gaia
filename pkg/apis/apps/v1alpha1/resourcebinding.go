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

// ComDataVolume describe data volume
type ComDataVolume struct {
	ComponentName string         `json:"componentName,omitempty"`
	Volumes       []CustomVolume `json:"volumes,omitempty"`
}

type CustomVolume struct {
	// ContainerName string `json:"containerName,omitempty"`

	// NFS
	Name     string `json:"name,omitempty"`
	Path     string `json:"path,omitempty"`
	ReadOnly bool   `json:"readOnly,omitempty"`
	Server   string `json:"server,omitempty"`

	// object-storage
	AccessKeyID     string `json:"accessKeyID,omitempty"`
	AccessKeySecret string `json:"accessKeySecret,omitempty"`
	BucketName      string `json:"bucketName,omitempty"`
	EndPoint        string `json:"endPoint,omitempty"`

	Type string `json:"type,omitempty"`
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
