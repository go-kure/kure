package fluxcd_test

import (
	"errors"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for ResourceGenerator.AdditiveCELDependencyCheckOff: with it, a
// dependsOn whose readyExpr is exactly "true" waits only for the dependency
// to exist (go-kure/kure#1161).

// celGateOffGenerator is a generator with AdditiveCELDependencyCheckOff set
// to off.
func celGateOffGenerator(off bool) *fluxstack.ResourceGenerator {
	g := fluxstack.NewResourceGenerator()
	g.AdditiveCELDependencyCheckOff = off
	return g
}

// webCreatedByHandWritten is the tree of TestBuiltView_TheOnlyApplyCreatesAndBuilds
// with the bootstrap skipping web: hand-written, which the platform
// application emits with extra, builds web's directory without that skip, so
// only hand-written's apply creates web.
func webCreatedByHandWritten(t *testing.T, placement layout.FluxPlacement, hw *kustv1.Kustomization, extra ...client.Object) *stack.Cluster {
	t.Helper()
	const unskip = "- op: remove\n  path: /metadata/annotations/kustomize.toolkit.fluxcd.io~1ssa\n"
	hw.Spec.Path = webDirectory(t, placement)
	hw.Spec.Patches = []kustomize.Patch{{Patch: unskip, Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"}}}
	platform := srBundle("platform", objApp("platform-app", append([]client.Object{hw}, extra...)...))
	platform.Wait = boolPtr(false)
	c := noWaitCluster(platform, false)
	c.Node.Children[1].Bundle.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
	return c
}

// Only hand-written's apply creates web. With "true", a dependsOn on web
// waits for web to exist, which it does only once hand-written has applied:
// hand-written depending on web, directly or through h (which the platform
// application emits, so it exists without hand-written), never applies. The
// cycle runs through web's existence, and web is a member, so it is refused
// with the setting too; without it, through web's Ready, as before.
func TestReadyExprTrue_ExistenceOfAMemberOnlyItsDependentCreates(t *testing.T) {
	for name, deps := range map[string]func() (*kustv1.Kustomization, []client.Object){
		"hand-written depends on web": func() (*kustv1.Kustomization, []client.Object) {
			hw := handWritten("hand-written")
			hw.Spec.DependsOn = []kustv1.DependencyReference{{Name: "web", ReadyExpr: "true"}}
			return hw, nil
		},
		"hand-written depends on h, which depends on web": func() (*kustv1.Kustomization, []client.Object) {
			h := handWritten("h")
			h.Spec.DependsOn = []kustv1.DependencyReference{{Name: "web", ReadyExpr: "true"}}
			return handWritten("hand-written", "h"), []client.Object{h}
		},
	} {
		for _, off := range []bool{false, true} {
			for _, placement := range integratedPlacements {
				t.Run(name+"/"+map[bool]string{false: "gate unknown", true: "gate off"}[off]+"/"+string(placement), func(t *testing.T) {
					hw, extra := deps()
					c := webCreatedByHandWritten(t, placement, hw, extra...)
					_, err := fluxstack.NewLayoutIntegrator(celGateOffGenerator(off)).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
					if err == nil || !strings.Contains(err.Error(), "can never all become Ready") {
						t.Fatalf("got %v, want the cycle through web refused", err)
					}
					want, not := "ready flux-system/web", "exist flux-system/web"
					if off {
						want, not = not, want
					}
					if !strings.Contains(err.Error(), want) || strings.Contains(err.Error(), not) {
						t.Fatalf("got %v, want a chain through %q and not %q", err, want, not)
					}
				})
			}
		}
	}
}

// siblingsDependingOnA is r -> [a (bundle), b (bundle)], b depending on a.
func siblingsDependingOnA() *stack.Cluster {
	a := &stack.Node{Name: "a", Bundle: srBundle("a", cmApp("a-app"))}
	b := &stack.Node{Name: "b", Bundle: srBundle("b", cmApp("b-app"))}
	b.Bundle.DependsOn = []*stack.Bundle{a.Bundle}
	r := &stack.Node{Name: "r", Children: []*stack.Node{a, b}}
	a.SetParent(r)
	b.SetParent(r)
	return &stack.Cluster{Name: "demo", Node: r}
}

// The integration keeps a Kustomization a in place of its own, depending on
// b with "true", and b depends on a. Without the setting a waits for b's
// Ready, which waits for a: refused by the members' check. With it, a waits
// only for b to exist, which a's apply does not create: admitted, in every
// form a kept Kustomization can have.
func TestReadyExprTrue_KeptMember(t *testing.T) {
	for _, off := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			for _, form := range keptForms {
				t.Run(map[bool]string{false: "gate unknown", true: "gate off"}[off]+"/"+string(placement)+"/"+form, func(t *testing.T) {
					rules := propertyGroupings["nodeOnly"]
					rules.FluxPlacement = placement
					ml, c := keptIn(t, siblingsDependingOnA, rules, "a", form, func(k *kustv1.Kustomization) {
						k.Spec.DependsOn = []kustv1.DependencyReference{{Name: "b", ReadyExpr: "true"}}
					}, nil)
					err := fluxstack.NewLayoutIntegrator(celGateOffGenerator(off)).IntegrateWithLayout(ml, c, rules)
					if off {
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), "never") {
						t.Fatalf("got %v, want a reconcile-order refusal", err)
					}
				})
			}
		}
	}
}

// web depends on platform, whose terminal leaf hand-written depends on web
// with "true". Under UmbrellasDoNotWait web's dependsOn is written on
// hand-written. Without the setting hand-written waits for web's Ready and
// the rewrite closes a cycle (ErrUmbrellaDependency). With it, hand-written
// waits only for web to exist, which the Flux bootstrap creates: admitted. A
// v1beta2 dependsOn has no readyExpr (meta.NamespacedObjectReference), so in
// that form the entry waits for Ready and is refused with the setting too.
func TestReadyExprTrue_UmbrellaRewrite(t *testing.T) {
	for _, off := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			for _, form := range kustomizationForms {
				t.Run(map[bool]string{false: "gate unknown", true: "gate off"}[off]+"/"+string(placement)+"/"+form, func(t *testing.T) {
					hw := handWritten("hand-written", "web")
					hw.Spec.DependsOn[0].ReadyExpr = "true"
					g := noWaitGenerator()
					g.AdditiveCELDependencyCheckOff = off
					_, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(noWaitCluster(srBundle("platform", formApp("platform-app", hw, form)), true), withPlacement(nodeOnly, placement))
					if off && form != "v1beta2" {
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						return
					}
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) {
						t.Fatalf("got %v, want the rewrite refused", err)
					}
				})
			}
		}
	}
}

// platform's application emits a and b, each depending on the other with
// "true". Under UmbrellasDoNotWait platform's terminal leaves are the
// Kustomizations it applies that no other one it applies awaits. Without the
// setting each awaits the other's Ready, so platform has no terminal leaf
// (ErrUmbrellaDependency). With it each waits only for the other to exist,
// which platform's apply creates: both are terminal leaves, and the tree is
// admitted. In the v1beta2 form the entries have no readyExpr and are refused
// with the setting too.
func TestReadyExprTrue_UmbrellaTerminalLeaves(t *testing.T) {
	for _, off := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			for _, form := range kustomizationForms {
				t.Run(map[bool]string{false: "gate unknown", true: "gate off"}[off]+"/"+string(placement)+"/"+form, func(t *testing.T) {
					a, b := handWritten("a", "b"), handWritten("b", "a")
					a.Spec.DependsOn[0].ReadyExpr = "true"
					b.Spec.DependsOn[0].ReadyExpr = "true"
					platform := srBundle("platform", formApp("a-app", a, form), formApp("b-app", b, form))
					g := noWaitGenerator()
					g.AdditiveCELDependencyCheckOff = off
					_, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
					if off && form != "v1beta2" {
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						return
					}
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "no terminal leaf") {
						t.Fatalf("got %v, want platform refused for having no terminal leaf", err)
					}
				})
			}
		}
	}
}

// platform's application emits h, depending on platform with "true". Under
// UmbrellasDoNotWait platform does not wait, and the integration does not
// rewrite h. Without the setting h waits for platform's Ready, which comes
// before what platform applies is Ready: refused (ErrUmbrellaDependency).
// With it h waits only for platform to exist, which the Flux bootstrap
// creates and which platform's wait does not change: admitted.
func TestReadyExprTrue_UnplacedDependsOnUmbrella(t *testing.T) {
	for _, off := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			for _, form := range kustomizationForms {
				t.Run(map[bool]string{false: "gate unknown", true: "gate off"}[off]+"/"+string(placement)+"/"+form, func(t *testing.T) {
					h := handWritten("h", "platform")
					h.Spec.DependsOn[0].ReadyExpr = "true"
					g := noWaitGenerator()
					g.AdditiveCELDependencyCheckOff = off
					_, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(noWaitCluster(srBundle("platform", formApp("platform-app", h, form)), false), withPlacement(nodeOnly, placement))
					if off && form != "v1beta2" {
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						return
					}
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "did not place") {
						t.Fatalf("got %v, want h's dependsOn on platform refused", err)
					}
				})
			}
		}
	}
}

// The integration keeps web in place of its own, depending on the umbrella
// platform with "true". A kept Kustomization's dependsOn on an umbrella is not
// rewritten, so without the setting it is refused (ErrUmbrellaDependency).
// With it web waits only for platform to exist, which platform's wait does not
// change: kept as written, and admitted.
func TestReadyExprTrue_KeptDependsOnUmbrella(t *testing.T) {
	build := func() *stack.Cluster {
		return noWaitCluster(srBundle("platform", objApp("platform-app", handWritten("h"))), false)
	}
	for _, off := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			for _, form := range keptForms {
				t.Run(map[bool]string{false: "gate unknown", true: "gate off"}[off]+"/"+string(placement)+"/"+form, func(t *testing.T) {
					rules := withPlacement(nodeOnly, placement)
					ml, c := keptIn(t, build, rules, "web", form, func(k *kustv1.Kustomization) {
						k.Spec.DependsOn = []kustv1.DependencyReference{{Name: "platform", ReadyExpr: "true"}}
					}, nil)
					g := noWaitGenerator()
					g.AdditiveCELDependencyCheckOff = off
					err := fluxstack.NewLayoutIntegrator(g).IntegrateWithLayout(ml, c, rules)
					if off {
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						return
					}
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), "is kept from the tree") {
						t.Fatalf("got %v, want web's dependsOn on platform refused", err)
					}
				})
			}
		}
	}
}
