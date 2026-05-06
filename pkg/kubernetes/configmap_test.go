package kubernetes_test

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"

	kubernetes "github.com/go-kure/kure/pkg/kubernetes"
)

func TestCreateConfigMap(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	if cm == nil {
		t.Fatal("expected non-nil ConfigMap")
	}
	if cm.Name != "cm" || cm.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", cm.Namespace, cm.Name)
	}
	if cm.Kind != "ConfigMap" {
		t.Errorf("unexpected kind %q", cm.Kind)
	}
	if cm.APIVersion != corev1.SchemeGroupVersion.String() {
		t.Errorf("unexpected apiVersion %q", cm.APIVersion)
	}
	if cm.Labels["app"] != "cm" {
		t.Errorf("expected default label app=cm, got %q", cm.Labels["app"])
	}
	if cm.Annotations["app"] != "cm" {
		t.Errorf("expected default annotation app=cm, got %q", cm.Annotations["app"])
	}
	if len(cm.Data) != 0 {
		t.Errorf("expected empty data map")
	}
	if len(cm.BinaryData) != 0 {
		t.Errorf("expected empty binary data map")
	}
}

func TestAddConfigMapData(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	kubernetes.AddConfigMapData(cm, "key", "val")
	if cm.Data["key"] != "val" {
		t.Errorf("AddConfigMapData: expected 'val', got %q", cm.Data["key"])
	}
}

func TestAddConfigMapDataMap(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	more := map[string]string{"a": "b", "c": "d"}
	kubernetes.AddConfigMapDataMap(cm, more)
	for k, v := range more {
		if cm.Data[k] != v {
			t.Errorf("AddConfigMapDataMap: missing key %s", k)
		}
	}
}

func TestSetConfigMapData(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapData(cm, "old", "value")

	newData := map[string]string{"x": "y"}
	kubernetes.SetConfigMapData(cm, newData)
	if !reflect.DeepEqual(cm.Data, newData) {
		t.Errorf("SetConfigMapData: got %+v", cm.Data)
	}
}

func TestAddConfigMapBinaryData(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	kubernetes.AddConfigMapBinaryData(cm, "bin", []byte{1, 2})
	if !reflect.DeepEqual(cm.BinaryData["bin"], []byte{1, 2}) {
		t.Errorf("AddConfigMapBinaryData: unexpected value")
	}
}

func TestAddConfigMapBinaryDataMap(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	more := map[string][]byte{"b1": {2, 3}, "b2": {4}}
	kubernetes.AddConfigMapBinaryDataMap(cm, more)
	for k, v := range more {
		if !reflect.DeepEqual(cm.BinaryData[k], v) {
			t.Errorf("AddConfigMapBinaryDataMap: missing key %s", k)
		}
	}
}

func TestSetConfigMapBinaryData(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapBinaryData(cm, "old", []byte{0})

	newData := map[string][]byte{"x": {9}}
	kubernetes.SetConfigMapBinaryData(cm, newData)
	if !reflect.DeepEqual(cm.BinaryData, newData) {
		t.Errorf("SetConfigMapBinaryData: got %+v", cm.BinaryData)
	}
}

func TestSetConfigMapImmutable(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	kubernetes.SetConfigMapImmutable(cm, true)
	if cm.Immutable == nil || !*cm.Immutable {
		t.Errorf("SetConfigMapImmutable: expected true")
	}

	kubernetes.SetConfigMapImmutable(cm, false)
	if cm.Immutable == nil || *cm.Immutable {
		t.Errorf("SetConfigMapImmutable: expected false")
	}
}

func TestAddConfigMapLabel(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapLabel(cm, "env", "prod")
	if cm.Labels["env"] != "prod" {
		t.Errorf("AddConfigMapLabel: label not set")
	}
}

func TestAddConfigMapAnnotation(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapAnnotation(cm, "owner", "team")
	if cm.Annotations["owner"] != "team" {
		t.Errorf("AddConfigMapAnnotation: annotation not set")
	}
}

func TestSetConfigMapLabels(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	labels := map[string]string{"x": "y"}
	kubernetes.SetConfigMapLabels(cm, labels)
	if !reflect.DeepEqual(cm.Labels, labels) {
		t.Errorf("SetConfigMapLabels: got %+v", cm.Labels)
	}
}

func TestSetConfigMapAnnotations(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	anns := map[string]string{"c": "d"}
	kubernetes.SetConfigMapAnnotations(cm, anns)
	if !reflect.DeepEqual(cm.Annotations, anns) {
		t.Errorf("SetConfigMapAnnotations: got %+v", cm.Annotations)
	}
}
