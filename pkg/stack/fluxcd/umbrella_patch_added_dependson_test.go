package fluxcd_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1221: under UmbrellasDoNotWait the pass rewrites a
// dependsOn on an umbrella from the entries the tree holds. A patch in the
// build that creates a Flux Kustomization can add an entry on an umbrella the
// tree does not hold; that entry is never rewritten, so its dependent waits
// for an umbrella that no longer waits.

// patchesS is a patch that sets the dependsOn of the Flux Kustomization named
// s to the entries in value (a YAML flow sequence).
func patchesS(value string) stack.Patch {
	return stack.Patch{
		Patch:  "- op: add\n  path: /spec/dependsOn\n  value: " + value + "\n",
		Target: &stack.PatchSelector{Kind: "Kustomization", Name: "s"},
	}
}

// patchedMemberCluster is noWaitCluster whose web bundle applies a Flux
// Kustomization s, the member, and carries patch. s is created by the build of
// web's Kustomization alone, which applies the patch; the tree holds s with no
// dependsOn.
func patchedMemberCluster(patch stack.Patch) *stack.Cluster {
	u := waitingUmbrella(nil)
	u.HealthChecks = nil
	c := noWaitCluster(u, false)
	for _, n := range c.Node.Children {
		if n.Name == "web" {
			n.Bundle = srBundle("web", objApp("web-app", handWritten("s")))
			n.Bundle.Patches = []stack.Patch{patch}
		}
	}
	return c
}

// wantRefused fails t unless err is the refusal of a class-above-plain dependsOn
// entry on an umbrella after the rewrite (check U1, checkNoWaitOnUmbrella).
func wantRefused(t *testing.T, err error) {
	t.Helper()
	wantRefusedWith(t, err, "a dependsOn that something other than the tree's own spec adds (a patch, say) is not rewritten")
}

// wantRefusedWith fails t unless err wraps ErrUmbrellaDependency and says text.
func wantRefusedWith(t *testing.T, err error, text string) {
	t.Helper()
	if !errors.Is(err, fluxstack.ErrUmbrellaDependency) || !strings.Contains(err.Error(), text) {
		t.Fatalf("got %v, want the tree refused as ErrUmbrellaDependency, saying %q", err, text)
	}
}

func TestUmbrellaPatchAddedDependsOn_ReadyEntryOnUmbrellaRefused(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(patchedMemberCluster(patchesS("[{name: platform}]")), rules)
			wantRefused(t, err)
		})
	}
}

// patchedChildCluster is noWaitCluster whose web bundle has one child bundle,
// s, so that s is a member (generated, placed in web's directory) that web's
// Kustomization builds, and carries patch. The umbrella platform has the
// leaves y-infra and y-services. With dependsOnU, s depends on platform, so
// the rewrite writes its dependsOn as [y-infra y-services] before web's build
// applies the patch.
func patchedChildCluster(dependsOnU bool, patch stack.Patch) *stack.Cluster {
	u := waitingUmbrella(nil)
	u.HealthChecks = nil
	c := noWaitCluster(u, false)
	for _, n := range c.Node.Children {
		if n.Name == "web" {
			s := srBundle("s", cmApp("s-app"))
			if dependsOnU {
				s.DependsOn = []*stack.Bundle{u}
			}
			n.Bundle = srBundle("web", cmApp("web-app"))
			n.Bundle.Children = []*stack.Bundle{s}
			n.Bundle.Patches = []stack.Patch{patch}
		}
	}
	return c
}

// A patch adds a Ready dependsOn on the umbrella to a member, a Kustomization
// the integration generated for a child bundle, that holds none.
func TestUmbrellaPatchAddedDependsOn_MemberRefused(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(patchedChildCluster(false, patchesS("[{name: platform}]")), rules)
			wantRefused(t, err)
		})
	}
}

// An entry whose readyExpr is "true" waits only for the umbrella to exist when
// the controller's additive CEL gate is off, and then the build reads it as
// existence-only (class 0): the rewrite writes existence entries on the
// terminal leaves, so a patch-added one is admitted (T2, go-kure/kure#1226).
// With the setting on it is an ordinary readyExpr entry (class 1) on an
// umbrella that does not wait, which is refused (U1).
func TestUmbrellaPatchAddedDependsOn_ExistenceEntryOnPatchedTree(t *testing.T) {
	for _, off := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			t.Run(fmt.Sprintf("setting %v/%s", off, placement), func(t *testing.T) {
				g := noWaitGenerator()
				g.AdditiveCELDependencyCheckOff = off
				rules := withPlacement(nodeOnly, placement)
				_, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(patchedMemberCluster(patchesS(`[{name: platform, readyExpr: "true"}]`)), rules)
				if off {
					if err != nil {
						t.Fatalf("got %v, want the existence-only entry admitted", err)
					}
					return
				}
				wantRefused(t, err)
			})
		}
	}
}

// A patch that adds a dependsOn on a Kustomization that is no umbrella changes
// nothing the rewrite reads: the tree is admitted.
func TestUmbrellaPatchAddedDependsOn_LeafEntryAdmitted(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			if _, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(patchedMemberCluster(patchesS("[{name: y-infra}]")), rules); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// s depends on the umbrella platform, so the rewrite writes its dependsOn as
// [y-infra y-services] before the patch in web's build applies; a patch
// addressed by position then lands on the rewritten list. One that removes
// y-services, or replaces it with y-infra, leaves s without an entry on a leaf
// it must wait for, though it holds no entry on the umbrella: the tree is
// refused. A patch that touches only the first entry keeps both leaves, and
// the tree is admitted. (The held list is [platform], with no index 1, so a
// reader that applies the patch to it fails or reads nothing.)
func TestUmbrellaPatchAddedDependsOn_PositionAddressedPatchOnRewrittenList(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"removes the second leaf":    {"- op: remove\n  path: /spec/dependsOn/1\n", true},
		"replaces the second leaf":   {"- op: replace\n  path: /spec/dependsOn/1\n  value:\n    name: y-infra\n", true},
		"weakens the second leaf":    {"- op: add\n  path: /spec/dependsOn/1/readyExpr\n  value: \"true\"\n", true},
		"keeps both leaves, renames": {"- op: add\n  path: /spec/dependsOn/0/readyExpr\n  value: \"\"\n", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				patch := stack.Patch{Patch: tc.patch, Target: &stack.PatchSelector{Kind: "Kustomization", Name: "s"}}
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(patchedChildCluster(true, patch), rules)
				if tc.refused {
					wantRefusedWith(t, err, "a patch landing on the rewritten dependsOn drops it")
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// A name in a held dependsOn that the build of the Kustomization's writer
// substitutes (spec.postBuild) is the same hole without a patch: the tree
// holds "${UMBRELLA}", the controller applies "platform". The final spec is
// read after the substitution, so the entry is refused.
func TestUmbrellaPatchAddedDependsOn_SubstitutedNameRefused(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			u := waitingUmbrella(nil)
			u.HealthChecks = nil
			c := noWaitCluster(u, false)
			for _, n := range c.Node.Children {
				if n.Name == "web" {
					n.Bundle = srBundle("web", objApp("web-app", handWritten("s", "${UMBRELLA}")))
					n.Bundle.PostBuild = &stack.PostBuild{Substitute: map[string]string{"UMBRELLA": "platform"}}
				}
			}
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
			wantRefused(t, err)
		})
	}
}

// renamedCopyCluster is noWaitCluster (web depends on the umbrella platform
// when dependsOnU) with a node whose hand-written Kustomization builds the
// directory that holds web's, with the name prefix x- and patch, which targets
// web. That build writes x-platform and x-web, renamed copies of the umbrella
// and of web, beside the originals, which stay applied.
func renamedCopyCluster(t *testing.T, placement layout.FluxPlacement, dependsOnU bool, patch kustomize.Patch) (*stack.Cluster, layout.LayoutRules) {
	t.Helper()
	rules := withPlacement(nodeOnly, placement)
	build := func(path string) *stack.Cluster {
		u := waitingUmbrella(nil)
		u.HealthChecks = nil
		c := noWaitCluster(u, dependsOnU)
		if path != "" {
			k := handWritten("hand-written")
			k.Spec.Path, k.Spec.NamePrefix = path, "x-"
			k.Spec.Patches = []kustomize.Patch{patch}
			c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
		}
		return c
	}
	return build(hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "web")), rules
}

// A build writes x-platform, the umbrella platform under a new name, and a
// patch gives x-web a dependsOn on it. The umbrella's copy does not wait
// either, so x-web would become Ready before the children x-platform applies:
// the tree is refused, though the tree holds no Kustomization by either name.
func TestUmbrellaPatchAddedDependsOn_RenamedUmbrellaRefused(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			patch := kustomize.Patch{
				Patch:  "- op: add\n  path: /spec/dependsOn\n  value:\n  - name: x-platform\n",
				Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
			}
			c, rules := renamedCopyCluster(t, placement, false, patch)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
			wantRefusedWith(t, err, "an umbrella the tree does not hold")
		})
	}
}

// web depends on the umbrella platform, so the rewrite writes its dependsOn as
// [y-infra y-services]; a build that renames web to x-web copies that list, and
// a patch there removes y-services. The original web keeps both, but x-web
// would become Ready before y-services: the tree is refused.
func TestUmbrellaPatchAddedDependsOn_RenamedMemberLosingALeafRefused(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			patch := kustomize.Patch{
				Patch:  "- op: remove\n  path: /spec/dependsOn/1\n",
				Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
			}
			c, rules := renamedCopyCluster(t, placement, true, patch)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
			wantRefusedWith(t, err, "which the tree does not hold")
		})
	}
}

// The route through spec.components: web's Kustomization is kept from the tree
// with a component that patches its child s, which only web's build creates,
// to depend on the umbrella platform. A caller's Kustomization is the only one
// that can carry components (the integration generates none).
func TestUmbrellaPatchAddedDependsOn_ComponentRoute(t *testing.T) {
	const component = "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\npatches:\n- target:\n    kind: Kustomization\n    name: s\n  patch: |-\n    - op: add\n      path: /spec/dependsOn\n      value:\n      - name: %s\n"
	for _, tc := range []struct {
		name, dep string
		refused   bool
	}{
		{"umbrella", "platform", true},
		{"leaf", "y-infra", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(tc.name+"/"+string(placement), func(t *testing.T) {
				build := func() *stack.Cluster {
					u := waitingUmbrella(nil)
					u.HealthChecks = nil
					c := noWaitCluster(u, false)
					for _, n := range c.Node.Children {
						if n.Name == "web" {
							n.Bundle = srBundle("web", cmApp("web-app"))
							n.Bundle.Children = []*stack.Bundle{srBundle("s", cmApp("s-app"))}
						}
					}
					return c
				}
				rules := withPlacement(nodeOnly, placement)
				var dir string
				ml, c := keptIn(t, build, rules, "web", "typed", func(k *kustv1.Kustomization) {
					k.Spec.Components = []string{"extra"}
					k.Spec.Wait, k.Spec.HealthChecks = false, nil
					dir = k.Spec.Path
				}, nil)
				l := layoutAtPath(t, ml, dir)
				l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: "extra/kustomization.yaml", Content: []byte(fmt.Sprintf(component, tc.dep))})
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if tc.refused {
					wantRefused(t, err)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// A build that renames an object that is no Flux Kustomization is not refused
// by the umbrella tree's identity rule: only a Kustomization is.
func TestUmbrellaPatchAddedDependsOn_RenamedOtherObjectAdmitted(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				u := waitingUmbrella(nil)
				u.HealthChecks = nil
				c := noWaitCluster(u, true)
				if path != "" {
					k := handWritten("hand-written")
					k.Spec.Path, k.Spec.NamePrefix = path, "x-"
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
				}
				return c
			}
			// The directory web-app lives in holds only a ConfigMap.
			dir := layoutPathNamed(t, func() *stack.Cluster { return build("") }, rules, "web")
			if _, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(dir), rules); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// A renamed copy of a Kustomization on a tree with no umbrella is admitted as
// before: the identity rule applies to umbrella trees only. hand-written sits
// in the root directory, which the Flux bootstrap applies, so no Kustomization
// of the tree applies a Flux Kustomization.
func TestUmbrellaPatchAddedDependsOn_RenamedCopyOnATreeWithoutAnUmbrellaAdmitted(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func() *stack.Cluster { return noWaitCluster(srBundle("platform", cmApp("platform-app")), true) }
			dir := hostPathOf(t, build, rules, "web")
			c := build()
			ml := mustWalk(t, c, rules)
			k := handWritten("hand-written")
			k.Spec.Path, k.Spec.NamePrefix = dir, "x-"
			ml.Resources = append(ml.Resources, k)
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// As TestUmbrellaPatchAddedDependsOn_RenamedMemberLosingALeafRefused, but the
// build keeps web alone (it deletes the other Kustomizations of the directory),
// and a local-config Kustomization named x-web sits elsewhere in the tree,
// which kustomize drops. That namesake must not stand for the copy x-web the
// build makes: the tree does not hold that copy, so the tree is refused all
// the same.
func TestUmbrellaPatchAddedDependsOn_LocalConfigNamesakeDoesNotHideACopy(t *testing.T) {
	dropped := "true"
	namesakeDoesNotHideACopy(t, annotateLocalConfig(handWritten("x-web"), &dropped))
}

// As the local-config case, with an ordinary namesake that every apply skips
// (kustomize.toolkit.fluxcd.io/ssa: Ignore). The rule is strict on purpose: a
// namesake that is no member never exempts the copy, so this is an
// over-refusal of a tree whose copy may well be skipped too. Telling the two
// apart would need the soundly tracked copy relation the rule refuses to
// build (go-kure/kure#1221).
func TestUmbrellaPatchAddedDependsOn_SkippedNamesakeDoesNotHideACopy(t *testing.T) {
	ns := handWritten("x-web")
	ns.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
	namesakeDoesNotHideACopy(t, ns)
}

// namesakeDoesNotHideACopy builds the tree of the two tests above around the
// non-member namesake and wants it refused for the copy x-web the build makes.
func namesakeDoesNotHideACopy(t *testing.T, namesake client.Object) {
	t.Helper()
	remove := func(name string) kustomize.Patch {
		return kustomize.Patch{Patch: "$patch: delete\napiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: " + name + "\n  namespace: flux-system\n"}
	}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				u := waitingUmbrella(nil)
				u.HealthChecks = nil
				c := noWaitCluster(u, true)
				other := &stack.Node{Name: "other", Bundle: srBundle("other", stack.NewApplication("namesake-app", "default", &fakeAppConfig{objs: []*client.Object{&namesake}}))}
				other.SetParent(c.Node)
				c.Node.Children = append(c.Node.Children, other)
				if path != "" {
					k := handWritten("hand-written")
					k.Spec.Path, k.Spec.NamePrefix = path, "x-"
					k.Spec.Patches = []kustomize.Patch{
						{Patch: "- op: remove\n  path: /spec/dependsOn/1\n", Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"}},
						remove("platform"), remove("patcher"), remove("other"),
					}
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
				}
				return c
			}
			dir := hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "web")
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(dir), rules)
			wantRefusedWith(t, err, "which the tree does not hold")
		})
	}
}
