package stack_test

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The tree example of the "Domain Model" concept page
// (site/content/concepts/domain-model.md) is generated from this function
// (scripts/gen-doc-examples.sh); the page's fluent-builder block is
// ExampleNewClusterBuilder.

// workload is the stack.ApplicationConfig the page's applications use: one
// ConfigMap named after the application.
type workload struct{}

func (workload) Generate(app *stack.Application) ([]*client.Object, error) {
	var obj client.Object = kubernetes.CreateConfigMap(app.Name, app.Namespace)
	return []*client.Object{&obj}, nil
}

var (
	certManagerConfig stack.ApplicationConfig = workload{}
	prometheusConfig  stack.ApplicationConfig = workload{}
	frontendConfig    stack.ApplicationConfig = workload{}
	apiConfig         stack.ApplicationConfig = workload{}
)

// printLayout prints the repository path of every directory in the layout.
func printLayout(ml *layout.ManifestLayout) {
	fmt.Println(ml.FullRepoPath())
	for _, child := range ml.Children {
		printLayout(child)
	}
}

func Example_domainModel() {
	cluster := stack.NewCluster("production", &stack.Node{
		Name: "production",
		Children: []*stack.Node{
			{Name: "infrastructure", Children: []*stack.Node{
				{Name: "cert-manager", Bundle: &stack.Bundle{Name: "cert-manager", Applications: []*stack.Application{
					stack.NewApplication("cert-manager", "cert-manager", certManagerConfig),
				}}},
				{Name: "monitoring", Bundle: &stack.Bundle{Name: "monitoring", Applications: []*stack.Application{
					stack.NewApplication("prometheus", "monitoring", prometheusConfig),
				}}},
			}},
			{Name: "applications", Bundle: &stack.Bundle{Name: "web-apps", Applications: []*stack.Application{
				stack.NewApplication("frontend", "web", frontendConfig),
				stack.NewApplication("api", "web", apiConfig),
			}}},
		},
	})

	rules := layout.DefaultLayoutRules()
	rules.BundleGrouping = layout.GroupByName
	rules.ApplicationGrouping = layout.GroupByName
	ml, err := layout.WalkCluster(cluster, rules)
	if err != nil {
		panic(err)
	}
	printLayout(ml)
	// Output:
	// production
	// production/infrastructure
	// production/infrastructure/cert-manager
	// production/infrastructure/cert-manager/cert-manager
	// production/infrastructure/cert-manager/cert-manager/cert-manager
	// production/infrastructure/monitoring
	// production/infrastructure/monitoring/monitoring
	// production/infrastructure/monitoring/monitoring/prometheus
	// production/applications
	// production/applications/web-apps
	// production/applications/web-apps/frontend
	// production/applications/web-apps/api
}
