package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func CreateNamespace(name string) *corev1.Namespace {
	obj := &corev1.Namespace{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Namespace",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.NamespaceSpec{
			Finalizers: []corev1.FinalizerName{},
		},
	}
	return obj
}

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

// SetNamespaceLabels replaces all labels on the Namespace.
func SetNamespaceLabels(ns *corev1.Namespace, labels map[string]string) {
	ns.Labels = labels
}

// SetNamespaceAnnotations replaces all annotations on the Namespace.
func SetNamespaceAnnotations(ns *corev1.Namespace, annotations map[string]string) {
	ns.Annotations = annotations
}

// SetNamespaceFinalizers replaces all finalizers on the Namespace spec.
func SetNamespaceFinalizers(ns *corev1.Namespace, finalizers []corev1.FinalizerName) {
	ns.Spec.Finalizers = finalizers
}
