package stack_test

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack"
)

// TestBundleValidate exercises the Bundle validation logic against
// the various failure modes as well as the happy path.
func TestBundleValidate(t *testing.T) {
	// nil bundle should error
	var nilBundle *stack.Bundle
	if err := nilBundle.Validate(); err == nil {
		t.Fatalf("expected error for nil bundle")
	}

	// empty name should error
	b := &stack.Bundle{Name: "", Applications: []*stack.Application{}}
	if err := b.Validate(); err == nil {
		t.Fatalf("expected validation error for empty name")
	}

	// nil application inside the slice should error
	b = &stack.Bundle{Name: "test", Applications: []*stack.Application{nil}}
	if err := b.Validate(); err == nil {
		t.Fatalf("expected error for nil application entry")
	}

	// valid bundle should pass
	app := stack.NewApplication("app", "ns", &fakeConfig{})
	b = &stack.Bundle{Name: "ok", Applications: []*stack.Application{app}}
	if err := b.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
