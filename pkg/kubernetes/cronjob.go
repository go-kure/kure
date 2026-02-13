package kubernetes

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// CreateCronJob creates a new batch/v1 CronJob with the given name, namespace,
// and schedule. The returned object has TypeMeta, labels, annotations, and a
// job template pre-populated so it can be serialized to YAML immediately.
// The pod template restart policy defaults to Never, which is required by
// Kubernetes for Job and CronJob pods.
func CreateCronJob(name, namespace, schedule string) *batchv1.CronJob {
	return &batchv1.CronJob{
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
							RestartPolicy: corev1.RestartPolicyNever,
						},
					},
				},
			},
		},
	}
}

// SetCronJobPodSpec assigns a PodSpec to the CronJob's job template.
func SetCronJobPodSpec(cron *batchv1.CronJob, spec *corev1.PodSpec) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if spec == nil {
		return errors.ErrNilPodSpec
	}
	cron.Spec.JobTemplate.Spec.Template.Spec = *spec
	return nil
}

// AddCronJobContainer appends a container to the CronJob's pod template.
func AddCronJobContainer(cron *batchv1.CronJob, container *corev1.Container) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if container == nil {
		return errors.ErrNilContainer
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Containers = append(
		cron.Spec.JobTemplate.Spec.Template.Spec.Containers, *container)
	return nil
}

// AddCronJobInitContainer appends an init container to the CronJob's pod template.
func AddCronJobInitContainer(cron *batchv1.CronJob, container *corev1.Container) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if container == nil {
		return errors.ErrNilInitContainer
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers = append(
		cron.Spec.JobTemplate.Spec.Template.Spec.InitContainers, *container)
	return nil
}

// AddCronJobVolume appends a volume to the CronJob's pod template.
func AddCronJobVolume(cron *batchv1.CronJob, volume *corev1.Volume) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if volume == nil {
		return errors.ErrNilVolume
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Volumes = append(
		cron.Spec.JobTemplate.Spec.Template.Spec.Volumes, *volume)
	return nil
}

// AddCronJobImagePullSecret appends an image pull secret to the CronJob's pod template.
func AddCronJobImagePullSecret(cron *batchv1.CronJob, imagePullSecret *corev1.LocalObjectReference) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if imagePullSecret == nil {
		return errors.ErrNilImagePullSecret
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets = append(
		cron.Spec.JobTemplate.Spec.Template.Spec.ImagePullSecrets, *imagePullSecret)
	return nil
}

// AddCronJobToleration appends a toleration to the CronJob's pod template.
func AddCronJobToleration(cron *batchv1.CronJob, toleration *corev1.Toleration) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if toleration == nil {
		return errors.ErrNilToleration
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Tolerations = append(
		cron.Spec.JobTemplate.Spec.Template.Spec.Tolerations, *toleration)
	return nil
}

// AddCronJobTopologySpreadConstraint appends a topology spread constraint to
// the CronJob's pod template. If the constraint is nil the call is a no-op.
func AddCronJobTopologySpreadConstraint(cron *batchv1.CronJob, topologySpreadConstraint *corev1.TopologySpreadConstraint) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	if topologySpreadConstraint == nil {
		return nil
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints = append(
		cron.Spec.JobTemplate.Spec.Template.Spec.TopologySpreadConstraints, *topologySpreadConstraint)
	return nil
}

// SetCronJobServiceAccountName sets the service account name on the CronJob's
// pod template.
func SetCronJobServiceAccountName(cron *batchv1.CronJob, serviceAccountName string) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.ServiceAccountName = serviceAccountName
	return nil
}

// SetCronJobSecurityContext sets the pod-level security context on the
// CronJob's pod template.
func SetCronJobSecurityContext(cron *batchv1.CronJob, securityContext *corev1.PodSecurityContext) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.SecurityContext = securityContext
	return nil
}

// SetCronJobAffinity assigns affinity rules to the CronJob's pod template.
func SetCronJobAffinity(cron *batchv1.CronJob, affinity *corev1.Affinity) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.Affinity = affinity
	return nil
}

// SetCronJobNodeSelector sets the node selector map on the CronJob's pod
// template.
func SetCronJobNodeSelector(cron *batchv1.CronJob, nodeSelector map[string]string) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.JobTemplate.Spec.Template.Spec.NodeSelector = nodeSelector
	return nil
}

// SetCronJobSchedule sets the cron schedule expression.
func SetCronJobSchedule(cron *batchv1.CronJob, schedule string) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.Schedule = schedule
	return nil
}

// SetCronJobConcurrencyPolicy sets the concurrency policy for the CronJob.
func SetCronJobConcurrencyPolicy(cron *batchv1.CronJob, policy batchv1.ConcurrencyPolicy) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.ConcurrencyPolicy = policy
	return nil
}

// SetCronJobSuspend sets whether the CronJob is suspended.
func SetCronJobSuspend(cron *batchv1.CronJob, suspend bool) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.Suspend = &suspend
	return nil
}

// SetCronJobSuccessfulJobsHistoryLimit sets the number of successful finished
// jobs to retain.
func SetCronJobSuccessfulJobsHistoryLimit(cron *batchv1.CronJob, limit int32) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.SuccessfulJobsHistoryLimit = &limit
	return nil
}

// SetCronJobFailedJobsHistoryLimit sets the number of failed finished jobs to
// retain.
func SetCronJobFailedJobsHistoryLimit(cron *batchv1.CronJob, limit int32) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.FailedJobsHistoryLimit = &limit
	return nil
}

// SetCronJobStartingDeadlineSeconds sets the optional deadline in seconds for
// starting the job if it misses its scheduled time.
func SetCronJobStartingDeadlineSeconds(cron *batchv1.CronJob, sec int64) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.StartingDeadlineSeconds = &sec
	return nil
}

// SetCronJobTimeZone sets the time zone for the CronJob schedule.
func SetCronJobTimeZone(cron *batchv1.CronJob, tz *string) error {
	if cron == nil {
		return errors.ErrNilCronJob
	}
	cron.Spec.TimeZone = tz
	return nil
}
