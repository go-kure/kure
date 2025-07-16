package k8s

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestCreateNamespace(t *testing.T) {
	ns := CreateNamespace("demo")
	if ns.Name != "demo" {
		t.Errorf("expected name demo got %s", ns.Name)
	}
	if ns.Kind != "Namespace" {
		t.Errorf("unexpected kind %q", ns.Kind)
	}
	if len(ns.Spec.Finalizers) != 0 {
		t.Errorf("expected no finalizers")
	}
	if ns.Labels["app"] != "demo" {
		t.Errorf("default label not set")
	}
	if ns.Annotations["app"] != "demo" {
		t.Errorf("default annotation not set")
	}
}

func TestNamespaceLabelFunctions(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceLabel(ns, "env", "prod")
	if ns.Labels["env"] != "prod" {
		t.Errorf("label not added")
	}
	newLabels := map[string]string{"a": "b"}
	SetNamespaceLabels(ns, newLabels)
	if !reflect.DeepEqual(ns.Labels, newLabels) {
		t.Errorf("labels not set correctly")
	}
}

func TestNamespaceAnnotationFunctions(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceAnnotation(ns, "team", "dev")
	if ns.Annotations["team"] != "dev" {
		t.Errorf("annotation not added")
	}
	newAnn := map[string]string{"x": "y"}
	SetNamespaceAnnotations(ns, newAnn)
	if !reflect.DeepEqual(ns.Annotations, newAnn) {
		t.Errorf("annotations not set correctly")
	}
}

func TestNamespaceFinalizerFunctions(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceFinalizer(ns, corev1.FinalizerKubernetes)
	if len(ns.Spec.Finalizers) != 1 || ns.Spec.Finalizers[0] != corev1.FinalizerKubernetes {
		t.Errorf("finalizer not added")
	}
	finals := []corev1.FinalizerName{"custom"}
	SetNamespaceFinalizers(ns, finals)
	if !reflect.DeepEqual(ns.Spec.Finalizers, finals) {
		t.Errorf("finalizers not set correctly")
	}
}
