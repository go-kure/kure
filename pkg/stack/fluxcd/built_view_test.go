package fluxcd_test

import (
	"fmt"
	"maps"
	"path"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	metaapi "github.com/fluxcd/pkg/apis/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

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
// Flux substitutes the object's YAML text and then parses it, so KIND set to
// Kustomization in YAML quotes is read as Kustomization too.
// With DEP set inline the tree is admitted.
func TestBuiltView_SubstitutedKindIsAKustomization(t *testing.T) {
	for name, c := range map[string]struct {
		kind      string
		depInline bool
	}{
		"DEP read from the cluster":                     {kind: "Kustomization"},
		"DEP read from the cluster, KIND a newline":     {kind: "Kustomization\n"},
		"DEP read from the cluster, KIND single-quoted": {kind: "'Kustomization'"},
		"DEP read from the cluster, KIND double-quoted": {kind: `"Kustomization"`},
		"DEP set inline":                                {kind: "Kustomization", depInline: true},
		"DEP set inline, KIND single-quoted":            {kind: "'Kustomization'", depInline: true},
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
	// Without hw the tree is read only for shop's and platform's paths, and
	// shop keeps web: a tree where nothing creates web is refused.
	tree := func(hw *kustv1.Kustomization) *stack.Cluster {
		shop := umbrellaOf("shop", srBundle("web", cmApp("web-app")))
		if hw != nil {
			shop.Patches = []stack.Patch{{Patch: deleteKust("web"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "web"}}}
		}
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

// TestBuiltView_ABuildThatReadsOpenAPIIsRefused: hand-written builds web's
// directory with a component that a layout extra file writes. A component
// setting openapi would set kustomize's global schema for every later build
// in the process, and Flux isolates each build only by resetting that schema,
// which kure does not do: the build is not read, naming hand-written and the
// file, which is no failed build: Flux builds and applies it. The control is
// the same component without openapi: it is admitted.
func TestBuiltView_ABuildThatReadsOpenAPIIsRefused(t *testing.T) {
	for name, openAPI := range map[string]string{"sets openapi": "openapi:\n  version: v1.21.2\n", "no openapi": ""} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.Components = webDirectory(t, placement), []string{"extra"}
				cluster := noWaitCluster(srBundle("platform", objApp("platform-app", k)), false)
				rules := withPlacement(nodeOnly, placement)
				ml := mustWalk(t, cluster, rules)
				l := layoutAtPath(t, ml, k.Spec.Path)
				l.ExtraFiles = append(l.ExtraFiles,
					layout.ExtraFile{Name: "extra/kustomization.yaml", Content: []byte("apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\n" + openAPI + "resources:\n- added.yaml\n")},
					layout.ExtraFile{Name: "extra/added.yaml", Content: []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: added\n  namespace: default\n")})
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, cluster, rules)
				if openAPI == "" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				want := `Flux Kustomization "flux-system/hand-written"`
				if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "is not read") || strings.Contains(err.Error(), "never applies") || !strings.Contains(err.Error(), "extra/kustomization.yaml, which sets openapi") {
					t.Fatalf("got %v, want %s not read (not a failed build) for a build that reads extra/kustomization.yaml, which sets openapi", err, want)
				}
			})
		}
	}
}

// TestBuiltView_ABuildThatReadsARemoteReferenceIsNotRead: hand-written builds
// web's directory with a component that layout extra files write, and the
// component names a remote reference in a field kustomize fetches from: a Git
// base it clones, or a URL it requests. Flux fetches them; kure never reaches
// the network during an integration (go-kure/kure#1168), so the build is not
// read, naming the file and the reference, which is no failed build. The
// witness shows that nothing ran git or sent a request. The controls run
// without either: a local resource, admitted, and a standalone
// ConfigMapGenerator whose singular env kustomize does not read there, which
// is not read only as a build that may merge into a Secret, as every
// generators entry is (go-kure/kure#1181), and never for its env.
func TestBuiltView_ABuildThatReadsARemoteReferenceIsNotRead(t *testing.T) {
	const component = "apiVersion: kustomize.config.k8s.io/v1alpha1\nkind: Component\n"
	const patchTransformer = "apiVersion: builtin\nkind: PatchTransformer\nmetadata:\n  name: remote-patch\npath: %s/p.yaml\ntarget:\n  kind: ConfigMap\n"
	for _, tc := range []struct {
		name  string
		files func(w *fluxstack.NetworkWitness) map[string]string
		file  string // the file the refusal names; "" admits the tree
		ref   func(w *fluxstack.NetworkWitness) string
		// merge: the build is not read only as one that may merge into a
		// Secret, naming no remote reference.
		merge bool
	}{
		{name: "a git base over file", file: "extra/kustomization.yaml",
			files: func(*fluxstack.NetworkWitness) map[string]string {
				return map[string]string{"extra/kustomization.yaml": component + "resources:\n- file:///nonexistent-kure-1168/repo//base?ref=v1\n"}
			},
			ref: func(*fluxstack.NetworkWitness) string { return "file:///nonexistent-kure-1168/repo//base?ref=v1" }},
		{name: "b git base over https", file: "extra/kustomization.yaml",
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				return map[string]string{"extra/kustomization.yaml": component + "resources:\n- " + w.URL + "/org/repo//base?ref=v1\n"}
			},
			ref: func(w *fluxstack.NetworkWitness) string { return w.URL + "/org/repo//base?ref=v1" }},
		{name: "c resource URL", file: "extra/kustomization.yaml",
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				return map[string]string{"extra/kustomization.yaml": component + "resources:\n- " + w.URL + "/cm.yaml\n"}
			},
			ref: func(w *fluxstack.NetworkWitness) string { return w.URL + "/cm.yaml" }},
		{name: "d patch path URL", file: "extra/kustomization.yaml",
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				return map[string]string{"extra/kustomization.yaml": component + "patches:\n- path: " + w.URL + "/p.yaml\n"}
			},
			ref: func(w *fluxstack.NetworkWitness) string { return w.URL + "/p.yaml" }},
		{name: "e transformer config file", file: "extra/pt.yaml",
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				return map[string]string{
					"extra/kustomization.yaml": component + "transformers:\n- pt.yaml\n",
					"extra/pt.yaml":            fmt.Sprintf(patchTransformer, w.URL),
				}
			},
			ref: func(w *fluxstack.NetworkWitness) string { return w.URL + "/p.yaml" }},
		{name: "f generator file source", file: "extra/kustomization.yaml",
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				return map[string]string{"extra/kustomization.yaml": component + "configMapGenerator:\n- name: generated\n  namespace: default\n  files:\n  - k=" + w.URL + "/f\n"}
			},
			ref: func(w *fluxstack.NetworkWitness) string { return w.URL + "/f" }},
		{name: "g inline transformer config", file: "extra/kustomization.yaml",
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				inline := strings.ReplaceAll(strings.TrimSuffix(fmt.Sprintf(patchTransformer, w.URL), "\n"), "\n", "\n  ")
				return map[string]string{"extra/kustomization.yaml": component + "transformers:\n- |\n  " + inline + "\n"}
			},
			ref: func(w *fluxstack.NetworkWitness) string { return w.URL + "/p.yaml" }},
		{name: "control local resource",
			files: func(*fluxstack.NetworkWitness) map[string]string {
				return map[string]string{
					"extra/kustomization.yaml": component + "resources:\n- added.yaml\n",
					"extra/added.yaml":         "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: added\n  namespace: default\n",
				}
			}},
		{name: "control standalone generator env", merge: true,
			files: func(w *fluxstack.NetworkWitness) map[string]string {
				return map[string]string{
					"extra/kustomization.yaml": component + "generators:\n- gen.yaml\n",
					"extra/gen.yaml":           "apiVersion: builtin\nkind: ConfigMapGenerator\nmetadata:\n  name: generated\n  namespace: default\nliterals:\n- k=v\nenv: " + w.URL + "/e\n",
				}
			}},
	} {
		for _, placement := range integratedPlacements {
			t.Run(tc.name+"/"+string(placement), func(t *testing.T) {
				w := fluxstack.NewNetworkWitness(t)
				k := handWritten("hand-written")
				k.Spec.Path, k.Spec.Components = webDirectory(t, placement), []string{"extra"}
				cluster := noWaitCluster(srBundle("platform", objApp("platform-app", k)), false)
				rules := withPlacement(nodeOnly, placement)
				ml := mustWalk(t, cluster, rules)
				l := layoutAtPath(t, ml, k.Spec.Path)
				files := tc.files(w)
				for _, name := range slices.Sorted(maps.Keys(files)) {
					l.ExtraFiles = append(l.ExtraFiles, layout.ExtraFile{Name: name, Content: []byte(files[name])})
				}
				err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).IntegrateWithLayout(ml, cluster, rules)
				w.Check(t)
				if tc.merge {
					if err == nil || !strings.Contains(err.Error(), "may merge into a Secret") || strings.Contains(err.Error(), "remote reference") || strings.Contains(err.Error(), w.URL) {
						t.Fatalf("got %v, want the build not read only as one that may merge into a Secret, naming no remote reference", err)
					}
					return
				}
				if tc.file == "" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				want := `Flux Kustomization "flux-system/hand-written"`
				if err == nil || !strings.Contains(err.Error(), want) || !strings.Contains(err.Error(), "is not read") || strings.Contains(err.Error(), "never applies") ||
					!strings.Contains(err.Error(), tc.file) || !strings.Contains(err.Error(), tc.ref(w)) {
					t.Fatalf("got %v, want %s not read (not a failed build), naming %s and %s", err, want, tc.file, tc.ref(w))
				}
			})
		}
	}
}

// TestBuiltView_ACopyAnotherBuildAppliesIsBuilt: platform's application
// emits h, which carries kustomize.toolkit.fluxcd.io/ssa Ignore, so
// platform's apply skips it, and z-builder, which builds platform's
// directory with a patch removing that annotation: z-builder's apply alone
// creates h, and h is built from z-builder's copy, though platform's skipped
// one was known first. h waits for platform, which its build of the
// directory holding platform writes, and platform waits for z-builder, which
// waits for h: the cycle is refused. Without z-builder's patch every apply
// skips h, nothing waits for it, and the tree is admitted.
func TestBuiltView_ACopyAnotherBuildAppliesIsBuilt(t *testing.T) {
	for name, unskipped := range map[string]bool{"z-builder applies h": true, "every apply skips h": false} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				h := handWritten("h")
				h.Spec.Path, h.Spec.Wait = webDirectory(t, placement), true
				h.Annotations = map[string]string{"kustomize.toolkit.fluxcd.io/ssa": "Ignore"}
				z := handWritten("z-builder")
				z.Spec.Path, z.Spec.Wait = platformDirectory(t, placement), true
				if unskipped {
					z.Spec.Patches = []kustomize.Patch{{
						Patch:  "- op: remove\n  path: /metadata/annotations/kustomize.toolkit.fluxcd.io~1ssa\n",
						Target: &kustomize.Selector{Kind: "Kustomization", Name: "h"},
					}}
				}
				platform := srBundle("platform", objApp("platform-app", h, z))
				platform.Wait = boolPtr(true)
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(platform, false), withPlacement(nodeOnly, placement))
				if !unskipped {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), "can never all become Ready") || !strings.Contains(err.Error(), "ready flux-system/h -> ready flux-system/platform") {
					t.Fatalf("got %v, want the cycle through h's wait refused", err)
				}
			})
		}
	}
}

// shopTree is demo -> shop, an umbrella whose one child is web, with patch,
// when set, on shop's copy of web.
func shopTree(patch string) *stack.Cluster {
	shop := umbrellaOf("shop", srBundle("web", cmApp("web-app")))
	if patch != "" {
		shop.Patches = []stack.Patch{{Patch: patch, Target: &stack.PatchSelector{Kind: "Kustomization", Name: "web"}}}
	}
	shopNode := &stack.Node{Name: "shop", Bundle: shop}
	root := &stack.Node{Name: "demo", Children: []*stack.Node{shopNode}}
	shopNode.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// TestBuiltView_AMemberAppliedElsewhereIsRefused: shop's patch gives its copy
// of web, the one its apply creates, a spec.kubeConfig or another Source.
// web then applies on another cluster, or another Source's directory, not
// what its directory in the tree holds, and the tree is refused, naming web
// and the field. A patch that changes neither is admitted.
func TestBuiltView_AMemberAppliedElsewhereIsRefused(t *testing.T) {
	for name, c := range map[string]struct{ patch, refused string }{
		"kubeConfig": {patch: "- op: add\n  path: /spec/kubeConfig\n  value:\n    secretRef:\n      name: remote-kubeconfig\n", refused: `Flux Kustomization "flux-system/web" is applied by the build of Flux Kustomization "flux-system/shop" with spec.kubeConfig`},
		"sourceRef":  {patch: "- op: replace\n  path: /spec/sourceRef/name\n  value: other-repo\n", refused: `Flux Kustomization "flux-system/web" is applied by the build of Flux Kustomization "flux-system/shop" with spec.sourceRef GitRepository flux-system/other-repo`},
		"force":      {patch: "- op: add\n  path: /spec/force\n  value: true\n"},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(shopTree(c.patch), withPlacement(nodeOnly, placement))
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

// TestBuiltView_AMemberMovedOutOfTheTreeIsReadAsApplied: shop's patch moves
// its copy of web to a spec.path that is no directory of the tree. web is
// read from that copy, which builds nothing of the tree, and not built again
// from its spec as the tree holds it, so nothing disagrees and the tree is
// admitted.
func TestBuiltView_AMemberMovedOutOfTheTreeIsReadAsApplied(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(shopTree("- op: replace\n  path: /spec/path\n  value: ./outside\n"), withPlacement(nodeOnly, placement))
			if err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// TestBuiltView_AMemberNoApplyCreatesIsRefused: shop's patch deletes web from
// shop's build, and no other build holds it, so no apply creates web: what
// its spec as the tree holds it builds is applied by nothing, and the tree is
// refused, naming web. Without the patch shop creates web and the tree is
// admitted.
func TestBuiltView_AMemberNoApplyCreatesIsRefused(t *testing.T) {
	for name, patch := range map[string]string{"deleted": deleteKust("web"), "kept": ""} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(shopTree(patch), withPlacement(nodeOnly, placement))
				if patch == "" {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				const refused = `Flux Kustomization "flux-system/web" is created by no apply`
				if err == nil || !strings.Contains(err.Error(), refused) {
					t.Fatalf("got %v, want %s", err, refused)
				}
			})
		}
	}
}

// TestBuiltView_IgnoreMissingComponentsIsCompared: hand-written builds web's
// directory with a patch setting web's spec.ignoreMissingComponents, which
// decides whether Flux drops a component it cannot find or fails the build:
// both writers apply web with different specs and the tree is refused, naming
// the field. Without the patch both copies agree and the tree is admitted.
func TestBuiltView_IgnoreMissingComponentsIsCompared(t *testing.T) {
	for name, patched := range map[string]bool{"set": true, "not set": false} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path = webDirectory(t, placement)
				if patched {
					k.Spec.Patches = []kustomize.Patch{{
						Patch:  "- op: add\n  path: /spec/ignoreMissingComponents\n  value: true\n",
						Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
					}}
				}
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", objApp("platform-app", k)), false), withPlacement(nodeOnly, placement))
				if !patched {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), `Flux Kustomization "flux-system/web" is written by`) || !strings.Contains(err.Error(), "spec.ignoreMissingComponents") {
					t.Fatalf("got %v, want the two writers of web refused on spec.ignoreMissingComponents", err)
				}
			})
		}
	}
}

// TestBuiltView_ImagesAreCompared: hand-written builds web's directory with a
// patch setting web's spec.images, which Flux's generator writes into web's
// build, where an image transformer may rewrite any field a component's
// configuration names, a dependsOn included: both writers apply web with
// different specs and the tree is refused, naming the field. Without the
// patch both copies agree and the tree is admitted.
func TestBuiltView_ImagesAreCompared(t *testing.T) {
	for name, patched := range map[string]bool{"set": true, "not set": false} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				k := handWritten("hand-written")
				k.Spec.Path = webDirectory(t, placement)
				if patched {
					k.Spec.Patches = []kustomize.Patch{{
						Patch:  "- op: add\n  path: /spec/images\n  value:\n  - name: safe\n    newName: m\n",
						Target: &kustomize.Selector{Kind: "Kustomization", Name: "web"},
					}}
				}
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(noWaitCluster(srBundle("platform", objApp("platform-app", k)), false), withPlacement(nodeOnly, placement))
				if !patched {
					if err != nil {
						t.Fatalf("got %v, want the tree admitted", err)
					}
					return
				}
				if err == nil || !strings.Contains(err.Error(), `Flux Kustomization "flux-system/web" is written by`) || !strings.Contains(err.Error(), "spec.images") {
					t.Fatalf("got %v, want the two writers of web refused on spec.images", err)
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

// shopDirectory is shop's spec.path in shopTree, read from an integration
// without the patch.
func shopDirectory(t *testing.T, placement layout.FluxPlacement) string {
	t.Helper()
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(shopTree(""), withPlacement(nodeOnly, placement))
	if err != nil {
		t.Fatalf("without the patch: %v", err)
	}
	return findKept(ml, "shop").Spec.Path
}

// rescueTree integrates c with rescue, a Kustomization the Flux bootstrap
// applies (go-kure/kure#1174): no dependsOn, on shop's Source, building
// shop's directory, added to the root layout and edited by edit.
func rescueTree(t *testing.T, c *stack.Cluster, placement layout.FluxPlacement, edit func(r *kustv1.Kustomization)) error {
	t.Helper()
	r := handWritten("rescue")
	r.Spec.Path = shopDirectory(t, placement)
	if edit != nil {
		edit(r)
	}
	return integrateWalked(t, c, placement, func(ml *layout.ManifestLayout) {
		ml.Resources = append(ml.Resources, r)
	})
}

// TestBuiltView_ABootstrapWrittenKustomizationCreatesAMember: shop deletes
// web from its build, and rescue, which the Flux bootstrap applies, builds
// shop's directory without that patch, so its apply creates web. rescue is on
// shop's Source and shop's build reads that directory, so kure reads
// rescue's build as the tree's and the tree is admitted (go-kure/kure#1174).
func TestBuiltView_ABootstrapWrittenKustomizationCreatesAMember(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			if err := rescueTree(t, shopTree(deleteKust("web")), placement, nil); err != nil {
				t.Fatalf("got %v, want the tree admitted", err)
			}
		})
	}
}

// TestBuiltView_AnUnreadBootstrapWrittenKustomizationIsNamed: a
// Kustomization the Flux bootstrap applies is read as a writer only on the
// Source of a member whose build reads its directory, and only on this
// cluster. rescue on a Source no member names, or on one only a member that
// does not read the directory names, and rescue with spec.kubeConfig, are not
// read: web is still created by no apply, and the refusal names rescue.
func TestBuiltView_AnUnreadBootstrapWrittenKustomizationIsNamed(t *testing.T) {
	otherRepo := func(r *kustv1.Kustomization) { r.Spec.SourceRef.Name = "other-repo" }
	withOther := func() *stack.Cluster {
		c := shopTree(deleteKust("web"))
		other := srBundle("other", cmApp("other-app"))
		other.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "other-repo", Namespace: "flux-system"}
		n := &stack.Node{Name: "other", Bundle: other}
		n.SetParent(c.Node)
		c.Node.Children = append(c.Node.Children, n)
		return c
	}
	for name, c := range map[string]struct {
		build func() *stack.Cluster
		edit  func(r *kustv1.Kustomization)
	}{
		"another Source": {build: func() *stack.Cluster { return shopTree(deleteKust("web")) }, edit: otherRepo},
		"kubeConfig": {build: func() *stack.Cluster { return shopTree(deleteKust("web")) }, edit: func(r *kustv1.Kustomization) {
			r.Spec.KubeConfig = &metaapi.KubeConfigReference{SecretRef: &metaapi.SecretKeyReference{Name: "remote"}}
		}},
		"a member's Source, not its build": {build: withOther, edit: otherRepo},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := rescueTree(t, c.build(), placement, c.edit)
				for _, want := range []string{`Flux Kustomization "flux-system/web" is created by no apply`, `flux-system/rescue, which the Flux bootstrap applies, may create it`} {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Fatalf("got %v, want %s", err, want)
					}
				}
			})
		}
	}
}

// TestBuiltView_ABootstrapWrittenKustomizationOnAFileIsNotRead: an
// AppFileSingle child is written as one file into its parent's directory and
// has no directory of its own, so a Kustomization the Flux bootstrap applies
// whose spec.path names it builds nothing. shop deletes web and helper from
// its build; helper, the AppFileSingle child of shop's directory, builds that
// directory without the patches and so would create web; rescue names
// helper's path on shop's Source. rescue is neither read as a writer nor named
// as one that may create web, and web is created by no apply.
func TestBuiltView_ABootstrapWrittenKustomizationOnAFileIsNotRead(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			c := shopTree("")
			c.Node.Children[0].Bundle.Patches = []stack.Patch{
				{Patch: deleteKust("web"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "web"}},
				{Patch: deleteKust("helper"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "helper"}},
			}
			dir := path.Clean(shopDirectory(t, placement))
			err := integrateWalked(t, c, placement, func(ml *layout.ManifestLayout) {
				d := layoutAtPath(t, ml, dir)
				helper := handWritten("helper")
				helper.Spec.Path = "./" + dir
				d.Children = append(d.Children, &layout.ManifestLayout{
					Name: "helper", Namespace: d.FullRepoPath(), ApplicationFileMode: layout.AppFileSingle,
					Resources: []client.Object{helper},
				})
				rescue := handWritten("rescue")
				rescue.Spec.Path = "./" + dir + "/helper"
				ml.Resources = append(ml.Resources, rescue)
			})
			if want := `Flux Kustomization "flux-system/web" is created by no apply`; err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("got %v, want %s", err, want)
			}
			if strings.Contains(err.Error(), "rescue") {
				t.Fatalf("got %v, want rescue, which names no directory, left unnamed", err)
			}
		})
	}
}

// TestBuiltView_ABootstrapWrittenCreatorIsJudged: rescue, the only apply that
// creates web, depends on web. web's apply waits for rescue's, which waits
// for web to become Ready: the creator is read, not assumed, and the cycle
// through it is refused.
func TestBuiltView_ABootstrapWrittenCreatorIsJudged(t *testing.T) {
	for _, placement := range integratedPlacements {
		t.Run(string(placement), func(t *testing.T) {
			err := rescueTree(t, shopTree(deleteKust("web")), placement, func(r *kustv1.Kustomization) {
				r.Spec.DependsOn = []kustv1.DependencyReference{{Name: "web"}}
			})
			if err == nil || !strings.Contains(err.Error(), "apply flux-system/rescue") {
				t.Fatalf("got %v, want the cycle through rescue's apply refused", err)
			}
		})
	}
}

// otherRepo is a second Source, beside testSR's.
func otherRepo() *stack.SourceRef {
	return &stack.SourceRef{Kind: "GitRepository", Name: "other-repo", Namespace: "flux-system"}
}

// secondViewTree integrates demo -> {shop, a, b[, d]} with the Kustomizations
// walked adds, under go-kure/kure#1174's second view. a, on testSR's Source,
// holds ConfigMap c, deletes it from its build and health-checks it; b, on
// other-repo, holds ConfigMap b-cm; d, also on other-repo, holds d-cm; shop
// deletes web, which rescue-shop, applied by the Flux bootstrap, creates, so
// the second view is built. patches holds each member's patches by name.
// walked receives the layout tree and a's, b's and d's directories.
func secondViewTree(t *testing.T, placement layout.FluxPlacement, withD bool, patches map[string][]stack.Patch, walked func(ml, a, b, d *layout.ManifestLayout)) error {
	t.Helper()
	a := srBundle("a", objApp("a-app", unstructuredObj("v1", "ConfigMap", "default", "c")))
	a.HealthChecks = []stack.HealthCheck{{APIVersion: "v1", Kind: "ConfigMap", Namespace: "default", Name: "c"}}
	b := srBundle("b", objApp("b-app", unstructuredObj("v1", "ConfigMap", "default", "b-cm")))
	b.SourceRef = otherRepo()
	c := shopTree(deleteKust("web"))
	members := []*stack.Bundle{a, b}
	if withD {
		d := srBundle("d", objApp("d-app", unstructuredObj("v1", "ConfigMap", "default", "d-cm")))
		d.SourceRef = otherRepo()
		members = append(members, d)
	}
	for _, m := range members {
		m.Patches = patches[m.Name]
		n := &stack.Node{Name: m.Name, Bundle: m}
		n.SetParent(c.Node)
		c.Node.Children = append(c.Node.Children, n)
	}
	rescue := handWritten("rescue-shop")
	rescue.Spec.Path = shopDirectory(t, placement)
	return integrateWalked(t, c, placement, func(ml *layout.ManifestLayout) {
		ml.Resources = append(ml.Resources, rescue)
		var d *layout.ManifestLayout
		if withD {
			d = findLayout(ml, "d-cm")
		}
		walked(ml, findLayout(ml, "c"), findLayout(ml, "b-cm"), d)
	})
}

// in returns k with spec.path the directory of l and spec.sourceRef src.
func in(k *kustv1.Kustomization, l *layout.ManifestLayout, src *stack.SourceRef) *kustv1.Kustomization {
	k.Spec.Path = "./" + l.FullRepoPath()
	k.Spec.SourceRef.Name = src.Name
	return k
}

// TestBuiltView_TheSecondViewKeepsTheFirstsRefusals: reading a Kustomization
// the Flux bootstrap applies as a writer is not monotone, so the second view
// (go-kure/kure#1174) never replaces the first: each refusal of the first
// stands. In each tree the first view refuses a cycle through a's health
// check on c, which the second view does not see:
//
//   - rescue, on b's Source with a longer spec.path, wins x's writer from w
//     and, on another Source than x, leaves x unread, so c's writer is
//     uncertain;
//   - the same through r, which rescue's build writes;
//   - x, every copy of which b's apply skips, is applied by rescue, which
//     strips the mark, and is then an unread source of c.
func TestBuiltView_TheSecondViewKeepsTheFirstsRefusals(t *testing.T) {
	deleteX := []stack.Patch{{Patch: deleteKust("x"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "x"}}}
	deleteC := []stack.Patch{{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: c\n  namespace: default\n$patch: delete\n", Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "c"}}}
	for name, c := range map[string]struct {
		withD   bool
		patches map[string][]stack.Patch
		walked  func(ml, a, b, d *layout.ManifestLayout)
		chain   string
	}{
		"rescue wins the writer": {
			patches: map[string][]stack.Patch{"a": deleteC, "b": deleteX},
			walked: func(ml, a, b, _ *layout.ManifestLayout) {
				a.Resources = append(a.Resources, in(handWritten("w"), b, testSR()))
				b.Resources = append(b.Resources, in(handWritten("x", "a"), a, testSR()))
				rescue := in(handWritten("rescue"), b, otherRepo())
				rescue.Spec.Path = "./" + rescue.Spec.Path
				ml.Resources = append(ml.Resources, rescue)
			},
			chain: "ready flux-system/a -> apply flux-system/x -> ready flux-system/a",
		},
		"a Kustomization rescue writes wins it": {
			withD:   true,
			patches: map[string][]stack.Patch{"a": deleteC, "b": deleteX, "d": {{Patch: deleteKust("r"), Target: &stack.PatchSelector{Kind: "Kustomization", Name: "r"}}}},
			walked: func(ml, a, b, d *layout.ManifestLayout) {
				a.Resources = append(a.Resources, in(handWritten("w"), b, testSR()))
				b.Resources = append(b.Resources, in(handWritten("x", "a"), a, testSR()))
				r := in(handWritten("r"), b, otherRepo())
				r.Spec.Path = "./" + r.Spec.Path
				d.Resources = append(d.Resources, r)
				ml.Resources = append(ml.Resources, in(handWritten("rescue"), d, otherRepo()))
			},
			chain: "ready flux-system/a -> apply flux-system/x -> ready flux-system/a",
		},
		"a skipped copy becomes unread": {
			patches: map[string][]stack.Patch{"a": deleteC, "b": {{
				Patch:  "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: x\n  namespace: flux-system\n  annotations:\n    kustomize.toolkit.fluxcd.io/ssa: Ignore\n",
				Target: &stack.PatchSelector{Kind: "Kustomization", Name: "x"},
			}}},
			walked: func(ml, a, b, _ *layout.ManifestLayout) {
				a.Resources = append(a.Resources, in(handWritten("w", "a"), a, testSR()))
				b.Resources = append(b.Resources, in(handWritten("x", "a"), a, testSR()))
				ml.Resources = append(ml.Resources, in(handWritten("rescue"), b, otherRepo()))
			},
			chain: "ready flux-system/a -> apply flux-system/w -> ready flux-system/a",
		},
	} {
		for _, placement := range integratedPlacements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				err := secondViewTree(t, placement, c.withD, c.patches, c.walked)
				if err == nil || !strings.Contains(err.Error(), c.chain) {
					t.Fatalf("got %v, want the cycle %s refused", err, c.chain)
				}
			})
		}
	}
}
