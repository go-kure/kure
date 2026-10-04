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
	// The root node's layout renders no bundle: bundle "web" has the directory
	// inside it.
	unit := ml.OriginUnit()
	fmt.Println(ml.FullRepoPath(), len(ml.Resources))
	fmt.Println(unit.FullRepoPath(), len(unit.Resources))
	// Output:
	// apps 0
	// apps/web 2
}
