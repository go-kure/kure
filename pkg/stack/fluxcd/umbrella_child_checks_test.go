package fluxcd_test

import (
	"fmt"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// omittingGenerator is the default generator with OmitUmbrellaChildHealthChecks set.
func omittingGenerator() *fluxstack.ResourceGenerator {
	g := fluxstack.NewResourceGenerator()
	g.OmitUmbrellaChildHealthChecks = true
	return g
}

// waitingUmbrellaCluster is a cluster whose one node carries waitingUmbrella(wait).
func waitingUmbrellaCluster(wait *bool) *stack.Cluster {
	node := &stack.Node{Name: "apps", Bundle: waitingUmbrella(wait)}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
	node.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// healthCheckKeys lists k's health checks as kind/namespace/name.
func healthCheckKeys(k *kustv1.Kustomization) []string {
	var got []string
	for _, hc := range k.Spec.HealthChecks {
		got = append(got, hc.Kind+"/"+hc.Namespace+"/"+hc.Name)
	}
	return got
}

// renderTree is every resource of ml, in layout pre-order, as YAML under its
// layout's path.
func renderTree(t *testing.T, ml *layout.ManifestLayout) string {
	t.Helper()
	var b strings.Builder
	var walk func(*layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			out, err := yaml.Marshal(r)
			if err != nil {
				t.Fatalf("marshal %s: %v", r.GetName(), err)
			}
			fmt.Fprintf(&b, "# %s\n%s---\n", l.FullRepoPath(), out)
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return b.String()
}

// The case of go-kure/kure#1064: an umbrella under FluxSeparate with wait
// false and a health check of its own carries only that check.
func TestOmitUmbrellaChildHealthChecks_SeparateKeepsOnlyCallersCheck(t *testing.T) {
	for name, wait := range map[string]*bool{"unset": nil, "false": boolPtr(false)} {
		t.Run(name, func(t *testing.T) {
			integrator := fluxstack.NewLayoutIntegrator(omittingGenerator())
			ml, err := integrator.CreateLayoutWithResources(waitingUmbrellaCluster(wait), layout.LayoutRules{
				BundleGrouping:      layout.GroupFlat,
				ApplicationGrouping: layout.GroupFlat,
				FluxPlacement:       layout.FluxSeparate,
			})
			if err != nil {
				t.Fatalf("CreateLayoutWithResources: %v", err)
			}
			platform := findKustomization(ml, "platform")
			if platform == nil {
				t.Fatal("no platform Kustomization in the layout")
			}
			if platform.Spec.Wait {
				t.Error("spec.wait is true, want false")
			}
			got := healthCheckKeys(platform)
			if want := "Deployment/default/manual-check"; strings.Join(got, ",") != want {
				t.Errorf("healthChecks = %v, want [%s]", got, want)
			}
		})
	}
}

// GenerateForBundle follows the field too: the whole Kustomization is the one
// without it, less the child entries.
func TestOmitUmbrellaChildHealthChecks_GenerateForBundle(t *testing.T) {
	objs, err := omittingGenerator().GenerateForBundle(waitingUmbrella(nil), "demo/apps")
	if err != nil {
		t.Fatalf("GenerateForBundle: %v", err)
	}
	got, err := yaml.Marshal(objs[0])
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	want := `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: platform
  namespace: flux-system
spec:
  healthChecks:
  - apiVersion: apps/v1
    kind: Deployment
    name: manual-check
    namespace: default
  interval: 1h0m0s
  path: demo/apps
  prune: false
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
status: {}
`
	if string(got) != want {
		t.Errorf("Kustomization:\n%s\nwant:\n%s", got, want)
	}
}

// GenerateFromCluster, which FluxSeparate's list comes from, follows it as well.
func TestOmitUmbrellaChildHealthChecks_GenerateFromCluster(t *testing.T) {
	objs, err := omittingGenerator().GenerateFromCluster(waitingUmbrellaCluster(nil), layout.LayoutRules{
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupFlat,
		FluxPlacement:       layout.FluxSeparate,
	})
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	var platform *kustv1.Kustomization
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok && k.Name == "platform" {
			platform = k
		}
	}
	if platform == nil {
		t.Fatal("no platform Kustomization in the list")
	}
	got := healthCheckKeys(platform)
	if want := "Deployment/default/manual-check"; strings.Join(got, ",") != want {
		t.Errorf("healthChecks = %v, want [%s]", got, want)
	}
}

// In every placement and for every Wait the field removes the child entries
// and changes nothing else: the tree rendered with it is the tree rendered
// without it, once the umbrella's entries on its children are taken off. With
// Wait true there are none to take off, so the trees are equal as rendered.
func TestOmitUmbrellaChildHealthChecks_ChangesNothingElse(t *testing.T) {
	placements := []layout.FluxPlacement{
		layout.FluxSeparate, layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout,
	}
	rules := func(p layout.FluxPlacement) layout.LayoutRules {
		return layout.LayoutRules{
			BundleGrouping:      layout.GroupFlat,
			ApplicationGrouping: layout.GroupFlat,
			FluxPlacement:       p,
		}
	}
	children := map[string]bool{"y-infra": true, "y-services": true}
	for _, placement := range placements {
		for name, wait := range map[string]*bool{"unset": nil, "false": boolPtr(false), "true": boolPtr(true)} {
			t.Run(string(placement)+"/wait-"+name, func(t *testing.T) {
				without, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
					CreateLayoutWithResources(waitingUmbrellaCluster(wait), rules(placement))
				if err != nil {
					t.Fatalf("CreateLayoutWithResources without the field: %v", err)
				}
				with, err := fluxstack.NewLayoutIntegrator(omittingGenerator()).
					CreateLayoutWithResources(waitingUmbrellaCluster(wait), rules(placement))
				if err != nil {
					t.Fatalf("CreateLayoutWithResources with the field: %v", err)
				}
				platform := findKustomization(without, "platform")
				if platform == nil {
					t.Fatal("no platform Kustomization in the layout")
				}
				kept := platform.Spec.HealthChecks[:0:0]
				for _, hc := range platform.Spec.HealthChecks {
					if hc.Kind == "Kustomization" && children[hc.Name] {
						continue
					}
					kept = append(kept, hc)
				}
				dropped := len(platform.Spec.HealthChecks) - len(kept)
				if want := map[bool]int{true: 0, false: 2}[wait != nil && *wait]; dropped != want {
					t.Fatalf("without the field: %d child entries, want %d", dropped, want)
				}
				platform.Spec.HealthChecks = kept
				if got, want := renderTree(t, with), renderTree(t, without); got != want {
					t.Errorf("tree with the field:\n%s\nwant:\n%s", got, want)
				}
			})
		}
	}
}

// The children's names are checked with the field set: each child's own
// Kustomization is written with its name.
func TestOmitUmbrellaChildHealthChecks_StillChecksChildNames(t *testing.T) {
	b := waitingUmbrella(nil)
	b.Children[1].Name = strings.Repeat("a", 64)
	_, err := omittingGenerator().GenerateForBundle(b, "demo/apps")
	if err == nil {
		t.Fatal("GenerateForBundle accepted a child name of 64 characters")
	}
	if !strings.Contains(err.Error(), strings.Repeat("a", 64)) {
		t.Errorf("error does not name the child: %v", err)
	}
}
