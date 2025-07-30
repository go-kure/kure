package k8s

import (
	"errors"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateJob(name, namespace string) *batchv1.Job {
	obj := &batchv1.Job{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Job",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: corev1.PodSpec{},
			},
		},
	}
	return obj
}

// SetJobPodSpec assigns a PodSpec to the Job template.
func SetJobPodSpec(job *batchv1.Job, spec *corev1.PodSpec) error {
	if job == nil || spec == nil {
		return errors.New("nil job or spec")
	}
	job.Spec.Template.Spec = *spec
	return nil
}

func AddJobContainer(job *batchv1.Job, container *corev1.Container) error {
	if job == nil {
		return errors.New("nil job")
	}
	return AddPodSpecContainer(&job.Spec.Template.Spec, container)
}

func AddJobInitContainer(job *batchv1.Job, container *corev1.Container) error {
	if job == nil {
		return errors.New("nil job")
	}
	return AddPodSpecInitContainer(&job.Spec.Template.Spec, container)
}

func AddJobVolume(job *batchv1.Job, volume *corev1.Volume) error {
	if job == nil {
		return errors.New("nil job")
	}
	return AddPodSpecVolume(&job.Spec.Template.Spec, volume)
}

func AddJobImagePullSecret(job *batchv1.Job, secret *corev1.LocalObjectReference) error {
	if job == nil {
		return errors.New("nil job")
	}
	return AddPodSpecImagePullSecret(&job.Spec.Template.Spec, secret)
}

func AddJobToleration(job *batchv1.Job, toleration *corev1.Toleration) error {
	if job == nil {
		return errors.New("nil job")
	}
	return AddPodSpecToleration(&job.Spec.Template.Spec, toleration)
}

func AddJobTopologySpreadConstraint(job *batchv1.Job, constraint *corev1.TopologySpreadConstraint) error {
	if job == nil {
		return errors.New("nil job")
	}
	return AddPodSpecTopologySpreadConstraints(&job.Spec.Template.Spec, constraint)
}

func SetJobServiceAccountName(job *batchv1.Job, name string) error {
	if job == nil {
		return errors.New("nil job")
	}
	return SetPodSpecServiceAccountName(&job.Spec.Template.Spec, name)
}

func SetJobSecurityContext(job *batchv1.Job, sc *corev1.PodSecurityContext) error {
	if job == nil {
		return errors.New("nil job")
	}
	return SetPodSpecSecurityContext(&job.Spec.Template.Spec, sc)
}

func SetJobAffinity(job *batchv1.Job, aff *corev1.Affinity) error {
	if job == nil {
		return errors.New("nil job")
	}
	return SetPodSpecAffinity(&job.Spec.Template.Spec, aff)
}

func SetJobNodeSelector(job *batchv1.Job, selector map[string]string) error {
	if job == nil {
		return errors.New("nil job")
	}
	return SetPodSpecNodeSelector(&job.Spec.Template.Spec, selector)
}

func SetJobCompletions(job *batchv1.Job, completions int32) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.Completions = &completions
	return nil
}

func SetJobParallelism(job *batchv1.Job, parallelism int32) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.Parallelism = &parallelism
	return nil
}

func SetJobBackoffLimit(job *batchv1.Job, limit int32) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.BackoffLimit = &limit
	return nil
}

func SetJobTTLSecondsAfterFinished(job *batchv1.Job, ttl int32) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.TTLSecondsAfterFinished = &ttl
	return nil
}

// SetJobActiveDeadlineSeconds sets the active deadline seconds for the job.
func SetJobActiveDeadlineSeconds(job *batchv1.Job, secs *int64) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.ActiveDeadlineSeconds = secs
	return nil
}

func CreateCronJob(name, namespace, schedule string) *batchv1.CronJob {
	obj := &batchv1.CronJob{
		TypeMeta: metav1.TypeMeta{
			Kind:       "CronJob",
			APIVersion: batchv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: batchv1.CronJobSpec{
			Schedule: schedule,
			JobTemplate: batchv1.JobTemplateSpec{
				Spec: batchv1.JobSpec{
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": name,
							},
						},
						Spec: corev1.PodSpec{},
					},
				},
			},
		},
	}
	return obj
}

// SetCronJobPodSpec assigns a PodSpec to the CronJob template.
func SetCronJobPodSpec(cron *batchv1.CronJob, spec *corev1.PodSpec) error {
	if cron == nil || spec == nil {
		return errors.New("nil cronjob or spec")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec = *spec
	return nil
}

func AddCronJobContainer(cron *batchv1.CronJob, container *corev1.Container) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return AddPodSpecContainer(&cron.Spec.JobTemplate.Spec.Template.Spec, container)
}

func AddCronJobInitContainer(cron *batchv1.CronJob, container *corev1.Container) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return AddPodSpecInitContainer(&cron.Spec.JobTemplate.Spec.Template.Spec, container)
}

func AddCronJobVolume(cron *batchv1.CronJob, volume *corev1.Volume) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return AddPodSpecVolume(&cron.Spec.JobTemplate.Spec.Template.Spec, volume)
}

func AddCronJobImagePullSecret(cron *batchv1.CronJob, secret *corev1.LocalObjectReference) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return AddPodSpecImagePullSecret(&cron.Spec.JobTemplate.Spec.Template.Spec, secret)
}

func AddCronJobToleration(cron *batchv1.CronJob, toleration *corev1.Toleration) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return AddPodSpecToleration(&cron.Spec.JobTemplate.Spec.Template.Spec, toleration)
}

func AddCronJobTopologySpreadConstraint(cron *batchv1.CronJob, constraint *corev1.TopologySpreadConstraint) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return AddPodSpecTopologySpreadConstraints(&cron.Spec.JobTemplate.Spec.Template.Spec, constraint)
}

func SetCronJobServiceAccountName(cron *batchv1.CronJob, name string) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return SetPodSpecServiceAccountName(&cron.Spec.JobTemplate.Spec.Template.Spec, name)
}

func SetCronJobSecurityContext(cron *batchv1.CronJob, sc *corev1.PodSecurityContext) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return SetPodSpecSecurityContext(&cron.Spec.JobTemplate.Spec.Template.Spec, sc)
}

func SetCronJobAffinity(cron *batchv1.CronJob, aff *corev1.Affinity) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return SetPodSpecAffinity(&cron.Spec.JobTemplate.Spec.Template.Spec, aff)
}

func SetCronJobNodeSelector(cron *batchv1.CronJob, selector map[string]string) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	return SetPodSpecNodeSelector(&cron.Spec.JobTemplate.Spec.Template.Spec, selector)
}

func SetCronJobSchedule(cron *batchv1.CronJob, schedule string) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.Schedule = schedule
	return nil
}

func SetCronJobConcurrencyPolicy(cron *batchv1.CronJob, policy batchv1.ConcurrencyPolicy) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.ConcurrencyPolicy = policy
	return nil
}

func SetCronJobSuspend(cron *batchv1.CronJob, suspend bool) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.Suspend = &suspend
	return nil
}

func SetCronJobSuccessfulJobsHistoryLimit(cron *batchv1.CronJob, limit int32) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.SuccessfulJobsHistoryLimit = &limit
	return nil
}

func SetCronJobFailedJobsHistoryLimit(cron *batchv1.CronJob, limit int32) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.FailedJobsHistoryLimit = &limit
	return nil
}

func SetCronJobStartingDeadlineSeconds(cron *batchv1.CronJob, sec int64) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.StartingDeadlineSeconds = &sec
	return nil
}

// SetCronJobTimeZone sets the time zone field.
func SetCronJobTimeZone(cron *batchv1.CronJob, tz *string) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.TimeZone = tz
	return nil
}
