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
	"github.com/spf13/pflag"

	apiserveroptions "k8s.io/apiserver/pkg/server/options"
)

// CombinedInsecureServingOptions sets up to two insecure listeners for healthz and metrics. The flags
// override the ComponentConfig and DeprecatedInsecureServingOptions values for both.
type CombinedInsecureServingOptions struct {
	Healthz *apiserveroptions.DeprecatedInsecureServingOptionsWithLoopback
	Metrics *apiserveroptions.DeprecatedInsecureServingOptionsWithLoopback

	BindPort    int    // overrides the structs above on ApplyTo, ignored on ApplyToFromLoadedConfig
	BindAddress string // overrides the structs above on ApplyTo, ignored on ApplyToFromLoadedConfig
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
