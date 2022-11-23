package option

import (
	"github.com/lmxia/gaia/cmd/gaia-scheduler/config"
	"github.com/spf13/pflag"
	apiserveroptions "k8s.io/apiserver/pkg/server/options"
	"k8s.io/component-base/metrics"
)

// ClusterRegistrationOptions holds the command-line options for command
type Options struct {
	Kubeconfig              string
	CombinedInsecureServing *config.CombinedInsecureServingOptions
	Metrics                 *metrics.Options
}

// AddFlags adds the flags to the flagset.
func (opts *Options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.kubeconfig, "kubeconfig", opts.kubeconfig,
		"Path to a kubeconfig file for current child cluster. Only required if out-of-cluster")
}

// NewOptions creates a new *options with sane defaults
func NewOptions() *Options {
	return &Options{
		CombinedInsecureServing: &config.CombinedInsecureServingOptions{
			Healthz: (&apiserveroptions.DeprecatedInsecureServingOptions{
				BindNetwork: "tcp",
			}).WithLoopback(),
			Metrics: (&apiserveroptions.DeprecatedInsecureServingOptions{
				BindNetwork: "tcp",
			}).WithLoopback(),
			BindPort:    2112,
			BindAddress: "localhost",
		},
		Metrics: metrics.NewOptions(),
	}
}
