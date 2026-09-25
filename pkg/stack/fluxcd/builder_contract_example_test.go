package fluxcd_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/stack/fluxcd"
)

// The defaults block of the "Builder Contract" concept page
// (site/content/concepts/builder-contract.md) is generated from this function
// (scripts/gen-doc-examples.sh).

func Example_builderContractDefaults() {
	// declared in pkg/stack/fluxcd/defaults.go
	fmt.Println(fluxcd.DefaultInterval)   // 60 * time.Minute
	fmt.Println(fluxcd.DefaultNamespace)  // "flux-system"
	fmt.Println(fluxcd.DefaultSourceKind) // "OCIRepository"
	// Output:
	// 1h0m0s
	// flux-system
	// OCIRepository
}
