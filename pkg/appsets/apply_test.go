package appsets

import (
	"os"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func writeTempFile(t *testing.T, content string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "file-*.yaml")
	if err != nil {
		t.Fatalf("temp create: %v", err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	return f.Name()
}

func TestApplyPatch(t *testing.T) {
	base := `apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  labels:
    app: demo
data:
  foo: bar
`
	patch := `- target: demo
  patch:
    data.foo: baz
    metadata.labels.env: prod
`
	basePath := writeTempFile(t, base)
	patchPath := writeTempFile(t, patch)

	objs, err := ApplyPatch(basePath, patchPath)
	if err != nil {
		t.Fatalf("ApplyPatch: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	labels, found, err := unstructured.NestedStringMap(objs[0].Object, "metadata", "labels")
	if err != nil || !found {
		t.Fatalf("labels missing")
	}
	if labels["env"] != "prod" {
		t.Fatalf("patch not applied: %+v", labels)
	}
}
