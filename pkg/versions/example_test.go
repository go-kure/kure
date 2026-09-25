package versions_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/versions"
)

// The README's Go block is generated from this function
// (scripts/gen-doc-examples.sh). It has no output comment, so go test compiles
// it without running it: every value it prints moves on a routine dependency
// or toolchain bump, which is the README's point.

func ExampleGet() {
	k8s, ok := versions.Get("kubernetes")
	if !ok {
		return // the key was renamed or removed
	}
	fmt.Println(k8s.SupportedRange, k8s.Min, k8s.Max)

	for _, d := range versions.All() {
		fmt.Printf("%s: %s (%s)\n", d.Name, d.SupportedRange, d.GoModule)
	}

	fmt.Println(versions.GoVersion) // versions.yaml's go.current
}
