package fluxcd_test

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for a bundle's patch list in which an entry is not a plain
// strategic-merge patch (go-kure/kure#1021): a JSON6902 patch, or a patch
// whose text carries an annotation of kustomize's own build state. Such an
// entry can change an object's identity, so it is written where it was
// before, and an untargeted strategic-merge patch after it is not placed by
// object and never refused, since the identities as generated no longer say
// what it names. These tests run the build of kustomize-controller on the
// tree the writers wrote.

// patchedBuild builds the directory k's spec.path names in a copy of root as
// kustomize-controller does, k's patches added to the directory's
// kustomization.yaml, and returns the data of every v1 ConfigMap in the
// namespace default the build makes, by name, or the error of the build.
func patchedBuild(t *testing.T, root string, k *kustv1.Kustomization) (map[string]map[string]string, error) {
	t.Helper()
	obj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		t.Fatal(err)
	}
	tree := filepath.Join(t.TempDir(), "repo")
	if err := os.CopyFS(tree, os.DirFS(root)); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(tree, filepath.FromSlash(k.Spec.Path))
	if _, err := fluxkustomize.NewGenerator(tree, unstructured.Unstructured{Object: obj}).WriteFile(path); err != nil {
		t.Fatalf("Flux generator on %q: %v", k.Spec.Path, err)
	}
	built, err := fluxkustomize.SecureBuild(tree, path, false)
	if err != nil {
		return nil, err
	}
	cms := map[string]map[string]string{}
	for _, r := range built.Resources() {
		if r.GetApiVersion() == "v1" && r.GetKind() == "ConfigMap" && r.GetNamespace() == "default" {
			cms[r.GetName()] = r.GetDataMap()
		}
	}
	return cms, nil
}

// patchedBuilds runs patchedBuild for every Kustomization of ml on the tree
// WriteToDisk writes, and returns what each made, or the error of its build,
// by the Kustomization's name.
func patchedBuilds(t *testing.T, ml *layout.ManifestLayout) (map[string]map[string]map[string]string, map[string]error) {
	t.Helper()
	root := writeAll(t, ml)["WriteToDisk"].root
	built, failed := map[string]map[string]map[string]string{}, map[string]error{}
	for name, k := range kustomizationsByName(ml) {
		cms, err := patchedBuild(t, root, k)
		if err != nil {
			failed[name] = err
			continue
		}
		built[name] = cms
	}
	return built, failed
}

// shopOwnHolding is the file of the bundle's own Kustomization of patchShop
// holding every one of patches, in their order: the file the integration
// wrote for that bundle before the per-layout Kustomizations took patches.
// It is made here from the patches and not by the integration; for the patch
// lists of these tests it was compared with what the code before
// go-kure/kure#1021 writes.
func shopOwnHolding(patches ...stack.Patch) string {
	var b strings.Builder
	b.WriteString("apiVersion: kustomize.toolkit.fluxcd.io/v1\nkind: Kustomization\nmetadata:\n  name: shop\n  namespace: flux-system\nspec:\n  interval: 1h0m0s\n  patches:\n")
	for _, p := range patches {
		b.WriteString("  - patch: |\n")
		for line := range strings.SplitSeq(strings.TrimSuffix(p.Patch, "\n"), "\n") {
			b.WriteString("      " + line + "\n")
		}
		if p.Target != nil {
			b.WriteString("    target:\n      kind: " + p.Target.Kind + "\n      name: " + p.Target.Name + "\n")
		}
	}
	b.WriteString("  path: prod/shop\n  prune: false\n  sourceRef:\n    kind: GitRepository\n    name: flux-system\n    namespace: flux-system\n")
	return b.String()
}

var patchedData = map[string]string{"patched": "yes"}

// annotateEveryConfigMap is a JSON6902 patch with a target, which selects
// every ConfigMap and which a build applies to each: none of the ConfigMaps of
// these trees has annotations.
func annotateEveryConfigMap() stack.Patch {
	return stack.Patch{
		Patch:  "- op: add\n  path: /metadata/annotations\n  value:\n    note: a\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap"},
	}
}

// renameAllowed is a strategic-merge patch with a target that gives the
// ConfigMap from the name to: its text carries the annotation by which
// kustomize allows a patch to change the name of the object it is merged
// into.
func renameAllowed(from, to string) stack.Patch {
	return stack.Patch{
		Patch:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + to + "\n  namespace: default\n  annotations:\n    internal.config.kubernetes.io/allowNameChange: enabled\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap", Name: from},
	}
}

// renamePrevious is a strategic-merge patch without a target that gives the
// ConfigMap from the name to: its text carries the annotations in which
// kustomize keeps the identities an object had, by which it finds the object,
// and the one that allows the change of name.
func renamePrevious(from, to string) stack.Patch {
	return stack.Patch{
		Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + to + "\n  namespace: default\n  annotations:\n" +
			"    internal.config.kubernetes.io/allowNameChange: enabled\n" +
			"    internal.config.kubernetes.io/previousKinds: ConfigMap\n" +
			"    internal.config.kubernetes.io/previousNames: " + from + "\n" +
			"    internal.config.kubernetes.io/previousNamespaces: default\n",
	}
}

// TestLayoutPatches_AfterAnEntryThatIsNotPlain: placement by object holds
// while every entry of the list is a plain strategic-merge patch. An entry
// that is not is written where it was before, with a target on the bundle's
// own Kustomization and on every per-layout one, without one on the bundle's
// own only. An untargeted strategic-merge patch that comes after it stays on
// the bundle's own Kustomization, is written besides on each per-layout
// Kustomization whose build holds every object it names as generated, and is
// not refused, whatever it names. One that comes before it is placed by
// object.
func TestLayoutPatches_AfterAnEntryThatIsNotPlain(t *testing.T) {
	layouts := func(i ...int) map[string][]int {
		out := map[string][]int{}
		for _, name := range shopLayoutNames {
			out[name] = i
		}
		return out
	}
	with := func(base map[string][]int, name string, i ...int) map[string][]int {
		base[name] = i
		return base
	}
	for name, tc := range map[string]struct {
		patches []stack.Patch
		want    map[string][]int
	}{
		"a plain list is placed by object": {
			patches: []stack.Patch{everyConfigMapMerged("a"), untargeted(cmDoc("pre")), untargeted(cmDoc("web-cm"))},
			want:    with(with(layouts(0), "shop", 0, 2), "shop-00-pre", 0, 1),
		},
		"an object the bundle's own builds": {
			patches: []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("web-cm"))},
			want:    with(layouts(0), "shop", 0, 1),
		},
		"an object of a layout: the bundle's own and the layout's": {
			patches: []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("pre"))},
			want:    with(with(layouts(0), "shop", 0, 1), "shop-00-pre", 0, 1),
		},
		"an object no build holds as generated: the bundle's own, not refused": {
			patches: []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("absent"))},
			want:    with(layouts(0), "shop", 0, 1),
		},
		"two documents whose objects two builds hold: the bundle's own, not refused": {
			patches: []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("web-cm"), cmDoc("pre"))},
			want:    with(layouts(0), "shop", 0, 1),
		},
		"the patch before the entry is placed by object": {
			patches: []stack.Patch{untargeted(cmDoc("pre")), annotateEveryConfigMap(), untargeted(cmDoc("main"))},
			want:    with(with(with(layouts(1), "shop", 1, 2), "shop-00-pre", 0, 1), "shop-01-main", 1, 2),
		},
		"a JSON6902 entry without a target: the bundle's own, and what follows is not placed by object": {
			patches: []stack.Patch{{Patch: json6902}, untargeted(cmDoc("pre")), untargeted(cmDoc("absent"))},
			want:    map[string][]int{"shop": {0, 1, 2}, "shop-00-pre": {1}},
		},
		"a build annotation in a patch with a target: as a JSON6902 one": {
			patches: []stack.Patch{renameAllowed("web-cm", "renamed"), untargeted(cmDoc("absent")), untargeted(cmDoc("pre"))},
			want:    with(with(layouts(0), "shop", 0, 1, 2), "shop-00-pre", 0, 2),
		},
		"a build annotation in a patch without a target: the bundle's own, whatever it names": {
			patches: []stack.Patch{renamePrevious("pre", "renamed"), untargeted(cmDoc("pre")), untargeted(cmDoc("absent"))},
			want:    map[string][]int{"shop": {0, 1, 2}, "shop-00-pre": {1}},
		},
		"a build annotation in one document of several": {
			patches: []stack.Patch{untargeted(cmDoc("main"), renamePrevious("pre", "renamed").Patch), untargeted(cmDoc("absent"))},
			want:    map[string][]int{"shop": {0, 1}},
		},
		"the patch before a build annotation is placed by object": {
			patches: []stack.Patch{untargeted(cmDoc("pre")), renamePrevious("web-cm", "renamed")},
			want:    map[string][]int{"shop": {1}, "shop-00-pre": {0}},
		},
	} {
		t.Run(name, func(t *testing.T) {
			ml := integrated(t, oneBundleCluster(patchShop(nil, tc.patches...)), perLayoutRules())
			if got := patchPlacement(t, ml, tc.patches); !reflect.DeepEqual(got, tc.want) {
				t.Errorf("patches are held by %v, want %v", got, tc.want)
			}
		})
	}
}

// TestLayoutPatches_ARenameThenAPatchBuildsAsBefore: a bundle whose patches
// rename an object its own Kustomization builds and then patch that object,
// by its new name or by the one it had, built on the cluster before the
// per-layout Kustomizations took patches. It is not refused, the file of the
// bundle's own Kustomization is the one written then, byte for byte, and
// kustomize builds every Kustomization of the tree: the renamed object is
// patched. The rename is a JSON6902 patch with a target, a strategic-merge
// patch with a target whose text allows the change of name, or a
// strategic-merge patch without one that names the object by an identity it
// had.
//
// Where the object gets the name a ConfigMap of a layout has, the patch stays
// on the bundle's own Kustomization, where it patches the renamed object, and
// is written on the layout's too, whose own ConfigMap it names.
func TestLayoutPatches_ARenameThenAPatchBuildsAsBefore(t *testing.T) {
	for name, tc := range map[string]struct {
		rename      stack.Patch
		to, patched string
		layoutToo   string
	}{
		"JSON6902, patched by its new name":                                    {rename: renameConfigMap("web-cm", "renamed"), to: "renamed", patched: "renamed"},
		"JSON6902, patched by the name it had":                                 {rename: renameConfigMap("web-cm", "renamed"), to: "renamed", patched: "web-cm"},
		"JSON6902, renamed to the name an object of a layout":                  {rename: renameConfigMap("web-cm", "pre"), to: "pre", patched: "pre", layoutToo: "shop-00-pre"},
		"allowed in the patch text, patched by its new name":                   {rename: renameAllowed("web-cm", "renamed"), to: "renamed", patched: "renamed"},
		"allowed in the patch text, patched by the name it had":                {rename: renameAllowed("web-cm", "renamed"), to: "renamed", patched: "web-cm"},
		"allowed in the patch text, renamed to the name an object of a layout": {rename: renameAllowed("web-cm", "pre"), to: "pre", patched: "pre", layoutToo: "shop-00-pre"},
		"by an identity it had, without a target":                              {rename: renamePrevious("web-cm", "renamed"), to: "renamed", patched: "renamed"},
		"by an identity it had, renamed to the name an object of a layout":     {rename: renamePrevious("web-cm", "pre"), to: "pre", patched: "pre", layoutToo: "shop-00-pre"},
	} {
		t.Run(name, func(t *testing.T) {
			patches := []stack.Patch{tc.rename, untargeted(cmDoc(tc.patched))}
			ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())

			// The rename is on every per-layout Kustomization when it has a
			// target, and on none when it has not.
			want := map[string][]int{"shop": {0, 1}}
			if tc.rename.Target != nil {
				for _, l := range shopLayoutNames {
					want[l] = []int{0}
				}
			}
			if tc.layoutToo != "" {
				want[tc.layoutToo] = append(want[tc.layoutToo], 1)
			}
			if got := patchPlacement(t, ml, patches); !reflect.DeepEqual(got, want) {
				t.Errorf("patches are held by %v, want %v", got, want)
			}

			disk := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
			if got, before := string(disk[shopKustomizationFile]), shopOwnHolding(patches...); got != before {
				t.Errorf("%s:\n%s\nwant the file written before:\n%s", shopKustomizationFile, got, before)
			}

			built, failed := patchedBuilds(t, ml)
			for k, err := range failed {
				t.Errorf("kustomize build of %s: %v", k, err)
			}
			if got, want := built["shop"], (map[string]map[string]string{tc.to: patchedData}); !reflect.DeepEqual(got, want) {
				t.Errorf("the bundle's own Kustomization builds the ConfigMaps %v, want %v", got, want)
			}
			if tc.layoutToo != "" {
				if got := built[tc.layoutToo][tc.patched]; !reflect.DeepEqual(got, patchedData) {
					t.Errorf("%s builds ConfigMap %q with data %v, want %v", tc.layoutToo, tc.patched, got, patchedData)
				}
			}
		})
	}
}

// TestLayoutPatches_ABuildAnnotationThatDoesNotAgree: kustomize panics where
// it reads the identities an object had from annotations that do not agree,
// as many names as namespaces and kinds being its rule. The integration does
// not read a patch that carries such an annotation: it returns, and the patch
// is on the bundle's own Kustomization as it was before the per-layout
// Kustomizations took patches. The tree is not built here, since kustomize
// panics on it there as it did.
func TestLayoutPatches_ABuildAnnotationThatDoesNotAgree(t *testing.T) {
	patches := []stack.Patch{
		{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: web-cm\n  namespace: default\n  annotations:\n    internal.config.kubernetes.io/previousNames: old,older\ndata:\n  patched: \"yes\"\n"},
		untargeted(cmDoc("absent")),
	}
	ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
	if got, want := patchPlacement(t, ml, patches), (map[string][]int{"shop": {0, 1}}); !reflect.DeepEqual(got, want) {
		t.Errorf("patches are held by %v, want %v", got, want)
	}
	disk := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
	if got, before := string(disk[shopKustomizationFile]), shopOwnHolding(patches...); got != before {
		t.Errorf("%s:\n%s\nwant the file written before:\n%s", shopKustomizationFile, got, before)
	}
}

// TestLayoutPatches_ARenameInALayoutsBuild is the limit of the rule: what an
// entry that is not a plain strategic-merge patch does is not followed, so an
// untargeted patch after it reaches a layout's Kustomization only for an
// object that build holds as generated and stays on the bundle's own in every
// case, and an untargeted entry that is not plain stays on the bundle's own
// alone. Three trees are left as they were, the build of the bundle's own
// Kustomization failing on the cluster and nothing refused: a later patch for
// an object only a layout builds, a later patch for the name the entry gives
// an object of a layout, and an untargeted entry with a build annotation for
// an object only a layout builds. Each has a remedy that builds.
func TestLayoutPatches_ARenameInALayoutsBuild(t *testing.T) {
	const noMatch = "no resource matches strategic merge patch"
	ownFails := func(t *testing.T, ml *layout.ManifestLayout) map[string]map[string]map[string]string {
		t.Helper()
		built, failed := patchedBuilds(t, ml)
		if err := failed["shop"]; err == nil || !strings.Contains(err.Error(), noMatch) {
			t.Errorf("kustomize build of the bundle's own Kustomization: %v, want an error containing %q", err, noMatch)
		}
		if len(failed) > 1 {
			t.Errorf("builds failed: %v, want the bundle's own alone", failed)
		}
		return built
	}
	allBuild := func(t *testing.T, ml *layout.ManifestLayout) map[string]map[string]map[string]string {
		t.Helper()
		built, failed := patchedBuilds(t, ml)
		for k, err := range failed {
			t.Errorf("kustomize build of %s: %v", k, err)
		}
		return built
	}

	t.Run("an object only a layout builds", func(t *testing.T) {
		patches := []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("pre"))}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		built := ownFails(t, ml)
		if got := built["shop-00-pre"]["pre"]; !reflect.DeepEqual(got, patchedData) {
			t.Errorf("shop-00-pre builds ConfigMap pre with data %v, want %v", got, patchedData)
		}
	})
	t.Run("remedy: the untargeted patch before the entry", func(t *testing.T) {
		patches := []stack.Patch{untargeted(cmDoc("pre")), annotateEveryConfigMap()}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		if got, want := patchPlacement(t, ml, patches)["shop"], []int{1}; !reflect.DeepEqual(got, want) {
			t.Errorf("the bundle's own holds the patches %v, want %v", got, want)
		}
		built := allBuild(t, ml)
		if got := built["shop-00-pre"]["pre"]; !reflect.DeepEqual(got, patchedData) {
			t.Errorf("shop-00-pre builds ConfigMap pre with data %v, want %v", got, patchedData)
		}
	})

	t.Run("the name the entry gives an object of a layout", func(t *testing.T) {
		patches := []stack.Patch{renameConfigMap("pre", "renamed"), untargeted(cmDoc("renamed"))}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		want := map[string][]int{"shop": {0, 1}}
		for _, l := range shopLayoutNames {
			want[l] = []int{0}
		}
		if got := patchPlacement(t, ml, patches); !reflect.DeepEqual(got, want) {
			t.Errorf("patches are held by %v, want %v", got, want)
		}
		disk := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
		if got, before := string(disk[shopKustomizationFile]), shopOwnHolding(patches...); got != before {
			t.Errorf("%s:\n%s\nwant the file written before:\n%s", shopKustomizationFile, got, before)
		}
		built := ownFails(t, ml)
		// The layout's build renames its ConfigMap and does not patch it.
		if got, want := built["shop-00-pre"], (map[string]map[string]string{"renamed": {}}); !reflect.DeepEqual(got, want) {
			t.Errorf("shop-00-pre builds the ConfigMaps %v, want %v", got, want)
		}
	})
	t.Run("remedy: a target on the patch", func(t *testing.T) {
		patched := stack.Patch{Patch: cmDoc("renamed"), Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "renamed"}}
		patches := []stack.Patch{renameConfigMap("pre", "renamed"), patched}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		built := allBuild(t, ml)
		if got, want := built["shop-00-pre"], (map[string]map[string]string{"renamed": patchedData}); !reflect.DeepEqual(got, want) {
			t.Errorf("shop-00-pre builds the ConfigMaps %v, want %v", got, want)
		}
	})

	t.Run("an untargeted entry with a build annotation for an object only a layout builds", func(t *testing.T) {
		patches := []stack.Patch{renamePrevious("pre", "renamed")}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		if got, want := patchPlacement(t, ml, patches), (map[string][]int{"shop": {0}}); !reflect.DeepEqual(got, want) {
			t.Errorf("patches are held by %v, want %v", got, want)
		}
		built := ownFails(t, ml)
		// The layout's build does not hold the patch: its ConfigMap keeps its name.
		if got, want := built["shop-00-pre"], (map[string]map[string]string{"pre": {}}); !reflect.DeepEqual(got, want) {
			t.Errorf("shop-00-pre builds the ConfigMaps %v, want %v", got, want)
		}
	})
	t.Run("remedy: a target on the entry", func(t *testing.T) {
		patches := []stack.Patch{renameAllowed("pre", "renamed")}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		built := allBuild(t, ml)
		if got, want := built["shop-00-pre"], (map[string]map[string]string{"renamed": {}}); !reflect.DeepEqual(got, want) {
			t.Errorf("shop-00-pre builds the ConfigMaps %v, want %v", got, want)
		}
	})

	// A target is a remedy for an entry of one document. Kustomize refuses
	// one on an entry of several before it selects anything, so that entry,
	// which is written on every Kustomization of the bundle, fails each of
	// their builds, the ones whose objects it does not select included.
	t.Run("a target on an entry of several documents", func(t *testing.T) {
		const refused = "Multiple Strategic-Merge Patches in one `patches` entry is not allowed to set `patches.target` field"
		entry := renameAllowed("pre", "renamed")
		entry.Patch += "---\n" + cmDoc("main")
		ml := integrated(t, oneBundleCluster(patchShop(nil, entry)), perLayoutRules())
		built, failed := patchedBuilds(t, ml)
		if len(built) != 0 {
			t.Errorf("builds made %v, want none to build", built)
		}
		for name, err := range failed {
			if !strings.Contains(err.Error(), refused) {
				t.Errorf("kustomize build of %s: %v, want an error containing %q", name, err, refused)
			}
		}
	})
	t.Run("remedy: one entry for each document, with its target", func(t *testing.T) {
		patched := stack.Patch{Patch: cmDoc("main"), Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "main"}}
		patches := []stack.Patch{renameAllowed("pre", "renamed"), patched}
		ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
		built := allBuild(t, ml)
		if got, want := built["shop-00-pre"], (map[string]map[string]string{"renamed": {}}); !reflect.DeepEqual(got, want) {
			t.Errorf("shop-00-pre builds the ConfigMaps %v, want %v", got, want)
		}
		if got := built["shop-01-main"]["main"]; !reflect.DeepEqual(got, patchedData) {
			t.Errorf("shop-01-main builds ConfigMap main with data %v, want %v", got, patchedData)
		}
	})
}

// TestKustomize_APlainStrategicMergePatchKeepsTheIdentity pins what the
// placement relies on to place by object after a plain strategic-merge patch
// with a target: kustomize merges it into the object the target selects and
// keeps that object's apiVersion, kind, name and namespace, whatever the
// document carries. The same document with the annotation that allows the
// change of name is not plain, and renames the object.
func TestKustomize_APlainStrategicMergePatchKeepsTheIdentity(t *testing.T) {
	const doc = "apiVersion: example.com/v2\nkind: Other\nmetadata:\n  name: renamed\n  namespace: elsewhere\n"
	const data = "data:\n  patched: \"yes\"\n"
	target := &stack.PatchSelector{Kind: "ConfigMap", Name: "web-cm"}
	for name, tc := range map[string]struct {
		patch string
		want  string
	}{
		"plain":                              {patch: doc + data, want: "web-cm"},
		"with the annotation that allows it": {patch: doc + "  annotations:\n    internal.config.kubernetes.io/allowNameChange: enabled\n" + data, want: "renamed"},
	} {
		t.Run(name, func(t *testing.T) {
			patch := stack.Patch{Patch: tc.patch, Target: target}
			ml := integrated(t, oneBundleCluster(patchShop(nil, patch)), perLayoutRules())
			built, failed := patchedBuilds(t, ml)
			for k, err := range failed {
				t.Errorf("kustomize build of %s: %v", k, err)
			}
			if got, want := built["shop"], (map[string]map[string]string{tc.want: patchedData}); !reflect.DeepEqual(got, want) {
				t.Errorf("the bundle's own Kustomization builds the ConfigMaps %v, want %v", got, want)
			}
		})
	}
}
