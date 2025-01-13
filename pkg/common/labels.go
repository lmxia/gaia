// copied from clusternet
package common

// label key
const (
	ObjectCreatedByLabel = "gaia.io/created-by"
)

// label value
const (
	RBACDefaults          = "rbac-defaults"
	GaiaControllerManager = "gaia-controller-manager"
	StatusScheduler       = "status-scheduler"

	// description labels on rb
	GaiaDescriptionLabel            = "apps.gaia.io/description"
	GaiaComponentLabel              = "apps.gaia.io/component"
	OriginDescriptionNameLabel      = "apps.gaia.io/ori.desc.name"
	OriginDescriptionNamespaceLabel = "apps.gaia.io/ori.desc.namespace"
	OriginDescriptionUIDLabel       = "apps.gaia.io/ori.desc.uid"
	OriginResourceBindingNameLabel  = "apps.gaia.io/ori.rb.name"
	UserNameLabel                   = "apps.gaia.io/user.name"

	ParentRBNameLabel  = "apps.gaia.io/parent-rb-name"
	GlobalRBNameLabel  = "apps.gaia.io/global-rb-name"
	FieldRBNameLabel   = "apps.gaia.io/field-rb-name"
	ClusterRBNameLabel = "apps.gaia.io/cluster-rb-name"

	// ParentRBTotalPeer  = "apps.gaia.io/total-peer-of-parent-rb"
	TotalPeerGlobalRB  = "apps.gaia.io/global-total-peer"
	TotalPeerFieldRB   = "apps.gaia.io/field-total-peer"
	TotalPeerClusterRB = "apps.gaia.io/cluster-total-peer"

	ParentNonZeroClusterNum  = "apps.gaia.io/parent-non-zero-cluster-num"
	NonZeroClusterNumGlobal  = "apps.gaia.io/global-non-zero-cluster-num"
	NonZeroClusterNumField   = "apps.gaia.io/field-non-zero-cluster-num"
	NonZeroClusterNumCluster = "apps.gaia.io/cluster-non-zero-cluster-num"

	// GPU Resources
	GPUProductLabel = "gpu-product"
	// GPUCountLabel        = "gpuCount"
	// GPUMemoryLabel       = "gpuMemory"
	// GPUscheduleTypeLabel = "scheduleType"
	// GPUIsolateLabel      = "gpuIsolate"
)
