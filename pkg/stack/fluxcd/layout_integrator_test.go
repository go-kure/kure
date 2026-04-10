package fluxcd_test

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestNewLayoutIntegrator(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	if integrator == nil {
		t.Fatal("expected non-nil integrator")
	}

	if integrator.Generator != generator {
		t.Error("generator not set correctly")
	}

	if integrator.FluxPlacement != layout.FluxIntegrated {
		t.Errorf("expected FluxIntegrated placement, got %s", integrator.FluxPlacement)
	}
}

func TestLayoutIntegrator_SetFluxPlacement(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Test setting to separate
	integrator.SetFluxPlacement(layout.FluxSeparate)
	if integrator.FluxPlacement != layout.FluxSeparate {
		t.Errorf("expected FluxSeparate, got %s", integrator.FluxPlacement)
	}

	// Test setting to integrated
	integrator.SetFluxPlacement(layout.FluxIntegrated)
	if integrator.FluxPlacement != layout.FluxIntegrated {
		t.Errorf("expected FluxIntegrated, got %s", integrator.FluxPlacement)
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_NilInputs(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Test with nil layout
	err := integrator.IntegrateWithLayout(nil, &stack.Cluster{}, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil layout, got %v", err)
	}

	// Test with nil cluster
	ml := &layout.ManifestLayout{}
	err = integrator.IntegrateWithLayout(ml, nil, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil cluster, got %v", err)
	}

	// Test with both nil
	err = integrator.IntegrateWithLayout(nil, nil, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil inputs, got %v", err)
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_InvalidPlacement(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Set invalid placement
	integrator.FluxPlacement = layout.FluxPlacement("invalid")

	ml := &layout.ManifestLayout{Name: "test"}
	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: &stack.Node{Name: "root"},
	}

	err := integrator.IntegrateWithLayout(ml, cluster, layout.LayoutRules{})
	if err == nil {
		t.Error("expected error for invalid placement")
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_Integrated(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)
	integrator.SetFluxPlacement(layout.FluxIntegrated)

	// Create a simple cluster with a bundle
	bundle := &stack.Bundle{
		Name: "test-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	node := &stack.Node{
		Name:   "test-node",
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	// Create a matching layout
	ml := &layout.ManifestLayout{
		Name:      "test-node",
		Namespace: "clusters/test-cluster",
	}

	rules := layout.DefaultLayoutRules()
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify flux resources were added
	if len(ml.Resources) == 0 {
		t.Error("expected Flux resources to be added to layout")
	}
}

func TestLayoutIntegrator_IntegrateWithLayout_Separate(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)
	integrator.SetFluxPlacement(layout.FluxSeparate)

	// Create a cluster with bundles
	bundle := &stack.Bundle{
		Name: "test-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	node := &stack.Node{
		Name:   "test-node",
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	// Create a layout
	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "clusters",
	}

	rules := layout.DefaultLayoutRules()
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify a flux-system child was added
	found := false
	for _, child := range ml.Children {
		if child.Name == "flux-system" {
			found = true
			if len(child.Resources) == 0 {
				t.Error("expected Flux resources in flux-system child")
			}
			break
		}
	}
	if !found {
		t.Error("expected flux-system child to be added")
	}
}

func TestLayoutIntegrator_CreateLayoutWithResources(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)

	// Test with nil cluster
	ml, err := integrator.CreateLayoutWithResources(nil, layout.LayoutRules{})
	if err != nil {
		t.Errorf("expected no error for nil cluster, got %v", err)
	}
	if ml != nil {
		t.Error("expected nil layout for nil cluster")
	}

	// Test with valid cluster
	bundle := &stack.Bundle{
		Name: "test-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	node := &stack.Node{
		Name:   "test-node",
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	rules := layout.DefaultLayoutRules()
	ml, err = integrator.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if ml == nil {
		t.Fatal("expected non-nil layout")
	}
}

func TestLayoutIntegrator_IntegrateWithNestedNodes(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)
	integrator.SetFluxPlacement(layout.FluxIntegrated)

	// Create a nested hierarchy
	grandchildBundle := &stack.Bundle{
		Name: "grandchild-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	grandchild := &stack.Node{
		Name:       "grandchild",
		ParentPath: "root/child",
		Bundle:     grandchildBundle,
	}

	childBundle := &stack.Bundle{
		Name: "child-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	child := &stack.Node{
		Name:       "child",
		ParentPath: "root",
		Bundle:     childBundle,
		Children:   []*stack.Node{grandchild},
	}

	rootBundle := &stack.Bundle{
		Name: "root-bundle",
		SourceRef: &stack.SourceRef{
			Kind:      "GitRepository",
			Name:      "test-source",
			Namespace: "flux-system",
		},
	}

	root := &stack.Node{
		Name:     "root",
		Bundle:   rootBundle,
		Children: []*stack.Node{child},
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: root,
	}

	// Create matching nested layout
	grandchildLayout := &layout.ManifestLayout{
		Name: "grandchild",
	}

	childLayout := &layout.ManifestLayout{
		Name:     "child",
		Children: []*layout.ManifestLayout{grandchildLayout},
	}

	rootLayout := &layout.ManifestLayout{
		Name:     "root",
		Children: []*layout.ManifestLayout{childLayout},
	}

	rules := layout.DefaultLayoutRules()
	err := integrator.IntegrateWithLayout(rootLayout, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify resources were added at all levels
	if len(rootLayout.Resources) == 0 {
		t.Error("expected resources in root layout")
	}
	if len(childLayout.Resources) == 0 {
		t.Error("expected resources in child layout")
	}
	if len(grandchildLayout.Resources) == 0 {
		t.Error("expected resources in grandchild layout")
	}
}

func TestLayoutIntegrator_SeparateMode_EmptyCluster(t *testing.T) {
	generator := fluxstack.NewResourceGenerator()
	integrator := fluxstack.NewLayoutIntegrator(generator)
	integrator.SetFluxPlacement(layout.FluxSeparate)

	// Create a cluster without bundles
	node := &stack.Node{
		Name: "test-node",
	}

	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	ml := &layout.ManifestLayout{
		Name:      "test-cluster",
		Namespace: "clusters",
	}

	rules := layout.DefaultLayoutRules()
	err := integrator.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// No flux-system child should be added if there are no resources
	for _, child := range ml.Children {
		if child.Name == "flux-system" {
			t.Error("did not expect flux-system child for cluster with no bundles")
		}
	}
}

func TestCreateLayoutWithResources_InvalidUmbrellaRejected(t *testing.T) {
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

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	if _, err := integrator.CreateLayoutWithResources(c, layout.LayoutRules{}); err == nil {
		t.Fatal("expected invalid umbrella cluster to be rejected by CreateLayoutWithResources")
	}
}

func TestCreateLayoutWithResources_UmbrellaIntegratedPlacement(t *testing.T) {
	// Integrated placement: the umbrella bundle's own Flux CR goes to the
	// node layout (existing behavior, referenced via the bundle-named
	// FluxIntegrated child reference), while each umbrella child's Flux CR
	// lands at the bundle layout (where the bundle-dir kustomization.yaml
	// references them via UmbrellaChild). Child sub-layouts carry no Flux CRs.
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "infra"},
			{Name: "services"},
		},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	integrator.SetFluxPlacement(layout.FluxIntegrated)

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Walk ml to find layouts: root -> apps -> platform
	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]

	// Node layout carries the umbrella's own Flux CR.
	nodeKustNames := map[string]bool{}
	for _, r := range nodeLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			nodeKustNames[k.Name] = true
		}
	}
	if !nodeKustNames["platform"] {
		t.Error("expected platform Kustomization at node layout")
	}
	if nodeKustNames["infra"] || nodeKustNames["services"] {
		t.Error("umbrella child CRs should not be at node layout")
	}

	if len(nodeLayout.Children) != 1 {
		t.Fatalf("expected 1 bundle child, got %d", len(nodeLayout.Children))
	}
	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "platform" {
		t.Fatalf("expected platform bundle, got %q", bundleLayout.Name)
	}

	// Bundle layout carries the umbrella children's Flux CRs.
	bundleKustNames := map[string]bool{}
	for _, r := range bundleLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			bundleKustNames[k.Name] = true
		}
	}
	for _, want := range []string{"infra", "services"} {
		if !bundleKustNames[want] {
			t.Errorf("missing umbrella child Kustomization %q at bundle layout", want)
		}
	}
	if bundleKustNames["platform"] {
		t.Error("umbrella self CR should NOT be at bundle layout (it lives at node layout)")
	}

	// Umbrella child sub-layouts should carry NO Flux CRs.
	for _, c := range bundleLayout.Children {
		if !c.UmbrellaChild {
			continue
		}
		for _, r := range c.Resources {
			if _, ok := r.(*kustv1.Kustomization); ok {
				t.Errorf("umbrella child %q contains Flux Kustomization CR — expected none", c.Name)
			}
		}
	}
}

func TestCreateLayoutWithResources_UmbrellaNodeOnlyPlacement(t *testing.T) {
	// In nodeOnly (GroupFlat) mode, the umbrella child Flux CRs should land
	// at the node layout directly (no intermediate bundle layer).
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "infra"},
		},
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	integrator.SetFluxPlacement(layout.FluxIntegrated)

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(ml.Children) != 1 {
		t.Fatalf("expected 1 node child, got %d", len(ml.Children))
	}
	nodeLayout := ml.Children[0]

	// Node layout should carry umbrella self + umbrella child CRs = 2 Kustomizations
	kustCount := 0
	childNames := map[string]bool{}
	for _, r := range nodeLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			kustCount++
			childNames[k.Name] = true
		}
	}
	if kustCount != 2 {
		t.Errorf("expected 2 Kustomizations at node layout, got %d", kustCount)
	}
	if !childNames["platform"] {
		t.Error("missing umbrella platform Kustomization")
	}
	if !childNames["infra"] {
		t.Error("missing umbrella child infra Kustomization")
	}
}

func TestCreateLayoutWithResources_UmbrellaNestedIntegratedPlacement(t *testing.T) {
	// Nested umbrellas: infra child has its own grandchild. The grandchild's
	// Flux CR should land at the infra umbrella child layout, not the
	// platform bundle layout or the grandchild sub-layout.
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
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	integrator.SetFluxPlacement(layout.FluxIntegrated)

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// ml -> apps(nodeLayout) -> platform(bundleLayout) -> infra(UmbrellaChild) -> networking(UmbrellaChild)
	// Platform's own CR lives at the node layout (via GenerateFromBundle).
	// The platform bundle layout carries only the direct umbrella children (infra).
	nodeLayout := ml.Children[0]
	nodeKustNames := map[string]bool{}
	for _, r := range nodeLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			nodeKustNames[k.Name] = true
		}
	}
	if !nodeKustNames["platform"] {
		t.Error("expected platform Kustomization at node layout")
	}

	bundleLayout := nodeLayout.Children[0]
	if bundleLayout.Name != "platform" {
		t.Fatalf("expected platform bundle layout, got %q", bundleLayout.Name)
	}
	bundleKustNames := map[string]bool{}
	for _, r := range bundleLayout.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			bundleKustNames[k.Name] = true
		}
	}
	if !bundleKustNames["infra"] {
		t.Error("expected infra Kustomization at platform bundle layout")
	}
	if bundleKustNames["platform"] {
		t.Error("umbrella self CR should NOT be at bundle layout")
	}
	if bundleKustNames["networking"] {
		t.Error("nested grandchild CR should NOT be at platform bundle layout; it belongs at infra")
	}

	// infra umbrella child layout should have the networking Kustomization = 1
	var infra *layout.ManifestLayout
	for _, c := range bundleLayout.Children {
		if c.UmbrellaChild && c.Name == "infra" {
			infra = c
		}
	}
	if infra == nil {
		t.Fatal("missing infra umbrella child layout")
	}
	infraCount := 0
	for _, r := range infra.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok {
			infraCount++
			if k.Name != "networking" {
				t.Errorf("unexpected Kustomization at infra layout: %q", k.Name)
			}
		}
	}
	if infraCount != 1 {
		t.Errorf("expected 1 Kustomization at infra layout, got %d", infraCount)
	}
}

func TestCreateLayoutWithResources_UmbrellaChildWithSource(t *testing.T) {
	// When an umbrella child has a SourceRef with URL, the Source CR should
	// be placed at the parent layout alongside the child Kustomization.
	umbrella := &stack.Bundle{
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
	}
	node := &stack.Node{Name: "apps", Bundle: umbrella}
	root := &stack.Node{Name: "root", Children: []*stack.Node{node}}
	node.SetParent(root)
	cluster := &stack.Cluster{Name: "demo", Node: root}

	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	integrator.SetFluxPlacement(layout.FluxIntegrated)

	ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	bundleLayout := ml.Children[0].Children[0]
	sawSource := false
	for _, r := range bundleLayout.Resources {
		if _, ok := r.(*sourcev1.GitRepository); ok {
			sawSource = true
		}
	}
	if !sawSource {
		t.Error("expected umbrella child's GitRepository at parent bundle layout")
	}
}
