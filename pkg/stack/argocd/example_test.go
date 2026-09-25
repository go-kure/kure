package argocd_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/argocd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

// configMapApp is a minimal stack.ApplicationConfig: one ConfigMap named after
// the application.
type configMapApp struct{}

func (configMapApp) Generate(app *stack.Application) ([]*client.Object, error) {
	var obj client.Object = kubernetes.CreateConfigMap(app.Name, app.Namespace)
	return []*client.Object{&obj}, nil
}

// exampleCluster is the cluster the examples generate from: one node with one
// bundle holding one application.
func exampleCluster() *stack.Cluster {
	cluster, err := stack.NewClusterBuilder("prod").
		WithNode("apps").
		WithBundle("web").
		WithApplication("web", configMapApp{}).
		End().
		End().
		Build()
	if err != nil {
		panic(err)
	}
	return cluster
}

func Example() {
	cluster := exampleCluster()
	dir, err := os.MkdirTemp("", "kure-argocd-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(dir) }()

	// Use via the stack.Workflow registry
	wf, err := stack.NewWorkflow("argocd")
	if err != nil {
		panic(err)
	}
	ml, err := wf.CreateLayoutWithResources(cluster, layout.LayoutRules{})
	if err != nil {
		panic(err)
	}
	if err := ml.WriteToDisk(filepath.Join(dir, "clusters/prod")); err != nil {
		panic(err)
	}

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			fmt.Println(rel)
		}
		return err
	})
	if err != nil {
		panic(err)
	}
	// Output:
	// clusters/prod/apps/argocd/argocd-application-web.yaml
	// clusters/prod/apps/argocd/kustomization.yaml
	// clusters/prod/apps/cluster-configmap-web.yaml
	// clusters/prod/apps/kustomization.yaml
}

func ExampleEngine() {
	// Direct construction (bypasses registry)
	engine := argocd.Engine()

	// Configure source repository and namespace
	engine.SetRepoURL("https://github.com/example/manifests.git")
	engine.SetDefaultNamespace("argocd")
	fmt.Println(engine.GetName(), engine.RepoURL, engine.DefaultNamespace)
	// Output: ArgoCD Workflow Engine https://github.com/example/manifests.git argocd
}

func ExampleWorkflowEngine_GenerateFromCluster() {
	cluster := exampleCluster()
	engine := argocd.Engine()

	// Generate ArgoCD Applications from a cluster
	objects, err := engine.GenerateFromCluster(cluster)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		path, _, _ := unstructured.NestedString(obj.(*unstructured.Unstructured).Object, "spec", "source", "path")
		fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName(), path)
	}
	// Output: Application web apps
}

func ExampleWorkflowEngine_CreateLayoutWithResources() {
	cluster := exampleCluster()
	engine := argocd.Engine()

	// Create layout with Applications placed in an argocd/ subdirectory
	result, err := engine.CreateLayoutWithResources(cluster, layout.LayoutRules{})
	if err != nil {
		panic(err)
	}
	ml := result.(*layout.ManifestLayout)

	// Integrate Applications into an existing layout (a no-op for ArgoCD)
	if err := engine.IntegrateWithLayout(ml, cluster, layout.LayoutRules{}); err != nil {
		panic(err)
	}
	for _, child := range ml.Children {
		fmt.Println(child.FullRepoPath(), len(child.Resources))
	}
	// Output: apps/argocd 1
}
