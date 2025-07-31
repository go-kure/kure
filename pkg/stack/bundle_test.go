package stack

import "testing"

// TestBundleValidate exercises the Bundle validation logic against
// the various failure modes as well as the happy path.
func TestBundleValidate(t *testing.T) {
	// nil bundle should error
	var nilBundle *Bundle
	if err := nilBundle.Validate(); err == nil {
		t.Fatalf("expected error for nil bundle")
	}

	// empty name should error
	b := &Bundle{Name: "", Applications: []*Application{}}
	if err := b.Validate(); err == nil {
		t.Fatalf("expected validation error for empty name")
	}

	// nil application inside the slice should error
	b = &Bundle{Name: "test", Applications: []*Application{nil}}
	if err := b.Validate(); err == nil {
		t.Fatalf("expected error for nil application entry")
	}

	// valid bundle should pass
	app := NewApplication("app", "ns", &fakeConfig{})
	b = &Bundle{Name: "ok", Applications: []*Application{app}}
	if err := b.Validate(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
