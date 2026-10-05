package layout_test

import (
	"slices"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// nodeFieldsCluster is r -> {platform, apps}, both without a bundle, each
// with one child node that has one. apps names its Kustomization and lists
// one dependency by name and one as a node.
func nodeFieldsCluster() (c *stack.Cluster, platform, apps *stack.Node) {
	platform = &stack.Node{Name: "platform", Children: []*stack.Node{
		{Name: "cert-manager", Bundle: &stack.Bundle{Name: "cert-manager", Applications: []*stack.Application{axisApp("cm")}}},
	}}
	apps = &stack.Node{
		Name:              "apps",
		KustomizationName: "apps-cr",
		DependsOn:         []*stack.Node{platform},
		NamedDependsOn:    []string{"external"},
		Children: []*stack.Node{
			{Name: "shop", Bundle: &stack.Bundle{Name: "shop", Applications: []*stack.Application{axisApp("web")}}},
		},
	}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "r", Children: []*stack.Node{platform, apps}}}, platform, apps
}

// TestWalkCluster_NodeKustomizationFields: the walker copies a node's
// KustomizationName and NamedDependsOn onto the node's own layout, where the
// layout integrator reads them; a node that sets neither leaves both empty.
// DependsOn holds nodes and stays on the node. The layout's list is its own.
func TestWalkCluster_NodeKustomizationFields(t *testing.T) {
	c, platform, apps := nodeFieldsCluster()
	ml, err := layout.WalkCluster(c, layout.LayoutRules{})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}

	l := ix.NodeLayout(apps)
	if l.KustomizationName != "apps-cr" {
		t.Errorf("layout of node apps has KustomizationName %q, want apps-cr", l.KustomizationName)
	}
	if !slices.Equal(l.DependsOn, []string{"external"}) {
		t.Errorf("layout of node apps has DependsOn %v, want [external], the node's NamedDependsOn", l.DependsOn)
	}
	l.DependsOn[0] = "changed"
	if apps.NamedDependsOn[0] != "external" {
		t.Error("changing the layout's DependsOn changed the node's NamedDependsOn")
	}

	if p := ix.NodeLayout(platform); p.KustomizationName != "" || len(p.DependsOn) != 0 {
		t.Errorf("layout of node platform has KustomizationName %q and DependsOn %v, want neither", p.KustomizationName, p.DependsOn)
	}
}

// TestWalkCluster_MergedNodeFieldsNotCopied: under NodeGrouping GroupFlat a
// child node is rendered into its parent's directory. That layout is the
// parent's own, so it carries nothing of the merged node: the fields are not
// taken over as the parent's.
func TestWalkCluster_MergedNodeFieldsNotCopied(t *testing.T) {
	c, _, apps := nodeFieldsCluster()
	ml, err := layout.WalkCluster(c, layout.LayoutRules{NodeGrouping: layout.GroupFlat})
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}
	l := ix.NodeLayout(apps)
	if l != ix.NodeLayout(c.Node) {
		t.Fatalf("node apps is rendered by %q, want the root node's layout", l.FullRepoPath())
	}
	if l.KustomizationName != "" || len(l.DependsOn) != 0 {
		t.Errorf("the root node's layout has KustomizationName %q and DependsOn %v, want neither: they are node apps's", l.KustomizationName, l.DependsOn)
	}
}
