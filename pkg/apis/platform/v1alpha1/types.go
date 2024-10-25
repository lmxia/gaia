package v1alpha1

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"

	"github.com/lmxia/gaia/pkg/common"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster",categories=gaia
// +k8s:openapi-gen=true
type Target struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// +optional
	Spec TargetSpec `json:"spec,omitempty"`
	// +optional
	Status TargetStatus `json:"status,omitempty"`
}

type TargetSpec struct {
	// +optional
	Self bool `json:"self,omitempty"`
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// +required
	ReportFrequency *int64 `json:"reportFrequency,omitempty" protobuf:"varint,5,opt,name=reportFrequency"`
	// +required
	CollectFrequency *int64 `json:"collectFrequency,omitempty" protobuf:"varint,5,opt,name=collectFrequency"`
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	ParentURL string `json:"parenturl,omitempty" protobuf:"bytes,1,opt,name=parenturl"`
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	BootstrapToken string `json:"bootstraptoken,omitempty" protobuf:"bytes,1,opt,name=bootstraptoken"`
}

type TargetStatus struct{}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// TargetList contains a list of Target
type TargetList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Target `json:"items"`
}

// ManagedClusterSpec defines the desired state of ManagedCluster
type ManagedClusterSpec struct {
	// ClusterID, a Random (Version 4) UUID, is a unique value in time and space value representing for child cluster.
	// It is typically generated by the controller agent on the successful creation of a "self-cluster" Lease
	// in the child cluster.
	// Also it is not allowed to change on PUT operations.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
	ClusterID types.UID `json:"clusterId"`

	// +optional
	Secondary bool `json:"secondary"`
	// Taints has the "effect" on any resource that does not tolerate the Taint.
	// +optional
	Taints []corev1.Taint `json:"taints,omitempty"`
}

// ManagedClusterStatus defines the observed state of ManagedCluster
type ManagedClusterStatus struct {
	// lastObservedTime is the time when last status from the series was seen before last heartbeat.
	// RFC 3339 date and time at which the object was acknowledged by the controller Agent.
	// +optional
	LastObservedTime metav1.Time `json:"lastObservedTime,omitempty"`

	// k8sVersion is the Kubernetes version of the cluster
	// +optional
	KubernetesVersion string `json:"k8sVersion,omitempty"`

	// platform indicates the running platform of the cluster
	// +optional
	Platform string `json:"platform,omitempty"`

	// APIServerURL indicates the advertising url/address of managed Kubernetes cluster
	// +optional
	APIServerURL string `json:"apiserverURL,omitempty"`

	// Healthz indicates the healthz status of the cluster
	// which is deprecated since Kubernetes v1.16. Please use Livez and Readyz instead.
	// Leave it here only for compatibility.
	// +optional
	Healthz bool `json:"healthz"`

	// Livez indicates the livez status of the cluster
	// +optional
	Livez bool `json:"livez"`

	// Readyz indicates the readyz status of the cluster
	// +optional
	Readyz bool `json:"readyz"`

	// Allocatable is the sum of allocatable resources for nodes in the cluster
	// +optional
	Allocatable corev1.ResourceList `json:"allocatable,omitempty"`

	// Capacity is the sum of capacity resources for nodes in the cluster
	// +optional
	Capacity corev1.ResourceList `json:"capacity,omitempty"`

	// Available is the sum of Available resources for nodes in the cluster
	// +optional
	Available corev1.ResourceList `json:"available,omitempty"`

	// +optional
	GPUs []*GPUResources `json:"gpus,omitempty"`

	// ClusterCIDR is the CIDR range of the cluster
	// +optional
	ClusterCIDR string `json:"clusterCIDR,omitempty"`

	// ServcieCIDR is the CIDR range of the services
	// +optional
	ServiceCIDR string `json:"serviceCIDR,omitempty"`

	// NodeStatistics is the info summary of nodes in the cluster
	// +optional
	NodeStatistics NodeStatistics `json:"nodeStatistics,omitempty"`

	// Conditions is an array of current cluster conditions.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// heartbeatFrequencySeconds is the frequency at which the agent reports current cluster status
	// +optional
	HeartbeatFrequencySeconds *int64 `json:"heartbeatFrequencySeconds,omitempty"`

	// TopologyInfo is the network topology of the cluster.
	//
	// +optional
	// +kubebuilder:validation:Type=object
	TopologyInfo Topo `json:"topologyInfo,omitempty"`
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Namespaced",shortName=mcls,categories=gaia
// +kubebuilder:printcolumn:name="CLUSTER ID",type=string,JSONPath=`.spec.clusterId`,description="Unique id for cluster"
// +kubebuilder:printcolumn:name="KUBERNETES",type=string,JSONPath=".status.k8sVersion"
// +kubebuilder:printcolumn:name="READYZ",type=string,JSONPath=".status.readyz"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ManagedCluster is the Schema for the managedclusters API
type ManagedCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ManagedClusterSpec   `json:"spec,omitempty"`
	Status ManagedClusterStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ManagedClusterList contains a list of ManagedCluster
type ManagedClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ManagedCluster `json:"items"`
}

type GPUResources struct {
	// +optional
	ClusterName string `json:"clusterName,omitempty"`
	// +optional
	Available map[string]int `json:"available,omitempty"`
	Capacity  map[string]int `json:"capacity,omitempty"`
	// +optional
	Children []*GPUResources `json:"children,omitempty"`
}

type NodeStatistics struct {
	// ReadyNodes is the number of ready nodes in the cluster
	// +optional
	ReadyNodes int32 `json:"readyNodes,omitempty"`

	// NotReadyNodes is the number of not ready nodes in the cluster
	// +optional
	NotReadyNodes int32 `json:"notReadyNodes,omitempty"`

	// UnknownNodes is the number of unknown nodes in the cluster
	// +optional
	UnknownNodes int32 `json:"unknownNodes,omitempty"`

	// LostNodes is the number of states lost nodes in the cluster
	// +optional
	LostNodes int32 `json:"lostNodes,omitempty"`
}

const (
	// ClusterReady means cluster is ready.
	ClusterReady = "Ready"
)

// ClusterRegistrationRequestSpec defines the desired state of ClusterRegistrationRequest
type ClusterRegistrationRequestSpec struct {
	// ClusterID, a Random (Version 4) UUID, is a unique value in time and space value representing for child cluster.
	// It is typically generated by the gaia agent on the successful creation of a "self-cluster" Lease
	// in the child cluster.
	// Also it is not allowed to change on PUT operations.
	//
	// +required
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:Pattern="[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}"
	ClusterID types.UID `json:"clusterId"`

	// ClusterNamePrefix is the prefix of cluster name.
	// a lower case alphanumeric characters or '-', and must start and end with an alphanumeric character
	//
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=30
	// +kubebuilder:validation:Pattern="[a-z0-9]([-a-z0-9]*[a-z0-9])?([a-z0-9]([-a-z0-9]*[a-z0-9]))*"
	ClusterNamePrefix string `json:"clusterNamePrefix,omitempty"`

	// ClusterName is the cluster name.
	// a lower case alphanumeric characters or '-', and must start and end with an alphanumeric character
	//
	// +optional
	// +kubebuilder:validation:Type=string
	// +kubebuilder:validation:MaxLength=30
	// +kubebuilder:validation:Pattern="[a-z0-9]([-a-z0-9]*[a-z0-9])?([a-z0-9]([-a-z0-9]*[a-z0-9]))*"
	ClusterName string `json:"clusterName,omitempty"`

	// ClusterLabels is the labels of the child cluster.
	//
	// +optional
	// +kubebuilder:validation:Type=object
	ClusterLabels map[string]string `json:"clusterLabels,omitempty"`

	// Secondary True means this rr is oriented from secondary platform.
	//
	// +optional
	Secondary bool `json:"secondary"`
}

// ClusterRegistrationRequestStatus defines the observed state of ClusterRegistrationRequest
type ClusterRegistrationRequestStatus struct {
	// DedicatedNamespace is a dedicated namespace for the child cluster, which is created in the parent cluster.
	//
	// +optional
	DedicatedNamespace string `json:"dedicatedNamespace,omitempty"`

	// DedicatedToken is populated by parent cluster when Result is RequestApproved.
	// With this token, the client could have full access on the resources created in DedicatedNamespace.
	//
	// +optional
	DedicatedToken []byte `json:"token,omitempty"`

	// CACertificate is the public certificate that is the root of trust for parent cluster
	// The certificate is encoded in PEM format.
	//
	// +optional
	CACertificate []byte `json:"caCertificate,omitempty"`

	// Result indicates whether this request has been approved.
	// When all necessary objects have been created and ready for child cluster registration,
	// this field will be set to "Approved". If any illegal updates on this object, "Illegal" will be set to this filed.
	//
	// +optional
	Result *ApprovedResult `json:"result,omitempty"`

	// ErrorMessage tells the reason why the request is not approved successfully.
	//
	// +optional
	ErrorMessage string `json:"errorMessage,omitempty"`

	// ManagedClusterName is the name of ManagedCluster object in the parent cluster corresponding to the child cluster
	//
	// +optional
	ManagedClusterName string `json:"managedClusterName,omitempty"`
}

type ApprovedResult string

// These are the possible results for a cluster registration request.
const (
	RequestDenied   ApprovedResult = "Denied"
	RequestApproved ApprovedResult = "Approved"
	RequestFailed   ApprovedResult = "Failed"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope="Cluster",shortName=clsrr,categories=gaia
// +kubebuilder:printcolumn:name="CLUSTER ID",type=string,JSONPath=`.spec.clusterId`,description="Unique id for cluster"
// +kubebuilder:printcolumn:name="STATUS",type=string,JSONPath=`.status.result`,description="state of request"
// +kubebuilder:printcolumn:name="AGE",type="date",JSONPath=".metadata.creationTimestamp"

// ClusterRegistrationRequest is the Schema for the clusterregistrationrequests API
type ClusterRegistrationRequest struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ClusterRegistrationRequestSpec   `json:"spec,omitempty"`
	Status ClusterRegistrationRequestStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ClusterRegistrationRequestList contains a list of ClusterRegistrationRequest
type ClusterRegistrationRequestList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ClusterRegistrationRequest `json:"items"`
}

// ManagedClusterOptions holds the command-line options about managedCluster
type ManagedClusterOptions struct {
	// ManagedClusterSource specified where to get the managerCluster Resource.
	ManagedClusterSource string
	// PrometheusMonitorURLPrefix specified the prefix of the prometheus monitor url.
	PrometheusMonitorURLPrefix string
	// TopoSyncBaseURL is the base url of the synccontroller service.
	TopoSyncBaseURL string
	// UseHypernodeController means whether use hypernode controller, default value is false.
	UseHypernodeController bool
}

// NewClusterRegistrationOptions creates a new *ClusterRegistrationOptions with sane defaults
func NewManagedClusterOptions() *ManagedClusterOptions {
	return &ManagedClusterOptions{
		ManagedClusterSource:       common.ManagedClusterSourceFromInformer,
		PrometheusMonitorURLPrefix: common.PrometheusURLPrefix,
		TopoSyncBaseURL:            common.TopoSyncBaseURL,
		UseHypernodeController:     false,
	}
}

// Complete completes all the required options.
func (opts *ManagedClusterOptions) Complete() {
	opts.ManagedClusterSource = strings.TrimSpace(opts.ManagedClusterSource)
	opts.PrometheusMonitorURLPrefix = strings.TrimSpace(opts.PrometheusMonitorURLPrefix)
	opts.TopoSyncBaseURL = strings.TrimSpace(opts.TopoSyncBaseURL)
}

var validateClusterNameRegex = regexp.MustCompile(common.NameFmt)

// Validate validates all the required options.
func (opts *ManagedClusterOptions) Validate() []error {
	var allErrs []error

	// validate managedClusterSource and prometheusMonitorUrlPrefix options
	if len(opts.ManagedClusterSource) > 0 {
		if !validateClusterNameRegex.MatchString(opts.ManagedClusterSource) {
			allErrs = append(allErrs,
				fmt.Errorf("invalid name for --%s, regex used for validation is %q",
					common.ClusterRegistrationName, common.NameFmt))
		}

		if opts.ManagedClusterSource != common.ManagedClusterSourceFromPrometheus &&
			opts.ManagedClusterSource != common.ManagedClusterSourceFromInformer {
			allErrs = append(allErrs, fmt.Errorf("invalid value for managedClusterSource --%s,"+
				" please use 'prometheus' or 'informer'. ", opts.ManagedClusterSource))
		}

		if len(opts.PrometheusMonitorURLPrefix) > 0 {
			_, err := url.ParseRequestURI(opts.PrometheusMonitorURLPrefix)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("invalid value for --%s: %v",
					opts.PrometheusMonitorURLPrefix, err))
			}
		}
	}
	return allErrs
}

type Topos struct {
	Topo []Topo `json:"topo,omitempty"`
}

type Topo struct {
	// field name
	Field string `json:"field,omitempty"`
	// topo in protobuf
	Content string `json:"content,omitempty"`
}

type Fields struct {
	Field []string `json:"field,omitempty"`
}

// GetHypernodeLabelsMapFromManagedCluster  returns hypernode labels of the current cluster from the managedCluster
func (cluster *ManagedCluster) GetHypernodeLabelsMapFromManagedCluster() (
	netEnvironmentMap, nodeRoleMap, resFormMap, runtimeStateMap, snMap, geolocationMap,
	providers, floatingIPMap, gpuProductMap, clusterNameMap, internalNetworkCapMap,
	pricelevelMap, vpcMap, resTypeMap, supportPublicIPMap, tokenMap map[string]struct{},
) {
	netEnvironmentMap, nodeRoleMap, resFormMap, runtimeStateMap, snMap =
		make(map[string]struct{}), make(map[string]struct{}), make(map[string]struct{}),
		make(map[string]struct{}), make(map[string]struct{})
	geolocationMap, providers, floatingIPMap, gpuProductMap =
		make(map[string]struct{}), make(map[string]struct{}), make(map[string]struct{}),
		make(map[string]struct{})
	clusterNameMap, internalNetworkCapMap, pricelevelMap =
		make(map[string]struct{}), make(map[string]struct{}), make(map[string]struct{})
	vpcMap, resTypeMap, supportPublicIPMap, tokenMap = make(map[string]struct{}), make(map[string]struct{}),
		make(map[string]struct{}), make(map[string]struct{})
	var labelValueArray []string

	clusterLabels := cluster.GetLabels()
	for labelKey, labelValue := range clusterLabels {
		if strings.HasPrefix(labelKey, common.SpecificNodeLabelsKeyPrefix) {
			if strings.Contains(labelValue, "__") {
				labelValueArray = strings.Split(labelValue, "__")
			} else {
				labelValueArray = []string{labelValue}
			}
			switch {
			case strings.HasPrefix(labelKey, ParsedNetEnvironmentKey):
				for _, v := range labelValueArray {
					netEnvironmentMap[v] = struct{}{}
				}
			case strings.HasPrefix(labelKey, ParsedNodeRoleKey):
				for _, v := range labelValueArray {
					nodeRoleMap[v] = struct{}{}
				}
			case strings.HasPrefix(labelKey, ParsedResFormKey):
				for _, v := range labelValueArray {
					resFormMap[v] = struct{}{}
				}
			case strings.HasPrefix(labelKey, ParsedRuntimeStateKey):
				for _, v := range labelValueArray {
					runtimeStateMap[v] = struct{}{}
				}
			case strings.HasPrefix(labelKey, ParsedSNKey):
				snMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedGeoLocationKey):
				geolocationMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedProviderKey):
				providers[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedHasFloatingIPKey):
				floatingIPMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedGPUProductKey):
				gpuProductMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedClusterNameKey):
				clusterNameMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedInternalNetworkCapKey):
				internalNetworkCapMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedPricelevelKey):
				pricelevelMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedVPCKey):
				vpcMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedResTypeKey):
				resTypeMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedSupportPublicIPKey):
				supportPublicIPMap[labelValue] = struct{}{}
			case strings.HasPrefix(labelKey, ParsedTokenKey):
				tokenMap[labelValue] = struct{}{}
			}
		}
	}
	return netEnvironmentMap, nodeRoleMap, resFormMap, runtimeStateMap, snMap, geolocationMap,
		providers, floatingIPMap, gpuProductMap, clusterNameMap, internalNetworkCapMap,
		pricelevelMap, vpcMap, resTypeMap, supportPublicIPMap, tokenMap
}

// GetGaiaLabels return gaia type labels.
func (cluster *ManagedCluster) GetGaiaLabels() map[string][]string {
	netEnvironmentMap, nodeRoleMap, resFormMap, runtimeStateMap, snMap, geolocationMap, providersMap,
		floatingIPMap, gpuProductMap, clusterNameMap, internalNetworkCapMap, pricelevelMap,
		vpcMap, resTypeMap, supportPublicIPMap, tokenMap :=
		cluster.GetHypernodeLabelsMapFromManagedCluster()
	clusterLabels := make(map[string][]string, 0)

	// no.1
	netEnvironments := make([]string, 0, len(netEnvironmentMap))
	for k := range netEnvironmentMap {
		netEnvironments = append(netEnvironments, k)
	}
	clusterLabels["net-environment"] = netEnvironments
	// no.2
	nodeRoles := make([]string, 0, len(nodeRoleMap))
	for k := range nodeRoleMap {
		nodeRoles = append(nodeRoles, k)
	}
	clusterLabels["node-role"] = nodeRoles
	// no.3
	resForms := make([]string, 0, len(resFormMap))
	for k := range resFormMap {
		resForms = append(resForms, k)
	}
	clusterLabels["res-form"] = resForms
	// no.4
	runtimeStates := make([]string, 0, len(runtimeStateMap))
	for k := range runtimeStateMap {
		runtimeStates = append(runtimeStates, k)
	}
	clusterLabels["runtime-state"] = runtimeStates
	// no.5
	sns := make([]string, 0, len(snMap))
	for k := range snMap {
		sns = append(sns, k)
	}
	clusterLabels["sn"] = sns
	// no.6
	geolocations := make([]string, 0, len(geolocationMap))
	for k := range geolocationMap {
		geolocations = append(geolocations, k)
	}
	clusterLabels["geo-location"] = geolocations
	// no.7
	providers := make([]string, 0, len(providersMap))
	for k := range providersMap {
		providers = append(providers, k)
	}
	clusterLabels["supplier-name"] = providers
	// no.8
	hasFloatingIPs := make([]string, 0, len(floatingIPMap))
	for k := range floatingIPMap {
		hasFloatingIPs = append(hasFloatingIPs, k)
	}
	clusterLabels["has-floating-ip"] = hasFloatingIPs
	// gpuProduct
	gpuProducts := make([]string, 0, len(gpuProductMap))
	for k := range gpuProductMap {
		gpuProducts = append(gpuProducts, k)
	}
	clusterLabels["gpu-product"] = gpuProducts

	// cluster-name
	clusterNames := make([]string, 0, len(clusterNameMap))
	for k := range clusterNameMap {
		clusterNames = append(clusterNames, k)
	}
	clusterLabels["cluster-name"] = clusterNames
	// internal-network-cap
	internalNetworkCaps := make([]string, 0, len(internalNetworkCapMap))
	for k := range internalNetworkCapMap {
		internalNetworkCaps = append(internalNetworkCaps, k)
	}
	clusterLabels["internal-network-cap"] = internalNetworkCaps
	// price-level
	priceLevels := make([]string, 0, len(pricelevelMap))
	for k := range pricelevelMap {
		priceLevels = append(priceLevels, k)
	}
	clusterLabels["price-level"] = priceLevels
	// vpc
	vpc := make([]string, 0, len(vpcMap))
	for k := range vpcMap {
		vpc = append(vpc, k)
	}
	clusterLabels["vpc"] = vpc
	// resType
	resTypes := make([]string, 0, len(resTypeMap))
	for k := range resTypeMap {
		resTypes = append(resTypes, k)
	}
	clusterLabels["res-type"] = resTypes
	// support-public-ip
	supportPublicIP := make([]string, 0, len(supportPublicIPMap))
	for k := range supportPublicIPMap {
		supportPublicIP = append(supportPublicIP, k)
	}
	clusterLabels["support-public-ip"] = supportPublicIP
	// token
	token := make([]string, 0, len(tokenMap))
	for k := range tokenMap {
		token = append(token, k)
	}
	clusterLabels["token"] = token

	return clusterLabels
}

var (
	GeoLocationKey        = common.SpecificNodeLabelsKeyPrefix + "GeoLocation"
	NetEnvironmentKey     = common.SpecificNodeLabelsKeyPrefix + "NetEnvironment"
	NodeRoleKey           = common.SpecificNodeLabelsKeyPrefix + "NodeRole"
	ResFormKey            = common.SpecificNodeLabelsKeyPrefix + "ResForm"
	RuntimeStateKey       = common.SpecificNodeLabelsKeyPrefix + "RuntimeState"
	SNKey                 = common.SpecificNodeLabelsKeyPrefix + "SN"
	SupplierNameKey       = common.SpecificNodeLabelsKeyPrefix + "SupplierName"
	FloatingIPKey         = common.SpecificNodeLabelsKeyPrefix + "FloatingIP"
	HasFloatingIPKey      = common.SpecificNodeLabelsKeyPrefix + "HasFloatingIP"
	VPCKey                = common.SpecificNodeLabelsKeyPrefix + "VPC"
	ClusterNameKey        = common.SpecificNodeLabelsKeyPrefix + "ClusterName"
	GPUProductKey         = common.SpecificNodeLabelsKeyPrefix + "GPUProduct"
	GPUCountKey           = common.SpecificNodeLabelsKeyPrefix + "GPUCount"
	InternalNetworkCapKey = common.SpecificNodeLabelsKeyPrefix + "InternalNetworkCap"
	PricelevelKey         = common.SpecificNodeLabelsKeyPrefix + "Pricelevel"
	ResNameKey            = common.SpecificNodeLabelsKeyPrefix + "ResName"
	ResTypeKey            = common.SpecificNodeLabelsKeyPrefix + "ResType"
	SupportPublicIPKey    = common.SpecificNodeLabelsKeyPrefix + "SupportPublicIP"
	TokenKey              = common.SpecificNodeLabelsKeyPrefix + "Token"

	HypernodeLabelKeyList = []string{
		GeoLocationKey,
		NetEnvironmentKey,
		NodeRoleKey,
		ResFormKey,
		RuntimeStateKey,
		SNKey,
		SupplierNameKey,
		FloatingIPKey,
		HasFloatingIPKey,
		VPCKey,
		ClusterNameKey,
		GPUProductKey,
		GPUCountKey,
		InternalNetworkCapKey,
		PricelevelKey,
		ResNameKey,
		ResTypeKey,
		SupportPublicIPKey,
		TokenKey,
	}

	ParsedGeoLocationKey        = common.SpecificNodeLabelsKeyPrefix + "geo-location"
	ParsedNetEnvironmentKey     = common.SpecificNodeLabelsKeyPrefix + "net-environment"
	ParsedNodeRoleKey           = common.SpecificNodeLabelsKeyPrefix + "node-role"
	ParsedResFormKey            = common.SpecificNodeLabelsKeyPrefix + "res-form"
	ParsedRuntimeStateKey       = common.SpecificNodeLabelsKeyPrefix + "runtime-state"
	ParsedSNKey                 = common.SpecificNodeLabelsKeyPrefix + "sn"
	ParsedProviderKey           = common.SpecificNodeLabelsKeyPrefix + "supplier-name"
	ParsedFloatingIPKey         = common.SpecificNodeLabelsKeyPrefix + "floating-ip"
	ParsedHasFloatingIPKey      = common.SpecificNodeLabelsKeyPrefix + "has-floating-ip"
	ParsedVPCKey                = common.SpecificNodeLabelsKeyPrefix + "vpc"
	ParsedClusterNameKey        = common.SpecificNodeLabelsKeyPrefix + "cluster-name"
	ParsedGPUProductKey         = common.SpecificNodeLabelsKeyPrefix + "gpu-product"
	ParsedGPUCountKey           = common.SpecificNodeLabelsKeyPrefix + "gpu-count"
	ParsedInternalNetworkCapKey = common.SpecificNodeLabelsKeyPrefix + "internal-network-cap"
	ParsedPricelevelKey         = common.SpecificNodeLabelsKeyPrefix + "price-level"
	ParsedResNameKey            = common.SpecificNodeLabelsKeyPrefix + "res-name"
	ParsedResTypeKey            = common.SpecificNodeLabelsKeyPrefix + "res-type"
	ParsedSupportPublicIPKey    = common.SpecificNodeLabelsKeyPrefix + "support-public-ip"
	ParsedTokenKey              = common.SpecificNodeLabelsKeyPrefix + "token"

	ParsedHypernodeLabelKeyList = []string{
		ParsedGeoLocationKey,
		ParsedNetEnvironmentKey,
		ParsedNodeRoleKey,
		ParsedResFormKey,
		ParsedRuntimeStateKey,
		ParsedSNKey,
		ParsedProviderKey,
		ParsedFloatingIPKey,
		ParsedHasFloatingIPKey,
		ParsedVPCKey,
		ParsedClusterNameKey,
		ParsedGPUProductKey,
		ParsedGPUCountKey,
		ParsedInternalNetworkCapKey,
		ParsedPricelevelKey,
		ParsedResNameKey,
		ParsedResTypeKey,
		ParsedSupportPublicIPKey,
		ParsedTokenKey,
	}

	HypernodeLabelKeyToStandardLabelKey = map[string]string{
		GeoLocationKey:        ParsedGeoLocationKey,
		NetEnvironmentKey:     ParsedNetEnvironmentKey,
		NodeRoleKey:           ParsedNodeRoleKey,
		ResFormKey:            ParsedResFormKey,
		RuntimeStateKey:       ParsedRuntimeStateKey,
		SNKey:                 ParsedSNKey,
		SupplierNameKey:       ParsedProviderKey,
		FloatingIPKey:         ParsedFloatingIPKey,
		HasFloatingIPKey:      ParsedHasFloatingIPKey,
		VPCKey:                ParsedVPCKey,
		ClusterNameKey:        ParsedClusterNameKey,
		GPUProductKey:         ParsedGPUProductKey,
		GPUCountKey:           ParsedGPUCountKey,
		InternalNetworkCapKey: ParsedInternalNetworkCapKey,
		PricelevelKey:         ParsedPricelevelKey,
		ResNameKey:            ParsedResNameKey,
		ResTypeKey:            ParsedResTypeKey,
		SupportPublicIPKey:    ParsedSupportPublicIPKey,
		TokenKey:              ParsedTokenKey,
	}
)
