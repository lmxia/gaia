package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generated" to regenerate code after modifying this file

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=nwr,categories=gaia
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=".status.phase"

// NetworkRequirement is the Schema for the resources to be installed
type NetworkRequirement struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   NetworkRequirementSpec   `json:"spec"`
	Status NetworkRequirementStatus `json:"status,omitempty"`
}

// NetworkRequirementSpec defines the spec of NetworkRequirement
type NetworkRequirementSpec struct {
	// +optional
	NetworkCommunication []NetworkCommunication `json:"networkCommunication,omitempty"`
}

type NetworkCommunication struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	SelfID []string `json:"selfID,omitempty"`
	// +optional
	InterSCNID []InterSCNID `json:"interSCNID,omitempty"`
}

type InterSCNID struct {
	// +optional
	Source      Direction  `json:"source,omitempty"`
	Destination Direction  `json:"destination,omitempty"`
	Sla         AppSlaAttr `json:"sla,omitempty"`
	Providers   []string   `json:"providers,omitempty"`
}
type AppSlaAttr struct {
	Delay     int32 `json:"delay,omitempty"`
	Lost      int32 `json:"lost,omitempty"`
	Jitter    int32 `json:"jitter,omitempty"`
	Bandwidth int64 `json:"bandwidth,omitempty"`
}
type Direction struct {
	// +optional
	Id string `json:"id,omitempty"`
	// +optional
	Attributes []Attributes `json:"attributes,omitempty"`
}
type Attributes struct {
	// +optional
	Key string `json:"key,omitempty"`
	// +optional
	Value string `json:"value,omitempty"`
}

// NetworkRequirementStatus defines the observed state of NetworkRequirement
type NetworkRequirementStatus struct {
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// NetworkRequirementList contains a list of NetworkRequirement
type NetworkRequirementList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []NetworkRequirement `json:"items"`
}
