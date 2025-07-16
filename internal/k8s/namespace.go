package k8s

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

func AddNamespaceLabel(ns *corev1.Namespace, key, value string) {
	if ns.Labels == nil {
		ns.Labels = make(map[string]string)
	}
	ns.Labels[key] = value
}

func AddNamespaceAnnotation(ns *corev1.Namespace, key, value string) {
	if ns.Annotations == nil {
		ns.Annotations = make(map[string]string)
	}
	ns.Annotations[key] = value
}

func AddNamespaceFinalizer(ns *corev1.Namespace, finalizer corev1.FinalizerName) {
	ns.Spec.Finalizers = append(ns.Spec.Finalizers, finalizer)
}

func SetNamespaceLabels(ns *corev1.Namespace, labels map[string]string) {
	ns.Labels = labels
}

func SetNamespaceAnnotations(ns *corev1.Namespace, annotations map[string]string) {
	ns.Annotations = annotations
}

func SetNamespaceFinalizers(ns *corev1.Namespace, finalizers []corev1.FinalizerName) {
	ns.Spec.Finalizers = finalizers
}
