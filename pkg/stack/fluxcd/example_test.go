package fluxcd_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/fluxcd"
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

// exampleCluster is the cluster the examples generate from: a root node
// "apps" whose bundle "web" holds one application.
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

// printFiles prints every file below dir, relative to it.
func printFiles(dir string) {
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			fmt.Println(rel)
		}
		return err
	})
	if err != nil {
		panic(err)
	}
}

func ExampleEngine() {
	cluster := exampleCluster()

	// Create engine with defaults. Placement is set on the LayoutRules passed
	// to the layout call, not on the engine — see Layout Integration below.
	engine := fluxcd.Engine()

	// Generate all Flux resources for a cluster (paths of a default-rules walk)
	objects, err := engine.GenerateFromCluster(cluster)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
	}
	// Output: Kustomization web
}

func ExampleNewWorkflowEngine() {
	// Default engine
	engine := fluxcd.Engine()

	// The same, built from its components
	built := fluxcd.NewWorkflowEngine()
	fmt.Println(engine.GetName() == built.GetName(), built.SupportedBootstrapModes())
	// Output: true [flux-operator gotk]
}

func ExampleResourceGenerator_GenerateFromLayout() {
	cluster := exampleCluster()
	engine := fluxcd.Engine()
	rules := layout.DefaultLayoutRules()
	bundle := cluster.Node.Bundle

	// From an entire cluster: walks it with layout.DefaultLayoutRules()
	objects, err := engine.GenerateFromCluster(cluster)
	if err != nil {
		panic(err)
	}
	fmt.Println(objects[0].GetName(), objects[0].(*kustv1.Kustomization).Spec.Path)

	// From a layout you walked (and will write) yourself
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		panic(err)
	}
	objects, err = engine.ResourceGen.GenerateFromLayout(ml, cluster)
	if err != nil {
		panic(err)
	}
	fmt.Println(objects[0].GetName(), objects[0].(*kustv1.Kustomization).Spec.Path)

	// For one bundle, at a path you supply
	objects, err = engine.ResourceGen.GenerateForBundle(bundle, "clusters/prod/apps")
	if err != nil {
		panic(err)
	}
	fmt.Println(objects[0].GetName(), objects[0].(*kustv1.Kustomization).Spec.Path)
	// Output:
	// web apps
	// web apps
	// web clusters/prod/apps
}

func ExampleWorkflowEngine_CreateLayoutWithResources() {
	cluster := exampleCluster()
	engine := fluxcd.Engine()
	rules := layout.DefaultLayoutRules()
	out, err := os.MkdirTemp("", "kure-fluxcd-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(out) }()

	// Create layout with Flux resources integrated
	ml, err := engine.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		panic(err)
	}

	// Write to disk: every spec.path is relative to <out>/clusters
	err = layout.WriteManifest(out, layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
	if err != nil {
		panic(err)
	}
	printFiles(out)
	// Output:
	// clusters/apps/cluster-configmap-web.yaml
	// clusters/apps/flux-system/flux-system-kustomization-web.yaml
	// clusters/apps/flux-system/kustomization.yaml
	// clusters/apps/kustomization.yaml
}

func ExampleWorkflowEngine_GenerateBootstrap() {
	engine := fluxcd.Engine()
	rootNode := &stack.Node{Name: "prod"}

	bootstrapConfig := &stack.BootstrapConfig{
		Enabled:     true,
		FluxMode:    "flux-operator", // or "gotk"; empty defaults to "flux-operator"
		FluxVersion: "v2.8.2",
		SourceURL:   "oci://registry.example.com/fleet",
		SourceRef:   "latest",
	}

	objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		if obj.GetObjectKind().GroupVersionKind().Kind == "FluxInstance" {
			fmt.Println(obj.GetNamespace(), obj.GetName())
		}
	}
	// Output: flux-system flux
}

func ExampleBootstrapGenerator_GenerateFluxInstance() {
	engine := fluxcd.Engine()
	rootNode := &stack.Node{Name: "prod"}

	bootstrapConfig := &stack.BootstrapConfig{
		Enabled:   true,
		SourceURL: "oci://registry.example.com/fleet",
		SourceRef: "latest",
		SyncName:  "fleet",
	}

	fi, err := engine.GetBootstrapGenerator().GenerateFluxInstance(bootstrapConfig, rootNode)
	if err != nil {
		panic(err)
	}
	fmt.Println(fi.Name, fi.Spec.Sync.Name, fi.Spec.Sync.Ref)
	// Output: flux fleet latest
}
