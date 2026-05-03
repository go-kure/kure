package layout

import (
	"testing"

	kustomizev1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
)

// flattenFakeConfig is a minimal stack.ApplicationConfig for these tests.
type flattenFakeConfig struct {
	objs []*client.Object
}

func (f *flattenFakeConfig) Generate(*stack.Application) ([]*client.Object, error) {
	return f.objs, nil
}

func newFlattenTestCluster(t *testing.T) *stack.Cluster {
	t.Helper()
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("only", "ns", &flattenFakeConfig{objs: []*client.Object{&o}})
	bundle := &stack.Bundle{Name: "bundle", Applications: []*stack.Application{app}}
	root := &stack.Node{Name: "apps", Bundle: bundle}
	return &stack.Cluster{Name: "demo", Node: root}
}

func TestFlatten_DisabledIsNoOp(t *testing.T) {
	cluster := newFlattenTestCluster(t)
	rules := LayoutRules{
		ClusterName:         "arc-runners",
		BundleGrouping:      GroupFlat,
		ApplicationGrouping: GroupFlat,
	}
	ml, err := WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	if len(ml.Children) == 0 {
		t.Fatalf("expected children when flatten is disabled, got resources directly")
	}
	if len(ml.Resources) != 0 {
		t.Errorf("synthetic root should not have resources when flatten is disabled, got %d", len(ml.Resources))
	}
	if ml.flattenInfo != nil {
		t.Errorf("no flattenInfo expected when flag is off")
	}
}

func TestFlatten_EnabledCollapsesSingleTier(t *testing.T) {
	cluster := newFlattenTestCluster(t)
	rules := LayoutRules{
		ClusterName:         "arc-runners",
		BundleGrouping:      GroupFlat,
		ApplicationGrouping: GroupFlat,
		FlattenSingleTier:   true,
	}
	ml, err := WalkCluster(cluster, rules)
	if err != nil {
		t.Fatalf("WalkCluster: %v", err)
	}
	if len(ml.Children) != 0 {
		t.Errorf("expected no children after collapse, got %d", len(ml.Children))
	}
	if len(ml.Resources) != 1 {
		t.Errorf("expected child resources lifted to root, got %d", len(ml.Resources))
	}
	if ml.flattenInfo == nil {
		t.Fatal("expected flattenInfo populated after collapse")
	}
	if got := ml.FlattenInfoNodeAlias("apps"); got != ml {
		t.Errorf("expected node alias for 'apps' to point at root, got %p", got)
	}
	rewrites := ml.FlattenInfoPathRewrites()
	if rewrites["arc-runners/apps"] != "arc-runners" {
		t.Errorf("expected path rewrite arc-runners/apps -> arc-runners, got %v", rewrites)
	}
}

func TestFlatten_PropagatesExtraFilesAndCMGen(t *testing.T) {
	cluster := newFlattenTestCluster(t)
	rules := LayoutRules{
		ClusterName:         "arc-runners",
		BundleGrouping:      GroupFlat,
		ApplicationGrouping: GroupFlat,
		FlattenSingleTier:   true,
	}
	// Build the layout, then artificially attach extras to the child before
	// flattening to mimic an augmenter that ran during walk.
	cluster.Node.Bundle.Applications[0] = stack.NewApplication("only", "ns", &flattenFakeConfig{
		objs: cluster.Node.Bundle.Applications[0].Config.(*flattenFakeConfig).objs,
	})

	// Manual layout for direct helper test (bypasses WalkCluster's own augmenter machinery).
	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{{
			Name:      "apps",
			Namespace: "arc-runners/apps",
			ExtraFiles: []ExtraFile{
				{Name: "values.yaml", Content: []byte("a: b")},
			},
			ConfigMapGenerators: []ConfigMapGeneratorSpec{
				{Name: "vals", Files: []string{"values.yaml"}},
			},
		}},
	}
	flattenSingleTier(parent, cluster, rules)
	if len(parent.ExtraFiles) != 1 || parent.ExtraFiles[0].Name != "values.yaml" {
		t.Errorf("ExtraFiles not propagated: %+v", parent.ExtraFiles)
	}
	if len(parent.ConfigMapGenerators) != 1 || parent.ConfigMapGenerators[0].Name != "vals" {
		t.Errorf("ConfigMapGenerators not propagated: %+v", parent.ConfigMapGenerators)
	}
}

func TestFlatten_NoCollapseWhenMultipleChildren(t *testing.T) {
	cluster := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps"}}
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{
			{Name: "a", Namespace: "arc-runners/a"},
			{Name: "b", Namespace: "arc-runners/b"},
		},
	}
	flattenSingleTier(parent, cluster, rules)
	if len(parent.Children) != 2 {
		t.Errorf("expected no collapse with multiple children")
	}
	if parent.flattenInfo != nil {
		t.Errorf("no flattenInfo expected when no collapse occurred")
	}
}

func TestFlatten_NoCollapseWhenUmbrellaChild(t *testing.T) {
	cluster := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps"}}
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{
			{Name: "apps", Namespace: "arc-runners/apps", UmbrellaChild: true},
		},
	}
	flattenSingleTier(parent, cluster, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse with umbrella child")
	}
}

func TestFlatten_NoCollapseWhenChildHasChildren(t *testing.T) {
	cluster := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps"}}
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{
			{
				Name:      "apps",
				Namespace: "arc-runners/apps",
				Children:  []*ManifestLayout{{Name: "deeper"}},
			},
		},
	}
	flattenSingleTier(parent, cluster, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse when child has its own children")
	}
}

func TestFlatten_NoCollapseWhenParentHasResources(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetKind("ConfigMap")
	cluster := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps"}}
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Resources: []client.Object{obj},
		Children:  []*ManifestLayout{{Name: "apps", Namespace: "arc-runners/apps"}},
	}
	flattenSingleTier(parent, cluster, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse when parent has its own Resources")
	}
}

func TestFlatten_NoCollapseWhenParentNamespaceHasSeparator(t *testing.T) {
	cluster := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "apps"}}
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "intermediate",
		Namespace: "arc-runners/intermediate",
		Children:  []*ManifestLayout{{Name: "apps", Namespace: "arc-runners/intermediate/apps"}},
	}
	flattenSingleTier(parent, cluster, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse when parent is not top-level")
	}
}

func TestFlatten_PackageWalkIsNoOp(t *testing.T) {
	cluster := newFlattenTestCluster(t)
	rules := LayoutRules{
		BundleGrouping:      GroupFlat,
		ApplicationGrouping: GroupFlat,
		FlattenSingleTier:   true,
	}
	packages, err := WalkClusterByPackage(cluster, rules)
	if err != nil {
		t.Fatalf("WalkClusterByPackage: %v", err)
	}
	for _, ml := range packages {
		if ml.flattenInfo != nil {
			t.Errorf("WalkClusterByPackage should not invoke flattenSingleTier; got flattenInfo on a package layout")
		}
	}
}

func TestApplyFlattenPathRewrites(t *testing.T) {
	kust := &kustomizev1.Kustomization{}
	kust.Spec.Path = "arc-runners/apps"

	deepKust := &kustomizev1.Kustomization{}
	deepKust.Spec.Path = "arc-runners/apps/sub"

	unrelated := &kustomizev1.Kustomization{}
	unrelated.Spec.Path = "other/path"

	root := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Resources: []client.Object{kust, unrelated},
		flattenInfo: &flattenInfo{
			pathRewrites: map[string]string{"arc-runners/apps": "arc-runners"},
		},
		Children: []*ManifestLayout{
			{
				Name:      "flux-system",
				Namespace: "arc-runners/flux-system",
				Resources: []client.Object{deepKust},
			},
		},
	}

	ApplyFlattenPathRewrites(root)

	if kust.Spec.Path != "arc-runners" {
		t.Errorf("exact-match rewrite failed: got %q, want %q", kust.Spec.Path, "arc-runners")
	}
	if deepKust.Spec.Path != "arc-runners/sub" {
		t.Errorf("prefix-match rewrite failed: got %q, want %q", deepKust.Spec.Path, "arc-runners/sub")
	}
	if unrelated.Spec.Path != "other/path" {
		t.Errorf("unrelated path should not be rewritten: got %q", unrelated.Spec.Path)
	}
	if root.flattenInfo == nil {
		t.Errorf("flattenInfo should remain populated after rewrite so subsequent integrator passes can resolve aliases")
	}
}

func TestApplyFlattenPathRewrites_NoOpWhenEmpty(t *testing.T) {
	root := &ManifestLayout{Name: "x", Namespace: "ns"}
	ApplyFlattenPathRewrites(root) // must not panic
}

func TestApplyFlattenPathRewrites_IsIdempotent(t *testing.T) {
	// After a rewrite pass leaves flattenInfo intact, a second pass on the
	// same layout must be a no-op (the already-rewritten Spec.Path no
	// longer matches the rewrite key).
	kust := &kustomizev1.Kustomization{}
	kust.Spec.Path = "arc-runners/apps"
	root := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Resources: []client.Object{kust},
		flattenInfo: &flattenInfo{
			pathRewrites: map[string]string{"arc-runners/apps": "arc-runners"},
		},
	}

	ApplyFlattenPathRewrites(root)
	if kust.Spec.Path != "arc-runners" {
		t.Fatalf("first pass: got %q, want %q", kust.Spec.Path, "arc-runners")
	}

	// Second pass: must not double-rewrite or panic.
	ApplyFlattenPathRewrites(root)
	if kust.Spec.Path != "arc-runners" {
		t.Errorf("second pass should be idempotent: got %q, want %q", kust.Spec.Path, "arc-runners")
	}
	if root.flattenInfo == nil {
		t.Errorf("flattenInfo must remain populated for repeated integration calls")
	}
}

func TestFindByNodeAlias(t *testing.T) {
	leaf := &ManifestLayout{Name: "leaf"}
	root := &ManifestLayout{
		Name: "",
		Children: []*ManifestLayout{
			{
				Name: "branch",
				Children: []*ManifestLayout{
					leaf,
				},
			},
		},
	}
	leaf.flattenInfo = &flattenInfo{
		nodeAliases: map[string]*ManifestLayout{"deep/path": leaf},
	}

	if got := FindByNodeAlias(root, "deep/path"); got != leaf {
		t.Errorf("expected alias to resolve to leaf, got %p", got)
	}
	if got := FindByNodeAlias(root, "missing"); got != nil {
		t.Errorf("expected nil for missing alias, got %p", got)
	}
}
