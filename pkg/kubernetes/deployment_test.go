package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeploymentNilErrors(t *testing.T) {
	assertPanics(t, func() { SetDeploymentReplicas(nil, 3) })
	assertPanics(t, func() { SetDeploymentRevisionHistoryLimit(nil, 5) })
	assertPanics(t, func() { SetDeploymentProgressDeadlineSeconds(nil, 60) })
}

// TestDeploymentPodTemplate covers the caller idiom the per-kind pod-template
// passthroughs were folded into: the PodSpec helpers applied to
// &dep.Spec.Template.Spec.
func TestDeploymentPodTemplate(t *testing.T) {
	dep := CreateDeployment("test", "default")
	spec := &dep.Spec.Template.Spec

	AddPodSpecContainer(spec, &corev1.Container{Name: "c"})
	AddPodSpecInitContainer(spec, &corev1.Container{Name: "init"})
	AddPodSpecVolume(spec, &corev1.Volume{Name: "vol"})
	AddPodSpecImagePullSecret(spec, &corev1.LocalObjectReference{Name: "secret"})
	AddPodSpecToleration(spec, &corev1.Toleration{Key: "k"})

	c := corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "topology.kubernetes.io/zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector: &metav1.LabelSelector{
			MatchLabels: map[string]string{"app": "test"},
		},
	}
	AddPodSpecTopologySpreadConstraints(spec, &c)

	sc := &corev1.PodSecurityContext{}
	SetPodSpecSecurityContext(spec, sc)
	aff := &corev1.Affinity{}
	SetPodSpecAffinity(spec, aff)

	tmpl := dep.Spec.Template.Spec
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
	if len(tmpl.TopologySpreadConstraints) != 1 {
		t.Fatalf("expected 1 constraint, got %d", len(tmpl.TopologySpreadConstraints))
	}
	if !reflect.DeepEqual(tmpl.TopologySpreadConstraints[0], c) {
		t.Errorf("constraint mismatch: got %+v, want %+v", tmpl.TopologySpreadConstraints[0], c)
	}
	if tmpl.SecurityContext != sc {
		t.Errorf("security context not set on the pod template")
	}
	if tmpl.Affinity != aff {
		t.Errorf("affinity not set on the pod template")
	}
}

func TestDeploymentFunctions(t *testing.T) {
	dep := CreateDeployment("app", "ns")
	if dep.Name != "app" || dep.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", dep.Namespace, dep.Name)
	}
	if dep.Kind != "Deployment" {
		t.Errorf("unexpected kind %q", dep.Kind)
	}

	SetDeploymentReplicas(dep, 3)
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 3 {
		t.Errorf("replicas not set")
	}

	SetDeploymentRevisionHistoryLimit(dep, 5)
	if dep.Spec.RevisionHistoryLimit == nil || *dep.Spec.RevisionHistoryLimit != 5 {
		t.Errorf("revision history limit not set")
	}

	SetDeploymentProgressDeadlineSeconds(dep, 60)
	if dep.Spec.ProgressDeadlineSeconds == nil || *dep.Spec.ProgressDeadlineSeconds != 60 {
		t.Errorf("progress deadline seconds not set")
	}
}

// TestDeploymentPodSpecAssignment covers the replacement for the removed
// SetDeploymentPodSpec: a whole PodSpec goes on by assignment.
func TestDeploymentPodSpecAssignment(t *testing.T) {
	dep := CreateDeployment("test", "default")
	dep.Spec.Template.Spec = corev1.PodSpec{
		Containers: []corev1.Container{
			{Name: "test", Image: "nginx"},
		},
	}
	if len(dep.Spec.Template.Spec.Containers) != 1 {
		t.Fatal("expected PodSpec to be assigned")
	}
}
