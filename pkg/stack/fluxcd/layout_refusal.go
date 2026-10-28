package fluxcd

import (
	stderrors "errors"
	"fmt"
	"reflect"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	"github.com/fluxcd/pkg/apis/kustomize"
	"sigs.k8s.io/kustomize/kyaml/resid"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// ownPatches is what placeBundlePatches wrote on a bundle's own Kustomization
// that holds a later patch: the bundle's path, the patches as written, in
// their order, and each later patch among them.
type ownPatches struct {
	bundle string
	kept   []kustomize.Patch
	later  []laterPatch
}

// laterPatch is an untargeted plain strategic-merge patch that comes after an
// entry that is not plain, which placeBundlePatches writes on the bundle's own
// Kustomization whatever it names (go-kure/kure#1160).
type laterPatch struct {
	// at is its position in the patches as written, index its index in the
	// bundle's patches and after the index of the first entry that is not
	// plain.
	at, index, after int
	// ids are the identities its documents name, as kustomize reads them.
	ids []resid.ResId
	// before says that placement by object would take it, were it before the
	// entry: some build of the bundle holds every object it names as
	// generated.
	before bool
}

// noMatch is kustomize's error for an untargeted strategic-merge patch whose
// object its build does not hold, the patch's identity quoted (kustomize api
// v0.21.1 internal/builtins/PatchTransformer.go:117-121), followed by its
// lookup's error for no object found (resource/reswrangler.go, GetById).
const noMatch = "no resource matches strategic merge patch %q: no matches for Id "

// laterPatchRefusal returns the refusal of the tree root for err, a check's
// refusal of it. Where err is the failure of the build of a bundle's own
// Kustomization that holds a later patch (buildFailure), and the builds show
// that patch is the entry kustomize stopped at, the refusal names the patch,
// the entry it comes after and the object kustomize names, with err as its
// cause. Every other refusal is err.
//
// The builds run in a directory of their own, written from root as the check
// wrote it: the build of the copy the check built from, which must fail as
// the check's did, and then, for each later patch in order, that copy with
// the patches cut after it. The first whose build fails is the one kustomize
// stopped at, where its build fails as the whole copy's does and the build
// cut before it does not.
func (p *integratedPlacement) laterPatchRefusal(root *layout.ManifestLayout, err error) error {
	var f *buildFailure
	if !stderrors.As(err, &f) {
		return err
	}
	rec, ok := p.laterPatches[f.key]
	if !ok {
		return err
	}
	builds := newTreeBuilds(root)
	defer builds.close()
	return rec.refusal(f, err, func(k *kustv1.Kustomization) error {
		_, err := builds.build(k)
		return err
	})
}

// refusal is laterPatchRefusal's for the failure f of the bundle's own
// Kustomization, build running the build of a copy of it.
func (rec ownPatches) refusal(f *buildFailure, err error, build func(*kustv1.Kustomization) error) error {
	// A parent's patch that changed what is written leaves positions that say
	// nothing of the copy built.
	if !reflect.DeepEqual(f.built.Spec.Patches, rec.kept) {
		return err
	}
	whole := build(f.built.DeepCopy())
	if whole == nil || whole.Error() != f.cause.Error() {
		return err
	}
	cut := func(n int) error {
		k := f.built.DeepCopy()
		k.Spec.Patches = k.Spec.Patches[:n]
		return build(k)
	}
	for _, lp := range rec.later {
		failed := cut(lp.at + 1)
		if failed == nil {
			continue
		}
		if failed.Error() != whole.Error() || cut(lp.at) != nil {
			return err
		}
		return errors.ResourceValidationError("Bundle", rec.bundle, "patches", lp.reason(f.key, whole), err)
	}
	return err
}

// reason words the refusal of lp, which fails the build of the Kustomization
// with key with buildErr.
func (lp laterPatch) reason(key string, buildErr error) string {
	out := fmt.Sprintf("patch %d has no target and fails the build of the bundle's own Kustomization %q, on which it is written because it comes after entry %d, which is not a plain strategic-merge patch",
		lp.index, key, lp.after)
	remedy := "give the patch a target that selects its object as the entries before it leave it"
	for _, id := range lp.ids {
		if strings.Contains(buildErr.Error(), fmt.Sprintf(noMatch, id.String())) {
			out += fmt.Sprintf("; that build does not hold %s once the entries before the patch have run", describeID(id))
			remedy = "give the patch a target that selects the object as the entries before it leave it"
			break
		}
	}
	if lp.before {
		remedy += fmt.Sprintf(", or put it before entry %d", lp.after)
	}
	return out + ": " + remedy
}
