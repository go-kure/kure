package fluxcd_test

import (
	"bytes"
	"maps"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for where a bundle's patches go under per-layout placement
// (go-kure/kure#1021): a patch with a target on the bundle's own
// Kustomization and on that of every layout of its applications, a patch
// without one on the Kustomizations whose build holds the object it names.

// patchShop is the bundle "shop" with patches and two applications: "web",
// whose ConfigMap web-cm is written into the bundle's directory prod/shop,
// which the bundle's own Kustomization builds, and "db" with the layouts of
// settingsAugmenter, each built by a Kustomization of its own: ConfigMap db in
// prod/shop/db, pre in 00-pre, main in 01-main and deep in 01-main/02-deep.
func patchShop(set func(app, pre, main, deep *layout.ManifestLayout), patches ...stack.Patch) *stack.Bundle {
	b := srBundle("shop", cmApp("web"), stack.NewApplication("db", "default", settingsAugmenter{set: set}))
	b.Patches = patches
	return b
}

// cmDoc is a strategic-merge patch document for the ConfigMap name in the
// namespace the test ConfigMaps are in.
func cmDoc(name string) string {
	return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n  namespace: default\ndata:\n  patched: \"yes\"\n"
}

// kustDoc is a strategic-merge patch document for the Flux Kustomization name.
func kustDoc(name string) string {
	return "apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: " + name + "\n  namespace: flux-system\nspec:\n  timeout: 1m\n"
}

// untargeted is a patch without a target made of docs.
func untargeted(docs ...string) stack.Patch {
	return stack.Patch{Patch: strings.Join(docs, "---\n")}
}

// everyConfigMap is a JSON6902 patch with a target, which selects every
// ConfigMap; note tells two of them apart.
func everyConfigMap(note string) stack.Patch {
	return stack.Patch{
		Patch:  "- op: add\n  path: /metadata/labels/" + note + "\n  value: \"yes\"\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap"},
	}
}

// everyConfigMapMerged is everyConfigMap as a strategic-merge patch: with a
// target kustomize merges the document into what the target selects, whatever
// name the document carries.
func everyConfigMapMerged(note string) stack.Patch {
	return stack.Patch{
		Patch:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: any\n  labels:\n    " + note + ": \"yes\"\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap"},
	}
}

// renameConfigMap is a JSON6902 patch with a target that gives the ConfigMap
// from the name to.
func renameConfigMap(from, to string) stack.Patch {
	return stack.Patch{
		Patch:  "- op: replace\n  path: /metadata/name\n  value: " + to + "\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap", Name: from},
	}
}

// json6902 is the text of a JSON6902 patch, which kustomize takes with a
// target only.
const json6902 = "- op: add\n  path: /metadata/labels/x\n  value: y\n"

// patchPlacement returns, for every Kustomization in ml that holds a patch,
// the indexes in patches of the ones it holds, in the order it holds them. A
// patch it holds that is none of patches fails the test.
func patchPlacement(t *testing.T, ml *layout.ManifestLayout, patches []stack.Patch) map[string][]int {
	t.Helper()
	out := map[string][]int{}
	for name, k := range kustomizationsByName(ml) {
		for _, p := range k.Spec.Patches {
			i := slices.IndexFunc(patches, func(want stack.Patch) bool {
				return want.Patch == p.Patch && (want.Target == nil) == (p.Target == nil)
			})
			if i < 0 {
				t.Fatalf("%s holds a patch that is none of the bundle's:\n%s", name, p.Patch)
			}
			out[name] = append(out[name], i)
		}
	}
	return out
}

// TestLayoutPatches_PlacedByObject: a patch with a target is written on the
// bundle's own Kustomization and on every per-layout Kustomization of its
// applications. A patch without one is written on the Kustomizations whose
// build holds every object it names, and on no other: the bundle's own for an
// object written into the bundle's directory, a layout's for an object of that
// layout. A build holds the Kustomizations hosted in it and the ConfigMap a
// configMapGenerator entry generates. Each Kustomization holds its patches in
// the bundle's order.
func TestLayoutPatches_PlacedByObject(t *testing.T) {
	all := func(i ...int) map[string][]int {
		out := map[string][]int{"shop": i}
		for _, name := range shopLayoutNames {
			out[name] = i
		}
		return out
	}
	for name, tc := range map[string]struct {
		set     func(app, pre, main, deep *layout.ManifestLayout)
		patches []stack.Patch
		want    map[string][]int
	}{
		"a target: the bundle's own and every layout's": {
			patches: []stack.Patch{everyConfigMap("a")},
			want:    all(0),
		},
		"a target on a strategic-merge patch: the same": {
			patches: []stack.Patch{everyConfigMapMerged("a")},
			want:    all(0),
		},
		"an object in the bundle's directory: the bundle's own and nowhere else": {
			patches: []stack.Patch{untargeted(cmDoc("web-cm"))},
			want:    map[string][]int{"shop": {0}},
		},
		"an object of the application's layout": {
			patches: []stack.Patch{untargeted(cmDoc("db"))},
			want:    map[string][]int{"shop-db": {0}},
		},
		"an object of a layout below the application's": {
			patches: []stack.Patch{untargeted(cmDoc("pre"))},
			want:    map[string][]int{"shop-00-pre": {0}},
		},
		"an object two layouts below the application's": {
			patches: []stack.Patch{untargeted(cmDoc("deep"))},
			want:    map[string][]int{"shop-02-deep": {0}},
		},
		"a document without a namespace names the object in the default one": {
			patches: []stack.Patch{untargeted("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: main\ndata:\n  patched: \"yes\"\n")},
			want:    map[string][]int{"shop-01-main": {0}},
		},
		"the application layout's Kustomization, hosted in the bundle's directory": {
			patches: []stack.Patch{untargeted(kustDoc("shop-db"))},
			want:    map[string][]int{"shop": {0}},
		},
		"a layout's Kustomization, hosted in the layout above it": {
			patches: []stack.Patch{untargeted(kustDoc("shop-02-deep"))},
			want:    map[string][]int{"shop-01-main": {0}},
		},
		"two documents whose objects one build holds": {
			patches: []stack.Patch{untargeted(cmDoc("db"), kustDoc("shop-00-pre"))},
			want:    map[string][]int{"shop-db": {0}},
		},
		"a ConfigMap a configMapGenerator entry generates": {
			set: func(_, pre, _, _ *layout.ManifestLayout) {
				pre.ExtraFiles = []layout.ExtraFile{{Name: "values.yaml", Content: []byte("a: b\n")}}
				pre.ConfigMapGenerators = []layout.ConfigMapGeneratorSpec{{Name: "values", Files: []string{"values.yaml"}}}
			},
			patches: []stack.Patch{untargeted("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: values\ndata:\n  patched: \"yes\"\n")},
			want:    map[string][]int{"shop-00-pre": {0}},
		},
		"a JSON6902 patch without a target stays on the bundle's own": {
			patches: []stack.Patch{{Patch: json6902}},
			want:    map[string][]int{"shop": {0}},
		},
		"each Kustomization holds its patches in the bundle's order": {
			patches: []stack.Patch{everyConfigMapMerged("a"), untargeted(cmDoc("pre")), untargeted(cmDoc("web-cm")), everyConfigMap("b")},
			want: map[string][]int{
				"shop":         {0, 2, 3},
				"shop-db":      {0, 3},
				"shop-00-pre":  {0, 1, 3},
				"shop-01-main": {0, 3},
				"shop-02-deep": {0, 3},
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ml := integrated(t, oneBundleCluster(patchShop(tc.set, tc.patches...)), perLayoutRules())
			if got := patchPlacement(t, ml, tc.patches); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("patches are held by %v, want %v", got, tc.want)
			}
			// The tree is one the writers take.
			writeAll(t, ml)
		})
	}
}

// TestLayoutPatches_Refusals: a patch without a target is refused when no
// Kustomization of the bundle builds an object it names, and when its
// documents name objects no one Kustomization builds together. The refusal
// names the bundle, the patch by its index, the object and the remedy, and
// comes before anything is written.
func TestLayoutPatches_Refusals(t *testing.T) {
	for name, tc := range map[string]struct {
		patches []stack.Patch
		wants   []string
	}{
		"an object no build holds": {
			patches: []stack.Patch{untargeted(cmDoc("absent"))},
			wants: []string{
				"Bundle", "'shop'",
				`patch 0 has no target and names ConfigMap "absent" (v1, namespace "default"), which no Kustomization of the bundle builds`,
				`"shop", spec.path "prod/shop"`,
				`"shop-db", spec.path "prod/shop/db"`,
				`"shop-00-pre", spec.path "prod/shop/db/00-pre"`,
				`"shop-01-main", spec.path "prod/shop/db/01-main"`,
				`"shop-02-deep", spec.path "prod/shop/db/01-main/02-deep"`,
				"kustomize fails the build of a Kustomization that holds a strategic-merge patch for an object it does not build",
				"give the patch a target (for a patch of several documents: one patch per document first, each with its target), or place the object in a build of the bundle",
			},
		},
		"the patch is named by its index among the bundle's": {
			patches: []stack.Patch{everyConfigMapMerged("a"), untargeted(cmDoc("pre")), untargeted(cmDoc("absent"))},
			wants:   []string{`patch 2 has no target and names ConfigMap "absent"`},
		},
		"an object of another apiVersion than the one built": {
			patches: []stack.Patch{untargeted("apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: web-cm\n  namespace: default\nspec:\n  replicas: 2\n")},
			wants:   []string{`patch 0 has no target and names Deployment "web-cm" (apps/v1, namespace "default"), which no Kustomization of the bundle builds`},
		},
		"the bundle's own Kustomization, which no build of the bundle holds": {
			patches: []stack.Patch{untargeted(kustDoc("shop"))},
			wants:   []string{`patch 0 has no target and names Kustomization "shop" (kustomize.toolkit.fluxcd.io/v1, namespace "flux-system"), which no Kustomization of the bundle builds`},
		},
		"one of two documents names an object no build holds": {
			patches: []stack.Patch{untargeted(cmDoc("pre"), cmDoc("absent"))},
			wants:   []string{`patch 0 has no target and names ConfigMap "absent" (v1, namespace "default"), which no Kustomization of the bundle builds`},
		},
		"two documents whose objects two builds hold": {
			patches: []stack.Patch{untargeted(cmDoc("web-cm"), cmDoc("pre"))},
			wants: []string{
				"Bundle", "'shop'",
				"patch 0 has no target and names objects that no one Kustomization of the bundle builds together",
				`ConfigMap "web-cm" (v1, namespace "default") is built by "shop", spec.path "prod/shop"`,
				`ConfigMap "pre" (v1, namespace "default") is built by "shop-00-pre", spec.path "prod/shop/db/00-pre"`,
				"the documents of one patch are applied in one build; write one patch per object",
			},
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
				CreateLayoutWithResources(oneBundleCluster(patchShop(nil, tc.patches...)), perLayoutRules())
			mustContainAll(t, err, tc.wants...)
		})
	}

	// One patch per object is the remedy of the second refusal.
	patches := []stack.Patch{untargeted(cmDoc("web-cm")), untargeted(cmDoc("pre"))}
	ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
	if got, want := patchPlacement(t, ml, patches), (map[string][]int{"shop": {0}, "shop-00-pre": {1}}); !reflect.DeepEqual(got, want) {
		t.Errorf("one patch per object: held by %v, want %v", got, want)
	}
}

// shopKustomizationFile is the file of the bundle's own Kustomization in the
// trees of these tests.
var shopKustomizationFile = filepath.Join("prod", "flux-system-kustomization-shop.yaml")

// shopOwnWithPatch is the file of the bundle's own Kustomization of patchShop
// with one patch, an untargeted one for the ConfigMap name, as the integration
// wrote it before the per-layout Kustomizations took patches: every patch of
// the bundle on the bundle's own Kustomization, whatever it builds.
func shopOwnWithPatch(name string) string {
	return `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: shop
  namespace: flux-system
spec:
  interval: 1h0m0s
  patches:
  - patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: ` + name + `
        namespace: default
      data:
        patched: "yes"
  path: prod/shop
  prune: false
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
`
}

// TestLayoutPatches_AnObjectTheBundlesOwnBuildsStaysThere: an untargeted patch
// for an object the bundle's own Kustomization builds is written on that
// Kustomization and on no other, and its file is the one written before the
// per-layout Kustomizations took patches. A tree that built on the cluster
// then had every such object in that build, so it is not refused and renders
// as it did.
func TestLayoutPatches_AnObjectTheBundlesOwnBuildsStaysThere(t *testing.T) {
	patches := []stack.Patch{untargeted(cmDoc("web-cm"))}
	ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
	if got, want := patchPlacement(t, ml, patches), (map[string][]int{"shop": {0}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("the patch is held by %v, want %v", got, want)
	}
	trees := writeAll(t, ml)
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if got, want := string(disk[shopKustomizationFile]), shopOwnWithPatch("web-cm"); got != want {
		t.Errorf("%s:\n%s\nwant:\n%s", shopKustomizationFile, got, want)
	}
	if !bytes.Equal(disk[shopKustomizationFile], tar[shopKustomizationFile]) {
		t.Errorf("%s differs between WriteToDisk and WriteToTar", shopKustomizationFile)
	}
}

// TestLayoutPatches_BundleWithoutLayoutKustomizations: a bundle none of whose
// applications got a Kustomization of its own keeps every patch on its own
// Kustomization under per-layout placement as well, the untargeted ones
// included, and none is refused, whatever it names: its file is the one
// written before the per-layout Kustomizations took patches.
func TestLayoutPatches_BundleWithoutLayoutKustomizations(t *testing.T) {
	patches := []stack.Patch{untargeted(cmDoc("web-cm")), everyConfigMap("a"), untargeted(cmDoc("absent")), {Patch: json6902}}
	b := srBundle("shop", cmApp("web"))
	b.Patches = patches
	ml := integrated(t, oneBundleCluster(b), perLayoutRules())
	if got, want := patchPlacement(t, ml, patches), (map[string][]int{"shop": {0, 1, 2, 3}}); !reflect.DeepEqual(got, want) {
		t.Fatalf("the patches are held by %v, want %v", got, want)
	}
	const want = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: shop
  namespace: flux-system
spec:
  interval: 1h0m0s
  patches:
  - patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: web-cm
        namespace: default
      data:
        patched: "yes"
  - patch: |
      - op: add
        path: /metadata/labels/a
        value: "yes"
    target:
      kind: ConfigMap
  - patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: absent
        namespace: default
      data:
        patched: "yes"
  - patch: |
      - op: add
        path: /metadata/labels/x
        value: y
  path: prod/shop
  prune: false
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
`
	trees := writeAll(t, ml)
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if got := string(disk[shopKustomizationFile]); got != want {
		t.Errorf("%s:\n%s\nwant:\n%s", shopKustomizationFile, got, want)
	}
	if !bytes.Equal(disk[shopKustomizationFile], tar[shopKustomizationFile]) {
		t.Errorf("%s differs between WriteToDisk and WriteToTar", shopKustomizationFile)
	}
}

// TestLayoutPatches_TheBundlesOwnLosesAPatchItDoesNotBuild: an untargeted
// patch for an object of a layout was written on the bundle's own
// Kustomization, whose build does not hold that object, so that kustomize
// failed that build on the cluster. It is now written on the layout's
// Kustomization and no longer on the bundle's own. before is the file of the
// bundle's own Kustomization as it was written then, after the one written
// now; the layout's Kustomization gained the patch.
func TestLayoutPatches_TheBundlesOwnLosesAPatchItDoesNotBuild(t *testing.T) {
	before := shopOwnWithPatch("pre")
	const after = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: shop
  namespace: flux-system
spec:
  interval: 1h0m0s
  path: prod/shop
  prune: false
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
`
	const layoutAfter = `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: shop-00-pre
  namespace: flux-system
spec:
  interval: 1h0m0s
  patches:
  - patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: pre
        namespace: default
      data:
        patched: "yes"
  path: prod/shop/db/00-pre
  prune: false
  sourceRef:
    kind: GitRepository
    name: flux-system
    namespace: flux-system
`
	ml := integrated(t, oneBundleCluster(patchShop(nil, untargeted(cmDoc("pre")))), perLayoutRules())
	trees := writeAll(t, ml)
	disk, tar := treeFiles(t, trees["WriteToDisk"].root), treeFiles(t, trees["WriteToTar"].root)
	if got := string(disk[shopKustomizationFile]); got != after {
		t.Errorf("%s:\n%s\nwant:\n%s", shopKustomizationFile, got, after)
	}
	// What the file lost is the patch, and nothing else.
	start, end := strings.Index(before, "  patches:\n"), strings.Index(before, "  path: ")
	if start < 0 || end < start || before[:start]+before[end:] != after {
		t.Errorf("the file written before differs from the one written now in more than the patch:\n%s\nnow:\n%s", before, after)
	}
	pre := filepath.Join("prod", "shop", "db", "flux-system-kustomization-shop-00-pre.yaml")
	if got := string(disk[pre]); got != layoutAfter {
		t.Errorf("%s:\n%s\nwant:\n%s", pre, got, layoutAfter)
	}
	for _, file := range []string{shopKustomizationFile, pre} {
		if !bytes.Equal(disk[file], tar[file]) {
			t.Errorf("%s differs between WriteToDisk and WriteToTar", file)
		}
	}
}

// TestLayoutPatches_OtherPlacementsUnchanged: under FluxIntegratedPerBundle
// and FluxSeparate no layout gets a Kustomization of its own, so every patch
// of the bundle stays on the bundle's own Kustomization and none is refused,
// an untargeted one for an object nothing builds included.
func TestLayoutPatches_OtherPlacementsUnchanged(t *testing.T) {
	patches := []stack.Patch{untargeted(cmDoc("absent")), untargeted(cmDoc("pre")), everyConfigMap("a"), untargeted(cmDoc("web-cm"), cmDoc("pre"))}
	for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerBundle, layout.FluxSeparate} {
		t.Run(string(placement), func(t *testing.T) {
			rules := layout.DefaultLayoutRules()
			rules.FluxPlacement = placement
			ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), rules)
			if names := slices.Sorted(maps.Keys(kustomizationsByName(ml))); !slices.Equal(names, []string{"shop"}) {
				t.Fatalf("Kustomizations = %q, want the bundle's alone", names)
			}
			if got, want := patchPlacement(t, ml, patches), (map[string][]int{"shop": {0, 1, 2, 3}}); !reflect.DeepEqual(got, want) {
				t.Errorf("the patches are held by %v, want %v", got, want)
			}
		})
	}
}

// TestLayoutPatches_RepeatedIntegration: a second integration of a tree keeps
// the Kustomizations of the first as they are. It adds no patch to them, and
// refuses nothing the first took: a kept Kustomization still counts as a
// build that holds a patch's object.
func TestLayoutPatches_RepeatedIntegration(t *testing.T) {
	patches := []stack.Patch{everyConfigMapMerged("a"), untargeted(cmDoc("pre")), untargeted(cmDoc("web-cm"))}
	c := oneBundleCluster(patchShop(nil, patches...))
	rules := perLayoutRules()
	ml := mustWalk(t, c, rules)
	if err := integrate(ml, c, rules); err != nil {
		t.Fatalf("IntegrateWithLayout: %v", err)
	}
	first := patchPlacement(t, ml, patches)
	if want := (map[string][]int{
		"shop": {0, 2}, "shop-db": {0}, "shop-00-pre": {0, 1}, "shop-01-main": {0}, "shop-02-deep": {0},
	}); !reflect.DeepEqual(first, want) {
		t.Fatalf("after the first integration the patches are held by %v, want %v", first, want)
	}
	before := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
	if err := integrate(ml, c, rules); err != nil {
		t.Fatalf("the second IntegrateWithLayout: %v", err)
	}
	if got := patchPlacement(t, ml, patches); !reflect.DeepEqual(got, first) {
		t.Errorf("after the second integration the patches are held by %v, after the first by %v", got, first)
	}
	// No byte of the tree changes: the patch for an object of a layout is on
	// that layout's kept Kustomization from the first integration, and the
	// second takes it off no Kustomization and adds it to none.
	after := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
	if len(after) != len(before) {
		t.Errorf("the second integration wrote %d files, the first %d", len(after), len(before))
	}
	for name, content := range before {
		if !bytes.Equal(after[name], content) {
			t.Errorf("%s changed in the second integration:\n%s\nwas:\n%s", name, after[name], content)
		}
	}

	// A patch the bundle gets between two integrations of one tree is not
	// written on a kept Kustomization: every Kustomization of the tree is
	// kept, so the patches on them are those of the first integration.
	added := append(slices.Clone(patches), untargeted(cmDoc("deep")))
	c.Node.Bundle.Patches = added
	if err := integrate(ml, c, rules); err != nil {
		t.Fatalf("the third IntegrateWithLayout: %v", err)
	}
	if got := patchPlacement(t, ml, added); !reflect.DeepEqual(got, first) {
		t.Errorf("with a patch added to the bundle the patches are held by %v, want the kept %v", got, first)
	}
}

// TestLayoutPatches_AGeneratedConfigMapHasNoNamespace: a configMapGenerator
// entry has no namespace field and the kustomization.yaml kure writes sets
// none, so the ConfigMap it generates is compared without one, which kustomize
// reads as "default". A patch document names it without a namespace or in
// "default"; one that names it in another namespace names an object no build
// holds.
func TestLayoutPatches_AGeneratedConfigMapHasNoNamespace(t *testing.T) {
	generated := func(_, pre, _, _ *layout.ManifestLayout) {
		pre.ExtraFiles = []layout.ExtraFile{{Name: "values.yaml", Content: []byte("a: b\n")}}
		pre.ConfigMapGenerators = []layout.ConfigMapGeneratorSpec{{Name: "values", Files: []string{"values.yaml"}}}
	}
	inDefault := []stack.Patch{untargeted(cmDoc("values"))}
	ml := integrated(t, oneBundleCluster(patchShop(generated, inDefault...)), perLayoutRules())
	if got, want := patchPlacement(t, ml, inDefault), (map[string][]int{"shop-00-pre": {0}}); !reflect.DeepEqual(got, want) {
		t.Errorf("a document in the default namespace: held by %v, want %v", got, want)
	}

	elsewhere := untargeted("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: values\n  namespace: other\ndata:\n  patched: \"yes\"\n")
	_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
		CreateLayoutWithResources(oneBundleCluster(patchShop(generated, elsewhere)), perLayoutRules())
	mustContainAll(t, err, `patch 0 has no target and names ConfigMap "values" (v1, namespace "other"), which no Kustomization of the bundle builds`)
}

// TestLayoutPatches_MergedBundles: where a grouping axis merges two bundles
// into one directory, their one Kustomization holds the patches of both, and
// the Kustomization of an application's layout those of the bundle that lists
// the application.
func TestLayoutPatches_MergedBundles(t *testing.T) {
	patch := func(app string) stack.Patch {
		return stack.Patch{
			Patch:  "- op: add\n  path: /metadata/labels/" + app + "\n  value: \"yes\"\n",
			Target: &stack.PatchSelector{Kind: "ConfigMap", Name: app + "-cm"},
		}
	}
	patches := []stack.Patch{patch("platform-app"), patch("apps-app")}
	platform, apps := srBundle("platform", cmApp("platform-app")), srBundle("apps", cmApp("apps-app"))
	platform.Patches, apps.Patches = patches[:1], patches[1:]
	child := &stack.Node{Name: "apps", Bundle: apps}
	root := &stack.Node{Name: "platform", Bundle: platform, Children: []*stack.Node{child}}
	child.SetParent(root)
	rules := layout.LayoutRules{
		NodeGrouping:        layout.GroupFlat,
		BundleGrouping:      layout.GroupFlat,
		ApplicationGrouping: layout.GroupByName,
		FluxPlacement:       layout.FluxIntegratedPerLayout,
	}
	ml := integrated(t, &stack.Cluster{Name: "demo", Node: root}, rules)
	want := map[string][]int{"platform": {0, 1}, "platform-platform-app": {0}, "platform-apps-app": {1}}
	if got := patchPlacement(t, ml, patches); !reflect.DeepEqual(got, want) {
		t.Errorf("the patches are held by %v, want %v", got, want)
	}
}

// TestLayoutPatches_UmbrellaChild: the Kustomization of an umbrella child is
// hosted in the umbrella's directory, so an untargeted patch of the umbrella
// for it stays on the umbrella's own Kustomization. An application of the
// child inherits the child's patches, not the umbrella's.
func TestLayoutPatches_UmbrellaChild(t *testing.T) {
	patches := []stack.Patch{untargeted(kustDoc("infra")), everyConfigMap("umbrella"), everyConfigMap("child")}
	infra := srBundle("infra", cmApp("infra-app"))
	infra.Patches = patches[2:]
	shop := srBundle("shop", cmApp("shop-app"))
	shop.Patches = patches[:2]
	shop.Children = []*stack.Bundle{infra}
	rules := perLayoutRules()
	rules.ApplicationGrouping = layout.GroupByName
	ml := integrated(t, oneBundleCluster(shop), rules)
	want := map[string][]int{"shop": {0, 1}, "shop-shop-app": {1}, "infra": {2}, "infra-infra-app": {2}}
	if got := patchPlacement(t, ml, patches); !reflect.DeepEqual(got, want) {
		t.Errorf("the patches are held by %v, want %v", got, want)
	}
}
