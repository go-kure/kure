package fluxcd_test

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestFluxWorkflow_PerLayoutKustomizationName: under FluxIntegratedPerLayout a
// directory that renders no bundle gets a Kustomization of its own, named
// after the layout: an application directory by its name, a bundle-less node
// by its path. The name is checked where that Kustomization is created, so
// one Flux cannot reconcile is refused with the layout's path instead of
// being written.
func TestFluxWorkflow_PerLayoutKustomizationName(t *testing.T) {
	integrate := func(root *stack.Node, rules layout.LayoutRules) (*layout.ManifestLayout, error) {
		rules.FluxPlacement = layout.FluxIntegratedPerLayout
		return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
			CreateLayoutWithResources(&stack.Cluster{Name: "demo", Node: root}, rules)
	}
	appDir := func(app string) *stack.Node {
		node := &stack.Node{Name: "apps", Bundle: &stack.Bundle{
			Name:         "web",
			SourceRef:    testSR(),
			Applications: []*stack.Application{fakeUmbrellaApp(app, "cm")},
		}}
		root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
		node.SetParent(root)
		return root
	}
	byName := layout.LayoutRules{ApplicationGrouping: layout.GroupByName}

	t.Run("application directory", func(t *testing.T) {
		ml, err := integrate(appDir("my-app"), byName)
		if err != nil {
			t.Fatalf("a valid application name: got %v, want nil", err)
		}
		got := map[string]string{}
		collectKustPaths(ml, got)
		if _, ok := got["my-app"]; !ok {
			t.Fatalf("Kustomizations = %v, want one named after the application directory", got)
		}

		ml, err = integrate(appDir("My_App"), byName)
		if err == nil {
			t.Fatal("application directory My_App: generated, want its Kustomization name refused")
		}
		if ml != nil {
			t.Errorf("returned a layout next to the error, want nothing to write")
		}
		for _, want := range []string{"My_App'", `"My_App" is not a valid Flux Kustomization name`} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %v, want it to contain %q", err, want)
			}
		}
	})

	t.Run("bundle-less node", func(t *testing.T) {
		long := strings.Repeat("n", stack.KustomizationNameMaxLength)
		leaf := &stack.Node{Name: "web", Bundle: &stack.Bundle{
			Name:         "web",
			SourceRef:    testSR(),
			Applications: []*stack.Application{fakeUmbrellaApp("web-app", "cm")},
		}}
		group := &stack.Node{Name: long, Children: []*stack.Node{leaf}}
		root := &stack.Node{Name: "demo", Children: []*stack.Node{group}}
		leaf.SetParent(group)
		group.SetParent(root)

		ml, err := integrate(root, layout.LayoutRules{})
		if err == nil {
			t.Fatal("a node whose derived Kustomization name is over the limit: generated, want it refused")
		}
		if ml != nil {
			t.Errorf("returned a layout next to the error, want nothing to write")
		}
		for _, want := range []string{long + "'", "-node", "at most 63 characters"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error = %v, want it to contain %q", err, want)
			}
		}
	})
}
