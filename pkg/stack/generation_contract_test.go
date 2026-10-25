package stack_test

import (
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	_ "github.com/go-kure/kure/pkg/stack/argocd"
	_ "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// The workflow half of the generation contract (stack.ApplicationConfig):
// every workflow entry point that renders applications walks the cluster
// once, so it calls Generate, Validate and AugmentLayout once per
// application. The walk half is pkg/stack/layout/generation_contract_test.go.
// Design: docs/history/20261025-DESIGN-generation-ownership.md
// (go-kure/kure#1067).

// countingConfig builds a fresh ConfigMap on every Generate call and counts
// the Generate, Validate and AugmentLayout calls it gets.
type countingConfig struct {
	generated, validated, augmented int
}

func (c *countingConfig) Validate() error {
	c.validated++
	return nil
}

func (c *countingConfig) Generate(*stack.Application) ([]*client.Object, error) {
	c.generated++
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName("chart")
	u.SetNamespace("default")
	var o client.Object = u
	return []*client.Object{&o}, nil
}

func (c *countingConfig) AugmentLayout(*layout.ManifestLayout) error {
	c.augmented++
	return nil
}

// TestGenerationContract_EntryPointsWalkOnce: one call to each workflow entry
// point generates, validates and augments each application once.
func TestGenerationContract_EntryPointsWalkOnce(t *testing.T) {
	entryPoints := map[string]func(stack.Workflow, *stack.Cluster) error{
		"GenerateFromCluster": func(wf stack.Workflow, c *stack.Cluster) error {
			_, err := wf.GenerateFromCluster(c, layout.DefaultLayoutRules())
			return err
		},
		"CreateLayoutWithResources": func(wf stack.Workflow, c *stack.Cluster) error {
			_, err := wf.CreateLayoutWithResources(c, layout.DefaultLayoutRules())
			return err
		},
	}
	for _, provider := range []string{"flux", "argocd"} {
		for name, call := range entryPoints {
			t.Run(provider+"/"+name, func(t *testing.T) {
				wf, err := stack.NewWorkflow(provider)
				if err != nil {
					t.Fatal(err)
				}
				cfg := &countingConfig{}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: &stack.Bundle{
					Name:         "platform",
					SourceRef:    &stack.SourceRef{Kind: "GitRepository", Name: "flux-system"},
					Applications: []*stack.Application{stack.NewApplication("chart", "default", cfg)},
				}}}
				if err := call(wf, c); err != nil {
					t.Fatal(err)
				}
				if cfg.generated != 1 || cfg.validated != 1 || cfg.augmented != 1 {
					t.Errorf("%d Generate, %d Validate and %d AugmentLayout calls, want 1 of each",
						cfg.generated, cfg.validated, cfg.augmented)
				}
			})
		}
	}
}
