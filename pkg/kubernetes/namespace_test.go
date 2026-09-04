package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

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
