package fluxcd_test

import (
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestFluxWorkflow_BundleNameOverLimit: the 63-character limit is Flux's, so
// cluster validation accepts a longer bundle name and the Flux workflow
// refuses it where it builds the bundle's Kustomization. Every entry point
// that generates from a cluster or a layout refuses, for a node's bundle and
// for an umbrella descendant, names the bundle by its path, and returns
// nothing to write.
func TestFluxWorkflow_BundleNameOverLimit(t *testing.T) {
	long := strings.Repeat("a", stack.KustomizationNameMaxLength+1)
	bundle := func(name string, children ...*stack.Bundle) *stack.Bundle {
		return &stack.Bundle{
			Name:         name,
			SourceRef:    testSR(),
			Children:     children,
			Applications: []*stack.Application{fakeUmbrellaApp(name+"-app", "cm-"+name)},
		}
	}
	clusters := []struct {
		name     string
		bundle   func() *stack.Bundle
		wantPath string
	}{
		{"node bundle", func() *stack.Bundle { return bundle(long) }, "'" + long + "'"},
		{
			"umbrella child",
			func() *stack.Bundle { return bundle("platform", bundle("infra"), bundle(long)) },
			"'platform/" + long + "'",
		},
		{
			"umbrella grandchild",
			func() *stack.Bundle { return bundle("platform", bundle("infra", bundle(long))) },
			"'platform/infra/" + long + "'",
		},
	}
	cluster := func(b *stack.Bundle) *stack.Cluster {
		node := &stack.Node{Name: "apps", Bundle: b}
		root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
		node.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	entryPoints := []struct {
		name string
		run  func(c *stack.Cluster) (any, error)
	}{
		{"GenerateFromCluster", func(c *stack.Cluster) (any, error) {
			objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c)
			if objs == nil {
				return nil, err
			}
			return objs, err
		}},
		{"GenerateFromLayout", func(c *stack.Cluster) (any, error) {
			ml, err := layout.WalkCluster(c, layout.DefaultLayoutRules())
			if err != nil {
				t.Fatalf("WalkCluster refused a name only Flux limits: %v", err)
			}
			objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			if objs == nil {
				return nil, err
			}
			return objs, err
		}},
	}
	for _, p := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout} {
		entryPoints = append(entryPoints, struct {
			name string
			run  func(c *stack.Cluster) (any, error)
		}{"CreateLayoutWithResources " + string(p), func(c *stack.Cluster) (any, error) {
			ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, layout.LayoutRules{
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				FluxPlacement:       p,
			})
			if ml == nil {
				return nil, err
			}
			return ml, err
		}})
	}

	for _, tc := range clusters {
		for _, ep := range entryPoints {
			t.Run(tc.name+"/"+ep.name, func(t *testing.T) {
				c := cluster(tc.bundle())
				if err := stack.ValidateCluster(c); err != nil {
					t.Fatalf("ValidateCluster() = %v, want nil: the limit is not the model's", err)
				}
				got, err := ep.run(c)
				if err == nil {
					t.Fatal("generated without an error, want the over-limit refusal")
				}
				if got != nil {
					t.Errorf("returned %v next to the error, want nothing to write", got)
				}
				for _, want := range []string{tc.wantPath, "at most 63 characters"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error = %v, want it to contain %q", err, want)
					}
				}
			})
		}
	}
}

// TestGenerateForBundle_NamesUmbrellaChildByPath: GenerateForBundle builds one
// Kustomization and walks no cluster, so the path it reports is the one the
// bundle itself carries. An umbrella child whose parent is known is named
// with it.
func TestGenerateForBundle_NamesUmbrellaChildByPath(t *testing.T) {
	long := strings.Repeat("a", stack.KustomizationNameMaxLength+1)
	child := &stack.Bundle{Name: long}
	umbrella := &stack.Bundle{Name: "platform", Children: []*stack.Bundle{child}}
	umbrella.InitializeUmbrella()

	_, err := fluxstack.NewResourceGenerator().GenerateForBundle(child, "clusters/prod/platform")
	if err == nil {
		t.Fatal("GenerateForBundle() generated, want the over-limit refusal")
	}
	for _, want := range []string{"'platform/" + long + "'", "at most 63 characters"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("GenerateForBundle() = %v, want it to contain %q", err, want)
		}
	}
}

// TestGenerateForBundle_RefusesOverLimitReference: the Kustomization built for
// a bundle also names other bundles' Kustomizations: an umbrella's children as
// health checks, its DependsOn bundles as dependencies. GenerateForBundle
// builds neither of those, so nothing else would check their names: a
// reference to a name Flux cannot reconcile is refused like the bundle's own,
// with the referenced bundle's path, and nothing is returned.
func TestGenerateForBundle_RefusesOverLimitReference(t *testing.T) {
	long := strings.Repeat("a", stack.KustomizationNameMaxLength+1)
	cases := []struct {
		name     string
		bundle   func() *stack.Bundle
		wantPath string
	}{
		{
			"umbrella child",
			func() *stack.Bundle {
				return &stack.Bundle{Name: "platform", Children: []*stack.Bundle{{Name: "infra"}, {Name: long}}}
			},
			"'platform/" + long + "'",
		},
		{
			"dependency",
			func() *stack.Bundle {
				return &stack.Bundle{Name: "web", DependsOn: []*stack.Bundle{{Name: "db"}, {Name: long}}}
			},
			"'" + long + "'",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b := tc.bundle()
			if err := b.Validate(); err != nil {
				t.Fatalf("Validate() = %v, want nil: the limit is not the model's", err)
			}
			objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(b, "clusters/prod/"+b.Name)
			if err == nil {
				t.Fatal("GenerateForBundle() generated, want the over-limit refusal")
			}
			if objs != nil {
				t.Errorf("GenerateForBundle() returned %v next to the error, want nothing to write", objs)
			}
			for _, want := range []string{tc.wantPath, "at most 63 characters"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("GenerateForBundle() = %v, want it to contain %q", err, want)
				}
			}
		})
	}
}
