package kubernetes_test

import (
	"reflect"
	"testing"

	kubernetes "github.com/go-kure/kure/pkg/kubernetes"
)

func TestAddConfigMapData(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	kubernetes.AddConfigMapData(cm, "key", "val")
	if cm.Data["key"] != "val" {
		t.Errorf("AddConfigMapData: expected 'val', got %q", cm.Data["key"])
	}
}

// TestConfigMapDataMerge covers what AddConfigMapDataMap used to do: merging a
// map is a loop over the single-key helper.
func TestConfigMapDataMerge(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapData(cm, "keep", "me")

	more := map[string]string{"keep": "replaced", "a": "b"}
	for k, v := range more {
		kubernetes.AddConfigMapData(cm, k, v)
	}

	want := map[string]string{"keep": "replaced", "a": "b"}
	if !reflect.DeepEqual(cm.Data, want) {
		t.Errorf("Data = %+v, want %+v", cm.Data, want)
	}
}

// TestConfigMapDataReplacement covers what SetConfigMapData used to do: a
// wholesale replacement is a field assignment, so it has no helper.
func TestConfigMapDataReplacement(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapData(cm, "old", "value")

	newData := map[string]string{"x": "y"}
	cm.Data = newData
	if !reflect.DeepEqual(cm.Data, newData) {
		t.Errorf("Data: got %+v", cm.Data)
	}
}

func TestAddConfigMapBinaryData(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	kubernetes.AddConfigMapBinaryData(cm, "bin", []byte{1, 2})
	if !reflect.DeepEqual(cm.BinaryData["bin"], []byte{1, 2}) {
		t.Errorf("AddConfigMapBinaryData: unexpected value")
	}
}

func TestConfigMapBinaryDataMerge(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")

	more := map[string][]byte{"b1": {2, 3}, "b2": {4}}
	for k, v := range more {
		kubernetes.AddConfigMapBinaryData(cm, k, v)
	}
	if !reflect.DeepEqual(cm.BinaryData, more) {
		t.Errorf("BinaryData = %+v, want %+v", cm.BinaryData, more)
	}
}

// TestConfigMapBinaryDataReplacement is the SetConfigMapBinaryData equivalent:
// assign the field.
func TestConfigMapBinaryDataReplacement(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddConfigMapBinaryData(cm, "old", []byte{0})

	newData := map[string][]byte{"x": {9}}
	cm.BinaryData = newData
	if !reflect.DeepEqual(cm.BinaryData, newData) {
		t.Errorf("BinaryData: got %+v", cm.BinaryData)
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

// TestConfigMapMetadataViaGenericHelpers covers what AddConfigMapLabel and
// AddConfigMapAnnotation used to do.
func TestConfigMapMetadataViaGenericHelpers(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	kubernetes.AddLabel(cm, "env", "prod")
	kubernetes.AddAnnotation(cm, "owner", "team")
	if cm.Labels["env"] != "prod" {
		t.Errorf("AddLabel: label not set: %+v", cm.Labels)
	}
	if cm.Annotations["owner"] != "team" {
		t.Errorf("AddAnnotation: annotation not set: %+v", cm.Annotations)
	}
}

// TestConfigMapMetadataReplacement covers what SetConfigMapLabels and
// SetConfigMapAnnotations used to do: a whole-map replacement is a direct field
// assignment, not builder sugar.
func TestConfigMapMetadataReplacement(t *testing.T) {
	cm := kubernetes.CreateConfigMap("cm", "ns")
	labels := map[string]string{"x": "y"}
	anns := map[string]string{"c": "d"}
	cm.Labels = labels
	cm.Annotations = anns
	if !reflect.DeepEqual(cm.Labels, labels) {
		t.Errorf("labels: got %+v", cm.Labels)
	}
	if !reflect.DeepEqual(cm.Annotations, anns) {
		t.Errorf("annotations: got %+v", cm.Annotations)
	}
}
