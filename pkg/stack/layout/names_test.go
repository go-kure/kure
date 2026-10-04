package layout_test

import (
	"strings"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestWalkCluster_ApplicationNames: an application name is checked as a
// directory name only where the application gets a directory of its own —
// under ApplicationGrouping by name, or as an augmenter that takes its own
// layout under either grouping. Written flat into its bundle's directory, the
// name reaches no path and is not checked.
func TestWalkCluster_ApplicationNames(t *testing.T) {
	plain := func() stack.ApplicationConfig {
		return &fakeConfig{objs: []*client.Object{makeCM("cm")}}
	}
	augmenter := func() stack.ApplicationConfig {
		return &fakeAugmentingConfig{objs: []*client.Object{makeCM("cm")}}
	}
	flatAugmenter := func() stack.ApplicationConfig {
		return &fakeIntentAugmentingConfig{objs: []*client.Object{makeCM("cm")}, wantsOwn: false}
	}
	byName := layout.LayoutRules{ApplicationGrouping: layout.GroupByName}
	flat := layout.DefaultLayoutRules()

	tests := []struct {
		name    string
		appName string
		cfg     func() stack.ApplicationConfig
		rules   layout.LayoutRules
		wantErr []string // every substring must be in the error; nil means accepted
	}{
		{"flat, slash", "a/b", plain, flat, nil},
		{"flat, empty", "", plain, flat, nil},
		{"flat, augmenter without its own layout", "a/b", flatAugmenter, flat, nil},
		{"by name, mixed case", "My_App", plain, byName, nil},
		{"by name, dotted", "my.app", plain, byName, nil},
		{"by name, slash", "a/b", plain, byName, []string{`'a/b'`, `"root/apps"`, "path separator"}},
		{"by name, backslash", `..\outside`, plain, byName, []string{`"root/apps"`, "path separator"}},
		{"by name, parent directory", "..", plain, byName, []string{`"root/apps"`, `".."`}},
		{"by name, empty", "", plain, byName, []string{`"root/apps"`, "empty"}},
		{"flat, augmenter with its own layout", "a/b", augmenter, flat, []string{`'a/b'`, `"root/apps"`, "path separator"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := stack.NewApplication(tt.appName, "ns", tt.cfg())
			node := &stack.Node{Name: "apps", Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{app}}}
			root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
			node.SetParent(root)

			_, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, tt.rules)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("WalkCluster() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("WalkCluster() = nil, want an error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("WalkCluster() = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}

// TestWalkCluster_ApplicationNameAfterFlatten: an application's name is
// checked on the tree WalkCluster returns, after FlattenSingleTier has run, so
// it is refused exactly when the application has a directory of its own in
// that tree. The test does not assume which layouts the flatten removes: for
// each shape it walks once with a valid name, looks whether the application
// kept a directory, and expects a name holding a separator to be refused in
// that case and in no other.
func TestWalkCluster_ApplicationNameAfterFlatten(t *testing.T) {
	cluster := func(names ...string) (*stack.Cluster, *stack.Application) {
		var apps []*stack.Application
		for i, n := range names {
			cm := makeCM("cm-" + string(rune('a'+i)))
			apps = append(apps, stack.NewApplication(n, "ns", &fakeConfig{objs: []*client.Object{cm}}))
		}
		root := &stack.Node{Name: "root", Bundle: &stack.Bundle{Name: "web", Applications: apps}}
		return &stack.Cluster{Name: "demo", Node: root}, apps[0]
	}
	var ownDirectory func(ml *layout.ManifestLayout, app *stack.Application) bool
	ownDirectory = func(ml *layout.ManifestLayout, app *stack.Application) bool {
		if ml == nil {
			return false
		}
		if ml.OriginApplication() == app && ml.Name == app.Name {
			return true
		}
		for _, child := range ml.Children {
			if ownDirectory(child, app) {
				return true
			}
		}
		return false
	}

	byName := layout.LayoutRules{ApplicationGrouping: layout.GroupByName}
	flatten := byName
	flatten.FlattenSingleTier = true
	flattenInCluster := flatten
	flattenInCluster.ClusterName = "prod"
	shapes := []struct {
		name  string
		rules layout.LayoutRules
		rest  []string // the applications beside the one under test
	}{
		{"one application", byName, nil},
		{"one application, flatten", flatten, nil},
		{"one application, flatten, cluster name", flattenInCluster, nil},
		{"two applications, flatten", flatten, []string{"other"}},
	}
	refused := 0
	for _, s := range shapes {
		t.Run(s.name, func(t *testing.T) {
			c, app := cluster(append([]string{"app"}, s.rest...)...)
			ml, err := layout.WalkCluster(c, s.rules)
			if err != nil {
				t.Fatalf("WalkCluster with a valid name: %v", err)
			}
			keepsDirectory := ownDirectory(ml, app)

			c, _ = cluster(append([]string{"a/b"}, s.rest...)...)
			_, err = layout.WalkCluster(c, s.rules)
			switch {
			case keepsDirectory && err == nil:
				t.Fatal("WalkCluster accepted a/b as the name of a directory")
			case keepsDirectory:
				refused++
				for _, want := range []string{`'a/b'`, "path separator"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("WalkCluster() = %v, want it to contain %q", err, want)
					}
				}
			case err != nil:
				t.Fatalf("WalkCluster() = %v, want nil: the application has no directory of its own in the returned tree (%v)", err, collectRepoPaths(ml))
			}
		})
	}
	if refused == 0 {
		t.Error("no shape kept the application's directory: the refusal was not exercised")
	}
}

// TestWalkCluster_NodeNamesUnderFlatGrouping: with NodeGrouping flat a child
// node is absorbed into its parent's directory, so its name becomes no
// directory. It is refused all the same, by the cluster validation the walk
// runs first: a node name is a segment of the node's path in the model, where
// an empty name or one holding "/" gives two nodes one path key.
func TestWalkCluster_NodeNamesUnderFlatGrouping(t *testing.T) {
	rules := layout.DefaultLayoutRules()
	rules.NodeGrouping = layout.GroupFlat

	tests := []struct {
		name      string
		childName string
		wantErr   []string // every substring must be in the error; nil means accepted
	}{
		{"named child", "apps", nil},
		{"unnamed child", "", []string{`node "root/"`, "empty"}},
		{"child with a slash", "a/b", []string{`node "root/a/b"`, "path separator"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := stack.NewApplication("web", "ns", &fakeConfig{objs: []*client.Object{makeCM("cm")}})
			node := &stack.Node{Name: tt.childName, Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{app}}}
			root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
			node.SetParent(root)

			_, err := layout.WalkCluster(&stack.Cluster{Name: "demo", Node: root}, rules)
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("WalkCluster() = %v, want nil", err)
				}
				return
			}
			if err == nil {
				t.Fatal("WalkCluster() = nil, want an error")
			}
			for _, want := range tt.wantErr {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("WalkCluster() = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}
