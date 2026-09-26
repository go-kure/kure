package fluxcd_test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for KustomizationRecursive layouts in the trees the integrator builds
// (go-kure/kure#868): such a layout gets no kustomization.yaml, and the Flux
// Kustomization kure generated for it builds the directory as Flux does,
// from every .yaml and .yml file below it.

var placements = []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle}

// recursiveCluster: prod -> platform (bundle) -> web (bundle), each bundle
// with a URL-bearing GitRepository, so Sources are generated and hosted.
func recursiveCluster() *stack.Cluster {
	gitRef := func(name string) *stack.SourceRef {
		return &stack.SourceRef{Kind: "GitRepository", Name: name, Namespace: "flux-system", URL: "https://example.com/" + name + ".git", Branch: "main"}
	}
	web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: gitRef("web-git"), Applications: []*stack.Application{cmApp("web-app")}}}
	root := &stack.Node{Name: "platform", Bundle: &stack.Bundle{Name: "platform", SourceRef: gitRef("platform-git"), Applications: []*stack.Application{cmApp("core")}}, Children: []*stack.Node{web}}
	web.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// recursiveClusterSharedSource is recursiveCluster with web on platform's
// Source, so a build that holds both bundles holds two identical copies
// unless the integrator hosts it once, in the root.
func recursiveClusterSharedSource() *stack.Cluster {
	c := recursiveCluster()
	shared := *c.Node.Bundle.SourceRef
	c.Node.Children[0].Bundle.SourceRef = &shared
	return c
}

func recursiveRules(grouping string, placement layout.FluxPlacement) layout.LayoutRules {
	rules := propertyGroupings[grouping]
	rules.ClusterName = "prod"
	rules.FluxPlacement = placement
	return rules
}

// setRecursive sets KustomizationRecursive on every layout that has children.
func setRecursive(ml *layout.ManifestLayout) {
	if len(ml.Children) > 0 {
		ml.Mode = layout.KustomizationRecursive
	}
	for _, c := range ml.Children {
		setRecursive(c)
	}
}

// setTargetsRecursive sets KustomizationRecursive on every layout a generated
// Kustomization targets that has no other target below it: the shape the
// writers accept for a Flux build.
func setTargetsRecursive(ml *layout.ManifestLayout) (n int) {
	var targets []string
	for _, k := range kustomizations(ml) {
		targets = append(targets, filepath.Clean(k.Spec.Path))
	}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		p := l.FullRepoPath()
		if slices.Contains(targets, p) && !slices.ContainsFunc(targets, func(o string) bool { return strings.HasPrefix(o, p+"/") }) {
			l.Mode = layout.KustomizationRecursive
			n++
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return n
}

// writeErrs runs every writer on ml and returns their errors by name.
func writeErrs(t *testing.T, ml *layout.ManifestLayout) map[string]error {
	t.Helper()
	var buf bytes.Buffer
	return map[string]error{
		"WriteToDisk":   ml.WriteToDisk(filepath.Join(t.TempDir(), "disk")),
		"WriteToTar":    ml.WriteToTar(&buf),
		"WriteManifest": layout.WriteManifest(filepath.Join(t.TempDir(), "manifest"), layout.DefaultLayoutConfig(), ml),
	}
}

// TestRecursive_NestedTargetsRefusedInEveryPlacement: a Recursive layout with
// children has a generated target below it that no kustomization.yaml
// shields, so its Flux build would apply that target's objects a second time.
// Every writer refuses it, whatever the placement.
func TestRecursive_NestedTargetsRefusedInEveryPlacement(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			ml := integrated(t, recursiveCluster(), recursiveRules("nodeOnly", placement))
			setRecursive(ml)
			for writer, err := range writeErrs(t, ml) {
				if err == nil || !strings.Contains(err.Error(), "would include layout") {
					t.Errorf("%s: got %v, want a nested-target refusal", writer, err)
				}
			}
		})
	}
}

// TestRecursive_RootBuildRefused: the root directory is built too (the Flux
// bootstrap applies it), so a Recursive root over a generated target is
// refused even when every other layout is Explicit and the target writes its
// own kustomization.yaml.
func TestRecursive_RootBuildRefused(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			ml := integrated(t, recursiveCluster(), recursiveRules("nodeOnly", placement))
			ml.Mode = layout.KustomizationRecursive
			for writer, err := range writeErrs(t, ml) {
				if err == nil || !strings.Contains(err.Error(), fmt.Sprintf("the Flux build of KustomizationRecursive layout %q would include layout", ml.FullRepoPath())) {
					t.Errorf("%s: got %v, want the root build refused", writer, err)
				}
			}
		})
	}
}

// TestRecursive_EmptyTargetRefused: a bundle with no applications yet gets a
// generated Kustomization over its directory. Written Recursive, that
// directory holds no file, so the Kustomization would name a path the written
// output does not contain; every writer refuses it, naming the directory
// (go-kure/kure#904). Written Explicit, it gets "resources: []" and is kept.
func TestRecursive_EmptyTargetRefused(t *testing.T) {
	cluster := func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: srBundle("web")}
		root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{web}}
		web.SetParent(root)
		return &stack.Cluster{Name: "demo", Node: root}
	}
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			rules := recursiveRules("nodeOnly", placement)
			writeAll(t, integrated(t, cluster(), rules))

			ml := integrated(t, cluster(), rules)
			if setTargetsRecursive(ml) == 0 {
				t.Fatal("no target layout to make Recursive")
			}
			if web := layoutAtPath(t, ml, "prod/platform/web"); !web.FluxBuild() || web.Mode != layout.KustomizationRecursive {
				t.Fatalf("prod/platform/web: FluxBuild %v, Mode %q, want a marked Recursive target", web.FluxBuild(), web.Mode)
			}
			want := `layout "prod/platform/web" is KustomizationRecursive and a Flux Kustomization kure generated builds its directory, but it holds no file`
			for writer, err := range writeErrs(t, ml) {
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("%s: got %v, want it to contain %q", writer, err, want)
					continue
				}
				if !strings.Contains(err.Error(), "prod/platform/web'") {
					t.Errorf("%s: %v does not name the directory", writer, err)
				}
			}
		})
	}
}

// TestRecursive_BuildsLikeExplicit: for the shapes the writers accept, each
// Flux build of the Recursive output — the Flux generator writing the
// kustomization.yaml, then Flux's own build — applies the same objects as the
// same build of the Explicit output.
func TestRecursive_BuildsLikeExplicit(t *testing.T) {
	type testCase struct {
		grouping  string
		placement layout.FluxPlacement
	}
	var cases []testCase
	for _, grouping := range []string{"nodeOnly", "GroupByName"} {
		for _, placement := range placements {
			// GroupByName under PerLayout gives every application
			// directory a Kustomization, which the writers refuse below a
			// Recursive target.
			if grouping == "GroupByName" && placement == layout.FluxIntegratedPerLayout {
				continue
			}
			cases = append(cases, testCase{grouping, placement})
		}
	}
	for _, tc := range cases {
		for name, cluster := range map[string]func() *stack.Cluster{
			"distinct": recursiveCluster,
			// One Source for both bundles: every Flux build must hold one
			// copy of it, or the build fails on a duplicate object.
			"shared": recursiveClusterSharedSource,
		} {
			grouping, placement := tc.grouping, tc.placement
			t.Run(grouping+"/"+string(placement)+"/"+name, func(t *testing.T) {
				rules := recursiveRules(grouping, placement)
				explicit := integrated(t, cluster(), rules)
				recursive := integrated(t, cluster(), rules)
				if setTargetsRecursive(recursive) == 0 {
					t.Fatal("no target layout to make Recursive")
				}
				want := writeAll(t, explicit)
				got := writeAll(t, recursive)
				for writer, w := range want {
					builds := fluxBuilds(t, w, explicit)
					if len(builds) < 2 {
						t.Fatalf("%s: %d builds, want the root and at least one Kustomization", writer, len(builds))
					}
					for _, k := range kustomizations(recursive) {
						if _, err := os.Stat(filepath.Join(got[writer].root, k.Spec.Path, "kustomization.yaml")); err == nil && layoutAtPath(t, recursive, filepath.Clean(k.Spec.Path)).Mode == layout.KustomizationRecursive {
							t.Errorf("%s: Recursive target %q has a kustomization.yaml", writer, k.Spec.Path)
						}
					}
					applied := 0
					for dir, kust := range builds {
						wantObjs := fluxBuild(t, w.root, dir, kust)
						applied += len(wantObjs)
						gotObjs := fluxBuild(t, got[writer].root, dir, kust)
						if !equalObjectSets(wantObjs, gotObjs) {
							t.Errorf("%s: Flux build of %q differs:\nExplicit:  %v\nRecursive: %v", writer, dir, keys(wantObjs), keys(gotObjs))
						}
					}
					if applied == 0 {
						t.Errorf("%s: the Flux builds apply nothing", writer)
					}
				}
			})
		}
	}
}

// fluxBuilds maps every directory a Flux Kustomization builds, relative to
// the written root, to that Kustomization: each generated spec.path, and each
// top directory the bootstrap applies (built with an empty Kustomization).
func fluxBuilds(t *testing.T, w writtenTree, ml *layout.ManifestLayout) map[string]unstructured.Unstructured {
	t.Helper()
	out := map[string]unstructured.Unstructured{}
	for _, top := range w.tops {
		rel, err := filepath.Rel(w.root, filepath.Dir(top))
		if err != nil {
			t.Fatal(err)
		}
		out[rel] = unstructured.Unstructured{Object: map[string]any{}}
	}
	for _, k := range kustomizations(ml) {
		obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
		if err != nil {
			t.Fatal(err)
		}
		out[filepath.Clean(k.Spec.Path)] = unstructured.Unstructured{Object: obj}
	}
	return out
}

// fluxBuild builds dir of a copy of root as kustomize-controller does, and
// returns each object's YAML by its id. The copy keeps the generated
// kustomization.yaml out of the other builds.
func fluxBuild(t *testing.T, root, dir string, kust unstructured.Unstructured) map[string]string {
	t.Helper()
	tree := filepath.Join(t.TempDir(), "repo")
	if err := os.CopyFS(tree, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tree, dir)
	if _, err := fluxkustomize.NewGenerator(tree, kust).WriteFile(path); err != nil {
		t.Fatalf("Flux generator on %q: %v", dir, err)
	}
	rm, err := fluxkustomize.SecureBuild(tree, path, false)
	if err != nil {
		t.Fatalf("Flux build of %q: %v", dir, err)
	}
	out := map[string]string{}
	for _, r := range rm.Resources() {
		y, err := r.AsYAML()
		if err != nil {
			t.Fatal(err)
		}
		out[r.CurId().String()] = string(y)
	}
	return out
}

func equalObjectSets(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for id, y := range a {
		if b[id] != y {
			return false
		}
	}
	return true
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
