package kubernetes

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestAddNamespaceFinalizer(t *testing.T) {
	ns := CreateNamespace("demo")
	AddNamespaceFinalizer(ns, corev1.FinalizerKubernetes)
	if len(ns.Spec.Finalizers) != 1 || ns.Spec.Finalizers[0] != corev1.FinalizerKubernetes {
		t.Errorf("expected finalizer %q, got %v", corev1.FinalizerKubernetes, ns.Spec.Finalizers)
	}
}

// TestNamespaceMetadataViaGenericHelpers covers what AddNamespaceLabel and
// AddNamespaceAnnotation used to do: the generic metadata helpers reach every
// kind, including a Namespace with nil maps.
func TestNamespaceMetadataViaGenericHelpers(t *testing.T) {
	ns := &corev1.Namespace{}
	AddLabel(ns, "env", "prod")
	AddAnnotation(ns, "team", "dev")

	if ns.Labels["env"] != "prod" {
		t.Errorf("expected label env=prod, got %q", ns.Labels["env"])
	}
	if ns.Annotations["team"] != "dev" {
		t.Errorf("expected annotation team=dev, got %q", ns.Annotations["team"])
	}
}
