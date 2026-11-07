package kubernetes

import (
	batchv1 "k8s.io/api/batch/v1"
)

// SetJobCompletions sets the desired number of successfully finished pods.
func SetJobCompletions(job *batchv1.Job, completions int32) {
	if job == nil {
		panic("SetJobCompletions: job must not be nil")
	}
	job.Spec.Completions = &completions
}

// SetJobParallelism sets the maximum number of pods that run at the same time.
func SetJobParallelism(job *batchv1.Job, parallelism int32) {
	if job == nil {
		panic("SetJobParallelism: job must not be nil")
	}
	job.Spec.Parallelism = &parallelism
}

// SetJobBackoffLimit sets the number of retries before the job is marked failed.
func SetJobBackoffLimit(job *batchv1.Job, limit int32) {
	if job == nil {
		panic("SetJobBackoffLimit: job must not be nil")
	}
	job.Spec.BackoffLimit = &limit
}

// SetJobTTLSecondsAfterFinished sets how many seconds a finished job is kept
// before it is deleted.
func SetJobTTLSecondsAfterFinished(job *batchv1.Job, ttl int32) {
	if job == nil {
		panic("SetJobTTLSecondsAfterFinished: job must not be nil")
	}
	job.Spec.TTLSecondsAfterFinished = &ttl
}

// SetJobActiveDeadlineSeconds sets the active deadline seconds for the job.
func SetJobActiveDeadlineSeconds(job *batchv1.Job, secs *int64) {
	if job == nil {
		panic("SetJobActiveDeadlineSeconds: job must not be nil")
	}
	job.Spec.ActiveDeadlineSeconds = secs
}
