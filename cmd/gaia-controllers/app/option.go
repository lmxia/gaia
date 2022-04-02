package app

import (
	"fmt"
	"github.com/lmxia/gaia/pkg/common"
	"github.com/spf13/pflag"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"net/url"
	"regexp"
	"strings"
)

// ClusterRegistrationOptions holds the command-line options for command
type options struct {
	kubeconfig                 string
	clusterHostName            string
	clusterRegistration        *ClusterRegistrationOptions
	managedClusterSource       string
	prometheusMonitorUrlPrefix string
}

// AddFlags adds the flags to the flagset.
func (opts *options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.kubeconfig, "kubeconfig", opts.kubeconfig,
		"Path to a kubeconfig file for current child cluster. Only required if out-of-cluster")
	fs.StringVar(&opts.clusterHostName, "clustername", opts.clusterHostName, "To generate ClusterRegistration name and gaiaName as gaia-'clustername'-UID ")
	fs.StringVar(&opts.managedClusterSource, "mcSource", opts.managedClusterSource,
		"where to get the managerCluster Resource.")
	fs.StringVar(&opts.prometheusMonitorUrlPrefix, "promUrlPrefix", opts.prometheusMonitorUrlPrefix,
		"The prefix of the prometheus monitor url.")
}

// NewOptions creates a new *options with sane defaults
func NewOptions() *options {
	return &options{
		clusterRegistration: NewClusterRegistrationOptions(),
	}
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
func (opts *options) Complete() {
	// complete cluster registration options
	opts.clusterRegistration.Complete()
}

// Complete completes all the required options.
func (opts *ClusterRegistrationOptions) Complete() {
	opts.ClusterName = strings.TrimSpace(opts.ClusterName)
	opts.ClusterNamePrefix = strings.TrimSpace(opts.ClusterNamePrefix)
}

// Validate validates all the required options.
func (opts *options) Validate() error {
	var allErrs []error

	// validate cluster registration options
	errs := opts.clusterRegistration.Validate()
	allErrs = append(allErrs, errs...)

	// validate managedClusterSource and prometheusMonitorUrlPrefix options
	if len(opts.managedClusterSource) > 0 {
		if !validateClusterNameRegex.MatchString(opts.managedClusterSource) {
			allErrs = append(allErrs,
				fmt.Errorf("invalid name for --%s, regex used for validation is %q", common.ClusterRegistrationName, common.NameFmt))
		}

		if opts.managedClusterSource != common.ManagedClusterSourceFromPrometheus && opts.managedClusterSource != common.ManagedClusterSourceFromInformer {
			allErrs = append(allErrs, fmt.Errorf("Invalid value for managedClusterSource --%s, please use 'prometheus' or 'informer'. ", opts.managedClusterSource))
		}

		if len(opts.prometheusMonitorUrlPrefix) > 0 {
			_, err := url.ParseRequestURI(opts.prometheusMonitorUrlPrefix)
			if err != nil {
				allErrs = append(allErrs, fmt.Errorf("invalid value for --%s: %v", opts.prometheusMonitorUrlPrefix, err))
			}
		}
	}
	return utilerrors.NewAggregate(allErrs)
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
