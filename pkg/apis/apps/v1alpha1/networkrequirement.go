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
	WorkloadComponents WorkloadComponents `json:"workloadComponents,omitempty"`
	// +optional
	Deployconditions DeploymentCondition `json:"deployconditions,omitempty"`
}

type Scn struct {
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	SelfID []string `json:"selfID,omitempty"`
}

type Link struct {
	// +optional
	LinkName string `json:"linkName,omitempty"`
	// +optional
	SourceID string `json:"sourceID,omitempty"`
	// +optional
	DestinationID string `json:"destinationID,omitempty"`
	// +optional
	SourceAttributes []Attributes `json:"sourceAttributes,omitempty"`
	// +optional
	DestinationAttributes []Attributes `json:"destinationAttributes,omitempty"`
}

type WorkloadComponents struct {
	// +optional
	Scns []Scn `json:"scns,omitempty"`
	// +optional
	Links []Link `json:"links,omitempty"`
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
