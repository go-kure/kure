package kubernetes

import (
	batchv1 "k8s.io/api/batch/v1"
)

// SetCronJobSuspend sets whether the CronJob is suspended.
func SetCronJobSuspend(cron *batchv1.CronJob, suspend bool) {
	if cron == nil {
		panic("SetCronJobSuspend: cron must not be nil")
	}
	cron.Spec.Suspend = &suspend
}

// SetCronJobSuccessfulJobsHistoryLimit sets the number of successful finished
// jobs to retain.
func SetCronJobSuccessfulJobsHistoryLimit(cron *batchv1.CronJob, limit int32) {
	if cron == nil {
		panic("SetCronJobSuccessfulJobsHistoryLimit: cron must not be nil")
	}
	cron.Spec.SuccessfulJobsHistoryLimit = &limit
}

// SetCronJobFailedJobsHistoryLimit sets the number of failed finished jobs to
// retain.
func SetCronJobFailedJobsHistoryLimit(cron *batchv1.CronJob, limit int32) {
	if cron == nil {
		panic("SetCronJobFailedJobsHistoryLimit: cron must not be nil")
	}
	cron.Spec.FailedJobsHistoryLimit = &limit
}

// SetCronJobStartingDeadlineSeconds sets the optional deadline in seconds for
// starting the job if it misses its scheduled time.
func SetCronJobStartingDeadlineSeconds(cron *batchv1.CronJob, sec int64) {
	if cron == nil {
		panic("SetCronJobStartingDeadlineSeconds: cron must not be nil")
	}
	cron.Spec.StartingDeadlineSeconds = &sec
}

// SetCronJobTimeZone sets the time zone for the CronJob schedule.
func SetCronJobTimeZone(cron *batchv1.CronJob, tz *string) {
	if cron == nil {
		panic("SetCronJobTimeZone: cron must not be nil")
	}
	cron.Spec.TimeZone = tz
}
