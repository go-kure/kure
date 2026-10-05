package fluxcd_test

import (
	"bytes"
	stderrors "errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/yaml"

	kerrors "github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for where a generated Source is hosted (go-kure/kure#979): in a build
// that is applied before any Kustomization that references it, and never
// inside a directory delivered through it.

func urlSR(name string) *stack.SourceRef {
	return &stack.SourceRef{Kind: "GitRepository", Name: name, Namespace: "flux-system", URL: "https://example.com/" + name + ".git", Branch: "main"}
}

func urlBundle(name, source string) *stack.Bundle {
	return &stack.Bundle{Name: name, SourceRef: urlSR(source), Applications: []*stack.Application{cmApp(name + "-app")}}
}

// sourceHostShapes are trees whose bundles take generated Sources: every
// SourceRef has a URL unless the shape says otherwise.
var sourceHostShapes = map[string]func() *stack.Cluster{
	"root bundle and child node, one Source": func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: urlBundle("web", "shared")}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: urlBundle("core", "shared"), Children: []*stack.Node{web}}}
	},
	"root bundle and child node, a Source each": func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: urlBundle("web", "web-src")}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: urlBundle("core", "core-src"), Children: []*stack.Node{web}}}
	},
	"root bundle with an umbrella child": func() *stack.Cluster {
		core := urlBundle("core", "shared")
		core.Children = []*stack.Bundle{urlBundle("core-db", "db-src")}
		web := &stack.Node{Name: "web", Bundle: urlBundle("web", "shared")}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: core, Children: []*stack.Node{web}}}
	},
	"root bundle beside a bundle-less child node": func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: urlBundle("web", "shared")}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: urlBundle("core", "shared"), Children: []*stack.Node{{Name: "empty"}, web}}}
	},
	"bundle-less root": nestedTree,
	// Node a's bundle names a Source the tree does not hold.
	"bundle-less root, one URL-less SourceRef": deepTree,
	"unnamed root with a bundle": func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: urlBundle("web", "shared")}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Bundle: urlBundle("core", "shared"), Children: []*stack.Node{web}}}
	},
	"unnamed bundle-less root": func() *stack.Cluster {
		web := &stack.Node{Name: "web", Bundle: urlBundle("web", "shared")}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Children: []*stack.Node{web}}}
	},
}

// fluxDoc is what the Source invariant reads of one written document, and of
// each item of a List.
type fluxDoc struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Metadata   struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Path      string `json:"path"`
		SourceRef struct {
			Kind      string `json:"kind"`
			Name      string `json:"name"`
			Namespace string `json:"namespace"`
		} `json:"sourceRef"`
	} `json:"spec"`
	Items []fluxDoc `json:"items"`
}

// writtenCR is a Flux Kustomization as written: the file that holds it, the
// directory it builds and the Source it names.
type writtenCR struct {
	file, name, path, source string
}

// readFluxObjects returns the Flux Kustomizations written below root, and the
// files that hold each Flux Source, keyed kind/namespace/name. The items of a
// List count as objects of the file that holds the List, as kustomize builds
// them; the items field of an object of any other kind does not.
func readFluxObjects(t *testing.T, root string) ([]writtenCR, map[string][]string) {
	t.Helper()
	var crs []writtenCR
	sources := map[string][]string{}
	var read func(p string, obj fluxDoc)
	read = func(p string, obj fluxDoc) {
		// A List is an envelope (a kind that ends in "List"): its items are
		// the objects. The items field of any other kind is that object's own.
		if strings.HasSuffix(obj.Kind, "List") {
			for _, item := range obj.Items {
				read(p, item)
			}
			return
		}
		switch {
		case obj.Kind == "Kustomization" && strings.HasPrefix(obj.APIVersion, "kustomize.toolkit.fluxcd.io/"):
			ns := obj.Spec.SourceRef.Namespace
			if ns == "" {
				ns = obj.Metadata.Namespace
			}
			crs = append(crs, writtenCR{file: p, name: obj.Metadata.Name, path: obj.Spec.Path,
				source: obj.Spec.SourceRef.Kind + "/" + ns + "/" + obj.Spec.SourceRef.Name})
		case strings.HasPrefix(obj.APIVersion, "source.toolkit.fluxcd.io/"):
			key := obj.Kind + "/" + obj.Metadata.Namespace + "/" + obj.Metadata.Name
			sources[key] = append(sources[key], p)
		}
	}
	err := filepath.Walk(root, func(p string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || info.Name() == "kustomization.yaml" || filepath.Ext(p) != ".yaml" {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		for _, doc := range strings.Split(string(data), "\n---\n") {
			var obj fluxDoc
			if err := yaml.Unmarshal([]byte(doc), &obj); err != nil {
				return fmt.Errorf("parse %s: %w", p, err)
			}
			read(p, obj)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return crs, sources
}

// checkSourceHosts asserts the Source invariant on one writer's output. A
// build is a directory something applies: the top of the written tree, and the
// spec.path of every Flux Kustomization. What a build delivers is the files it
// takes in and, through the Kustomizations among them, what their builds
// deliver. For every Kustomization that names a Source the tree holds:
//
//   - no file holding that Source is delivered by the Kustomization: with the
//     top of the written tree as the one thing applied from outside, it would
//     wait for a Source that only its own apply creates;
//   - some build takes in a file holding the Source and delivers the
//     Kustomization, so the Source is applied with it or before it.
func checkSourceHosts(t *testing.T, writer string, w writtenTree) {
	t.Helper()
	violations, checked := sourceHostViolations(t, w)
	for _, v := range violations {
		t.Errorf("%s: %s", writer, v)
	}
	if checked == 0 {
		t.Errorf("%s: no Kustomization names a Source the tree holds: the check met nothing", writer)
	}
}

// sourceHostViolations returns what in w breaks the Source invariant
// (checkSourceHosts), and how many Kustomizations it was checked on.
func sourceHostViolations(t *testing.T, w writtenTree) (violations []string, checked int) {
	t.Helper()
	crs, sources := readFluxObjects(t, w.root)
	rel := func(p string) string {
		r, _ := filepath.Rel(w.root, p)
		return r
	}

	takes := map[string]map[string]bool{}
	for _, top := range w.tops {
		takes["the top "+rel(filepath.Dir(top))] = buildFiles(t, filepath.Dir(top))
	}
	for _, cr := range crs {
		takes["Kustomization "+cr.name] = buildFiles(t, filepath.Join(w.root, cr.path))
	}
	delivers := func(build string) map[string]bool {
		out := map[string]bool{}
		var add func(build string)
		seen := map[string]bool{}
		add = func(build string) {
			if seen[build] {
				return
			}
			seen[build] = true
			for f := range takes[build] {
				out[f] = true
			}
			for _, cr := range crs {
				if takes[build][cr.file] {
					add("Kustomization " + cr.name)
				}
			}
		}
		add(build)
		return out
	}

	builds := make([]string, 0, len(takes))
	for b := range takes {
		builds = append(builds, b)
	}
	slices.Sort(builds)

	for _, cr := range crs {
		files := sources[cr.source]
		if len(files) == 0 {
			continue
		}
		checked++
		own := delivers("Kustomization " + cr.name)
		for _, f := range files {
			if own[f] {
				violations = append(violations, fmt.Sprintf("Kustomization %q (spec.path %q) takes its source from %s, which %s holds: a file the Kustomization itself delivers", cr.name, cr.path, cr.source, rel(f)))
			}
		}
		before := false
		for _, b := range builds {
			holds := false
			for _, f := range files {
				holds = holds || takes[b][f]
			}
			if holds && delivers(b)[cr.file] {
				before = true
			}
		}
		if !before {
			violations = append(violations, fmt.Sprintf("no build holds %s and delivers Kustomization %q (in %s), which takes its source from it", cr.source, cr.name, rel(cr.file)))
		}
	}
	return violations, checked
}

// TestCheckSourceHosts_SeesWhatBreaksTheInvariant is the control of
// checkSourceHosts, on trees written by hand: a Kustomization whose build
// holds its own Source is reported, as an object of its own and inside a List,
// and so is one whose Source no build delivers with it; a Source in the build
// that also holds the Kustomization is not, and a Source-shaped item of an
// object that is no List is not read as a Source.
func TestCheckSourceHosts_SeesWhatBreaksTheInvariant(t *testing.T) {
	const source = `apiVersion: source.toolkit.fluxcd.io/v1
kind: GitRepository
metadata:
  name: shared
  namespace: flux-system
`
	cr := func(indent, specPath string) string {
		lines := []string{
			"apiVersion: kustomize.toolkit.fluxcd.io/v1",
			"kind: Kustomization",
			"metadata:",
			"  name: core",
			"  namespace: flux-system",
			"spec:",
			"  path: " + specPath,
			"  sourceRef:",
			"    kind: GitRepository",
			"    name: shared",
		}
		return indent + strings.Join(lines, "\n"+indent) + "\n"
	}
	asList := func(specPath string) string {
		return "apiVersion: v1\nkind: List\nitems:\n- " + strings.TrimPrefix(cr("  ", specPath), "  ")
	}
	// A kustomization.yaml in the form the writers emit, which buildFiles
	// reads.
	lists := func(names ...string) string {
		out := "resources:\n"
		for _, n := range names {
			out += "  - " + n + "\n"
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		files map[string]string
		want  string
	}{
		{
			name: "the Source beside the Kustomization, which applies a directory below",
			files: map[string]string{
				"kustomization.yaml":               lists("source.yaml", "cr.yaml"),
				"source.yaml":                      source,
				"cr.yaml":                          cr("", "platform/core"),
				"platform/core/kustomization.yaml": lists(),
			},
		},
		{
			name: "the Source in the directory the Kustomization applies",
			files: map[string]string{
				"kustomization.yaml":          lists("cr.yaml"),
				"cr.yaml":                     cr("", "platform"),
				"platform/kustomization.yaml": lists("source.yaml"),
				"platform/source.yaml":        source,
			},
			want: "a file the Kustomization itself delivers",
		},
		{
			name: "the same, the Kustomization inside a List",
			files: map[string]string{
				"kustomization.yaml":          lists("cr.yaml"),
				"cr.yaml":                     asList("platform"),
				"platform/kustomization.yaml": lists("source.yaml"),
				"platform/source.yaml":        source,
			},
			want: "a file the Kustomization itself delivers",
		},
		{
			// Read as a Source, the Widget's item would be a copy in the
			// directory the Kustomization applies.
			name: "a Source-shaped item of an object that is no List",
			files: map[string]string{
				"kustomization.yaml":               lists("source.yaml", "cr.yaml"),
				"source.yaml":                      source,
				"cr.yaml":                          cr("", "platform/core"),
				"platform/core/kustomization.yaml": lists("widget.yaml"),
				"platform/core/widget.yaml":        "apiVersion: example.com/v1\nkind: Widget\nmetadata:\n  name: w\nitems:\n- " + strings.ReplaceAll(strings.TrimSuffix(source, "\n"), "\n", "\n  ") + "\n",
			},
		},
		{
			name: "the Source in a directory no build takes in",
			files: map[string]string{
				"kustomization.yaml":               lists("cr.yaml"),
				"cr.yaml":                          cr("", "platform/core"),
				"platform/core/kustomization.yaml": lists(),
				"elsewhere/source.yaml":            source,
			},
			want: "no build holds GitRepository/flux-system/shared and delivers Kustomization \"core\"",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for name, content := range tc.files {
				p := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			violations, checked := sourceHostViolations(t, writtenTree{root: root, tops: []string{filepath.Join(root, "kustomization.yaml")}})
			if checked != 1 {
				t.Fatalf("checked %d Kustomizations, want 1", checked)
			}
			if tc.want == "" {
				if len(violations) != 0 {
					t.Errorf("got %q, want no violation", violations)
				}
				return
			}
			if !slices.ContainsFunc(violations, func(v string) bool { return strings.Contains(v, tc.want) }) {
				t.Errorf("got %q, want a violation that says %q", violations, tc.want)
			}
		})
	}
}

// ownSourceRefusal is what the refusal of a Kustomization that would take a
// Source from inside what it applies says.
const ownSourceRefusal = "would take its source from"

// TestGeneratedSourceIsHostedBeforeItsKustomizations renders every shape under
// every placement, grouping and ClusterName, with every writer, and holds each
// written tree to the Source invariant (checkSourceHosts). A tree that cannot
// meet it is refused: under FluxIntegratedPerLayout a named root node below a
// ClusterName directory has a layout Kustomization, which applies the
// directory that hosts the generated Sources and so needs a Source the tree
// does not generate.
func TestGeneratedSourceIsHostedBeforeItsKustomizations(t *testing.T) {
	// No ClusterName directory, the root of the tree as one, and one and two
	// directories above the root node.
	clusterNames := []string{"", ".", "prod", "env/prod"}
	shapes := make([]string, 0, len(sourceHostShapes))
	for s := range sourceHostShapes {
		shapes = append(shapes, s)
	}
	slices.Sort(shapes)
	const urlLess = "bundle-less root, one URL-less SourceRef"
	// wantRefusal is what the refusal of a combination says, "" when the
	// tree is integrated. Every combination is one or the other.
	wantRefusal := func(placement layout.FluxPlacement, grouping, clusterName, shape string) string {
		perLayout := placement == layout.FluxIntegratedPerLayout
		switch {
		// The root node's layout has a Kustomization of its own, hosted in
		// the ClusterName directory above it, and every SourceRef has a URL.
		case perLayout && clusterName != "" && !strings.HasPrefix(shape, "unnamed") && shape != urlLess:
			return ownSourceRefusal
		// Not this invariant's: a flat NodeGrouping merges bundles with
		// different SourceRefs into one Kustomization.
		case grouping == "nodeFlat" && (shape == urlLess || shape == "root bundle and child node, a Source each"):
			return "sourceRef differ"
		// Not this invariant's: with a directory per bundle no node's layout
		// renders one, and a SourceRef with a URL is not taken from a bundle
		// below, so a node's layout Kustomization has no source.
		case perLayout && grouping == "GroupByName":
			return "needs a Kustomization CR but no enclosing bundle"
		}
		return ""
	}
	accepted := map[string]int{}
	for _, placement := range placements {
		for _, grouping := range []string{"nodeOnly", "GroupByName", "nodeFlat"} {
			for _, clusterName := range clusterNames {
				for _, shape := range shapes {
					t.Run(fmt.Sprintf("%s/%s/ClusterName=%q/%s", placement, grouping, clusterName, shape), func(t *testing.T) {
						rules := propertyGroupings[grouping]
						rules.FluxPlacement = placement
						rules.ClusterName = clusterName
						want := wantRefusal(placement, grouping, clusterName, shape)
						ml, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(sourceHostShapes[shape](), rules)
						if want != "" {
							if err == nil || !strings.Contains(err.Error(), want) {
								t.Errorf("got %v, want a refusal that says %q", err, want)
							}
							return
						}
						if err != nil {
							t.Fatalf("refused, want the tree integrated: %v", err)
						}
						accepted[fmt.Sprintf("%s under ClusterName %q", placement, clusterName)]++
						trees := writeAll(t, ml)
						for writer, w := range trees {
							checkSourceHosts(t, writer, w)
						}
						disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
						if len(disk) != len(tar) {
							t.Errorf("WriteToDisk wrote %d files, WriteToTar %d", len(disk), len(tar))
						}
						for p, content := range disk {
							if other, ok := tar[p]; !ok || !bytes.Equal(content, other) {
								t.Errorf("%s differs between WriteToDisk and WriteToTar", p)
							}
						}
					})
				}
			}
		}
	}
	for _, placement := range placements {
		for _, clusterName := range clusterNames {
			if key := fmt.Sprintf("%s under ClusterName %q", placement, clusterName); accepted[key] == 0 {
				t.Errorf("%s: no shape was accepted, so the invariant was checked on no written tree", key)
			}
		}
	}
}

// urlAndPlainTree: platform (bundle core, a SourceRef with a URL) -> web
// (bundle web, a SourceRef without a URL: plain, or testSR when plain is nil).
func urlAndPlainTree(plain *stack.SourceRef) func() *stack.Cluster {
	return func() *stack.Cluster {
		webBundle := srBundle("web", cmApp("web-app"))
		if plain != nil {
			webBundle.SourceRef = plain
		}
		web := &stack.Node{Name: "web", Bundle: webBundle}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: urlBundle("core", "shared"), Children: []*stack.Node{web}}}
	}
}

// TestPerLayout_RootNodeLayoutTakesNoGeneratedSource: below a ClusterName
// directory the root node's layout has a Kustomization of its own under
// FluxIntegratedPerLayout. It applies the directory that hosts every generated
// Source, so it takes none of them: a SourceRef without a URL among the
// bundles below is its source instead, and a tree with none is refused,
// naming the Kustomization, the Source and the directory (go-kure/kure#979).
// Without the ClusterName directory, or under FluxIntegratedPerBundle, the
// root node's layout has no Kustomization and the same tree is integrated.
func TestPerLayout_RootNodeLayoutTakesNoGeneratedSource(t *testing.T) {
	integrator := func() *fluxstack.LayoutIntegrator {
		return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())
	}
	onlyURLs := sourceHostShapes["root bundle and child node, one Source"]
	// The child node's bundle names, without a URL, the Source the root
	// bundle's SourceRef generates: the same Source.
	sameSource := urlAndPlainTree(&stack.SourceRef{Kind: "GitRepository", Name: "shared"})

	for _, tc := range []struct {
		name  string
		build func() *stack.Cluster
	}{
		{"every SourceRef has a URL", onlyURLs},
		{"a SourceRef without a URL names the generated Source", sameSource},
	} {
		for _, grouping := range []string{"nodeOnly", "GroupByName"} {
			for clusterName, dir := range map[string]string{".": "platform", "prod": "prod/platform"} {
				t.Run(fmt.Sprintf("refused/%s/%s/ClusterName=%q", tc.name, grouping, clusterName), func(t *testing.T) {
					rules := propertyGroupings[grouping]
					rules.FluxPlacement = layout.FluxIntegratedPerLayout
					rules.ClusterName = clusterName
					_, err := integrator().CreateLayoutWithResources(tc.build(), rules)
					if err == nil {
						t.Fatal("got no error, want the root node's layout Kustomization refused")
					}
					for _, want := range []string{
						fmt.Sprintf("Flux Kustomization %q (spec.path %q) %s GitRepository %q", strings.ReplaceAll(dir, "/", "-")+"-node", dir, ownSourceRefusal, "shared"),
						fmt.Sprintf("hosts in %q, the root node's layout", dir),
						"give a bundle a SourceRef without a URL",
					} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("refusal %q does not say %q", err, want)
						}
					}
				})
			}
		}
	}

	// A URL on a kind no Source is generated for is the generator's own
	// error: nothing is hosted for it, so it is not passed over and the
	// refusal does not claim a Source the integration never generates.
	t.Run("a kind no Source is generated for", func(t *testing.T) {
		c := onlyURLs()
		c.Node.Bundle.SourceRef.Kind = "Bucket"
		rules := propertyGroupings["nodeOnly"]
		rules.FluxPlacement = layout.FluxIntegratedPerLayout
		rules.ClusterName = "prod"
		_, err := integrator().CreateLayoutWithResources(c, rules)
		if err == nil || !strings.Contains(err.Error(), "Bucket") || strings.Contains(err.Error(), ownSourceRefusal) {
			t.Errorf("got %v, want the generator's refusal of the kind Bucket", err)
		}
		// The kinds the error lists are the caller's copy: writing to them
		// changes neither the next error nor what counts as generated.
		var invalid *kerrors.ValidationError
		if !stderrors.As(err, &invalid) || len(invalid.ValidValues) == 0 {
			t.Fatalf("got %v, want a ValidationError that lists the kinds a Source is generated for", err)
		}
		invalid.ValidValues[0] = "Bucket"
		_, again := integrator().CreateLayoutWithResources(c, rules)
		if again == nil || !strings.Contains(again.Error(), "Bucket") || strings.Contains(again.Error(), ownSourceRefusal) {
			t.Errorf("after writing to the first error's ValidValues: got %v, want the generator's refusal of the kind Bucket", again)
		}
	})

	// One Kustomization applies the root node's directory, however many
	// segments the ClusterName has: the wrapper is one layout.
	for _, clusterName := range []string{"prod", "env/prod"} {
		for _, grouping := range []string{"nodeOnly", "GroupByName"} {
			t.Run(fmt.Sprintf("another source is taken/%s/ClusterName=%q", grouping, clusterName), func(t *testing.T) {
				rules := propertyGroupings[grouping]
				rules.FluxPlacement = layout.FluxIntegratedPerLayout
				rules.ClusterName = clusterName
				rootDir := clusterName + "/platform"
				applying := []string{rootDir}
				ml := integrated(t, urlAndPlainTree(nil)(), rules)
				var got []string
				for _, k := range kustomizations(ml) {
					if p := filepath.Clean(k.Spec.Path); p != rootDir && !strings.HasPrefix(rootDir, p+"/") {
						continue
					}
					got = append(got, filepath.Clean(k.Spec.Path))
					if k.Spec.SourceRef.Name != testSR().Name {
						t.Errorf("Kustomization %q (spec.path %q) takes its source from %q, want %q, the one SourceRef without a URL below it", k.Name, k.Spec.Path, k.Spec.SourceRef.Name, testSR().Name)
					}
				}
				slices.Sort(got)
				if !slices.Equal(got, applying) {
					t.Errorf("Kustomizations that apply the root node's directory have spec.path %v, want %v", got, applying)
				}
				if got, want := sourceCopies(ml, "shared"), map[string]int{rootDir: 1}; !intMapsEqual(got, want) {
					t.Errorf("GitRepository shared hosted %v, want %v", got, want)
				}
				for writer, w := range writeAll(t, ml) {
					checkSourceHosts(t, writer, w)
				}
				checkIdempotent(t, urlAndPlainTree(nil)(), rules)
			})
		}
	}

	for name, rules := range map[string]layout.LayoutRules{
		"no ClusterName directory": {FluxPlacement: layout.FluxIntegratedPerLayout},
		"FluxIntegratedPerBundle":  {FluxPlacement: layout.FluxIntegratedPerBundle, ClusterName: "prod"},
	} {
		t.Run("control/"+name, func(t *testing.T) {
			rules.BundleGrouping, rules.ApplicationGrouping = layout.GroupFlat, layout.GroupFlat
			ml := integrated(t, onlyURLs(), rules)
			for writer, w := range writeAll(t, ml) {
				checkSourceHosts(t, writer, w)
			}
		})
	}
}

// TestKeptKustomization_SourceInsideWhatItApplies: a Kustomization of the
// root node's layout that the integration keeps in place of its own is held to
// the same rule in every form: one that names a Source the integration
// generates is refused, the tree untouched, also when it leaves the
// namespace out. Unchanged, it is kept and the tree writes.
func TestKeptKustomization_SourceInsideWhatItApplies(t *testing.T) {
	const rootDir = "prod/platform"
	build := urlAndPlainTree(nil)
	rules := propertyGroupings["nodeOnly"]
	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	rules.ClusterName = "prod"
	rootCR := rootLayoutCRName(t, build, rules, rootDir)
	for _, form := range keptForms {
		for _, namespace := range []string{"flux-system", ""} {
			t.Run(fmt.Sprintf("%s/sourceRef namespace %q", form, namespace), func(t *testing.T) {
				ml, c := keptIn(t, build, rules, rootCR, form, func(k *kustv1.Kustomization) {
					k.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{Kind: "GitRepository", Name: "shared", Namespace: namespace}
				}, nil)
				before := countResources(ml)
				err := integrate(ml, c, rules)
				if err == nil {
					t.Fatal("got no error, want the kept Kustomization refused")
				}
				for _, want := range []string{
					fmt.Sprintf("Flux Kustomization %q (spec.path %q) %s GitRepository %q", rootCR, rootDir, ownSourceRefusal, "shared"),
					fmt.Sprintf("hosts in %q, the root node's layout", rootDir),
					"name a Source that exists before the tree is applied",
				} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("refusal %q does not say %q", err, want)
					}
				}
				if after := countResources(ml); after != before {
					t.Errorf("refused integration left %d resources, want the %d before it", after, before)
				}
			})
		}
		t.Run(form+"/unchanged", func(t *testing.T) {
			checkKeptTreeWrites(t, build, rules, rootCR, form)
		})
	}
}
