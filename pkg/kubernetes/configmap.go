// Package kubernetes exposes ConfigMap builders that delegate to internal/kubernetes,
// keeping the implementation in one place while providing the public API.
package kubernetes

import (
	corev1 "k8s.io/api/core/v1"

	intk8s "github.com/go-kure/kure/internal/kubernetes"
)

// CreateConfigMap returns a basic ConfigMap object with common metadata preset.
func CreateConfigMap(name, namespace string) *corev1.ConfigMap {
	return intk8s.CreateConfigMap(name, namespace)
}

// AddConfigMapData inserts a single key/value pair into the ConfigMap's Data field.
func AddConfigMapData(cm *corev1.ConfigMap, key, value string) {
	intk8s.AddConfigMapData(cm, key, value)
}

// AddConfigMapDataMap merges all entries from the provided map into the ConfigMap's Data field.
func AddConfigMapDataMap(cm *corev1.ConfigMap, data map[string]string) {
	intk8s.AddConfigMapDataMap(cm, data)
}

// AddConfigMapBinaryData inserts a single binary entry into the ConfigMap.
func AddConfigMapBinaryData(cm *corev1.ConfigMap, key string, value []byte) {
	intk8s.AddConfigMapBinaryData(cm, key, value)
}

// AddConfigMapBinaryDataMap merges all binary entries into the ConfigMap's BinaryData field.
func AddConfigMapBinaryDataMap(cm *corev1.ConfigMap, data map[string][]byte) {
	intk8s.AddConfigMapBinaryDataMap(cm, data)
}

// SetConfigMapData replaces the ConfigMap's Data map entirely.
func SetConfigMapData(cm *corev1.ConfigMap, data map[string]string) {
	intk8s.SetConfigMapData(cm, data)
}

// SetConfigMapBinaryData replaces the ConfigMap's BinaryData map entirely.
func SetConfigMapBinaryData(cm *corev1.ConfigMap, data map[string][]byte) {
	intk8s.SetConfigMapBinaryData(cm, data)
}

// SetConfigMapImmutable sets the immutable field for the ConfigMap.
func SetConfigMapImmutable(cm *corev1.ConfigMap, immutable bool) {
	intk8s.SetConfigMapImmutable(cm, immutable)
}

// AddConfigMapLabel adds a label to the ConfigMap.
func AddConfigMapLabel(cm *corev1.ConfigMap, key, value string) {
	intk8s.AddConfigMapLabel(cm, key, value)
}

// AddConfigMapAnnotation adds an annotation to the ConfigMap.
func AddConfigMapAnnotation(cm *corev1.ConfigMap, key, value string) {
	intk8s.AddConfigMapAnnotation(cm, key, value)
}

// SetConfigMapLabels replaces all labels on the ConfigMap.
func SetConfigMapLabels(cm *corev1.ConfigMap, labels map[string]string) {
	intk8s.SetConfigMapLabels(cm, labels)
}

// SetConfigMapAnnotations replaces all annotations on the ConfigMap.
func SetConfigMapAnnotations(cm *corev1.ConfigMap, anns map[string]string) {
	intk8s.SetConfigMapAnnotations(cm, anns)
}
