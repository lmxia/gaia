package common

import "time"

// default values
const (
	NameFmt = "[a-z0-9]([-a-z0-9]*[a-z0-9])?([a-z0-9]([-a-z0-9]*[a-z0-9]))*"
	// RegistrationNamePrefix is a prefix name for cluster registration
	RegistrationNamePrefix = "gaia-cluster"
	// such as namespace, sa, etc
	NamePrefixForGaiaObjects = "gaia-"
	SubCluster               = "Gaia-Controllermanager"

	// ClusterRegistrationURL flag denotes the url of parent cluster
	ClusterRegistrationURL = "cluster-reg-parent-url"
	// ClusterRegistrationName flag specifies the cluster registration name
	ClusterRegistrationName           = "cluster-reg-name"
	GaiaControllerLeaseName           = "self-cluster"
	GaiaSchedulerLeaseName            = "self-scheduler"
	GaiaSystemNamespace               = "gaia-system"
	GaiaReservedNamespace             = "gaia-reserved"
	GaiaFrontendNamespace             = "gaia-frontend"
	GaiaRSToBeMergedReservedNamespace = "gaia-to-be-merged"
	GaiaRBMergedReservedNamespace     = "gaia-merged"
	GaiaPushReservedNamespace         = "gaia-push-reserved"
	// ClusterAPIServerURLKey denotes the apiserver address
	ClusterAPIServerURLKey  = "apiserver-advertise-url"
	ParentClusterSecretName = "parent-cluster"
	ParentClusterTargetName = "parent-cluster"

	ClusterRegisteredByLabel  = "clusters.gaia.io/registered-by"
	ClusterIDLabel            = "clusters.gaia.io/cluster-id"
	ClusterNameLabel          = "clusters.gaia.io/cluster-name"
	ClusterBootstrappingLabel = "clusters.gaia.io/bootstrapping"
	YuanLaoClusterLabel       = "clusters.gaia.io/yuanlao"
	RBMergerLabel             = "clusters.gaia.io/merger"
	CredentialsAuto           = "credentials-auto"

	GaiaAppsLabelPrefix = "apps.gaia.io/"
	AppFinalizer        = "apps.gaia.io/finalizer"
	HyperLabelFinalizer = "service.gaia.io/finalizer"

	DefaultClusterStatusCollectFrequency = 20 * time.Second
	DefaultClusterStatusReportFrequency  = 3 * time.Minute

	DefaultAppStatusCollectFrequency = 10 * time.Second
	// max length for clustername
	ClusterNameMaxLength = 30
	// default length for random uid
	DefaultRandomUIDLength = 5
	// GaiaAppSA is the service account where we store credentials to deploy resources
	GaiaAppSA = "gaia-resource-deployer"

	SpecificNodeLabelsKeyPrefix = "hypernode.cluster.pml.com.cn/"
	VpcLabel                    = "vpc"

	ManagedClusterSourceFromInformer   = "informer"
	ManagedClusterSourceFromPrometheus = "prometheus"
	// current cluster
	PrometheusURLPrefix = "http://prometheus-kube-prometheus-hypermoni.hypermonitor.svc:9090"

	NetPlanLabel          = "apps.gaia.io/net-plan" // ip or identification
	IPNetPlan             = "ip"
	IdentificationNetPlan = "identification"

	NetworkLocationCore = "core"
	NodeResourceForm    = "pool"

	TopoSyncBaseURL = "http://ssiexpose.synccontroller.svc:8080" // network controller address, maybe on global
	TopoSyncURLPath = "/v1.0/globalsync/topo"

	// env
	ResourceBindingMergerPostURL = "RESOURCEBINDING_MERGER_POST_URL"

	MetricConfigMapAbsFilePath             = "/etc/config/gaia-prometheus_metrics.conf"
	ServiceMaintenanceConfigMapAbsFilePath = "/etc/config/service-maintenance-prometheus_metrics.conf"
	MetricConfigMapNodeAbsFilePath         = "/etc/config/gaia-prometheus_node_metrics.conf"

	HypernodeClusterNodeRole       = "hypernode.cluster.pml.com.cn/node-role"
	HypernodeClusterNodeRolePublic = "Public"

	FrontendAliyunCdnName             = "aliyun"
	FrontendAliyunCdnEndpoint         = "cdn.aliyuncs.com"
	FrontendAliyunFinalizers          = "apps.gaia.io/aliyunfinalizer"
	FrontendAliyunCdnRegionID         = "cn-hangzhou"
	FrontendAliyunCdnExist            = true
	FrontendAliyunCdnNoExist          = false
	FrontendAliyunDNSCnameExist       = true
	FrontendAliyunDNSCnameNoExist     = false
	FrontendAliyunCdnOnlineStatus     = "online"
	FrontendAliyunCdnConfigureStatus  = "configuring"
	FrontendAliyunCdnVhostKind        = "Vhost"
	FrontendAliyunCdnVhostAPIVersion  = "frontend.pml.com.cn/v1"
	FrontendAliyunCdnVhostFinalizer   = "apps.gaia.io/vhostfinalizer"
	FrontendAliyunCdnTagUserID        = "userID"
	FrontendAliyunCdnTagComponentName = "componentName"
	FrontendAliyunCdnTagSupplierName  = "supplierName"
	FrontendAliyunCdnTagDomainName    = "domainName"

	FrontendAliyunCdnSleepWait  = 1000
	FrontendAliyunCdnSleepError = 2000

	RBKind         = "ResourceBinding"
	GaiaAPIVersion = "apps.gaia.io/v1alpha1"

	ClusterLayer = "cluster"
	FieldLayer   = "field"
	GlobalLayer  = "global"
)

// lease lock
const (
	DefaultLeaseDuration = 30 * time.Second
	DefaultRenewDeadline = 20 * time.Second
	// DefaultRetryPeriod means the default retry period
	DefaultRetryPeriod = 10 * time.Second
	// DefaultResync means the default resync time
	DefaultResync      = time.Hour * 12
	DefaultThreadiness = 2

	WatcherCheckPeriod = 30 * time.Second
	WatcherTimeOut     = 120 * time.Second
)

const (
	// NoteLengthLimit denotes the maximum note length.
	// copied from k8s.io/kubernetes/pkg/apis/core/validation/events.go
	NoteLengthLimit = 1024
)

const (
	// component name
	ServiceComponentName = "service.gaia.io/componentName"

	// vn
	ServiceManagedByLabel = "service.gaia.io/managedby"
	ServiceCreatedByLabel = "service.gaia.io/createdby"

	// 类型。serverless 还是 非serverless
	ServiceOwnerByLabel = "service.gaia.io/ownby"
	ServicelessOwner    = "serverless"

	// hyperlabel 名字，需要打到service里
	HyperLabelName = "hyperLabelName"

	ExposeTypeCENI      = "ceni"
	ExposeTypePublic    = "public"
	VirtualClusterIPKey = "services.gaia.io/clusterip"
)

const (
	CletName = "clet"

	MonitorNamespaces           = "hypermonitor"
	GaiaRelatedNamespaces       = "gaia-"
	KubeSystemRelatedNamespaces = "kube-"
)

// fields should be ignored when compared
const (
	MetaGeneration      = "/metadata/generation"
	CreationTimestamp   = "/metadata/creationTimestamp"
	ManagedFields       = "/metadata/managedFields"
	MetaUID             = "/metadata/uid"
	MetaSelflink        = "/metadata/selfLink"
	MetaResourceVersion = "/metadata/resourceVersion"

	SectionStatus = "/status"
)
