package flux_test

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	layoutpkg "github.com/go-kure/kure/pkg/layout"
	"github.com/go-kure/kure/pkg/stack"
	fluxpkg "github.com/go-kure/kure/pkg/stack/flux"
)

func TestWorkflowBundlePathMode(t *testing.T) {
	parent := &stack.Bundle{Name: "parent"}
	child := &stack.Bundle{Name: "child", Parent: parent}

	wf := fluxpkg.NewWorkflow()
	wf.Mode = layoutpkg.KustomizationExplicit
	objs, err := wf.Bundle(child)
	if err != nil {
		t.Fatalf("bundle explicit: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}
	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "parent/child" {
		t.Fatalf("explicit path mismatch: %s", k.Spec.Path)
	}

	wf.Mode = layoutpkg.KustomizationRecursive
	objs, err = wf.Bundle(child)
	if err != nil {
		t.Fatalf("bundle recursive: %v", err)
	}
	k = objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "parent" {
		t.Fatalf("recursive path mismatch: %s", k.Spec.Path)
	}
}
