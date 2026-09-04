package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// AddNamespaceFinalizer appends a finalizer to the Namespace spec.
func AddNamespaceFinalizer(ns *corev1.Namespace, finalizer corev1.FinalizerName) {
	ns.Spec.Finalizers = append(ns.Spec.Finalizers, finalizer)
}

// Labels and annotations use the generic AddLabel / AddAnnotation over
// metav1.Object (metadata.go); there is no per-kind Namespace helper. PSA
// labels come from PSALabels (psa.go), which returns the map.
