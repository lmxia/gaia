// This file was copied from k8s.io/kubernetes/pkg/scheduler/metrics/profile_metrics.go and modified
// This file contains helpers for metrics that are associated to a profile.

package metrics

var (
	registeredResult     = "registered"
	unregisteredResult = "unregistered"
	errorResult         = "error"
)

// ClusterRegisteredApproved can record a successful scheduling attempt and the duration
// since `start`.
func ClusterRegisteredApproved(duration float64) {
	observeScheduleAttemptAndLatency(registeredResult, duration)
}

func observeScheduleAttemptAndLatency(result string, duration float64) {
	registeringLatency.WithLabelValues(result).Observe(duration)
	registerAttempts.WithLabelValues(result).Inc()
}
