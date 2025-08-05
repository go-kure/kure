package layout_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// buildTestCluster constructs a simple cluster with one node, bundle and application.
func buildTestCluster() *stack.Cluster {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &fakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	node := &stack.Node{Name: "apps", Bundle: bundle}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

func TestWalkClusterGroupingCombinations(t *testing.T) {
	type testCase struct {
		name     string
		rules    layout.LayoutRules
		nodeOnly bool
	}

	modes := []layout.GroupingMode{layout.GroupByName, layout.GroupFlat}
	var tests []testCase
	for _, ng := range modes {
		for _, bg := range modes {
			for _, ag := range modes {
				tc := testCase{
					name:     string(ng) + "_" + string(bg) + "_" + string(ag),
					rules:    layout.LayoutRules{NodeGrouping: ng, BundleGrouping: bg, ApplicationGrouping: ag},
					nodeOnly: bg == layout.GroupFlat && ag == layout.GroupFlat,
				}
				tests = append(tests, tc)
			}
		}
	}

	cluster := buildTestCluster()

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ml, err := layout.WalkCluster(cluster, tc.rules)
			if err != nil {
				t.Fatalf("walk cluster: %v", err)
			}
			if len(ml.Children) != 1 {
				t.Fatalf("expected one child, got %d", len(ml.Children))
			}
			nodeLayout := ml.Children[0]
			if tc.nodeOnly {
				if len(nodeLayout.Resources) != 1 {
					t.Fatalf("expected resources at node, got %d", len(nodeLayout.Resources))
				}
				if len(nodeLayout.Children) != 0 {
					t.Fatalf("unexpected children: %d", len(nodeLayout.Children))
				}
			} else {
				if len(nodeLayout.Resources) != 0 {
					t.Fatalf("unexpected node resources: %d", len(nodeLayout.Resources))
				}
				if len(nodeLayout.Children) != 1 {
					t.Fatalf("expected bundle child, got %d", len(nodeLayout.Children))
				}
				bundleLayout := nodeLayout.Children[0]
				if len(bundleLayout.Children) != 1 {
					t.Fatalf("expected application child, got %d", len(bundleLayout.Children))
				}
				appLayout := bundleLayout.Children[0]
				if len(appLayout.Resources) != 1 {
					t.Fatalf("expected application resources, got %d", len(appLayout.Resources))
				}
			}
		})
	}
}
