package fluxcd_test

import (
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Each Flux Kustomization is read as its writer's apply makes it
// (go-kure/kure#1094): its type after substitution, its fields without what
// the controller drops before applying, only the copies an apply does not
// skip, and under every identity a build gives it.

// x's kind is ${KIND}, which platform's postBuild sets inline to
// Kustomization, so x is applied as a Flux Kustomization, and its dependsOn
// references DEP. With substituteFrom, DEP may be read from the cluster
// whatever its default, so what x waits for cannot be read and the tree is
// refused, naming DEP. Flux removes every newline from an inline value before
// it substitutes, so KIND set to Kustomization and a newline is read the same.
// With DEP set inline the tree is admitted.
func TestBuiltView_SubstitutedKindIsAKustomization(t *testing.T) {
	for name, c := range map[string]struct {
		kind      string
		depInline bool
	}{
		"DEP read from the cluster":                 {kind: "Kustomization"},
		"DEP read from the cluster, KIND a newline": {kind: "Kustomization\n"},
		"DEP set inline":                            {kind: "Kustomization", depInline: true},
	} {
		depInline := c.depInline
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				u := unstructuredObj(kustv1.GroupVersion.String(), "${KIND}", "flux-system", "x")
				for _, f := range []struct {
					value any
					path  []string
				}{
					{"./elsewhere", []string{"spec", "path"}},
					{map[string]any{"kind": "GitRepository", "name": "flux-system"}, []string{"spec", "sourceRef"}},
					{[]any{map[string]any{"name": "${DEP:=safe}"}}, []string{"spec", "dependsOn"}},
				} {
					if err := unstructured.SetNestedField(u.Object, f.value, f.path...); err != nil {
						t.Fatalf("SetNestedField: %v", err)
					}
				}
				platform := srBundle("platform", objApp("platform-app", u))
				inline := map[string]string{"KIND": c.kind}
				if depInline {
					inline["DEP"] = "safe"
				}
				platform.PostBuild = &stack.PostBuild{Substitute: inline, SubstituteFrom: []stack.SubstituteRef{{Kind: "ConfigMap", Name: "vars"}}}
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
				if depInline {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), `references "DEP" in the Kustomization`) {
					t.Fatalf("got %v, want DEP in x refused", err)
				}
			})
		}
	}
}

// hand-written waits for web's directory, as in
// TestMissedCycles_NonMemberWaitIsACycleWithoutTheSetting, and its file
// carries a status or a creation timestamp that does not decode.
// kustomize-controller drops both before applying, so hand-written is read
// without them and the cycle is refused. With platform not waiting no cycle
// closes and the tree is admitted.
func TestBuiltView_DroppedFieldsDoNotHideAKustomization(t *testing.T) {
	for name, field := range map[string][]string{
		"a status":             {"status", "conditions"},
		"a creation timestamp": {"metadata", "creationTimestamp"},
	} {
		for _, wait := range []bool{true, false} {
			for _, placement := range integratedPlacements {
				t.Run(name+"/"+map[bool]string{true: "cycle", false: "acyclic"}[wait]+"/"+string(placement), func(t *testing.T) {
					k := handWritten("hand-written")
					k.Spec.Wait, k.Spec.Path = true, webDirectory(t, placement)
					content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
					if err != nil {
						t.Fatal(err)
					}
					u := &unstructured.Unstructured{Object: content}
					if err := unstructured.SetNestedField(u.Object, "invalid", field...); err != nil {
						t.Fatalf("SetNestedField: %v", err)
					}
					platform := srBundle("platform", objApp("platform-app", u))
					platform.Wait = boolPtr(wait)
					_, err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, true), withPlacement(nodeOnly, placement))
					if !wait {
						if err != nil {
							t.Fatalf("got %v, want the tree admitted", err)
						}
						return
					}
					if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/hand-written -> ready flux-system/") {
						t.Fatalf("got %v, want the cycle through hand-written's wait refused", err)
					}
				})
			}
		}
	}
}

// hand-written builds web's directory with the name prefix x-, and a patch on
// web, which its build holds, gives the copy x-web a dependsOn. x-web is web
// renamed: the tree holds no Kustomization by that name, so it cannot tell
// whose x-web is, and judges it as the integration's. With the dependsOn on
// x-web itself, x-web can never become Ready and the tree is refused; with
// one on web, no cycle closes and the tree is admitted. Where hand-written's
// apply skips every object of its build, kustomize.toolkit.fluxcd.io/ssa
// Ignore in its spec.commonMetadata, x-web is never created, no cycle runs
// through it, and the tree is admitted.
func TestBuiltView_ARenamedMemberIsJudged(t *testing.T) {
	for name, c := range map[string]struct {
		dep     string
		skipped bool
	}{
		"dependsOn x-web":          {dep: "x-web"},
		"dependsOn web":            {dep: "web"},
		"dependsOn x-web, skipped": {dep: "x-web", skipped: true},
	} {
		dep := c.dep
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.NamePrefix = webDirectory(t, placement), "x-"
				if c.skipped {
					k.Spec.CommonMetadata = &kustv1.CommonMetadata{Annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}}
				}
				k.Spec.Patches = []kustomize.Patch{{
					Patch:  "- op: add\n  path: /spec/dependsOn\n  value:\n  - name: " + dep + "\n",
					Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
				}}
				platform := srBundle("platform", objApp("platform-app", k))
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, false), withPlacement(nodeOnly, placement))
				if dep == "web" || c.skipped {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready: flux-system/x-web waits for itself (ready flux-system/x-web -> apply flux-system/x-web -> ready flux-system/x-web)") {
					t.Fatalf("got %v, want x-web's dependsOn on itself refused", err)
				}
			})
		}
	}
}

// hand-written builds web's directory with a patch giving web another
// dependsOn, so web has two writers that disagree: both apply it and keep
// overwriting each other, and the tree is refused. Where hand-written's apply
// skips its copy, with kustomize.toolkit.fluxcd.io/ssa Ignore on the copy or
// in its spec.commonMetadata, which Flux sets over every object of its build,
// only the other writer's copy is applied and the tree is admitted.
func TestBuiltView_ASkippedCopyDoesNotDisagree(t *testing.T) {
	for name, c := range map[string]struct {
		annotation string
		metadata   *kustv1.CommonMetadata
	}{
		"applied":                   {},
		"skipped by its annotation": {annotation: "- op: add\n  path: /metadata/annotations\n  value:\n    kustomize.toolkit.fluxcd.io/ssa: Ignore\n"},
		"skipped by commonMetadata": {metadata: &kustv1.CommonMetadata{Annotations: map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}}},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.CommonMetadata = webDirectory(t, placement), c.metadata
				k.Spec.Patches = []kustomize.Patch{{
					Patch:  "- op: add\n  path: /spec/dependsOn\n  value:\n  - name: platform\n" + c.annotation,
					Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
				}}
				platform := srBundle("platform", objApp("platform-app", k))
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, false), withPlacement(nodeOnly, placement))
				if name != "applied" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), `Flux Kustomization "flux-system/web" is written by`) {
					t.Fatalf("got %v, want the two writers of web refused", err)
				}
			})
		}
	}
}

// web carries kustomize.toolkit.fluxcd.io/ssa Ignore, so the bootstrap's
// apply skips its copy, and hand-written builds web's directory with a patch
// removing it: hand-written's apply alone creates web. With hand-written
// depending on web, web is never created and the tree is refused; without
// the annotation the bootstrap creates web and the tree is admitted. Where
// hand-written's copy also gives web wait and platform's directory as its
// path, web applies hand-written and, with hand-written's wait, each waits
// for the other; web was built from the bootstrap's copy, the first known,
// so what web applies cannot be read and the tree is refused. The control is
// the same tree with hand-written's copy identical to web's but for the
// annotation: it is admitted.
func TestBuiltView_TheOnlyApplyCreatesAndBuilds(t *testing.T) {
	const unskip = "- op: remove\n  path: /metadata/annotations/kustomize.toolkit.fluxcd.io~1ssa\n"
	for name, c := range map[string]struct {
		skipped, dependsOnWeb, rebuilt bool
		refused                        string
	}{
		"bootstrap skips web, dependsOn web":   {skipped: true, dependsOnWeb: true, refused: "can never all become Ready: flux-system/web waits for itself (ready flux-system/web -> apply flux-system/web -> apply flux-system/hand-written -> ready flux-system/web)"},
		"bootstrap applies web, dependsOn web": {dependsOnWeb: true},
		"bootstrap skips web, copy differs":    {skipped: true, rebuilt: true, refused: `Flux Kustomization "flux-system/web" is built from the copy the Flux bootstrap`},
		"bootstrap skips web, copy identical":  {skipped: true},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.Wait = webDirectory(t, placement), c.rebuilt
				if c.dependsOnWeb {
					k.Spec.DependsOn = []kustv1.DependencyReference{{Name: "web"}}
				}
				if c.skipped {
					patch := unskip
					if c.rebuilt {
						patch += "- op: add\n  path: /spec/wait\n  value: true\n- op: add\n  path: /spec/path\n  value: " + platformDirectory(t, placement) + "\n"
					}
					k.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"}}}
				}
				platform := srBundle("platform", objApp("platform-app", k))
				platform.Wait = boolPtr(false)
				cluster := noWaitCluster(platform, false)
				if c.skipped {
					cluster.Node.Children[1].Bundle.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
				}
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(cluster, withPlacement(nodeOnly, placement))
				if c.refused == "" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), c.refused) {
					t.Fatalf("got %v, want %s", err, c.refused)
				}
			})
		}
	}
}

// TestBuiltView_AMemberNoBuildHoldsIsComparedWithItsAppliedCopy: shop's
// patch deletes its child web from shop's build, so no build that the tree's
// results reach holds web, and web is built from its spec as the tree holds
// it. platform's application emits hand-written, building shop's directory
// without that patch: hand-written's apply alone creates web. Where
// hand-written's copy gives web wait and platform's directory as its path,
// web applies hand-written and, with hand-written's wait, each waits for the
// other; what web applies cannot be read from the spec it was built from, so
// the tree is refused. The control is the same tree with hand-written's copy
// of web as the tree holds it: it is admitted.
func TestBuiltView_AMemberNoBuildHoldsIsComparedWithItsAppliedCopy(t *testing.T) {
	tree := func(hw *kustv1.Kustomization) *stack.Cluster {
		shop := umbrellaOf("shop", srBundle("web", cmApp("web-app")))
		shop.Patches = []stack.Patch{{Patch: deleteKust("web"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "web"}}}
		app := cmApp("platform-app")
		if hw != nil {
			app = objApp("platform-app", hw)
		}
		apps := &stack.Node{Name: "apps", Bundle: srBundle("platform", app)}
		shopNode := &stack.Node{Name: "shop", Bundle: shop}
		root := &stack.Node{Name: "demo", Children: []*stack.Node{apps, shopNode}}
		apps.SetParent(root)
		shopNode.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	for name, rebuilt := range map[string]bool{"copy differs": true, "copy identical": false} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(tree(nil), withPlacement(nodeOnly, placement))
				if err != nil {
					t.Fatalf("without hand-written: %v", err)
				}
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.Wait = findKept(off, "shop").Spec.Path, true
				if rebuilt {
					patch := "- op: add\n  path: /spec/wait\n  value: true\n- op: replace\n  path: /spec/path\n  value: " + findKept(off, "platform").Spec.Path + "\n"
					k.Spec.Patches = []kustomize.Patch{{Patch: patch, Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"}}}
				}
				_, err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(tree(k), withPlacement(nodeOnly, placement))
				if !rebuilt {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				const refused = `Flux Kustomization "flux-system/web" is built from its spec as the tree holds it`
				if err == nil || !strings.Contains(err.Error(), refused) {
					t.Fatalf("got %v, want %s", err, refused)
				}
			})
		}
	}
}

// platformDirectory is the spec.path of platform in noWaitCluster.
func platformDirectory(t *testing.T, placement layout.FluxPlacement) string {
	t.Helper()
	off, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", cmApp("platform-app")), true), withPlacement(nodeOnly, placement))
	if err != nil {
		t.Fatalf("without the setting: %v", err)
	}
	return findKept(off, "platform").Spec.Path
}
