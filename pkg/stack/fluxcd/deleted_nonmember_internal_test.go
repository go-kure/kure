package fluxcd

import (
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// Every copy of a Kustomization that is no writer counts, not only the one
// the tree index keeps per key (go-kure/kure#1183): h, in a's directory,
// which the bootstrap loads, is created by no apply only while no copy of n
// builds that directory, and while every copy of n reads in typed form.
func TestOnlyWritersLoad_ReadsEveryTreeCopy(t *testing.T) {
	const h, n = "flux-system/h", "flux-system/n"
	copyAt := func(p string) *kustv1.Kustomization {
		k := &kustv1.Kustomization{}
		k.Spec.Path = p
		return k
	}
	for name, tc := range map[string]struct {
		copies  []*kustv1.Kustomization
		copyErr bool
		want    bool
	}{
		"no copy of n builds a's directory":    {copies: []*kustv1.Kustomization{copyAt("./demo/b")}, want: true},
		"the first of n's copies builds it":    {copies: []*kustv1.Kustomization{copyAt("./demo/a"), copyAt("./demo/b")}},
		"the last of n's copies builds it":     {copies: []*kustv1.Kustomization{copyAt("./demo/b"), copyAt("./demo/a")}},
		"a copy of n unreadable in typed form": {copies: []*kustv1.Kustomization{copyAt("./demo/b")}, copyErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			root := &layout.ManifestLayout{Name: "demo", Namespace: "."}
			a := &layout.ManifestLayout{Name: "a", Namespace: "demo"}
			b := &layout.ManifestLayout{Name: "b", Namespace: "demo"}
			root.Children = []*layout.ManifestLayout{a, b}
			tree := &kustomizationTree{
				root:     root,
				layoutAt: map[string]*layout.ManifestLayout{"demo": root, "demo/a": a, "demo/b": b},
				others:   map[*layout.ManifestLayout][]string{a: {h}, b: {n}},
				copies:   map[string][]*kustv1.Kustomization{n: tc.copies},
				copyErr:  map[string]bool{n: tc.copyErr},
			}
			v := &builtView{t: tree, writers: []string{""}, writerSpec: map[string]*kustv1.Kustomization{"": nil}}
			if got := v.onlyWritersLoad(h, v.applyLoads()); got != tc.want {
				t.Errorf("onlyWritersLoad(h) = %v, want %v", got, tc.want)
			}
		})
	}
}
