package fluxcd_test

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	sourcev1beta2 "github.com/fluxcd/source-controller/api/v1beta2"

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

func TestWorkflowBundleHealthChecks(t *testing.T) {
	b := &stack.Bundle{
		Name: "infra",
		HealthChecks: []stack.HealthCheck{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "my-app",
				Namespace:  "default",
			},
			{
				APIVersion: "helm.toolkit.fluxcd.io/v2",
				Kind:       "HelmRelease",
				Name:       "cert-manager",
				Namespace:  "flux-system",
			},
		},
	}

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("GenerateFromBundle() error = %v", err)
	}

	if len(objs) != 1 {
		t.Fatalf("expected 1 object, got %d", len(objs))
	}

	k := objs[0].(*kustv1.Kustomization)
	if len(k.Spec.HealthChecks) != 2 {
		t.Fatalf("HealthChecks length = %d, want 2", len(k.Spec.HealthChecks))
	}

	hc0 := k.Spec.HealthChecks[0]
	if hc0.APIVersion != "apps/v1" || hc0.Kind != "Deployment" || hc0.Name != "my-app" || hc0.Namespace != "default" {
		t.Errorf("HealthChecks[0] = %+v, want apps/v1 Deployment my-app default", hc0)
	}

	hc1 := k.Spec.HealthChecks[1]
	if hc1.Kind != "HelmRelease" || hc1.Name != "cert-manager" {
		t.Errorf("HealthChecks[1] = %+v, want HelmRelease cert-manager", hc1)
	}
}

func TestWorkflowBundleHealthChecksEmpty(t *testing.T) {
	b := &stack.Bundle{Name: "simple"}

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("GenerateFromBundle() error = %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if len(k.Spec.HealthChecks) != 0 {
		t.Errorf("HealthChecks should be empty, got %d", len(k.Spec.HealthChecks))
	}
}

func TestWorkflowEngine_GetName(t *testing.T) {
	wf := fluxstack.Engine()
	name := wf.GetName()
	if name == "" {
		t.Fatal("expected non-empty name")
	}
	if name != "FluxCD Workflow Engine" {
		t.Errorf("expected 'FluxCD Workflow Engine', got %q", name)
	}
}

func TestWorkflowEngine_GetVersion(t *testing.T) {
	wf := fluxstack.Engine()
	version := wf.GetVersion()
	if version == "" {
		t.Fatal("expected non-empty version")
	}
	// Version should follow semantic versioning
	if version[0] != 'v' {
		t.Errorf("expected version to start with 'v', got %q", version)
	}
}

func TestWorkflowEngine_GetResourceGenerator(t *testing.T) {
	wf := fluxstack.Engine()
	gen := wf.GetResourceGenerator()
	if gen == nil {
		t.Fatal("expected non-nil resource generator")
	}
}

func TestWorkflowEngine_GetLayoutIntegrator(t *testing.T) {
	wf := fluxstack.Engine()
	integ := wf.GetLayoutIntegrator()
	if integ == nil {
		t.Fatal("expected non-nil layout integrator")
	}
}

func TestWorkflowEngine_GetBootstrapGenerator(t *testing.T) {
	wf := fluxstack.Engine()
	gen := wf.GetBootstrapGenerator()
	if gen == nil {
		t.Fatal("expected non-nil bootstrap generator")
	}
}

func TestCreateSource_EmptyURL(t *testing.T) {
	// When URL is empty, createSource should return nil (reference-only mode)
	bundle := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Should only have the Kustomization, no source CRD
	if len(objs) != 1 {
		t.Fatalf("expected 1 object (Kustomization only), got %d", len(objs))
	}
	if _, ok := objs[0].(*kustv1.Kustomization); !ok {
		t.Fatalf("expected Kustomization, got %T", objs[0])
	}
}

func TestCreateSource_OCIRepository(t *testing.T) {
	bundle := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind:      "OCIRepository",
			Name:      "my-oci-source",
			Namespace: "flux-system",
			URL:       "oci://registry.example.com/manifests",
			Tag:       "v1.0.0",
		},
	}

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects (Kustomization + OCIRepository), got %d", len(objs))
	}

	// First object is the Kustomization
	kust, ok := objs[0].(*kustv1.Kustomization)
	if !ok {
		t.Fatalf("expected Kustomization, got %T", objs[0])
	}
	if kust.Spec.SourceRef.Kind != "OCIRepository" {
		t.Errorf("expected sourceRef kind OCIRepository, got %s", kust.Spec.SourceRef.Kind)
	}

	// Second object is the OCIRepository
	oci, ok := objs[1].(*sourcev1beta2.OCIRepository)
	if !ok {
		t.Fatalf("expected OCIRepository, got %T", objs[1])
	}
	if oci.Name != "my-oci-source" {
		t.Errorf("expected name my-oci-source, got %s", oci.Name)
	}
	if oci.Namespace != "flux-system" {
		t.Errorf("expected namespace flux-system, got %s", oci.Namespace)
	}
	if oci.Spec.URL != "oci://registry.example.com/manifests" {
		t.Errorf("expected URL oci://registry.example.com/manifests, got %s", oci.Spec.URL)
	}
	if oci.Spec.Reference == nil || oci.Spec.Reference.Tag != "v1.0.0" {
		t.Errorf("expected tag v1.0.0, got %v", oci.Spec.Reference)
	}
}

func TestCreateSource_GitRepository(t *testing.T) {
	bundle := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "my-git-source",
			Namespace: "flux-system",
			URL:       "https://github.com/example/repo",
			Branch:    "main",
		},
	}

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects (Kustomization + GitRepository), got %d", len(objs))
	}

	git, ok := objs[1].(*sourcev1.GitRepository)
	if !ok {
		t.Fatalf("expected GitRepository, got %T", objs[1])
	}
	if git.Name != "my-git-source" {
		t.Errorf("expected name my-git-source, got %s", git.Name)
	}
	if git.Spec.URL != "https://github.com/example/repo" {
		t.Errorf("expected URL https://github.com/example/repo, got %s", git.Spec.URL)
	}
	if git.Spec.Reference == nil || git.Spec.Reference.Branch != "main" {
		t.Errorf("expected branch main, got %v", git.Spec.Reference)
	}
}

func TestCreateSource_GitRepositoryWithTag(t *testing.T) {
	bundle := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "my-git-source",
			Namespace: "flux-system",
			URL:       "https://github.com/example/repo",
			Tag:       "v2.0.0",
		},
	}

	wf := fluxstack.Engine()
	objs, err := wf.GenerateFromBundle(bundle)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}

	git := objs[1].(*sourcev1.GitRepository)
	if git.Spec.Reference == nil || git.Spec.Reference.Tag != "v2.0.0" {
		t.Errorf("expected tag v2.0.0, got %v", git.Spec.Reference)
	}
}

func TestCreateSource_InvalidKind(t *testing.T) {
	bundle := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind: "InvalidKind",
			Name: "test-source",
			URL:  "https://example.com",
		},
	}

	wf := fluxstack.Engine()
	_, err := wf.GenerateFromBundle(bundle)
	if err == nil {
		t.Fatal("expected error for invalid source kind")
	}
}
