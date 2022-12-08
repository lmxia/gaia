/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package config

import (
	"fmt"
	"github.com/spf13/pflag"
	"net"
)

// DeprecatedInsecureServingInfo is the main context object for the insecure http server.
// HTTP does NOT include authentication or authorization.
// You shouldn't be using this.  It makes sig-auth sad.
type DeprecatedInsecureServingInfo struct {
	// Listener is the secure server network listener.
	Listener net.Listener
	// optional server name for log messages
	Name string
}

// CombinedInsecureServingOptions sets up to two insecure listeners for healthz and metrics. The flags
// override the ComponentConfig and DeprecatedInsecureServingOptions values for both.
type CombinedInsecureServingOptions struct {
	Healthz *DeprecatedInsecureServingOptionsWithLoopback
	Metrics *DeprecatedInsecureServingOptionsWithLoopback

	BindPort    int    // overrides the structs above on ApplyTo, ignored on ApplyToFromLoadedConfig
	BindAddress string // overrides the structs above on ApplyTo, ignored on ApplyToFromLoadedConfig
}

type DeprecatedInsecureServingOptions struct {
	BindAddress net.IP
	BindPort    int
	// BindNetwork is the type of network to bind to - defaults to "tcp", accepts "tcp",
	// "tcp4", and "tcp6".
	BindNetwork string

	// Listener is the secure server network listener.
	// either Listener or BindAddress/BindPort/BindNetwork is set,
	// if Listener is set, use it and omit BindAddress/BindPort/BindNetwork.
	Listener net.Listener

	// ListenFunc can be overridden to create a custom listener, e.g. for mocking in tests.
	// It defaults to options.CreateListener.
	ListenFunc func(network, addr string, config net.ListenConfig) (net.Listener, int, error)
}

// AddFlags adds flags for the insecure serving options.
func (o *CombinedInsecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	if o == nil {
		return
	}

	fs.StringVar(&o.BindAddress, "address", o.BindAddress, "DEPRECATED: the IP address on which to listen for the --port port (set to 0.0.0.0 or :: for listening in all interfaces and IP families). See --bind-address instead. This parameter is ignored if a config file is specified in --config.")
	// MarkDeprecated hides the flag from the help. We don't want that:
	// fs.MarkDeprecated("address", "see --bind-address instead.")
	fs.IntVar(&o.BindPort, "port", o.BindPort, "DEPRECATED: the port on which to serve HTTP insecurely without authentication and authorization. If 0, don't serve plain HTTP at all. See --secure-port instead. This parameter is ignored if a config file is specified in --config.")
	// MarkDeprecated hides the flag from the help. We don't want that:
	// fs.MarkDeprecated("port", "see --secure-port instead.")
}

// Validate ensures that the insecure port values within the range of the port.
func (s *DeprecatedInsecureServingOptions) Validate() []error {
	if s == nil {
		return nil
	}

	errors := []error{}

	if s.BindPort < 0 || s.BindPort > 65535 {
		errors = append(errors, fmt.Errorf("insecure port %v must be between 0 and 65535, inclusive. 0 for turning off insecure (HTTP) port", s.BindPort))
	}

	return errors
}

// AddFlags adds flags related to insecure serving to the specified FlagSet.
func (s *DeprecatedInsecureServingOptions) AddFlags(fs *pflag.FlagSet) {
	if s == nil {
		return
	}

	fs.IPVar(&s.BindAddress, "insecure-bind-address", s.BindAddress, ""+
		"The IP address on which to serve the --insecure-port (set to 0.0.0.0 or :: for listening in all interfaces and IP families).")
	// Though this flag is deprecated, we discovered security concerns over how to do health checks without it e.g. #43784
	fs.MarkDeprecated("insecure-bind-address", "This flag will be removed in a future version.")
	fs.Lookup("insecure-bind-address").Hidden = false

	fs.IntVar(&s.BindPort, "insecure-port", s.BindPort, ""+
		"The port on which to serve unsecured, unauthenticated access.")
	// Though this flag is deprecated, we discovered security concerns over how to do health checks without it e.g. #43784
	fs.MarkDeprecated("insecure-port", "This flag will be removed in a future version.")
	fs.Lookup("insecure-port").Hidden = false
}

// AddUnqualifiedFlags adds flags related to insecure serving without the --insecure prefix to the specified FlagSet.
func (s *DeprecatedInsecureServingOptions) AddUnqualifiedFlags(fs *pflag.FlagSet) {
	if s == nil {
		return
	}

	fs.IPVar(&s.BindAddress, "address", s.BindAddress,
		"The IP address on which to serve the insecure --port (set to '0.0.0.0' or '::' for listening in all interfaces and IP families).")
	fs.MarkDeprecated("address", "see --bind-address instead.")
	fs.Lookup("address").Hidden = false

	fs.IntVar(&s.BindPort, "port", s.BindPort, "The port on which to serve unsecured, unauthenticated access. Set to 0 to disable.")
	fs.MarkDeprecated("port", "see --secure-port instead.")
	fs.Lookup("port").Hidden = false
}

// DeprecatedInsecureServingOptionsWithLoopback adds loopback functionality to the DeprecatedInsecureServingOptions.
// DEPRECATED: all insecure serving options will be removed in a future version, however note that
// there are security concerns over how health checks can work here - see e.g. #43784
type DeprecatedInsecureServingOptionsWithLoopback struct {
	*DeprecatedInsecureServingOptions
}

// WithLoopback adds loopback functionality to the serving options.
func (o *DeprecatedInsecureServingOptions) WithLoopback() *DeprecatedInsecureServingOptionsWithLoopback {
	return &DeprecatedInsecureServingOptionsWithLoopback{o}
}
