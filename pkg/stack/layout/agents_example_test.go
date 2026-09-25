package layout_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// The Layout Generation block of AGENTS.md is generated from this function
// (scripts/gen-doc-examples.sh).

func Example_agentsLayout() {
	cluster := exampleCluster()

	rules := layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	}
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		panic(err)
	}
	fmt.Println(ml.FullRepoPath(), len(ml.Resources))
	// Output: apps 2
}
