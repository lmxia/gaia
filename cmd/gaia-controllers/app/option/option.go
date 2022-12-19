package option

import (
	"fmt"
	clusterapi "github.com/lmxia/gaia/pkg/apis/platform/v1alpha1"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/component-base/metrics"
	"net/url"
	"regexp"
	"strings"

	gaiaappconfig "github.com/lmxia/gaia/cmd/gaia-controllers/app/config"
	cliflag "k8s.io/component-base/cli/flag"
	netutils "k8s.io/utils/net"
	"net"
)

// ClusterRegistrationOptions holds the command-line options for command
type Options struct {
	Kubeconfig          string
	ClusterHostName     string
	NetworkBindUrl      string
	ClusterRegistration *ClusterRegistrationOptions
	ManagedCluster      *clusterapi.ManagedClusterOptions

	SecureServing  *apiserveroptions.SecureServingOptionsWithLoopback
	Authentication *apiserveroptions.DelegatingAuthenticationOptions
	Authorization  *apiserveroptions.DelegatingAuthorizationOptions
	Metrics        *metrics.Options

	// Flags hold the parsed CLI flags.
	Flags *cliflag.NamedFlagSets
}

// AddFlags adds the flags to the flagset.
func (opts *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.Kubeconfig, "kubeconfig", opts.Kubeconfig,
		"Path to a kubeconfig file for current child cluster. Only required if out-of-cluster")
	fs.StringVar(&opts.ClusterHostName, "clustername", opts.ClusterHostName, "To generate ClusterRegistration name and gaiaName as gaia-'clustername'-UID ")
	fs.StringVar(&opts.NetworkBindUrl, "networkBindUrl", opts.NetworkBindUrl, "send network path Url")
	fs.StringVar(&opts.ManagedCluster.ManagedClusterSource, "mcSource", opts.ManagedCluster.ManagedClusterSource,
		"where to get the managerCluster Resource.")
	fs.StringVar(&opts.ManagedCluster.PrometheusMonitorUrlPrefix, "promUrlPrefix", opts.ManagedCluster.PrometheusMonitorUrlPrefix,
		"The prefix of the prometheus monitor url.")
	fs.StringVar(&opts.ManagedCluster.TopoSyncBaseUrl, "topoSyncBaseUrl", opts.ManagedCluster.TopoSyncBaseUrl,
		"The base url of the synccontroller service.")
	fs.BoolVar(&opts.ManagedCluster.UseHypernodeController, "useHypernodeController", opts.ManagedCluster.UseHypernodeController,
		"Whether use hypernode controller, default value is false.")
}

// NewOptions creates a new *options with sane defaults
func NewOptions() *Options {
	o := &Options{
		ClusterRegistration: NewClusterRegistrationOptions(),
		ManagedCluster:      clusterapi.NewManagedClusterOptions(),

		SecureServing:  apiserveroptions.NewSecureServingOptions().WithLoopback(),
		Authentication: apiserveroptions.NewDelegatingAuthenticationOptions(),
		Authorization:  apiserveroptions.NewDelegatingAuthorizationOptions(),
		Metrics:        metrics.NewOptions(),
	}

	o.Authentication.TolerateInClusterLookupFailure = true
	o.Authentication.RemoteKubeConfigFileOptional = true
	o.Authorization.RemoteKubeConfigFileOptional = true

	// Set the PairName but leave certificate directory blank to generate in-memory by default
	o.SecureServing.ServerCert.CertDirectory = ""
	o.SecureServing.ServerCert.PairName = "gaia-controller-manager"
	o.SecureServing.BindPort = 2111

	o.initFlags()

	return o
}

// initFlags initializes flags by section name.
func (o *Options) initFlags() {
	if o.Flags != nil {
		return
	}

	nfs := cliflag.NamedFlagSets{}
	o.SecureServing.AddFlags(nfs.FlagSet("secure serving"))
	o.Authentication.AddFlags(nfs.FlagSet("authentication"))
	o.Authorization.AddFlags(nfs.FlagSet("authorization"))
	o.Metrics.AddFlags(nfs.FlagSet("metrics"))

	o.Flags = &nfs
}

var validateClusterNameRegex = regexp.MustCompile(common.NameFmt)

// ClusterRegistrationOptions holds the command-line options about cluster registration
type ClusterRegistrationOptions struct {
	// ClusterName denotes the cluster name you want to register/display in parent cluster
	ClusterName string
	// ClusterNamePrefix specifies the cluster name prefix for registration
	ClusterNamePrefix string
	// ClusterLabels specifies the labels for the cluster
	ClusterLabels string

	// ClusterStatusReportFrequency is the frequency at which the agent reports current cluster's status
	ClusterStatusReportFrequency metav1.Duration
	// ClusterStatusCollectFrequency is the frequency at which the agent updates current cluster's status
	ClusterStatusCollectFrequency metav1.Duration

	ParentURL      string
	BootstrapToken string
}

// NewClusterRegistrationOptions creates a new *ClusterRegistrationOptions with sane defaults
func NewClusterRegistrationOptions() *ClusterRegistrationOptions {
	return &ClusterRegistrationOptions{
		ClusterNamePrefix:             common.RegistrationNamePrefix,
		ClusterStatusReportFrequency:  metav1.Duration{Duration: common.DefaultClusterStatusReportFrequency},
		ClusterStatusCollectFrequency: metav1.Duration{Duration: common.DefaultClusterStatusCollectFrequency},
	}
}

// Complete completes all the required options.
func (opts *Options) Complete() {
	// complete cluster registration options
	opts.ClusterRegistration.Complete()
	opts.ManagedCluster.Complete()
}

// Validate validates all the required options.
func (opts *Options) Validate() []error {
	var allErrs []error

	// validate cluster registration options
	errs := opts.ClusterRegistration.Validate()
	allErrs = append(allErrs, errs...)

	allErrs = append(allErrs, opts.SecureServing.Validate()...)
	allErrs = append(allErrs, opts.Authentication.Validate()...)
	allErrs = append(allErrs, opts.Authorization.Validate()...)
	allErrs = append(allErrs, opts.Metrics.Validate()...)

	// validate managed cluster options
	mcErrs := opts.ManagedCluster.Validate()
	allErrs = append(allErrs, mcErrs...)

	return allErrs
}


// Config return a gaia-controller-manager config object
func (o *Options) Config() (*gaiaappconfig.Config, error) {
	if o.SecureServing != nil {
		if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
			return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
		}
	}
	c := &gaiaappconfig.Config{}
	if err := o.ApplyTo(c); err != nil {
		return nil, err
	}

	return c, nil
}

// ApplyTo applies the gaia-controller-manager options to the given gaia-controller-manager app configuration.
func (o *Options) ApplyTo(c *gaiaappconfig.Config) error {
	if err := o.SecureServing.ApplyTo(&c.SecureServing, &c.LoopbackClientConfig); err != nil {
		return err
	}
	if o.SecureServing != nil && (o.SecureServing.BindPort != 0 || o.SecureServing.Listener != nil) {
		if err := o.Authentication.ApplyTo(&c.Authentication, c.SecureServing, nil); err != nil {
			return err
		}
		if err := o.Authorization.ApplyTo(&c.Authorization); err != nil {
			return err
		}
	}
	o.Metrics.Apply()
	return nil
}

// Complete completes all the required options.
func (opts *ClusterRegistrationOptions) Complete() {
	opts.ClusterName = strings.TrimSpace(opts.ClusterName)
	opts.ClusterNamePrefix = strings.TrimSpace(opts.ClusterNamePrefix)
}

// Validate validates all the required options.
func (opts *ClusterRegistrationOptions) Validate() []error {
	var allErrs []error

	if len(opts.ParentURL) > 0 {
		_, err := url.ParseRequestURI(opts.ParentURL)
		if err != nil {
			allErrs = append(allErrs, fmt.Errorf("invalid value for --%s: %v", common.ClusterRegistrationURL, err))
		}
	}

	if len(opts.ClusterName) > 0 {
		if len(opts.ClusterName) > common.ClusterNameMaxLength {
			allErrs = append(allErrs, fmt.Errorf("cluster name %s is longer than %d", opts.ClusterName, common.ClusterNameMaxLength))
		}

		if !validateClusterNameRegex.MatchString(opts.ClusterName) {
			allErrs = append(allErrs,
				fmt.Errorf("invalid name for --%s, regex used for validation is %q", common.ClusterRegistrationName, common.NameFmt))
		}
	}
	return allErrs
}
