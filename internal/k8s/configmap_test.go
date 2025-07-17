package k8s

import (
	"reflect"
	"testing"
)

func TestCreateConfigMap(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")
	if cm.Name != "cm" || cm.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", cm.Namespace, cm.Name)
	}
	if cm.Kind != "ConfigMap" {
		t.Errorf("unexpected kind %q", cm.Kind)
	}
	if len(cm.Data) != 0 {
		t.Errorf("expected empty data map")
	}
	if len(cm.BinaryData) != 0 {
		t.Errorf("expected empty binary data map")
	}
}

func TestConfigMapDataFunctions(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")

	if err := AddConfigMapData(cm, "k", "v"); err != nil {
		t.Fatalf("AddConfigMapData returned error: %v", err)
	}
	if val, ok := cm.Data["k"]; !ok || val != "v" {
		t.Errorf("data not added: %+v", cm.Data)
	}

	more := map[string]string{"a": "b", "c": "d"}
	if err := AddConfigMapDataMap(cm, more); err != nil {
		t.Fatalf("AddConfigMapDataMap returned error: %v", err)
	}
	for k, v := range more {
		if cm.Data[k] != v {
			t.Errorf("data map merge failed for key %s", k)
		}
	}

	newData := map[string]string{"x": "y"}
	if err := SetConfigMapData(cm, newData); err != nil {
		t.Fatalf("SetConfigMapData returned error: %v", err)
	}
	if !reflect.DeepEqual(cm.Data, newData) {
		t.Errorf("set data failed: %+v", cm.Data)
	}
}

func TestConfigMapBinaryDataFunctions(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")

	if err := AddConfigMapBinaryData(cm, "bin", []byte{1}); err != nil {
		t.Fatalf("AddConfigMapBinaryData returned error: %v", err)
	}
	if val, ok := cm.BinaryData["bin"]; !ok || !reflect.DeepEqual(val, []byte{1}) {
		t.Errorf("binary data not added: %+v", cm.BinaryData)
	}

	more := map[string][]byte{"b1": {2, 3}, "b2": {4}}
	if err := AddConfigMapBinaryDataMap(cm, more); err != nil {
		t.Fatalf("AddConfigMapBinaryDataMap returned error: %v", err)
	}
	for k, v := range more {
		if !reflect.DeepEqual(cm.BinaryData[k], v) {
			t.Errorf("binary data map merge failed for key %s", k)
		}
	}

	newData := map[string][]byte{"x": {9}}
	if err := SetConfigMapBinaryData(cm, newData); err != nil {
		t.Fatalf("SetConfigMapBinaryData returned error: %v", err)
	}
	if !reflect.DeepEqual(cm.BinaryData, newData) {
		t.Errorf("set binary data failed: %+v", cm.BinaryData)
	}
}

func TestSetConfigMapImmutable(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")
	if err := SetConfigMapImmutable(cm, true); err != nil {
		t.Fatalf("SetConfigMapImmutable returned error: %v", err)
	}
	if cm.Immutable == nil || !*cm.Immutable {
		t.Errorf("immutable not set to true")
	}
	if err := SetConfigMapImmutable(cm, false); err != nil {
		t.Fatalf("SetConfigMapImmutable returned error: %v", err)
	}
	if cm.Immutable == nil || *cm.Immutable {
		t.Errorf("immutable not updated to false")
	}
}

func TestConfigMapMetadataFunctions(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")

	if err := AddConfigMapLabel(cm, "k", "v"); err != nil {
		t.Fatalf("AddConfigMapLabel returned error: %v", err)
	}
	if cm.Labels["k"] != "v" {
		t.Errorf("label not added")
	}

	if err := AddConfigMapAnnotation(cm, "a", "b"); err != nil {
		t.Fatalf("AddConfigMapAnnotation returned error: %v", err)
	}
	if cm.Annotations["a"] != "b" {
		t.Errorf("annotation not added")
	}

	if err := SetConfigMapLabels(cm, map[string]string{"x": "y"}); err != nil {
		t.Fatalf("SetConfigMapLabels returned error: %v", err)
	}
	if !reflect.DeepEqual(cm.Labels, map[string]string{"x": "y"}) {
		t.Errorf("labels not set")
	}

	if err := SetConfigMapAnnotations(cm, map[string]string{"c": "d"}); err != nil {
		t.Fatalf("SetConfigMapAnnotations returned error: %v", err)
	}
	if !reflect.DeepEqual(cm.Annotations, map[string]string{"c": "d"}) {
		t.Errorf("annotations not set")
	}
}
