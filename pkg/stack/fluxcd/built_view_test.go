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
)

// Each Flux Kustomization is read as its writer's apply makes it
// (go-kure/kure#1094): its type after substitution, its fields without what
// the controller drops before applying, only the copies an apply does not
// skip, and under every identity a build gives it.

// x's kind is ${KIND}, which platform's postBuild sets inline to
// Kustomization, so x is applied as a Flux Kustomization, and its dependsOn
// references DEP. With substituteFrom, DEP may be read from the cluster
// whatever its default, so what x waits for cannot be read and the tree is
// refused, naming DEP. With DEP set inline the tree is admitted.
func TestBuiltView_SubstitutedKindIsAKustomization(t *testing.T) {
	for _, depInline := range []bool{false, true} {
		for _, placement := range integratedPlacements {
			name := "DEP read from the cluster"
			if depInline {
				name = "DEP set inline"
			}
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
				inline := map[string]string{"KIND": "Kustomization"}
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
// one on web, no cycle closes and the tree is admitted.
func TestBuiltView_ARenamedMemberIsJudged(t *testing.T) {
	for _, dep := range []string{"x-web", "web"} {
		for _, placement := range integratedPlacements {
			t.Run("dependsOn "+dep+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.NamePrefix = webDirectory(t, placement), "x-"
				k.Spec.Patches = []kustomize.Patch{{
					Patch:  "- op: add\n  path: /spec/dependsOn\n  value:\n  - name: " + dep + "\n",
					Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
				}}
				platform := srBundle("platform", objApp("platform-app", k))
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, false), withPlacement(nodeOnly, placement))
				if dep == "web" {
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
