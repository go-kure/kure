package fluxcd_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// generatorOnly is an application config that renders no object and attaches
// one values file and one configMapGenerator to its layout. Its directory has
// no child and no resource file: the one shape FlattenSingleTier collapsed
// into the directory the bundles share before go-kure/kure#979 item 1.
type generatorOnly struct {
	app         string
	annotations map[string]string
}

func (g *generatorOnly) Generate(*stack.Application) ([]*client.Object, error) {
	return nil, nil
}

func (g *generatorOnly) AugmentLayout(ml *layout.ManifestLayout) error {
	ml.ExtraFiles = append(ml.ExtraFiles, layout.ExtraFile{Name: "values.yaml", Content: []byte("k: v\n")})
	ml.ConfigMapGenerators = append(ml.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{
		Name: g.app + "-values", Files: []string{"values.yaml"}, Annotations: g.annotations,
	})
	return nil
}

// generatorOnlyCluster is a root node (bundle b1, no application) with child
// node c2, whose bundle b2 has the generatorOnly application "hook" and no
// other. b1 carries one patch with the given target.
func generatorOnlyCluster(rootName string, target stack.PatchSelector) *stack.Cluster {
	b1 := srBundle("b1")
	b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &target}}
	b2 := srBundle("b2", stack.NewApplication("hook", "default", &generatorOnly{
		app: "hook", annotations: map[string]string{"tier": "backend"},
	}))
	c2 := &stack.Node{Name: "c2", Bundle: b2}
	r := &stack.Node{Name: rootName, Bundle: b1, Children: []*stack.Node{c2}}
	c2.SetParent(r)
	return &stack.Cluster{Name: "demo", Node: r}
}

// generatedConfigMapShapes build a cluster in which bundle b1's one patch has
// the given target and bundle b2 has an augmenter application "hook" whose
// configMapGenerator makes the ConfigMap hook-values, annotated tier=backend.
// Each call builds a fresh cluster: a walk changes the bundles.
var generatedConfigMapShapes = map[string]func(target stack.PatchSelector) *stack.Cluster{
	"beside other applications": func(target stack.PatchSelector) *stack.Cluster {
		return mergedCluster(func(_, b1, b2 *stack.Bundle) {
			b2.Applications = append(b2.Applications, stack.NewApplication("hook", "default", &annotatedGenerator{
				hookAugmenter: hookAugmenter{app: "hook"},
				annotations:   map[string]string{"tier": "backend"},
			}))
			b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &target}}
		})
	},
	"the bundle's only application": func(target stack.PatchSelector) *stack.Cluster {
		return generatorOnlyCluster("r", target)
	},
	"the bundle's only application, unnamed root": func(target stack.PatchSelector) *stack.Cluster {
		return generatorOnlyCluster("", target)
	},
}

// TestPerLayoutPatchScope_GeneratedConfigMapStaysOutOfTheUnitsBuild pins
// go-kure/kure#979 item 5. With every grouping flat, bundle b1's patch targets
// the ConfigMap that a configMapGenerator of bundle b2's augmenter application
// makes, by name and by an annotation selector. Under FluxIntegratedPerLayout
// the patch is accepted, and rightly: the generator is written into the
// application's own directory, which a Kustomization of its own applies
// without patches, so the Kustomization that carries the patch does not build
// that ConfigMap. FlattenSingleTier changes none of it, as it collapses no
// directory that renders a bundle and no directory that has one below it
// (item 1). The same cluster under FluxSeparate is the control: there one
// Kustomization builds the ConfigMap and applies the patch, and the patch is
// refused.
//
// A failure here means a directory that renders bundles builds a generated
// ConfigMap again. checkPatchScope does not count one under
// FluxIntegratedPerLayout (it keeps only the objects held in Resources), so
// it has to learn to before such a tree is accepted.
func TestPerLayoutPatchScope_GeneratedConfigMapStaysOutOfTheUnitsBuild(t *testing.T) {
	targets := map[string]stack.PatchSelector{
		"by name":       {Kind: "ConfigMap", Name: "hook-values"},
		"by annotation": {Kind: "ConfigMap", AnnotationSelector: "tier=backend"},
	}
	for shape, build := range generatedConfigMapShapes {
		for targetName, target := range targets {
			for _, clusterName := range []string{"", "prod"} {
				for _, flatten := range []bool{false, true} {
					name := fmt.Sprintf("%s/%s/ClusterName=%q/FlattenSingleTier=%t", shape, targetName, clusterName, flatten)
					t.Run(name, func(t *testing.T) {
						rules := allFlat
						rules.ClusterName = clusterName
						rules.FlattenSingleTier = flatten
						checkGeneratedConfigMapPatchScope(t, func() *stack.Cluster { return build(target) }, rules)
					})
				}
			}
		}
	}
}

func checkGeneratedConfigMapPatchScope(t *testing.T, build func() *stack.Cluster, rules layout.LayoutRules) {
	t.Helper()
	const item = "go-kure/kure#979 item 5"
	integrator := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator())

	// Control: one Kustomization builds everything, the generated ConfigMap
	// included, so the patch reaches it and is refused.
	rules.FluxPlacement = layout.FluxSeparate
	_, err := integrator.CreateLayoutWithResources(build(), rules)
	if err == nil {
		t.Fatalf("%s, control: FluxSeparate accepted a patch that reaches another bundle's generated ConfigMap", item)
	}
	for _, want := range []string{`"b1"`, `"b2"`, `ConfigMap "hook-values"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%s, control: error %q does not contain %q", item, err, want)
		}
	}

	rules.FluxPlacement = layout.FluxIntegratedPerLayout
	ml, err := integrator.CreateLayoutWithResources(build(), rules)
	if err != nil {
		t.Fatalf("%s: FluxIntegratedPerLayout refused the patch: %v", item, err)
	}

	// The one directory that renders the bundles, and the one layout that
	// carries the generator.
	var units, gens []*layout.ManifestLayout
	var walk func(l *layout.ManifestLayout)
	walk = func(l *layout.ManifestLayout) {
		if len(l.OriginBundles()) > 0 {
			units = append(units, l)
		}
		for _, g := range l.ConfigMapGenerators {
			if g.Name == "hook-values" {
				gens = append(gens, l)
			}
		}
		for _, c := range l.Children {
			walk(c)
		}
	}
	walk(ml)
	if len(units) != 1 || len(gens) != 1 {
		t.Fatalf("%s: %d layouts render bundles and %d carry the generator, want one of each", item, len(units), len(gens))
	}
	unitDir, genDir := units[0].FullRepoPath(), gens[0].FullRepoPath()
	if gens[0] == units[0] || !strings.HasPrefix(genDir, unitDir+"/") {
		t.Fatalf("%s: the generator is in %q, want the application's own directory below the bundles' directory %q", item, genDir, unitDir)
	}

	// The application's directory has a Kustomization of its own, without
	// patches; the patch is on the Kustomization of the bundles' directory.
	var patched, own []*kustv1.Kustomization
	var dirs []string
	for _, k := range kustomizations(ml) {
		dirs = append(dirs, k.Spec.Path)
		if len(k.Spec.Patches) > 0 {
			patched = append(patched, k)
		}
		if k.Spec.Path == genDir {
			own = append(own, k)
		}
	}
	if len(own) != 1 {
		t.Fatalf("%s: %d Kustomizations apply the generator's directory %q, want one", item, len(own), genDir)
	}
	if len(own[0].Spec.Patches) != 0 {
		t.Errorf("%s: Kustomization %s of the generator's directory carries patches %v", item, own[0].Name, own[0].Spec.Patches)
	}
	if len(patched) != 1 || patched[0].Spec.Path != unitDir || len(patched[0].Spec.Patches) != 1 {
		t.Fatalf("%s: Kustomizations with patches = %v, want the one of %q with b1's one patch", item, crNames(asObjects(patched)), unitDir)
	}

	// On the written files: the generator entry is in the application's
	// directory only, the build of the bundles' directory makes no such
	// ConfigMap, and the build of the application's directory makes it with
	// its annotation.
	for writer, w := range writeAll(t, ml) {
		if got := generatorDirs(t, w.root, "hook-values"); !slices.Equal(got, []string{genDir}) {
			t.Errorf("%s, %s: configMapGenerator hook-values is written in %v, want %q only", item, writer, got, genDir)
		}
		for name := range builtConfigMaps(t, filepath.Join(w.root, unitDir)) {
			if strings.HasPrefix(name, "hook-values") {
				t.Errorf("%s, %s: the build of %q, which the patch is applied to, makes ConfigMap %s", item, writer, unitDir, name)
			}
		}
		var generated []string
		for name, annotations := range builtConfigMaps(t, filepath.Join(w.root, genDir)) {
			if !strings.HasPrefix(name, "hook-values-") {
				continue
			}
			generated = append(generated, name)
			if annotations["tier"] != "backend" {
				t.Errorf("%s, %s: ConfigMap %s has annotations %v, want tier=backend", item, writer, name, annotations)
			}
		}
		if len(generated) != 1 {
			t.Errorf("%s, %s: the build of %q makes ConfigMaps %v from the generator, want one", item, writer, genDir, generated)
		}
		checkWrittenTree(t, writer, w, dirs)
	}
}

func asObjects(kusts []*kustv1.Kustomization) []client.Object {
	out := make([]client.Object, 0, len(kusts))
	for _, k := range kusts {
		out = append(out, k)
	}
	return out
}

// generatorDirs returns the directories below root, relative to it and sorted,
// whose kustomization.yaml has a configMapGenerator entry with the given name.
func generatorDirs(t *testing.T, root, name string) []string {
	t.Helper()
	var dirs []string
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "kustomization.yaml" {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		var kust struct {
			ConfigMapGenerator []struct {
				Name string `json:"name"`
			} `json:"configMapGenerator"`
		}
		if err := yaml.Unmarshal(data, &kust); err != nil {
			return fmt.Errorf("parse %s: %w", p, err)
		}
		for _, g := range kust.ConfigMapGenerator {
			if g.Name == name {
				rel, err := filepath.Rel(root, filepath.Dir(p))
				if err != nil {
					return err
				}
				dirs = append(dirs, filepath.ToSlash(rel))
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(dirs)
	return dirs
}

// builtConfigMaps runs a kustomize build of dir, as kustomize-controller does
// for a Kustomization's spec.path, and returns the annotations of every
// ConfigMap it makes, by name.
func builtConfigMaps(t *testing.T, dir string) map[string]map[string]string {
	t.Helper()
	built, err := krusty.MakeKustomizer(krusty.MakeDefaultOptions()).Run(filesys.MakeFsOnDisk(), dir)
	if err != nil {
		t.Fatalf("kustomize build of %s: %v", dir, err)
	}
	out := map[string]map[string]string{}
	for _, r := range built.Resources() {
		if r.GetKind() == "ConfigMap" {
			out[r.GetName()] = r.GetAnnotations()
		}
	}
	return out
}
