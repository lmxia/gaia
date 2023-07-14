package cronmaster

import (
	"fmt"
	"time"

	appsV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	"k8s.io/klog/v2"
)

var (
	nextScheduleDelta = 100 * time.Millisecond
)

// getMostRecentScheduleTime returns the latest schedule time between (earliestTime or now)
func nextScheduledTimeDuration(config appsV1alpha1.SchedulerConfig, now time.Time) (*time.Duration, bool, error) {
	t, isStart, err := getNextScheduledTime(config, now)
	if err != nil {
		klog.WarningDepth(2, "failed go get nextScheduledTimeDuration, error=", err)
		return nil, false, fmt.Errorf("failed go get nextScheduledTimeDuration, cronmaster start and stop are all error")
	}
	td := t.Add(nextScheduleDelta).Sub(now)
	return &td, isStart, nil
}

func getNextScheduledTime(config appsV1alpha1.SchedulerConfig, now time.Time) (time.Time, bool, error) {
	var start, end time.Time
	scheduleOfWeekday := map[time.Weekday]appsV1alpha1.ScheduleTimeSet{
		time.Sunday:    config.Sunday,
		time.Monday:    config.Monday,
		time.Tuesday:   config.Tuesday,
		time.Wednesday: config.Wednesday,
		time.Thursday:  config.Thursday,
		time.Friday:    config.Friday,
		time.Saturday:  config.Saturday,
	}
	if config.StartEnable {
		reStartTime, err := getRecentStartTime(now, scheduleOfWeekday)
		if err == nil {
			start = reStartTime
		}
	}
	if config.EndEnable {
		reEndTime, err := getRecentEndTime(now, scheduleOfWeekday)
		if err == nil {
			end = reEndTime
		}
	}

	// start and end all enable
	if start.After(now) && end.After(now) {
		if start.After(end) {
			return end, false, nil
		} else {
			return start, true, nil
		}
	}
	// only start
	if start.After(now) {
		return start, true, nil
	}
	// only end
	if end.After(now) {
		return end, false, nil
	}
	return now, false, fmt.Errorf("failed to get next schedule time")
}

func getRecentStartTime(now time.Time, scheduleOfWeekday map[time.Weekday]appsV1alpha1.ScheduleTimeSet) (time.Time, error) {
	nowWeekday := now.Weekday()
	for j := 0; j < 7; j++ {
		if nowWeekday == 7 {
			nowWeekday = nowWeekday - 7
		}
		if scheduleOfWeekday[nowWeekday].StartSchedule != "" {
			tOnly, err := time.Parse(time.TimeOnly, scheduleOfWeekday[nowWeekday].StartSchedule)
			if err != nil {
				klog.WarningDepth(2, "invalid ScheduleTimeSet, 'Weekday' is ", nowWeekday, "'StartSchedule' is ", scheduleOfWeekday[nowWeekday].StartSchedule)
				return now, err
			}
			klog.V(5).Info("StartSchedule Time: ", tOnly)
			tSchedule := time.Date(now.Year(), now.Month(), now.Day(), tOnly.Hour(), tOnly.Minute(), tOnly.Second(), 0, now.Location())
			tSchedule = tSchedule.AddDate(0, 0, j)
			klog.V(5).Info("next start schedule DateTime: ", tSchedule)
			if tSchedule.After(now) {
				return tSchedule, nil
			}
		}
		nowWeekday++
	}
	return now, fmt.Errorf("no available StartSchedule datetime")
}

func getRecentEndTime(now time.Time, scheduleOfWeekday map[time.Weekday]appsV1alpha1.ScheduleTimeSet) (time.Time, error) {
	nowWeekday := now.Weekday()
	for j := 0; j < 7; j++ {
		if nowWeekday == 7 {
			nowWeekday = nowWeekday - 7
		}
		if scheduleOfWeekday[nowWeekday].EndSchedule != "" {
			tOnly, err := time.Parse(time.TimeOnly, scheduleOfWeekday[nowWeekday].EndSchedule)
			if err != nil {
				klog.WarningDepth(2, "invalid ScheduleTimeSet, 'Weekday' is ", nowWeekday, "'EndSchedule' is ", scheduleOfWeekday[nowWeekday].EndSchedule)
				return now, err
			}
			klog.V(5).Info("schedule time: %q", tOnly)
			tSchedule := time.Date(now.Year(), now.Month(), now.Day(), tOnly.Hour(), tOnly.Minute(), tOnly.Second(), 0, now.Location())
			tSchedule = tSchedule.AddDate(0, 0, j)
			klog.V(5).Info("next end schedule DateTime: ", tSchedule)
			if tSchedule.After(now) {
				return tSchedule, nil
			}
		}
		nowWeekday++
	}
	return now, fmt.Errorf("no available EndSchedule datetime")
}
