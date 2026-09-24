package layout

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sschema "k8s.io/apimachinery/pkg/runtime/schema"
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
			Namespace: "arc-runners",
			ExtraFiles: []ExtraFile{
				{Name: "values.yaml", Content: []byte("a: b")},
			},
			ConfigMapGenerators: []ConfigMapGeneratorSpec{
				{Name: "vals", Files: []string{"values.yaml"}},
			},
		}},
	}
	flattenSingleTier(parent, rules)
	if len(parent.ExtraFiles) != 1 || parent.ExtraFiles[0].Name != "values.yaml" {
		t.Errorf("ExtraFiles not propagated: %+v", parent.ExtraFiles)
	}
	if len(parent.ConfigMapGenerators) != 1 || parent.ConfigMapGenerators[0].Name != "vals" {
		t.Errorf("ConfigMapGenerators not propagated: %+v", parent.ConfigMapGenerators)
	}
}

// TestFlatten_HandBuiltStillCollapses: a hand-built tree carries no origins;
// the collapse itself does not depend on them.
func TestFlatten_HandBuiltStillCollapses(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cm")
	parent := &ManifestLayout{
		Namespace: "arc-runners",
		Children:  []*ManifestLayout{{Name: "apps", Namespace: "arc-runners", Resources: []client.Object{obj}}},
	}
	got := flattenSingleTier(parent, LayoutRules{FlattenSingleTier: true})
	if got != parent || len(parent.Children) != 0 || len(parent.Resources) != 1 {
		t.Fatalf("hand-built single tier not collapsed: children %d, resources %d", len(parent.Children), len(parent.Resources))
	}
	if len(parent.OriginNodes()) != 0 || len(parent.OriginBundles()) != 0 || parent.OriginApplication() != nil {
		t.Errorf("a hand-built collapse invented origins: %v %v %v", parent.OriginNodes(), parent.OriginBundles(), parent.OriginApplication())
	}
}

func TestFlatten_NoCollapseWhenMultipleChildren(t *testing.T) {
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{
			{Name: "a", Namespace: "arc-runners/a"},
			{Name: "b", Namespace: "arc-runners/b"},
		},
	}
	flattenSingleTier(parent, rules)
	if len(parent.Children) != 2 {
		t.Errorf("expected no collapse with multiple children")
	}
}

func TestFlatten_NoCollapseWhenUmbrellaChild(t *testing.T) {
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{
			{Name: "apps", Namespace: "arc-runners/apps", UmbrellaChild: true},
		},
	}
	flattenSingleTier(parent, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse with umbrella child")
	}
}

func TestFlatten_NoCollapseWhenChildHasChildren(t *testing.T) {
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Children: []*ManifestLayout{
			{
				Name:      "apps",
				Namespace: "arc-runners",
				Children:  []*ManifestLayout{{Name: "deeper"}},
			},
		},
	}
	flattenSingleTier(parent, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse when child has its own children")
	}
}

func TestFlatten_NoCollapseWhenParentHasResources(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetKind("ConfigMap")
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Resources: []client.Object{obj},
		Children:  []*ManifestLayout{{Name: "apps", Namespace: "arc-runners/apps"}},
	}
	flattenSingleTier(parent, rules)
	if len(parent.Children) != 1 {
		t.Errorf("expected no collapse when parent has its own Resources")
	}
}

func TestFlatten_NoCollapseWhenParentNamespaceHasSeparator(t *testing.T) {
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "intermediate",
		Namespace: "arc-runners/intermediate",
		Children:  []*ManifestLayout{{Name: "apps", Namespace: "arc-runners/intermediate/apps"}},
	}
	flattenSingleTier(parent, rules)
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
	// The same cluster collapses under WalkCluster (TestFlatten_Enabled...):
	// the package walk must keep its single tier.
	ml := packages["default"]
	if ml == nil || len(ml.Children) != 0 || len(ml.Resources) != 1 || ml.FullRepoPath() != "apps" {
		t.Fatalf("package layout changed shape: %+v", ml)
	}
}

func TestCanFlatten_Nil(t *testing.T) {
	if canFlatten(nil) {
		t.Error("canFlatten(nil) should return false")
	}
}

func TestCanFlatten_NilChild(t *testing.T) {
	parent := &ManifestLayout{
		Name:      "",
		Namespace: "cluster",
		Children:  []*ManifestLayout{nil},
	}
	if canFlatten(parent) {
		t.Error("canFlatten with nil child should return false")
	}
}

func TestFlattenSingleTier_InheritsChildMode(t *testing.T) {
	// When parent has KustomizationUnset and child has a set Mode, the parent
	// should inherit the child's Mode after collapsing.
	rules := LayoutRules{FlattenSingleTier: true}

	parent := &ManifestLayout{
		Name:      "",
		Namespace: "arc-runners",
		Mode:      KustomizationUnset,
		Children: []*ManifestLayout{{
			Name:                "apps",
			Namespace:           "arc-runners",
			Mode:                KustomizationExplicit,
			FilePer:             FilePerResource,
			ApplicationFileMode: AppFileSingle,
			FileNaming:          FileNamingKindName,
		}},
	}
	flattenSingleTier(parent, rules)
	if parent.Mode != KustomizationExplicit {
		t.Errorf("expected Mode=KustomizationExplicit after inherit, got %v", parent.Mode)
	}
	if parent.FilePer != FilePerResource {
		t.Errorf("expected FilePer=FilePerResource after inherit, got %v", parent.FilePer)
	}
	if parent.ApplicationFileMode != AppFileSingle {
		t.Errorf("expected ApplicationFileMode=AppFileSingle after inherit, got %v", parent.ApplicationFileMode)
	}
	if parent.FileNaming != FileNamingKindName {
		t.Errorf("expected FileNaming=FileNamingKindName after inherit, got %v", parent.FileNaming)
	}
}

func TestCollectPackageRefs_NilNode(t *testing.T) {
	packages := make(map[string]*k8sschema.GroupVersionKind)
	// Must not panic or error when called with nil node
	collectPackageRefs(nil, nil, packages)
	if len(packages) != 0 {
		t.Errorf("expected empty map for nil node, got %v", packages)
	}
}

func TestWalkNodeForPackageInternal_Nil(t *testing.T) {
	got, err := walkNodeForPackageInternal(nil, nil, false, FilePerResource, nil, nil, "default", "")
	if err != nil {
		t.Fatalf("unexpected error for nil node: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nil node, got %v", got)
	}
}

func TestWalkNode_Nil(t *testing.T) {
	// walkNode(nil, ...) should return nil without error
	got, err := walkNode(nil, nil, false, false, FilePerResource, nil, FluxSeparate, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil for nil node, got %v", got)
	}
}

func TestWalkUmbrellaChildLayouts_NilChild(t *testing.T) {
	// Passing a nil child bundle in the slice should be silently skipped.
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName("cfg")
	obj.SetNamespace("default")
	var o client.Object = obj

	app := stack.NewApplication("app", "ns", &flattenFakeConfig{objs: []*client.Object{&o}})
	realChild := &stack.Bundle{Name: "real", Applications: []*stack.Application{app}}

	results, err := walkUmbrellaChildLayouts(
		[]*stack.Bundle{nil, realChild},
		[]string{"cluster"},
		FilePerResource,
		FluxSeparate,
		"",
	)
	if err != nil {
		t.Fatalf("unexpected error with nil child: %v", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result (nil child skipped), got %d", len(results))
	}
}
