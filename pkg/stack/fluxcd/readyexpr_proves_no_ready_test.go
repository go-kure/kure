package fluxcd_test

import (
	"errors"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for go-kure/kure#1190: a dependsOn entry that sets a readyExpr is no
// proof that its dependency is Ready. Where the controller's additive CEL
// gate is off (its default), the controller evaluates the expression and then
// skips its own Ready check, so an expression such as "true" makes the entry
// wait for existence only. Terminal selection and the kept-node check count
// only an entry with no readyExpr as a wait for Ready.

// readyExprs are the readyExpr values the tests set: the one the setting
// AdditiveCELDependencyCheckOff reads as existence-only, and one it does not.
var readyExprs = []string{"true", "dep.status.conditions.exists(c, c.type == 'Ready' && c.status == 'True')"}

// settings names the two values of AdditiveCELDependencyCheckOff.
var settings = map[bool]string{false: "setting unset", true: "setting set"}

// exprDep is the dependsOn entry of the Kustomization named name, with expr as
// its readyExpr ("" for a plain one).
func exprDep(name, expr string) kustv1.DependencyReference {
	return kustv1.DependencyReference{Name: name, ReadyExpr: expr}
}

// pairLayout is a tree with the Flux Kustomization platform, applying a and
// b, where b depends on a through dep. Its objects are in form, "typed" or
// "unstructured".
func pairLayout(t *testing.T, dep kustv1.DependencyReference, form string) *layout.ManifestLayout {
	t.Helper()
	root := &layout.ManifestLayout{Name: "demo", Namespace: "clusters"}
	apps := &layout.ManifestLayout{Name: "apps", Namespace: root.FullRepoPath()}
	root.Children = []*layout.ManifestLayout{apps}
	platform := handWritten("platform")
	platform.Spec.Path = apps.FullRepoPath()
	root.Resources = []client.Object{platform}
	b := handWritten("b")
	b.Spec.DependsOn = []kustv1.DependencyReference{dep}
	apps.Resources = []client.Object{inForm(t, handWritten("a"), form), inForm(t, b, form)}
	root.SetFluxKustomizations([]layout.FluxKustomizationRecord{{
		Namespace: "flux-system", Name: "platform", Path: apps.FullRepoPath(),
		SourceKind: "GitRepository", SourceName: "flux-system",
	}})
	return root
}

// inForm is k as a typed object or as an unstructured one.
func inForm(t *testing.T, k *kustv1.Kustomization, form string) client.Object {
	t.Helper()
	if form == "typed" {
		return k
	}
	content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatalf("ToUnstructured: %v", err)
	}
	u := &unstructured.Unstructured{Object: content}
	u.SetAPIVersion(kustv1.GroupVersion.String())
	u.SetKind("Kustomization")
	return u
}

// platform applies a and b, and b depends on a with a readyExpr. b's entry
// does not prove a Ready (the controller skips its Ready check), so a is a
// terminal leaf of platform beside b: whoever depends on platform waits for
// both. Before the fix a was dropped and the dependent could proceed before a
// was Ready. The control, a plain entry, drops a: b waits for a's Ready.
func TestTerminalLeaves_ReadyExprEntryProvesNoReady(t *testing.T) {
	for _, expr := range append(readyExprs, "") {
		for _, form := range []string{"typed", "unstructured"} {
			for set, label := range settings {
				want := []string{"flux-system/a", "flux-system/b"}
				if expr == "" {
					want = []string{"flux-system/b"}
				}
				t.Run(label+"/"+form+"/"+expr, func(t *testing.T) {
					root := pairLayout(t, exprDep("a", expr), form)
					for name, read := range map[string]func(...*layout.ManifestLayout) (map[string][]string, error){
						"UmbrellaTerminals":                   fluxstack.UmbrellaTerminals,
						"ResourceGenerator.UmbrellaTerminals": celGateOffGenerator(set).UmbrellaTerminals,
					} {
						got, err := read(root)
						if err != nil {
							t.Fatalf("%s: %v", name, err)
						}
						if !slices.Equal(got["flux-system/platform"], want) {
							t.Errorf("%s = %v, want flux-system/platform -> %v", name, got, want)
						}
					}
				})
			}
		}
	}
}

// Standalone UmbrellaTerminals runs no cycle check. On a tree whose only
// mutual entries set a readyExpr it returned ErrUmbrellaTerminals (no
// terminal leaf): each entry counted as a wait for the other's Ready. It now
// returns both Kustomizations as leaves, whatever the setting.
func TestUmbrellaTerminals_MutualReadyExprEntriesReturnLeaves(t *testing.T) {
	for set, label := range settings {
		t.Run(label, func(t *testing.T) {
			root := pairLayout(t, exprDep("a", "true"), "typed")
			apps := root.Children[0]
			a := handWritten("a", "b")
			a.Spec.DependsOn[0].ReadyExpr = "true"
			apps.Resources[0] = a
			want := []string{"flux-system/a", "flux-system/b"}
			for name, read := range map[string]func(...*layout.ManifestLayout) (map[string][]string, error){
				"UmbrellaTerminals":                   fluxstack.UmbrellaTerminals,
				"ResourceGenerator.UmbrellaTerminals": celGateOffGenerator(set).UmbrellaTerminals,
			} {
				got, err := read(root)
				if err != nil {
					t.Fatalf("%s: %v", name, err)
				}
				if !slices.Equal(got["flux-system/platform"], want) {
					t.Errorf("%s = %v, want flux-system/platform -> %v", name, got, want)
				}
			}
		})
	}
}

// The integration with UmbrellasDoNotWait writes web's dependsOn on platform
// as one on platform's terminal leaves. With a readyExpr on b's entry for a
// those are a and b; without, b alone.
func TestUmbrellaRewrite_DependentWaitsForBothLeaves(t *testing.T) {
	for _, expr := range append(readyExprs, "") {
		for _, placement := range integratedPlacements {
			for _, form := range kustomizationForms {
				for set, label := range settings {
					t.Run(label+"/"+string(placement)+"/"+form+"/"+expr, func(t *testing.T) {
						a, b := handWritten("a"), handWritten("b", "a")
						b.Spec.DependsOn[0].ReadyExpr = expr
						platform := srBundle("platform", formApp("a-app", a, form), formApp("b-app", b, form))
						g := noWaitGenerator()
						g.AdditiveCELDependencyCheckOff = set
						ml, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						want := []string{"a", "b"}
						if expr == "" || form == "v1beta2" {
							want = []string{"b"}
						}
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
	}
}

// keptApps is a cluster whose group node apps depends on the group node
// platform and is named "apps", so that the Kustomization the integration
// writes for apps can be kept from the tree. platform's children are
// children.
func keptApps(children ...*stack.Node) func() *stack.Cluster {
	return func() *stack.Cluster {
		platform := &stack.Node{Name: "platform", Children: children}
		apps := &stack.Node{Name: "apps", KustomizationName: "apps", DependsOn: []*stack.Node{platform}, Children: []*stack.Node{
			{Name: "shop", Bundle: srBundle("shop", cmApp("shop-app"))},
		}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "prod", Children: []*stack.Node{platform, apps}}}
	}
}

// The node apps asks for platform (Kustomization prod-platform-node) and its Kustomization is kept from the
// tree. A kept dependsOn names platform with a readyExpr: it waits for
// platform to exist but, where the controller's additive CEL gate is off,
// not for its Ready, so it does not satisfy the node and is refused. The
// plain entry is the control.
func TestKeptNode_ReadyExprEntryDoesNotSatisfyDependsOn(t *testing.T) {
	build := keptApps(&stack.Node{Name: "cert-manager", Bundle: srBundle("cert-manager", cmApp("cert-manager-app"))})
	for _, expr := range append(readyExprs, "") {
		for _, form := range keptForms {
			t.Run(form+"/"+expr, func(t *testing.T) {
				rules := perLayoutNodeOnly()
				ml, c := keptIn(t, build, rules, "apps", form, func(k *kustv1.Kustomization) {
					k.Spec.DependsOn = []kustv1.DependencyReference{exprDep("prod-platform-node", expr)}
				}, nil)
				err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
				if expr == "" {
					if err != nil {
						t.Fatalf("got %v, want the plain entry admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "which lacks") || !strings.Contains(err.Error(), "named only with a readyExpr") {
					t.Fatalf("got %v, want the kept node refused for a dependency it names only with a readyExpr", err)
				}
			})
		}
	}
}

// Under UmbrellasDoNotWait the node apps asks for the umbrella platform,
// which applies a and b, where b depends on a with a readyExpr. A kept
// dependsOn on platform's terminal leaves satisfies it (R3): those are now a
// and b, so one naming b alone, which satisfied it while a was dropped, is
// refused. Naming both is admitted.
func TestKeptNode_UmbrellaLeavesIncludeReadyExprDependency(t *testing.T) {
	for _, expr := range readyExprs {
		build := func() *stack.Cluster {
			b := handWritten("b", "a")
			b.Spec.DependsOn[0].ReadyExpr = expr
			return keptApps(&stack.Node{Name: "p", Bundle: srBundle("p", objApp("a-app", handWritten("a")), objApp("b-app", b))})()
		}
		for _, form := range keptForms {
			for set, label := range settings {
				for _, c := range []struct {
					name    string
					entries []kustv1.DependencyReference
					admit   bool
				}{
					{"a+b", []kustv1.DependencyReference{{Name: "a"}, {Name: "b"}}, true},
					{"b alone", []kustv1.DependencyReference{{Name: "b"}}, false},
					// A leaf named only with a readyExpr waits for its
					// existence, not its Ready.
					{"a+b with a readyExpr", []kustv1.DependencyReference{{Name: "a"}, exprDep("b", expr)}, false},
				} {
					t.Run(label+"/"+form+"/"+expr+"/"+c.name, func(t *testing.T) {
						rules := perLayoutNodeOnly()
						ml, cl := keptIn(t, build, rules, "apps", form, func(k *kustv1.Kustomization) {
							k.Spec.DependsOn = c.entries
						}, nil)
						g := noWaitGenerator()
						g.AdditiveCELDependencyCheckOff = set
						err := fluxstack.NewLayoutIntegrator(g).IntegrateWithLayout(ml, cl, rules)
						if c.admit {
							if err != nil {
								t.Fatalf("got %v, want the kept dependsOn admitted", err)
							}
							return
						}
						if !errors.Is(err, fluxstack.ErrUmbrellaDependency) {
							t.Fatalf("got %v, want the kept dependsOn refused, wrapping ErrUmbrellaDependency", err)
						}
					})
				}
			}
		}
	}
}

// platform applies a and b, each depending on the other through a readyExpr,
// and no Kustomization depends on platform, so no dependsOn is rewritten.
// Terminal selection alone finds both leaves, but a readyExpr that requires
// Ready makes the two a deadlock, and the integration refuses it as it did
// before go-kure/kure#1190 ("no terminal leaf"). Only the exact "true" with
// AdditiveCELDependencyCheckOff waits for existence, which platform's apply
// creates: admitted. Standalone queries still return the leaves.
func TestUmbrellaRewrite_MutualReadyExprEntriesStayRefusedWithoutDependent(t *testing.T) {
	for _, expr := range readyExprs {
		for _, placement := range integratedPlacements {
			for set, label := range settings {
				t.Run(label+"/"+string(placement)+"/"+expr, func(t *testing.T) {
					a, b := handWritten("a", "b"), handWritten("b", "a")
					a.Spec.DependsOn[0].ReadyExpr = expr
					b.Spec.DependsOn[0].ReadyExpr = expr
					platform := srBundle("platform", objApp("a-app", a), objApp("b-app", b))
					g := noWaitGenerator()
					g.AdditiveCELDependencyCheckOff = set
					_, err := fluxstack.NewLayoutIntegrator(g).CreateLayoutWithResources(noWaitCluster(platform, false), withPlacement(nodeOnly, placement))
					if set && expr == "true" {
						if err != nil {
							t.Fatalf("got %v, want the existence-only tree admitted", err)
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
