package io

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ---------- isDeepEmpty ----------

func TestIsDeepEmpty(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]interface{}
		want bool
	}{
		{
			name: "empty map",
			m:    map[string]interface{}{},
			want: true,
		},
		{
			name: "nested empty maps",
			m: map[string]interface{}{
				"a": map[string]interface{}{},
				"b": map[string]interface{}{
					"c": map[string]interface{}{},
				},
			},
			want: true,
		},
		{
			name: "map with string value",
			m: map[string]interface{}{
				"key": "value",
			},
			want: false,
		},
		{
			name: "nested map with deep non-empty value",
			m: map[string]interface{}{
				"a": map[string]interface{}{
					"b": map[string]interface{}{
						"c": "value",
					},
				},
			},
			want: false,
		},
		{
			name: "map with nil value",
			m: map[string]interface{}{
				"key": nil,
			},
			want: false,
		},
		{
			name: "map with int value",
			m: map[string]interface{}{
				"key": 42,
			},
			want: false,
		},
		{
			name: "mixed empty and non-empty nested maps",
			m: map[string]interface{}{
				"empty":    map[string]interface{}{},
				"notempty": map[string]interface{}{"k": "v"},
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDeepEmpty(tt.m)
			if got != tt.want {
				t.Errorf("isDeepEmpty() = %v, want %v", got, tt.want)
			}
		})
	}
}

// ---------- removeEmptyStatus ----------

func TestRemoveEmptyStatus(t *testing.T) {
	tests := []struct {
		name       string
		m          map[string]interface{}
		wantStatus bool // whether "status" key should remain
	}{
		{
			name:       "no status key",
			m:          map[string]interface{}{"apiVersion": "v1"},
			wantStatus: false,
		},
		{
			name:       "nil status",
			m:          map[string]interface{}{"status": nil},
			wantStatus: false,
		},
		{
			name:       "empty map status",
			m:          map[string]interface{}{"status": map[string]interface{}{}},
			wantStatus: false,
		},
		{
			name: "deep empty map status",
			m: map[string]interface{}{
				"status": map[string]interface{}{
					"nested": map[string]interface{}{},
				},
			},
			wantStatus: false,
		},
		{
			name: "non-empty status",
			m: map[string]interface{}{
				"status": map[string]interface{}{
					"phase": "Running",
				},
			},
			wantStatus: true,
		},
		{
			name:       "string status (non-map, non-nil)",
			m:          map[string]interface{}{"status": "active"},
			wantStatus: true,
		},
		{
			name:       "int status (non-map, non-nil)",
			m:          map[string]interface{}{"status": 42},
			wantStatus: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			removeEmptyStatus(tt.m)
			_, exists := tt.m["status"]
			if exists != tt.wantStatus {
				t.Errorf("status key exists = %v, want %v", exists, tt.wantStatus)
			}
		})
	}
}

// ---------- SaveFile / LoadFile error paths ----------

func TestSaveFile_InvalidPath(t *testing.T) {
	err := SaveFile("/nonexistent/dir/file.yaml", demo{Name: "test"})
	if err == nil {
		t.Fatalf("expected error for invalid path, got nil")
	}
}

func TestLoadFile_MissingFile(t *testing.T) {
	var d demo
	err := LoadFile("/nonexistent/dir/file.yaml", &d)
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestSaveFile_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.yaml")

	in := demo{Name: "roundtrip", Age: 42}
	if err := SaveFile(path, in); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	var out demo
	if err := LoadFile(path, &out); err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if in.Name != out.Name || in.Age != out.Age {
		t.Errorf("round trip mismatch: got %+v, want %+v", out, in)
	}
}

// ---------- Unmarshal with error reader ----------

type errReader struct{}

func (e errReader) Read([]byte) (int, error) {
	return 0, errors.New("read error")
}

func TestUnmarshal_ReadError(t *testing.T) {
	var d demo
	err := Unmarshal(errReader{}, &d)
	if err == nil {
		t.Fatalf("expected error from errReader, got nil")
	}
	if !strings.Contains(err.Error(), "read error") {
		t.Errorf("expected 'read error' in error message, got: %v", err)
	}
}

func TestUnmarshal_ValidData(t *testing.T) {
	data := "name: hello\nage: 10\n"
	var d demo
	err := Unmarshal(strings.NewReader(data), &d)
	if err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if d.Name != "hello" || d.Age != 10 {
		t.Errorf("unexpected result: %+v", d)
	}
}

// ---------- Marshal with writer ----------

func TestMarshal_Writer(t *testing.T) {
	var buf bytes.Buffer
	err := Marshal(&buf, demo{Name: "test", Age: 5})
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	s := buf.String()
	if !strings.Contains(s, "test") {
		t.Errorf("expected 'test' in output, got: %s", s)
	}
	if !strings.Contains(s, "5") {
		t.Errorf("expected '5' in output, got: %s", s)
	}
}

// ---------- marshalCleanResource edge cases ----------

func TestMarshalCleanResource_NoKubernetesFieldOrder(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")

	out, err := marshalCleanResource(obj, EncodeOptions{
		KubernetesFieldOrder: false,
		ServerFieldStripping: StripServerFieldsFull,
	})
	if err != nil {
		t.Fatalf("marshalCleanResource: %v", err)
	}
	if !strings.Contains(string(out), "kind: ConfigMap") {
		t.Errorf("expected 'kind: ConfigMap' in output, got: %s", out)
	}
}

func TestMarshalCleanResource_WithKubernetesFieldOrder(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")

	out, err := marshalCleanResource(obj, EncodeOptions{
		KubernetesFieldOrder: true,
		ServerFieldStripping: StripServerFieldsFull,
	})
	if err != nil {
		t.Fatalf("marshalCleanResource: %v", err)
	}
	s := string(out)

	apiIdx := strings.Index(s, "apiVersion:")
	kindIdx := strings.Index(s, "kind:")
	if apiIdx >= kindIdx {
		t.Errorf("apiVersion should come before kind: %s", s)
	}
}

// ---------- EncodeObjectsToYAMLWithOptions error in marshalCleanResource ----------

func TestEncodeObjectsToYAMLWithOptions_MultipleObjects(t *testing.T) {
	obj1 := &unstructured.Unstructured{}
	obj1.SetAPIVersion("v1")
	obj1.SetKind("ConfigMap")
	obj1.SetName("cm1")

	obj2 := &unstructured.Unstructured{}
	obj2.SetAPIVersion("v1")
	obj2.SetKind("Secret")
	obj2.SetName("s1")

	co1 := client.Object(obj1)
	co2 := client.Object(obj2)
	objects := []*client.Object{&co1, &co2}

	out, err := EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{})
	if err != nil {
		t.Fatalf("EncodeObjectsToYAMLWithOptions: %v", err)
	}
	s := string(out)

	// Should have separator between objects
	if !strings.Contains(s, "---") {
		t.Errorf("expected YAML separator between objects: %s", s)
	}
	if !strings.Contains(s, "name: cm1") {
		t.Errorf("expected cm1 in output: %s", s)
	}
	if !strings.Contains(s, "name: s1") {
		t.Errorf("expected s1 in output: %s", s)
	}
}

// ---------- cleanResourceMap edge cases ----------

func TestCleanResourceMap_NoSpecKey(t *testing.T) {
	m := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name":              "test",
			"creationTimestamp": nil,
		},
	}

	cleanResourceMap(m, StripServerFieldsFull)

	// creationTimestamp should be removed
	md := m["metadata"].(map[string]interface{})
	if _, exists := md["creationTimestamp"]; exists {
		t.Errorf("expected creationTimestamp to be removed")
	}
}

func TestCleanResourceMap_SpecWithoutTemplate(t *testing.T) {
	m := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name":              "test",
			"creationTimestamp": nil,
		},
		"spec": map[string]interface{}{
			"type": "ClusterIP",
		},
	}

	cleanResourceMap(m, StripServerFieldsFull)

	// Should not panic or error when spec has no template
	md := m["metadata"].(map[string]interface{})
	if _, exists := md["creationTimestamp"]; exists {
		t.Errorf("expected creationTimestamp to be removed")
	}
}

func TestCleanResourceMap_MetadataNotAMap(t *testing.T) {
	m := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata":   "not-a-map",
	}

	// Should not panic when metadata is not a map
	cleanResourceMap(m, StripServerFieldsFull)
}

func TestCleanResourceMap_SpecNotAMap(t *testing.T) {
	m := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"name": "test",
		},
		"spec": "not-a-map",
	}

	// Should not panic when spec is not a map
	cleanResourceMap(m, StripServerFieldsFull)
}

// ---------- cleanMetadata edge cases ----------

func TestCleanMetadata_TemplateNotAMap(t *testing.T) {
	parent := map[string]interface{}{
		"template": "not-a-map",
	}

	// Should not panic when template is not a map
	cleanMetadata(parent, "template", StripServerFieldsFull)
}

func TestCleanMetadata_TemplateWithoutMetadata(t *testing.T) {
	parent := map[string]interface{}{
		"template": map[string]interface{}{
			"spec": map[string]interface{}{},
		},
	}

	// Should not panic when template has no metadata
	cleanMetadata(parent, "template", StripServerFieldsFull)
}

func TestCleanMetadata_MetadataDirectly(t *testing.T) {
	parent := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name":              "test",
			"creationTimestamp": nil,
			"uid":               "abc-123",
		},
	}

	cleanMetadata(parent, "metadata", StripServerFieldsFull)

	md := parent["metadata"].(map[string]interface{})
	if _, exists := md["creationTimestamp"]; exists {
		t.Errorf("expected creationTimestamp to be removed")
	}
	if _, exists := md["uid"]; exists {
		t.Errorf("expected uid to be removed")
	}
}
