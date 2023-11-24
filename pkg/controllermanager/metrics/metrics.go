// This file was copied from k8s.io/kubernetes/pkg/scheduler/metrics/metrics.go and modified

package metrics

import (
	"sync"
	"time"

	"k8s.io/component-base/metrics"
	"k8s.io/component-base/metrics/legacyregistry"
)

const (
	// GaiaControllerSubsystem - subsystem name used by gaia-controller-manager
	GaiaControllerSubsystem = "gaia_controller_manager"
	metrixStart             = 0.001
)

// All the histogram based metrics have 1ms as size for the smallest bucket.
var (
	registerAttempts = metrics.NewCounterVec(
		&metrics.CounterOpts{
			Subsystem: GaiaControllerSubsystem,
			Name:      "register_attempts_total",
			Help: "Number of attempts to schedule subscriptions, by the result." +
				" 'unregistered' means a subscription could not be scheduled, while 'error'" +
				" means an internal gaia-controller-manager problem.",
			StabilityLevel: metrics.ALPHA,
		}, []string{"result"})

	registeringLatency = metrics.NewHistogramVec(
		&metrics.HistogramOpts{
			Subsystem:      GaiaControllerSubsystem,
			Name:           "registering_attempt_duration_seconds",
			Help:           "Registering attempt latency in seconds (registering + approve)",
			Buckets:        metrics.ExponentialBuckets(metrixStart, 2, 15),
			StabilityLevel: metrics.STABLE,
		}, []string{"result"})

	RegisteringAlgorithmLatency = metrics.NewHistogram(
		&metrics.HistogramOpts{
			Subsystem:      GaiaControllerSubsystem,
			Name:           "registering_algorithm_duration_seconds",
			Help:           "Registering algorithm latency in seconds",
			Buckets:        metrics.ExponentialBuckets(metrixStart, 2, 15),
			StabilityLevel: metrics.ALPHA,
		},
	)

	metricsList = []metrics.Registerable{
		registerAttempts,
		registeringLatency,
		RegisteringAlgorithmLatency,
	}
)

var registerMetrics sync.Once

// Register all metrics.
func Register() {
	// Register the metrics.
	registerMetrics.Do(func() {
		RegisterMetrics(metricsList...)
	})
}

// RegisterMetrics registers a list of metrics.
// This function is exported because it is intended to be used by out-of-tree plugins to register their custom metrics.
func RegisterMetrics(extraMetrics ...metrics.Registerable) {
	for _, metric := range extraMetrics {
		legacyregistry.MustRegister(metric)
	}
}

// SinceInSeconds gets the time since the specified start in seconds.
func SinceInSeconds(start time.Time) float64 {
	return time.Since(start).Seconds()
}
