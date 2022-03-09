package app

import (
	"github.com/spf13/pflag"
)

// ClusterRegistrationOptions holds the command-line options for command
type options struct {
	kubeconfig string
}

// AddFlags adds the flags to the flagset.
func (opts *options) AddFlags(fs *pflag.FlagSet) {
	fs.StringVar(&opts.kubeconfig, "kubeconfig", opts.kubeconfig,
		"Path to a kubeconfig file for current child cluster. Only required if out-of-cluster")
}

// NewOptions creates a new *options with sane defaults
func NewOptions() *options {
	return &options{}
}
