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

	AddConfigMapData(cm, "k", "v")
	if val, ok := cm.Data["k"]; !ok || val != "v" {
		t.Errorf("data not added: %+v", cm.Data)
	}

	more := map[string]string{"a": "b", "c": "d"}
	AddConfigMapDataMap(cm, more)
	for k, v := range more {
		if cm.Data[k] != v {
			t.Errorf("data map merge failed for key %s", k)
		}
	}

	newData := map[string]string{"x": "y"}
	SetConfigMapData(cm, newData)
	if !reflect.DeepEqual(cm.Data, newData) {
		t.Errorf("set data failed: %+v", cm.Data)
	}
}

func TestConfigMapBinaryDataFunctions(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")

	AddConfigMapBinaryData(cm, "bin", []byte{1})
	if val, ok := cm.BinaryData["bin"]; !ok || !reflect.DeepEqual(val, []byte{1}) {
		t.Errorf("binary data not added: %+v", cm.BinaryData)
	}

	more := map[string][]byte{"b1": {2, 3}, "b2": {4}}
	AddConfigMapBinaryDataMap(cm, more)
	for k, v := range more {
		if !reflect.DeepEqual(cm.BinaryData[k], v) {
			t.Errorf("binary data map merge failed for key %s", k)
		}
	}

	newData := map[string][]byte{"x": {9}}
	SetConfigMapBinaryData(cm, newData)
	if !reflect.DeepEqual(cm.BinaryData, newData) {
		t.Errorf("set binary data failed: %+v", cm.BinaryData)
	}
}

func TestSetConfigMapImmutable(t *testing.T) {
	cm := CreateConfigMap("cm", "ns")
	SetConfigMapImmutable(cm, true)
	if cm.Immutable == nil || !*cm.Immutable {
		t.Errorf("immutable not set to true")
	}
	SetConfigMapImmutable(cm, false)
	if cm.Immutable == nil || *cm.Immutable {
		t.Errorf("immutable not updated to false")
	}
}
