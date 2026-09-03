package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCronJobNilErrors(t *testing.T) {
	assertPanics(t, func() { SetCronJobSuspend(nil, true) })
	assertPanics(t, func() { SetCronJobSuccessfulJobsHistoryLimit(nil, 3) })
	assertPanics(t, func() { SetCronJobFailedJobsHistoryLimit(nil, 1) })
	assertPanics(t, func() { SetCronJobStartingDeadlineSeconds(nil, 60) })
	tz := "UTC"
	assertPanics(t, func() { SetCronJobTimeZone(nil, &tz) })
}

// TestCronJobPodTemplate covers the caller idiom the per-kind pod-template
// passthroughs were folded into. A CronJob nests its pod template one level
// deeper than the other workload kinds.
func TestCronJobPodTemplate(t *testing.T) {
	cj := CreateCronJob("app", "ns")
	spec := &cj.Spec.JobTemplate.Spec.Template.Spec

	AddPodSpecContainer(spec, &corev1.Container{Name: "c"})
	AddPodSpecInitContainer(spec, &corev1.Container{Name: "init"})
	AddPodSpecVolume(spec, &corev1.Volume{Name: "vol"})
	AddPodSpecImagePullSecret(spec, &corev1.LocalObjectReference{Name: "secret"})
	AddPodSpecToleration(spec, &corev1.Toleration{Key: "k"})

	first := corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "topology.kubernetes.io/zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "test"},
		},
	}
	second := corev1.TopologySpreadConstraint{
		MaxSkew:           2,
		TopologyKey:       "hostname",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "test"},
		},
	}
	AddPodSpecTopologySpreadConstraints(spec, &first)
	AddPodSpecTopologySpreadConstraints(spec, &second)

	sc := &corev1.PodSecurityContext{RunAsUser: func(i int64) *int64 { return &i }(1)}
	SetPodSpecSecurityContext(spec, sc)
	aff := &corev1.Affinity{}
	SetPodSpecAffinity(spec, aff)

	tmpl := cj.Spec.JobTemplate.Spec.Template.Spec
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
		t.Errorf("image pull secret not added to the pod template")
	}
	if len(tmpl.Tolerations) != 1 {
		t.Errorf("toleration not added to the pod template")
	}
	if len(tmpl.TopologySpreadConstraints) != 2 {
		t.Fatalf("expected 2 constraints, got %d", len(tmpl.TopologySpreadConstraints))
	}
	if !reflect.DeepEqual(tmpl.TopologySpreadConstraints[0], first) {
		t.Errorf("first constraint mismatch")
	}
	if !reflect.DeepEqual(tmpl.TopologySpreadConstraints[1], second) {
		t.Errorf("second constraint mismatch")
	}
	if tmpl.SecurityContext != sc {
		t.Errorf("security context not set on the pod template")
	}
	if tmpl.Affinity != aff {
		t.Errorf("affinity not set on the pod template")
	}
}

func TestCronJobFunctions(t *testing.T) {
	cj := CreateCronJob("app", "ns")
	if cj.Name != "app" || cj.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", cj.Namespace, cj.Name)
	}
	if cj.Kind != "CronJob" {
		t.Errorf("unexpected kind %q", cj.Kind)
	}

	SetCronJobSuspend(cj, true)
	if cj.Spec.Suspend == nil || !*cj.Spec.Suspend {
		t.Errorf("suspend not set")
	}

	SetCronJobSuccessfulJobsHistoryLimit(cj, 1)
	if cj.Spec.SuccessfulJobsHistoryLimit == nil || *cj.Spec.SuccessfulJobsHistoryLimit != 1 {
		t.Errorf("successful jobs history limit not set")
	}

	SetCronJobFailedJobsHistoryLimit(cj, 2)
	if cj.Spec.FailedJobsHistoryLimit == nil || *cj.Spec.FailedJobsHistoryLimit != 2 {
		t.Errorf("failed jobs history limit not set")
	}

	SetCronJobStartingDeadlineSeconds(cj, 60)
	if cj.Spec.StartingDeadlineSeconds == nil || *cj.Spec.StartingDeadlineSeconds != 60 {
		t.Errorf("starting deadline seconds not set")
	}

	tz := "UTC"
	SetCronJobTimeZone(cj, &tz)
	if cj.Spec.TimeZone == nil || *cj.Spec.TimeZone != "UTC" {
		t.Errorf("timezone not set")
	}
}

// TestCronJobPodSpecAssignment covers the replacement for the removed
// SetCronJobPodSpec: a whole PodSpec goes on by assignment.
func TestCronJobPodSpecAssignment(t *testing.T) {
	cj := CreateCronJob("test", "default")
	cj.Spec.JobTemplate.Spec.Template.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	if len(cj.Spec.JobTemplate.Spec.Template.Spec.Containers) != 1 {
		t.Fatal("expected PodSpec to be assigned")
	}
}
