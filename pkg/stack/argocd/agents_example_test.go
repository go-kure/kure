package argocd_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/stack"
)

// The ArgoCD Integration block of AGENTS.md is generated from this function
// (scripts/gen-doc-examples.sh).

func Example_agentsWorkflow() {
	cluster := exampleCluster()

	// The provider registers itself from its init, so the package has to be
	// imported: import _ "github.com/go-kure/kure/pkg/stack/argocd"
	wf, err := stack.NewWorkflow("argocd")
	if err != nil {
		panic(err)
	}
	// Application paths are the directories a default-rules WalkCluster writes;
	// CreateLayoutWithResources generates from the layout it walks with your rules.
	apps, err := wf.GenerateFromCluster(cluster)
	if err != nil {
		panic(err)
	}
	fmt.Println(len(apps), apps[0].GetObjectKind().GroupVersionKind().Kind, apps[0].GetName())
	// Output: 1 Application web
}
