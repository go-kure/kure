package fluxcd

import (
	stderrors "errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/kustomize/kyaml/resid"
)

// laterTree writes, under a new directory, the directory prod/shop holding
// ConfigMap web-cm, and returns builds of that tree.
func laterTree(t *testing.T) *treeBuilds {
	t.Helper()
	tree := filepath.Join(t.TempDir(), "tree")
	shop := filepath.Join(tree, "prod", "shop")
	if err := os.MkdirAll(shop, 0o750); err != nil {
		t.Fatal(err)
	}
	files := map[string]string{
		"kustomization.yaml": "apiVersion: kustomize.config.k8s.io/v1beta1\nkind: Kustomization\nresources:\n- web-cm.yaml\n",
		"web-cm.yaml":        "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: web-cm\n  namespace: default\n",
	}
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(shop, name), []byte(text), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return &treeBuilds{dir: tree, written: true}
}

// laterShop is the bundle's own Kustomization of laterTree with patches.
func laterShop(patches ...kustomize.Patch) *kustv1.Kustomization {
	k := &kustv1.Kustomization{}
	k.Name, k.Namespace = "shop", "flux-system"
	k.Spec.Path = "prod/shop"
	k.Spec.Patches = patches
	return k
}

var (
	laterEntry = kustomize.Patch{
		Patch:  "- op: add\n  path: /metadata/annotations\n  value:\n    note: a\n",
		Target: &kustomize.Selector{Kind: "ConfigMap"},
	}
	laterPre = kustomize.Patch{Patch: "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: pre\n  namespace: default\n"}
)

// laterRecord is what placeBundlePatches records for laterShop with
// laterEntry and laterPre.
func laterRecord() ownPatches {
	return ownPatches{
		bundle: "shop",
		kept:   []kustomize.Patch{laterEntry, laterPre},
		later: []laterPatch{{
			at: 1, index: 1, after: 0,
			ids: []resid.ResId{resid.NewResIdWithNamespace(resid.Gvk{Version: "v1", Kind: "ConfigMap"}, "pre", "default")},
		}},
	}
}

// TestLaterPatchRefusal_TheRerunAgrees: laterPatchRefusal compares the error
// of a build it runs again, in a directory of its own, with the error of the
// check that refused. The build of one copy in two directories fails with
// one text: kustomize's error names no directory.
func TestLaterPatchRefusal_TheRerunAgrees(t *testing.T) {
	k := laterShop(laterEntry, laterPre)
	var texts []string
	for range 2 {
		b := laterTree(t)
		_, err := b.build(k.DeepCopy())
		if err == nil {
			t.Fatal("the build succeeded; want it to fail on patch 1")
		}
		texts = append(texts, err.Error())
	}
	if texts[0] != texts[1] {
		t.Fatalf("two builds of one copy fail with different text:\n%s\n%s", texts[0], texts[1])
	}
	if !strings.Contains(texts[0], `no resource matches strategic merge patch "ConfigMap.v1.[noGrp]/pre.default": no matches for Id `) {
		t.Errorf("the build fails with %q, want kustomize's no-match error for pre", texts[0])
	}
}

// TestLaterPatchRefusal_ARerunThatDiffersKeepsTheGenericRefusal: where the
// build run again fails with another text than the check's, or does not fail,
// the refusal is the check's, unchanged; where it agrees, the patch is named.
func TestLaterPatchRefusal_ARerunThatDiffersKeepsTheGenericRefusal(t *testing.T) {
	b := laterTree(t)
	k := laterShop(laterEntry, laterPre)
	_, cause := b.build(k.DeepCopy())
	if cause == nil {
		t.Fatal("the build succeeded; want it to fail on patch 1")
	}
	generic := stderrors.New("generic refusal")
	f := &buildFailure{key: "flux-system/shop", built: k, cause: cause, err: generic}
	real := func(k *kustv1.Kustomization) error {
		_, err := b.build(k)
		return err
	}
	for name, c := range map[string]struct {
		build func(*kustv1.Kustomization) error
		named bool
	}{
		"the rerun agrees": {build: real, named: true},
		"the rerun fails otherwise": {build: func(k *kustv1.Kustomization) error {
			if len(k.Spec.Patches) == 2 {
				return stderrors.New("another failure")
			}
			return real(k)
		}},
		"the rerun builds": {build: func(k *kustv1.Kustomization) error {
			if len(k.Spec.Patches) == 2 {
				return nil
			}
			return real(k)
		}},
	} {
		t.Run(name, func(t *testing.T) {
			got := laterRecord().refusal(f, generic, c.build)
			if !c.named {
				if got == nil || got.Error() != generic.Error() {
					t.Fatalf("got %v, want the check's refusal unchanged", got)
				}
				return
			}
			if got == nil || got.Error() == generic.Error() || !strings.Contains(got.Error(), "patch 1 has no target and fails the build") || !stderrors.Is(got, generic) {
				t.Fatalf("got %v, want patch 1 named, with the check's refusal as its cause", got)
			}
		})
	}
}
