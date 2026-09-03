package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// The four helpers below are the metadata sugar the builder contract admits
// over metav1.Object (contract §5). They replace the per-kind
// Add<Kind>Label / Set<Kind>Labels / Add<Kind>Annotation scatter: one set of
// names for every registered kind, including kinds kure never names. A nil
// obj panics, like every other sugar helper.

// SetLabels replaces obj's labels with labels.
func SetLabels(obj metav1.Object, labels map[string]string) {
	obj.SetLabels(labels)
}

// AddLabel sets one label on obj, initialising the map when it is nil.
func AddLabel(obj metav1.Object, key, value string) {
	labels := obj.GetLabels()
	if labels == nil {
		labels = map[string]string{}
	}
	labels[key] = value
	obj.SetLabels(labels)
}

// SetAnnotations replaces obj's annotations with annotations.
func SetAnnotations(obj metav1.Object, annotations map[string]string) {
	obj.SetAnnotations(annotations)
}

// AddAnnotation sets one annotation on obj, initialising the map when it is nil.
func AddAnnotation(obj metav1.Object, key, value string) {
	annotations := obj.GetAnnotations()
	if annotations == nil {
		annotations = map[string]string{}
	}
	annotations[key] = value
	obj.SetAnnotations(annotations)
}
