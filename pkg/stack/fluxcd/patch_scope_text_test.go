package fluxcd_test

import (
	"maps"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// writtenMetaCMApp is an application rendering one ConfigMap whose labels and
// annotations are both meta, as written: decoded from YAML they may hold a
// boolean, a number or null as well as a string.
func writtenMetaCMApp(name string, meta map[string]any) *stack.Application {
	obj := cmObj(name + "-cm")
	metadata := (*obj).(*unstructured.Unstructured).Object["metadata"].(map[string]any)
	metadata["labels"] = maps.Clone(meta)
	metadata["annotations"] = maps.Clone(meta)
	return stack.NewApplication(name, "default", &fakeAppConfig{objs: []*client.Object{obj}})
}

// TestGenerateFromLayout_PatchScopeReadsMetadataAsKustomizeDoes pins
// go-kure/kure#1100: a patch target's label and annotation selectors are
// matched against an object's labels and annotations as kustomize reads them
// from what is written, every value as its text, so that a map holding a
// boolean or a number is not read as no map at all. The objects of rb and b2
// carry team "web", flag true and replicas 3 in both maps; b1's own object
// carries neither, so a selector that reaches past b1 is refused and one that
// does not is accepted.
func TestGenerateFromLayout_PatchScopeReadsMetadataAsKustomizeDoes(t *testing.T) {
	meta := map[string]any{"team": "web", "flag": true, "replicas": int64(3)}
	tests := map[string]struct {
		target  stack.PatchSelector
		refused bool
	}{
		"label, a string":               {stack.PatchSelector{LabelSelector: "team=web"}, true},
		"label, a boolean as its text":  {stack.PatchSelector{LabelSelector: "flag=true"}, true},
		"label, a number as its text":   {stack.PatchSelector{LabelSelector: "replicas=3"}, true},
		"label, without the key":        {stack.PatchSelector{LabelSelector: "!flag"}, false},
		"label, another value":          {stack.PatchSelector{LabelSelector: "team!=web"}, false},
		"label, not in the values":      {stack.PatchSelector{LabelSelector: "flag notin (true)"}, false},
		"annotation, a string":          {stack.PatchSelector{AnnotationSelector: "team=web"}, true},
		"annotation, a boolean as text": {stack.PatchSelector{AnnotationSelector: "flag=true"}, true},
		"annotation, a number as text":  {stack.PatchSelector{AnnotationSelector: "replicas=3"}, true},
		"annotation, without the key":   {stack.PatchSelector{AnnotationSelector: "!flag"}, false},
		"annotation, another value":     {stack.PatchSelector{AnnotationSelector: "team!=web"}, false},
		"annotation, not in the values": {stack.PatchSelector{AnnotationSelector: "flag notin (true)"}, false},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			c := mergedCluster(func(rb, b1, b2 *stack.Bundle) {
				rb.Applications = []*stack.Application{writtenMetaCMApp("core", meta)}
				b2.Applications = []*stack.Application{writtenMetaCMApp("two", meta)}
				target := tt.target
				b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &target}}
			})
			_, err := generateUnits(t, c, allFlat)
			if !tt.refused {
				if err != nil {
					t.Fatalf("GenerateFromLayout refused a patch that selects only b1's own object: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatal("GenerateFromLayout accepted a patch that reaches another bundle's object")
			}
			if !strings.Contains(err.Error(), "patch target") {
				t.Errorf("error %q is not the patch scope refusal", err)
			}
		})
	}
}
