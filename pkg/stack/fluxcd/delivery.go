package fluxcd

import (
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/built"
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
//
// sources maps the identity (sourceKey) of every Flux Source among those
// objects to the annotations its application's intent asks for. A Source the
// integration derives with that identity is the same object, so it carries
// them too (integratedPlacement.add).
func applyDeliveryIntents(ml *layout.ManifestLayout) (sources map[string]map[string]string, undo func(), err error) {
	sources = map[string]map[string]string{}
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
					if key, ok := sourceKey(item); ok {
						sources[key], _ = merge(sources[key], want)
					}
				}
				written, err := verifyWritten(obj, want)
				if err != nil {
					return errors.Wrapf(err, "application %q", rec.Application.Name)
				}
				// A Source the Go value gave no access to, raw JSON for one,
				// is the application's as well.
				for _, built := range written {
					if key, ok := sourceKey(&unstructured.Unstructured{Object: built}); ok {
						sources[key], _ = merge(sources[key], want)
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
		return nil, nil, err
	}
	return sources, undo, nil
}

// refuseDeliveryIntent fails when an application the tree under ml records
// sets a delivery intent. It is for GenerateFromCluster, which returns a list
// of Kustomizations and Sources and no layout: the intent is annotations on
// the application's objects, and that list holds none of them.
func refuseDeliveryIntent(ml *layout.ManifestLayout) error {
	if ml == nil {
		return nil
	}
	for _, rec := range ml.OriginApplicationObjects() {
		if rec.Application != nil && !rec.Application.Delivery.IsZero() {
			return errors.Errorf("GenerateFromCluster cannot apply the delivery intent of application %q (rendered in %q): the intent is set as annotations on the application's objects, and this list holds only Kustomizations and Sources; use CreateLayoutWithResources, which sets them on the layout it returns",
				rec.Application.Name, ml.FullRepoPath())
		}
	}
	for _, child := range ml.Children {
		if err := refuseDeliveryIntent(child); err != nil {
			return err
		}
	}
	return nil
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

// builtObjects returns the objects of r that an annotation can be set on: of
// the objects kustomize builds from r once it is written (built.Objects, the
// rule the layout writers' pre-write check reads a resource by), the ones r
// holds as Go values with object metadata. That is r itself, or what a List
// holds, a List among the items opened the same way whether or not it carries
// object metadata of its own (metav1.List has none). They are the List's own,
// so a change to one is a change to the List.
//
// An item held as raw JSON is not among them, whatever object is held beside
// it, and neither is one the Go value gives no access to. Whether what was
// reached is all the written objects is checked afterwards, on the written
// form (verifyWritten).
func builtObjects(r runtime.Object) ([]client.Object, error) {
	objs, err := built.Objects(r)
	if err != nil {
		return nil, err
	}
	var out []client.Object
	for _, obj := range objs {
		if own, ok := obj.Object.(client.Object); ok && !obj.Read {
			out = append(out, own)
		}
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
// form, so there is nothing to check. It returns the objects it checked: what
// kustomize builds from a typed r.
func verifyWritten(r client.Object, want map[string]string) ([]map[string]any, error) {
	if _, ok := r.(*unstructured.Unstructured); ok {
		return nil, nil
	}
	written, err := writtenForm(r)
	if err != nil {
		return nil, err
	}
	objs := writtenObjects(written)
	for _, b := range objs {
		have, _, _ := unstructured.NestedStringMap(b, "metadata", "annotations")
		for _, key := range slices.Sorted(maps.Keys(want)) {
			if have[key] != want[key] {
				return nil, errors.Errorf("%s is written with %s lacking annotation %s: %q; the integrator cannot reach that object to set it",
					describeObject(r), describeObject(&unstructured.Unstructured{Object: b}), key, want[key])
			}
		}
	}
	return objs, nil
}
