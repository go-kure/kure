package fluxcd

import (
	"fmt"
	"maps"
	"slices"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// deliveryAnnotations is Flux's form of a delivery intent: the per-object
// kustomize-controller annotations that state it. Nil when the intent asks
// for nothing.
func deliveryAnnotations(d stack.DeliveryIntent) map[string]string {
	if d.IsZero() {
		return nil
	}
	out := map[string]string{}
	if d.PruneProtection {
		out[stack.AnnotationFluxPruneKey] = stack.AnnotationFluxPruneDisabled
	}
	if d.ForceReplace {
		out[stack.AnnotationFluxForceKey] = stack.AnnotationFluxForceEnabled
	}
	return out
}

// applyDeliveryIntents sets, for every application the tree under ml records
// (layout.OriginApplicationObjects) whose Delivery asks for something, the
// matching annotations on that application's objects — List items, not the
// envelope — and, as options.annotations, on the configMapGenerators of its
// own layout and the layouts below it, since kustomize builds those
// ConfigMaps later. An annotation already present with the wanted value is
// left alone, so integrating a tree again changes nothing; one present with
// another value is refused, naming the object.
//
// The annotations are set on the objects the layout holds, in place. undo
// takes back exactly what this call added; on error it has already run.
func applyDeliveryIntents(ml *layout.ManifestLayout) (undo func(), err error) {
	var undos []func()
	undo = func() {
		for _, u := range slices.Backward(undos) {
			u()
		}
		undos = nil
	}
	var walk func(l *layout.ManifestLayout) error
	walk = func(l *layout.ManifestLayout) error {
		if l == nil {
			return nil
		}
		for _, rec := range l.OriginApplicationObjects() {
			if rec.Application == nil {
				continue
			}
			want := deliveryAnnotations(rec.Application.Delivery)
			if len(want) == 0 {
				continue
			}
			for _, obj := range rec.Objects {
				if obj == nil {
					continue
				}
				items, err := objectItems(obj)
				if err != nil {
					return errors.Wrapf(err, "application %q: read list items", rec.Application.Name)
				}
				for _, item := range items {
					u, err := annotateObject(rec.Application, item, want)
					if err != nil {
						return err
					}
					if u != nil {
						undos = append(undos, u)
					}
				}
			}
			us, err := annotateGenerators(rec.Application, rec.Layout, want)
			undos = append(undos, us...)
			if err != nil {
				return err
			}
		}
		for _, child := range l.Children {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(ml); err != nil {
		undo()
		return nil, err
	}
	return undo, nil
}

// annotateObject adds want to obj's annotations and returns how to take that
// back, or nil when obj already carries all of want. It changes nothing when
// one of them is present with another value.
func annotateObject(app *stack.Application, obj client.Object, want map[string]string) (func(), error) {
	orig := obj.GetAnnotations()
	if key, have, conflict := conflicting(orig, want); conflict {
		return nil, errors.Errorf("application %q: %s already carries annotation %s: %q, but the application's delivery intent needs %q",
			app.Name, describeObject(obj), key, have, want[key])
	}
	merged, changed := merge(orig, want)
	if !changed {
		return nil, nil
	}
	obj.SetAnnotations(merged)
	return func() { obj.SetAnnotations(orig) }, nil
}

// annotateGenerators adds want to every configMapGenerator of l and of the
// layouts below it. The undo functions it returns are valid on error too.
func annotateGenerators(app *stack.Application, l *layout.ManifestLayout, want map[string]string) ([]func(), error) {
	if l == nil {
		return nil, nil
	}
	var undos []func()
	for i := range l.ConfigMapGenerators {
		gen := &l.ConfigMapGenerators[i]
		orig := gen.Annotations
		if key, have, conflict := conflicting(orig, want); conflict {
			return undos, errors.Errorf("application %q: configMapGenerator %q of layout %q already sets annotation %s: %q, but the application's delivery intent needs %q",
				app.Name, gen.Name, l.FullRepoPath(), key, have, want[key])
		}
		merged, changed := merge(orig, want)
		if !changed {
			continue
		}
		gen.Annotations = merged
		undos = append(undos, func() { gen.Annotations = orig })
	}
	for _, child := range l.Children {
		us, err := annotateGenerators(app, child, want)
		undos = append(undos, us...)
		if err != nil {
			return undos, err
		}
	}
	return undos, nil
}

// conflicting returns the first key of want (in key order) that have sets to
// another value.
func conflicting(have, want map[string]string) (key, value string, found bool) {
	for _, k := range slices.Sorted(maps.Keys(want)) {
		if v, ok := have[k]; ok && v != want[k] {
			return k, v, true
		}
	}
	return "", "", false
}

// merge returns a new map holding have plus want, and whether that differs
// from have. have itself is never written, so it can be put back as it was.
func merge(have, want map[string]string) (map[string]string, bool) {
	changed := false
	for k := range want {
		if _, ok := have[k]; !ok {
			changed = true
		}
	}
	if !changed {
		return have, false
	}
	out := make(map[string]string, len(have)+len(want))
	maps.Copy(out, have)
	maps.Copy(out, want)
	return out, true
}

// describeObject names obj for an error: kind, then namespace/name.
func describeObject(obj client.Object) string {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	if kind == "" {
		kind = fmt.Sprintf("%T", obj)
	}
	name := obj.GetName()
	if ns := obj.GetNamespace(); ns != "" {
		name = ns + "/" + name
	}
	return fmt.Sprintf("%s %q", kind, name)
}

// objectItems returns r itself, or its items when r is a List: a List is an
// envelope, and kustomize builds the items. The items are the List's own, so
// a change to one is a change to the List.
func objectItems(r client.Object) ([]client.Object, error) {
	if u, ok := r.(*unstructured.Unstructured); ok && u.IsList() {
		list, err := u.ToList()
		if err != nil {
			return nil, err
		}
		out := make([]client.Object, 0, len(list.Items))
		for i := range list.Items {
			out = append(out, &list.Items[i])
		}
		return out, nil
	}
	if meta.IsListType(r) {
		items, err := meta.ExtractList(r)
		if err != nil {
			return nil, err
		}
		var out []client.Object
		for _, item := range items {
			if obj, ok := item.(client.Object); ok {
				out = append(out, obj)
			}
		}
		return out, nil
	}
	return []client.Object{r}, nil
}
