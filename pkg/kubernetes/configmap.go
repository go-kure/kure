package kubernetes

import (
	corev1 "k8s.io/api/core/v1"
)

// AddConfigMapData inserts a single key/value pair into the ConfigMap's Data field.
func AddConfigMapData(cm *corev1.ConfigMap, key, value string) {
	if cm == nil {
		panic("AddConfigMapData: cm must not be nil")
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = value
}

// AddConfigMapBinaryData inserts a single binary entry into the ConfigMap.
func AddConfigMapBinaryData(cm *corev1.ConfigMap, key string, value []byte) {
	if cm == nil {
		panic("AddConfigMapBinaryData: cm must not be nil")
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	cm.BinaryData[key] = value
}

// SetConfigMapImmutable sets the immutable field for the ConfigMap.
func SetConfigMapImmutable(cm *corev1.ConfigMap, immutable bool) {
	if cm == nil {
		panic("SetConfigMapImmutable: cm must not be nil")
	}
	cm.Immutable = &immutable
}

// AddConfigMapLabel adds a label to the ConfigMap.
func AddConfigMapLabel(cm *corev1.ConfigMap, key, value string) {
	if cm == nil {
		panic("AddConfigMapLabel: cm must not be nil")
	}
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[key] = value
}

// AddConfigMapAnnotation adds an annotation to the ConfigMap.
func AddConfigMapAnnotation(cm *corev1.ConfigMap, key, value string) {
	if cm == nil {
		panic("AddConfigMapAnnotation: cm must not be nil")
	}
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[key] = value
}

// SetConfigMapLabels replaces all labels on the ConfigMap.
func SetConfigMapLabels(cm *corev1.ConfigMap, labels map[string]string) {
	if cm == nil {
		panic("SetConfigMapLabels: cm must not be nil")
	}
	cm.Labels = labels
}

// SetConfigMapAnnotations replaces all annotations on the ConfigMap.
func SetConfigMapAnnotations(cm *corev1.ConfigMap, anns map[string]string) {
	if cm == nil {
		panic("SetConfigMapAnnotations: cm must not be nil")
	}
	cm.Annotations = anns
}
