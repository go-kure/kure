package fluxcd_test

import (
	"errors"
	"reflect"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1226: under UmbrellasDoNotWait the pass reads each
// dependsOn entry as the build of the tree shows it, with patches applied,
// before the rewrite, and compares it with the same build after it. The
// umbrella platform has the leaves y-infra and y-services; v and w are
// further leaf Kustomizations.

// patchOnS is a patch of the Flux Kustomization s.
func patchOnS(text string) stack.Patch {
	return stack.Patch{Patch: text, Target: &stack.PatchSelector{Kind: "Kustomization", Name: "s"}}
}

// leafNode adds a leaf node named name, a Kustomization with one ConfigMap,
// beside the nodes of c.
func leafNode(c *stack.Cluster, name string) *stack.Bundle {
	b := srBundle(name, cmApp(name+"-app"))
	n := &stack.Node{Name: name, Bundle: b}
	n.SetParent(c.Node)
	c.Node.Children = append(c.Node.Children, n)
	return b
}

// builtClassCluster is noWaitCluster with a member s, a child bundle of web
// that web's Kustomization builds, depending on the umbrella platform and then
// on the leaves named in leaves, in that order. patches are web's, so they
// apply to s in web's build, after the rewrite wrote its dependsOn as
// [y-infra y-services v ...].
func builtClassCluster(leaves []string, patches ...stack.Patch) *stack.Cluster {
	u := waitingUmbrella(nil)
	u.HealthChecks = nil
	c := noWaitCluster(u, false)
	s := srBundle("s", cmApp("s-app"))
	s.DependsOn = []*stack.Bundle{u}
	for _, l := range leaves {
		s.DependsOn = append(s.DependsOn, leafNode(c, l))
	}
	for _, n := range c.Node.Children {
		if n.Name == "web" {
			n.Bundle = srBundle("web", cmApp("web-app"))
			n.Bundle.Children = []*stack.Bundle{s}
			n.Bundle.Patches = patches
		}
	}
	return c
}

// A positional patch on the rewritten list. The build before the rewrite shows
// s as [platform v]; one that removes index 1 removes v there and y-services
// after the rewrite, [y-infra y-services v], so s loses a wait the rewrite
// owed it (U2). One that removes index 0 removes platform before the rewrite,
// which asked for no wait on it, and y-infra after: the tree is admitted.
func TestBuiltClasses_PositionPatchOnTheRewrittenList(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"removes index 1":       {"- op: remove\n  path: /spec/dependsOn/1\n", true},
		"removes index 0 (ctl)": {"- op: remove\n  path: /spec/dependsOn/0\n", false},
		"no patch (ctl)":        {"- op: add\n  path: /metadata/labels\n  value:\n    a: b\n", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(builtClassCluster([]string{"v"}, patchOnS(tc.patch)), rules)
				if tc.refused {
					wantRefusedWith(t, err, `does not wait for its terminal leaf "flux-system/y-services" after it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// s is [platform v w]. A patch replaces index 2 with v carrying a readyExpr:
// before the rewrite that replaces w, so s holds v plain and v with an
// expression; after it, [y-infra y-services v w], it replaces v, so v is left
// with an expression only. The plain wait on v is lost (the class set of an
// entry on v keeps class 2 or the tree is refused).
func TestBuiltClasses_MixedClassesLosePlainWait(t *testing.T) {
	patch := "- op: replace\n  path: /spec/dependsOn/2\n  value:\n    name: v\n    readyExpr: \"1 == 1\"\n"
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(builtClassCluster([]string{"v", "w"}, patchOnS(patch)), rules)
			wantRefusedWith(t, err, `waits for "flux-system/v" before the rewrite and not after it (class 2 required`)
		})
	}
}

// A readyExpr "true" on an umbrella is existence-only where the setting is on,
// and the tree holds a patch that never touches dependsOn: the pass admits it
// and leaves the entry as written, which waits for the umbrella to exist and
// needs no rewrite (it refused any patched tree, go-kure/kure#1225).
func TestBuiltClasses_ExistenceEntryOnPatchedTreeNotTouchingDependsOn(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			u := waitingUmbrella(nil)
			u.HealthChecks = nil
			c := noWaitCluster(u, false)
			s := handWritten("s")
			s.Spec.DependsOn = []kustv1.DependencyReference{{Name: "platform", ReadyExpr: "true"}}
			for _, n := range c.Node.Children {
				if n.Name == "web" {
					n.Bundle = srBundle("web", objApp("web-app", s))
					n.Bundle.Patches = []stack.Patch{patchOnS("- op: add\n  path: /metadata/labels\n  value:\n    a: b\n")}
				}
			}
			g := noWaitGenerator()
			g.AdditiveCELDependencyCheckOff = true
			ml, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
			if err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
			for _, k := range kustomizations(ml) {
				if k.Name != "s" {
					continue
				}
				if len(k.Spec.DependsOn) != 1 || k.Spec.DependsOn[0].Name != "platform" || k.Spec.DependsOn[0].ReadyExpr != "true" {
					t.Fatalf("s dependsOn = %+v, want the entry on platform kept as written", k.Spec.DependsOn)
				}
				return
			}
			t.Fatal("no Kustomization named s")
		})
	}
}

// health checks smuggled across the rewrite: a patch copies the name of
// dependsOn index 1 into a health check and drops dependsOn. Before the
// rewrite that is v, after it y-services: no dependsOn is left to compare, but
// s no longer health-checks what it did (U4: fields other than dependsOn stay
// equal).
func TestBuiltClasses_HealthCheckSmuggledByCopy(t *testing.T) {
	patch := "- op: add\n  path: /spec/healthChecks\n  value:\n  - apiVersion: v1\n    kind: ConfigMap\n    name: placeholder\n    namespace: default\n" +
		"- op: copy\n  from: /spec/dependsOn/1/name\n  path: /spec/healthChecks/0/name\n" +
		"- op: remove\n  path: /spec/dependsOn\n"
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(builtClassCluster([]string{"v"}, patchOnS(patch)), rules)
			wantRefusedWith(t, err, `has a different spec.healthChecks after the rewrite than before it`)
		})
	}
}

// A tree umbrella s applies two children and waits for neither; a patch gives
// it a health check or wait back. The rewrite cleared them, so the umbrella
// would wait for what its children apply (U4 umbrella clause).
func TestBuiltClasses_UmbrellaRegainsAWait(t *testing.T) {
	for name, tc := range map[string]struct{ patch, field string }{
		"health check": {"- op: add\n  path: /spec/healthChecks\n  value:\n  - apiVersion: v1\n    kind: ConfigMap\n    name: x\n    namespace: default\n", "spec.healthChecks"},
		"wait":         {"- op: add\n  path: /spec/wait\n  value: true\n", "spec.wait"},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				u := waitingUmbrella(nil)
				u.HealthChecks = nil
				c := noWaitCluster(u, false)
				s := srBundle("s", cmApp("s-app"))
				s.Children = []*stack.Bundle{srBundle("s1", cmApp("s1-app")), srBundle("s2", cmApp("s2-app"))}
				for _, n := range c.Node.Children {
					if n.Name == "web" {
						n.Bundle = srBundle("web", cmApp("web-app"))
						n.Bundle.Children = []*stack.Bundle{s}
						n.Bundle.Patches = []stack.Patch{patchOnS(tc.patch)}
					}
				}
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
				wantRefusedWith(t, err, "applies Flux Kustomizations and must not wait, and a build gives it "+tc.field)
			})
		}
	}
}

// renamedSCluster is builtClassCluster with the leaf v, plus a hand-written
// Kustomization that builds the directory holding s with the name prefix x-
// and, when patch is set, that patch on s. That build writes x-s, a renamed
// copy of s, which the tree does not hold; s keeps its own dependsOn. namesake,
// when set, is an object applied elsewhere in the tree.
func renamedSCluster(t *testing.T, placement layout.FluxPlacement, patch string, namesake client.Object) (*stack.Cluster, layout.LayoutRules) {
	t.Helper()
	rules := withPlacement(nodeOnly, placement)
	build := func(path string) *stack.Cluster {
		c := builtClassCluster([]string{"v"})
		if namesake != nil {
			other := &stack.Node{Name: "other", Bundle: srBundle("other", stack.NewApplication("namesake-app", "default", &fakeAppConfig{objs: []*client.Object{&namesake}}))}
			other.SetParent(c.Node)
			c.Node.Children = append(c.Node.Children, other)
		}
		if path != "" {
			k := handWritten("hand-written")
			k.Spec.Path, k.Spec.NamePrefix = path, "x-"
			if patch != "" {
				k.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "s"}}}
			}
			c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
		}
		return c
	}
	return build(hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "s")), rules
}

// x-s is a renamed copy of s that the tree does not hold. Before the rewrite it
// waits for [platform v] and the patch removes v; after it, [y-infra
// y-services v] and the same positional patch removes y-services. The copy
// loses a wait the rewrite owed it, so the tree is refused (U2). A removal at
// index 0 and no patch at all are controls and are admitted.
func TestBuiltClasses_RenamedMemberCopyLosingALeaf(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"removes index 1":       {"- op: remove\n  path: /spec/dependsOn/1\n", true},
		"removes index 0 (ctl)": {"- op: remove\n  path: /spec/dependsOn/0\n", false},
		"no patch (ctl)":        {"", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c, rules := renamedSCluster(t, placement, tc.patch, nil)
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
				if tc.refused {
					wantRefusedWith(t, err, `Flux Kustomization "flux-system/x-s" depends on "flux-system/platform", which applies Flux Kustomizations, before the rewrite and does not wait for its terminal leaf "flux-system/y-services" after it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// The renamed copy x-s of TestBuiltClasses_RenamedMemberCopyLosingALeaf, with
// an ordinary object named x-s applied elsewhere in the tree that the build of
// the tree does not make into the copy: a Kustomization that every apply skips
// (ssa: Ignore) or that kustomize drops (local config). A namesake that is no
// applied member never stands for the copy: the patch is refused all the same,
// and the namesake alone, and the patcher without the patch, are admitted.
func TestBuiltClasses_RenamedMemberCopyBesideANamesake(t *testing.T) {
	dropped := "true"
	skipped := handWritten("x-s")
	skipped.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
	namesakes := map[string]client.Object{
		"apply-skipped": skipped,
		"local-config":  annotateLocalConfig(handWritten("x-s"), &dropped),
	}
	const remove1 = "- op: remove\n  path: /spec/dependsOn/1\n"
	for name, ns := range namesakes {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c, rules := renamedSCluster(t, placement, remove1, ns)
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
				wantRefusedWith(t, err, `Flux Kustomization "flux-system/x-s" depends on "flux-system/platform"`)

				c, rules = renamedSCluster(t, placement, "", ns)
				if _, err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules); err != nil {
					t.Fatalf("patcher without the patch: got %v, want the tree admitted", err)
				}

				c = builtClassCluster([]string{"v"})
				other := &stack.Node{Name: "other", Bundle: srBundle("other", stack.NewApplication("namesake-app", "default", &fakeAppConfig{objs: []*client.Object{&ns}}))}
				other.SetParent(c.Node)
				c.Node.Children = append(c.Node.Children, other)
				if _, err = fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement)); err != nil {
					t.Fatalf("namesake alone: got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// A patch of the renamed copy of s copies the name of dependsOn index 0 into
// metadata.name: before the rewrite that names platform, after it y-infra, so
// the build holds a Kustomization under a name the unrewritten build does not
// (identity-set check). Reading the two builds as one would hide it.
func TestBuiltClasses_PatchRenamesByCopyingADependsOnName(t *testing.T) {
	patch := "- op: copy\n  from: /spec/dependsOn/0/name\n  path: /metadata/name\n"
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			c, rules := renamedSCluster(t, placement, patch, nil)
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, rules)
			wantRefusedWith(t, err, `makes Flux Kustomization "flux-system/x-platform" before the rewrite but not after it`)
		})
	}
}

// x-platform is a renamed copy of the umbrella platform that a build makes, and
// a patch gives x-web a dependsOn on it. An ordinary Kustomization named
// x-platform, applied by another node but skipped by every apply (ssa: Ignore),
// differs from the copy and stands for nothing: the tree is refused (U3).
func TestBuiltClasses_RenamedUmbrellaBesideASkippedNamesake(t *testing.T) {
	patch := kustomize.Patch{
		Patch:  "- op: add\n  path: /spec/dependsOn\n  value:\n  - name: x-platform\n",
		Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
	}
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				u := waitingUmbrella(nil)
				u.HealthChecks = nil
				c := noWaitCluster(u, false)
				k := handWritten("x-platform", "elsewhere")
				k.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
				var ns client.Object = k
				other := &stack.Node{Name: "other", Bundle: srBundle("other", stack.NewApplication("namesake-app", "default", &fakeAppConfig{objs: []*client.Object{&ns}}))}
				other.SetParent(c.Node)
				c.Node.Children = append(c.Node.Children, other)
				if path != "" {
					p := handWritten("hand-written")
					p.Spec.Path, p.Spec.NamePrefix = path, "x-"
					p.Spec.Patches = []kustomize.Patch{patch}
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", p))})
				}
				return c
			}
			dir := hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "web")
			_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(dir), rules)
			wantRefusedWith(t, err, "an umbrella the tree does not hold")
		})
	}
}

// A patch the build of the tree cannot apply (it removes dependsOn index 5 of
// a list that holds one entry) leaves the tree unreadable as built, so the
// classes fall back to the blanket: an existence-only entry on a patched tree
// is refused as before go-kure/kure#1226, the setting on.
func TestBuiltClasses_UnbuildableTreeKeepsTheBlanket(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			u := waitingUmbrella(nil)
			u.HealthChecks = nil
			c := noWaitCluster(u, false)
			s := handWritten("s")
			s.Spec.DependsOn = []kustv1.DependencyReference{{Name: "platform", ReadyExpr: "true"}}
			for _, n := range c.Node.Children {
				if n.Name == "web" {
					n.Bundle = srBundle("web", objApp("web-app", s))
					n.Bundle.Patches = []stack.Patch{patchOnS("- op: remove\n  path: /spec/dependsOn/5\n")}
				}
			}
			g := noWaitGenerator()
			g.AdditiveCELDependencyCheckOff = true
			_, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
			if !errors.Is(err, fluxstack.ErrUmbrellaDependency) {
				t.Fatalf("got %v, want the tree refused as ErrUmbrellaDependency", err)
			}
		})
	}
}

// UmbrellaTerminals, the package function and the method of a generator with
// and without AdditiveCELDependencyCheckOff, returns one result on a patched
// tree, and it is the leaves the rewrite writes for the umbrella platform.
func TestBuiltClasses_UmbrellaTerminalsParityOnAPatchedTree(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(builtClassCluster([]string{"v"}, patchOnS("- op: add\n  path: /metadata/labels\n  value:\n    a: b\n")), rules)
			if err != nil {
				t.Fatal(err)
			}
			want, err := fluxstack.UmbrellaTerminals(ml)
			if err != nil {
				t.Fatal(err)
			}
			if got := want["flux-system/platform"]; !reflect.DeepEqual(got, []string{"flux-system/y-infra", "flux-system/y-services"}) {
				t.Fatalf("terminals of platform = %v, want [y-infra y-services]", got)
			}
			for _, off := range []bool{false, true} {
				g := noWaitGenerator()
				g.AdditiveCELDependencyCheckOff = off
				got, err := g.UmbrellaTerminals(ml)
				if err != nil || !reflect.DeepEqual(got, want) {
					t.Fatalf("method (check off %v) = %v, %v; want %v", off, got, err, want)
				}
			}
			for _, k := range kustomizations(ml) {
				if k.Name != "s" {
					continue
				}
				var names []string
				for _, d := range k.Spec.DependsOn {
					names = append(names, d.Name)
				}
				if !reflect.DeepEqual(names, []string{"y-infra", "y-services", "v"}) {
					t.Fatalf("s dependsOn = %v, want the rewrite's leaves [y-infra y-services v]", names)
				}
			}
		})
	}
}

// strayLayout is a layout under ml that no Kustomization of the tree builds and
// the Flux bootstrap does not apply, holding objs.
func strayLayout(ml *layout.ManifestLayout, objs ...client.Object) *layout.ManifestLayout {
	l := &layout.ManifestLayout{Name: "stray", Namespace: ml.FullRepoPath(), UmbrellaChild: true, Resources: objs}
	ml.Children = append(ml.Children, l)
	return l
}

// x-s is a renamed copy of s that a build makes; the tree also holds an ordinary
// object x-s in a directory no writer builds, which takes no part in any build.
// It stands for nothing: a patch that drops a wait the rewrite owed the copy is
// refused (U2 when the build before the rewrite succeeds, the fallback when it
// fails: `remove /spec/dependsOn/2` is out of range for [platform v] and valid
// for [y-infra y-services v]). The same tree without the patch, and the
// namesake alone, are admitted.
func TestBuiltClasses_RenamedMemberCopyBesideAnUnbuiltNamesake(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"U2: removes index 1":       {"- op: remove\n  path: /spec/dependsOn/1\n", true},
		"fallback: removes index 2": {"- op: remove\n  path: /spec/dependsOn/2\n", true},
		"no-op label patch (ctl)":   {"- op: add\n  path: /metadata/labels\n  value:\n    a: b\n", false},
		"U2: removes index 0 (ctl)": {"- op: remove\n  path: /spec/dependsOn/0\n", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c, rules := renamedSCluster(t, placement, tc.patch, nil)
				ml := mustWalk(t, c, rules)
				strayLayout(ml, handWritten("x-s"))
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if tc.refused {
					wantRefusedWith(t, err, `Flux Kustomization "flux-system/x-s"`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// The namesake x-s in a directory no writer builds, and no build makes a copy:
// admitted.
func TestBuiltClasses_UnbuiltNamesakeAlone(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			c := builtClassCluster([]string{"v"})
			rules := withPlacement(nodeOnly, placement)
			ml := mustWalk(t, c, rules)
			strayLayout(ml, handWritten("x-s"))
			if err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// The build before the rewrite fails (index 2 is out of range for the held
// [platform v]) and the build after it succeeds ([y-infra y-services v]), where
// the patch removes v, an entry on an ordinary Kustomization: the fallback judges
// every entry a member held, not only the leaves the rewrite wrote (go-kure/kure#1226).
//
// A deliberate over-refusal: a patch that removes a held entry by its index in
// the rewritten list is the same tree, and kure cannot tell the author's intent
// from the lost wait, so it refuses.
func TestBuiltClasses_FallbackKeepsHeldOrdinaryEntries(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"removes index 2":   {"- op: remove\n  path: /spec/dependsOn/2\n", true},
		"no-op patch (ctl)": {"- op: add\n  path: /metadata/labels\n  value:\n    a: b\n", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(builtClassCluster([]string{"v"}, patchOnS(tc.patch)), withPlacement(nodeOnly, placement))
				if tc.refused {
					wantRefusedWith(t, err, `waits for "flux-system/v" before the rewrite and not after it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// s holds [platform external v]; external is no Kustomization of the tree (it
// may exist outside it, or nowhere). `remove /spec/dependsOn/2` drops v before
// the rewrite and external after it, [y-infra y-services external v], and both
// builds succeed: the entry on a target the view does not build keeps its
// obligation (go-kure/kure#1226).
//
// A deliberate over-refusal: a patch that drops an entry on a dependency that
// exists nowhere is refused as well; kure cannot prove a target absent.
func TestBuiltClasses_ObligationOnATargetOutsideTheTree(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"removes index 2":   {"- op: remove\n  path: /spec/dependsOn/2\n", true},
		"no-op patch (ctl)": {"- op: add\n  path: /metadata/labels\n  value:\n    a: b\n", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c := builtClassCluster(nil, patchOnS(tc.patch))
				leafNode(c, "v")
				for _, n := range c.Node.Children {
					if n.Name == "web" {
						sb := n.Bundle.Children[0]
						sb.NamedDependsOn = []string{"external", "v"}
					}
				}
				_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(c, withPlacement(nodeOnly, placement))
				if tc.refused {
					wantRefusedWith(t, err, `waits for "flux-system/external" before the rewrite and not after it`)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// x-s is a renamed copy of s that a build makes, and the tree holds an ordinary
// x-s in a directory no writer builds. A patch that only adds an entry,
// `add /spec/dependsOn/3` (out of range before the rewrite, an append after it),
// puts the build before the rewrite on the fallback, which exempts a namesake
// only when the copy has the graph fields it was held with. The copy has not
// (it has the dependsOn of s): refused, though the patch drops nothing.
//
// A deliberate over-refusal: a held non-member whose applied copy legitimately
// differs is not trusted on the fallback path.
func TestBuiltClasses_FallbackNamesakeThatDiffersFromTheCopy(t *testing.T) {
	patch := "- op: add\n  path: /spec/dependsOn/3\n  value:\n    name: w\n"
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			rules := withPlacement(nodeOnly, placement)
			build := func(path string) *stack.Cluster {
				c := builtClassCluster([]string{"v"})
				leafNode(c, "w")
				if path != "" {
					k := handWritten("hand-written")
					k.Spec.Path, k.Spec.NamePrefix = path, "x-"
					k.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "s"}}}
					c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", k))})
				}
				return c
			}
			c := build(hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "s"))
			ml := mustWalk(t, c, rules)
			strayLayout(ml, handWritten("x-s"))
			err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
			wantRefusedWith(t, err, `the build of the tree makes Flux Kustomization "flux-system/x-s", which the tree does not hold`)
		})
	}
}

// rescueBuiltTree integrates a tree whose member web2, shop's child, depends on
// the umbrella platform and then on v, and which shop deletes from its own build.
// The Kustomization rescue, which the Flux bootstrap applies, builds shop's
// directory without that patch and so creates web2, with patch on it. No other
// apply creates web2, so the second view of the tree (the rescue view,
// go-kure/kure#1174) is the only one that holds it.
func rescueBuiltTree(t *testing.T, placement layout.FluxPlacement, patch string) error {
	t.Helper()
	build := func(deleteWeb bool) *stack.Cluster {
		u := waitingUmbrella(nil)
		u.HealthChecks = nil
		c := noWaitCluster(u, false)
		v := leafNode(c, "v")
		web := srBundle("web2", cmApp("web2-app"))
		web.DependsOn = []*stack.Bundle{u, v}
		shop := umbrellaOf("shop", web)
		if deleteWeb {
			shop.Patches = []stack.Patch{{Patch: deleteKust("web2"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "web2"}}}
		}
		shopNode := &stack.Node{Name: "shop", Bundle: shop}
		shopNode.SetParent(c.Node)
		c.Node.Children = append(c.Node.Children, shopNode)
		return c
	}
	rules := withPlacement(nodeOnly, placement)
	probe, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(build(false), rules)
	if err != nil {
		t.Fatalf("without the patch: %v", err)
	}
	rescue := handWritten("rescue")
	rescue.Spec.Path = findKept(probe, "shop").Spec.Path
	if patch != "" {
		rescue.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "web2"}}}
	}
	c := build(true)
	ml := mustWalk(t, c, rules)
	ml.Resources = append(ml.Resources, rescue)
	return fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
}

// web2 is created by rescue alone, whose build holds the patch (the rescue
// view of the tree, go-kure/kure#1174). A patch there that changes dependsOn
// makes the build of rescue write web2 differently from the tree's spec, which
// an earlier check refuses before the classes are compared: the obligation
// check in the rescue view is therefore not reachable through a patch of
// dependsOn, and the tests pin that a tree with such a view is read, admitted
// when the patch leaves dependsOn alone and refused when it does not.
func TestBuiltClasses_RescueView(t *testing.T) {
	for name, tc := range map[string]struct {
		patch   string
		refused bool
	}{
		"removes index 1":   {"- op: remove\n  path: /spec/dependsOn/1\n", true},
		"removes index 0":   {"- op: remove\n  path: /spec/dependsOn/0\n", true},
		"label patch (ctl)": {"- op: add\n  path: /metadata/labels\n  value:\n    a: b\n", false},
		"no patch (ctl)":    {"", false},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := rescueBuiltTree(t, placement, tc.patch)
				if tc.refused {
					if err == nil || !strings.Contains(err.Error(), `the build of Flux Kustomization "flux-system/rescue" applies it with different spec.dependsOn`) {
						t.Fatalf("got %v, want the tree refused for the rescue build's dependsOn", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}

// namesakesOfTheCopies are two ordinary Kustomizations, x-platform and
// x-y-infra, in a directory no writer builds. Each clones the graph fields of
// the generated s (its path, source and, with wait, its wait) and holds the
// dependsOn the rewrite gives the copy, [y-infra y-services v]. The rename
// patch of TestBuiltClasses_PatchRenamesByCopyingADependsOnName moves the
// built copy from x-platform to x-y-infra; these two stand for neither.
func namesakesOfTheCopies(t *testing.T, rules layout.LayoutRules, wait bool) []client.Object {
	t.Helper()
	s := findKustomization(integrated(t, builtClassCluster([]string{"v"}), rules), "s")
	if s == nil {
		t.Fatal("no generated Kustomization s")
	}
	var out []client.Object
	for _, name := range []string{"x-platform", "x-y-infra"} {
		k := s.DeepCopy()
		k.Name = name
		k.Spec.Wait = wait
		k.Spec.DependsOn = nil
		for _, d := range []string{"y-infra", "y-services", "v"} {
			k.Spec.DependsOn = append(k.Spec.DependsOn, kustv1.DependencyReference{Name: d})
		}
		out = append(out, k)
	}
	return out
}

// A patch of the renamed copy of s copies the name of dependsOn index 0 into
// metadata.name, so the build makes x-platform before the rewrite and
// x-y-infra after it. The tree also holds an unbuilt namesake of each name that
// matches the copy in every graph field and in the rewritten dependsOn, so
// reading held objects as identities of the build would hide the move: the
// identity-set check compares what the build makes, and refuses it
// (go-kure/kure#1226, go-kure/kure#1243). Namesakes that differ in wait are
// refused as well, by the same check. The same namesakes with a patch that
// renames nothing are admitted: an unbuilt namesake alone is no defect.
func TestBuiltClasses_RenameBesideMatchingUnbuiltNamesakes(t *testing.T) {
	const rename = "- op: copy\n  from: /spec/dependsOn/0/name\n  path: /metadata/name\n"
	const noop = "- op: add\n  path: /metadata/labels\n  value:\n    a: b\n"
	for name, tc := range map[string]struct {
		patch string
		wait  bool
		want  string
	}{
		"rename, namesakes match":        {rename, true, `makes Flux Kustomization "flux-system/x-platform" before the rewrite but not after it`},
		"rename, namesakes differ (ctl)": {rename, false, `makes Flux Kustomization "flux-system/x-platform" before the rewrite but not after it`},
		"no rename, namesakes (ctl)":     {noop, true, ""},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c, rules := renamedSCluster(t, placement, tc.patch, nil)
				ml := mustWalk(t, c, rules)
				strayLayout(ml, namesakesOfTheCopies(t, rules, tc.wait)...)
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if tc.want != "" {
					wantRefusedWith(t, err, tc.want)
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
			})
		}
	}
}
