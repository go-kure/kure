package k8s

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateConfigMap returns a basic ConfigMap object with common metadata preset.
func CreateConfigMap(name, namespace string) *corev1.ConfigMap {
	obj := &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ConfigMap",
			APIVersion: corev1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Data:       map[string]string{},
		BinaryData: map[string][]byte{},
	}
	return obj
}

// AddConfigMapData inserts a single key/value pair into the ConfigMap's Data field.
func AddConfigMapData(cm *corev1.ConfigMap, key, value string) {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = value
}

// AddConfigMapDataMap merges all entries from the provided map into the ConfigMap's Data field.
func AddConfigMapDataMap(cm *corev1.ConfigMap, data map[string]string) {
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	for k, v := range data {
		cm.Data[k] = v
	}
}

// AddConfigMapBinaryData inserts a single binary entry into the ConfigMap.
func AddConfigMapBinaryData(cm *corev1.ConfigMap, key string, value []byte) {
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	cm.BinaryData[key] = value
}

// AddConfigMapBinaryDataMap merges all binary entries into the ConfigMap's BinaryData field.
func AddConfigMapBinaryDataMap(cm *corev1.ConfigMap, data map[string][]byte) {
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	for k, v := range data {
		cm.BinaryData[k] = v
	}
}

// SetConfigMapData replaces the ConfigMap's Data map entirely.
func SetConfigMapData(cm *corev1.ConfigMap, data map[string]string) {
	cm.Data = data
}

// SetConfigMapBinaryData replaces the ConfigMap's BinaryData map entirely.
func SetConfigMapBinaryData(cm *corev1.ConfigMap, data map[string][]byte) {
	cm.BinaryData = data
}

// SetConfigMapImmutable sets the immutable field for the ConfigMap.
func SetConfigMapImmutable(cm *corev1.ConfigMap, immutable bool) {
	cm.Immutable = &immutable
}
