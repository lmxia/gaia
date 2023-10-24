package cronmaster

import (
	"fmt"
	"time"

	appsV1alpha1 "github.com/lmxia/gaia/pkg/apis/apps/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/klog/v2"
)

var nextScheduleDelta = 100 * time.Millisecond

// nextScheduledTimeDuration returns
func nextScheduledTimeDuration(t, now time.Time) *time.Duration {
	td := t.Add(nextScheduleDelta).Sub(now)
	return &td
}

// getCronNextScheduleTime gets the time of next schedule after last scheduled and before now
func getCronNextScheduleTime(cron *appsV1alpha1.CronMaster, now time.Time) (*time.Time, bool, error) {
	var earliestTime time.Time
	if cron.Status.LastScheduleTime != nil {
		earliestTime = cron.Status.LastScheduleTime.Time
	} else {
		// If none found, then this is either a recently created cronJob,
		// or the active/completed info was somehow lost (contract for status
		// in kubernetes says it may need to be recreated), or that we have
		// started a job, but have not noticed it yet (distributed systems can
		// have arbitrary delays).  In any case, use the creation time of the
		// CronJob as last known start time.
		earliestTime = cron.ObjectMeta.CreationTimestamp.Time
	}
	if earliestTime.After(now) {
		return nil, false, nil
	}

	t, isStart, numberOfMissedSchedules, err := getMostRecentScheduleTime(cron.Spec.Schedule, earliestTime, now)
	if numberOfMissedSchedules > 100 {
		klog.InfoS("too many missed times", "CronMaster", klog.KRef(cron.GetNamespace(), cron.GetName()), "missed times", numberOfMissedSchedules)
	}

	return t, isStart, err
}

// getMostRecentScheduleTime returns the latest next schedule time after earliestTime
func getMostRecentScheduleTime(config appsV1alpha1.SchedulerConfig, earliestTime, now time.Time) (*time.Time, bool, int64, error) {
	t1, isStart1 := getNextScheduledTimeAfterNow(config, earliestTime)
	t2, _ := getNextScheduledTimeAfterNow(config, t1)

	if t1.After(now) {
		return nil, isStart1, 0, nil
	}
	if t2.After(now) {
		return &t1, isStart1, 1, nil
	}

	// It is possible for cron.ParseStandard("59 23 31 2 *") to return an invalid schedule
	// seconds - 59, minute - 23, hour - 31 (?!)  dom - 2, and dow is optional, clearly 31 is invalid
	// In this case the timeBetweenTwoSchedules will be 0, and we error out the invalid schedule
	timeBetweenTwoSchedules := int64(t2.Sub(t1).Round(time.Second).Seconds())
	if timeBetweenTwoSchedules < 1 {
		return nil, isStart1, 0, fmt.Errorf("time difference between two schedules less than 1 second")
	}
	timeElapsed := int64(now.Sub(t1).Seconds())
	numberOfMissedSchedules := (timeElapsed / timeBetweenTwoSchedules) + 1
	t := time.Unix(t1.Unix()+((numberOfMissedSchedules-1)*timeBetweenTwoSchedules), 0).Local()

	return &t, isStart1, numberOfMissedSchedules, nil
}

// getNextScheduledTimeAfterNow return the next scheduled time after now
func getNextScheduledTimeAfterNow(config appsV1alpha1.SchedulerConfig, now time.Time) (time.Time, bool) {
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
		reStartTime := getRecentStartTime(now, scheduleOfWeekday)
		if reStartTime != nil {
			start = *reStartTime
		}
	}
	if config.EndEnable {
		reEndTime := getRecentEndTime(now, scheduleOfWeekday)
		if reEndTime != nil {
			end = *reEndTime
		}
	}

	// start and end all enable
	if start.After(now) && end.After(now) {
		if start.After(end) {
			return end, false
		} else {
			return start, true
		}
	}
	// only start
	if start.After(now) {
		return start, true
	}
	// only end
	if end.After(now) {
		return end, false
	}

	return now, false
}

func getRecentStartTime(now time.Time, scheduleOfWeekday map[time.Weekday]appsV1alpha1.ScheduleTimeSet) *time.Time {
	nowWeekday := now.Weekday()
	for j := 0; j < 8; j++ {
		if nowWeekday == 7 {
			nowWeekday = nowWeekday - 7
		}
		if scheduleOfWeekday[nowWeekday].StartSchedule != "" {
			tOnly, err := time.Parse(time.TimeOnly, scheduleOfWeekday[nowWeekday].StartSchedule)
			if err != nil {
				klog.WarningDepth(2, "invalid ScheduleTimeSet, 'Weekday' is ", nowWeekday, "'StartSchedule' is ", scheduleOfWeekday[nowWeekday].StartSchedule)
				utilruntime.HandleError(err)
				return nil
			}
			tSchedule := time.Date(now.Year(), now.Month(), now.Day(), tOnly.Hour(), tOnly.Minute(), tOnly.Second(), 0, now.Location())
			tSchedule = tSchedule.AddDate(0, 0, j)
			if tSchedule.After(now) {
				klog.V(5).InfoS("next start schedule", "DateTime", tSchedule, "weekday ", nowWeekday, "after the datetime", now)
				return &tSchedule
			}
		}
		nowWeekday++
	}
	klog.Info("no available StartSchedule datetime")
	return nil
}

func getRecentEndTime(now time.Time, scheduleOfWeekday map[time.Weekday]appsV1alpha1.ScheduleTimeSet) *time.Time {
	nowWeekday := now.Weekday()
	for j := 0; j < 8; j++ {
		if nowWeekday == 7 {
			nowWeekday = nowWeekday - 7
		}
		if scheduleOfWeekday[nowWeekday].EndSchedule != "" {
			tOnly, err := time.Parse(time.TimeOnly, scheduleOfWeekday[nowWeekday].EndSchedule)
			if err != nil {
				klog.WarningDepth(2, "invalid ScheduleTimeSet, 'Weekday' is ", nowWeekday, "'EndSchedule' is ", scheduleOfWeekday[nowWeekday].EndSchedule)
				utilruntime.HandleError(err)
				return nil
			}
			tSchedule := time.Date(now.Year(), now.Month(), now.Day(), tOnly.Hour(), tOnly.Minute(), tOnly.Second(), 0, now.Location())
			tSchedule = tSchedule.AddDate(0, 0, j)
			if tSchedule.After(now) {
				klog.V(5).InfoS("next stop schedule", "DateTime", tSchedule, "weekday", nowWeekday, "after the datetime", now)
				return &tSchedule
			}
		}
		nowWeekday++
	}
	klog.Info("no available EndSchedule datetime")
	return nil
}

func inActiveList(cron appsV1alpha1.CronMaster, uid types.UID) bool {
	for _, d := range cron.Status.Active {
		if d.UID == uid {
			return true
		}
	}
	return false
}

func getFinishedStatus(d *appsv1.Deployment) (bool, appsv1.DeploymentConditionType) {
	for _, c := range d.Status.Conditions {
		if (c.Type == appsv1.DeploymentAvailable) && c.Status == corev1.ConditionTrue {
			return true, c.Type
		}
	}
	return false, ""
}

func deleteFromActiveList(cron *appsV1alpha1.CronMaster, uid types.UID) {
	if cron == nil {
		return
	}
	// TODO: @alpatel the memory footprint can may be reduced here by
	//  cj.Status.Active = append(cj.Status.Active[:indexToRemove], cj.Status.Active[indexToRemove:]...)
	newActive := []corev1.ObjectReference{}
	for _, d := range cron.Status.Active {
		if d.UID != uid {
			newActive = append(newActive, d)
		}
	}
	cron.Status.Active = newActive
}
