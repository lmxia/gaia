package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=supplier,categories=gaia
// +kubebuilder:printcolumn:name="KIND",type="string",JSONPath=".spec.supplierName"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// CdnSupplier is a cdn resource
type CdnSupplier struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CdnSupplierSpec `json:"spec"`
	Status            SupplierStatus  `json:"status,omitempty"`
}

type CdnSupplierSpec struct {
	// +required
	SupplierName string `json:"supplierName,omitempty"`
	// +required
	CloudAccessKeyid string `json:"cloudAccessKeyid,omitempty"`
	// +required
	CloudAccessKeysecret string `json:"cloudAccessKeysecret,omitempty"`
}

type SupplierStatus struct{}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CdnSupplierList contains a list of CdnSupplier
type CdnSupplierList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CdnSupplier `json:"items"`
}
