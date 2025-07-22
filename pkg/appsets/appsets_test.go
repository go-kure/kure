package appsets

import (
	"io"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/yaml"
)

func TestParsePatchLine(t *testing.T) {
	op, err := ParsePatchLine("spec.template.spec.containers[+=name=main]", map[string]interface{}{"image": "nginx"})
	if err != nil {
		t.Fatalf("parse error: %v", err)
	}
	if op.Op != "insertAfter" || op.Selector != "name=main" {
		t.Fatalf("unexpected op %+v", op)
	}
	if op.Path != "spec.template.spec.containers" {
		t.Fatalf("unexpected path %s", op.Path)
	}
}

func TestParsePatchPath(t *testing.T) {
	cases := []struct {
		in   string
		want []PathPart
	}{
		{
			in: "spec.template.spec.containers[0].image",
			want: []PathPart{
				{Field: "spec"},
				{Field: "template"},
				{Field: "spec"},
				{Field: "containers", MatchType: "index", MatchValue: "0"},
				{Field: "image"},
			},
		},
		{
			in: "spec.containers[name=main].image",
			want: []PathPart{
				{Field: "spec"},
				{Field: "containers", MatchType: "key", MatchValue: "name=main"},
				{Field: "image"},
			},
		},
	}

	for _, tc := range cases {
		got, err := ParsePatchPath(tc.in)
		if err != nil {
			t.Fatalf("ParsePatchPath error: %v", err)
		}
		if len(got) != len(tc.want) {
			t.Fatalf("segments len mismatch for %s: got %d want %d", tc.in, len(got), len(tc.want))
		}
		for i, p := range got {
			if p != tc.want[i] {
				t.Fatalf("segment %d mismatch for %s: got %+v want %+v", i, tc.in, p, tc.want[i])
			}
		}
	}
}

func TestDeleteOperation(t *testing.T) {
	yamlStr := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
  labels:
    app: demo
`
	var m map[string]interface{}
	if err := yaml.Unmarshal([]byte(yamlStr), &m); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	obj := &unstructured.Unstructured{Object: m}
	op, err := ParsePatchLine("metadata.labels.app[delete]", "")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if err := applyPatchOp(obj.Object, op); err != nil {
		t.Fatalf("apply: %v", err)
	}
	labels, _, _ := unstructured.NestedStringMap(obj.Object, "metadata", "labels")
	if _, ok := labels["app"]; ok {
		t.Fatalf("label not deleted")
	}
}

func TestExplicitTargetAndSmart(t *testing.T) {
	resYaml := `
apiVersion: v1
kind: ConfigMap
metadata:
  name: demo
data:
  foo: bar
`
	var rm map[string]interface{}
	if err := yaml.Unmarshal([]byte(resYaml), &rm); err != nil {
		t.Fatalf("yaml decode: %v", err)
	}
	patchYaml := "- target: demo\n  patch:\n    data.foo: baz\n"
	patches, err := LoadPatchFile(strings.NewReader(patchYaml))
	if err != nil {
		t.Fatalf("load patch: %v", err)
	}
	if len(patches) != 1 {
		t.Fatalf("expected 1 patch")
	}
	set, err := LoadPatchableAppSet([]io.Reader{strings.NewReader(resYaml)}, strings.NewReader(patchYaml))
	if err != nil {
		t.Fatalf("LoadPatchableAppSet: %v", err)
	}
	resolved, err := set.Resolve()
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if len(resolved) != 1 || resolved[0].Name != "demo" {
		t.Fatalf("unexpected resolve result")
	}
	if err := resolved[0].Apply(); err != nil {
		t.Fatalf("apply: %v", err)
	}
	val, _, _ := unstructured.NestedString(resolved[0].Base.Object, "data", "foo")
	if val != "baz" {
		t.Fatalf("patch not applied")
	}
}
