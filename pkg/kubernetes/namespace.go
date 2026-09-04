package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// AddNamespaceLabel adds a label to the Namespace, initializing the map if needed.
func AddNamespaceLabel(ns *corev1.Namespace, key, value string) {
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[key] = value
}

// AddNamespaceAnnotation adds an annotation to the Namespace, initializing the map if needed.
func AddNamespaceAnnotation(ns *corev1.Namespace, key, value string) {
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	ns.Annotations[key] = value
}

// AddNamespaceFinalizer appends a finalizer to the Namespace spec.
func AddNamespaceFinalizer(ns *corev1.Namespace, finalizer corev1.FinalizerName) {
	ns.Spec.Finalizers = append(ns.Spec.Finalizers, finalizer)
}

// PSA labels are not set by a per-namespace helper: PSALabels (psa.go) returns
// the label map, and AddLabel applies each entry.
