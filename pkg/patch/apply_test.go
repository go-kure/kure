package patch

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

func TestInsertAfterAtListBoundary(t *testing.T) {
	obj := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}

	op := PatchOp{
		Op:       "insertAfter",
		Path:     "items",
		Selector: "2", // index == len(list)-1, resolveListIndex allows i == len(list) for numeric
		Value:    "d",
	}

	if err := applyPatchOp(obj, op); err != nil {
		t.Fatalf("insertAfter at last element: %v", err)
	}

	items, _, _ := unstructured.NestedSlice(obj, "items")
	if len(items) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items))
	}
	if items[3] != "d" {
		t.Errorf("expected 'd' at index 3, got %v", items[3])
	}

	// Test insertAfter with index == len(list) (boundary case that previously panicked)
	obj2 := map[string]interface{}{
		"items": []interface{}{"a", "b", "c"},
	}

	op2 := PatchOp{
		Op:       "insertAfter",
		Path:     "items",
		Selector: "3", // index == len(list), should append
		Value:    "d",
	}

	if err := applyPatchOp(obj2, op2); err != nil {
		t.Fatalf("insertAfter at list boundary (idx == len): %v", err)
	}

	items2, _, _ := unstructured.NestedSlice(obj2, "items")
	if len(items2) != 4 {
		t.Fatalf("expected 4 items, got %d", len(items2))
	}
	if items2[3] != "d" {
		t.Errorf("expected 'd' at index 3, got %v", items2[3])
	}
}
