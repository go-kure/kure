package fluxcd

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// A caller may build an unstructured object, a Source say, with a plain int:
// the copies kure reads and compares hold it as JSON decodes it, an int64,
// and never panic (go-kure/kure#1199).
func TestCallerInteger_IsCopiedAsJSON(t *testing.T) {
	obj := func(n any) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "source.toolkit.fluxcd.io/v1", "kind": "GitRepository",
			"metadata": map[string]any{"name": "repo", "namespace": "flux-system"},
			"spec":     map[string]any{"extra": n},
		}}
	}
	for name, read := range map[string]func(*unstructured.Unstructured) (map[string]any, error){
		"writtenContent":    func(u *unstructured.Unstructured) (map[string]any, error) { return writtenContent(u) },
		"comparableContent": func(u *unstructured.Unstructured) (map[string]any, error) { return comparableContent(u) },
	} {
		t.Run(name, func(t *testing.T) {
			got, err := read(obj(53))
			if err != nil {
				t.Fatal(err)
			}
			if v := got["spec"].(map[string]any)["extra"]; v != int64(53) {
				t.Errorf("spec.extra = %#v, want int64(53)", v)
			}
		})
	}
	if !sameObject(obj(53), obj(int64(53))) {
		t.Error("an object holding int 53 and one holding int64 53 compare different")
	}
	if reflect.DeepEqual(obj(53).Object, obj(int64(53)).Object) {
		t.Fatal("the test's two objects must differ in Go type")
	}
}
