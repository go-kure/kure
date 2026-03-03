package patch

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func testObj(fields map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: fields}
}

// ---------------------------------------------------------------------------
// applyPatchOp (via ResourceWithPatches.Apply)
// ---------------------------------------------------------------------------

func TestApply_Replace(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app": "old",
			},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "replace", Path: "metadata.labels.app", Value: "new"},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	val, found, err := unstructured.NestedString(obj.Object, "metadata", "labels", "app")
	if err != nil || !found {
		t.Fatalf("field not found after replace")
	}
	if val != "new" {
		t.Fatalf("expected 'new', got %q", val)
	}
}

func TestApply_ReplaceWithSelector(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "main", "image": "nginx:1.24"},
				map[string]any{"name": "sidecar", "image": "envoy:1.0"},
			},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "replace", Path: "spec.containers", Selector: "name=main", Value: map[string]any{"image": "nginx:1.25"}},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "containers")
	item, ok := containers[0].(map[string]any)
	if !ok {
		t.Fatalf("expected map at index 0")
	}
	if item["image"] != "nginx:1.25" {
		t.Fatalf("expected image 'nginx:1.25', got %v", item["image"])
	}
}

func TestApply_Delete(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{
				"app": "demo",
				"env": "prod",
			},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "delete", Path: "metadata.labels.env"},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	_, found, _ := unstructured.NestedString(obj.Object, "metadata", "labels", "env")
	if found {
		t.Fatalf("field should have been deleted")
	}
}

func TestApply_DeleteWithSelector(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "main", "image": "nginx"},
				map[string]any{"name": "sidecar", "image": "envoy"},
			},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "delete", Path: "spec.containers", Selector: "name=sidecar"},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	containers, _, _ := unstructured.NestedSlice(obj.Object, "spec", "containers")
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	item := containers[0].(map[string]any)
	if item["name"] != "main" {
		t.Fatalf("expected 'main' to remain, got %v", item["name"])
	}
}

func TestApply_Append(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{
			"items": []any{"a", "b"},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "append", Path: "spec.items", Value: "c"},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	items, _, _ := unstructured.NestedSlice(obj.Object, "spec", "items")
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	if items[2] != "c" {
		t.Fatalf("expected 'c' at index 2, got %v", items[2])
	}
}

func TestApply_InsertBefore(t *testing.T) {
	obj := testObj(map[string]any{
		"items": []any{
			map[string]any{"name": "a"},
			map[string]any{"name": "b"},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "insertBefore", Path: "items", Selector: "name=b", Value: map[string]any{"name": "inserted"}},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	items, _, _ := unstructured.NestedSlice(obj.Object, "items")
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	mid := items[1].(map[string]any)
	if mid["name"] != "inserted" {
		t.Fatalf("expected 'inserted' at index 1, got %v", mid["name"])
	}
}

func TestApply_InsertAfter(t *testing.T) {
	obj := testObj(map[string]any{
		"items": []any{
			map[string]any{"name": "a"},
			map[string]any{"name": "b"},
		},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "insertAfter", Path: "items", Selector: "name=a", Value: map[string]any{"name": "inserted"}},
		},
	}
	if err := r.Apply(); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	items, _, _ := unstructured.NestedSlice(obj.Object, "items")
	if len(items) != 3 {
		t.Fatalf("expected 3 items, got %d", len(items))
	}
	mid := items[1].(map[string]any)
	if mid["name"] != "inserted" {
		t.Fatalf("expected 'inserted' at index 1, got %v", mid["name"])
	}
}

func TestApply_InvalidOp(t *testing.T) {
	obj := testObj(map[string]any{"foo": "bar"})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "bogus", Path: "foo", Value: "baz"},
		},
	}
	if err := r.Apply(); err == nil {
		t.Fatalf("expected error for unknown op")
	}
}

func TestApply_DeleteNotFound(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "delete", Path: "metadata.labels.nonexistent"},
		},
	}
	if err := r.Apply(); err == nil {
		t.Fatalf("expected error when deleting non-existent path")
	}
}

func TestApply_AppendNotFound(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{},
	})
	r := &ResourceWithPatches{
		Name: "test",
		Base: obj,
		Patches: []PatchOp{
			{Op: "append", Path: "spec.missing", Value: "x"},
		},
	}
	if err := r.Apply(); err == nil {
		t.Fatalf("expected error when appending to missing list")
	}
}

// ---------------------------------------------------------------------------
// resolveListIndex
// ---------------------------------------------------------------------------

func TestResolveListIndex_KeyValue(t *testing.T) {
	list := []any{
		map[string]any{"name": "alpha"},
		map[string]any{"name": "beta"},
	}
	idx, err := resolveListIndex(list, "name=beta")
	if err != nil {
		t.Fatalf("resolveListIndex: %v", err)
	}
	if idx != 1 {
		t.Fatalf("expected index 1, got %d", idx)
	}
}

func TestResolveListIndex_NumericIndex(t *testing.T) {
	list := []any{"a", "b", "c"}
	idx, err := resolveListIndex(list, "2")
	if err != nil {
		t.Fatalf("resolveListIndex: %v", err)
	}
	if idx != 2 {
		t.Fatalf("expected index 2, got %d", idx)
	}
}

func TestResolveListIndex_NegativeIndex(t *testing.T) {
	list := []any{"a", "b", "c"}
	idx, err := resolveListIndex(list, "-1")
	if err != nil {
		t.Fatalf("resolveListIndex: %v", err)
	}
	if idx != 2 {
		t.Fatalf("expected index 2 (last element), got %d", idx)
	}
}

func TestResolveListIndex_OutOfBounds(t *testing.T) {
	list := []any{"a", "b"}
	_, err := resolveListIndex(list, "10")
	if err == nil {
		t.Fatalf("expected error for out-of-bounds index")
	}
}

func TestResolveListIndex_InvalidSelector(t *testing.T) {
	list := []any{"a"}
	_, err := resolveListIndex(list, "notanumber")
	if err == nil {
		t.Fatalf("expected error for invalid selector")
	}
}

func TestResolveListIndex_KeyValueNotFound(t *testing.T) {
	list := []any{
		map[string]any{"name": "alpha"},
	}
	_, err := resolveListIndex(list, "name=missing")
	if err == nil {
		t.Fatalf("expected error when key=value not found")
	}
}

// ---------------------------------------------------------------------------
// applyArrayReplace
// ---------------------------------------------------------------------------

func TestApplyArrayReplace_NestedPatch(t *testing.T) {
	obj := map[string]any{
		"containers": []any{
			map[string]any{"name": "main", "image": "nginx:1.24"},
		},
	}
	op := PatchOp{
		Op:       "replace",
		Path:     "containers",
		Selector: "name=main",
		Value:    map[string]any{"image": "nginx:1.25"},
	}
	if err := applyArrayReplace(obj, op); err != nil {
		t.Fatalf("applyArrayReplace: %v", err)
	}
	containers := obj["containers"].([]any)
	item := containers[0].(map[string]any)
	if item["image"] != "nginx:1.25" {
		t.Fatalf("expected 'nginx:1.25', got %v", item["image"])
	}
	// name should still be present
	if item["name"] != "main" {
		t.Fatalf("expected name 'main' to be preserved, got %v", item["name"])
	}
}

func TestApplyArrayReplace_DirectReplace(t *testing.T) {
	obj := map[string]any{
		"items": []any{"a", "b", "c"},
	}
	op := PatchOp{
		Op:       "replace",
		Path:     "items",
		Selector: "1",
		Value:    "replaced",
	}
	if err := applyArrayReplace(obj, op); err != nil {
		t.Fatalf("applyArrayReplace: %v", err)
	}
	items := obj["items"].([]any)
	if items[1] != "replaced" {
		t.Fatalf("expected 'replaced', got %v", items[1])
	}
}

func TestApplyArrayReplace_NonObjectItem(t *testing.T) {
	obj := map[string]any{
		"items": []any{"a", "b"},
	}
	// A single-key map value triggers the nested-patch branch; item at index
	// is a string, not a map, so it should error.
	op := PatchOp{
		Op:       "replace",
		Path:     "items",
		Selector: "0",
		Value:    map[string]any{"field": "val"},
	}
	if err := applyArrayReplace(obj, op); err == nil {
		t.Fatalf("expected error when array item is not a map")
	}
}

// ---------------------------------------------------------------------------
// ParsePatchLine
// ---------------------------------------------------------------------------

func TestParsePatchLine_Append(t *testing.T) {
	op, err := ParsePatchLine("spec.items[-]", "val")
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "append" {
		t.Fatalf("expected op 'append', got %q", op.Op)
	}
	if op.Path != "spec.items" {
		t.Fatalf("expected path 'spec.items', got %q", op.Path)
	}
}

func TestParsePatchLine_Delete(t *testing.T) {
	op, err := ParsePatchLine("metadata.labels.env[delete]", "")
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "delete" {
		t.Fatalf("expected op 'delete', got %q", op.Op)
	}
	if op.Path != "metadata.labels.env" {
		t.Fatalf("expected path 'metadata.labels.env', got %q", op.Path)
	}
	if op.Selector != "" {
		t.Fatalf("expected empty selector, got %q", op.Selector)
	}
}

func TestParsePatchLine_DeleteWithSelector(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[delete=name=foo]", "")
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "delete" {
		t.Fatalf("expected op 'delete', got %q", op.Op)
	}
	if op.Path != "spec.containers" {
		t.Fatalf("expected path 'spec.containers', got %q", op.Path)
	}
	if op.Selector != "name=foo" {
		t.Fatalf("expected selector 'name=foo', got %q", op.Selector)
	}
}

func TestParsePatchLine_InsertBefore(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[-name=foo]", map[string]any{"image": "nginx"})
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "insertBefore" {
		t.Fatalf("expected op 'insertBefore', got %q", op.Op)
	}
	if op.Selector != "name=foo" {
		t.Fatalf("expected selector 'name=foo', got %q", op.Selector)
	}
	if op.Path != "spec.containers" {
		t.Fatalf("expected path 'spec.containers', got %q", op.Path)
	}
}

func TestParsePatchLine_InsertAfter(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[+name=foo]", map[string]any{"image": "nginx"})
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "insertAfter" {
		t.Fatalf("expected op 'insertAfter', got %q", op.Op)
	}
	if op.Selector != "name=foo" {
		t.Fatalf("expected selector 'name=foo', got %q", op.Selector)
	}
}

func TestParsePatchLine_InsertBeforeIndex(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[-3]", map[string]any{"name": "x"})
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "insertBefore" {
		t.Fatalf("expected op 'insertBefore', got %q", op.Op)
	}
	if op.Selector != "3" {
		t.Fatalf("expected selector '3', got %q", op.Selector)
	}
}

func TestParsePatchLine_InsertAfterIndex(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[+2]", map[string]any{"name": "x"})
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "insertAfter" {
		t.Fatalf("expected op 'insertAfter', got %q", op.Op)
	}
	if op.Selector != "2" {
		t.Fatalf("expected selector '2', got %q", op.Selector)
	}
}

func TestParsePatchLine_ReplaceWithSelector(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[name=main]", map[string]any{"image": "nginx"})
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "replace" {
		t.Fatalf("expected op 'replace', got %q", op.Op)
	}
	if op.Selector != "name=main" {
		t.Fatalf("expected selector 'name=main', got %q", op.Selector)
	}
}

func TestParsePatchLine_MidSelector(t *testing.T) {
	op, err := ParsePatchLine("spec.containers[name=main].image", "nginx:latest")
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "replace" {
		t.Fatalf("expected op 'replace', got %q", op.Op)
	}
	if op.Selector != "name=main" {
		t.Fatalf("expected selector 'name=main', got %q", op.Selector)
	}
	if op.Path != "spec.containers" {
		t.Fatalf("expected path 'spec.containers', got %q", op.Path)
	}
	valueMap, ok := op.Value.(map[string]any)
	if !ok {
		t.Fatalf("expected Value to be map, got %T", op.Value)
	}
	if valueMap["image"] != "nginx:latest" {
		t.Fatalf("expected remaining path value 'nginx:latest', got %v", valueMap["image"])
	}
}

func TestParsePatchLine_PlainReplace(t *testing.T) {
	op, err := ParsePatchLine("metadata.labels.app", "newval")
	if err != nil {
		t.Fatalf("ParsePatchLine: %v", err)
	}
	if op.Op != "replace" {
		t.Fatalf("expected op 'replace', got %q", op.Op)
	}
	if op.Path != "metadata.labels.app" {
		t.Fatalf("expected path 'metadata.labels.app', got %q", op.Path)
	}
	if op.Selector != "" {
		t.Fatalf("expected empty selector, got %q", op.Selector)
	}
	if op.Value != "newval" {
		t.Fatalf("expected value 'newval', got %v", op.Value)
	}
}

// ---------------------------------------------------------------------------
// InferPatchOp
// ---------------------------------------------------------------------------

func TestInferPatchOp(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{name: "insert after key", path: "containers[+name=x]", want: "insertafter"},
		{name: "insert before key", path: "containers[-name=x]", want: "insertbefore"},
		{name: "insert after index", path: "items[+2]", want: "insertafter"},
		{name: "insert before index", path: "items[-3]", want: "insertbefore"},
		// Note: "[-]" is caught by the first regex \[([+-])[^0-9] before the
		// HasSuffix("[-]") append check, so InferPatchOp returns "insertbefore".
		{name: "append bracket", path: "items[-]", want: "insertbefore"},
		{name: "plain replace", path: "metadata.labels.app", want: "replace"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := InferPatchOp(tt.path)
			if got != tt.want {
				t.Errorf("InferPatchOp(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// ParsePatchPath
// ---------------------------------------------------------------------------

func TestParsePatchPath_Simple(t *testing.T) {
	parts, err := ParsePatchPath("a.b.c")
	if err != nil {
		t.Fatalf("ParsePatchPath: %v", err)
	}
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	for i, want := range []string{"a", "b", "c"} {
		if parts[i].Field != want {
			t.Errorf("part %d: expected field %q, got %q", i, want, parts[i].Field)
		}
		if parts[i].MatchType != "" {
			t.Errorf("part %d: expected no MatchType, got %q", i, parts[i].MatchType)
		}
	}
}

func TestParsePatchPath_WithKeySelector(t *testing.T) {
	parts, err := ParsePatchPath("containers[name=app].image")
	if err != nil {
		t.Fatalf("ParsePatchPath: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Field != "containers" || parts[0].MatchType != "key" || parts[0].MatchValue != "name=app" {
		t.Fatalf("unexpected part 0: %+v", parts[0])
	}
	if parts[1].Field != "image" || parts[1].MatchType != "" {
		t.Fatalf("unexpected part 1: %+v", parts[1])
	}
}

func TestParsePatchPath_WithIndexSelector(t *testing.T) {
	parts, err := ParsePatchPath("items[0].name")
	if err != nil {
		t.Fatalf("ParsePatchPath: %v", err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected 2 parts, got %d", len(parts))
	}
	if parts[0].Field != "items" || parts[0].MatchType != "index" || parts[0].MatchValue != "0" {
		t.Fatalf("unexpected part 0: %+v", parts[0])
	}
}

func TestParsePatchPath_EmptyPath(t *testing.T) {
	_, err := ParsePatchPath("")
	if err == nil {
		t.Fatalf("expected error for empty path")
	}
}

func TestParsePatchPath_EmptySegment(t *testing.T) {
	_, err := ParsePatchPath("a..b")
	if err == nil {
		t.Fatalf("expected error for empty segment")
	}
}

func TestParsePatchPath_MalformedSelector(t *testing.T) {
	_, err := ParsePatchPath("a[b")
	if err == nil {
		t.Fatalf("expected error for malformed selector")
	}
}

func TestParsePatchPath_EmptySelector(t *testing.T) {
	_, err := ParsePatchPath("a[]")
	if err == nil {
		t.Fatalf("expected error for empty selector")
	}
}

func TestParsePatchPath_InvalidIndex(t *testing.T) {
	_, err := ParsePatchPath("a[abc]")
	if err == nil {
		t.Fatalf("expected error for invalid index")
	}
}

// ---------------------------------------------------------------------------
// convertValueForUnstructured
// ---------------------------------------------------------------------------

func TestConvertValueForUnstructured(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  any
	}{
		{name: "int to int64", input: int(42), want: int64(42)},
		{name: "int32 to int64", input: int32(7), want: int64(7)},
		{name: "int64 passthrough", input: int64(99), want: int64(99)},
		{name: "float32 to float64", input: float32(1.5), want: float64(1.5)},
		{name: "float64 passthrough", input: float64(2.5), want: float64(2.5)},
		{name: "bool", input: true, want: true},
		{name: "string", input: "hello", want: "hello"},
		{name: "nil", input: nil, want: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertValueForUnstructured(tt.input)
			if tt.input == nil {
				if got != nil {
					t.Errorf("expected nil, got %v", got)
				}
				return
			}
			if got != tt.want {
				t.Errorf("convertValueForUnstructured(%v) = %v (%T), want %v (%T)", tt.input, got, got, tt.want, tt.want)
			}
		})
	}
}

func TestConvertValueForUnstructured_MapRecursion(t *testing.T) {
	input := map[string]any{
		"count": int(5),
		"name":  "test",
	}
	got := convertValueForUnstructured(input)
	m, ok := got.(map[string]any)
	if !ok {
		t.Fatalf("expected map, got %T", got)
	}
	if m["count"] != int64(5) {
		t.Errorf("expected count int64(5), got %v (%T)", m["count"], m["count"])
	}
	if m["name"] != "test" {
		t.Errorf("expected name 'test', got %v", m["name"])
	}
}

func TestConvertValueForUnstructured_SliceRecursion(t *testing.T) {
	input := []any{int(1), int32(2), "three"}
	got := convertValueForUnstructured(input)
	s, ok := got.([]any)
	if !ok {
		t.Fatalf("expected slice, got %T", got)
	}
	if len(s) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(s))
	}
	if s[0] != int64(1) {
		t.Errorf("element 0: expected int64(1), got %v (%T)", s[0], s[0])
	}
	if s[1] != int64(2) {
		t.Errorf("element 1: expected int64(2), got %v (%T)", s[1], s[1])
	}
	if s[2] != "three" {
		t.Errorf("element 2: expected 'three', got %v", s[2])
	}
}

// ---------------------------------------------------------------------------
// ValidateAgainst
// ---------------------------------------------------------------------------

func TestValidateAgainst_ReplaceFound(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{"app": "demo"},
		},
	})
	p := &PatchOp{Op: "replace", Path: "metadata.labels.app", Value: "new"}
	if err := p.ValidateAgainst(obj); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateAgainst_ReplaceNotFound(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{},
	})
	p := &PatchOp{Op: "replace", Path: "metadata.labels.app", Value: "new"}
	if err := p.ValidateAgainst(obj); err == nil {
		t.Fatalf("expected error for missing replace path")
	}
}

func TestValidateAgainst_DeleteFound(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{
			"labels": map[string]any{"app": "demo"},
		},
	})
	p := &PatchOp{Op: "delete", Path: "metadata.labels.app"}
	if err := p.ValidateAgainst(obj); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateAgainst_DeleteNotFound(t *testing.T) {
	obj := testObj(map[string]any{
		"metadata": map[string]any{},
	})
	p := &PatchOp{Op: "delete", Path: "metadata.nonexistent"}
	if err := p.ValidateAgainst(obj); err == nil {
		t.Fatalf("expected error for missing delete path")
	}
}

func TestValidateAgainst_DeleteWithSelector(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{
			"containers": []any{
				map[string]any{"name": "main"},
			},
		},
	})
	p := &PatchOp{Op: "delete", Path: "spec.containers", Selector: "name=main"}
	if err := p.ValidateAgainst(obj); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateAgainst_AppendFound(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{
			"items": []any{"a"},
		},
	})
	p := &PatchOp{Op: "append", Path: "spec.items", Value: "b"}
	if err := p.ValidateAgainst(obj); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestValidateAgainst_AppendNotFound(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{},
	})
	p := &PatchOp{Op: "append", Path: "spec.missing", Value: "b"}
	if err := p.ValidateAgainst(obj); err == nil {
		t.Fatalf("expected error for missing append path")
	}
}

func TestValidateAgainst_InsertFound(t *testing.T) {
	obj := testObj(map[string]any{
		"spec": map[string]any{
			"items": []any{"a", "b"},
		},
	})
	for _, op := range []string{"insertBefore", "insertAfter"} {
		p := &PatchOp{Op: op, Path: "spec.items", Selector: "0", Value: "new"}
		if err := p.ValidateAgainst(obj); err != nil {
			t.Fatalf("ValidateAgainst(%s): expected no error, got %v", op, err)
		}
	}
}

// ---------------------------------------------------------------------------
// parsePath
// ---------------------------------------------------------------------------

func TestParsePath(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{name: "basic split", input: "a.b.c", want: []string{"a", "b", "c"}},
		{name: "empty string", input: "", want: []string{}},
		{name: "leading dot", input: ".a.b", want: []string{"a", "b"}},
		{name: "trailing dot", input: "a.b.", want: []string{"a", "b"}},
		{name: "both dots", input: ".a.", want: []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parsePath(tt.input)
			if len(got) != len(tt.want) {
				t.Fatalf("parsePath(%q) returned %d elements, want %d", tt.input, len(got), len(tt.want))
			}
			for i, w := range tt.want {
				if got[i] != w {
					t.Errorf("parsePath(%q)[%d] = %q, want %q", tt.input, i, got[i], w)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// NormalizePath
// ---------------------------------------------------------------------------

func TestNormalizePath_Valid(t *testing.T) {
	p := &PatchOp{Op: "replace", Path: "spec.containers[name=main].image"}
	if err := p.NormalizePath(); err != nil {
		t.Fatalf("NormalizePath: %v", err)
	}
	if len(p.ParsedPath) == 0 {
		t.Fatalf("expected ParsedPath to be populated")
	}
	// Verify basic structure: spec, containers[name=main], image
	if p.ParsedPath[0].Field != "spec" {
		t.Errorf("part 0: expected field 'spec', got %q", p.ParsedPath[0].Field)
	}
	if p.ParsedPath[1].Field != "containers" || p.ParsedPath[1].MatchType != "key" {
		t.Errorf("part 1: unexpected %+v", p.ParsedPath[1])
	}
	if p.ParsedPath[2].Field != "image" {
		t.Errorf("part 2: expected field 'image', got %q", p.ParsedPath[2].Field)
	}
}

func TestNormalizePath_Invalid(t *testing.T) {
	p := &PatchOp{Op: "replace", Path: ""}
	if err := p.NormalizePath(); err == nil {
		t.Fatalf("expected error for empty path")
	}
}
