package kubernetes

import (
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/go-kure/kure/pkg/errors"
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
	if job == nil {
		return errors.ErrNilJob
	}
	if spec == nil {
		return errors.ErrNilSpec
	}
	job.Spec.Template.Spec = *spec
	return nil
}

func AddJobContainer(job *batchv1.Job, container *corev1.Container) error {
	if job == nil {
		return errors.ErrNilJob
	}
	return AddPodSpecContainer(&job.Spec.Template.Spec, container)
}

func AddJobInitContainer(job *batchv1.Job, container *corev1.Container) error {
	if job == nil {
		return errors.ErrNilJob
	}
	return AddPodSpecInitContainer(&job.Spec.Template.Spec, container)
}

func AddJobVolume(job *batchv1.Job, volume *corev1.Volume) error {
	if job == nil {
		return errors.ErrNilJob
	}
	return AddPodSpecVolume(&job.Spec.Template.Spec, volume)
}

func AddJobImagePullSecret(job *batchv1.Job, secret *corev1.LocalObjectReference) error {
	if job == nil {
		return errors.ErrNilJob
	}
	return AddPodSpecImagePullSecret(&job.Spec.Template.Spec, secret)
}

func AddJobToleration(job *batchv1.Job, toleration *corev1.Toleration) error {
	if job == nil {
		return errors.ErrNilJob
	}
	return AddPodSpecToleration(&job.Spec.Template.Spec, toleration)
}

func AddJobTopologySpreadConstraint(job *batchv1.Job, constraint *corev1.TopologySpreadConstraint) error {
	if job == nil {
		return errors.ErrNilJob
	}
	return AddPodSpecTopologySpreadConstraints(&job.Spec.Template.Spec, constraint)
}

func SetJobServiceAccountName(job *batchv1.Job, name string) {
	if job == nil {
		panic("SetJobServiceAccountName: job must not be nil")
	}
	SetPodSpecServiceAccountName(&job.Spec.Template.Spec, name)
}

func SetJobSecurityContext(job *batchv1.Job, sc *corev1.PodSecurityContext) {
	if job == nil {
		panic("SetJobSecurityContext: job must not be nil")
	}
	SetPodSpecSecurityContext(&job.Spec.Template.Spec, sc)
}

func SetJobAffinity(job *batchv1.Job, aff *corev1.Affinity) {
	if job == nil {
		panic("SetJobAffinity: job must not be nil")
	}
	SetPodSpecAffinity(&job.Spec.Template.Spec, aff)
}

func SetJobNodeSelector(job *batchv1.Job, selector map[string]string) {
	if job == nil {
		panic("SetJobNodeSelector: job must not be nil")
	}
	SetPodSpecNodeSelector(&job.Spec.Template.Spec, selector)
}

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
