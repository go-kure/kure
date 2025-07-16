package yaml

import (
	"os"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestParse(t *testing.T) {
	data := `apiVersion: v1
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
`
	objs, err := Parse([]byte(data))
	if err != nil {
		t.Fatalf("Parse returned error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
	if sa, ok := objs[0].(*corev1.ServiceAccount); !ok || sa.Name != "sa" {
		t.Fatalf("unexpected first object: %#v", objs[0])
	}
	if pod, ok := objs[1].(*corev1.Pod); !ok || pod.Name != "pod" {
		t.Fatalf("unexpected second object: %#v", objs[1])
	}
}

func TestParseFile(t *testing.T) {
	data := `apiVersion: v1
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
`
	dir := t.TempDir()
	path := filepath.Join(dir, "objs.yaml")
	if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	objs, err := ParseFile(path)
	if err != nil {
		t.Fatalf("ParseFile returned error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}
	if sa, ok := objs[0].(*corev1.ServiceAccount); !ok || sa.Name != "sa" {
		t.Fatalf("unexpected first object: %#v", objs[0])
	}
	if pod, ok := objs[1].(*corev1.Pod); !ok || pod.Name != "pod" {
		t.Fatalf("unexpected second object: %#v", objs[1])
	}
}
