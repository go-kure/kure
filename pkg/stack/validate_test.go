package stack

import (
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

func TestValidateCluster_InvalidBundleBubblesUp(t *testing.T) {
	// Parent has Wait=false but has Children — invalid per Bundle.Validate.
	falseVal := false
	root := &Bundle{Name: "root", Wait: &falseVal, Children: []*Bundle{{Name: "c"}}}
	c := &Cluster{Name: "c", Node: &Node{Name: "n", Bundle: root}}
	if err := ValidateCluster(c); err == nil {
		t.Fatal("expected Wait=false umbrella to fail")
	}
}
