package fluxcd_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1214 (effect 2): a dependsOn entry without a
// readyExpr, as the tree holds it, proves its dependency Ready only where no
// patch can add a readyExpr to it. With the controller's additive CEL gate off
// (its default) an entry with a readyExpr is evaluated in place of the
// controller's own Ready check.

// toAddReadyExpr adds a readyExpr to the first dependsOn entry of the Flux
// Kustomization named name, a plain one in the tree that held it.
func toAddReadyExpr(name string) kustomize.Patch {
	return kustomize.Patch{
		Patch:  "- op: add\n  path: /spec/dependsOn/0/readyExpr\n  value: \"dep.status.conditions.exists(c, c.type == 'Ready' && c.status == 'True')\"\n",
		Target: &kustomize.Selector{Kind: "Kustomization", Name: name},
	}
}

// patchedProofTree is a tree whose member a builds the directory apps, which
// holds h, depending on x with a plain entry, and x. edit patches it (or
// not) before the tree is recorded.
func patchedProofTree(edit func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization)) *layout.ManifestLayout {
	root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
	apps := &layout.ManifestLayout{Name: "apps", Namespace: root.FullRepoPath()}
	root.Children = []*layout.ManifestLayout{apps}
	a := handWritten("a")
	a.Spec.Path = apps.FullRepoPath()
	root.Resources = []client.Object{a}
	apps.Resources = []client.Object{handWritten("h", "x"), handWritten("x")}
	edit(root, apps, a)
	root.SetFluxKustomizations([]layout.FluxKustomizationRecord{{
		Namespace: "flux-system", Name: "a", Path: apps.FullRepoPath(),
		SourceKind: "GitRepository", SourceName: "flux-system",
	}})
	return root
}

// A patch that lands on h adds a readyExpr to h's plain entry on x, so the
// entry no longer proves x Ready: x stays a terminal leaf of a beside h. The
// reading follows the build (go-kure/kure#1226): a patch naming no
// Kustomization of its build reaches nothing and h's entry still proves x. A
// patch of a non-member the build does not read may reach h from where the
// view cannot see, so the entry proves nothing then, as before.
func TestUmbrellaTerminals_PatchedPlainEntryProvesNothing(t *testing.T) {
	for name, tc := range map[string]struct {
		edit func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization)
		want []string
	}{
		"unpatched": {
			edit: func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization) {},
			want: []string{"flux-system/h"},
		},
		"patched by the member": {
			edit: func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization) {
				a.Spec.Patches = []kustomize.Patch{toAddReadyExpr("h")}
			},
			want: []string{"flux-system/h", "flux-system/x"},
		},
		"patched by another copy of the member": {
			edit: func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization) {
				more := &layout.ManifestLayout{Name: "more", Namespace: root.FullRepoPath()}
				copyOfA := handWritten("a")
				copyOfA.Spec.Path = apps.FullRepoPath()
				copyOfA.Spec.Patches = []kustomize.Patch{toAddReadyExpr("h")}
				more.Resources = []client.Object{copyOfA}
				root.Children = append(root.Children, more)
			},
			want: []string{"flux-system/h", "flux-system/x"},
		},
		"patch of a non-member the build does not read": {
			edit: func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization) {
				elsewhere := &layout.ManifestLayout{Name: "elsewhere", Namespace: root.FullRepoPath()}
				root.Children = append(root.Children, elsewhere)
				b := handWritten("b")
				b.Spec.Path = elsewhere.FullRepoPath()
				b.Spec.Patches = []kustomize.Patch{toAddReadyExpr("h")}
				root.Resources = append(root.Resources, b)
			},
			want: []string{"flux-system/h", "flux-system/x"},
		},
		"patch naming nothing the build holds": {
			edit: func(root, apps *layout.ManifestLayout, a *kustv1.Kustomization) {
				a.Spec.Patches = []kustomize.Patch{toAddReadyExpr("nothing-is-named-this")}
			},
			want: []string{"flux-system/h"},
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := fluxstack.UmbrellaTerminals(patchedProofTree(tc.edit))
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got["flux-system/a"], tc.want) {
				t.Errorf("got %v, want flux-system/a -> %v", got, tc.want)
			}
		})
	}
}

// hostPathOf returns the path of the layout that holds the Flux Kustomization
// the integration generates for build() under name.
func hostPathOf(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules, name string) string {
	t.Helper()
	var host string
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if k, ok := r.(*kustv1.Kustomization); ok && k.Name == name {
				host = l.FullRepoPath()
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(integrated(t, build(), rules))
	if host == "" {
		t.Fatalf("no layout holds the generated Kustomization %q", name)
	}
	return host
}

// layoutPathNamed returns the path of the layout named name in the tree the
// integration builds for build().
func layoutPathNamed(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules, name string) string {
	t.Helper()
	var found string
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if l.Name == name {
			found = l.FullRepoPath()
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(integrated(t, build(), rules))
	if found == "" {
		t.Fatalf("no layout named %q", name)
	}
	return found
}

// The node apps asks for platform, and its Kustomization is kept from the tree
// with a dependsOn entry on platform that sets no readyExpr. Where the tree
// holds a Kustomization that patches what it builds, the patch may add a
// readyExpr to that entry (directly, or by adding patches to another
// Kustomization), so it no longer proves platform Ready and the kept apps is
// refused (go-kure/kure#1214), whichever directory the patch builds. With no
// patch it is admitted as before.
func TestKeptNode_PlainEntryProvesNothingWherePatched(t *testing.T) {
	build := keptApps(&stack.Node{Name: "cert-manager", Bundle: srBundle("cert-manager", cmApp("cert-manager-app"))})
	for _, reach := range []string{"none", "reaches the kept CR", "reaches another directory", "unreadable non-member patcher"} {
		for _, form := range keptForms {
			t.Run(reach+"/"+form, func(t *testing.T) {
				rules := perLayoutNodeOnly()
				ml, c := keptIn(t, build, rules, "apps", form, func(k *kustv1.Kustomization) {
					k.Spec.DependsOn = []kustv1.DependencyReference{exprDep("prod-platform-node", "")}
				}, nil)
				if reach != "none" {
					p := handWritten("patcher")
					p.Spec.Path = hostPathOf(t, build, rules, "apps")
					if reach == "reaches another directory" {
						p.Spec.Path = layoutPathNamed(t, build, rules, "shop")
					}
					p.Spec.Patches = []kustomize.Patch{toAddReadyExpr("apps")}
					if reach == "unreadable non-member patcher" {
						// An interval Flux substitutes at apply time does not
						// read in typed form: the copy may carry patches.
						content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(p)
						if err != nil {
							t.Fatal(err)
						}
						if err := unstructured.SetNestedField(content, "${INTERVAL}", "spec", "interval"); err != nil {
							t.Fatal(err)
						}
						ml.Resources = append(ml.Resources, &unstructured.Unstructured{Object: content})
					} else {
						ml.Resources = append(ml.Resources, p)
					}
				}
				err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
				if reach != "none" {
					if err == nil || !strings.Contains(err.Error(), "a patch of a Flux Kustomization in the tree may reach it") {
						t.Fatalf("got %v, want the kept apps refused", err)
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

// Under UmbrellasDoNotWait the kept apps names the leaves a and b of the
// umbrella p with entries that set no readyExpr, the form an earlier
// integration wrote (R3). Where the tree holds a patch it may add a readyExpr
// to them, so they no longer prove the leaves Ready and the kept apps is
// refused (go-kure/kure#1214); with no patch it is admitted as before.
func TestKeptNode_UmbrellaLeafEntriesProveNothingWherePatched(t *testing.T) {
	build := keptApps(&stack.Node{Name: "p", Bundle: srBundle("p", objApp("a-app", handWritten("a")), objApp("b-app", handWritten("b", "a")))})
	for _, reach := range []string{"none", "reaches the kept CR", "reaches another directory"} {
		for _, form := range keptForms {
			t.Run(reach+"/"+form, func(t *testing.T) {
				rules := perLayoutNodeOnly()
				ml, c := keptIn(t, build, rules, "apps", form, func(k *kustv1.Kustomization) {
					k.Spec.DependsOn = []kustv1.DependencyReference{{Name: "a"}, {Name: "b"}}
				}, nil)
				if reach != "none" {
					p := handWritten("patcher")
					p.Spec.Path = hostPathOf(t, build, rules, "apps")
					if reach == "reaches another directory" {
						p.Spec.Path = layoutPathNamed(t, build, rules, "shop")
					}
					p.Spec.Patches = []kustomize.Patch{toAddReadyExpr("apps")}
					ml.Resources = append(ml.Resources, p)
				}
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, c, rules)
				if reach != "none" {
					if !errors.Is(err, fluxstack.ErrUmbrellaDependency) {
						t.Fatalf("got %v, want the kept apps refused, wrapping ErrUmbrellaDependency", err)
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

// With UmbrellasDoNotWait web's dependsOn on the umbrella platform is written
// as one on its terminal leaves. b holds a plain entry on a, which proves a
// Ready, so the leaf is b alone; where the tree holds a patch it may add a
// readyExpr to that entry. A patch that lands on b makes a second write of b
// with another dependsOn, which is refused; one that builds another directory
// reaches nothing, and the leaf stays b alone (go-kure/kure#1226).
func TestUmbrellaRewrite_PatchedEntryLeavesBothLeaves(t *testing.T) {
	for _, reach := range []string{"none", "reaches b", "reaches another directory"} {
		for _, placement := range integratedPlacements {
			t.Run(reach+"/"+string(placement), func(t *testing.T) {
				rules := withPlacement(nodeOnly, placement)
				build := func(patch string) *stack.Cluster {
					platform := srBundle("platform", objApp("a-app", handWritten("a")), objApp("b-app", handWritten("b", "a")))
					c := noWaitCluster(platform, true)
					if patch != "" {
						p := handWritten("patch-writer")
						p.Spec.Path = patch
						p.Spec.Patches = []kustomize.Patch{toAddReadyExpr("b")}
						c.Node.Children = append(c.Node.Children, &stack.Node{Name: "patcher", Bundle: srBundle("patcher", objApp("patcher-app", p))})
					}
					return c
				}
				patch := ""
				switch reach {
				case "reaches b":
					patch = hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "b")
				case "reaches another directory":
					patch = hostPathOf(t, func() *stack.Cluster { return build("") }, rules, "web")
				}
				ml, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).CreateLayoutWithResources(build(patch), rules)
				if reach == "reaches b" {
					// The patched b and platform's own b are two writes of one
					// identity with different dependsOn: refused whatever the
					// rewrite does, so a patch that lands on b cannot be admitted.
					if err == nil || !strings.Contains(err.Error(), "with different spec.dependsOn") {
						t.Fatalf("got %v, want the two writes of b refused", err)
					}
					return
				}
				if err != nil {
					t.Fatalf("got %v, want the tree admitted", err)
				}
				want := []string{"b"}
				for _, k := range kustomizations(ml) {
					if k.Name != "web" {
						continue
					}
					got := dependsOnNames(k)
					slices.Sort(got)
					if !slices.Equal(got, want) {
						t.Fatalf("web dependsOn = %v, want %v", got, want)
					}
					return
				}
				t.Fatal("no Kustomization named web")
			})
		}
	}
}

// a builds controllers, which holds b, and patches b to add spec.patches; b,
// which holds no patch itself, builds apps, which holds h, depending on x with
// a plain entry, and x. The patch b then applies adds a readyExpr to h, so
// the entry proves nothing and x is a terminal leaf of a beside h, though no
// held patch list is in the build that holds h (a scoped reading missed it).
func TestUmbrellaTerminals_PatchAddedToAnotherKustomizationProvesNothing(t *testing.T) {
	root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
	controllers := &layout.ManifestLayout{Name: "controllers", Namespace: root.FullRepoPath()}
	apps := &layout.ManifestLayout{Name: "apps", Namespace: controllers.FullRepoPath()}
	root.Children = []*layout.ManifestLayout{controllers}
	controllers.Children = []*layout.ManifestLayout{apps}
	a := handWritten("a")
	a.Spec.Path = controllers.FullRepoPath()
	a.Spec.Patches = []kustomize.Patch{{
		Patch:  "- op: add\n  path: /spec/patches\n  value:\n  - target: {kind: Kustomization, name: h}\n    patch: |-\n      - op: add\n        path: /spec/dependsOn/0/readyExpr\n        value: \"true\"\n",
		Target: &kustomize.Selector{Kind: "Kustomization", Name: "b"},
	}}
	b := handWritten("b")
	b.Spec.Path = apps.FullRepoPath()
	root.Resources = []client.Object{a}
	controllers.Resources = []client.Object{b}
	apps.Resources = []client.Object{handWritten("h", "x"), handWritten("x")}
	root.SetFluxKustomizations([]layout.FluxKustomizationRecord{
		{Namespace: "flux-system", Name: "a", Path: controllers.FullRepoPath(), SourceKind: "GitRepository", SourceName: "flux-system"},
		{Namespace: "flux-system", Name: "b", Path: apps.FullRepoPath(), SourceKind: "GitRepository", SourceName: "flux-system"},
	})
	got, err := fluxstack.UmbrellaTerminals(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"flux-system/b", "flux-system/h", "flux-system/x"}
	if !slices.Equal(got["flux-system/a"], want) {
		t.Errorf("got %v, want flux-system/a -> %v", got, want)
	}
}

// a builds controllers, which holds b; b builds apps, which holds h, depending
// on x with a plain entry, and x. The tree also holds two Flux Kustomizations
// named p, each in a directory nothing builds, on the Source of a and b and
// with the path of apps: p1 patches h to add a readyExpr to that entry, p2
// patches nothing. The Flux bootstrap may apply either, so p1's patch may
// land on h, and x is a terminal leaf of a beside h whichever order the tree
// lists them in (go-kure/kure#1243). p2 alone patches nothing: the entry
// proves x Ready, and x is no leaf.
func TestUmbrellaTerminals_HeldCopiesOfAPatcher(t *testing.T) {
	both := []string{"flux-system/b", "flux-system/h", "flux-system/x"}
	for name, tc := range map[string]struct {
		order []string
		want  []string
	}{
		"patching copy, then plain copy": {[]string{"p1", "p2"}, both},
		"plain copy, then patching copy": {[]string{"p2", "p1"}, both},
		"patching copy alone (ctl)":      {[]string{"p1"}, both},
		"plain copy alone (ctl)":         {[]string{"p2"}, both[:2]},
	} {
		t.Run(name, func(t *testing.T) {
			root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
			controllers := &layout.ManifestLayout{Name: "controllers", Namespace: root.FullRepoPath()}
			apps := &layout.ManifestLayout{Name: "apps", Namespace: controllers.FullRepoPath()}
			root.Children = []*layout.ManifestLayout{controllers}
			controllers.Children = []*layout.ManifestLayout{apps}
			a := handWritten("a")
			a.Spec.Path = controllers.FullRepoPath()
			b := handWritten("b")
			b.Spec.Path = apps.FullRepoPath()
			root.Resources = []client.Object{a}
			controllers.Resources = []client.Object{b}
			apps.Resources = []client.Object{handWritten("h", "x"), handWritten("x")}
			for i, copyName := range tc.order {
				p := handWritten("p")
				p.Spec.Path = apps.FullRepoPath()
				if copyName == "p1" {
					p.Spec.Patches = []kustomize.Patch{toAddReadyExpr("h")}
				}
				root.Children = append(root.Children, &layout.ManifestLayout{
					Name: "unbuilt" + string(rune('0'+i)), Namespace: root.FullRepoPath(), UmbrellaChild: true,
					Resources: []client.Object{p},
				})
			}
			root.SetFluxKustomizations([]layout.FluxKustomizationRecord{
				{Namespace: "flux-system", Name: "a", Path: controllers.FullRepoPath(), SourceKind: "GitRepository", SourceName: "flux-system"},
				{Namespace: "flux-system", Name: "b", Path: apps.FullRepoPath(), SourceKind: "GitRepository", SourceName: "flux-system"},
			})
			got, err := fluxstack.UmbrellaTerminals(root)
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(got["flux-system/a"], tc.want) {
				t.Errorf("got %v, want flux-system/a -> %v", got, tc.want)
			}
		})
	}
}
