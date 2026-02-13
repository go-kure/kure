package io

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type demo struct {
	Name string `yaml:"name"`
	Age  int    `yaml:"age"`
}

func TestBufferMarshalUnmarshal(t *testing.T) {
	b := &Buffer{}
	in := demo{Name: "test", Age: 5}
	if err := b.Marshal(in); err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out demo
	if err := b.Unmarshal(&out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Fatalf("round trip mismatch: %#v != %#v", in, out)
	}
}

func TestEncodeObjectsToYAMLWithOptions(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName("test-deploy")
	obj.SetNamespace("default")
	obj.Object["spec"] = map[string]interface{}{
		"replicas": int64(3),
		"selector": map[string]interface{}{
			"matchLabels": map[string]interface{}{
				"app": "test",
			},
		},
	}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	opts := EncodeOptions{KubernetesFieldOrder: true}
	out, err := EncodeObjectsToYAMLWithOptions(objects, opts)
	if err != nil {
		t.Fatalf("EncodeObjectsToYAMLWithOptions: %v", err)
	}

	s := string(out)

	// Verify Kubernetes-conventional field order
	apiVersionIdx := strings.Index(s, "apiVersion:")
	kindIdx := strings.Index(s, "kind:")
	metadataIdx := strings.Index(s, "metadata:")
	specIdx := strings.Index(s, "spec:")

	if apiVersionIdx >= kindIdx {
		t.Errorf("apiVersion should come before kind:\n%s", s)
	}
	if kindIdx >= metadataIdx {
		t.Errorf("kind should come before metadata:\n%s", s)
	}
	if metadataIdx >= specIdx {
		t.Errorf("metadata should come before spec:\n%s", s)
	}

	// Verify the content is valid YAML with expected values
	if !strings.Contains(s, "apiVersion: apps/v1") {
		t.Errorf("expected apiVersion: apps/v1 in output:\n%s", s)
	}
	if !strings.Contains(s, "kind: Deployment") {
		t.Errorf("expected kind: Deployment in output:\n%s", s)
	}
}

func TestEncodeObjectsToYAML_BackwardCompatible(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("test-cm")
	obj.SetNamespace("default")
	obj.Object["data"] = map[string]interface{}{
		"key": "value",
	}

	co := client.Object(obj)
	objects := []*client.Object{&co}

	out, err := EncodeObjectsToYAML(objects)
	if err != nil {
		t.Fatalf("EncodeObjectsToYAML: %v", err)
	}

	s := string(out)

	// Verify the existing function still produces valid YAML
	if !strings.Contains(s, "apiVersion: v1") {
		t.Errorf("expected apiVersion: v1 in output:\n%s", s)
	}
	if !strings.Contains(s, "kind: ConfigMap") {
		t.Errorf("expected kind: ConfigMap in output:\n%s", s)
	}
	if !strings.Contains(s, "key: value") {
		t.Errorf("expected data key in output:\n%s", s)
	}

	// Should NOT have creationTimestamp (it's null and should be stripped)
	if strings.Contains(s, "creationTimestamp") {
		t.Errorf("expected creationTimestamp to be stripped:\n%s", s)
	}
}

func TestSaveLoadFile(t *testing.T) {
	d := demo{Name: "file", Age: 8}
	dir := t.TempDir()
	path := filepath.Join(dir, "demo.yaml")
	if err := SaveFile(path, d); err != nil {
		t.Fatalf("save: %v", err)
	}
	var out demo
	if err := LoadFile(path, &out); err != nil {
		t.Fatalf("load: %v", err)
	}
	if !reflect.DeepEqual(d, out) {
		t.Fatalf("file round trip mismatch: %#v != %#v", d, out)
	}
}
