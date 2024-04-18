// This file was copied from k8s.io/kubernetes/pkg/scheduler/metrics/profile_metrics.go and modified
// This file contains helpers for metrics that are associated to a profile.

package metrics

var (
	scheduledResult = "scheduled"
)

// DescriptionScheduled can record a successful scheduling attempt and the duration
// since `start`.
func DescriptionScheduled(profile string, duration float64) {
	observeScheduleAttemptAndLatency(scheduledResult, profile, duration)
}

func observeScheduleAttemptAndLatency(result, profile string, duration float64) {
	e2eSchedulingLatency.WithLabelValues(result, profile).Observe(duration)
	schedulingLatency.WithLabelValues(result, profile).Observe(duration)
	scheduleAttempts.WithLabelValues(result, profile).Inc()
}
