package v1alpha1

import (
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=cron,categories=gaia
// +kubebuilder:printcolumn:name="KIND",type="string",JSONPath=".spec.resource.kind"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// CronMaster is a cron resource
type CronMaster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CronMasterSpec   `json:"spec"`
	Status            CronMasterStatus `json:"status,omitempty"`
}

type CronMasterSpec struct {
	// +optional
	Schedule SchedulerConfig `json:"schedule,omitempty"`
	// deployment or serverless
	// +required
	Resource ReferenceResource `json:"resource,omitempty"`
}

type NextScheduleType string

const (
	Start     NextScheduleType = "start"
	Stop      NextScheduleType = "stop"
	Processed NextScheduleType = "processed"
)

type ReferenceResource struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// +optional
	Kind string `json:"kind,omitempty"`
	// +optional
	Version string `json:"version,omitempty"`
	// +optional
	Group string `json:"group,omitempty"`
	// deployment, serverless
	RawData []byte `json:"rawData,omitempty"`
}

type CronMasterStatus struct {
	// A list of pointers to currently running jobs.
	// +optional
	// +listType=atomic
	Active []v1.ObjectReference `json:"active,omitempty"`

	// +required
	NextScheduleAction NextScheduleType `json:"nextScheduleAction,omitempty"`
	// +optional
	NextScheduleDateTime *metav1.Time `json:"nextScheduleDateTime,omitempty"`

	// Information when was the last time the cron resource was successfully scheduled.
	// +optional
	LastScheduleTime *metav1.Time `json:"lastScheduleTime,omitempty"`

	// Information when was the last time the cron resource successfully completed.
	// +optional
	LastSuccessfulTime *metav1.Time `json:"lastSuccessfulTime,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CronMasterList contains a list of CronMaster
type CronMasterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CronMaster `json:"items"`
}

func (in *CronMaster) GetResourceKind() string {
	return in.Spec.Resource.Kind
}
