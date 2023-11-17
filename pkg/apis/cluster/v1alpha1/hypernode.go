package v1alpha1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:resource:scope="Namespaced",shortName=hn,categories=gaia
// +kubebuilder:printcolumn:name="MyArea Name",type=string,JSONPath=".spec.myAreaName"
// +kubebuilder:printcolumn:name="NodeArea Type",type=string,JSONPath=".spec.nodeAreaType"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// Hypernode is a specification for a Hypernode resource
type Hypernode struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec HypernodeSpec `json:"spec"`
}

// HypernodeSpec is the spec for a Hypernode resource
type HypernodeSpec struct {
	MyAreaName     string `json:"myAreaName"`
	NodeAreaType   string `json:"nodeAreaType"`
	SupervisorName string `json:"supervisorName"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// HypernodeList is a list of Foo resources
type HypernodeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Hypernode `json:"items"`
}
