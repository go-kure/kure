package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCreateNamespace(t *testing.T) {
	ns := CreateNamespace("demo")
	if ns == nil {
		t.Fatal("expected non-nil Namespace")
	}
	if ns.Kind != "Namespace" {
		t.Errorf("expected Kind=Namespace, got %q", ns.Kind)
	}
	if ns.APIVersion != "v1" {
		t.Errorf("expected APIVersion=v1, got %q", ns.APIVersion)
	}
	if ns.Name != "demo" {
		t.Errorf("expected Name=demo, got %q", ns.Name)
	}
	if ns.Labels["app"] != "demo" {
		t.Errorf("expected default label app=demo, got %q", ns.Labels["app"])
	}
	if ns.Annotations["app"] != "demo" {
		t.Errorf("expected default annotation app=demo, got %q", ns.Annotations["app"])
	}
	if len(ns.Spec.Finalizers) != 0 {
		t.Errorf("expected empty Spec.Finalizers, got %v", ns.Spec.Finalizers)
	}
}

func TestAddNamespaceLabel(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceLabel(ns, "env", "prod")
	if ns.Labels["env"] != "prod" {
		t.Errorf("expected label env=prod, got %q", ns.Labels["env"])
	}
}

func TestAddNamespaceLabel_NilMap(t *testing.T) {
	ns := &corev1.Namespace{}
	AddNamespaceLabel(ns, "env", "prod")
	if ns.Labels["env"] != "prod" {
		t.Errorf("expected label added on nil map, got %q", ns.Labels["env"])
	}
}

func TestAddNamespaceAnnotation(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceAnnotation(ns, "team", "dev")
	if ns.Annotations["team"] != "dev" {
		t.Errorf("expected annotation team=dev, got %q", ns.Annotations["team"])
	}
}

func TestAddNamespaceAnnotation_NilMap(t *testing.T) {
	ns := &corev1.Namespace{}
	AddNamespaceAnnotation(ns, "team", "dev")
	if ns.Annotations["team"] != "dev" {
		t.Errorf("expected annotation added on nil map, got %q", ns.Annotations["team"])
	}
}

func TestAddNamespaceFinalizer(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceFinalizer(ns, corev1.FinalizerKubernetes)
	if len(ns.Spec.Finalizers) != 1 || ns.Spec.Finalizers[0] != corev1.FinalizerKubernetes {
		t.Errorf("expected finalizer %q, got %v", corev1.FinalizerKubernetes, ns.Spec.Finalizers)
	}
}

func TestSetNamespaceLabels(t *testing.T) {
	ns := CreateNamespace("demo")
	newLabels := map[string]string{"a": "b"}
	SetNamespaceLabels(ns, newLabels)
	if !reflect.DeepEqual(ns.Labels, newLabels) {
		t.Errorf("expected labels %v, got %v", newLabels, ns.Labels)
	}
}

func TestSetNamespaceAnnotations(t *testing.T) {
	ns := CreateNamespace("demo")
	newAnn := map[string]string{"x": "y"}
	SetNamespaceAnnotations(ns, newAnn)
	if !reflect.DeepEqual(ns.Annotations, newAnn) {
		t.Errorf("expected annotations %v, got %v", newAnn, ns.Annotations)
	}
}

func TestSetNamespaceFinalizers(t *testing.T) {
	ns := CreateNamespace("demo")
	finals := []corev1.FinalizerName{"custom"}
	SetNamespaceFinalizers(ns, finals)
	if !reflect.DeepEqual(ns.Spec.Finalizers, finals) {
		t.Errorf("expected finalizers %v, got %v", finals, ns.Spec.Finalizers)
	}
}

func TestSetNamespacePSALabels_AllModes(t *testing.T) {
	ns := CreateNamespace("demo")
	SetNamespacePSALabels(ns, PSARestricted, PSARestricted, PSARestricted, "v1.28")

	want := map[string]string{
		"pod-security.kubernetes.io/enforce":         "restricted",
		"pod-security.kubernetes.io/enforce-version": "v1.28",
		"pod-security.kubernetes.io/warn":            "restricted",
		"pod-security.kubernetes.io/warn-version":    "v1.28",
		"pod-security.kubernetes.io/audit":           "restricted",
		"pod-security.kubernetes.io/audit-version":   "v1.28",
	}
	for k, v := range want {
		if ns.Labels[k] != v {
			t.Errorf("expected label %s=%s, got %q", k, v, ns.Labels[k])
		}
	}
}

func TestSetNamespacePSALabels_NoVersion(t *testing.T) {
	ns := CreateNamespace("demo")
	SetNamespacePSALabels(ns, PSARestricted, PSABaseline, PSAPrivileged, "")

	if ns.Labels["pod-security.kubernetes.io/enforce"] != "restricted" {
		t.Errorf("expected enforce=restricted")
	}
	if ns.Labels["pod-security.kubernetes.io/warn"] != "baseline" {
		t.Errorf("expected warn=baseline")
	}
	if ns.Labels["pod-security.kubernetes.io/audit"] != "privileged" {
		t.Errorf("expected audit=privileged")
	}
	// No version labels should be set
	for _, k := range []string{
		"pod-security.kubernetes.io/enforce-version",
		"pod-security.kubernetes.io/warn-version",
		"pod-security.kubernetes.io/audit-version",
	} {
		if _, ok := ns.Labels[k]; ok {
			t.Errorf("expected no version label %s when version is empty", k)
		}
	}
}

func TestSetNamespacePSALabels_SkipEmpty(t *testing.T) {
	ns := CreateNamespace("demo")
	// Only enforce is set; warn and audit are empty
	SetNamespacePSALabels(ns, PSARestricted, "", "", "latest")

	if ns.Labels["pod-security.kubernetes.io/enforce"] != "restricted" {
		t.Errorf("expected enforce=restricted")
	}
	if ns.Labels["pod-security.kubernetes.io/enforce-version"] != "latest" {
		t.Errorf("expected enforce-version=latest")
	}
	for _, k := range []string{
		"pod-security.kubernetes.io/warn",
		"pod-security.kubernetes.io/warn-version",
		"pod-security.kubernetes.io/audit",
		"pod-security.kubernetes.io/audit-version",
	} {
		if _, ok := ns.Labels[k]; ok {
			t.Errorf("expected label %s to be absent when mode is empty", k)
		}
	}
}

func TestSetNamespacePSALabels_CranePattern(t *testing.T) {
	// Mirrors the pattern used in crane's namespace.go: enforce+warn+audit all "restricted", no version
	ns := CreateNamespace("myapp")
	SetNamespacePSALabels(ns, PSARestricted, PSARestricted, PSARestricted, "")

	wantLabels := map[string]string{
		"pod-security.kubernetes.io/enforce": "restricted",
		"pod-security.kubernetes.io/audit":   "restricted",
		"pod-security.kubernetes.io/warn":    "restricted",
	}
	for k, v := range wantLabels {
		if ns.Labels[k] != v {
			t.Errorf("expected %s=%s, got %q", k, v, ns.Labels[k])
		}
	}
	// No version labels
	for _, k := range []string{
		"pod-security.kubernetes.io/enforce-version",
		"pod-security.kubernetes.io/warn-version",
		"pod-security.kubernetes.io/audit-version",
	} {
		if _, ok := ns.Labels[k]; ok {
			t.Errorf("unexpected version label %s", k)
		}
	}
}
