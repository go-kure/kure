package layout_test

import (
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// holding returns the layout, in l or below it, whose origin lists every
// bundle in want.
func holding(l *layout.ManifestLayout, want ...*stack.Bundle) *layout.ManifestLayout {
	if l == nil {
		return nil
	}
	have := map[*stack.Bundle]bool{}
	for _, b := range l.OriginBundles() {
		have[b] = true
	}
	all := true
	for _, b := range want {
		all = all && have[b]
	}
	if all {
		return l
	}
	for _, c := range l.Children {
		if found := holding(c, want...); found != nil {
			return found
		}
	}
	return holding(l.OriginUnit(), want...)
}

// standInNamed returns the object of objs of kind ConfigMap and the given name.
func standInNamed(objs []client.Object, name string) client.Object {
	for _, o := range objs {
		if o.GetName() == name && o.GetObjectKind().GroupVersionKind().Kind == "ConfigMap" {
			return o
		}
	}
	return nil
}

// TestOriginBundleObjects_SyncsOnlyTheRequestedBundlesStandIns: a layout that
// several bundles are merged into keeps one record of their objects. A call
// for one bundle brings the stand-ins of that bundle's applications up to
// date with the annotations their generator entries set now, and leaves the
// other bundles' stand-ins as they were until a call for those bundles
// (go-kure/kure#1297). Before, every call compared every bundle's generators.
func TestOriginBundleObjects_SyncsOnlyTheRequestedBundlesStandIns(t *testing.T) {
	bundle := func(name, generated string) *stack.Bundle {
		o := testObj("v1", "ConfigMap", name)
		app := stack.NewApplication(name, "ns", &fakeAugmentingConfig{objs: []*client.Object{&o}, cmgName: generated})
		return &stack.Bundle{Name: name, Applications: []*stack.Application{app}}
	}
	rb, b1, b2 := bundle("root", "root-values"), bundle("one", "one-values"), bundle("two", "two-values")
	c1 := &stack.Node{Name: "c1", Bundle: b1}
	c2 := &stack.Node{Name: "c2", Bundle: b2}
	root := &stack.Node{Name: "root", Bundle: rb, Children: []*stack.Node{c1, c2}}
	c1.SetParent(root)
	c2.SetParent(root)

	ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, layout.LayoutRules{
		NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	merged := holding(ml, b1, b2)
	if merged == nil {
		t.Fatal("no layout holds both bundles: the case is not a merged layout")
	}
	s1 := standInNamed(merged.OriginBundleObjects(b1), "one-values")
	s2 := standInNamed(merged.OriginBundleObjects(b2), "two-values")
	if s1 == nil || s2 == nil {
		t.Fatalf("stand-ins one-values = %v, two-values = %v; want both in their bundles' records", s1, s2)
	}

	// An entry gains annotations after the walk, as a delivery intent applied by a workflow does.
	gained := 0
	for _, rec := range merged.OriginApplicationObjects() {
		if rec.Layout == nil {
			continue
		}
		for i := range rec.Layout.ConfigMapGenerators {
			rec.Layout.ConfigMapGenerators[i].Annotations = map[string]string{"sync": "yes"}
			gained++
		}
	}
	if gained < 2 {
		t.Fatalf("only %d generator entries to annotate; want one per application", gained)
	}

	merged.OriginBundleObjects(b1)
	if got := s1.GetAnnotations()["sync"]; got != "yes" {
		t.Errorf("after a call for b1, its stand-in has sync = %q; want yes", got)
	}
	if got := s2.GetAnnotations()["sync"]; got != "" {
		t.Errorf("after a call for b1, b2's stand-in has sync = %q; want it left as it was", got)
	}
	merged.OriginBundleObjects(b2)
	if got := s2.GetAnnotations()["sync"]; got != "yes" {
		t.Errorf("after a call for b2, its stand-in has sync = %q; want yes", got)
	}
}

// mergedGeneratorLayout walks a root with two child bundles merged into one
// layout and returns that layout and both bundles. A bundle's application
// generates a ConfigMap named after the bundle with "-values".
func mergedGeneratorLayout(t *testing.T, apps func(name string) []*stack.Application) (*layout.ManifestLayout, *stack.Bundle, *stack.Bundle) {
	t.Helper()
	bundle := func(name string) *stack.Bundle {
		return &stack.Bundle{Name: name, Applications: apps(name)}
	}
	rb, b1, b2 := bundle("root"), bundle("one"), bundle("two")
	c1 := &stack.Node{Name: "c1", Bundle: b1}
	c2 := &stack.Node{Name: "c2", Bundle: b2}
	root := &stack.Node{Name: "root", Bundle: rb, Children: []*stack.Node{c1, c2}}
	c1.SetParent(root)
	c2.SetParent(root)
	ml, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, layout.LayoutRules{
		NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("walk: %v", err)
	}
	merged := holding(ml, b1, b2)
	if merged == nil {
		t.Fatal("no layout holds both bundles: the case is not a merged layout")
	}
	return merged, b1, b2
}

// annotateGenerators sets sync=yes on every generator entry the layout's
// application records hold, as a delivery intent applied after the walk does.
func annotateGenerators(t *testing.T, l *layout.ManifestLayout) {
	t.Helper()
	gained := 0
	for _, rec := range l.OriginApplicationObjects() {
		if rec.Layout == nil {
			continue
		}
		for i := range rec.Layout.ConfigMapGenerators {
			rec.Layout.ConfigMapGenerators[i].Annotations = map[string]string{"sync": "yes"}
			gained++
		}
	}
	if gained < 2 {
		t.Fatalf("only %d generator entries to annotate; want one per application", gained)
	}
}

// TestOriginBundleObjects_SyncFollowsTheBundleThatEmittedTheRecord: the
// stand-ins a bundle's call refreshes are the ones the bundle emitted when it
// was walked, not those of the applications it lists at the time of the call
// (go-kure/kure#1297). A bundle whose Applications changed after the walk
// still has its stand-ins refreshed, so a patch that reaches one of its
// generated ConfigMaps by annotation is still seen.
func TestOriginBundleObjects_SyncFollowsTheBundleThatEmittedTheRecord(t *testing.T) {
	merged, b1, b2 := mergedGeneratorLayout(t, func(name string) []*stack.Application {
		o := testObj("v1", "ConfigMap", name)
		return []*stack.Application{stack.NewApplication(name, "ns", &fakeAugmentingConfig{objs: []*client.Object{&o}, cmgName: name + "-values"})}
	})
	s1 := standInNamed(merged.OriginBundleObjects(b1), "one-values")
	s2 := standInNamed(merged.OriginBundleObjects(b2), "two-values")
	if s1 == nil || s2 == nil {
		t.Fatalf("stand-ins one-values = %v, two-values = %v; want both in their bundles' records", s1, s2)
	}
	annotateGenerators(t, merged)

	b2.Applications = nil
	merged.OriginBundleObjects(b2)
	if got := s2.GetAnnotations()["sync"]; got != "yes" {
		t.Errorf("after a call for b2, whose Applications is now empty, its stand-in has sync = %q; want yes", got)
	}
	if got := s1.GetAnnotations()["sync"]; got != "" {
		t.Errorf("after a call for b2, b1's stand-in has sync = %q; want it left as it was", got)
	}
}

// TestOriginBundleObjects_BundlesSharingAnApplicationKeepApart: two bundles of
// one layout that list the same *Application each have their own record of it
// and their own stand-ins. A call for one refreshes its stand-ins only.
func TestOriginBundleObjects_BundlesSharingAnApplicationKeepApart(t *testing.T) {
	o := testObj("v1", "ConfigMap", "shared")
	shared := stack.NewApplication("shared", "ns", &fakeAugmentingConfig{objs: []*client.Object{&o}, cmgName: "shared-values"})
	merged, b1, b2 := mergedGeneratorLayout(t, func(string) []*stack.Application { return []*stack.Application{shared} })
	s1 := standInNamed(merged.OriginBundleObjects(b1), "shared-values")
	s2 := standInNamed(merged.OriginBundleObjects(b2), "shared-values")
	if s1 == nil || s2 == nil || s1 == s2 {
		t.Fatalf("stand-ins b1 = %v, b2 = %v; want one distinct stand-in in each bundle's record", s1, s2)
	}
	annotateGenerators(t, merged)

	merged.OriginBundleObjects(b1)
	if got := s1.GetAnnotations()["sync"]; got != "yes" {
		t.Errorf("after a call for b1, its stand-in has sync = %q; want yes", got)
	}
	if got := s2.GetAnnotations()["sync"]; got != "" {
		t.Errorf("after a call for b1, b2's stand-in has sync = %q; want it left as it was", got)
	}
}
