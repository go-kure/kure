package stack

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestValidateCluster_Nil(t *testing.T) {
	if err := ValidateCluster(nil); err != nil {
		t.Errorf("nil cluster should pass: %v", err)
	}
	if err := ValidateCluster(&Cluster{Name: "c"}); err != nil {
		t.Errorf("cluster with nil Node should pass: %v", err)
	}
}

func TestValidateCluster_HappyUmbrella(t *testing.T) {
	child := &Bundle{Name: "child"}
	root := &Bundle{Name: "root", Children: []*Bundle{child}}
	c := &Cluster{Name: "c", Node: &Node{Name: "n", Bundle: root}}
	if err := ValidateCluster(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCluster_ChildAlsoNodeRejected(t *testing.T) {
	sharedBundle := &Bundle{Name: "shared"}
	// Build a cluster where "shared" is both the child node's Bundle and an
	// umbrella child of the root node's bundle.
	rootBundle := &Bundle{Name: "root", Children: []*Bundle{sharedBundle}}
	root := &Node{
		Name:   "root-node",
		Bundle: rootBundle,
		Children: []*Node{
			{Name: "child-node", Bundle: sharedBundle},
		},
	}
	c := &Cluster{Name: "c", Node: root}
	if err := ValidateCluster(c); err == nil {
		t.Fatal("expected overlap between node bundle and umbrella child to fail")
	}
}

func TestValidateCluster_SharedChildBetweenUmbrellasRejected(t *testing.T) {
	shared := &Bundle{Name: "shared"}
	u1 := &Bundle{Name: "u1", Children: []*Bundle{shared}}
	u2 := &Bundle{Name: "u2", Children: []*Bundle{shared}}
	root := &Node{
		Name:   "root",
		Bundle: u1,
		Children: []*Node{
			{Name: "child", Bundle: u2},
		},
	}
	c := &Cluster{Name: "c", Node: root}
	if err := ValidateCluster(c); err == nil {
		t.Fatal("expected shared umbrella child between two umbrellas to fail")
	}
}

func TestValidateCluster_MultiPackageWithUmbrellaRejected(t *testing.T) {
	child := &Bundle{Name: "child"}
	root := &Node{
		Name:       "root",
		PackageRef: &schema.GroupVersionKind{Group: "g", Version: "v", Kind: "K"},
		Bundle:     &Bundle{Name: "root", Children: []*Bundle{child}},
	}
	c := &Cluster{Name: "c", Node: root}
	if err := ValidateCluster(c); err == nil {
		t.Fatal("expected PackageRef + umbrella to be rejected")
	}
}

func TestValidateCluster_NoUmbrellaMultiPackageAllowed(t *testing.T) {
	root := &Node{
		Name:       "root",
		PackageRef: &schema.GroupVersionKind{Group: "g", Version: "v", Kind: "K"},
		Bundle:     &Bundle{Name: "root"},
	}
	c := &Cluster{Name: "c", Node: root}
	if err := ValidateCluster(c); err != nil {
		t.Fatalf("multi-package with no umbrella should pass: %v", err)
	}
}

func TestValidateCluster_NodeWithNilChild(t *testing.T) {
	// A node tree with a nil child pointer should not panic.
	root := &Node{
		Name:     "root",
		Bundle:   &Bundle{Name: "bundle"},
		Children: []*Node{nil, {Name: "valid", Bundle: &Bundle{Name: "child"}}},
	}
	c := &Cluster{Name: "c", Node: root}
	// Should not panic — the walkNodes nil guard should handle this.
	if err := ValidateCluster(c); err != nil {
		t.Fatalf("unexpected error with nil child node: %v", err)
	}
}

func TestValidateCluster_UmbrellaWithNilChild(t *testing.T) {
	// Bundle.Validate rejects a nil child entry — ValidateCluster must surface that error.
	root := &Bundle{
		Name:     "root",
		Children: []*Bundle{nil, {Name: "valid"}},
	}
	c := &Cluster{Name: "c", Node: &Node{Name: "n", Bundle: root}}
	if err := ValidateCluster(c); err == nil {
		t.Fatal("expected error for umbrella bundle with nil child entry")
	}
}

func TestValidateCluster_InvalidBundleBubblesUp(t *testing.T) {
	// Parent has two children with the same name — invalid per Bundle.Validate.
	root := &Bundle{Name: "root", Children: []*Bundle{{Name: "c"}, {Name: "c"}}}
	c := &Cluster{Name: "c", Node: &Node{Name: "n", Bundle: root}}
	if err := ValidateCluster(c); err == nil {
		t.Fatal("expected duplicate umbrella child names to fail")
	}
}

func TestValidateCluster_NodeCycle(t *testing.T) {
	// A Node reached again from its own descendant is a cycle: ValidateCluster
	// must return an error naming the node, not recurse until the stack runs out.
	root := &Node{Name: "root"}
	mid := &Node{Name: "mid"}
	root.Children = []*Node{mid}
	mid.Children = []*Node{root}
	c := &Cluster{Name: "c", Node: root}
	err := ValidateCluster(c)
	if err == nil {
		t.Fatal("expected error for cyclic Node graph")
	}
	if !strings.Contains(err.Error(), `node cycle detected at "root"`) {
		t.Errorf("error %q does not name the node where the cycle closes", err)
	}
}

func TestValidateCluster_NodeSelfCycle(t *testing.T) {
	n := &Node{Name: "self"}
	n.Children = []*Node{n}
	if err := ValidateCluster(&Cluster{Name: "c", Node: n}); err == nil {
		t.Fatal("expected error for a Node that is its own child")
	}
}

func TestValidateCluster_NodeCycleWithPackageRefScan(t *testing.T) {
	// The PackageRef scan runs only when an umbrella exists; a cycle must be
	// rejected before it, not recursed by it.
	umbrella := &Bundle{Name: "u", Children: []*Bundle{{Name: "child"}}}
	root := &Node{Name: "root", Bundle: umbrella}
	leaf := &Node{Name: "leaf"}
	root.Children = []*Node{leaf}
	leaf.Children = []*Node{root}
	if err := ValidateCluster(&Cluster{Name: "c", Node: root}); err == nil {
		t.Fatal("expected error for cyclic Node graph with an umbrella present")
	}
}

func TestValidateCluster_SharedNodeIsNotACycle(t *testing.T) {
	// A node reachable from two parents is not a cycle, so the cycle check
	// must not report one. (Whether such a tree is supported is a separate
	// question this check does not answer.)
	shared := &Node{Name: "shared", Bundle: &Bundle{Name: "b"}}
	a := &Node{Name: "a", Children: []*Node{shared}}
	b := &Node{Name: "b-node", Children: []*Node{shared}}
	root := &Node{Name: "root", Children: []*Node{a, b}}
	if err := ValidateCluster(&Cluster{Name: "c", Node: root}); err != nil {
		t.Fatalf("shared node rejected as a cycle: %v", err)
	}
}
