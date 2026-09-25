package stack_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
)

// The Input Validation block of docs/ARCHITECTURE.md is generated from this
// function (scripts/gen-doc-examples.sh).

func Example_architectureValidate() {
	c := stack.NewCluster("prod", &stack.Node{
		Name:   "apps",
		Bundle: &stack.Bundle{Name: "web", Applications: []*stack.Application{nil}},
	})

	check := func(c *stack.Cluster) error {
		// Validation is a call the caller makes, not a side effect of construction.
		if err := stack.ValidateCluster(c); err != nil {
			return errors.Wrap(err, "cluster is not layoutable")
		}
		return nil
	}
	fmt.Println(check(c))
	// Output: cluster is not layoutable: bundle "web" failed validation: validation failed for Bundle 'web' field 'applications': application at index 0 is nil
}
