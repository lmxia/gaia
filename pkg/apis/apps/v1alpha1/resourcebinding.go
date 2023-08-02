package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

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
	ParentRB string `json:"parentRB,omitempty"`
	// +optional
	// +kubebuilder:pruning:PreserveUnknownFields
	// +kubebuilder:validation:Schemaless
	RbApps      []*ResourceBindingApps `json:"rbApps,omitempty"`
	NetworkPath [][]byte               `json:"networkPath,omitempty"`

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
type ResourceBindingStatus struct {
	Status string `json:"status,omitempty"`
	// Reason indicates the reason of ResourceBinding deployment Status
	// +optional
	Reason string `json:"reason,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ResourceBindingList contains a list of ResourceBinding
type ResourceBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ResourceBinding `json:"items"`
}
