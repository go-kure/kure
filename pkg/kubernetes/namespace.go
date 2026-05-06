package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNamespace creates a new Namespace with TypeMeta, a default "app" label
// and annotation, and an empty Spec.Finalizers slice.
func CreateNamespace(name string) *corev1.Namespace {
	return &corev1.Namespace{
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

// SetNamespacePSALabels sets Pod Security Admission labels on the namespace.
// enforce, warn, audit are PSA levels: use PSALevel constants (PSARestricted,
// PSABaseline, PSAPrivileged) or an empty string to skip that mode.
// version is applied to all configured modes; pass "latest", a specific
// Kubernetes version like "v1.28", or an empty string to omit version labels.
func SetNamespacePSALabels(ns *corev1.Namespace, enforce, warn, audit PSALevel, version string) {
	if enforce != "" {
		AddNamespaceLabel(ns, "pod-security.kubernetes.io/enforce", string(enforce))
		if version != "" {
			AddNamespaceLabel(ns, "pod-security.kubernetes.io/enforce-version", version)
		}
	}
	if warn != "" {
		AddNamespaceLabel(ns, "pod-security.kubernetes.io/warn", string(warn))
		if version != "" {
			AddNamespaceLabel(ns, "pod-security.kubernetes.io/warn-version", version)
		}
	}
	if audit != "" {
		AddNamespaceLabel(ns, "pod-security.kubernetes.io/audit", string(audit))
		if version != "" {
			AddNamespaceLabel(ns, "pod-security.kubernetes.io/audit-version", version)
		}
	}
}
