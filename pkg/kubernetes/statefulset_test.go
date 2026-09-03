package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestStatefulSetPodTemplate covers the caller idiom the per-kind pod-template
// passthroughs were folded into: the PodSpec helpers applied to
// &sts.Spec.Template.Spec.
func TestStatefulSetPodTemplate(t *testing.T) {
	sts := CreateStatefulSet("app", "ns")
	spec := &sts.Spec.Template.Spec

	AddPodSpecContainer(spec, &corev1.Container{Name: "c"})
	AddPodSpecInitContainer(spec, &corev1.Container{Name: "init"})
	AddPodSpecVolume(spec, &corev1.Volume{Name: "vol"})
	AddPodSpecImagePullSecret(spec, &corev1.LocalObjectReference{Name: "secret"})
	AddPodSpecToleration(spec, &corev1.Toleration{Key: "k"})

	first := corev1.TopologySpreadConstraint{
		MaxSkew:           1,
		TopologyKey:       "zone",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
	}
	second := corev1.TopologySpreadConstraint{
		MaxSkew:           2,
		TopologyKey:       "hostname",
		WhenUnsatisfiable: corev1.DoNotSchedule,
		LabelSelector:     &metav1.LabelSelector{MatchLabels: map[string]string{"app": "test"}},
	}
	AddPodSpecTopologySpreadConstraints(spec, &first)
	AddPodSpecTopologySpreadConstraints(spec, &second)

	sc := &corev1.PodSecurityContext{}
	SetPodSpecSecurityContext(spec, sc)
	aff := &corev1.Affinity{}
	SetPodSpecAffinity(spec, aff)

	// ServiceAccountName and NodeSelector are plain fields — assigned directly.
	sts.Spec.Template.Spec.ServiceAccountName = "sa"
	ns := map[string]string{"role": "db"}
	sts.Spec.Template.Spec.NodeSelector = ns

	tmpl := sts.Spec.Template.Spec
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
	if tmpl.ServiceAccountName != "sa" {
		t.Errorf("service account name not set")
	}
	if !reflect.DeepEqual(tmpl.NodeSelector, ns) {
		t.Errorf("node selector not set")
	}
}

func TestStatefulSetFunctions(t *testing.T) {
	sts := CreateStatefulSet("app", "ns")
	if sts.Name != "app" || sts.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", sts.Namespace, sts.Name)
	}
	if sts.Kind != "StatefulSet" {
		t.Errorf("unexpected kind %q", sts.Kind)
	}

	pvc := corev1.PersistentVolumeClaim{ObjectMeta: metav1.ObjectMeta{Name: "data"}}
	AddStatefulSetVolumeClaimTemplate(sts, pvc)
	if len(sts.Spec.VolumeClaimTemplates) != 1 {
		t.Errorf("volume claim template not added")
	}

	SetStatefulSetReplicas(sts, 3)
	if sts.Spec.Replicas == nil || *sts.Spec.Replicas != 3 {
		t.Errorf("replicas not set")
	}

	rhl := int32(4)
	SetStatefulSetRevisionHistoryLimit(sts, &rhl)
	if sts.Spec.RevisionHistoryLimit == nil || *sts.Spec.RevisionHistoryLimit != 4 {
		t.Errorf("revision history limit not set")
	}
}

func TestStatefulSetNilGuards(t *testing.T) {
	rhl := int32(1)

	assertPanics(t, func() { AddStatefulSetVolumeClaimTemplate(nil, corev1.PersistentVolumeClaim{}) })
	assertPanics(t, func() { SetStatefulSetReplicas(nil, 1) })
	assertPanics(t, func() { SetStatefulSetRevisionHistoryLimit(nil, &rhl) })
}
