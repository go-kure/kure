package fluxcd_test

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"

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
	oci, ok := objs[1].(*sourcev1.OCIRepository)
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

func TestGenerateFromBundle_UmbrellaAutoHealthChecks(t *testing.T) {
	wf := fluxstack.Engine()
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "infra"},
			{Name: "services"},
			{Name: "apps"},
		},
	}

	objs, err := wf.GenerateFromBundle(umbrella)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Self-only: umbrella bundle emits ONE Kustomization, not four.
	if len(objs) != 1 {
		t.Fatalf("expected 1 object (self-only), got %d", len(objs))
	}

	k, ok := objs[0].(*kustv1.Kustomization)
	if !ok {
		t.Fatalf("expected *kustv1.Kustomization, got %T", objs[0])
	}

	if !k.Spec.Wait {
		t.Error("expected Wait=true for umbrella bundle")
	}

	if len(k.Spec.HealthChecks) != 3 {
		t.Fatalf("expected 3 auto HealthChecks, got %d", len(k.Spec.HealthChecks))
	}
	wantNames := []string{"infra", "services", "apps"}
	for i, want := range wantNames {
		hc := k.Spec.HealthChecks[i]
		if hc.Name != want {
			t.Errorf("HealthChecks[%d].Name = %q, want %q", i, hc.Name, want)
		}
		if hc.Kind != "Kustomization" {
			t.Errorf("HealthChecks[%d].Kind = %q, want Kustomization", i, hc.Kind)
		}
		if hc.APIVersion != kustv1.GroupVersion.String() {
			t.Errorf("HealthChecks[%d].APIVersion = %q, want %q", i, hc.APIVersion, kustv1.GroupVersion.String())
		}
		if hc.Namespace != "flux-system" {
			t.Errorf("HealthChecks[%d].Namespace = %q, want flux-system", i, hc.Namespace)
		}
	}
}

func TestGenerateFromBundle_UmbrellaUserHealthChecksAppended(t *testing.T) {
	wf := fluxstack.Engine()
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "infra"},
		},
		HealthChecks: []stack.HealthCheck{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Name:       "manual-check",
				Namespace:  "default",
			},
		},
	}

	objs, err := wf.GenerateFromBundle(umbrella)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := objs[0].(*kustv1.Kustomization)

	if len(k.Spec.HealthChecks) != 2 {
		t.Fatalf("expected 2 HealthChecks (1 auto + 1 user), got %d", len(k.Spec.HealthChecks))
	}
	// Auto entries come first.
	if k.Spec.HealthChecks[0].Name != "infra" {
		t.Errorf("HealthChecks[0] = %q, want auto entry 'infra'", k.Spec.HealthChecks[0].Name)
	}
	if k.Spec.HealthChecks[1].Name != "manual-check" {
		t.Errorf("HealthChecks[1] = %q, want user entry 'manual-check'", k.Spec.HealthChecks[1].Name)
	}
}

func TestGenerateFromBundle_UmbrellaPreservesTimeout(t *testing.T) {
	wf := fluxstack.Engine()
	umbrella := &stack.Bundle{
		Name:          "platform",
		Timeout:       "10m",
		RetryInterval: "2m",
		Children: []*stack.Bundle{
			{Name: "infra"},
		},
	}

	objs, err := wf.GenerateFromBundle(umbrella)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	k := objs[0].(*kustv1.Kustomization)

	if k.Spec.Timeout == nil || k.Spec.Timeout.Duration.String() != "10m0s" {
		t.Errorf("Timeout = %v, want 10m", k.Spec.Timeout)
	}
	if k.Spec.RetryInterval == nil || k.Spec.RetryInterval.Duration.String() != "2m0s" {
		t.Errorf("RetryInterval = %v, want 2m", k.Spec.RetryInterval)
	}
}

func TestGenerateFromBundle_UmbrellaDoesNotRecurse(t *testing.T) {
	// Assert the GenerateFromBundle self-only invariant: a two-level umbrella
	// still yields exactly 1 object (just the top bundle's Kustomization).
	wf := fluxstack.Engine()
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{
				Name: "infra",
				Children: []*stack.Bundle{
					{Name: "networking"},
				},
			},
		},
	}

	objs, err := wf.GenerateFromBundle(umbrella)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) != 1 {
		t.Fatalf("expected 1 object (self-only), got %d", len(objs))
	}
}

func TestGenerateFromNode_UmbrellaClosure(t *testing.T) {
	// GenerateFromNode must walk the umbrella subtree so flat-list consumers
	// (separate Flux placement) see every descendant Kustomization.
	wf := fluxstack.Engine()
	n := &stack.Node{
		Name: "root",
		Bundle: &stack.Bundle{
			Name: "platform",
			Children: []*stack.Bundle{
				{Name: "infra"},
				{Name: "services"},
				{Name: "apps"},
			},
		},
	}

	objs, err := wf.GenerateFromNode(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// 1 umbrella + 3 children = 4 Kustomizations.
	if len(objs) != 4 {
		t.Fatalf("expected 4 objects (umbrella + 3 children), got %d", len(objs))
	}

	names := map[string]bool{}
	for _, o := range objs {
		k, ok := o.(*kustv1.Kustomization)
		if !ok {
			t.Fatalf("unexpected object type %T", o)
		}
		names[k.Name] = true
	}
	for _, want := range []string{"platform", "infra", "services", "apps"} {
		if !names[want] {
			t.Errorf("missing Kustomization %q in closure output", want)
		}
	}
}

func TestGenerateFromNode_NestedUmbrellaClosure(t *testing.T) {
	wf := fluxstack.Engine()
	n := &stack.Node{
		Name: "root",
		Bundle: &stack.Bundle{
			Name: "platform",
			Children: []*stack.Bundle{
				{
					Name: "infra",
					Children: []*stack.Bundle{
						{Name: "networking"},
						{Name: "storage"},
					},
				},
				{Name: "apps"},
			},
		},
	}

	objs, err := wf.GenerateFromNode(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// platform + infra + networking + storage + apps = 5
	if len(objs) != 5 {
		t.Fatalf("expected 5 objects, got %d", len(objs))
	}
}

func TestGenerateFromNode_UmbrellaChildWithSource(t *testing.T) {
	wf := fluxstack.Engine()
	n := &stack.Node{
		Name: "root",
		Bundle: &stack.Bundle{
			Name: "platform",
			Children: []*stack.Bundle{
				{
					Name: "ext",
					SourceRef: &stack.SourceRef{
						Kind:   "GitRepository",
						Name:   "ext-repo",
						URL:    "https://github.com/example/ext",
						Branch: "main",
					},
				},
			},
		},
	}

	objs, err := wf.GenerateFromNode(n)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// umbrella Kustomization + ext Kustomization + ext GitRepository source = 3
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	var sawSource bool
	for _, o := range objs {
		if _, ok := o.(*sourcev1.GitRepository); ok {
			sawSource = true
		}
	}
	if !sawSource {
		t.Error("expected umbrella child's GitRepository source in output")
	}
}

func TestGenerateFromBundle_UmbrellaInitializesParent(t *testing.T) {
	// Umbrella path should call InitializeUmbrella which sets child parent
	// pointers, allowing path derivation to work without manual SetParent.
	wf := fluxstack.EngineWithMode(layout.KustomizationExplicit)
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "infra"},
		},
	}

	// Generate the umbrella first — this should initialize children.
	if _, err := wf.GenerateFromBundle(umbrella); err != nil {
		t.Fatalf("umbrella generate: %v", err)
	}

	// Now generate the child — its path must include the parent.
	childObjs, err := wf.GenerateFromBundle(umbrella.Children[0])
	if err != nil {
		t.Fatalf("child generate: %v", err)
	}
	k := childObjs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "platform/infra" {
		t.Errorf("child Path = %q, want platform/infra (parent not wired)", k.Spec.Path)
	}
}
