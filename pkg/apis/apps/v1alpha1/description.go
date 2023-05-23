package v1alpha1

import (
	lmmserverless "github.com/SUMMERLm/serverless/api/v1"
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
	// +required
	// +kubebuilder:validation:Required
	WorkloadComponents []WorkloadComponent `json:"workloadComponents,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	DeploymentCondition DeploymentCondition `json:"deploymentCondition,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	ExpectedPerformance ExpectedPerformance `json:"expectedPerformance,omitempty"`
}

type SandboxType string

const (
	Runc    SandboxType = "runc"
	Process SandboxType = "process"
	Kata    SandboxType = "kata"
	Wasm    SandboxType = "wasm"
)

type WorkloadComponent struct {
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	ComponentName string `json:"componentName,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Sandbox SandboxType `json:"sandbox,omitempty"`
	// +optional
	Preoccupy string `json:"preoccupy,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Schedule string `json:"schedule,omitempty"`
	// +required
	Module corev1.PodTemplateSpec `json:"module" protobuf:"bytes,3,opt,name=module"`
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	WorkloadType string `json:"workloadType,omitempty"`
}

// Xject means Sub or Ob
type Xject struct {
	// +required
	Name string `json:"name,omitempty"`
	Type string `json:"type,omitempty"`
}

type Condition struct {
	// +required
	// +kubebuilder:validation:Required
	Subject Xject `json:"subject,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Object Xject `json:"object,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	Relation string `json:"relation,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Extent []byte `json:"extent,omitempty"`
}

type DeploymentCondition struct {
	// +optional
	Mandatory []Condition `json:"mandatory,omitempty"`
	// +optional
	BestEffort []Condition `json:"BestEffort,omitempty"`
}

type Boundary struct {
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Subject string `json:"subject,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Type string `json:"type,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Value []byte `json:"value,omitempty"`
}

type Boundaries struct {
	// +optional
	// +kubebuilder:validation:Optional
	Inner []Boundary `json:"inner,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Inter []Boundary `json:"inter,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Extra []Boundary `json:"extra,omitempty"`
}

type XPAStrategy struct {
	// +required
	// +kubebuilder:validation:Required
	Type string `json:"type,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Value []byte `json:"value,omitempty"`
}

// XPA means HPA or VPA
type XPA struct {
	// +required
	// +kubebuilder:validation:Required
	Name string `json:"name,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Subject string `json:"subject,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Trigger string `json:"trigger,omitempty"`
	// +required
	// +kubebuilder:validation:Required
	Strategy XPAStrategy `json:"strategy,omitempty"`
}

type Maintenance struct {
	// +optional
	// +kubebuilder:validation:Optional
	HPA []XPA `json:"hpa,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	VPA []XPA `json:"vpa,omitempty"`
}

type ExpectedPerformance struct {
	// +optional
	// +kubebuilder:validation:Optional
	Boundaries Boundaries `json:"boundaries,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Maintenance Maintenance `json:"maintenance,omitempty"`
}

type Component struct {
	// +optional
	Namespace string `json:"namespace,omitempty"`
	// +required
	Name string `json:"name,omitempty"`
	// +optional
	Preoccupy string `json:"preoccupy,omitempty"`
	// +optional
	// +kubebuilder:validation:Optional
	Schedule string `json:"schedule,omitempty"`
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
	WorkloadTypeTask           WorkloadType = "task"
	WorkloadTypeStatefulSet    WorkloadType = "statefulset"
)

type Workload struct {
	// +required
	// +kubebuilder:validation:Enum=deployment;serverless;affinitydaemon;userapp
	Workloadtype WorkloadType `json:"workloadtype,omitempty"`
	// +optional
	TraitDeployment *TraitDeployment `json:"traitDeployment,omitempty"`
	// +optional
	TraitServerless *lmmserverless.TraitServerless `json:"traitServerless,omitempty"`
	// +optional
	TraitAffinityDaemon *TraitAffinityDaemon `json:"traitaffinitydaemon,omitempty"`
	// +optional
	TraitUserAPP *TraitUserAPP `json:"traitUserAPP,omitempty"`
	// +optional
	TraitTask *TraitTask `json:"traitTask,omitempty"`
	// +optional
	TraitStatefulSet *TraitStatefulSet `json:"traitStatefulSet,omitempty"`
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

type TraitTask struct {
	Completions int32  `json:"completions,omitempty"`
	Schedule    string `json:"schedule,omitempty"`
}

type TraitStatefulSet struct {
	Replicas int32 ` json:"replicas,omitempty"`
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
