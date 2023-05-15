package option

import (
	"fmt"
	schedulerappconfig "github.com/lmxia/gaia/cmd/gaia-scheduler/app/config"
	"github.com/spf13/pflag"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	cliflag "k8s.io/component-base/cli/flag"
	"k8s.io/component-base/metrics"
	netutils "k8s.io/utils/net"
	"net"
)

// ClusterRegistrationOptions holds the command-line options for command
type Options struct {
	Kubeconfig string

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
}

// NewOptions creates a new *options with sane defaults
func NewOptions() *Options {
	o := &Options{
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
	o.SecureServing.ServerCert.PairName = "gaia-scheduler"
	o.SecureServing.BindPort = 12116

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

// Validate validates all the required options.
func (o *Options) Validate() []error {
	var errs []error

	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.Authentication.Validate()...)
	errs = append(errs, o.Authorization.Validate()...)
	errs = append(errs, o.Metrics.Validate()...)

	return errs
}

// Config return a scheduler config object
func (o *Options) Config() (*schedulerappconfig.Config, error) {
	if o.SecureServing != nil {
		if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{netutils.ParseIPSloppy("127.0.0.1")}); err != nil {
			return nil, fmt.Errorf("error creating self-signed certificates: %v", err)
		}
	}
	c := &schedulerappconfig.Config{}
	if err := o.ApplyTo(c); err != nil {
		return nil, err
	}

	return c, nil
}

// ApplyTo applies the scheduler options to the given scheduler app configuration.
func (o *Options) ApplyTo(c *schedulerappconfig.Config) error {
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
