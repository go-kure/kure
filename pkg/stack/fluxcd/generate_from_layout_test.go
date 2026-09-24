package fluxcd_test

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	sourcev1 "github.com/fluxcd/source-controller/api/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for deriving every Flux Kustomization spec.path from the layout tree:
// the directory of the layout that renders a bundle is its path, in every
// placement, grouping, ClusterName and tree shape.

// hookAugmenter is an application config that wants its own layout, attaches
// a values file and a configMapGenerator to it, and adds one child layout
// (the way a chart augmenter adds a hook group).
type hookAugmenter struct{ app string }

func (h *hookAugmenter) Generate(*stack.Application) ([]*client.Object, error) {
	return []*client.Object{cmObj(h.app + "-cm")}, nil
}

func (h *hookAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.ExtraFiles = append(ml.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
	ml.ConfigMapGenerators = append(ml.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: h.app + "-values", Files: []string{"values.yaml"}})
	hooks := &layout.ManifestLayout{
		Name:      h.app + "-hooks",
		Namespace: ml.FullRepoPath(),
		Resources: []client.Object{*cmObj(h.app + "-hook")},
	}
	ml.Children = append(ml.Children, hooks)
	return nil
}

func cmObj(name string) *client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("v1")
	u.SetKind("ConfigMap")
	u.SetName(name)
	u.SetNamespace("default")
	var o client.Object = u
	return &o
}

func cmApp(name string) *stack.Application {
	return stack.NewApplication(name, "default", &fakeAppConfig{objs: []*client.Object{cmObj(name + "-cm")}})
}

func srBundle(name string, apps ...*stack.Application) *stack.Bundle {
	return &stack.Bundle{Name: name, SourceRef: testSR(), Applications: apps}
}

// propertyShapes builds a fresh cluster per call (walking mutates bundles).
// Every tree is platform -> apps -> web, with the shape's feature on web, so a
// nodeFlat walk always merges it.
var propertyShapes = map[string]func() *stack.Cluster{
	"same-name": func() *stack.Cluster {
		return threeTier("platform", "apps", "web", nil)
	},
	"different-name": func() *stack.Cluster {
		return threeTier("platform-bundle", "apps-bundle", "web-bundle", nil)
	},
	"umbrella": func() *stack.Cluster {
		return threeTier("platform", "apps", "web", func(web *stack.Bundle) {
			web.Children = []*stack.Bundle{srBundle("web-api", cmApp("web-api-app")), srBundle("web-ui", cmApp("web-ui-app"))}
		})
	},
	"nested umbrella": func() *stack.Cluster {
		return threeTier("platform", "apps", "web", func(web *stack.Bundle) {
			api := srBundle("web-api", cmApp("web-api-app"))
			api.Children = []*stack.Bundle{srBundle("web-api-db", cmApp("web-api-db-app"))}
			web.Children = []*stack.Bundle{api}
		})
	},
	"augmenter": func() *stack.Cluster {
		return threeTier("platform", "apps", "web", func(web *stack.Bundle) {
			web.Applications = append(web.Applications, stack.NewApplication("web-chart", "default", &hookAugmenter{app: "web-chart"}))
		})
	},
	// A bundle-less root with one terminal child: the single tier collapses
	// wherever the root layout is top-level and has that child only.
	"FlattenSingleTier": func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
		root := &stack.Node{Name: "platform", Children: []*stack.Node{web}}
		return &stack.Cluster{Name: "demo", Node: root}
	},
}

func threeTier(platform, apps, web string, decorate func(*stack.Bundle)) *stack.Cluster {
	webB := srBundle(web, cmApp(web+"-app"))
	if decorate != nil {
		decorate(webB)
	}
	webN := &stack.Node{Name: "web", Bundle: webB}
	appsN := &stack.Node{Name: "apps", Bundle: srBundle(apps, cmApp(apps+"-app")), Children: []*stack.Node{webN}}
	root := &stack.Node{Name: "platform", Bundle: srBundle(platform, cmApp(platform+"-app")), Children: []*stack.Node{appsN}}
	return &stack.Cluster{Name: "demo", Node: root}
}

var propertyGroupings = map[string]layout.LayoutRules{
	"nodeOnly":    {BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat},
	"GroupByName": {BundleGrouping: layout.GroupByName, ApplicationGrouping: layout.GroupByName},
	"nodeFlat":    {NodeGrouping: layout.GroupFlat, BundleGrouping: layout.GroupFlat, ApplicationGrouping: layout.GroupFlat},
}

// allGroupings is every (Node, Bundle, App) grouping combination, named
// node_bundle_app; each axis is honoured on its own.
func allGroupings() map[string]layout.LayoutRules {
	out := map[string]layout.LayoutRules{}
	modes := []layout.GroupingMode{layout.GroupByName, layout.GroupFlat}
	for _, n := range modes {
		for _, b := range modes {
			for _, a := range modes {
				out[string(n)+"_"+string(b)+"_"+string(a)] = layout.LayoutRules{NodeGrouping: n, BundleGrouping: b, ApplicationGrouping: a}
			}
		}
	}
	return out
}

// reachableBundles returns every bundle reachable from c: node bundles and
// their umbrella descendants.
func reachableBundles(c *stack.Cluster) []*stack.Bundle {
	var out []*stack.Bundle
	var umb func(b *stack.Bundle)
	umb = func(b *stack.Bundle) {
		out = append(out, b)
		for _, ch := range b.Children {
			umb(ch)
		}
	}
	var nodes func(n *stack.Node)
	nodes = func(n *stack.Node) {
		if n.Bundle != nil {
			umb(n.Bundle)
		}
		for _, ch := range n.Children {
			nodes(ch)
		}
	}
	nodes(c.Node)
	return out
}

// kustomizations returns every Flux Kustomization CR in the tree.
func kustomizations(ml *layout.ManifestLayout) []*kustv1.Kustomization {
	var out []*kustv1.Kustomization
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			if k, ok := r.(*kustv1.Kustomization); ok {
				out = append(out, k)
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	return out
}

// expectedLayoutCRs is the PerLayout contract: every child layout that is not
// an umbrella child, not AppFileSingle and renders no bundle gets one CR; a
// bundle-less node layout's is named after its path plus "-node".
func expectedLayoutCRs(ml *layout.ManifestLayout) map[string]string {
	want := map[string]string{}
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		for _, c := range l.Children {
			if !c.UmbrellaChild && c.ApplicationFileMode != layout.AppFileSingle && len(c.OriginBundles()) == 0 {
				name := c.Name
				if len(c.OriginNodes()) > 0 {
					name = strings.ReplaceAll(c.FullRepoPath(), "/", "-") + "-node"
				}
				want[name] = c.FullRepoPath()
			}
			walk(c)
		}
	}
	walk(ml)
	return want
}

// writtenTree is one writer's output: root is the directory the Flux source
// is rooted at, tops the kustomization.yaml files it applies first.
type writtenTree struct {
	root string
	tops []string
}

func writeAll(t *testing.T, ml *layout.ManifestLayout) map[string]writtenTree {
	t.Helper()
	out := map[string]writtenTree{}

	disk := filepath.Join(t.TempDir(), "disk")
	if err := ml.WriteToDisk(disk); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}
	out["WriteToDisk"] = writtenTree{root: disk}

	var buf bytes.Buffer
	if err := ml.WriteToTar(&buf); err != nil {
		t.Fatalf("WriteToTar: %v", err)
	}
	tarDir := filepath.Join(t.TempDir(), "tar")
	extractTar(t, &buf, tarDir)
	out["WriteToTar"] = writtenTree{root: tarDir}

	base := filepath.Join(t.TempDir(), "manifest")
	cfg := layout.DefaultLayoutConfig()
	if err := layout.WriteManifest(base, cfg, ml); err != nil {
		t.Fatalf("WriteManifest: %v", err)
	}
	out["WriteManifest"] = writtenTree{root: filepath.Join(base, cfg.ManifestsDir)}

	for name, w := range out {
		top := filepath.Join(w.root, ml.FullRepoPath(), "kustomization.yaml")
		if _, err := os.Stat(top); err == nil {
			w.tops = []string{top}
		} else {
			// WriteManifest skips an empty cluster root: each child's
			// kustomization.yaml is then applied directly.
			for _, c := range ml.Children {
				w.tops = append(w.tops, filepath.Join(w.root, c.FullRepoPath(), "kustomization.yaml"))
			}
		}
		out[name] = w
	}
	return out
}

func extractTar(t *testing.T, r io.Reader, dir string) {
	t.Helper()
	tr := tar.NewReader(r)
	for {
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("tar: %v", err)
		}
		p := filepath.Join(dir, hdr.Name)
		if hdr.Typeflag == tar.TypeDir {
			if err := os.MkdirAll(p, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		data, err := io.ReadAll(tr)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// kustomizationRefs returns the `- ref` entries of a kustomization.yaml.
func kustomizationRefs(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var refs []string
	inResources := false
	for _, line := range strings.Split(string(data), "\n") {
		switch {
		case line == "resources:":
			inResources = true
		case strings.HasPrefix(line, "  - ") && inResources:
			refs = append(refs, strings.TrimPrefix(line, "  - "))
		case line != "" && !strings.HasPrefix(line, " "):
			inResources = false
		}
	}
	return refs
}

// checkWrittenTree asserts, for one writer's output: every resources entry
// exists; every file holding a Flux Kustomization is applied — reached from the
// top kustomization(s) through resources entries and through the spec.path of
// every Flux Kustomization already reached, as Flux applies them — or sits in
// flux-system; every expected directory was written. With exclusive set
// (FluxIntegratedPerLayout), a directory some Flux Kustomization applies is
// never also pulled in by a kustomization reference: it would be applied, and
// owned, twice.
func checkWrittenTree(t *testing.T, writer string, w writtenTree, dirs []string, exclusive bool) {
	t.Helper()
	crDirs := map[string]bool{}
	if exclusive {
		for _, d := range dirs {
			crDirs[filepath.Clean(d)] = true
		}
	}
	reached := map[string]bool{}
	var visit func(kust string)
	visitFile := func(p string) {
		if reached[p] {
			return
		}
		reached[p] = true
		for _, path := range fluxPaths(t, p) {
			kust := filepath.Join(w.root, path, "kustomization.yaml")
			if _, err := os.Stat(kust); err != nil {
				t.Errorf("%s: %s applies spec.path %q, which has no kustomization.yaml", writer, p, path)
				continue
			}
			visit(kust)
		}
	}
	visit = func(kust string) {
		if reached[kust] {
			return
		}
		reached[kust] = true
		// A listed directory without a kustomization.yaml is reported by
		// name, and the walk goes on, rather than aborting the subtest.
		if _, err := os.Stat(kust); err != nil {
			rel, _ := filepath.Rel(w.root, filepath.Dir(kust))
			t.Errorf("%s: directory %q is applied but has no kustomization.yaml", writer, rel)
			return
		}
		for _, ref := range kustomizationRefs(t, kust) {
			p := filepath.Join(filepath.Dir(kust), ref)
			info, err := os.Stat(p)
			if err != nil {
				rel, _ := filepath.Rel(w.root, p)
				t.Errorf("%s: %s lists %q, which does not exist", writer, kust, rel)
				continue
			}
			if info.IsDir() {
				if rel, _ := filepath.Rel(w.root, p); crDirs[rel] {
					t.Errorf("%s: %s lists directory %q, which its Flux Kustomization also applies", writer, kust, rel)
				}
				visit(filepath.Join(p, "kustomization.yaml"))
			} else {
				visitFile(p)
			}
		}
	}
	for _, top := range w.tops {
		visit(top)
	}
	err := filepath.Walk(w.root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() == "kustomization.yaml" {
			return err
		}
		if len(fluxPaths(t, p)) == 0 && !holdsFluxSource(t, p) {
			return nil
		}
		if !reached[p] && filepath.Base(filepath.Dir(p)) != fluxstack.DefaultFluxDirName {
			rel, _ := filepath.Rel(w.root, p)
			t.Errorf("%s: Flux Kustomization or Source file %s is not applied", writer, rel)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range dirs {
		if info, err := os.Stat(filepath.Join(w.root, d)); err != nil || !info.IsDir() {
			t.Errorf("%s: spec.path %q is not a written directory", writer, d)
		} else if _, err := os.Stat(filepath.Join(w.root, d, "kustomization.yaml")); err != nil {
			// An empty directory is not a directory in Git (or in most
			// artifacts built from one): Flux would find nothing there.
			t.Errorf("%s: spec.path %q has no kustomization.yaml", writer, d)
		} else if err := kustomizeBuild(filepath.Join(w.root, d)); err != nil {
			// What the kustomize-controller does with the directory.
			t.Errorf("%s: spec.path %q does not build: %v", writer, d, err)
		}
	}
}

// holdsFluxSource reports whether a manifest file holds a Flux source
// (GitRepository, OCIRepository, ...): a generated Kustomization references it,
// so it must be applied too.
func holdsFluxSource(t *testing.T, p string) bool {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	return strings.Contains(string(data), "apiVersion: source.toolkit.fluxcd.io/")
}

// kustomizeBuild runs a kustomize build of dir, as Flux's kustomize-controller
// does for a Kustomization's spec.path.
func kustomizeBuild(dir string) error {
	_, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(filesys.MakeFsOnDisk(), dir)
	return err
}

// fluxPaths returns the spec.path of every Flux Kustomization in a manifest
// file (a file may hold several documents).
func fluxPaths(t *testing.T, p string) []string {
	t.Helper()
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, doc := range strings.Split(string(data), "\n---\n") {
		var obj struct {
			APIVersion string `json:"apiVersion"`
			Kind       string `json:"kind"`
			Spec       struct {
				Path string `json:"path"`
			} `json:"spec"`
		}
		if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
			t.Fatalf("parse %s: %v", p, err)
		}
		if obj.Kind == "Kustomization" && strings.HasPrefix(obj.APIVersion, "kustomize.toolkit.fluxcd.io/") {
			out = append(out, obj.Spec.Path)
		}
	}
	return out
}

func TestEverySpecPathIsAWrittenDirectory(t *testing.T) {
	placements := []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle}
	shapes := slices.Sorted(maps.Keys(propertyShapes))
	groupingRules := allGroupings()
	groupings := slices.Sorted(maps.Keys(groupingRules))
	for _, placement := range placements {
		for _, grouping := range groupings {
			for _, clusterName := range []string{"", ".", "prod", "platform"} {
				for _, shape := range shapes {
					name := fmt.Sprintf("%s/%s/ClusterName=%q/%s", placement, grouping, clusterName, shape)
					t.Run(name, func(t *testing.T) {
						c := propertyShapes[shape]()
						rules := groupingRules[grouping]
						rules.FluxPlacement = placement
						rules.ClusterName = clusterName
						rules.FlattenSingleTier = shape == "FlattenSingleTier"
						checkEverySpecPath(t, c, rules)
					})
				}
			}
		}
	}
}

func checkEverySpecPath(t *testing.T, c *stack.Cluster, rules layout.LayoutRules) {
	t.Helper()
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	ml, err := integrator.CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	ix, err := layout.IndexOrigins(ml, c)
	if err != nil {
		t.Fatalf("IndexOrigins: %v", err)
	}

	bundles := map[string]*stack.Bundle{}
	for _, b := range reachableBundles(c) {
		bundles[b.Name] = b
	}
	crs := kustomizations(ml)
	seen := map[string]int{}
	layoutCRs := map[string]string{}
	var dirs []string
	for _, k := range crs {
		seen[k.Name]++
		if b, ok := bundles[k.Name]; ok {
			want := ix.BundleLayout(b).FullRepoPath()
			if k.Spec.Path != want {
				t.Errorf("bundle %s: spec.path %q, want its layout directory %q", k.Name, k.Spec.Path, want)
			}
			dirs = append(dirs, k.Spec.Path)
			continue
		}
		layoutCRs[k.Name] = k.Spec.Path
		dirs = append(dirs, k.Spec.Path)
	}
	for name, n := range seen {
		if n != 1 {
			t.Errorf("Kustomization %q emitted %d times, want once", name, n)
		}
	}
	for name := range bundles {
		if seen[name] == 0 {
			t.Errorf("bundle %s has no Kustomization", name)
		}
	}
	wantLayoutCRs := map[string]string{}
	if rules.FluxPlacement == layout.FluxIntegratedPerLayout {
		wantLayoutCRs = expectedLayoutCRs(ml)
	}
	if !mapsEqual(layoutCRs, wantLayoutCRs) {
		t.Errorf("layout CRs = %v, want %v", layoutCRs, wantLayoutCRs)
	}
	if rules.FluxPlacement != layout.FluxSeparate {
		for _, k := range crs {
			if k.Spec.SourceRef.Name == "" {
				t.Errorf("Kustomization %s has no sourceRef", k.Name)
			}
		}
	}
	for writer, w := range writeAll(t, ml) {
		checkWrittenTree(t, writer, w, dirs, rules.FluxPlacement == layout.FluxIntegratedPerLayout)
	}
}

func mapsEqual(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

func TestGenerateForBundle_UsesCallerPath(t *testing.T) {
	b := &stack.Bundle{Name: "web", SourceRef: &stack.SourceRef{Kind: "GitRepository", Name: "web-src", URL: "https://example.com/web.git", Branch: "main"}}
	objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(b, "clusters/prod/web")
	if err != nil {
		t.Fatal(err)
	}
	if len(objs) != 2 {
		t.Fatalf("got %d objects, want the Kustomization and its GitRepository", len(objs))
	}
	k, ok := objs[0].(*kustv1.Kustomization)
	if !ok {
		t.Fatalf("first object is %T, want a Kustomization", objs[0])
	}
	if k.Spec.Path != "clusters/prod/web" {
		t.Errorf("spec.path = %q, want the caller's path verbatim", k.Spec.Path)
	}
	if _, ok := objs[1].(*sourcev1.GitRepository); !ok {
		t.Errorf("second object is %T, want a GitRepository", objs[1])
	}
	if objs, err := fluxstack.NewResourceGenerator().GenerateForBundle(nil, "x"); err != nil || objs != nil {
		t.Errorf("GenerateForBundle(nil) = %v, %v; want nil, nil", objs, err)
	}
}

func crNames(objs []client.Object) []string {
	var out []string
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			out = append(out, k.Name)
		}
	}
	return out
}

func TestGenerateFromLayout_Order(t *testing.T) {
	build := func() *stack.Cluster {
		svc := srBundle("svc", cmApp("svc-app"))
		svc.Children = []*stack.Bundle{srBundle("svc-db", cmApp("svc-db-app"))}
		platform := srBundle("platform", stack.NewApplication("chart", "default", &hookAugmenter{app: "chart"}))
		platform.Children = []*stack.Bundle{svc}
		web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: platform, Children: []*stack.Node{web}}}
	}
	nodeOnly := propertyGroupings["nodeOnly"]

	// GenerateFromLayout: layout pre-order.
	c := build()
	ml, err := layout.WalkCluster(c, nodeOnly)
	if err != nil {
		t.Fatal(err)
	}
	objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := crNames(objs), []string{"platform", "svc", "svc-db", "web"}; !slices.Equal(got, want) {
		t.Errorf("GenerateFromLayout order = %v, want %v", got, want)
	}

	// PerLayout host order: the host's own bundle, then the layout CRs of its
	// children, then the bundle CRs of its children in tree order.
	c = build()
	rules := nodeOnly
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	ml, err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := crNames(ml.Resources), []string{"platform", "chart", "svc", "web"}; !slices.Equal(got, want) {
		t.Errorf("PerLayout root CR order = %v, want %v", got, want)
	}
}

func TestGenerateFromCluster_DefaultRules(t *testing.T) {
	web := &stack.Node{Name: "web", Bundle: srBundle("web-bundle", cmApp("web-app"))}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform-bundle"), Children: []*stack.Node{web}}}
	objs, err := fluxstack.NewResourceGenerator().GenerateFromCluster(c)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, o := range objs {
		if k, ok := o.(*kustv1.Kustomization); ok {
			got[k.Name] = k.Spec.Path
		}
	}
	// The directories WalkCluster writes under the default rules.
	want := map[string]string{"platform-bundle": "platform", "web-bundle": "platform/web"}
	if !mapsEqual(got, want) {
		t.Errorf("GenerateFromCluster paths = %v, want %v", got, want)
	}
}

func integrated(t *testing.T, c *stack.Cluster, rules layout.LayoutRules) *layout.ManifestLayout {
	t.Helper()
	ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
	if err != nil {
		t.Fatalf("CreateLayoutWithResources: %v", err)
	}
	return ml
}

func layoutAtPath(t *testing.T, root *layout.ManifestLayout, p string) *layout.ManifestLayout {
	t.Helper()
	var found *layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if found == nil && l.FullRepoPath() == p {
			found = l
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(root)
	if found == nil {
		t.Fatalf("no layout at %q", p)
	}
	return found
}

func TestIntegrateWithLayout_PerLayout_BundleLayoutGetsExactlyOneCR(t *testing.T) {
	web := &stack.Node{Name: "web", Bundle: srBundle("web-bundle", cmApp("web-app"))}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform-bundle", cmApp("core")), Children: []*stack.Node{web}}}
	rules := propertyGroupings["GroupByName"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	ml := integrated(t, c, rules)
	var paths []string
	for _, k := range kustomizations(ml) {
		if k.Name == "web-bundle" {
			paths = append(paths, k.Spec.Path)
		}
	}
	if !slices.Equal(paths, []string{"platform/web/web-bundle"}) {
		t.Errorf("web-bundle CRs have paths %v, want exactly one at platform/web/web-bundle", paths)
	}
	// Hosted by the bundle layout's parent (the web node layout).
	if got := crNames(layoutAtPath(t, ml, "platform/web").Resources); !slices.Contains(got, "web-bundle") {
		t.Errorf("platform/web hosts %v, want the web-bundle CR", got)
	}
}

func TestIntegrateWithLayout_Idempotent(t *testing.T) {
	// A flattened layout (the collapsed node's origins live on the absorber)
	// and an umbrella tree, integrated a second time.
	flattened := func() (*stack.Cluster, layout.LayoutRules) {
		c := &stack.Cluster{Name: "arc-runners", Node: &stack.Node{Name: "apps", Bundle: srBundle("bundle", cmApp("runner"))}}
		rules := propertyGroupings["nodeOnly"]
		rules.ClusterName = "arc-runners"
		rules.FlattenSingleTier = true
		return c, rules
	}
	umbrella := func() (*stack.Cluster, layout.LayoutRules) {
		rules := propertyGroupings["GroupByName"]
		rules.ClusterName = "prod"
		return propertyShapes["umbrella"](), rules
	}
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		for shape, build := range map[string]func() (*stack.Cluster, layout.LayoutRules){"flattened": flattened, "umbrella": umbrella} {
			t.Run(string(placement)+"/"+shape, func(t *testing.T) {
				c, rules := build()
				rules.FluxPlacement = placement
				checkIdempotent(t, c, rules)
			})
		}
	}
}

func checkIdempotent(t *testing.T, c *stack.Cluster, rules layout.LayoutRules) {
	t.Helper()
	ml := integrated(t, c, rules)
	before, objs := len(kustomizations(ml)), countResources(ml)
	children := len(ml.Children)
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	if err := integrator.IntegrateWithLayout(ml, c, rules); err != nil {
		t.Fatalf("second IntegrateWithLayout: %v", err)
	}
	if after := len(kustomizations(ml)); after != before {
		t.Errorf("second IntegrateWithLayout changed the CR count %d -> %d", before, after)
	}
	if after := countResources(ml); after != objs {
		t.Errorf("second IntegrateWithLayout changed the resource count %d -> %d", objs, after)
	}
	if len(ml.Children) != children {
		t.Errorf("second IntegrateWithLayout added root children: %d -> %d", children, len(ml.Children))
	}
}

func countResources(ml *layout.ManifestLayout) int {
	n := len(ml.Resources)
	for _, c := range ml.Children {
		n += countResources(c)
	}
	return n
}

func TestIntegrateWithLayout_PerLayout_BundlelessNodeGetsLayoutCR(t *testing.T) {
	perLayout := func(r layout.LayoutRules, clusterName string) layout.LayoutRules {
		r.FluxPlacement = layout.FluxIntegratedPerLayout
		r.ClusterName = clusterName
		return r
	}
	for _, tc := range []struct {
		name  string
		c     func() *stack.Cluster
		rules layout.LayoutRules
		want  map[string]string // CR name -> spec.path
	}{
		{
			name: "bundle-less root",
			c: func() *stack.Cluster {
				web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
				return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{web}}}
			},
			rules: perLayout(propertyGroupings["nodeOnly"], "prod"),
			want:  map[string]string{"prod-platform-node": "prod/platform", "web": "prod/platform/web"},
		},
		{
			name: "GroupByName node = bundle name, ClusterName dot",
			c: func() *stack.Cluster {
				web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
				return &stack.Cluster{Name: "demo", Node: &stack.Node{Children: []*stack.Node{web}}}
			},
			rules: perLayout(propertyGroupings["GroupByName"], "."),
			want:  map[string]string{"web-node": "web", "web": "web/web", "web-app": "web/web/web-app"},
		},
		{
			name: "GroupByName node = bundle name, ClusterName prod",
			c: func() *stack.Cluster {
				web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
				return &stack.Cluster{Name: "demo", Node: &stack.Node{Children: []*stack.Node{web}}}
			},
			rules: perLayout(propertyGroupings["GroupByName"], "prod"),
			want:  map[string]string{"prod-web-node": "prod/web", "web": "prod/web/web", "web-app": "prod/web/web/web-app"},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ml := integrated(t, tc.c(), tc.rules)
			got := map[string]string{}
			for _, k := range kustomizations(ml) {
				got[k.Name] = k.Spec.Path
				if k.Spec.SourceRef.Name != testSR().Name || k.Spec.SourceRef.Kind != testSR().Kind {
					t.Errorf("%s: sourceRef %+v, want the bundles' shared SourceRef", k.Name, k.Spec.SourceRef)
				}
			}
			if !mapsEqual(got, tc.want) {
				t.Errorf("CRs = %v, want %v", got, tc.want)
			}
			for writer, w := range writeAll(t, ml) {
				var dirs []string
				for _, k := range kustomizations(ml) {
					dirs = append(dirs, k.Spec.Path)
				}
				checkWrittenTree(t, writer, w, dirs, true)
			}
		})
	}
}

func TestIntegrateWithLayout_MixedSourcesErrors(t *testing.T) {
	other := &stack.SourceRef{Kind: "OCIRepository", Name: "other", Namespace: "flux-system"}
	build := func(augment bool) *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: other, Applications: []*stack.Application{cmApp("web-app")}}}
		apps := []*stack.Application{cmApp("core")}
		if augment {
			apps = append(apps, stack.NewApplication("chart", "default", &hookAugmenter{app: "chart"}))
		}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", apps...), Children: []*stack.Node{web}}}
	}
	rules := propertyGroupings["nodeFlat"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	_, err := integrator.CreateLayoutWithResources(build(true), rules)
	if err == nil || !strings.Contains(err.Error(), "different SourceRefs") {
		t.Fatalf("a merged layout with two sources that hosts layout CRs: got %v, want a different-SourceRefs error", err)
	}
	// Without layout CRs to host, the two bundle CRs keep their own sources.
	if _, err := integrator.CreateLayoutWithResources(build(false), rules); err != nil {
		t.Errorf("merged layout without layout CRs: %v", err)
	}
}

func TestIntegrateWithLayout_SameNameDifferentPathErrors(t *testing.T) {
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	t.Run("existing CR with another path", func(t *testing.T) {
		c := propertyShapes["different-name"]()
		ml := integrated(t, c, rules)
		for _, k := range kustomizations(ml) {
			if k.Name == "web-bundle" {
				k.Spec.Path = "somewhere/else"
			}
		}
		err := integrator.IntegrateWithLayout(ml, c, rules)
		if err == nil || !strings.Contains(err.Error(), `"web-bundle"`) || !strings.Contains(err.Error(), "somewhere/else") {
			t.Errorf("got %v, want an error naming web-bundle and the conflicting path", err)
		}
	})
	t.Run("layout CR named like a bundle", func(t *testing.T) {
		web := &stack.Node{Name: "web", Bundle: srBundle("web", cmApp("web-app"))}
		c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform",
			Bundle:   srBundle("platform", stack.NewApplication("web", "default", &hookAugmenter{app: "web"})),
			Children: []*stack.Node{web}}}
		_, err := integrator.CreateLayoutWithResources(c, rules)
		if err == nil || !strings.Contains(err.Error(), `Flux Kustomization name "web"`) {
			t.Errorf("got %v, want a CR name collision error", err)
		}
	})
}

func TestIntegrateWithLayout_FlattenSingleTier_PathIsPostCollapseDir(t *testing.T) {
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			c := &stack.Cluster{Name: "arc-runners", Node: &stack.Node{Name: "apps", Bundle: &stack.Bundle{
				Name:         "bundle",
				SourceRef:    &stack.SourceRef{Kind: "GitRepository", Name: "test-source", Namespace: "flux-system"},
				Applications: []*stack.Application{cmApp("runner")},
			}}}
			rules := propertyGroupings["nodeOnly"]
			rules.ClusterName = "arc-runners"
			rules.FlattenSingleTier = true
			rules.FluxPlacement = placement
			ml := integrated(t, c, rules)
			var got []string
			for _, k := range kustomizations(ml) {
				got = append(got, k.Name+"="+k.Spec.Path)
			}
			if !slices.Equal(got, []string{"bundle=arc-runners"}) {
				t.Errorf("CRs %v, want bundle=arc-runners (the post-collapse directory)", got)
			}
		})
	}
}

func TestIntegrateWithLayout_HandBuiltWithoutOrigins_Rejected(t *testing.T) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform")}}
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		hand := &layout.ManifestLayout{Name: "platform", Namespace: "."}
		err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(hand, c, layout.LayoutRules{FluxPlacement: placement})
		if err == nil || !strings.Contains(err.Error(), "build it with layout.WalkCluster") {
			t.Errorf("%s: hand-built layout: got %v, want the origins refusal", placement, err)
		}
	}
}

func TestFluxSeparate_ClusterNamePathsResolve(t *testing.T) {
	for _, clusterName := range []string{"", ".", "prod", "platform"} {
		t.Run(clusterName, func(t *testing.T) {
			c := propertyShapes["different-name"]()
			rules := propertyGroupings["GroupByName"]
			rules.ClusterName = clusterName
			rules.FluxPlacement = layout.FluxSeparate
			ml := integrated(t, c, rules)
			var dirs []string
			for _, k := range kustomizations(ml) {
				dirs = append(dirs, k.Spec.Path)
			}
			sort.Strings(dirs)
			if len(dirs) != 3 {
				t.Fatalf("got CR paths %v, want three bundles", dirs)
			}
			for writer, w := range writeAll(t, ml) {
				checkWrittenTree(t, writer, w, dirs, false)
			}
		})
	}
}

// setRecursive writes every layout that has children in KustomizationRecursive
// mode, which lists child references instead of the layout's own files.
func setRecursive(ml *layout.ManifestLayout) {
	if len(ml.Children) > 0 {
		ml.Mode = layout.KustomizationRecursive
	}
	for _, c := range ml.Children {
		setRecursive(c)
	}
}

// TestPerLayoutRecursive_AppliesGeneratedSources: in Recursive mode a
// PerLayout layout lists the Flux objects it hosts in place of its child
// references — the Sources generated for a URL-bearing SourceRef included, or
// the Kustomization referencing them cannot reconcile.
func TestPerLayoutRecursive_AppliesGeneratedSources(t *testing.T) {
	for _, ref := range []*stack.SourceRef{
		{Kind: "GitRepository", Name: "web-git", Namespace: "flux-system", URL: "https://example.com/web.git", Branch: "main"},
		{Kind: "OCIRepository", Name: "web-oci", Namespace: "flux-system", URL: "oci://example.com/web", Tag: "v1"},
	} {
		t.Run(ref.Kind, func(t *testing.T) {
			web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: ref, Applications: []*stack.Application{cmApp("web-app")}}}
			root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{web}}
			rules := propertyGroupings["nodeOnly"]
			rules.ClusterName = "prod"
			rules.FluxPlacement = layout.FluxIntegratedPerLayout
			ml := integrated(t, &stack.Cluster{Name: "demo", Node: root}, rules)
			setRecursive(ml)
			var dirs []string
			for _, k := range kustomizations(ml) {
				dirs = append(dirs, k.Spec.Path)
			}
			for writer, w := range writeAll(t, ml) {
				checkWrittenTree(t, writer, w, dirs, true)
			}
		})
	}
}

// fluxKustomization returns an unstructured Flux Kustomization, as an
// application or a caller may emit one.
func fluxKustomization(name, path string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("kustomize.toolkit.fluxcd.io/v1")
	u.SetKind("Kustomization")
	u.SetName(name)
	u.SetNamespace("flux-system")
	_ = unstructured.SetNestedField(u.Object, path, "spec", "path")
	return u
}

// TestIntegrateWithLayout_RejectsExistingDuplicateCRs: a Kustomization name
// already present in the tree is a CR identity collision, whether the object
// is typed or unstructured and wherever it sits.
func TestIntegrateWithLayout_RejectsExistingDuplicateCRs(t *testing.T) {
	typed := func() client.Object {
		dup := &kustv1.Kustomization{}
		dup.Name = "web-bundle"
		dup.Namespace = "flux-system"
		dup.Spec.Path = "wrong/path"
		return dup
	}
	objects := map[string]func() client.Object{
		"typed":        typed,
		"unstructured": func() client.Object { return fluxKustomization("web-bundle", "wrong/path") },
	}
	hosts := map[string]func(ml *layout.ManifestLayout) *layout.ManifestLayout{
		// web-bundle's CR is hosted by the apps layout (ml.Children[0]) under
		// PerLayout, and by the web node layout under PerBundle.
		"child layout": func(ml *layout.ManifestLayout) *layout.ManifestLayout { return ml.Children[0] },
		"root layout":  func(ml *layout.ManifestLayout) *layout.ManifestLayout { return ml },
	}
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		for oname, obj := range objects {
			for hname, pick := range hosts {
				t.Run(string(placement)+"/"+oname+"/"+hname, func(t *testing.T) {
					rules := propertyGroupings["nodeOnly"]
					rules.FluxPlacement = placement
					c := propertyShapes["different-name"]()
					ml := integrated(t, c, rules)
					host := pick(ml)
					host.Resources = append(host.Resources, obj())
					err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
					if err == nil || !strings.Contains(err.Error(), `"web-bundle"`) {
						t.Errorf("got %v, want an error naming the duplicated web-bundle", err)
					}
				})
			}
		}
	}
}

// TestFluxSeparate_RejectsExistingCRCollision: an application that emits a
// Flux Kustomization with a generated CR's identity would make the root
// kustomize build register one id twice.
func TestFluxSeparate_RejectsExistingCRCollision(t *testing.T) {
	platformApp := stack.NewApplication("platform-ks", "default", &fakeAppConfig{objs: func() []*client.Object {
		o := fluxKustomization("web-bundle", "elsewhere")
		return []*client.Object{&o}
	}()})
	web := &stack.Node{Name: "web", Bundle: srBundle("web-bundle", cmApp("web-app"))}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", platformApp), Children: []*stack.Node{web}}}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxSeparate
	_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
	if err == nil || !strings.Contains(err.Error(), `"web-bundle"`) {
		t.Errorf("got %v, want an error naming the colliding web-bundle", err)
	}
}

// TestIntegrateWithLayout_ConflictingSourcesErrors: two bundles whose
// URL-bearing SourceRefs share a name but not a URL generate two Sources with
// one identity in one host. Keeping either would silently point the other
// bundle's Kustomization at the wrong artifact.
func TestIntegrateWithLayout_ConflictingSourcesErrors(t *testing.T) {
	ref := func(url string) *stack.SourceRef {
		return &stack.SourceRef{Kind: "GitRepository", Name: "shared", Namespace: "flux-system", URL: url, Branch: "main"}
	}
	build := func(urlB string) *stack.Cluster {
		a := &stack.Node{Name: "a", Bundle: &stack.Bundle{Name: "a", SourceRef: ref("https://example.com/a.git"), Applications: []*stack.Application{cmApp("a-app")}}}
		b := &stack.Node{Name: "b", Bundle: &stack.Bundle{Name: "b", SourceRef: ref(urlB), Applications: []*stack.Application{cmApp("b-app")}}}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Children: []*stack.Node{a, b}}}
	}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	if _, err := integrator.CreateLayoutWithResources(build("https://example.com/b.git"), rules); err == nil || !strings.Contains(err.Error(), `"shared"`) {
		t.Errorf("two different Sources named shared: got %v, want a conflict error naming it", err)
	}
	// The same Source twice is one object, kept once.
	ml, err := integrator.CreateLayoutWithResources(build("https://example.com/a.git"), rules)
	if err != nil {
		t.Fatalf("identical Sources: %v", err)
	}
	n := 0
	for _, r := range ml.Resources {
		if r.GetObjectKind().GroupVersionKind().Kind == "GitRepository" {
			n++
		}
	}
	if n != 1 {
		t.Errorf("root hosts %d GitRepository objects, want the identical Source once", n)
	}
}

// TestFluxSeparate_RejectsDuplicateCRInExistingFluxSystemSubtree: an earlier
// flux-system child is kept only when it is exactly what this integration
// generates; a layout added beneath it (here holding a copy of a generated
// Kustomization) is not something the integrator produced.
func TestFluxSeparate_RejectsDuplicateCRInExistingFluxSystemSubtree(t *testing.T) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("web", cmApp("web-app"))}}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxSeparate
	ml := integrated(t, c, rules)
	var fluxDir *layout.ManifestLayout
	for _, ch := range ml.Children {
		if ch.Name == fluxstack.DefaultFluxDirName {
			fluxDir = ch
		}
	}
	if fluxDir == nil {
		t.Fatal("no flux-system child")
	}
	fluxDir.Children = append(fluxDir.Children, &layout.ManifestLayout{
		Name:      "extra",
		Namespace: fluxDir.FullRepoPath(),
		Resources: []client.Object{fluxKustomization("web", "platform")},
	})
	err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil || !strings.Contains(err.Error(), fluxstack.DefaultFluxDirName) {
		t.Errorf("got %v, want a refusal of the modified flux-system child", err)
	}
}

// TestPerLayout_EmptyNodeBundleSurvivesGitTree: a bundle with no applications
// renders a directory with nothing in it. Its Kustomization still names that
// directory, so the writers give it a kustomization.yaml — an empty directory
// does not survive a Git tree.
func TestPerLayout_EmptyNodeBundleSurvivesGitTree(t *testing.T) {
	for _, grouping := range []string{"nodeOnly", "GroupByName"} {
		for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
			for _, clusterName := range []string{"", ".", "prod"} {
				t.Run(fmt.Sprintf("%s/%s/%q", grouping, placement, clusterName), func(t *testing.T) {
					web := &stack.Node{Name: "web", Bundle: srBundle("web")}
					c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{web}}}
					rules := propertyGroupings[grouping]
					rules.FluxPlacement = placement
					rules.ClusterName = clusterName
					ml := integrated(t, c, rules)
					var dirs []string
					for _, k := range kustomizations(ml) {
						dirs = append(dirs, k.Spec.Path)
					}
					for writer, w := range writeAll(t, ml) {
						checkWrittenTree(t, writer, w, dirs, placement == layout.FluxIntegratedPerLayout)
					}
				})
			}
		}
	}
}

// emptyAugmenter wants its own layout and adds an empty hook-group child.
type emptyAugmenter struct{}

func (emptyAugmenter) Generate(*stack.Application) ([]*client.Object, error) { return nil, nil }

func (emptyAugmenter) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.Children = append(ml.Children, &layout.ManifestLayout{Name: ml.Name + "-hooks", Namespace: ml.FullRepoPath(), FluxPlacement: ml.FluxPlacement})
	return nil
}

// TestPerLayout_EmptyBundlelessNodeSurvivesGitTree: every directory a
// PerLayout CR targets gets a kustomization.yaml — an empty bundle-less node,
// an application that renders nothing and an empty augmenter layout included.
func TestPerLayout_EmptyBundlelessNodeSurvivesGitTree(t *testing.T) {
	for _, grouping := range []string{"nodeOnly", "GroupByName"} {
		for _, clusterName := range []string{"", ".", "prod"} {
			t.Run(fmt.Sprintf("%s/%q", grouping, clusterName), func(t *testing.T) {
				empty := &stack.Node{Name: "empty"}
				chart := &stack.Node{Name: "chart", Bundle: srBundle("chart", stack.NewApplication("chart-app", "default", emptyAugmenter{}))}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{empty, chart}}}
				rules := propertyGroupings[grouping]
				rules.FluxPlacement = layout.FluxIntegratedPerLayout
				rules.ClusterName = clusterName
				if grouping == "GroupByName" {
					// The root node layout renders no bundle here (its bundle
					// has a layout of its own, with or without a ClusterName)
					// and nothing below "empty" has a SourceRef: its CR has no
					// source, which S5 refuses.
					_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
					if err == nil || !strings.Contains(err.Error(), `platform/empty" needs a Kustomization CR`) {
						t.Fatalf("got %v, want the no-source refusal for platform/empty", err)
					}
					return
				}
				ml := integrated(t, c, rules)
				var dirs []string
				for _, k := range kustomizations(ml) {
					dirs = append(dirs, k.Spec.Path)
				}
				for writer, w := range writeAll(t, ml) {
					checkWrittenTree(t, writer, w, dirs, true)
				}
			})
		}
	}
}

// TestFluxSeparate_RejectsRelocatedFluxSystem: an earlier flux-system child is
// kept only in the directory the parent's kustomization.yaml references;
// moved elsewhere, the parent's "- flux-system" reference would dangle.
func TestFluxSeparate_RejectsRelocatedFluxSystem(t *testing.T) {
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("web", cmApp("web-app"))}}
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxSeparate
	ml := integrated(t, c, rules)
	for _, ch := range ml.Children {
		if ch.Name == fluxstack.DefaultFluxDirName {
			ch.Namespace = "elsewhere"
		}
	}
	err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil || !strings.Contains(err.Error(), fluxstack.DefaultFluxDirName) {
		t.Errorf("got %v, want a refusal of the relocated flux-system child", err)
	}
}

// TestEmptyApplicationDirectoriesBuild pins that every directory the writers
// reference has a kustomization.yaml, including an application that renders
// nothing: with ApplicationGrouping by name it still gets a directory, in a
// bundle and inside an umbrella child, and its parent lists it.
func TestEmptyApplicationDirectoriesBuild(t *testing.T) {
	empty := func(name string) *stack.Application {
		return stack.NewApplication(name, "default", &fakeAppConfig{})
	}
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
		t.Run(string(placement), func(t *testing.T) {
			u := &stack.Bundle{Name: "u", SourceRef: testSR(), Applications: []*stack.Application{empty("u-empty"), cmApp("ua")}}
			b := srBundle("b", empty("b-empty"), cmApp("api"))
			b.Children = []*stack.Bundle{u}
			web := &stack.Node{Name: "web", Bundle: b}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{web}}}
			rules := propertyGroupings["GroupByName"]
			rules.FluxPlacement = placement
			ml := integrated(t, c, rules)
			for writer, w := range writeAll(t, ml) {
				checkWrittenTree(t, writer, w, nil, false)
			}
		})
	}
}
