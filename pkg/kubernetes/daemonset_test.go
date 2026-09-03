package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestDaemonSetPodTemplate covers the caller idiom the per-kind pod-template
// passthroughs were folded into: the PodSpec helpers applied to
// &ds.Spec.Template.Spec.
func TestDaemonSetPodTemplate(t *testing.T) {
	ds := CreateDaemonSet("app", "ns")
	spec := &ds.Spec.Template.Spec

	AddPodSpecContainer(spec, &corev1.Container{Name: "c"})
	AddPodSpecInitContainer(spec, &corev1.Container{Name: "init"})
	AddPodSpecVolume(spec, &corev1.Volume{Name: "vol"})
	AddPodSpecImagePullSecret(spec, &corev1.LocalObjectReference{Name: "secret"})
	AddPodSpecToleration(spec, &corev1.Toleration{Key: "k"})

	c := corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
	}
	AddPodSpecTopologySpreadConstraints(spec, &c)

	sc := &corev1.PodSecurityContext{}
	SetPodSpecSecurityContext(spec, sc)
	aff := &corev1.Affinity{}
	SetPodSpecAffinity(spec, aff)

	// ServiceAccountName and NodeSelector are plain fields — assigned directly.
	ds.Spec.Template.Spec.ServiceAccountName = "sa"
	ns := map[string]string{"role": "db"}
	ds.Spec.Template.Spec.NodeSelector = ns

	tmpl := ds.Spec.Template.Spec
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
	if tmpl.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}
	if !reflect.DeepEqual(tmpl.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}
}

func TestDaemonSetFunctions(t *testing.T) {
	ds := CreateDaemonSet("app", "ns")
	if ds.Name != "app" || ds.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", ds.Namespace, ds.Name)
	}
	if ds.Kind != "DaemonSet" {
		t.Errorf("unexpected kind %q", ds.Kind)
	}

	rhl := int32(3)
	SetDaemonSetRevisionHistoryLimit(ds, &rhl)
	if ds.Spec.RevisionHistoryLimit == nil || *ds.Spec.RevisionHistoryLimit != 3 {
		t.Errorf("revision history limit not set")
	}
}

func TestDaemonSetNilGuards(t *testing.T) {
	rhl := int32(1)

	assertPanics(t, func() { SetDaemonSetRevisionHistoryLimit(nil, &rhl) })
}
