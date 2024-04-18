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
	RBMergerLabel             = "clusters.gaia.io/merger"
	CredentialsAuto           = "credentials-auto"

	AppsNameLabel = "apps.gaia.io/description-name"
	AppFinalizer  = "apps.gaia.io/finalizer"

	DefaultClusterStatusCollectFrequency = 20 * time.Second
	DefaultClusterStatusReportFrequency  = 3 * time.Minute
	// max length for clustername
	ClusterNameMaxLength = 30
	// default length for random uid
	DefaultRandomUIDLength = 5
	// GaiaAppSA is the service account where we store credentials to deploy resources
	GaiaAppSA = "gaia-resource-deployer"

	SpecificNodeLabelsKeyPrefix = "hypernode.cluster.pml.com.cn/"

	ManagedClusterSourceFromInformer   = "informer"
	ManagedClusterSourceFromPrometheus = "prometheus"
	PrometheusUrlPrefix                = "http://prometheus-kube-prometheus-hypermoni.hypermonitor.svc:9090" // current cluster

	// description labels on rb
	GaiaDescriptionLabel            = "apps.gaia.io/description"
	GaiaComponentLabel              = "apps.gaia.io/component"
	OriginDescriptionNameLabel      = "apps.gaia.io/ori.desc.name"
	OriginDescriptionNamespaceLabel = "apps.gaia.io/ori.desc.namespace"
	OriginDescriptionUIDLabel       = "apps.gaia.io/ori.desc.uid"
	UserNameLabel                   = "apps.gaia.io/user.name"

	NetworkLocationCore = "core"
	NodeResourceForm    = "pool"

	TopoSyncBaseUrl = "http://ssiexpose.synccontroller.svc:8080" // network controller address, maybe on global
	TopoSyncUrlPath = "/v1.0/globalsync/topo"

	// env
	ResourceBindingMergerPostURL = "RESOURCEBINDING_MERGER_POST_URL"
	GaiaPushModeClusterName      = "GAIA_PUSH_MODE_CLUSTER_NAME"

	MetricConfigMapAbsFilePath             = "/etc/config/gaia-prometheus_metrics.conf"
	ServiceMaintenanceConfigMapAbsFilePath = "/etc/config/service-maintenance-prometheus_metrics.conf"

	HypernodeClusterNodeRole       = "hypernode.cluster.pml.com.cn/node-role"
	HypernodeClusterNodeRolePublic = "Public"
)

// lease lock
const (
	DefaultLeaseDuration = 80 * time.Second
	DefaultRenewDeadline = 70 * time.Second
	// DefaultRetryPeriod means the default retry period
	DefaultRetryPeriod = 15 * time.Second
	// DefaultResync means the default resync time
	DefaultResync               = time.Hour * 12
	DefaultThreadiness          = 2
	DefaultResouceBindingNumber = 2
)

const (
	// NoteLengthLimit denotes the maximum note length.
	// copied from k8s.io/kubernetes/pkg/apis/core/validation/events.go
	NoteLengthLimit = 1024
)
