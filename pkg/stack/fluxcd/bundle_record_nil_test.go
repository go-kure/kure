package fluxcd_test

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// emittedRecord returns the live record of the objects bundle b emitted, from
// the layout of the tree that holds it. OriginBundleObjects returns the slice
// the layout keeps, so a write to an element changes the record itself.
func emittedRecord(l *layout.ManifestLayout, b *stack.Bundle) []client.Object {
	if l == nil {
		return nil
	}
	if rec := l.OriginBundleObjects(b); len(rec) > 0 {
		return rec
	}
	for _, child := range l.Children {
		if rec := emittedRecord(child, b); len(rec) > 0 {
			return rec
		}
	}
	return emittedRecord(l.OriginUnit(), b)
}

// TestGenerateFromLayout_RefusesATypedNilInABundlesEmittedRecord: a caller
// that leaves a typed nil in the record of a bundle's emitted objects gets an
// error naming the bundle, not a panic. A patch of another bundle that has a
// target reads that record to check its scope, and called a method on the
// nil (go-kure/kure#1297).
func TestGenerateFromLayout_RefusesATypedNilInABundlesEmittedRecord(t *testing.T) {
	var b2 *stack.Bundle
	c := mergedCluster(func(_, b1, b *stack.Bundle) {
		b2 = b
		b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "one"}}}
	})
	ml, err := layout.WalkCluster(c, allFlat)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	rec := emittedRecord(ml, b2)
	if len(rec) == 0 {
		t.Fatal("bundle b2 emitted nothing: the case has no record to leave a nil in")
	}
	rec[0] = (*unstructured.Unstructured)(nil)

	err = callEntry(func() error {
		_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
		return err
	})
	if err == nil || strings.HasPrefix(err.Error(), "PANIC:") {
		t.Fatalf("GenerateFromLayout: err = %v; want a refusal, not a panic", err)
	}
	for _, want := range []string{"a nil", b2.Name} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("GenerateFromLayout: err = %q; want it to contain %q", err, want)
		}
	}
}

// annotateGeneratorNamed sets the annotations of every configMapGenerator
// entry called name in l or below it, as a delivery intent applied after the
// walk does.
func annotateGeneratorNamed(l *layout.ManifestLayout, name string, annotations map[string]string) int {
	if l == nil {
		return 0
	}
	n := 0
	for i := range l.ConfigMapGenerators {
		if l.ConfigMapGenerators[i].Name == name {
			l.ConfigMapGenerators[i].Annotations = annotations
			n++
		}
	}
	for _, child := range l.Children {
		n += annotateGeneratorNamed(child, name, annotations)
	}
	return n
}

// TestGenerateFromLayout_PatchScopeFollowsTheBundleThatEmittedTheRecord: a
// patch that reaches a ConfigMap another bundle's generator builds is refused
// whatever b2.Applications lists when the check runs. The stand-in of that
// ConfigMap takes its annotations from the walk's record of the bundle that
// emitted it (go-kure/kure#1297).
func TestGenerateFromLayout_PatchScopeFollowsTheBundleThatEmittedTheRecord(t *testing.T) {
	var b2 *stack.Bundle
	c := mergedCluster(func(_, b1, b *stack.Bundle) {
		b2 = b
		b2.Applications = append(b2.Applications, stack.NewApplication("hook", "default", &hookAugmenter{app: "hook"}))
		b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &stack.PatchSelector{Name: "hook-values", AnnotationSelector: "scope=other"}}}
	})
	ml, err := layout.WalkCluster(c, allFlat)
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	// The entry gains its annotation after the walk, and the bundle's list changes.
	if n := annotateGeneratorNamed(ml, "hook-values", map[string]string{"scope": "other"}); n != 1 {
		t.Fatalf("annotated %d generator entries named hook-values; want 1", n)
	}
	b2.Applications = nil

	_, err = fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	if err == nil {
		t.Fatal("GenerateFromLayout accepted a patch that reaches another bundle's generated ConfigMap")
	}
	if !errors.IsType(err, errors.ErrorTypeResource) {
		t.Errorf("GenerateFromLayout: err = %v (%T); want a resource validation error", err, err)
	}
	for _, want := range []string{`"b1"`, `"b2"`, `ConfigMap "hook-values"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("GenerateFromLayout: err = %q; want it to contain %q", err, want)
		}
	}
}
