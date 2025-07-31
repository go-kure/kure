package kubernetes

import (
	"errors"

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
func AddConfigMapData(cm *corev1.ConfigMap, key, value string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	cm.Data[key] = value
	return nil
}

// AddConfigMapDataMap merges all entries from the provided map into the ConfigMap's Data field.
func AddConfigMapDataMap(cm *corev1.ConfigMap, data map[string]string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	for k, v := range data {
		cm.Data[k] = v
	}
	return nil
}

// AddConfigMapBinaryData inserts a single binary entry into the ConfigMap.
func AddConfigMapBinaryData(cm *corev1.ConfigMap, key string, value []byte) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	cm.BinaryData[key] = value
	return nil
}

// AddConfigMapBinaryDataMap merges all binary entries into the ConfigMap's BinaryData field.
func AddConfigMapBinaryDataMap(cm *corev1.ConfigMap, data map[string][]byte) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	if cm.BinaryData == nil {
		cm.BinaryData = make(map[string][]byte)
	}
	for k, v := range data {
		cm.BinaryData[k] = v
	}
	return nil
}

// SetConfigMapData replaces the ConfigMap's Data map entirely.
func SetConfigMapData(cm *corev1.ConfigMap, data map[string]string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	cm.Data = data
	return nil
}

// SetConfigMapBinaryData replaces the ConfigMap's BinaryData map entirely.
func SetConfigMapBinaryData(cm *corev1.ConfigMap, data map[string][]byte) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	cm.BinaryData = data
	return nil
}

// SetConfigMapImmutable sets the immutable field for the ConfigMap.
func SetConfigMapImmutable(cm *corev1.ConfigMap, immutable bool) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	cm.Immutable = &immutable
	return nil
}

// AddConfigMapLabel adds a label to the ConfigMap.
func AddConfigMapLabel(cm *corev1.ConfigMap, key, value string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	if cm.Labels == nil {
		cm.Labels = make(map[string]string)
	}
	cm.Labels[key] = value
	return nil
}

// AddConfigMapAnnotation adds an annotation to the ConfigMap.
func AddConfigMapAnnotation(cm *corev1.ConfigMap, key, value string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	if cm.Annotations == nil {
		cm.Annotations = make(map[string]string)
	}
	cm.Annotations[key] = value
	return nil
}

// SetConfigMapLabels replaces all labels on the ConfigMap.
func SetConfigMapLabels(cm *corev1.ConfigMap, labels map[string]string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	cm.Labels = labels
	return nil
}

// SetConfigMapAnnotations replaces all annotations on the ConfigMap.
func SetConfigMapAnnotations(cm *corev1.ConfigMap, anns map[string]string) error {
	if cm == nil {
		return errors.New("nil configmap")
	}
	cm.Annotations = anns
	return nil
}
