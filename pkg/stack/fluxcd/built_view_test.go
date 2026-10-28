package fluxcd_test

import (
	"fmt"
	"maps"
	"slices"
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
