package fluxcd

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

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
// matching annotations on that application's objects — what a List holds, not
// the envelope (builtObjects) — and, as options.annotations, on the configMapGenerators of its
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
				items, err := builtObjects(obj)
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
				if err := verifyWritten(obj, want); err != nil {
					return errors.Wrapf(err, "application %q", rec.Application.Name)
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

// builtObjects returns the objects kustomize builds from r once it is written:
// r itself, or the objects a List holds, a List among them opened the same
// way. A List is what kustomize takes for one (its resource factory's
// inlineAnyEmbeddedLists): a kind ending in "List" that has an items field.
// Any other kind is one object, whatever fields it has, and so is a kind
// ending in "List" without items; items set to null hold nothing. For a typed
// object the field is read from its written form, so one left out when empty
// counts as absent. The items are the List's own, so a change to one is a
// change to the List. For a typed List they are the ones its Go value gives
// access to; verifyWritten refuses a written item that is not among them.
func builtObjects(r client.Object) ([]client.Object, error) {
	self := []client.Object{r}
	if !strings.HasSuffix(r.GetObjectKind().GroupVersionKind().Kind, "List") {
		return self, nil
	}
	var items []client.Object
	if u, ok := r.(*unstructured.Unstructured); ok {
		held, ok := u.Object["items"]
		if !ok {
			return self, nil
		}
		if held == nil {
			return nil, nil
		}
		if !u.IsList() {
			// Not an array: kustomize refuses the file when it builds it.
			return self, nil
		}
		list, err := u.ToList()
		if err != nil {
			return nil, err
		}
		for i := range list.Items {
			items = append(items, &list.Items[i])
		}
	} else {
		held, ok, err := writtenItems(r)
		if err != nil {
			return nil, err
		}
		if !ok {
			return self, nil
		}
		if held == nil {
			return nil, nil
		}
		if _, ok := held.([]any); !ok {
			// Not an array: kustomize refuses the file when it builds it.
			return self, nil
		}
		// What the Go value gives access to. Whether that is all the
		// written items is checked afterwards, on the written form
		// (verifyWritten).
		if meta.IsListType(r) {
			extracted, err := meta.ExtractList(r)
			if err != nil {
				return nil, err
			}
			for _, item := range extracted {
				if obj, ok := item.(client.Object); ok {
					items = append(items, obj)
				}
			}
		}
	}
	var out []client.Object
	for _, item := range items {
		built, err := builtObjects(item)
		if err != nil {
			return nil, err
		}
		out = append(out, built...)
	}
	return out, nil
}

// writtenForm returns a typed object as it is written: the writers marshal an
// object to JSON first.
func writtenForm(r client.Object) (map[string]any, error) {
	data, err := json.Marshal(r)
	if err != nil {
		return nil, errors.Wrapf(err, "marshal %s", describeObject(r))
	}
	var written map[string]any
	if err := json.Unmarshal(data, &written); err != nil {
		return nil, errors.Wrapf(err, "read %s as written", describeObject(r))
	}
	return written, nil
}

// writtenItems returns the items field of a typed object as it is written,
// and whether the written form has one: an items field left out when empty is
// not there for kustomize.
func writtenItems(r client.Object) (items any, present bool, err error) {
	written, err := writtenForm(r)
	if err != nil {
		return nil, false, err
	}
	items, present = written["items"]
	return items, present, nil
}

// writtenObjects is builtObjects on a written form: the objects kustomize
// builds from it.
func writtenObjects(m map[string]any) []map[string]any {
	self := []map[string]any{m}
	if kind, _ := m["kind"].(string); !strings.HasSuffix(kind, "List") {
		return self
	}
	held, ok := m["items"]
	if !ok {
		return self
	}
	if held == nil {
		return nil
	}
	items, ok := held.([]any)
	if !ok {
		return self
	}
	var out []map[string]any
	for _, item := range items {
		if obj, ok := item.(map[string]any); ok {
			out = append(out, writtenObjects(obj)...)
		}
	}
	return out
}

// verifyWritten checks, once the annotations are set, what they were set for:
// that every object kustomize builds from r's written form carries want. It
// fails for a typed object whose written form holds something its Go value
// gave no access to — a List that writes its items from a field apimachinery
// does not take for them, say. An unstructured object is its own written
// form, so there is nothing to check.
func verifyWritten(r client.Object, want map[string]string) error {
	if _, ok := r.(*unstructured.Unstructured); ok {
		return nil
	}
	written, err := writtenForm(r)
	if err != nil {
		return err
	}
	for _, built := range writtenObjects(written) {
		have, _, _ := unstructured.NestedStringMap(built, "metadata", "annotations")
		for _, key := range slices.Sorted(maps.Keys(want)) {
			if have[key] != want[key] {
				return errors.Errorf("%s is written with %s lacking annotation %s: %q; the integrator cannot reach that object to set it",
					describeObject(r), describeObject(&unstructured.Unstructured{Object: built}), key, want[key])
			}
		}
	}
	return nil
}
