package fluxcd_test

import (
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

func boolPtr(b bool) *bool { return &b }

// waitingUmbrella is an umbrella with two children and one health check of
// the caller's own.
func waitingUmbrella(wait *bool) *stack.Bundle {
	return &stack.Bundle{
		Name:      "platform",
		SourceRef: testSR(),
		Wait:      wait,
		Children: []*stack.Bundle{
			{
				Name:         "y-infra",
				SourceRef:    testSR(),
				Applications: []*stack.Application{fakeUmbrellaApp("infra-app", "cm-infra")},
			},
			{
				Name:         "y-services",
				SourceRef:    testSR(),
				Applications: []*stack.Application{fakeUmbrellaApp("services-app", "cm-services")},
			},
		},
		HealthChecks: []stack.HealthCheck{
			{APIVersion: "apps/v1", Kind: "Deployment", Name: "manual-check", Namespace: "default"},
		},
	}
}

// With Wait true Flux ignores spec.healthChecks, so the generator writes no
// entry for the children; the caller's own check is written as given.
func TestGenerateForBundle_UmbrellaWaitWritesNoChildHealthChecks(t *testing.T) {
	b := waitingUmbrella(boolPtr(true))
	objs, err := fluxstack.Engine().ResourceGen.GenerateForBundle(b, "demo/apps")
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
  wait: true
status: {}
`
	if string(got) != want {
		t.Errorf("Kustomization:\n%s\nwant:\n%s", got, want)
	}
}

// With Wait unset or false the entries are written as before: one per child,
// ahead of the caller's own.
func TestGenerateForBundle_UmbrellaWithoutWaitKeepsChildHealthChecks(t *testing.T) {
	for name, wait := range map[string]*bool{"unset": nil, "false": boolPtr(false)} {
		t.Run(name, func(t *testing.T) {
			b := waitingUmbrella(wait)
			objs, err := fluxstack.Engine().ResourceGen.GenerateForBundle(b, "demo/apps")
			if err != nil {
				t.Fatalf("GenerateForBundle: %v", err)
			}
			k := objs[0].(*kustv1.Kustomization)
			if k.Spec.Wait {
				t.Error("spec.wait is true, want false")
			}
			var got []string
			for _, hc := range k.Spec.HealthChecks {
				got = append(got, hc.Kind+"/"+hc.Namespace+"/"+hc.Name)
			}
			want := []string{
				"Kustomization/flux-system/y-infra",
				"Kustomization/flux-system/y-services",
				"Deployment/default/manual-check",
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Errorf("healthChecks = %v, want %v", got, want)
			}
		})
	}
}

// The child's name is checked with Wait true too: the child's own
// Kustomization is written with it.
func TestGenerateForBundle_UmbrellaWaitStillChecksChildNames(t *testing.T) {
	b := waitingUmbrella(boolPtr(true))
	b.Children[1].Name = strings.Repeat("a", 64)
	_, err := fluxstack.Engine().ResourceGen.GenerateForBundle(b, "demo/apps")
	if err == nil {
		t.Fatal("GenerateForBundle accepted a child name of 64 characters with Wait true")
	}
	if !strings.Contains(err.Error(), strings.Repeat("a", 64)) {
		t.Errorf("error does not name the child: %v", err)
	}
}

// In every placement the umbrella's Kustomization gets the entries without
// Wait and none with it.
func TestCreateLayoutWithResources_UmbrellaWaitPerPlacement(t *testing.T) {
	placements := []layout.FluxPlacement{
		layout.FluxSeparate, layout.FluxIntegratedPerBundle, layout.FluxIntegratedPerLayout,
	}
	for _, placement := range placements {
		for name, wait := range map[string]*bool{"unset": nil, "true": boolPtr(true)} {
			t.Run(string(placement)+"/wait-"+name, func(t *testing.T) {
				node := &stack.Node{Name: "apps", Bundle: waitingUmbrella(wait)}
				root := &stack.Node{Name: "demo", Children: []*stack.Node{node}}
				node.SetParent(root)
				cluster := &stack.Cluster{Name: "demo", Node: root}

				integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
				ml, err := integrator.CreateLayoutWithResources(cluster, layout.LayoutRules{
					BundleGrouping:      layout.GroupFlat,
					ApplicationGrouping: layout.GroupFlat,
					FluxPlacement:       placement,
				})
				if err != nil {
					t.Fatalf("CreateLayoutWithResources: %v", err)
				}
				platform := findKustomization(ml, "platform")
				if platform == nil {
					t.Fatal("no platform Kustomization in the layout")
				}
				var children []string
				user := 0
				for _, hc := range platform.Spec.HealthChecks {
					if hc.Kind == "Kustomization" {
						children = append(children, hc.Name)
					} else {
						user++
					}
				}
				if user != 1 {
					t.Errorf("caller's health checks = %d, want 1", user)
				}
				if wait != nil {
					if len(children) != 0 {
						t.Errorf("child health checks under wait: %v", children)
					}
					return
				}
				if strings.Join(children, ",") != "y-infra,y-services" {
					t.Errorf("child health checks = %v, want [y-infra y-services]", children)
				}
			})
		}
	}
}

// findKustomization is the Flux Kustomization named name anywhere in ml.
func findKustomization(ml *layout.ManifestLayout, name string) *kustv1.Kustomization {
	for _, r := range ml.Resources {
		if k, ok := r.(*kustv1.Kustomization); ok && k.Name == name {
			return k
		}
	}
	for _, c := range ml.Children {
		if k := findKustomization(c, name); k != nil {
			return k
		}
	}
	return nil
}
