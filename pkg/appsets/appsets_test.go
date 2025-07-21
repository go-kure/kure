package appsets

import "testing"

func TestParsePatchLine(t *testing.T) {
	op, err := ParsePatchLine("spec/template/spec/containers[+=name=main]", map[string]interface{}{"image": "nginx"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if op.Op != "insertAfter" || op.Selector != "name=main" {
		t.Fatalf("unexpected op %+v", op)
	}
	if op.Path != "spec/template/spec/containers" {
		t.Fatalf("unexpected path %s", op.Path)
	}
}
