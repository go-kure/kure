package stack_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/stack"
)

// The construction block of pkg/stack/DESIGN.md is generated from this
// function (scripts/gen-doc-examples.sh).

func ExampleCluster() {
	cluster := &stack.Cluster{
		Name: "prod",
		Node: &stack.Node{Name: "apps"},
	}
	fmt.Println(cluster.Name, cluster.Node.Name)
	// Output: prod apps
}
