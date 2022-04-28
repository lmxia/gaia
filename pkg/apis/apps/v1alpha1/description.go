package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
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
	Module corev1.PodSpec `json:"module" protobuf:"bytes,3,opt,name=module"`
	// +required
	RuntimeType string `json:"runtimeType,omitempty"`
	// +required
	Workload Workload `json:"workload,omitempty"`
	// +required
	SchedulePolicy SchedulePolicy `json:"schedulePolicy,omitempty"`
	// ClusterTolerations tolerates any matched taints of ManagedCluster.
	//
	// +optional
	ClusterTolerations []corev1.Toleration `json:"clusterTolerations,omitempty"`
}

type Workload struct {
	// +optional
	Workloadtype string `json:"workloadtype,omitempty"`
	// +optional
	TraitDeployment *TraitDeployment `json:"traitDeployment,omitempty"`
	// +optional
	TraitServerless *TraitServerless `json:"traitServerless,omitempty"`
}
type ServerlessSpec struct {
	// +optional
	Revision RevisionSpec `json:"revision,omitempty"`
	// Traffic specifies how to distribute traffic over a collection of
	// revisions and configurations.
	// +optional
	Traffic []TrafficTarget `json:"traffic,omitempty"`
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

// TrafficTarget holds a single entry of the routing table for a Route.
type TrafficTarget struct {
	// Tag is optionally used to expose a dedicated url for referencing
	// this target exclusively.
	// +optional
	Tag string `json:"tag,omitempty"`

	// RevisionName of a specific revision to which to send this portion of
	// traffic.  This is mutually exclusive with ConfigurationName.
	// +optional
	RevisionName string `json:"revisionName,omitempty"`

	// ConfigurationName of a configuration to whose latest revision we will send
	// this portion of traffic. When the "status.latestReadyRevisionName" of the
	// referenced configuration changes, we will automatically migrate traffic
	// from the prior "latest ready" revision to the new one.  This field is never
	// set in Route's status, only its spec.  This is mutually exclusive with
	// RevisionName.
	// +optional
	ConfigurationName string `json:"configurationName,omitempty"`

	// LatestRevision may be optionally provided to indicate that the latest
	// ready Revision of the Configuration should be used for this traffic
	// target.  When provided LatestRevision must be true if RevisionName is
	// empty; it must be false when RevisionName is non-empty.
	// +optional
	LatestRevision *bool `json:"latestRevision,omitempty"`

	// Percent indicates that percentage based routing should be used and
	// the value indicates the percent of traffic that is be routed to this
	// Revision or Configuration. `0` (zero) mean no traffic, `100` means all
	// traffic.
	// When percentage based routing is being used the follow rules apply:
	// - the sum of all percent values must equal 100
	// - when not specified, the implied value for `percent` is zero for
	//   that particular Revision or Configuration
	// +optional
	Percent *int64 `json:"percent,omitempty"`

	// URL displays the URL for accessing named traffic targets. URL is displayed in
	// status, and is disallowed on spec. URL must contain a scheme (e.g. http://) and
	// a hostname, but may not contain anything else (e.g. basic auth, url path, etc.)
	// +optional
	URL *apis.URL `json:"url,omitempty"`
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
	SpecificResource *metav1.LabelSelector `json:"specificResource,omitempty"`
	// +optional
	Netenvironment *metav1.LabelSelector `json:"netenvironment,omitempty"`
	// +optional
	Geolocation *metav1.LabelSelector `json:"geolocation,omitempty"`
	// +optional
	Provider *metav1.LabelSelector `json:"provider,omitempty"`
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
