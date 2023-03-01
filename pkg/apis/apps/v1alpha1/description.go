package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	serveringv1 "knative.dev/serving/pkg/apis/serving/v1"
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
	// +required
	AppID string `json:"appID,omitempty"` // appID是蓝图的id
	// +optional
	Preoccupy string `json:"preoccupy,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Components []Component `json:"components,omitempty"`
}
type Component struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	Name string `json:"name,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Serverless ServerlessSpec `json:"serverless,omitempty" protobuf:"bytes,3,opt,name=serverless"`
	// +optional
	// +kubebuilder:validation:Optional
	// +kubebuilder:pruning:PreserveUnknownFields
	Module corev1.PodTemplateSpec `json:"module" protobuf:"bytes,3,opt,name=module"`
	// +required
	RuntimeType string `json:"runtimeType,omitempty"`
	// +required
	// +kubebuilder:default=1
	Dispersion int32 `json:"dispersion,omitempty"`
	// +required
	Workload Workload `json:"workload,omitempty"`
	// +required
	SchedulePolicy SchedulePolicy `json:"schedulePolicy,omitempty"`
	// +optional
	ClusterTolerations []corev1.Toleration `json:"clusterTolerations,omitempty"`
}

type WorkloadType string

const (
	WorkloadTypeDeployment     WorkloadType = "deployment"
	WorkloadTypeServerless     WorkloadType = "serverless"
	WorkloadTypeAffinityDaemon WorkloadType = "affinitydaemon"
	WorkloadTypeUserApp        WorkloadType = "userapp"
)

type Workload struct {
	// +required
	// +kubebuilder:validation:Enum=deployment;serverless;affinitydaemon;userapp
	Workloadtype WorkloadType `json:"workloadtype,omitempty"`
	// +optional
	TraitDeployment *TraitDeployment `json:"traitDeployment,omitempty"`
	// +optional
	TraitServerless *TraitServerless `json:"traitServerless,omitempty"`
	// +optional
	TraitAffinityDaemon *TraitAffinityDaemon `json:"traitaffinitydaemon,omitempty"`
	// +optional
	TraitUserAPP *TraitUserAPP `json:"traitUserAPP,omitempty"`
}
type ServerlessSpec struct {
	// +optional
	Revision RevisionSpec `json:"revision,omitempty"`
	// Traffic specifies how to distribute traffic over a collection of
	// revisions and configurations.
	// +optional
	Traffic []serveringv1.TrafficTarget `json:"traffic,omitempty"`
}

// RevisionSpec holds the desired state of the Revision (from the client).
type RevisionSpec struct {
	// ContainerConcurrency specifies the maximum allowed in-flight (concurrent)
	// requests per container of the Revision.  Defaults to `0` which means
	// concurrency to the application is not limited, and the system decides the
	// target concurrency for the autoscaler.
	// +optional
	ContainerConcurrency *int64 `json:"containerConcurrency,omitempty"`

	// TimeoutSeconds is the maximum duration in seconds that the request routing
	// layer will wait for a request delivered to a container to begin replying
	// (send network traffic). If unspecified, a system default will be provided.
	// +optional
	TimeoutSeconds *int64 `json:"timeoutSeconds,omitempty"`
}

type TraitDeployment struct {
	Replicas int32 ` json:"replicas,omitempty"`
}

type TraitUserAPP struct {
	SN string `json:"sn,omitempty"`
}

type TraitServerless struct {
	MiniInstancenumber int32  `json:"miniInstancenumber,omitempty"`
	Step               int32  `json:"step,omitempty"`
	Threshold          string `json:"threshold,omitempty"`
}

type TraitAffinityDaemon struct {
	SNS []string `json:"sns,omitempty"`
}

type SchedulePolicy struct {
	// +optional
	SpecificResource *metav1.LabelSelector `json:"specificResource,omitempty"`
	// +optional
	NetEnvironment *metav1.LabelSelector `json:"netenvironment,omitempty"`
	// +optional
	GeoLocation *metav1.LabelSelector `json:"geolocation,omitempty"`
	// +optional
	Provider *metav1.LabelSelector `json:"provider,omitempty"`
}

// DescriptionStatus defines the observed state of Description
type DescriptionStatus struct {
	// Phase denotes the phase of Description
	// +optional
	// +kubebuilder:validation:Enum=Pending;Scheduled;Failure
	Phase DescriptionPhase `json:"phase,omitempty"`

	// Reason indicates the reason of DescriptionPhase
	// +optional
	Reason string `json:"reason,omitempty"`
}

type DescriptionPhase string

const (
	DescriptionPhaseScheduled  DescriptionPhase = "Scheduled"
	DescriptionPhasePending    DescriptionPhase = "Pending"
	DescriptionPhaseFailure    DescriptionPhase = "Failure"
	DescriptionPhaseReSchedule DescriptionPhase = "ReSchedule"
)

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DescriptionList contains a list of Description
type DescriptionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Description `json:"items"`
}
