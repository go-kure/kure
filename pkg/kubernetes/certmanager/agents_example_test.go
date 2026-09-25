package certmanager_test

import (
	"fmt"

	cmmeta "github.com/cert-manager/cert-manager/pkg/apis/meta/v1"
)

// The Secret Management block of AGENTS.md is generated from this function
// (scripts/gen-doc-examples.sh).

func Example_agentsSecretKeySelector() {
	key := cmmeta.SecretKeySelector{
		LocalObjectReference: cmmeta.LocalObjectReference{Name: "secret-name"},
		Key:                  "key-name",
	}
	fmt.Println(key.Name, key.Key)
	// Output: secret-name key-name
}
