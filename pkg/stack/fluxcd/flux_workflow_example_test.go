package fluxcd_test

import (
	"fmt"
	"os"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The Go blocks of the "Generating Flux Manifests" guide
// (site/content/guides/flux-workflow.md) are generated from these functions
// (scripts/gen-doc-examples.sh), except the two bootstrap blocks it shares
// with this package's README.

// workloadApp is the stack.ApplicationConfig the guide's applications use: a
// Deployment and a Service named after the application.
type workloadApp struct{}

func (workloadApp) Generate(app *stack.Application) ([]*client.Object, error) {
	var dep client.Object = kubernetes.CreateDeployment(app.Name, app.Namespace)
	var svc client.Object = kubernetes.CreateService(app.Name, app.Namespace)
	return []*client.Object{&dep, &svc}, nil
}

var (
	certManagerConfig stack.ApplicationConfig = workloadApp{}
	frontendConfig    stack.ApplicationConfig = workloadApp{}
	apiConfig         stack.ApplicationConfig = workloadApp{}
)

// productionCluster is the cluster Example_fluxWorkflowDefine builds, for the
// steps that start from it. Keep the two in step.
func productionCluster() *stack.Cluster {
	certManager, err := stack.NewBundle("cert-manager", []*stack.Application{
		stack.NewApplication("cert-manager", "cert-manager", certManagerConfig),
	}, nil)
	if err != nil {
		panic(err)
	}
	webTier, err := stack.NewBundle("web-tier", []*stack.Application{
		stack.NewApplication("frontend", "web", frontendConfig),
		stack.NewApplication("api-gateway", "web", apiConfig),
	}, nil)
	if err != nil {
		panic(err)
	}
	return stack.NewCluster("production", &stack.Node{
		Name: "production",
		Children: []*stack.Node{
			{Name: "infrastructure", Bundle: certManager},
			{Name: "applications", Bundle: webTier},
		},
	})
}

// productionLayout is the layout Example_fluxWorkflowLayout creates.
func productionLayout() stack.ManifestLayoutResult {
	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupByName,
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
		FilePer:             layout.FilePerResource,
		FluxPlacement:       layout.FluxSeparate,
	}
	ml, err := fluxcd.Engine().CreateLayoutWithResources(productionCluster(), rules)
	if err != nil {
		panic(err)
	}
	return ml
}

func Example_fluxWorkflowDefine() {
	certManager, err := stack.NewBundle("cert-manager", []*stack.Application{
		stack.NewApplication("cert-manager", "cert-manager", certManagerConfig),
	}, nil)
	if err != nil {
		panic(err)
	}
	webTier, err := stack.NewBundle("web-tier", []*stack.Application{
		stack.NewApplication("frontend", "web", frontendConfig),
		stack.NewApplication("api-gateway", "web", apiConfig),
	}, nil)
	if err != nil {
		panic(err)
	}

	cluster := stack.NewCluster("production", &stack.Node{
		Name: "production",
		Children: []*stack.Node{
			{Name: "infrastructure", Bundle: certManager},
			{Name: "applications", Bundle: webTier},
		},
	})
	for _, node := range cluster.Node.Children {
		fmt.Println(node.Name, node.Bundle.Name, len(node.Bundle.Applications))
	}
	// Output:
	// infrastructure cert-manager 1
	// applications web-tier 2
}

func Example_fluxWorkflowEngine() {
	engine := fluxcd.Engine()
	fmt.Println(engine.GetName())
	// Output: FluxCD Workflow Engine
}

func Example_fluxWorkflowLayout() {
	cluster := productionCluster()
	engine := fluxcd.Engine()

	// Define layout rules
	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupByName,
		BundleGrouping:      layout.GroupByName,
		ApplicationGrouping: layout.GroupByName,
		FilePer:             layout.FilePerResource,
		FluxPlacement:       layout.FluxSeparate, // Flux resources in separate tree
	}

	// Generate layout with Flux resources integrated
	ml, err := engine.CreateLayoutWithResources(cluster, rules)
	if err != nil {
		panic(err)
	}
	for _, child := range ml.(*layout.ManifestLayout).Children {
		fmt.Println(child.FullRepoPath())
	}
	// Output:
	// production/infrastructure
	// production/applications
	// production/flux-system
}

func Example_fluxWorkflowWrite() {
	ml := productionLayout()
	out, err := os.MkdirTemp("", "kure-flux-workflow")
	if err != nil {
		panic(err)
	}
	defer func() { _ = os.RemoveAll(out) }()

	err = layout.WriteManifest(out, layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
	if err != nil {
		panic(err)
	}
	printFiles(out)
	// Output:
	// clusters/production/applications/kustomization.yaml
	// clusters/production/applications/web-tier/api-gateway/kustomization.yaml
	// clusters/production/applications/web-tier/api-gateway/web-deployment-api-gateway.yaml
	// clusters/production/applications/web-tier/api-gateway/web-service-api-gateway.yaml
	// clusters/production/applications/web-tier/frontend/kustomization.yaml
	// clusters/production/applications/web-tier/frontend/web-deployment-frontend.yaml
	// clusters/production/applications/web-tier/frontend/web-service-frontend.yaml
	// clusters/production/applications/web-tier/kustomization.yaml
	// clusters/production/flux-system/flux-system-kustomization-cert-manager.yaml
	// clusters/production/flux-system/flux-system-kustomization-web-tier.yaml
	// clusters/production/flux-system/kustomization.yaml
	// clusters/production/infrastructure/cert-manager/cert-manager/cert-manager-deployment-cert-manager.yaml
	// clusters/production/infrastructure/cert-manager/cert-manager/cert-manager-service-cert-manager.yaml
	// clusters/production/infrastructure/cert-manager/cert-manager/kustomization.yaml
	// clusters/production/infrastructure/cert-manager/kustomization.yaml
	// clusters/production/infrastructure/kustomization.yaml
	// clusters/production/kustomization.yaml
}

func Example_fluxWorkflowUmbrella() {
	umbrella := &stack.Bundle{
		Name: "platform",
		Children: []*stack.Bundle{
			{Name: "platform-infra"},
			{Name: "platform-services"},
			{Name: "platform-apps"},
		},
	}

	objects, err := fluxcd.Engine().ResourceGen.GenerateForBundle(umbrella, "production/apps/platform")
	if err != nil {
		panic(err)
	}
	ks := objects[0].(*kustv1.Kustomization)
	for _, hc := range ks.Spec.HealthChecks {
		fmt.Println(hc.Kind, hc.Namespace, hc.Name)
	}
	// Output:
	// Kustomization flux-system platform-infra
	// Kustomization flux-system platform-services
	// Kustomization flux-system platform-apps
}

func Example_fluxWorkflowDependsOn() {
	preInstall := &layout.ManifestLayout{
		Name: "nginx-00-pre-install",
		// ...
	}
	hooks := &layout.ManifestLayout{
		Name:      "nginx-01-hooks",
		DependsOn: []string{"nginx-00-pre-install"},
		// ...
	}
	fmt.Println(hooks.Name, "after", hooks.DependsOn[0] == preInstall.Name)
	// Output: nginx-01-hooks after true
}

func Example_fluxWorkflowBootstrapNamespace() {
	engine := fluxcd.Engine()
	rootNode := &stack.Node{Name: "production"}
	bootstrapConfig := &stack.BootstrapConfig{Enabled: true}

	engine.GetBootstrapGenerator().DefaultNamespace = "custom-flux" // default: "flux-system"

	objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
	if err != nil {
		panic(err)
	}
	for _, obj := range objects {
		switch obj.GetObjectKind().GroupVersionKind().Kind {
		case "FluxInstance":
			fmt.Println("FluxInstance in", obj.GetNamespace())
		case "Namespace":
			fmt.Println("Namespace", obj.GetName())
		}
	}
	// Output:
	// Namespace flux-system
	// FluxInstance in custom-flux
}
