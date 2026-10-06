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

// Tests for a bundle's patch list in which an entry can change an object's
// identity (go-kure/kure#1021): a JSON6902 patch with a target. An untargeted
// strategic-merge patch after it is not placed by object and never refused,
// since the identities as generated no longer say what it names. These tests
// run the build of kustomize-controller on the tree the writers wrote.

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

// shopOwnWithPatches is the file of the bundle's own Kustomization of
// patchShop with two patches, a rename of the ConfigMap from to the name to
// and an untargeted patch for the ConfigMap patched, as the integration wrote
// it before the per-layout Kustomizations took patches: both on it.
func shopOwnWithPatches(from, to, patched string) string {
	return `apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: shop
  namespace: flux-system
spec:
  interval: 1h0m0s
  patches:
  - patch: |
      - op: replace
        path: /metadata/name
        value: ` + to + `
    target:
      kind: ConfigMap
      name: ` + from + `
  - patch: |
      apiVersion: v1
      kind: ConfigMap
      metadata:
        name: ` + patched + `
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

// TestLayoutPatches_AfterAnEntryThatCanRename: an untargeted strategic-merge
// patch that comes after a JSON6902 patch with a target stays on the bundle's
// own Kustomization, is written besides on each per-layout Kustomization
// whose build holds every object it names as generated, and is not refused,
// whatever it names. One that comes before that entry is placed by object,
// and so is one after a strategic-merge patch with a target or a JSON6902
// entry without one, neither of which can change an identity.
func TestLayoutPatches_AfterAnEntryThatCanRename(t *testing.T) {
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
		"a strategic-merge patch with a target is no such entry": {
			patches: []stack.Patch{everyConfigMapMerged("a"), untargeted(cmDoc("pre"))},
			want:    with(with(layouts(0), "shop", 0), "shop-00-pre", 0, 1),
		},
		"a JSON6902 entry without a target is no such entry": {
			patches: []stack.Patch{{Patch: json6902}, untargeted(cmDoc("pre"))},
			want:    map[string][]int{"shop": {0}, "shop-00-pre": {1}},
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
// patched.
//
// The third case gives the object the name a ConfigMap of a layout has: the
// patch stays on the bundle's own Kustomization, where it patches the renamed
// object, and is written on the layout's too, whose own ConfigMap it names.
func TestLayoutPatches_ARenameThenAPatchBuildsAsBefore(t *testing.T) {
	for name, tc := range map[string]struct {
		from, to, patched string
		layoutToo         string
	}{
		"patched by its new name":                   {from: "web-cm", to: "renamed", patched: "renamed"},
		"patched by the name it had":                {from: "web-cm", to: "renamed", patched: "web-cm"},
		"renamed to the name an object of a layout": {from: "web-cm", to: "pre", patched: "pre", layoutToo: "shop-00-pre"},
	} {
		t.Run(name, func(t *testing.T) {
			patches := []stack.Patch{renameConfigMap(tc.from, tc.to), untargeted(cmDoc(tc.patched))}
			ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())

			want := map[string][]int{"shop": {0, 1}}
			for _, l := range shopLayoutNames {
				want[l] = []int{0}
			}
			if tc.layoutToo != "" {
				want[tc.layoutToo] = []int{0, 1}
			}
			if got := patchPlacement(t, ml, patches); !reflect.DeepEqual(got, want) {
				t.Errorf("patches are held by %v, want %v", got, want)
			}

			disk := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
			if got, before := string(disk[shopKustomizationFile]), shopOwnWithPatches(tc.from, tc.to, tc.patched); got != before {
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

// TestLayoutPatches_ARenameInALayoutsBuild is the limit of the rule: what an
// untargeted patch after an entry that can rename names is not followed, so
// the patch reaches a layout's Kustomization only for an object that build
// holds as generated, and stays on the bundle's own in every case. Two trees
// are left as they were, the build of the bundle's own Kustomization failing
// on the cluster and nothing refused: a patch for an object only a layout
// builds, and a patch for the name the entry gives an object of a layout.
// Each has a remedy that builds.
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
		if got, before := string(disk[shopKustomizationFile]), shopOwnWithPatches("pre", "renamed", "renamed"); got != before {
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
}

// TestKustomize_AStrategicMergePatchWithATargetKeepsTheIdentity pins what
// the placement relies on to count a JSON6902 patch with a target as the one
// entry that can change an identity: kustomize merges a strategic-merge
// patch with a target into the object the target selects and keeps that
// object's apiVersion, kind, name and namespace, whatever the document
// carries.
func TestKustomize_AStrategicMergePatchWithATargetKeepsTheIdentity(t *testing.T) {
	patch := stack.Patch{
		Patch:  "apiVersion: example.com/v2\nkind: Other\nmetadata:\n  name: renamed\n  namespace: elsewhere\ndata:\n  patched: \"yes\"\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "web-cm"},
	}
	ml := integrated(t, oneBundleCluster(patchShop(nil, patch)), perLayoutRules())
	built, failed := patchedBuilds(t, ml)
	for k, err := range failed {
		t.Errorf("kustomize build of %s: %v", k, err)
	}
	if got, want := built["shop"], (map[string]map[string]string{"web-cm": patchedData}); !reflect.DeepEqual(got, want) {
		t.Errorf("the bundle's own Kustomization builds the ConfigMaps %v, want %v", got, want)
	}
}
