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

type (
	SouthboundTraffic struct {
		// +optional
		TargetLabel string `json:"targetlabel,omitempty" protobuf:"bytes,1,opt,name=targetlabel"`
		// +optional
		Delay *int64 `json:"delay,omitempty" protobuf:"varint,5,opt,name=delay"`
		// +optional
		Robustness *int64 `json:"robustness,omitempty" protobuf:"varint,5,opt,name=robustness"`
		// +optional
		Bandwidth *int64 `json:"bandwidth,omitempty" protobuf:"varint,5,opt,name=bandwidth"`
	}

	NorthboundTraffic struct {
		// +optional
		Concurrency *int64 `json:"concurrency,omitempty" protobuf:"varint,5,opt,name=concurrency"`
		// +optional
		Capacity *int64 `json:"capacity,omitempty" protobuf:"varint,5,opt,name=capacity"`
	}

	ServiceRequirement struct {
		// +optional
		Selector map[string]string `json:"selector,omitempty" protobuf:"bytes,2,rep,name=selector"`

		// +optional
		South SouthboundTraffic `json:"south,omitempty"`

		// +optional
		North NorthboundTraffic `json:"north,omitempty"`
	}
)

// NetworkRequirementSpec defines the spec of NetworkRequirement
type NetworkRequirementSpec struct {
	// Raw is the underlying serialization of all objects.
	//
	// +optional
	Requirement []ServiceRequirement `json:"requirement,omitempty"`
	// +optional
	Name string `json:"name,omitempty"`
	// +optional
	SelfID []string `json:"selfID,omitempty"`
	// +optional
	InterSCNID InterSCNID `json:"interSCNID,omitempty"`
}

type InterSCNID struct {
	// +optional
	Source      Direction `json:"source,omitempty"`
	Destination Direction `json:"destination,omitempty"`
	Sla         []string  `json:"sla,omitempty"`
	Providers   []string  `json:"providers,omitempty"`
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
