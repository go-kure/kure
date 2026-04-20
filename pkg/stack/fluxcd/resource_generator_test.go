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

func TestGenerateFromBundle_Force(t *testing.T) {
	wf := fluxstack.Engine()
	forceVal := true
	b := &stack.Bundle{
		Name:  "test",
		Force: &forceVal,
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if !k.Spec.Force {
		t.Error("expected Force to be true")
	}
}

func TestGenerateFromBundle_Suspend(t *testing.T) {
	wf := fluxstack.Engine()
	suspendVal := true
	b := &stack.Bundle{
		Name:    "test",
		Suspend: &suspendVal,
	}

	objs, err := wf.GenerateFromBundle(b)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	k := objs[0].(*kustv1.Kustomization)
	if !k.Spec.Suspend {
		t.Error("expected Suspend to be true")
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

func TestGeneratePath_UmbrellaChild(t *testing.T) {
	// Umbrella initialization wires child parent pointers, so the child's
	// Kustomization path should reflect the umbrella hierarchy.
	wf := fluxstack.EngineWithMode(layout.KustomizationExplicit)

	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "infra"},
		},
	}

	// Trigger InitializeUmbrella via GenerateFromBundle on the umbrella.
	if _, err := wf.GenerateFromBundle(umbrella); err != nil {
		t.Fatalf("umbrella generate: %v", err)
	}

	objs, err := wf.GenerateFromBundle(umbrella.Children[0])
	if err != nil {
		t.Fatalf("child generate: %v", err)
	}
	k := objs[0].(*kustv1.Kustomization)
	if k.Spec.Path != "platform/infra" {
		t.Errorf("Path = %q, want platform/infra", k.Spec.Path)
	}
}

func TestGenerateFromBundle_Patches(t *testing.T) {
	t.Run("empty patches leaves spec nil", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{Name: "test"}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if k.Spec.Patches != nil {
			t.Errorf("expected nil Patches, got %v", k.Spec.Patches)
		}
	})

	t.Run("single patch without target", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{
			Name: "test",
			Patches: []stack.Patch{
				{Patch: `{"op":"add","path":"/metadata/labels/env","value":"prod"}`},
			},
		}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if len(k.Spec.Patches) != 1 {
			t.Fatalf("expected 1 patch, got %d", len(k.Spec.Patches))
		}
		if k.Spec.Patches[0].Patch != `{"op":"add","path":"/metadata/labels/env","value":"prod"}` {
			t.Errorf("Patch content mismatch: %q", k.Spec.Patches[0].Patch)
		}
		if k.Spec.Patches[0].Target != nil {
			t.Errorf("expected nil Target, got %v", k.Spec.Patches[0].Target)
		}
	})

	t.Run("patch with full selector", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{
			Name: "test",
			Patches: []stack.Patch{
				{
					Patch: "apiVersion: apps/v1\nkind: Deployment",
					Target: &stack.PatchSelector{
						Group:              "apps",
						Version:            "v1",
						Kind:               "Deployment",
						Name:               "my-app",
						Namespace:          "default",
						LabelSelector:      "app=my-app",
						AnnotationSelector: "tier=backend",
					},
				},
			},
		}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if len(k.Spec.Patches) != 1 {
			t.Fatalf("expected 1 patch, got %d", len(k.Spec.Patches))
		}
		sel := k.Spec.Patches[0].Target
		if sel == nil {
			t.Fatal("expected non-nil Target selector")
		}
		if sel.Group != "apps" {
			t.Errorf("Group = %q, want %q", sel.Group, "apps")
		}
		if sel.Version != "v1" {
			t.Errorf("Version = %q, want %q", sel.Version, "v1")
		}
		if sel.Kind != "Deployment" {
			t.Errorf("Kind = %q, want %q", sel.Kind, "Deployment")
		}
		if sel.Name != "my-app" {
			t.Errorf("Name = %q, want %q", sel.Name, "my-app")
		}
		if sel.Namespace != "default" {
			t.Errorf("Namespace = %q, want %q", sel.Namespace, "default")
		}
		if sel.LabelSelector != "app=my-app" {
			t.Errorf("LabelSelector = %q, want %q", sel.LabelSelector, "app=my-app")
		}
		if sel.AnnotationSelector != "tier=backend" {
			t.Errorf("AnnotationSelector = %q, want %q", sel.AnnotationSelector, "tier=backend")
		}
	})

	t.Run("multiple patches preserve order", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{
			Name: "test",
			Patches: []stack.Patch{
				{Patch: "first"},
				{Patch: "second"},
				{Patch: "third"},
			},
		}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if len(k.Spec.Patches) != 3 {
			t.Fatalf("expected 3 patches, got %d", len(k.Spec.Patches))
		}
		for i, want := range []string{"first", "second", "third"} {
			if k.Spec.Patches[i].Patch != want {
				t.Errorf("Patches[%d].Patch = %q, want %q", i, k.Spec.Patches[i].Patch, want)
			}
		}
	})
}

func TestGenerateFromBundle_PostBuild(t *testing.T) {
	t.Run("nil PostBuild leaves spec nil", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{Name: "test"}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if k.Spec.PostBuild != nil {
			t.Errorf("expected nil PostBuild, got %v", k.Spec.PostBuild)
		}
	})

	t.Run("Substitute only", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{
			Name: "test",
			PostBuild: &stack.PostBuild{
				Substitute: map[string]string{
					"CLUSTER_ENV": "production",
					"REGION":      "eu-west-1",
				},
			},
		}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if k.Spec.PostBuild == nil {
			t.Fatal("expected PostBuild to be set")
		}
		if k.Spec.PostBuild.Substitute["CLUSTER_ENV"] != "production" {
			t.Errorf("CLUSTER_ENV = %q, want production", k.Spec.PostBuild.Substitute["CLUSTER_ENV"])
		}
		if k.Spec.PostBuild.Substitute["REGION"] != "eu-west-1" {
			t.Errorf("REGION = %q, want eu-west-1", k.Spec.PostBuild.Substitute["REGION"])
		}
		if len(k.Spec.PostBuild.SubstituteFrom) != 0 {
			t.Errorf("expected empty SubstituteFrom, got %d entries", len(k.Spec.PostBuild.SubstituteFrom))
		}
	})

	t.Run("SubstituteFrom only", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{
			Name: "test",
			PostBuild: &stack.PostBuild{
				SubstituteFrom: []stack.SubstituteRef{
					{Kind: "ConfigMap", Name: "cluster-vars", Optional: false},
					{Kind: "Secret", Name: "cluster-secrets", Optional: true},
				},
			},
		}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if k.Spec.PostBuild == nil {
			t.Fatal("expected PostBuild to be set")
		}
		if len(k.Spec.PostBuild.SubstituteFrom) != 2 {
			t.Fatalf("expected 2 SubstituteFrom entries, got %d", len(k.Spec.PostBuild.SubstituteFrom))
		}
		if k.Spec.PostBuild.SubstituteFrom[0].Kind != "ConfigMap" {
			t.Errorf("SubstituteFrom[0].Kind = %q, want ConfigMap", k.Spec.PostBuild.SubstituteFrom[0].Kind)
		}
		if k.Spec.PostBuild.SubstituteFrom[0].Name != "cluster-vars" {
			t.Errorf("SubstituteFrom[0].Name = %q, want cluster-vars", k.Spec.PostBuild.SubstituteFrom[0].Name)
		}
		if k.Spec.PostBuild.SubstituteFrom[0].Optional {
			t.Error("SubstituteFrom[0].Optional = true, want false")
		}
		if k.Spec.PostBuild.SubstituteFrom[1].Kind != "Secret" {
			t.Errorf("SubstituteFrom[1].Kind = %q, want Secret", k.Spec.PostBuild.SubstituteFrom[1].Kind)
		}
		if !k.Spec.PostBuild.SubstituteFrom[1].Optional {
			t.Error("SubstituteFrom[1].Optional = false, want true")
		}
	})

	t.Run("Substitute and SubstituteFrom combined", func(t *testing.T) {
		wf := fluxstack.Engine()
		b := &stack.Bundle{
			Name: "test",
			PostBuild: &stack.PostBuild{
				Substitute: map[string]string{"ENV": "staging"},
				SubstituteFrom: []stack.SubstituteRef{
					{Kind: "ConfigMap", Name: "extra-vars"},
				},
			},
		}
		objs, err := wf.GenerateFromBundle(b)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		k := objs[0].(*kustv1.Kustomization)
		if k.Spec.PostBuild == nil {
			t.Fatal("expected PostBuild to be set")
		}
		if k.Spec.PostBuild.Substitute["ENV"] != "staging" {
			t.Errorf("ENV = %q, want staging", k.Spec.PostBuild.Substitute["ENV"])
		}
		if len(k.Spec.PostBuild.SubstituteFrom) != 1 {
			t.Fatalf("expected 1 SubstituteFrom entry, got %d", len(k.Spec.PostBuild.SubstituteFrom))
		}
	})
}
