package fluxcd_test

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the refusal of an untargeted strategic-merge patch that comes
// after an entry that is not plain and fails the build of the bundle's own
// Kustomization (go-kure/kure#1160). The placement leaves such a patch there,
// and the tree stays refused; the refusal names the patch, the entry and the
// object, where the builds show that patch is the one kustomize stopped at.
// Every other failure keeps the generic refusal, kustomize's message.

// laterPatchText is the start of the refusal of patch at of shop, which comes
// after entry after.
func laterPatchText(at, after int) string {
	return fmt.Sprintf(`patch %d has no target and fails the build of the bundle's own Kustomization "flux-system/shop", on which it is written because it comes after entry %d, which is not a plain strategic-merge patch`, at, after)
}

// mustNameALaterPatch fails unless err refuses patch at of shop, after entry
// after, as the patch that fails the build of shop's own Kustomization on
// ConfigMap object, with the generic refusal as its cause, and offers putting
// the patch before the entry exactly when reorder is set.
func mustNameALaterPatch(t *testing.T, err error, at, after int, object string, reorder bool) {
	t.Helper()
	mustContainAll(t, err, "validation failed for Bundle", laterPatchText(at, after),
		fmt.Sprintf(`that build does not hold ConfigMap %q (v1, namespace "default") once the entries before the patch have run: give the patch a target that selects the object as the entries before it leave it`, object),
		`Flux Kustomization "flux-system/shop" (spec.path "`, "does not build as kustomize-controller builds it")
	if got := strings.Contains(err.Error(), fmt.Sprintf("or put it before entry %d", after)); got != reorder {
		t.Errorf("offers putting the patch before entry %d: %v, want %v:\n%v", after, got, reorder, err)
	}
	for i := range 8 {
		if i != at && strings.Contains(err.Error(), fmt.Sprintf("patch %d has no target and fails the build", i)) {
			t.Errorf("names patch %d as well:\n%v", i, err)
		}
	}
}

// mustNotNameALaterPatch fails unless err is a refusal that names no later
// patch.
func mustNotNameALaterPatch(t *testing.T, err error) {
	t.Helper()
	if err == nil {
		t.Fatal("accepted; want the generic refusal")
	}
	if strings.Contains(err.Error(), "has no target and fails the build") {
		t.Errorf("names a later patch; want the generic refusal:\n%v", err)
	}
}

// deleteConfigMap is a strategic-merge patch with a target that removes the
// ConfigMap name from the build.
func deleteConfigMap(name string) stack.Patch {
	return stack.Patch{
		Patch:  "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: " + name + "\n  namespace: default\n$patch: delete\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap", Name: name},
	}
}

// refusalOf integrates the cluster of the bundle shop with patches, under
// per-layout placement, and returns the error.
func refusalOf(patches ...stack.Patch) error {
	_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
		CreateLayoutWithResources(oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
	return err
}

// TestLaterPatchRefusal_TheRemedyBuildsAsBefore: a target on the later patch
// that selects its object is admitted, and the file of the bundle's own
// Kustomization is the one written before the per-layout Kustomizations took
// patches. The placement does not change, so neither does what it admits.
func TestLaterPatchRefusal_TheRemedyBuildsAsBefore(t *testing.T) {
	t.Parallel()
	targeted := func(name string) stack.Patch {
		return stack.Patch{Patch: cmDoc(name), Target: &stack.PatchSelector{Kind: "ConfigMap", Name: name}}
	}
	for name, patches := range map[string][]stack.Patch{
		"an object of a layout":                     {annotateEveryConfigMap(), targeted("pre")},
		"an object no build holds":                  {annotateEveryConfigMap(), targeted("absent")},
		"two documents, one patch per object first": {annotateEveryConfigMap(), targeted("web-cm"), targeted("pre")},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			ml := integrated(t, oneBundleCluster(patchShop(nil, patches...)), perLayoutRules())
			disk := treeFiles(t, writeAll(t, ml)["WriteToDisk"].root)
			if got, before := string(disk[shopKustomizationFile]), shopOwnHolding(patches...); got != before {
				t.Errorf("%s:\n%s\nwant the file written before:\n%s", shopKustomizationFile, got, before)
			}
		})
	}
}

// TestLaterPatchRefusal_OtherFailuresKeepTheGenericRefusal: the build of the
// bundle's own Kustomization fails, a later patch is on it, and the builds do
// not show that patch is the one kustomize stopped at. The refusal is the
// generic one.
func TestLaterPatchRefusal_OtherFailuresKeepTheGenericRefusal(t *testing.T) {
	t.Parallel()
	generic := `Flux Kustomization "flux-system/shop" (spec.path "prod/shop") does not build as kustomize-controller builds it`

	t.Run("a later patch builds, and an entry after it that is not plain fails", func(t *testing.T) {
		t.Parallel()
		err := refusalOf(annotateEveryConfigMap(), untargeted(cmDoc("web-cm")), renamePrevious("pre", "renamed"))
		mustContainAll(t, err, generic)
		mustNotNameALaterPatch(t, err)
	})

	t.Run("an entry that is not plain fails with kustomize's text in its path", func(t *testing.T) {
		t.Parallel()
		missing := stack.Patch{
			Patch:  `- op: remove` + "\n" + `  path: '/data/no resource matches strategic merge patch "ConfigMap.v1.[noGrp]/pre.default": no matches for Id x'` + "\n",
			Target: &stack.PatchSelector{Kind: "ConfigMap", Name: "web-cm"},
		}
		err := refusalOf(missing, untargeted(cmDoc("pre")))
		mustContainAll(t, err, generic)
		mustNotNameALaterPatch(t, err)
	})

	t.Run("a build annotation that does not agree, then a later patch", func(t *testing.T) {
		t.Parallel()
		disagrees := stack.Patch{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: web-cm\n  namespace: default\n  annotations:\n    internal.config.kubernetes.io/previousNames: old,older\ndata:\n  patched: \"yes\"\n"}
		err := refusalOf(disagrees, untargeted(cmDoc("web-cm")))
		mustContainAll(t, err, generic, "recovered from kustomize build panic")
		mustNotNameALaterPatch(t, err)
	})

	t.Run("a later patch fails only in the back-reference pass, and a later entry fails otherwise", func(t *testing.T) {
		t.Parallel()
		hpa := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "autoscaling/v2",
			"kind":       "HorizontalPodAutoscaler",
			"metadata":   map[string]any{"name": "web-hpa", "namespace": "default"},
			"spec": map[string]any{
				"scaleTargetRef": map[string]any{"apiVersion": "apps/v1", "kind": "Deployment"},
				"maxReplicas":    int64(2),
			},
		}}
		// The HPA has no scaleTargetRef.name, so the patch adds one, a map
		// kustomize merges and its back-reference pass, after every patch,
		// refuses; the targeted entry after it replaces the map.
		mapName := untargeted("apiVersion: autoscaling/v2\nkind: HorizontalPodAutoscaler\nmetadata:\n  name: web-hpa\n  namespace: default\nspec:\n  scaleTargetRef:\n    name:\n      invalid: value\n")
		repair := stack.Patch{
			Patch:  "- op: replace\n  path: /spec/scaleTargetRef/name\n  value: web\n",
			Target: &stack.PatchSelector{Kind: "HorizontalPodAutoscaler", Name: "web-hpa"},
		}
		b := patchShop(nil, annotateEveryConfigMap(), mapName, repair, renamePrevious("pre", "renamed"))
		b.Applications[0] = objApp("web", cmObjValue("web-cm"), hpa)
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
			CreateLayoutWithResources(oneBundleCluster(b), perLayoutRules())
		mustContainAll(t, err, generic)
		mustNotNameALaterPatch(t, err)
	})

	t.Run("a parent's patch changes the copy the bundle's own Kustomization is built from", func(t *testing.T) {
		t.Parallel()
		shop := patchShop(nil, annotateEveryConfigMap(), untargeted(cmDoc("pre")))
		outer := umbrellaOf("outer", shop)
		outer.Patches = []stack.Patch{{
			Patch:  "- op: replace\n  path: /spec/patches/0/patch\n  value: |\n    - op: add\n      path: /metadata/annotations\n      value:\n        note: b\n",
			Target: &stack.PatchSelector{Kind: "Kustomization", Name: "shop"},
		}}
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
			CreateLayoutWithResources(oneBundleCluster(outer), perLayoutRules())
		mustContainAll(t, err, `Flux Kustomization "flux-system/shop"`, "does not build as kustomize-controller builds it",
			`no resource matches strategic merge patch "ConfigMap.v1.[noGrp]/pre.default"`)
		mustNotNameALaterPatch(t, err)
	})
}

// TestLaterPatchRefusal_AtEachCheckThatBuilds: the refusal names the later
// patch wherever the integration's checks first build the bundle's own
// Kustomization: the reconcile-order check, and under UmbrellasDoNotWait the
// check of the rewritten dependsOn, which runs first. Separate placement
// writes every patch on the bundle's own Kustomization, as before
// go-kure/kure#1021: its check refuses such a build with the generic
// refusal.
func TestLaterPatchRefusal_AtEachCheckThatBuilds(t *testing.T) {
	t.Parallel()
	patches := func() []stack.Patch { return []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("pre"))} }

	t.Run("the reconcile-order check", func(t *testing.T) {
		t.Parallel()
		mustNameALaterPatch(t, refusalOf(patches()...), 1, 0, "pre", true)
	})

	t.Run("the check of the rewritten dependsOn", func(t *testing.T) {
		t.Parallel()
		_, err := fluxstack.NewLayoutIntegrator(noWaitGenerator()).
			CreateLayoutWithResources(noWaitCluster(umbrellaOf("platform", patchShop(nil, patches()...)), true), perLayoutRules())
		mustNameALaterPatch(t, err, 1, 0, "pre", true)
	})

	t.Run("separate placement", func(t *testing.T) {
		t.Parallel()
		shop := srBundle("shop", cmApp("web"))
		shop.Patches = []stack.Patch{annotateEveryConfigMap(), untargeted(cmDoc("absent"))}
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).
			CreateLayoutWithResources(oneBundleCluster(shop), withPlacement(layout.DefaultLayoutRules(), layout.FluxSeparate))
		mustContainAll(t, err, `Flux Kustomization "flux-system/shop"`, "does not build as kustomize-controller builds it",
			`no resource matches strategic merge patch "ConfigMap.v1.[noGrp]/absent.default"`)
		mustNotNameALaterPatch(t, err)
	})
}

// cmObjValue is the ConfigMap name in the namespace default.
func cmObjValue(name string) client.Object { return *cmObj(name) }
