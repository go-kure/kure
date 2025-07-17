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
				Spec: corev1.PodSpec{
					Containers:                    []corev1.Container{},
					InitContainers:                []corev1.Container{},
					Volumes:                       []corev1.Volume{},
					RestartPolicy:                 corev1.RestartPolicyNever,
					TerminationGracePeriodSeconds: new(int64),
					SecurityContext:               &corev1.PodSecurityContext{},
					ImagePullSecrets:              []corev1.LocalObjectReference{},
					ServiceAccountName:            "",
					NodeSelector:                  map[string]string{},
					Affinity:                      &corev1.Affinity{},
					Tolerations:                   []corev1.Toleration{},
				},
			},
		},
	}
	return obj
}

func AddJobContainer(job *batchv1.Job, container *corev1.Container) error {
	if job == nil || container == nil {
		return errors.New("nil job or container")
	}
	job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, *container)
	return nil
}

func AddJobInitContainer(job *batchv1.Job, container *corev1.Container) error {
	if job == nil || container == nil {
		return errors.New("nil job or container")
	}
	job.Spec.Template.Spec.InitContainers = append(job.Spec.Template.Spec.InitContainers, *container)
	return nil
}

func AddJobVolume(job *batchv1.Job, volume *corev1.Volume) error {
	if job == nil || volume == nil {
		return errors.New("nil job or volume")
	}
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, *volume)
	return nil
}

func AddJobImagePullSecret(job *batchv1.Job, secret *corev1.LocalObjectReference) error {
	if job == nil || secret == nil {
		return errors.New("nil job or secret")
	}
	job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, *secret)
	return nil
}

func AddJobToleration(job *batchv1.Job, toleration *corev1.Toleration) error {
	if job == nil || toleration == nil {
		return errors.New("nil job or toleration")
	}
	job.Spec.Template.Spec.Tolerations = append(job.Spec.Template.Spec.Tolerations, *toleration)
	return nil
}

func AddJobTopologySpreadConstraint(job *batchv1.Job, constraint *corev1.TopologySpreadConstraint) error {
	if job == nil {
		return errors.New("nil job")
	}
	if constraint == nil {
		return nil
	}
	job.Spec.Template.Spec.TopologySpreadConstraints = append(job.Spec.Template.Spec.TopologySpreadConstraints, *constraint)
	return nil
}

func SetJobServiceAccountName(job *batchv1.Job, name string) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.Template.Spec.ServiceAccountName = name
	return nil
}

func SetJobSecurityContext(job *batchv1.Job, sc *corev1.PodSecurityContext) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.Template.Spec.SecurityContext = sc
	return nil
}

func SetJobAffinity(job *batchv1.Job, aff *corev1.Affinity) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.Template.Spec.Affinity = aff
	return nil
}

func SetJobNodeSelector(job *batchv1.Job, selector map[string]string) error {
	if job == nil {
		return errors.New("nil job")
	}
	job.Spec.Template.Spec.NodeSelector = selector
	return nil
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
						Spec: corev1.PodSpec{
							Containers:                    []corev1.Container{},
							InitContainers:                []corev1.Container{},
							Volumes:                       []corev1.Volume{},
							RestartPolicy:                 corev1.RestartPolicyNever,
							TerminationGracePeriodSeconds: new(int64),
							SecurityContext:               &corev1.PodSecurityContext{},
							ImagePullSecrets:              []corev1.LocalObjectReference{},
							ServiceAccountName:            "",
							NodeSelector:                  map[string]string{},
							Affinity:                      &corev1.Affinity{},
							Tolerations:                   []corev1.Toleration{},
						},
					},
				},
			},
		},
	}
	return obj
}

func AddCronJobContainer(cron *batchv1.CronJob, container *corev1.Container) error {
	if cron == nil || container == nil {
		return errors.New("nil cronjob or container")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Containers = append(cron.Spec.JobTemplate.Spec.Template.Spec.Containers, *container)
	return nil
}

func AddCronJobInitContainer(cron *batchv1.CronJob, container *corev1.Container) error {
	if cron == nil || container == nil {
		return errors.New("nil cronjob or container")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers = append(cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers, *container)
	return nil
}

func AddCronJobVolume(cron *batchv1.CronJob, volume *corev1.Volume) error {
	if cron == nil || volume == nil {
		return errors.New("nil cronjob or volume")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(cron.Spec.JobTemplate.Spec.Template.Spec.Volumes, *volume)
	return nil
}

func AddCronJobImagePullSecret(cron *batchv1.CronJob, secret *corev1.LocalObjectReference) error {
	if cron == nil || secret == nil {
		return errors.New("nil cronjob or secret")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = append(cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets, *secret)
	return nil
}

func AddCronJobToleration(cron *batchv1.CronJob, toleration *corev1.Toleration) error {
	if cron == nil || toleration == nil {
		return errors.New("nil cronjob or toleration")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Tolerations = append(cron.Spec.JobTemplate.Spec.Template.Spec.Tolerations, *toleration)
	return nil
}

func AddCronJobTopologySpreadConstraint(cron *batchv1.CronJob, constraint *corev1.TopologySpreadConstraint) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	if constraint == nil {
		return nil
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints = append(cron.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints, *constraint)
	return nil
}

func SetCronJobServiceAccountName(cron *batchv1.CronJob, name string) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName = name
	return nil
}

func SetCronJobSecurityContext(cron *batchv1.CronJob, sc *corev1.PodSecurityContext) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.SecurityContext = sc
	return nil
}

func SetCronJobAffinity(cron *batchv1.CronJob, aff *corev1.Affinity) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Affinity = aff
	return nil
}

func SetCronJobNodeSelector(cron *batchv1.CronJob, selector map[string]string) error {
	if cron == nil {
		return errors.New("nil cronjob")
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = selector
	return nil
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
