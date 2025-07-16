package k8s

import (
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

func AddJobContainer(job *batchv1.Job, container *corev1.Container) {
	job.Spec.Template.Spec.Containers = append(job.Spec.Template.Spec.Containers, *container)
}

func AddJobInitContainer(job *batchv1.Job, container *corev1.Container) {
	job.Spec.Template.Spec.InitContainers = append(job.Spec.Template.Spec.InitContainers, *container)
}

func AddJobVolume(job *batchv1.Job, volume *corev1.Volume) {
	job.Spec.Template.Spec.Volumes = append(job.Spec.Template.Spec.Volumes, *volume)
}

func AddJobImagePullSecret(job *batchv1.Job, secret *corev1.LocalObjectReference) {
	job.Spec.Template.Spec.ImagePullSecrets = append(job.Spec.Template.Spec.ImagePullSecrets, *secret)
}

func AddJobToleration(job *batchv1.Job, toleration *corev1.Toleration) {
	job.Spec.Template.Spec.Tolerations = append(job.Spec.Template.Spec.Tolerations, *toleration)
}

func AddJobTopologySpreadConstraint(job *batchv1.Job, constraint *corev1.TopologySpreadConstraint) {
	if constraint == nil {
		return
	}
	job.Spec.Template.Spec.TopologySpreadConstraints = append(job.Spec.Template.Spec.TopologySpreadConstraints, *constraint)
}

func SetJobServiceAccountName(job *batchv1.Job, name string) {
	job.Spec.Template.Spec.ServiceAccountName = name
}

func SetJobSecurityContext(job *batchv1.Job, sc *corev1.PodSecurityContext) {
	job.Spec.Template.Spec.SecurityContext = sc
}

func SetJobAffinity(job *batchv1.Job, aff *corev1.Affinity) {
	job.Spec.Template.Spec.Affinity = aff
}

func SetJobNodeSelector(job *batchv1.Job, selector map[string]string) {
	job.Spec.Template.Spec.NodeSelector = selector
}

func SetJobCompletions(job *batchv1.Job, completions int32) {
	job.Spec.Completions = &completions
}

func SetJobParallelism(job *batchv1.Job, parallelism int32) {
	job.Spec.Parallelism = &parallelism
}

func SetJobBackoffLimit(job *batchv1.Job, limit int32) {
	job.Spec.BackoffLimit = &limit
}

func SetJobTTLSecondsAfterFinished(job *batchv1.Job, ttl int32) {
	job.Spec.TTLSecondsAfterFinished = &ttl
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

func AddCronJobContainer(cron *batchv1.CronJob, container *corev1.Container) {
	cron.Spec.JobTemplate.Spec.Template.Spec.Containers = append(cron.Spec.JobTemplate.Spec.Template.Spec.Containers, *container)
}

func AddCronJobInitContainer(cron *batchv1.CronJob, container *corev1.Container) {
	cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers = append(cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers, *container)
}

func AddCronJobVolume(cron *batchv1.CronJob, volume *corev1.Volume) {
	cron.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(cron.Spec.JobTemplate.Spec.Template.Spec.Volumes, *volume)
}

func AddCronJobImagePullSecret(cron *batchv1.CronJob, secret *corev1.LocalObjectReference) {
	cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = append(cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets, *secret)
}

func AddCronJobToleration(cron *batchv1.CronJob, toleration *corev1.Toleration) {
	cron.Spec.JobTemplate.Spec.Template.Spec.Tolerations = append(cron.Spec.JobTemplate.Spec.Template.Spec.Tolerations, *toleration)
}

func AddCronJobTopologySpreadConstraint(cron *batchv1.CronJob, constraint *corev1.TopologySpreadConstraint) {
	if constraint == nil {
		return
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints = append(cron.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints, *constraint)
}

func SetCronJobServiceAccountName(cron *batchv1.CronJob, name string) {
	cron.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName = name
}

func SetCronJobSecurityContext(cron *batchv1.CronJob, sc *corev1.PodSecurityContext) {
	cron.Spec.JobTemplate.Spec.Template.Spec.SecurityContext = sc
}

func SetCronJobAffinity(cron *batchv1.CronJob, aff *corev1.Affinity) {
	cron.Spec.JobTemplate.Spec.Template.Spec.Affinity = aff
}

func SetCronJobNodeSelector(cron *batchv1.CronJob, selector map[string]string) {
	cron.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = selector
}

func SetCronJobSchedule(cron *batchv1.CronJob, schedule string) {
	cron.Spec.Schedule = schedule
}

func SetCronJobConcurrencyPolicy(cron *batchv1.CronJob, policy batchv1.ConcurrencyPolicy) {
	cron.Spec.ConcurrencyPolicy = policy
}

func SetCronJobSuspend(cron *batchv1.CronJob, suspend bool) {
	cron.Spec.Suspend = &suspend
}

func SetCronJobSuccessfulJobsHistoryLimit(cron *batchv1.CronJob, limit int32) {
	cron.Spec.SuccessfulJobsHistoryLimit = &limit
}

func SetCronJobFailedJobsHistoryLimit(cron *batchv1.CronJob, limit int32) {
	cron.Spec.FailedJobsHistoryLimit = &limit
}

func SetCronJobStartingDeadlineSeconds(cron *batchv1.CronJob, sec int64) {
	cron.Spec.StartingDeadlineSeconds = &sec
}
