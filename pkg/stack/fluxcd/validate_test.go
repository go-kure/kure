package fluxcd

import (
	"testing"

	"github.com/go-kure/kure/pkg/stack"
)

func sr() *stack.SourceRef {
	return &stack.SourceRef{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"}
}

func TestValidateSourceRefsForFluxIntegrated_NilCluster(t *testing.T) {
	if err := validateSourceRefsForFluxIntegrated(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSourceRefsForFluxIntegrated_NilBundle(t *testing.T) {
	c := &stack.Cluster{Node: &stack.Node{Name: "prod"}}
	if err := validateSourceRefsForFluxIntegrated(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSourceRefsForFluxIntegrated_ValidSourceRef(t *testing.T) {
	c := &stack.Cluster{
		Node: &stack.Node{Name: "prod", Bundle: &stack.Bundle{Name: "apps", SourceRef: sr()}},
	}
	if err := validateSourceRefsForFluxIntegrated(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateSourceRefsForFluxIntegrated_InvalidSourceRef(t *testing.T) {
	cases := []struct {
		name string
		ref  *stack.SourceRef
	}{
		{"nil", nil},
		{"empty struct", &stack.SourceRef{}},
		{"missing Kind", &stack.SourceRef{Name: "flux-system"}},
		{"missing Name", &stack.SourceRef{Kind: "GitRepository"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &stack.Cluster{
				Node: &stack.Node{Name: "prod", Bundle: &stack.Bundle{Name: "apps", SourceRef: tc.ref}},
			}
			if err := validateSourceRefsForFluxIntegrated(c); err == nil {
				t.Fatalf("sourceRef=%v: expected error, got nil", tc.ref)
			}
		})
	}
}

func TestValidateSourceRefsForFluxIntegrated_MultiNode_OneMissing(t *testing.T) {
	infra := &stack.Node{Name: "infra", Bundle: &stack.Bundle{Name: "infra-apps", SourceRef: nil}}
	prod := &stack.Node{
		Name:     "prod",
		Bundle:   &stack.Bundle{Name: "apps", SourceRef: sr()},
		Children: []*stack.Node{infra},
	}
	if err := validateSourceRefsForFluxIntegrated(&stack.Cluster{Node: prod}); err == nil {
		t.Fatal("expected error for child node with nil SourceRef, got nil")
	}
}

func TestValidateSourceRefsForFluxIntegrated_UmbrellaChild_MissingSourceRef(t *testing.T) {
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: sr(),
		Children:  []*stack.Bundle{{Name: "platform-infra", SourceRef: nil}},
	}
	c := &stack.Cluster{Node: &stack.Node{Name: "prod", Bundle: umbrella}}
	if err := validateSourceRefsForFluxIntegrated(c); err == nil {
		t.Fatal("expected error for umbrella child with nil SourceRef, got nil")
	}
}

func TestValidateSourceRefsForFluxIntegrated_UmbrellaChildren_AllValid(t *testing.T) {
	umbrella := &stack.Bundle{
		Name:      "platform",
		SourceRef: sr(),
		Children: []*stack.Bundle{
			{Name: "platform-infra", SourceRef: sr()},
			{Name: "platform-apps", SourceRef: sr()},
		},
	}
	c := &stack.Cluster{Node: &stack.Node{Name: "prod", Bundle: umbrella}}
	if err := validateSourceRefsForFluxIntegrated(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
