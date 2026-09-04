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

// Labels and annotations use the generic AddLabel / AddAnnotation / SetLabels /
// SetAnnotations over metav1.Object (metadata.go); there is no per-kind
// ConfigMap helper.
