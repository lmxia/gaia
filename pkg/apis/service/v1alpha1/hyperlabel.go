package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=hyperlabel,categories=gaia
// +k8s:openapi-gen=true
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type HyperLabel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              []HyperLabelItem `json:"spec,omitempty"`
	// +optional
	Status HyperLabelStatus `json:"status,omitempty"`
}

// HyperLabelItem describe one component, may have many ExposeType: ceni public, use 2 item describe
type HyperLabelItem struct {
	ExposeType string   `json:"exposeType"`
	VNList     []string `json:"vnList,omitempty"` // sn
	// +optional ceniip should be related to vnlist one by one.
	CeniIPList    []string `json:"ceniIPList,omitempty"`
	ComponentName string   `json:"componentName"`
	// +optional
	FQDNCENI string `json:"fqdnCeni,omitempty"`
	// +optional
	FQDNPublic string           `json:"fqdnPublic,omitempty"`
	Ports      []v1.ServicePort `json:"ports,omitempty" patchStrategy:"merge" patchMergeKey:"port"`
}

type StatusDeploy string

const (
	ConditionFail    StatusDeploy = "Failure"
	ConditionSuccess StatusDeploy = "Success"
)

type HyperLabelStatus struct {
	// +optional
	Status       StatusDeploy                 `json:"status,omitempty"`
	PublicIPInfo map[string]map[string]string `json:"info,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HyperLabelList contains a list of HyperLabel
type HyperLabelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HyperLabel `json:"items"`
}
