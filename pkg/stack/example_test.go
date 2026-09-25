package stack_test

import (
	"fmt"
	"strconv"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd" // registers the "flux" workflow provider
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

// myConfig is the ApplicationConfig the README's "Optional Validation"
// section declares; the other examples use it too.
type myConfig struct{ Port int }

func (c *myConfig) Validate() error {
	if c.Port <= 0 {
		return errors.New("port must be positive")
	}
	return nil
}

func (c *myConfig) Generate(app *stack.Application) ([]*client.Object, error) {
	// Only called if Validate() passes (or is not implemented)
	cm := kubernetes.CreateConfigMap(app.Name, app.Namespace)
	cm.Data = map[string]string{"port": strconv.Itoa(c.Port)}
	var obj client.Object = cm
	return []*client.Object{&obj}, nil
}

func ExampleNewCluster() {
	rootNode := &stack.Node{Name: "flux-system"}

	cluster := stack.NewCluster("production", rootNode)
	cluster.SetGitOps(&stack.GitOpsConfig{
		Type: "flux",
	})
	fmt.Println(cluster.GetName(), cluster.GetNode().Name, cluster.GetGitOps().Type)
	// Output: production flux-system flux
}

func ExampleNode() {
	childNode := &stack.Node{Name: "cert-manager"}
	monitoringBundle := &stack.Bundle{Name: "monitoring"}

	node := &stack.Node{
		Name:     "infrastructure",
		Children: []*stack.Node{childNode},
		Bundle:   monitoringBundle,
	}
	fmt.Println(node.Name, node.Children[0].Name, node.Bundle.Name)
	// Output: infrastructure cert-manager monitoring
}

func ExampleNewBundle() {
	apps := []*stack.Application{
		stack.NewApplication("prometheus", "monitoring", &myConfig{Port: 9090}),
	}
	labels := map[string]string{"team": "platform"}
	certManagerBundle := &stack.Bundle{Name: "cert-manager"}

	bundle, err := stack.NewBundle("monitoring", apps, labels)
	if err != nil {
		panic(err)
	}
	// Pointer-based — when you hold the bundle object:
	bundle.DependsOn = []*stack.Bundle{certManagerBundle}
	// Name-based — when you only know the name (e.g. a hook-phase bundle):
	bundle.NamedDependsOn = []string{"cert-manager-pre-install"}
	bundle.Interval = "10m"

	if err := bundle.Validate(); err != nil {
		panic(err)
	}
	fmt.Println(bundle.Name, bundle.DependsOn[0].Name, bundle.NamedDependsOn, bundle.Interval)
	// Output: monitoring cert-manager [cert-manager-pre-install] 10m
}

func ExampleHealthCheck() {
	bundle := &stack.Bundle{Name: "web"}

	bundle.HealthChecks = []stack.HealthCheck{
		{APIVersion: "apps/v1", Kind: "Deployment", Name: "web", Namespace: "default"},
	}
	fmt.Println(bundle.HealthChecks[0].Kind, bundle.HealthChecks[0].Name)
	// Output: Deployment web
}

func ExampleApplication_Generate() {
	prometheusConfig := &myConfig{Port: 9090}

	app := stack.NewApplication("prometheus", "monitoring", prometheusConfig)
	resources, err := app.Generate()
	if err != nil {
		panic(err)
	}
	fmt.Println(len(resources), (*resources[0]).GetName())
	// Output: 1 prometheus
}

func ExampleNewClusterBuilder() {
	appConfig := &myConfig{Port: 9090}

	cluster, err := stack.NewClusterBuilder("production").
		WithNode("infrastructure").
		WithBundle("monitoring").
		WithApplication("prometheus", appConfig).
		End().
		End().
		Build()
	if err != nil {
		panic(err)
	}
	fmt.Println(cluster.Name, cluster.Node.Name, cluster.Node.Bundle.Name)
	// Output: production infrastructure monitoring
}

func ExampleNewWorkflow() {
	cluster, err := stack.NewClusterBuilder("production").
		WithNode("infrastructure").
		WithBundle("monitoring").
		WithApplication("prometheus", &myConfig{Port: 9090}).
		End().
		End().
		Build()
	if err != nil {
		panic(err)
	}

	// Create a workflow for your GitOps tool
	wf, err := stack.NewWorkflow("flux")
	if err != nil {
		panic(err)
	}

	// Generate GitOps resources from the cluster definition
	objects, err := wf.GenerateFromCluster(cluster)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		fmt.Println(obj.GetObjectKind().GroupVersionKind().Kind, obj.GetName())
	}
	// Output: Kustomization monitoring
}

func ExampleSourceRef() {
	// A bundle's SourceRef names the Flux source its Kustomization reads from;
	// with URL set, the generator creates that source as well.
	bundle := &stack.Bundle{Name: "apps"}
	bundle.SourceRef = &stack.SourceRef{
		Kind:      "OCIRepository",
		Name:      "my-registry",
		Namespace: "flux-system",
		URL:       "oci://registry.example.com/manifests",
		Tag:       "v1.0.0",
	}

	// A node's PackageRef names the package its subtree is bundled into;
	// child nodes without one inherit it.
	node := &stack.Node{Name: "apps", Bundle: bundle}
	node.SetPackageRef(&schema.GroupVersionKind{
		Group:   "source.toolkit.fluxcd.io",
		Version: "v1",
		Kind:    "OCIRepository",
	})
	fmt.Println(bundle.SourceRef.URL, node.GetPackageRef().Kind)
	// Output: oci://registry.example.com/manifests OCIRepository
}
