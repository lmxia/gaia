package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make generated" to regenerate code after modifying this file

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=desc,categories=gaia
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Description is the Schema for the resources to be installed
type Description struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DescriptionSpec   `json:"spec"`
	Status DescriptionStatus `json:"status,omitempty"`
}

// DescriptionSpec defines the spec of Description
type DescriptionSpec struct {
	// Raw is the underlying serialization of all objects.
	//
	//Raw [][]byte `json:"raw,omitempty"`
	// +required
	AppID string `json:"appID,omitempty"` // appID是蓝图的id
	// +optional
	Component []Components `json:"component,omitempty"`
}
type Components struct {
	// +required
	//AppID string `json:"appID,omitempty"` // appID是蓝图的id
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	Name string `json:"name,omitempty"`
	// +optional
	Moudles corev1.PodTemplateSpec `json:"template" protobuf:"bytes,3,opt,name=template"` //module是container意思。
	// +required
	RuntimeType string `json:"runtimeType,omitempty"`
	// +required
	Workload Workload `json:"workload,omitempty"`
	// +required
	SchedulePolicy SchedulePolicy `json:"schedulePolicy,omitempty"`
}

type Workload struct {
	// +optional
	Workloadtype string `json:"workloadtype,omitempty"`
	// +optional
	TraitDeployment *TraitDeployment `json:"traitServerless,omitempty"`
	// +optional
	TraitServerless *TraitServerless `json:"traitServerless,omitempty"`
}

type TraitDeployment struct {
	Replicas int32 ` json:"replicas,omitempty"`
}

type TraitServerless struct {
	MiniInstancenumber int32  ` json:"miniInstancenumber,omitempty"`
	Step               int32  `json:"step,omitempty"`
	Threshold          string `json:"threshold,omitempty"`
}

//type Trait struct {
//	// +optional
//	replicas int32 `json:"replicas,omitempty"` // deploy和serverless确定结构。annity的deamonset trait是空，userapp空
//}
type SchedulePolicy struct {
	// +optional
	SpecificResource NodeResource `json:"specificResource,omitempty"`
	// +optional
	Netenvironment CoreResource `json:"netenvironment,omitempty"`
	// +optional
	Geolocation CoreResource `json:"geolocation,omitempty"`
	// +optional
	Provider CoreResource `json:"provider,omitempty"`
}
type NodeResource struct {
	// +optional
	Sn string `json:"name,omitempty"`
	// +optional
	Sname string `json:"sname,omitempty"`
}
type CoreResource struct {
	// +optional
	Hards []string `json:"hards,omitempty"`
}

// DescriptionStatus defines the observed state of Description
type DescriptionStatus struct {
	// Phase denotes the phase of Description
	// +optional
	// +kubebuilder:validation:Enum=Pending;Success;Failure
	Phase DescriptionPhase `json:"phase,omitempty"`

	// Reason indicates the reason of DescriptionPhase
	// +optional
	Reason string `json:"reason,omitempty"`
}

type DescriptionDeployer string

const (
	DescriptionHelmDeployer    DescriptionDeployer = "Helm"
	DescriptionGenericDeployer DescriptionDeployer = "Generic"
)

type DescriptionPhase string

const (
	DescriptionPhaseSuccess DescriptionPhase = "Success"
	DescriptionPhaseFailure DescriptionPhase = "Failure"
)

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DescriptionList contains a list of Description
type DescriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Description `json:"items"`
}
