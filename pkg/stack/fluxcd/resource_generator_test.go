package fluxcd_test

import (
	"testing"
	"time"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestResourceGenerator_NewDefaults(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()

	if gen.Mode != layout.KustomizationExplicit {
		t.Errorf("Mode = %q, want %q", gen.Mode, layout.KustomizationExplicit)
	}
	if gen.DefaultInterval != 5*time.Minute {
		t.Errorf("DefaultInterval = %v, want %v", gen.DefaultInterval, 5*time.Minute)
	}
	if gen.DefaultNamespace != "flux-system" {
		t.Errorf("DefaultNamespace = %q, want %q", gen.DefaultNamespace, "flux-system")
	}
}

func TestGenerateFromCluster_Nil(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	objs, err := gen.GenerateFromCluster(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Fatalf("expected nil objects, got %d", len(objs))
	}
}

func TestGenerateFromCluster_NilNode(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	c := &stack.Cluster{Name: "test-cluster"}
	objs, err := gen.GenerateFromCluster(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Fatalf("expected nil objects, got %d", len(objs))
	}
}

func TestGenerateFromCluster_WithBundle(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()

	b := &stack.Bundle{
		Name: "infra",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "flux-system",
			Namespace: "flux-system",
		},
	}
	c := &stack.Cluster{
		Name: "prod",
		Node: &stack.Node{
			Name:   "root",
			Bundle: b,
		},
	}

	objs, err := gen.GenerateFromCluster(c)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(objs) == 0 {
		t.Fatal("expected at least one object")
	}

	k, ok := objs[0].(*kustv1.Kustomization)
	if !ok {
		t.Fatalf("expected Kustomization, got %T", objs[0])
	}
	if k.Name != "infra" {
		t.Errorf("Name = %q, want %q", k.Name, "infra")
	}
}

func TestGenerateFromNode_Nil(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	objs, err := gen.GenerateFromNode(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Fatalf("expected nil objects, got %d", len(objs))
	}
}

func TestGenerateFromNode_Recursive(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()

	childBundle := &stack.Bundle{Name: "child-bundle"}
	grandchildBundle := &stack.Bundle{Name: "grandchild-bundle"}

	root := &stack.Node{
		Name: "root",
		Bundle: &stack.Bundle{
			Name: "root-bundle",
		},
		Children: []*stack.Node{
			{
				Name:   "child",
				Bundle: childBundle,
				Children: []*stack.Node{
					{
						Name:   "grandchild",
						Bundle: grandchildBundle,
					},
				},
			},
		},
	}

	objs, err := gen.GenerateFromNode(root)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// root-bundle + child-bundle + grandchild-bundle = 3 Kustomizations
	if len(objs) != 3 {
		t.Fatalf("expected 3 objects, got %d", len(objs))
	}

	names := make(map[string]bool)
	for _, obj := range objs {
		k, ok := obj.(*kustv1.Kustomization)
		if !ok {
			t.Fatalf("expected Kustomization, got %T", obj)
		}
		names[k.Name] = true
	}

	for _, want := range []string{"root-bundle", "child-bundle", "grandchild-bundle"} {
		if !names[want] {
			t.Errorf("missing Kustomization %q", want)
		}
	}
}

func TestGenerateFromBundle_Nil(t *testing.T) {
	gen := fluxstack.NewResourceGenerator()
	objs, err := gen.GenerateFromBundle(nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Fatalf("expected nil objects, got %d", len(objs))
	}
}

func TestGenerateFromBundle_PruneDefault(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{Name: "test"}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if !k.Spec.Prune {
		t.Error("expected Prune to default to true")
	}
}

func TestGenerateFromBundle_PruneExplicitFalse(t *testing.T) {
	wf := fluxstack.Engine()
	pruneVal := false
	b := &stack.Bundle{
		Name:  "test",
		Prune: &pruneVal,
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Prune {
		t.Error("expected Prune to be false when explicitly set")
	}
}

func TestGenerateFromBundle_Wait(t *testing.T) {
	wf := fluxstack.Engine()
	waitVal := true
	b := &stack.Bundle{
		Name: "test",
		Wait: &waitVal,
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if !k.Spec.Wait {
		t.Error("expected Wait to be true")
	}
}

func TestGenerateFromBundle_Timeout(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{
		Name:    "test",
		Timeout: "5m",
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Timeout == nil {
		t.Fatal("expected Timeout to be set")
	}
	if k.Spec.Timeout.Duration != 5*time.Minute {
		t.Errorf("Timeout = %v, want %v", k.Spec.Timeout.Duration, 5*time.Minute)
	}
}

func TestGenerateFromBundle_RetryInterval(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{
		Name:          "test",
		RetryInterval: "2m",
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.RetryInterval == nil {
		t.Fatal("expected RetryInterval to be set")
	}
	if k.Spec.RetryInterval.Duration != 2*time.Minute {
		t.Errorf("RetryInterval = %v, want %v", k.Spec.RetryInterval.Duration, 2*time.Minute)
	}
}

func TestGenerateFromBundle_Labels(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{
		Name: "test",
		Labels: map[string]string{
			"app.kubernetes.io/part-of": "platform",
			"team":                      "infra",
		},
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Labels == nil {
		t.Fatal("expected labels to be set")
	}
	if k.Labels["app.kubernetes.io/part-of"] != "platform" {
		t.Errorf("label app.kubernetes.io/part-of = %q, want %q", k.Labels["app.kubernetes.io/part-of"], "platform")
	}
	if k.Labels["team"] != "infra" {
		t.Errorf("label team = %q, want %q", k.Labels["team"], "infra")
	}
}

func TestGenerateFromBundle_Annotations(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{
		Name: "test",
		Annotations: map[string]string{
			"description": "Core infrastructure bundle",
		},
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Annotations == nil {
		t.Fatal("expected annotations to be set")
	}
	if k.Annotations["description"] != "Core infrastructure bundle" {
		t.Errorf("annotation description = %q, want %q", k.Annotations["description"], "Core infrastructure bundle")
	}
}

func TestGenerateFromBundle_SourceRefNamespace(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "my-repo",
			Namespace: "custom-ns",
		},
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.SourceRef.Namespace != "custom-ns" {
		t.Errorf("SourceRef.Namespace = %q, want %q", k.Spec.SourceRef.Namespace, "custom-ns")
	}
}

func TestCreateSource_DefaultNamespace(t *testing.T) {
	wf := fluxstack.Engine()
	b := &stack.Bundle{
		Name: "test",
		SourceRef: &stack.SourceRef{
			Kind:   "GitRepository",
			Name:   "my-repo",
			URL:    "https://github.com/example/repo",
			Branch: "main",
			// Namespace intentionally empty
		},
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have Kustomization + GitRepository
	if len(objs) != 2 {
		t.Fatalf("expected 2 objects, got %d", len(objs))
	}

	git, ok := objs[1].(*sourcev1.GitRepository)
	if !ok {
		t.Fatalf("expected GitRepository, got %T", objs[1])
	}
	if git.Namespace != "flux-system" {
		t.Errorf("GitRepository.Namespace = %q, want %q (default)", git.Namespace, "flux-system")
	}
}

func TestGeneratePath_Explicit(t *testing.T) {
	wf := fluxstack.EngineWithMode(layout.KustomizationExplicit)

	parent := &stack.Bundle{Name: "infra"}
	child := &stack.Bundle{Name: "networking"}
	child.SetParent(parent)

	objs, err := wf.GenerateFromBundle(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "infra/networking" {
		t.Errorf("Path = %q, want %q", k.Spec.Path, "infra/networking")
	}
}

func TestGeneratePath_Recursive(t *testing.T) {
	wf := fluxstack.EngineWithMode(layout.KustomizationRecursive)

	parent := &stack.Bundle{Name: "infra"}
	child := &stack.Bundle{Name: "networking"}
	child.SetParent(parent)

	objs, err := wf.GenerateFromBundle(child)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "infra" {
		t.Errorf("Path = %q, want %q", k.Spec.Path, "infra")
	}
}

func TestBundlePath_MultiLevel(t *testing.T) {
	wf := fluxstack.EngineWithMode(layout.KustomizationExplicit)

	root := &stack.Bundle{Name: "cluster"}
	mid := &stack.Bundle{Name: "infrastructure"}
	mid.SetParent(root)
	leaf := &stack.Bundle{Name: "networking"}
	leaf.SetParent(mid)

	objs, err := wf.GenerateFromBundle(leaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "cluster/infrastructure/networking" {
		t.Errorf("Path = %q, want %q", k.Spec.Path, "cluster/infrastructure/networking")
	}

	// In recursive mode, the leaf should use the parent's path
	wf.SetKustomizationMode(layout.KustomizationRecursive)
	objs, err = wf.GenerateFromBundle(leaf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k = objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "cluster/infrastructure" {
		t.Errorf("recursive Path = %q, want %q", k.Spec.Path, "cluster/infrastructure")
	}
}

func TestGenerateFromCluster_InvalidUmbrellaRejected(t *testing.T) {
	// Shared pointer is both a child node Bundle and an umbrella child —
	// ValidateCluster must reject.
	shared := &stack.Bundle{Name: "shared"}
	root := &stack.Node{
		Name:   "root",
		Bundle: &stack.Bundle{Name: "root", Children: []*stack.Bundle{shared}},
		Children: []*stack.Node{
			{Name: "child", Bundle: shared},
		},
	}
	c := &stack.Cluster{Name: "c", Node: root}

	gen := fluxstack.NewResourceGenerator()
	_, err := gen.GenerateFromCluster(c)
	if err == nil {
		t.Fatal("expected invalid umbrella cluster to be rejected by GenerateFromCluster")
	}
}
