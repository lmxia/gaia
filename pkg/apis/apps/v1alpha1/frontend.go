package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=front,categories=gaia
// +kubebuilder:printcolumn:name="DomainName",type="string",JSONPath=".spec.domainName"
// +kubebuilder:printcolumn:name="ACRegion",type="string",JSONPath=".spec.cdnSpec[0].accerlateRegion"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Frontend is a front resource
type Frontend struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              FrontendSpec   `json:"spec"`
	Status            FrontendStatus `json:"status,omitempty"`
}

type FrontendSpec struct {
	// +required
	DomainName string `json:"domainName,omitempty"`
	// +optional
	CdnAccelerate bool `json:"cdnAccelerate,omitempty"`
	// +required
	Cdn []CdnSpec `json:"cdnSpec,omitempty"`
}

type CdnSpec struct {
	// +optional
	SourceSite string `json:"sourceSite,omitempty"`
	// +required
	Supplier string `json:"supplier,omitempty"`
	// +required
	AccerlateRegion string `json:"accerlateRegion,omitempty"`
	// +optional
	InternetChargeType string `json:"internetChargeType,omitempty"`
	// +optional
	CdnType string `json:"cdnType,omitempty"`
	// +optional
	RecordType string `json:"recordType,omitempty"`
	// +optional
	PkgName string `json:"pkgName,omitempty"`
}
type FrontendStatus struct{}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FrontendList contains a list of Frontend
type FrontendList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Frontend `json:"items"`
}
