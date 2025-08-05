package fluxcd_test

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestWorkflowBundlePathMode(t *testing.T) {
	parent := &stack.Bundle{Name: "parent"}
	child := &stack.Bundle{Name: "child"}
	child.SetParent(parent)

	wf := fluxstack.EngineWithMode(layout.KustomizationExplicit)
	objs, err := wf.GenerateFromBundle(child)
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

	wf.SetKustomizationMode(layout.KustomizationRecursive)
	objs, err = wf.GenerateFromBundle(child)
	if err != nil {
		t.Fatalf("bundle recursive: %v", err)
	}
	k = objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "parent" {
		t.Fatalf("recursive path mismatch: %s", k.Spec.Path)
	}
}

func TestWorkflowBundleMetadata(t *testing.T) {
	parent := &stack.Bundle{Name: "parent"}
	dep := &stack.Bundle{Name: "dep"}
	child := &stack.Bundle{
		Name:     "child",
		Interval: "1m",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "demo",
			Namespace: "flux-system",
		},
		DependsOn: []*stack.Bundle{dep},
	}
	child.SetParent(parent)

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(child)
	if err != nil {
		t.Fatalf("bundle: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Interval.Duration.String() != "1m0s" {
		t.Fatalf("interval mismatch: %s", k.Spec.Interval.Duration)
	}
	if k.Spec.SourceRef.Name != "demo" || k.Spec.SourceRef.Kind != "GitRepository" {
		t.Fatalf("source ref mismatch: %#v", k.Spec.SourceRef)
	}
	if len(k.Spec.DependsOn) != 1 || k.Spec.DependsOn[0].Name != "dep" {
		t.Fatalf("dependsOn mismatch: %#v", k.Spec.DependsOn)
	}
}
