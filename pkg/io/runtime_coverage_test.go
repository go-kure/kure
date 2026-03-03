package io

import (
	"testing"
)

func TestParseYAML_Basic(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: sa
---
apiVersion: v1
kind: Pod
metadata:
  name: pod
spec:
  containers: []
`)
	objs, err := ParseYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
}

func TestParseYAML_EmptyDocument(t *testing.T) {
	data := []byte(`---
---
`)
	objs, err := ParseYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if len(objs) != 0 {
		t.Fatalf("expected 0 objects, got %d", len(objs))
	}
}

func TestParseYAML_SingleDocument(t *testing.T) {
	data := []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: test
data:
  key: value
`)
	objs, err := ParseYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML returned error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	if objs[0].GetName() != "test" {
		t.Errorf("expected name 'test', got %q", objs[0].GetName())
	}
}

func TestParseFileWithOptions_MissingFile(t *testing.T) {
	_, err := ParseFileWithOptions("/nonexistent/file.yaml", ParseOptions{})
	if err == nil {
		t.Fatalf("expected error for missing file, got nil")
	}
}

func TestParse_EmptyBytes(t *testing.T) {
	objs, err := parse([]byte{}, ParseOptions{})
	if err != nil {
		t.Fatalf("parse returned error for empty input: %v", err)
	}
	if len(objs) != 0 {
		t.Fatalf("expected 0 objects, got %d", len(objs))
	}
}

func TestCheckType_NilObject(t *testing.T) {
	err := checkType(nil)
	if err == nil {
		t.Fatalf("expected error for nil object, got nil")
	}
}
