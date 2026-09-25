package layout_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

// exampleApp is a minimal stack.ApplicationConfig: one ConfigMap named after
// the application.
type exampleApp struct{}

func (exampleApp) Generate(app *stack.Application) ([]*client.Object, error) {
	var obj client.Object = kubernetes.CreateConfigMap(app.Name, app.Namespace)
	return []*client.Object{&obj}, nil
}

// exampleCluster is the cluster the examples walk: a root node "apps" whose
// bundle "web" holds the applications "api" and "ui".
func exampleCluster() *stack.Cluster {
	cluster, err := stack.NewClusterBuilder("prod").
		WithNode("apps").
		WithBundle("web").
		WithApplication("api", exampleApp{}).
		WithApplication("ui", exampleApp{}).
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

func ExampleLayoutRulesForPreset() {
	rules, err := layout.LayoutRulesForPreset(layout.PresetCentralizedControlPlane)
	if err != nil {
		panic(err)
	}
	cfg, err := layout.ConfigForPreset(layout.PresetCentralizedControlPlane)
	if err != nil {
		panic(err)
	}
	fmt.Println(rules.FluxPlacement, rules.NodeGrouping, rules.FileNaming, cfg.KustomizationFileName("web"))
	// Output: separate flat kind-name kustomization-web.yaml
}

func ExampleWalkCluster() {
	cluster := exampleCluster()
	out, err := os.MkdirTemp("", "kure-layout-example")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(out) }()

	// Create layout rules
	rules := layout.DefaultLayoutRules()
	rules.BundleGrouping = layout.GroupFlat
	rules.ApplicationGrouping = layout.GroupFlat

	// Walk cluster to create layout
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		panic(err)
	}

	// Write to disk
	cfg := layout.DefaultLayoutConfig()
	err = layout.WriteManifest(filepath.Join(out, "out/manifests"), cfg, ml)
	if err != nil {
		panic(err)
	}
	printFiles(out)
	// Output:
	// out/manifests/clusters/apps/cluster-configmap-api.yaml
	// out/manifests/clusters/apps/cluster-configmap-ui.yaml
	// out/manifests/clusters/apps/kustomization.yaml
}
