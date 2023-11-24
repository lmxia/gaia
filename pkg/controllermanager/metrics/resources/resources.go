/*
Copyright 2020 The Kubernetes Authors.

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

// Package resources provides a metrics collector that reports the
// resource consumption (requests and limits) of the pods in the cluster
// as the scheduler and kubelet would interpret it.
package resources

import (
	corelisters "k8s.io/client-go/listers/core/v1"
	"k8s.io/component-base/metrics"
)

type resourceLifecycleDescriptors struct {
	total *metrics.Desc
}

func (d resourceLifecycleDescriptors) Describe(ch chan<- *metrics.Desc) {
	ch <- d.total
}

type resourceMetricsDescriptors struct {
	requests resourceLifecycleDescriptors
	limits   resourceLifecycleDescriptors
}

func (d resourceMetricsDescriptors) Describe(ch chan<- *metrics.Desc) {
	d.requests.Describe(ch)
	d.limits.Describe(ch)
}

var podResourceDesc = resourceMetricsDescriptors{
	requests: resourceLifecycleDescriptors{
		total: metrics.NewDesc("kube_pod_resource_request",
			"Resources requested by workloads on the cluster, broken down by pod. "+
				"This shows the resource usage the scheduler and kubelet expect per pod "+
				"for resources along with the unit for the resource if any.",
			[]string{"namespace", "pod", "node", "scheduler", "priority", "resource", "unit"},
			nil,
			metrics.ALPHA,
			""),
	},
	limits: resourceLifecycleDescriptors{
		total: metrics.NewDesc("kube_pod_resource_limit",
			"Resources limit for workloads on the cluster, broken down by pod. "+
				"This shows the resource usage the scheduler and kubelet expect per pod for resources"+
				" along with the unit for the resource if any.",
			[]string{"namespace", "pod", "node", "scheduler", "priority", "resource", "unit"},
			nil,
			metrics.ALPHA,
			""),
	},
}

// Check if resourceMetricsCollector implements necessary interface
var _ metrics.StableCollector = &podResourceCollector{}

// NewPodResourcesMetricsCollector registers a O(pods) cardinality metric that
// reports the current resources requested by all pods on the cluster within
// the Kubernetes resource model. Metrics are broken down by pod, node, resource,
// and phase of lifecycle. Each pod returns two series per resource - one for
// their aggregate usage (required to schedule) and one for their phase specific
// usage. This allows admins to assess the cost per resource at different phases
// of startup and compare to actual resource usage.
func NewPodResourcesMetricsCollector(podLister corelisters.PodLister) metrics.StableCollector {
	return &podResourceCollector{
		lister: podLister,
	}
}

type podResourceCollector struct {
	metrics.BaseStableCollector
	lister corelisters.PodLister
}

func (c *podResourceCollector) DescribeWithStability(ch chan<- *metrics.Desc) {
	podResourceDesc.Describe(ch)
}
