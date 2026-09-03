package kubernetes

import (
	batchv1 "k8s.io/api/batch/v1"
)

func SetJobCompletions(job *batchv1.Job, completions int32) {
	if job == nil {
		panic("SetJobCompletions: job must not be nil")
	}
	job.Spec.Completions = &completions
}

func SetJobParallelism(job *batchv1.Job, parallelism int32) {
	if job == nil {
		panic("SetJobParallelism: job must not be nil")
	}
	job.Spec.Parallelism = &parallelism
}

func SetJobBackoffLimit(job *batchv1.Job, limit int32) {
	if job == nil {
		panic("SetJobBackoffLimit: job must not be nil")
	}
	job.Spec.BackoffLimit = &limit
}

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
