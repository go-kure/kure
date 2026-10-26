package fluxcd

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	kyaml "sigs.k8s.io/kustomize/kyaml/yaml"

	kio "github.com/go-kure/kure/pkg/io"
	"github.com/go-kure/kure/pkg/stack"
)

// TestPatchTargetMatcher_SelectsAsKustomizeDoes pins go-kure/kure#1100 on
// patchTargetMatcher, which both of its callers use (checkPatchScope and the
// hosted-Source check): on an object whose labels and annotations hold a
// boolean, a number and null as well as a string, every selector selects the
// object exactly when kustomize selects it from what the writers write it as.
// That includes a key the writers drop from the annotations and keep in the
// labels.
func TestPatchTargetMatcher_SelectsAsKustomizeDoes(t *testing.T) {
	const lastApplied = "kubectl.kubernetes.io/last-applied-configuration"
	meta := map[string]any{"team": "web", "flag": true, "replicas": int64(3), "ratio": 1.5, "none": nil, lastApplied: true}
	var obj client.Object = &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]any{
			"name": "app", "namespace": "default",
			"labels": meta, "annotations": meta,
		},
	}}
	written, err := kio.EncodeObjectsToYAML([]*client.Object{&obj})
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	node, err := kyaml.Parse(string(written))
	if err != nil {
		t.Fatalf("read the written object: %v", err)
	}
	type expect struct{ label, annotation bool }
	selectors := map[string]expect{
		"team=web":          {true, true},
		"flag=true":         {true, true},
		"replicas=3":        {true, true},
		"ratio=1.5":         {true, true},
		"none=null":         {true, true},
		"flag":              {true, true},
		"team in (web,api)": {true, true},
		"!flag":             {false, false},
		"team!=web":         {false, false},
		"flag notin (true)": {false, false},
		"replicas=4":        {false, false},
		lastApplied:         {true, false},
		"!" + lastApplied:   {false, true},
	}
	for selector, wants := range selectors {
		for _, field := range []string{"label", "annotation"} {
			want := wants.label
			if field == "annotation" {
				want = wants.annotation
			}
			t.Run(field+" "+selector, func(t *testing.T) {
				target := &stack.PatchSelector{Kind: "ConfigMap"}
				kustomizeSelects, err := node.MatchesLabelSelector(selector)
				if field == "label" {
					target.LabelSelector = selector
				} else {
					target.AnnotationSelector = selector
					kustomizeSelects, err = node.MatchesAnnotationSelector(selector)
				}
				if err != nil {
					t.Fatalf("kustomize: %v", err)
				}
				if kustomizeSelects != want {
					t.Fatalf("kustomize selects the object: %v; the case expects %v", kustomizeSelects, want)
				}
				selects, err := patchTargetMatcher(target)
				if err != nil {
					t.Fatalf("patchTargetMatcher: %v", err)
				}
				got, err := selects(obj, true)
				if err != nil {
					t.Fatalf("selects: %v", err)
				}
				if got != want {
					t.Errorf("patchTargetMatcher selects the object: %v; kustomize: %v", got, want)
				}
			})
		}
	}
}
