package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestJobPodTemplate covers the caller idiom the per-kind pod-template
// passthroughs were folded into: the PodSpec helpers applied to
// &job.Spec.Template.Spec.
func TestJobPodTemplate(t *testing.T) {
	job := CreateJob("job", "ns")
	spec := &job.Spec.Template.Spec

	AddPodSpecContainer(spec, &corev1.Container{Name: "c"})
	AddPodSpecInitContainer(spec, &corev1.Container{Name: "init"})
	AddPodSpecVolume(spec, &corev1.Volume{Name: "vol"})
	AddPodSpecImagePullSecret(spec, &corev1.LocalObjectReference{Name: "pull"})
	AddPodSpecToleration(spec, &corev1.Toleration{Key: "k"})

	tsc := corev1.TopologySpreadConstraint{MaxSkew: 1, TopologyKey: "zone", WhenUnsatisfiable: corev1.DoNotSchedule, LabelSelector: &metav1.LabelSelector{}}
	AddPodSpecTopologySpreadConstraints(spec, &tsc)

	sc := &corev1.PodSecurityContext{}
	SetPodSpecSecurityContext(spec, sc)
	aff := &corev1.Affinity{}
	SetPodSpecAffinity(spec, aff)

	// ServiceAccountName and NodeSelector are plain fields — assigned directly.
	job.Spec.Template.Spec.ServiceAccountName = "sa"
	sel := map[string]string{"role": "db"}
	job.Spec.Template.Spec.NodeSelector = sel

	tmpl := job.Spec.Template.Spec
	if len(tmpl.Containers) != 1 || tmpl.Containers[0].Name != "c" {
		t.Errorf("container not added to the pod template")
	}
	if len(tmpl.InitContainers) != 1 {
		t.Errorf("init container not added to the pod template")
	}
	if len(tmpl.Volumes) != 1 {
		t.Errorf("volume not added to the pod template")
	}
	if len(tmpl.ImagePullSecrets) != 1 {
		t.Errorf("pull secret not added to the pod template")
	}
	if len(tmpl.Tolerations) != 1 {
		t.Errorf("toleration not added to the pod template")
	}
	if len(tmpl.TopologySpreadConstraints) != 1 {
		t.Errorf("topology constraint not added to the pod template")
	}
	if tmpl.SecurityContext != sc {
		t.Errorf("security context not set on the pod template")
	}
	if tmpl.Affinity != aff {
		t.Errorf("affinity not set on the pod template")
	}
	if tmpl.ServiceAccountName != "sa" {
		t.Errorf("service account not set")
	}
	if !reflect.DeepEqual(tmpl.NodeSelector, sel) {
		t.Errorf("node selector not set")
	}
}

func TestJobFunctions(t *testing.T) {
	job := CreateJob("job", "ns")

	SetJobCompletions(job, 2)
	if job.Spec.Completions == nil || *job.Spec.Completions != 2 {
		t.Errorf("completions not set")
	}

	SetJobParallelism(job, 3)
	if job.Spec.Parallelism == nil || *job.Spec.Parallelism != 3 {
		t.Errorf("parallelism not set")
	}

	SetJobBackoffLimit(job, 4)
	if job.Spec.BackoffLimit == nil || *job.Spec.BackoffLimit != 4 {
		t.Errorf("backoff limit not set")
	}

	SetJobTTLSecondsAfterFinished(job, 30)
	if job.Spec.TTLSecondsAfterFinished == nil || *job.Spec.TTLSecondsAfterFinished != 30 {
		t.Errorf("ttl not set")
	}

	ad := int64(100)
	SetJobActiveDeadlineSeconds(job, &ad)
	if job.Spec.ActiveDeadlineSeconds == nil || *job.Spec.ActiveDeadlineSeconds != 100 {
		t.Errorf("active deadline not set")
	}
}

func TestJobNilGuards(t *testing.T) {
	ad := int64(1)

	assertPanics(t, func() { SetJobCompletions(nil, 1) })
	assertPanics(t, func() { SetJobParallelism(nil, 1) })
	assertPanics(t, func() { SetJobBackoffLimit(nil, 1) })
	assertPanics(t, func() { SetJobTTLSecondsAfterFinished(nil, 1) })
	assertPanics(t, func() { SetJobActiveDeadlineSeconds(nil, &ad) })
}
