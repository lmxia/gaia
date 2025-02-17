package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=hyperImage,categories=gaia
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"
type HyperImage struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              []HyperImageItem `json:"spec,omitempty"`
	// +optional
	Status HyperImageStatus `json:"status,omitempty"`
}

// HyperImageItem describe one component, may be located into many vns
type HyperImageItem struct {
	VNList        map[string]*HyperImageTransportInfo `json:"vnList,omitempty"` // sn
	ComponentName string                              `json:"componentName"`
}

type HyperImageTransportInfo struct {
	// image source, of couse it's in a nfs or some kind network storage path.
	ImageSource string
	// image tag info
	ImageTag string
}

type StatusDeploy string

const (
	ConditionFail    StatusDeploy = "Failure"
	ConditionSuccess StatusDeploy = "Success"
	ConditionRunning StatusDeploy = "Running"
)

type HyperImageStatus struct {
	// +optional
	Status   StatusDeploy                       `json:"status,omitempty"`
	SyncInfo map[string]map[string]StatusDeploy `json:"info,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HyperImageList contains a list of HyperImage
type HyperImageList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []HyperImage `json:"items"`
}
