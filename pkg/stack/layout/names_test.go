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
