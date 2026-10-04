package argocd

import (
	"io"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func TestEngine(t *testing.T) {
	engine := Engine()
	if engine == nil {
		t.Fatal("expected non-nil WorkflowEngine")
	}

	if engine.RepoURL != "https://github.com/example/manifests.git" {
		t.Errorf("expected default RepoURL 'https://github.com/example/manifests.git', got %s", engine.RepoURL)
	}

	if engine.DefaultNamespace != "argocd" {
		t.Errorf("expected default DefaultNamespace 'argocd', got %s", engine.DefaultNamespace)
	}
}

func TestWorkflowEngineInterface(t *testing.T) {
	var engine any = Engine()

	// Test that it implements stack.Workflow interface
	if _, ok := engine.(stack.Workflow); !ok {
		t.Error("WorkflowEngine should implement stack.Workflow interface")
	}
}

func TestGetName(t *testing.T) {
	engine := Engine()
	name := engine.GetName()

	if name != "ArgoCD Workflow Engine" {
		t.Errorf("expected name 'ArgoCD Workflow Engine', got %s", name)
	}
}

func TestGetVersion(t *testing.T) {
	engine := Engine()
	version := engine.GetVersion()

	if version != "v1.0.0" {
		t.Errorf("expected version 'v1.0.0', got %s", version)
	}
}

func TestSetRepoURL(t *testing.T) {
	engine := Engine()
	newURL := "https://github.com/test/repo.git"

	engine.SetRepoURL(newURL)

	if engine.RepoURL != newURL {
		t.Errorf("expected RepoURL '%s', got %s", newURL, engine.RepoURL)
	}
}

func TestSetDefaultNamespace(t *testing.T) {
	engine := Engine()
	newNamespace := "custom-argocd"

	engine.SetDefaultNamespace(newNamespace)

	if engine.DefaultNamespace != newNamespace {
		t.Errorf("expected DefaultNamespace '%s', got %s", newNamespace, engine.DefaultNamespace)
	}
}

func TestSupportedBootstrapModes(t *testing.T) {
	engine := Engine()
	modes := engine.SupportedBootstrapModes()

	if modes != nil {
		t.Errorf("expected nil bootstrap modes (not implemented), got %v", modes)
	}
}

func TestGenerateFromCluster_NilCluster(t *testing.T) {
	engine := Engine()

	objs, err := engine.GenerateFromCluster(nil, layout.DefaultLayoutRules())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for nil cluster")
	}
}

func TestGenerateFromCluster_NilNode(t *testing.T) {
	engine := Engine()
	cluster := &stack.Cluster{Node: nil}

	objs, err := engine.GenerateFromCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for cluster with nil node")
	}
}

func TestGenerateFromCluster_Success(t *testing.T) {
	engine := Engine()

	bundle := &stack.Bundle{
		Name:   "test-bundle",
		Labels: map[string]string{"app": "test"},
	}

	node := &stack.Node{
		Bundle: bundle,
	}

	cluster := &stack.Cluster{
		Node: node,
	}

	objs, err := engine.GenerateFromCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
}

func TestGenerateFromLayout_NilInputs(t *testing.T) {
	engine := Engine()

	objs, err := engine.generateFromLayout(nil, &stack.Cluster{Node: &stack.Node{Name: "root"}})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for a nil layout")
	}
}

func TestGenerateFromCluster_WithBundle(t *testing.T) {
	engine := Engine()

	bundle := &stack.Bundle{
		Name: "test-bundle",
	}

	node := &stack.Node{
		Bundle: bundle,
	}

	objs, err := engine.GenerateFromCluster(&stack.Cluster{Node: node}, layout.DefaultLayoutRules())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(objs) != 1 {
		t.Errorf("expected 1 object, got %d", len(objs))
	}
}

func TestGenerateFromCluster_WithChildren(t *testing.T) {
	engine := Engine()

	childBundle1 := &stack.Bundle{Name: "child1"}
	childBundle2 := &stack.Bundle{Name: "child2"}

	child1 := &stack.Node{Name: "child1", Bundle: childBundle1}
	child2 := &stack.Node{Name: "child2", Bundle: childBundle2}

	parentBundle := &stack.Bundle{Name: "parent"}
	parent := &stack.Node{
		Bundle:   parentBundle,
		Children: []*stack.Node{child1, child2},
	}
	cluster := &stack.Cluster{Node: parent}

	objs, err := engine.GenerateFromCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// One Application per bundle the default-rules walk renders.
	ml, err := layout.WalkCluster(cluster, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatal(err)
	}
	ix, err := layout.IndexOrigins(ml, cluster)
	if err != nil {
		t.Fatal(err)
	}
	if want := len(ix.Bundles()); len(objs) != want || want != 3 {
		t.Errorf("expected %d objects (parent + 2 children), got %d", want, len(objs))
	}
}

func TestApplicationForBundle_Success(t *testing.T) {
	engine := Engine()
	engine.SetRepoURL("https://github.com/test/repo.git")
	engine.SetDefaultNamespace("test-namespace")

	bundle := &stack.Bundle{
		Name:   "test-bundle",
		Labels: map[string]string{"app": "test", "env": "dev"},
		DependsOn: []*stack.Bundle{
			{Name: "dependency1"},
			{Name: "dependency2"},
		},
	}

	obj, err := engine.applicationForBundle(bundle, "clusters/test-bundle")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify the generated Application
	app := obj.(*unstructured.Unstructured)

	if app.GetAPIVersion() != "argoproj.io/v1alpha1" {
		t.Errorf("expected APIVersion 'argoproj.io/v1alpha1', got %s", app.GetAPIVersion())
	}

	if app.GetKind() != "Application" {
		t.Errorf("expected Kind 'Application', got %s", app.GetKind())
	}

	if app.GetName() != "test-bundle" {
		t.Errorf("expected name 'test-bundle', got %s", app.GetName())
	}

	if app.GetNamespace() != "test-namespace" {
		t.Errorf("expected namespace 'test-namespace', got %s", app.GetNamespace())
	}

	// Check labels
	labels := app.GetLabels()
	if labels["app"] != "test" {
		t.Errorf("expected label app='test', got %s", labels["app"])
	}
	if labels["env"] != "dev" {
		t.Errorf("expected label env='dev', got %s", labels["env"])
	}

	// Check source configuration
	source, found, err := unstructured.NestedMap(app.Object, "spec", "source")
	if err != nil {
		t.Errorf("error getting source: %v", err)
	}
	if !found {
		t.Error("source not found in spec")
	}
	if source["repoURL"] != "https://github.com/test/repo.git" {
		t.Errorf("expected repoURL 'https://github.com/test/repo.git', got %s", source["repoURL"])
	}
	if source["path"] != "clusters/test-bundle" {
		t.Errorf("expected the caller's path 'clusters/test-bundle', got %s", source["path"])
	}

	// Check destination configuration
	dest, found, err := unstructured.NestedMap(app.Object, "spec", "destination")
	if err != nil {
		t.Errorf("error getting destination: %v", err)
	}
	if !found {
		t.Error("destination not found in spec")
	}
	if dest["server"] != "https://kubernetes.default.svc" {
		t.Errorf("expected server 'https://kubernetes.default.svc', got %s", dest["server"])
	}
	if dest["namespace"] != "default" {
		t.Errorf("expected namespace 'default', got %s", dest["namespace"])
	}

	// Check dependencies
	deps, found, err := unstructured.NestedStringSlice(app.Object, "spec", "dependencies")
	if err != nil {
		t.Errorf("error getting dependencies: %v", err)
	}
	if !found {
		t.Error("dependencies not found in spec")
	}
	expectedDeps := []string{"dependency1", "dependency2"}
	if len(deps) != len(expectedDeps) {
		t.Errorf("expected %d dependencies, got %d", len(expectedDeps), len(deps))
	}
	for i, expected := range expectedDeps {
		if deps[i] != expected {
			t.Errorf("expected dependency[%d] '%s', got '%s'", i, expected, deps[i])
		}
	}
}

func TestApplicationForBundle_NoDependencies(t *testing.T) {
	engine := Engine()

	bundle := &stack.Bundle{
		Name: "simple-bundle",
	}

	obj, err := engine.applicationForBundle(bundle, "simple-bundle")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	app := obj.(*unstructured.Unstructured)

	// Dependencies should not be set
	_, found, err := unstructured.NestedStringSlice(app.Object, "spec", "dependencies")
	if err != nil {
		t.Errorf("error checking dependencies: %v", err)
	}
	if found {
		t.Error("dependencies should not be found for bundle with no dependencies")
	}
}

func TestIntegrateWithLayout(t *testing.T) {
	engine := Engine()
	ml := &layout.ManifestLayout{}
	cluster := &stack.Cluster{}
	rules := layout.LayoutRules{}

	// Should return nil error as ArgoCD doesn't need layout integration
	err := engine.IntegrateWithLayout(ml, cluster, rules)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestCreateLayoutWithResources_Success(t *testing.T) {
	engine := Engine()

	bundle := &stack.Bundle{Name: "test-bundle"}
	node := &stack.Node{Bundle: bundle, Name: "test-node"}
	cluster := &stack.Cluster{
		Name: "test-cluster",
		Node: node,
	}

	rules := layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
	}

	result, err := engine.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	ml, ok := result.(*layout.ManifestLayout)
	if !ok {
		t.Error("expected ManifestLayout result")
		return
	}

	// The layout name comes from the node name
	if ml.Name != "test-node" {
		t.Errorf("expected layout name 'test-node', got %s", ml.Name)
	}

	// Two children: the directory of the root node's bundle, then the argocd
	// directory
	if len(ml.Children) != 2 {
		t.Errorf("expected 2 children, got %d", len(ml.Children))
		return
	}
	if ml.Children[0].Name != "test-bundle" || ml.OriginUnit() != ml.Children[0] {
		t.Errorf("expected the bundle's directory 'test-bundle' first, got %s", ml.Children[0].Name)
	}

	argoCDLayout := ml.Children[1]
	if argoCDLayout.Name != "argocd" {
		t.Errorf("expected child name 'argocd', got %s", argoCDLayout.Name)
	}

	if len(argoCDLayout.Resources) != 1 {
		t.Errorf("expected 1 resource in argocd layout, got %d", len(argoCDLayout.Resources))
	}
}

func TestGenerateBootstrap_Disabled(t *testing.T) {
	engine := Engine()
	config := &stack.BootstrapConfig{Enabled: false}
	node := &stack.Node{}

	objs, err := engine.GenerateBootstrap(config, node)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for disabled bootstrap")
	}
}

func TestGenerateBootstrap_NilConfig(t *testing.T) {
	engine := Engine()
	node := &stack.Node{}

	objs, err := engine.GenerateBootstrap(nil, node)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if objs != nil {
		t.Error("expected nil objects for nil config")
	}
}

func TestGenerateBootstrap_Enabled(t *testing.T) {
	engine := Engine()
	config := &stack.BootstrapConfig{Enabled: true}
	node := &stack.Node{}

	objs, err := engine.GenerateBootstrap(config, node)
	if err == nil {
		t.Error("expected error for unimplemented ArgoCD bootstrap")
	}
	if objs != nil {
		t.Errorf("expected nil objects for unimplemented bootstrap, got %d", len(objs))
	}
}

func TestWorkflowEngineImplementsInterfaces(t *testing.T) {
	engine := Engine()

	// Test type assertions for main interface
	var _ stack.Workflow = engine
}

func TestApplicationForBundle_ClientObjectInterface(t *testing.T) {
	engine := Engine()
	bundle := &stack.Bundle{Name: "test"}

	obj, err := engine.applicationForBundle(bundle, "test")
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	// Verify it implements client.Object
	if obj == nil {
		t.Error("object should not be nil")
		return
	}

	// Test client.Object methods
	if obj.GetName() != "test" {
		t.Errorf("expected name 'test', got %s", obj.GetName())
	}

	// Test that it's an unstructured object
	if unstructuredObj, ok := obj.(*unstructured.Unstructured); ok {
		if unstructuredObj.GetKind() != "Application" {
			t.Errorf("expected kind 'Application', got %s", unstructuredObj.GetKind())
		}
	} else {
		t.Error("expected object to be *unstructured.Unstructured")
	}

	// Test DeepCopyObject
	copied := obj.DeepCopyObject()
	if copied == nil {
		t.Error("DeepCopyObject should not return nil")
	}

	// Type assert back to client.Object to access GetName
	if copiedClientObj, ok := copied.(interface{ GetName() string }); ok {
		if copiedClientObj.GetName() != obj.GetName() {
			t.Error("copied object should have same name as original")
		}
	} else {
		t.Error("copied object should implement GetName method")
	}
}

func TestArgoWorkflowFactory_Init(t *testing.T) {
	// stack.NewWorkflow("argocd") invokes the factory registered in init(),
	// covering the lambda body "return Engine()".
	w, err := stack.NewWorkflow("argocd")
	if err != nil {
		t.Fatalf("NewWorkflow(argocd): %v", err)
	}
	if w == nil {
		t.Fatal("expected non-nil workflow")
	}
}

func TestCreateLayoutWithResources_BadRulesType(t *testing.T) {
	w := Engine()
	_, err := w.CreateLayoutWithResources(&stack.Cluster{}, badArgoRules{})
	if err == nil {
		t.Fatal("expected error for wrong rules type")
	}
}

func TestCreateLayoutWithResources_InvalidCluster(t *testing.T) {
	w := Engine()
	// A bundle with an empty Name fails Bundle.Validate → ValidateCluster returns error.
	invalidCluster := &stack.Cluster{
		Name: "test",
		Node: &stack.Node{
			Name:   "node",
			Bundle: &stack.Bundle{Name: ""},
		},
	}
	rules := layout.LayoutRules{
		NodeGrouping:  layout.GroupByName,
		FilePer:       layout.FilePerResource,
		FluxPlacement: layout.FluxSeparate,
	}
	_, err := w.CreateLayoutWithResources(invalidCluster, rules)
	if err == nil {
		t.Fatal("expected error from ValidateCluster for invalid bundle")
	}
}

// badArgoRules satisfies stack.LayoutRulesProvider but is not layout.LayoutRules.
type badArgoRules struct{}

func (badArgoRules) Validate() error { return nil }

func TestCreateLayoutWithResources_ArgoCDInsideRootDirectory(t *testing.T) {
	// The argocd layout is a child of the root layout, whose kustomization.yaml
	// references it as "- argocd": it must live in the root's own directory.
	for _, tc := range []struct {
		clusterName, wantRoot string
	}{
		{"", "test-node"},
		{"prod", "prod"}, // the cluster wrapper is the returned root
	} {
		cluster := &stack.Cluster{Name: "test-cluster", Node: &stack.Node{Name: "test-node", Bundle: &stack.Bundle{Name: "test-bundle"}}}
		rules := layout.LayoutRules{BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat, ClusterName: tc.clusterName}
		result, err := Engine().CreateLayoutWithResources(cluster, rules)
		if err != nil {
			t.Fatalf("ClusterName %q: %v", tc.clusterName, err)
		}
		ml := result.(*layout.ManifestLayout)
		if got := ml.FullRepoPath(); got != tc.wantRoot {
			t.Fatalf("ClusterName %q: root at %q, want %q", tc.clusterName, got, tc.wantRoot)
		}
		var argo *layout.ManifestLayout
		for _, c := range ml.Children {
			if c.Name == "argocd" {
				argo = c
			}
		}
		if argo == nil {
			t.Fatalf("ClusterName %q: no argocd child", tc.clusterName)
		}
		if got, want := argo.FullRepoPath(), tc.wantRoot+"/argocd"; got != want {
			t.Errorf("ClusterName %q: argocd at %q, want %q", tc.clusterName, got, want)
		}
	}
}

// appPaths maps every Application's name to its spec.source.path.
func appPaths(t *testing.T, objs []client.Object) map[string]string {
	t.Helper()
	out := map[string]string{}
	for _, o := range objs {
		u, ok := o.(*unstructured.Unstructured)
		if !ok {
			t.Fatalf("object %T is not unstructured", o)
		}
		p, _, err := unstructured.NestedString(u.Object, "spec", "source", "path")
		if err != nil {
			t.Fatal(err)
		}
		out[u.GetName()] = p
	}
	return out
}

func pathsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func argoTestCluster() *stack.Cluster {
	umbrella := &stack.Bundle{Name: "web-bundle", Children: []*stack.Bundle{{Name: "web-db"}}}
	web := &stack.Node{Name: "web", Bundle: umbrella}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform-bundle"}, Children: []*stack.Node{web}}}
}

// TestGenerateFromCluster_ApplicationPathIsLayoutDir: every Application's
// path is the directory the default-rules walk writes its bundle to, and
// umbrella children get Applications too.
func TestGenerateFromCluster_ApplicationPathIsLayoutDir(t *testing.T) {
	objs, err := Engine().GenerateFromCluster(argoTestCluster(), layout.DefaultLayoutRules())
	if err != nil {
		t.Fatal(err)
	}
	// The root node's bundle has a directory of its own, beside its child nodes.
	want := map[string]string{"platform-bundle": "platform/platform-bundle", "web-bundle": "platform/web", "web-db": "platform/web/web-db"}
	if got := appPaths(t, objs); !pathsEqual(got, want) {
		t.Errorf("Application paths = %v, want %v", got, want)
	}
}

// TestCreateLayoutWithResources_RefusesLayoutInTheArgoDirectory: the root
// node's bundle is rendered in a directory named after it, inside the root
// node's (go-kure/kure#979), and a child node in one named after the node.
// Named like the Applications' directory, either would share it with them, so
// the call refuses it instead of returning a tree the writers refuse.
func TestCreateLayoutWithResources_RefusesLayoutInTheArgoDirectory(t *testing.T) {
	named := func(bundle, node string) *stack.Cluster {
		c := argoTestCluster()
		c.Node.Bundle.Name = bundle
		c.Node.Children[0].Name = node
		return c
	}
	for name, tc := range map[string]struct {
		c    *stack.Cluster
		want string
	}{
		"root bundle":            {named("argocd", "web"), `bundle "argocd" is rendered to "platform/argocd", the directory the ArgoCD Applications are written to`},
		"root bundle, case only": {named("ArgoCD", "web"), `bundle "ArgoCD" is rendered to "platform/ArgoCD", the directory the ArgoCD Applications are written to`},
		"child node":             {named("platform-bundle", "argocd"), `node "argocd" is rendered to "platform/argocd", the directory the ArgoCD Applications are written to`},
		// A name that resolves to the same directory is the same directory.
		"child node, rooted name":    {named("platform-bundle", "/argocd"), `node "/argocd" is rendered to "platform/argocd", the directory the ArgoCD Applications are written to`},
		"child node, dot-slash name": {named("platform-bundle", "./argocd"), `node "./argocd" is rendered to "platform/argocd", the directory the ArgoCD Applications are written to`},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := Engine().CreateLayoutWithResources(tc.c, layout.DefaultLayoutRules()); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("got %v, want the refusal %q", err, tc.want)
			}
		})
	}
	// A rooted ClusterName that ends in the root node's name: the writers
	// resolve it under their output directory, and so does the refusal.
	rooted := layout.DefaultLayoutRules()
	rooted.ClusterName = "/platform"
	if _, err := Engine().CreateLayoutWithResources(named("platform-bundle", "argocd"), rooted); err == nil || !strings.Contains(err.Error(), `node "argocd" is rendered to`) {
		t.Errorf("ClusterName /platform: got %v, want the refusal of node %q", err, "argocd")
	}
	// Under a ClusterName the Applications' directory is beside the root
	// node's, so the same names are written.
	rules := layout.DefaultLayoutRules()
	rules.ClusterName = "prod"
	result, err := Engine().CreateLayoutWithResources(named("argocd", "web"), rules)
	if err != nil {
		t.Fatalf("ClusterName prod: %v", err)
	}
	if err := result.(*layout.ManifestLayout).WriteToDisk(t.TempDir()); err != nil {
		t.Errorf("ClusterName prod, WriteToDisk: %v", err)
	}
}

// TestCreateLayoutWithResources_UsesWalkedRules: the Applications point at the
// directories of the layout this call walked with the caller's rules, not at
// a second derivation.
func TestCreateLayoutWithResources_UsesWalkedRules(t *testing.T) {
	rules := layout.LayoutRules{BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName, ClusterName: "prod"}
	result, err := Engine().CreateLayoutWithResources(argoTestCluster(), rules)
	if err != nil {
		t.Fatal(err)
	}
	ml := result.(*layout.ManifestLayout)
	var argo *layout.ManifestLayout
	for _, c := range ml.Children {
		if c.Name == "argocd" {
			argo = c
		}
	}
	if argo == nil {
		t.Fatal("no argocd layout")
	}
	// With BundleGrouping by name the root bundle has its own directory too;
	// the ClusterName no longer flattens it into the root node's.
	want := map[string]string{"platform-bundle": "prod/platform/platform-bundle", "web-bundle": "prod/platform/web/web-bundle", "web-db": "prod/platform/web/web-bundle/web-db"}
	if got := appPaths(t, argo.Resources); !pathsEqual(got, want) {
		t.Errorf("Application paths = %v, want %v", got, want)
	}

	// No Flux CRs exist in an Argo layout to reference argocd/, so an
	// integrated Flux placement is refused.
	for _, p := range []layout.FluxPlacement{layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		rules.FluxPlacement = p
		if _, err := Engine().CreateLayoutWithResources(argoTestCluster(), rules); err == nil || !strings.Contains(err.Error(), "FluxPlacement") {
			t.Errorf("FluxPlacement %s: got %v, want a refusal", p, err)
		}
	}
}

// TestCreateLayoutWithResources_RefusesInvalidRules: the engine does not check
// the rules on its own; the walk's validation refuses an unknown option value
// and a ClusterName no writer would write, and no layout is returned
// (go-kure/kure#979).
func TestCreateLayoutWithResources_RefusesInvalidRules(t *testing.T) {
	cases := map[string]struct {
		rules layout.LayoutRules
		field string
	}{
		"node grouping":        {layout.LayoutRules{NodeGrouping: "nested"}, "NodeGrouping"},
		"bundle grouping":      {layout.LayoutRules{BundleGrouping: "nested"}, "BundleGrouping"},
		"application grouping": {layout.LayoutRules{ApplicationGrouping: "nested"}, "ApplicationGrouping"},
		"file per":             {layout.LayoutRules{FilePer: "namespace"}, "FilePer"},
		"flux placement":       {layout.LayoutRules{FluxPlacement: "inline"}, "FluxPlacement"},
		"file naming":          {layout.LayoutRules{FileNaming: "name-kind"}, "FileNaming"},
		"cluster name":         {layout.LayoutRules{ClusterName: "../prod"}, "ClusterName"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			result, err := Engine().CreateLayoutWithResources(argoTestCluster(), tc.rules)
			if err == nil || !strings.Contains(err.Error(), tc.field) {
				t.Fatalf("err = %v, want one naming %s", err, tc.field)
			}
			if result != nil {
				t.Error("a layout was returned with the error")
			}
		})
	}
}

// TestCreateLayoutWithResources_UmbrellaTreeWrites: an ArgoCD tree is not one
// Flux delivers, so nothing in it is marked with SetFluxBuild, and the
// writers' check for a directory no Flux Kustomization builds
// (go-kure/kure#977) does not apply: a tree with an umbrella child, which its
// parent's kustomization.yaml does not list, is written by every writer.
func TestCreateLayoutWithResources_UmbrellaTreeWrites(t *testing.T) {
	rules := layout.LayoutRules{BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName, ClusterName: "prod"}
	result, err := Engine().CreateLayoutWithResources(argoTestCluster(), rules)
	if err != nil {
		t.Fatal(err)
	}
	ml := result.(*layout.ManifestLayout)
	umbrellaChildren := 0
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l.FluxBuild() {
			t.Errorf("layout %q is marked as built by a Flux Kustomization", l.FullRepoPath())
		}
		if l.UmbrellaChild {
			umbrellaChildren++
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	if umbrellaChildren == 0 {
		t.Fatal("the tree has no umbrella child layout")
	}
	if err := ml.WriteToDisk(t.TempDir()); err != nil {
		t.Errorf("WriteToDisk: %v", err)
	}
	if err := ml.WriteToTar(io.Discard); err != nil {
		t.Errorf("WriteToTar: %v", err)
	}
	if err := layout.WriteManifest(t.TempDir(), layout.DefaultLayoutConfig(), ml); err != nil {
		t.Errorf("WriteManifest: %v", err)
	}
}

// TestGenerateFromCluster_Refusals: the walk's validation error and the
// layout index's refusal (two bundles with one name, the Application's
// identity) both reach the caller.
func TestGenerateFromCluster_Refusals(t *testing.T) {
	invalid := &stack.Cluster{Name: "c", Node: &stack.Node{Name: "n", Bundle: &stack.Bundle{Name: ""}}}
	if _, err := Engine().GenerateFromCluster(invalid, layout.DefaultLayoutRules()); err == nil {
		t.Error("GenerateFromCluster accepted a bundle without a name")
	}
	dupNames := func() *stack.Cluster {
		return &stack.Cluster{Name: "c", Node: &stack.Node{Name: "platform", Children: []*stack.Node{
			{Name: "a", Bundle: &stack.Bundle{Name: "web"}},
			{Name: "b", Bundle: &stack.Bundle{Name: "web"}},
		}}}
	}
	if _, err := Engine().GenerateFromCluster(dupNames(), layout.DefaultLayoutRules()); err == nil || !strings.Contains(err.Error(), `two bundles are named "web"`) {
		t.Errorf("GenerateFromCluster: got %v, want the duplicate-name refusal", err)
	}
	if _, err := Engine().CreateLayoutWithResources(dupNames(), layout.LayoutRules{}); err == nil || !strings.Contains(err.Error(), `two bundles are named "web"`) {
		t.Errorf("CreateLayoutWithResources: got %v, want the duplicate-name refusal", err)
	}
	if objs, err := Engine().generateFromLayout(&layout.ManifestLayout{Name: "platform", Namespace: "."}, dupNames()); err == nil || objs != nil {
		t.Errorf("generateFromLayout on a hand-built tree: got %v, %v; want a refusal", objs, err)
	}
}

// TestArgoWorkflow_LongBundleName pins that the 63-character limit of a Flux
// Kustomization name is not the model's: a bundle name between 64 and 253
// characters validates, and the ArgoCD workflow renders an Application named
// after it, for a node's bundle and for an umbrella child.
func TestArgoWorkflow_LongBundleName(t *testing.T) {
	for _, n := range []int{stack.KustomizationNameMaxLength + 1, 253} {
		long := strings.Repeat("a", n)
		childLong := strings.Repeat("b", n)
		build := func() *stack.Cluster {
			umbrella := &stack.Bundle{Name: "platform", Children: []*stack.Bundle{{Name: childLong}}}
			return &stack.Cluster{Name: "c", Node: &stack.Node{Name: "root", Children: []*stack.Node{
				{Name: "apps", Bundle: &stack.Bundle{Name: long}},
				{Name: "platform", Bundle: umbrella},
			}}}
		}
		applications := func(t *testing.T, objs []client.Object) map[string]bool {
			t.Helper()
			names := map[string]bool{}
			for _, o := range objs {
				if o.GetObjectKind().GroupVersionKind().Kind == "Application" {
					names[o.GetName()] = true
				}
			}
			return names
		}
		check := func(t *testing.T, names map[string]bool) {
			t.Helper()
			for _, want := range []string{long, childLong, "platform"} {
				if !names[want] {
					t.Errorf("no Application named after the %d-character bundle %q", len(want), want)
				}
			}
		}

		if err := stack.ValidateCluster(build()); err != nil {
			t.Fatalf("ValidateCluster() with %d-character bundle names = %v, want nil", n, err)
		}

		objs, err := Engine().GenerateFromCluster(build())
		if err != nil {
			t.Fatalf("GenerateFromCluster() with %d-character bundle names = %v, want nil", n, err)
		}
		check(t, applications(t, objs))

		result, err := Engine().CreateLayoutWithResources(build(), layout.LayoutRules{})
		if err != nil {
			t.Fatalf("CreateLayoutWithResources() with %d-character bundle names = %v, want nil", n, err)
		}
		var rendered []client.Object
		var collect func(l *layout.ManifestLayout)
		collect = func(l *layout.ManifestLayout) {
			rendered = append(rendered, l.Resources...)
			for _, c := range l.Children {
				collect(c)
			}
		}
		collect(result.(*layout.ManifestLayout))
		check(t, applications(t, rendered))
	}
}

// TestGenerateFromLayout_UmbrellaDependencyChain pins that ArgoCD, which has
// no umbrella health checks, accepts an acyclic dependency chain through an
// umbrella: c (an umbrella child of p) depends on b, and b on p.
func TestGenerateFromLayout_UmbrellaDependencyChain(t *testing.T) {
	p := &stack.Bundle{Name: "p"}
	c := &stack.Bundle{Name: "c"}
	p.Children = []*stack.Bundle{c}
	b := &stack.Bundle{Name: "b", DependsOn: []*stack.Bundle{p}}
	c.DependsOn = []*stack.Bundle{b}
	pn := &stack.Node{Name: "pn", Bundle: p}
	bn := &stack.Node{Name: "bn", Bundle: b}
	r := &stack.Node{Name: "r", Children: []*stack.Node{pn, bn}}
	pn.SetParent(r)
	bn.SetParent(r)
	cluster := &stack.Cluster{Name: "demo", Node: r}
	if _, err := Engine().GenerateFromCluster(cluster, layout.DefaultLayoutRules()); err != nil {
		t.Fatalf("acyclic ArgoCD dependencies refused: %v", err)
	}
}

// TestGenerateFromCluster_ArgoNamedDependenciesMatchValidation pins that
// ArgoCD, which emits no NamedDependsOn, is not refused for a cycle through
// them: reciprocal named dependencies are the Flux workflow's to check.
func TestGenerateFromCluster_ArgoNamedDependenciesMatchValidation(t *testing.T) {
	a := &stack.Bundle{Name: "a", NamedDependsOn: []string{"b"}}
	b := &stack.Bundle{Name: "b", NamedDependsOn: []string{"a"}}
	an := &stack.Node{Name: "an", Bundle: a}
	bn := &stack.Node{Name: "bn", Bundle: b}
	r := &stack.Node{Name: "r", Children: []*stack.Node{an, bn}}
	an.SetParent(r)
	bn.SetParent(r)
	if _, err := Engine().GenerateFromCluster(&stack.Cluster{Name: "demo", Node: r}, layout.DefaultLayoutRules()); err != nil {
		t.Fatalf("ArgoCD refused for named dependencies it does not emit: %v", err)
	}
}
