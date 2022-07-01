package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generated" to regenerate code after modifying this file

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster",shortName=userapp,categories=gaia
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// UserAPP is the Schema for the resources to be installed
type UserAPP struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   UserAPPSpec   `json:"spec"`
	Status UserAPPStatus `json:"status,omitempty"`
}

// UserAPPSpec defines the spec of UserAPP
type UserAPPSpec struct {
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Module corev1.PodTemplateSpec `json:"module" protobuf:"bytes,3,opt,name=module"`
	SN     string                 `json:"sn,omitempty"`
}

// UserAPPStatus defines the observed state of UserAPP
type UserAPPStatus struct {
	// Phase denotes the phase of UserAPP
	// +optional
	// +kubebuilder:validation:Enum=Pending;Scheduled;Failure
	Phase UserAPPPhase `json:"phase,omitempty"`

	// Reason indicates the reason of UserAPPPhase
	// +optional
	Reason string `json:"reason,omitempty"`
}

type UserAPPPhase string

const (
	UserAPPPhaseScheduled UserAPPPhase = "Scheduled"
	UserAPPPhasePending   UserAPPPhase = "Pending"
	UserAPPPhaseFailure   UserAPPPhase = "Failure"
)

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// UserAPPList contains a list of UserAPPP
type UserAPPList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []UserAPP `json:"items"`
}
