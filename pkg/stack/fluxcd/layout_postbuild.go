package fluxcd

import (
	"context"
	stderrors "errors"
	"path"
	"reflect"
	"slices"
	"strings"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	fluxkustomize "github.com/fluxcd/pkg/kustomize"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/kustomize/api/provider"
	"sigs.k8s.io/kustomize/api/resource"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// substituteOptOut is the key of Flux's opt-out from postBuild substitution:
// an object whose label or annotation of that key is "disabled" is applied
// as built, without substitution (fluxcd/pkg/kustomize, SubstituteVariables).
const substituteOptOut = "kustomize.toolkit.fluxcd.io/substitute"

// keepEmbeddedPostBuild keeps the postBuild and the patches of each per-layout
// Kustomization this pass created as they were written, through the build of
// the Kustomization that applies it (go-kure/kure#1021). A per-layout
// Kustomization is a resource of its parent's build, and Flux's postBuild
// substitution of that build runs over everything it applies, this
// Kustomization's own postBuild and patches among it, before this
// Kustomization reads them.
//
// The substitution of every Kustomization this pass placed or kept whose build
// holds the per-layout one is run over it as Flux's own, offline
// (parentSubstitution), and that run decides:
//
//   - when it changes neither the postBuild nor the patches in a way that
//     matters, the Kustomization is written as before;
//   - when it does and changes no other field, the Kustomization gets Flux's
//     opt-out annotation, so that every build holding it applies it as
//     written;
//   - when it does and changes another field as well (a label, an annotation,
//     the path), the integration is refused: the opt-out would drop that
//     substitution, which the parent's build has made until now, without a
//     word.
//
// A change to the postBuild always matters: the Kustomization uses its
// substitute values as written, so a value holding ${...} must not be expanded
// before. A change to the patches matters only where it changes what the
// Kustomization's own substitution makes of them: the patched objects are
// substituted after the patches are applied, with the Kustomization's own
// vars, so what counts is that substitution over the patches the parent left
// against the same over the patches as written. With the vars of the same
// bundle on both, ${VAR} comes out the same whether the parent substituted it
// first or not, and an escape does not: $${VAR}, which the Kustomization's own
// substitution writes as the literal ${VAR}, is unescaped by the parent and
// substituted after all. The parent's vars are not always the child's: a
// Kustomization of the caller's own, kept in the place of the generated one
// of the bundle, carries its own postBuild, and with other values a plain
// ${VAR} matters too. A var that only substituteFrom sets cannot be read
// offline, and counts as a value with no $ in it, as the offline run sets it;
// a value read from the cluster that itself holds ${...} or $$ is therefore
// substituted twice, and that is not seen here.
//
// A Kustomization that carries neither a postBuild nor patches is not read,
// and neither is one an earlier integration placed and this pass kept: that
// one is left as it is.
func (p *integratedPlacement) keepEmbeddedPostBuild(top *layout.ManifestLayout) error {
	if !p.perLayout {
		return nil
	}
	var children []patchBuild
	for _, b := range p.patchOrder {
		for _, build := range p.patchBuilds[b].layouts {
			if build.cr != nil && (build.cr.Spec.PostBuild != nil || len(build.cr.Spec.Patches) > 0) {
				children = append(children, build)
			}
		}
	}
	if len(children) == 0 {
		return nil
	}
	layoutAt, crs, err := p.indexGenerated(top)
	if err != nil {
		return err
	}
	rf := provider.NewDefaultDepProvider().GetResourceFactory()
	for _, child := range children {
		optOut := false
		for _, k := range crs {
			if k.Spec.PostBuild == nil {
				continue
			}
			b := layoutAt[path.Clean(k.Spec.Path)]
			if b == nil || !slices.ContainsFunc(buildScope(b), func(l *layout.ManifestLayout) bool { return holds(l, child.cr) }) {
				continue
			}
			own, other, err := parentSubstitution(rf, k, child.cr)
			if err != nil {
				return errors.Wrapf(err, "Flux Kustomization %q (spec.path %q) builds Flux Kustomization %q, of layout %q, and its postBuild substitution fails on it",
					k.Name, k.Spec.Path, child.name, child.layout.FullRepoPath())
			}
			if len(own) > 0 && len(other) > 0 {
				return errors.Errorf("Flux Kustomization %q (spec.path %q) builds Flux Kustomization %q, of layout %q, and its postBuild substitution changes what the latter applies to its own build (%s) as well as its %s: Flux's opt-out from the substitution would keep the first as written and drop the second, which the build substitutes today; remove the ${...} expressions from its %s (written from the bundle's labels and annotations and the layout's path), or the escapes ($${...}) from the bundle's patches and the ${...} expressions from its postBuild substitute values",
					k.Name, k.Spec.Path, child.name, child.layout.FullRepoPath(), strings.Join(own, " and "), strings.Join(other, ", "), strings.Join(other, ", "))
			}
			optOut = optOut || len(own) > 0
		}
		if optOut {
			annotations := child.cr.GetAnnotations()
			if annotations == nil {
				annotations = map[string]string{}
			}
			annotations[substituteOptOut] = fluxkustomize.DisabledValue
			child.cr.SetAnnotations(annotations)
		}
	}
	return nil
}

// parentSubstitution runs Flux's postBuild substitution with k's vars over
// child, a Kustomization in k's build, as Flux's own in dry-run mode, which
// reads no cluster: a var only substituteFrom sets is substituted with
// nothing. It returns the fields it changes by their path below the object's
// top ("metadata.labels", "spec.path"; "kind" for a top-level value), in
// order: own those of child's postBuild and patches where the change matters
// (see keepEmbeddedPostBuild), other every other field it changes. An object
// Flux's opt-out excludes changes nothing.
func parentSubstitution(rf *resource.Factory, k, child *kustv1.Kustomization) (own, other []string, err error) {
	content, err := comparableContent(child)
	if err != nil {
		return nil, nil, err
	}
	before, after, err := substituteOnce(rf, k, content)
	if err != nil || after == nil {
		return nil, nil, err
	}
	for _, f := range fieldPaths(before, after) {
		changed := !reflect.DeepEqual(fieldAt(before, f), fieldAt(after, f))
		switch f {
		case "spec.postBuild":
			if changed {
				own = append(own, f)
			}
		case "spec.patches":
			matters, err := patchesChangeForChild(rf, child, before, after)
			if err != nil {
				return nil, nil, err
			}
			if matters {
				own = append(own, f)
			}
		default:
			if changed {
				other = append(other, f)
			}
		}
	}
	return own, other, nil
}

// patchesChangeForChild reports whether child's own substitution makes
// something else of the patches the parent's substitution left (in after)
// than of the patches as written (in before): child(parent(p)) != child(p).
// Only the patches are read: the other fields are not substituted a second
// time by anything. A child without a postBuild substitutes nothing, so any
// change the parent made counts. A run that fails on one side and not on the
// other counts as a change: the child's own substitution would fail on the
// objects one set of patches writes and not on those of the other.
func patchesChangeForChild(rf *resource.Factory, child *kustv1.Kustomization, before, after map[string]any) (bool, error) {
	written, left := fieldAt(before, "spec.patches"), fieldAt(after, "spec.patches")
	if child.Spec.PostBuild == nil {
		return !reflect.DeepEqual(written, left), nil
	}
	fromWritten, writtenFails, err := childSubstitution(rf, child, before, written)
	if err != nil {
		return false, err
	}
	fromLeft, leftFails, err := childSubstitution(rf, child, after, left)
	if err != nil {
		return false, err
	}
	if writtenFails || leftFails {
		return writtenFails != leftFails, nil
	}
	return !reflect.DeepEqual(fromWritten, fromLeft), nil
}

// childSubstitution runs child's own substitution, offline, over patches, the
// patches of content. fails is set when the substitution itself fails on them.
func childSubstitution(rf *resource.Factory, child *kustv1.Kustomization, content map[string]any, patches any) (out any, fails bool, err error) {
	if patches == nil {
		return nil, false, nil
	}
	alone := map[string]any{
		"apiVersion": content["apiVersion"],
		"kind":       content["kind"],
		"metadata":   map[string]any{"name": fieldAt(content, "metadata.name")},
		"spec":       map[string]any{"patches": patches},
	}
	_, substituted, err := substituteOnce(rf, child, alone)
	if err != nil {
		var substitution *substitutionError
		if stderrors.As(err, &substitution) {
			return nil, true, nil
		}
		return nil, false, err
	}
	if substituted == nil {
		return patches, false, nil
	}
	return fieldAt(substituted, "spec.patches"), false, nil
}

// substitutionError is a failure of Flux's substitution itself, as opposed to
// one reading or converting the object.
type substitutionError struct{ err error }

func (e *substitutionError) Error() string { return e.err.Error() }
func (e *substitutionError) Unwrap() error { return e.err }

// substituteOnce runs k's postBuild substitution, offline, over content. It
// returns content as read, and as substituted, which is nil when the
// substitution does not run (Flux's opt-out, or no vars and no
// substituteFrom).
func substituteOnce(rf *resource.Factory, k *kustv1.Kustomization, content map[string]any) (before, after map[string]any, err error) {
	res, err := rf.FromMap(content)
	if err != nil {
		return nil, nil, errors.Wrap(err, "read the object")
	}
	before, err = res.Map()
	if err != nil {
		return nil, nil, errors.Wrap(err, "read the object")
	}
	kust, err := runtime.DefaultUnstructuredConverter.ToUnstructured(k)
	if err != nil {
		return nil, nil, errors.Wrap(err, "convert the Kustomization to unstructured")
	}
	opts := []fluxkustomize.SubstituteOption{fluxkustomize.SubstituteWithDryRun(true)}
	if len(k.Spec.PostBuild.SubstituteFrom) > 0 {
		opts = append(opts, fluxkustomize.SubstituteWithAlways(true))
	}
	out, err := fluxkustomize.SubstituteVariables(context.Background(), nil, unstructured.Unstructured{Object: kust}, res, opts...)
	if err != nil {
		return nil, nil, &substitutionError{err: err}
	}
	if out == nil {
		return before, nil, nil
	}
	after, err = out.Map()
	if err != nil {
		return nil, nil, errors.Wrap(err, "read the substituted object")
	}
	return before, after, nil
}

// fieldPaths lists, in order, the fields of two objects' content: the keys of a
// map at the top joined to that map's own keys, and every other top-level key
// alone.
func fieldPaths(a, b map[string]any) []string {
	var out []string
	for _, key := range unionKeys(a, b) {
		am, aIsMap := a[key].(map[string]any)
		bm, bIsMap := b[key].(map[string]any)
		if (aIsMap || a[key] == nil) && (bIsMap || b[key] == nil) && (aIsMap || bIsMap) {
			for _, sub := range unionKeys(am, bm) {
				out = append(out, key+"."+sub)
			}
			continue
		}
		out = append(out, key)
	}
	return out
}

// unionKeys is the sorted union of a's and b's keys.
func unionKeys(a, b map[string]any) []string {
	var keys []string
	for k := range a {
		keys = append(keys, k)
	}
	for k := range b {
		if _, ok := a[k]; !ok {
			keys = append(keys, k)
		}
	}
	slices.Sort(keys)
	return keys
}

// fieldAt is the value at f, a path fieldPaths returned, in content.
func fieldAt(content map[string]any, f string) any {
	top, sub, nested := strings.Cut(f, ".")
	if !nested {
		return content[top]
	}
	m, _ := content[top].(map[string]any)
	return m[sub]
}
