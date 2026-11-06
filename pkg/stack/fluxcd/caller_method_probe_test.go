package fluxcd_test

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// A caller's object is code kure does not own. Its metadata methods are read
// at many places after the entry phase of an entry point (go-kure/kure#1245).
// An entry point that reads such an object checks the five methods once, at
// its entry, and a method that panics is a refusal that names the Go type of
// the object, never the panic's text and never a panic. The cases below run
// every method that can panic through every entry point that reads a caller's
// object (the entries a recorder showed reach one: UmbrellaTerminals,
// IntegrateWithLayout, CreateLayoutWithResources and the layout writers), in
// each shape a caller can hand the object over in.

const callerMethodPanicText = "secret-panic-text-of-a-callers-method"

// panicObject is a typed object whose method panicOn panics.
type panicObject struct {
	*corev1.ConfigMap
	panicOn string
	// setCalls counts the calls of SetAnnotations; panicOnSetCall, when
	// positive, is the call of it that panics.
	setCalls       *int
	panicOnSetCall int
}

func (o *panicObject) boom(method string) {
	if o.panicOn == method {
		panic(callerMethodPanicText)
	}
}

func (o *panicObject) GetObjectKind() schema.ObjectKind {
	o.boom("GetObjectKind")
	return o.ConfigMap.GetObjectKind()
}
func (o *panicObject) GetName() string { o.boom("GetName"); return o.ConfigMap.GetName() }
func (o *panicObject) GetNamespace() string {
	o.boom("GetNamespace")
	return o.ConfigMap.GetNamespace()
}
func (o *panicObject) GetLabels() map[string]string {
	o.boom("GetLabels")
	return o.ConfigMap.GetLabels()
}
func (o *panicObject) GetAnnotations() map[string]string {
	o.boom("GetAnnotations")
	return o.ConfigMap.GetAnnotations()
}
func (o *panicObject) SetAnnotations(a map[string]string) {
	o.boom("SetAnnotations")
	if o.setCalls != nil {
		*o.setCalls++
		if o.panicOnSetCall > 0 && *o.setCalls == o.panicOnSetCall {
			panic(callerMethodPanicText)
		}
	}
	o.ConfigMap.SetAnnotations(a)
}
func (o *panicObject) DeepCopyObject() runtime.Object { c := *o; return &c }

// panicWrapper is a wrapper that embeds an unstructured object, which the
// layout keeps as it is, and whose method panicOn panics.
type panicWrapper struct {
	*unstructured.Unstructured
	panicOn string
}

func (o panicWrapper) boom(method string) {
	if o.panicOn == method {
		panic(callerMethodPanicText)
	}
}

func (o panicWrapper) GetObjectKind() schema.ObjectKind {
	o.boom("GetObjectKind")
	return o.Unstructured.GetObjectKind()
}
func (o panicWrapper) GetName() string { o.boom("GetName"); return o.Unstructured.GetName() }
func (o panicWrapper) GetNamespace() string {
	o.boom("GetNamespace")
	return o.Unstructured.GetNamespace()
}
func (o panicWrapper) GetLabels() map[string]string {
	o.boom("GetLabels")
	return o.Unstructured.GetLabels()
}
func (o panicWrapper) GetAnnotations() map[string]string {
	o.boom("GetAnnotations")
	return o.Unstructured.GetAnnotations()
}

var callerMethods = []string{"GetObjectKind", "GetName", "GetNamespace", "GetLabels", "GetAnnotations"}

// callerKinds are the kinds the entries read differently: a Flux Kustomization
// and a Source are indexed by name, any other object by kind alone.
var callerKinds = [][2]string{{"v1", "ConfigMap"}, {"kustomize.toolkit.fluxcd.io/v1", "Kustomization"}, {"source.toolkit.fluxcd.io/v1", "GitRepository"}}

func panicConfigMap(kind [2]string) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: kind[0], Kind: kind[1]},
		ObjectMeta: metav1.ObjectMeta{Name: "hostile", Namespace: "default", Labels: map[string]string{"a": "b"}, Annotations: map[string]string{"c": "d"}},
	}
}

func panicWrapperOf(kind [2]string, method string) client.Object {
	u := &unstructured.Unstructured{Object: map[string]any{}}
	u.SetAPIVersion(kind[0])
	u.SetKind(kind[1])
	u.SetName("hostile")
	u.SetNamespace("default")
	return panicWrapper{Unstructured: u, panicOn: method}
}

// callerShapes hand a caller's object over as the object itself, as a wrapper
// around an unstructured one, as an item of a typed List, and as an item of a
// List within a List.
var callerShapes = map[string]func(kind [2]string, method string) client.Object{
	"object": func(kind [2]string, method string) client.Object {
		return &panicObject{ConfigMap: panicConfigMap(kind), panicOn: method}
	},
	"wrapper": panicWrapperOf,
	"item of a list": func(kind [2]string, method string) client.Object {
		l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
		l.Name, l.Namespace = "holder", "default"
		l.Items = []runtime.Object{&panicObject{ConfigMap: panicConfigMap(kind), panicOn: method}, panicWrapperOf(kind, method)}
		return l
	},
	"item of a list within a list": func(kind [2]string, method string) client.Object {
		inner := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
		inner.Name, inner.Namespace = "inner", "default"
		inner.Items = []runtime.Object{&panicObject{ConfigMap: panicConfigMap(kind), panicOn: method}}
		l := &objectItems{TypeMeta: metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "List"}}
		l.Name, l.Namespace = "holder", "default"
		l.Items = []runtime.Object{inner}
		return l
	},
}

// readsCallerObject are the entry points that read a caller's object, each
// run on a fresh cluster that emits r.
var readsCallerObject = map[string]func(r client.Object, intent stack.DeliveryIntent) error{
	"UmbrellaTerminals": func(r client.Object, intent stack.DeliveryIntent) error {
		_, ml, err := walkPanicCluster(r, intent)
		if err != nil {
			return err
		}
		_, err = fluxstack.UmbrellaTerminals(ml)
		return err
	},
	"IntegrateWithLayout": func(r client.Object, intent stack.DeliveryIntent) error {
		c, ml, err := walkPanicCluster(r, intent)
		if err != nil {
			return err
		}
		return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, nodeOnly)
	},
	"CreateLayoutWithResources": func(r client.Object, intent stack.DeliveryIntent) error {
		_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(panicCluster(r, intent), nodeOnly)
		return err
	},
	"WriteToDisk of a walked layout": func(r client.Object, intent stack.DeliveryIntent) error {
		_, ml, err := walkPanicCluster(r, intent)
		if err != nil {
			return err
		}
		return ml.WriteToDisk(panicOutDir)
	},
	"WriteToTar of a walked layout": func(r client.Object, intent stack.DeliveryIntent) error {
		_, ml, err := walkPanicCluster(r, intent)
		if err != nil {
			return err
		}
		return ml.WriteToTar(io.Discard)
	},
	"WriteToTar of an integrated layout": func(r client.Object, intent stack.DeliveryIntent) error {
		c, ml, err := walkPanicCluster(r, intent)
		if err != nil {
			return err
		}
		if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, nodeOnly); err != nil {
			return err
		}
		return ml.WriteToTar(io.Discard)
	},
}

// panicOutDir is where the WriteToDisk case writes; the test sets it.
var panicOutDir string

func panicCluster(r client.Object, intent stack.DeliveryIntent) *stack.Cluster {
	c := hostileCluster(r)
	c.Node.Bundle.Applications[0].Delivery = intent
	return c
}

func walkPanicCluster(r client.Object, intent stack.DeliveryIntent) (*stack.Cluster, *layout.ManifestLayout, error) {
	c := panicCluster(r, intent)
	ml, err := layout.WalkCluster(c, nodeOnly)
	return c, ml, err
}

// callEntry runs f and reports a panic as an error that says so.
func callEntry(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("PANIC: %v", r)
		}
	}()
	return f()
}

// snapshotOf is the JSON of a caller's object, for a test that a refusal left
// it as it was. Marshalling reads fields and calls none of the metadata
// methods the cases make panic.
func snapshotOf(t *testing.T, obj client.Object) string {
	t.Helper()
	data, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("snapshot of %T: %v", obj, err)
	}
	return string(data)
}

// TestCallerMethodPanic_EveryReadingEntryRefuses: a metadata method of a
// caller's object that panics is a refusal at every entry point that reads the
// object, whichever shape the object came in and whichever method it is.
func TestCallerMethodPanic_EveryReadingEntryRefuses(t *testing.T) {
	panicOutDir = t.TempDir()
	for entry, run := range readsCallerObject {
		for shape, build := range callerShapes {
			for _, kind := range callerKinds {
				for _, method := range callerMethods {
					t.Run(entry+"/"+shape+"/"+kind[1]+"/"+method, func(t *testing.T) {
						obj := build(kind, method)
						before := snapshotOf(t, obj)
						err := callEntry(func() error { return run(obj, stack.DeliveryIntent{}) })
						if after := snapshotOf(t, obj); after != before {
							t.Errorf("the refusal left the caller's object changed:\nbefore %s\nafter  %s", before, after)
						}
						if err == nil {
							t.Fatalf("%s returned no error for an object whose %s panics", entry, method)
						}
						msg := err.Error()
						if strings.HasPrefix(msg, "PANIC: ") {
							t.Fatalf("%s panicked instead of refusing: %s", entry, msg)
						}
						if strings.Contains(msg, callerMethodPanicText) {
							t.Errorf("the refusal carries the text of the panic: %s", msg)
						}
						if !strings.Contains(msg, "fluxcd_test.panic") {
							t.Errorf("the refusal does not name the Go type of the object: %s", msg)
						}
						if entries, _ := os.ReadDir(panicOutDir); len(entries) != 0 {
							t.Errorf("the refusal came after the writer wrote %d entries to %s", len(entries), panicOutDir)
						}
					})
				}
			}
		}
	}
}

// TestCallerMethodPanic_SetAnnotationsOfADeliveryIntentIsARefusal: an
// application's delivery intent is set as annotations on the application's
// objects, in place, by SetAnnotations of a caller's object. That call that
// panics is a refusal, not a panic, and the refusal names the Go type.
func TestCallerMethodPanic_SetAnnotationsOfADeliveryIntentIsARefusal(t *testing.T) {
	panicOutDir = t.TempDir()
	intent := stack.DeliveryIntent{PruneProtection: true}
	for _, entry := range []string{"IntegrateWithLayout", "CreateLayoutWithResources", "WriteToTar of an integrated layout"} {
		t.Run(entry, func(t *testing.T) {
			obj := &panicObject{ConfigMap: panicConfigMap(callerKinds[0]), panicOn: "SetAnnotations"}
			before := snapshotOf(t, obj)
			err := callEntry(func() error { return readsCallerObject[entry](obj, intent) })
			if after := snapshotOf(t, obj); after != before {
				t.Errorf("the refusal left the caller's object changed:\nbefore %s\nafter  %s", before, after)
			}
			if err == nil {
				t.Fatalf("%s returned no error for an object whose SetAnnotations panics", entry)
			}
			if strings.HasPrefix(err.Error(), "PANIC: ") {
				t.Fatalf("%s panicked instead of refusing: %s", entry, err)
			}
			if strings.Contains(err.Error(), callerMethodPanicText) || !strings.Contains(err.Error(), "fluxcd_test.panicObject") {
				t.Errorf("the refusal must name the Go type and not the text: %s", err)
			}
		})
	}
}

// TestCallerMethodPanic_UndoOfADeliveryIntentDoesNotPanic: a check that
// refuses after the delivery intent was applied makes the entry point take the
// annotations back, by SetAnnotations of the caller's object. That call panics
// here (the second call of it: the first applied the intent). The error holds
// both refusals: the one that started the undo, and the undo's, which names
// the Go type and says the object could not be restored, because kure's
// annotations stay on it. Nothing panics. A setter that panics part-way may
// leave the object changed, and the undo does not promise otherwise.
func TestCallerMethodPanic_UndoOfADeliveryIntentDoesNotPanic(t *testing.T) {
	intent := stack.DeliveryIntent{PruneProtection: true}
	for _, entry := range []string{"IntegrateWithLayout", "CreateLayoutWithResources"} {
		t.Run(entry, func(t *testing.T) {
			calls := 0
			obj := &panicObject{ConfigMap: panicConfigMap(callerKinds[0]), setCalls: &calls, panicOnSetCall: 2}
			err := callEntry(func() error {
				r := client.Object(obj)
				c := panicCluster(r, intent)
				// A JSON patch no target names is refused after the intent is applied.
				c.Node.Bundle.Patches = []stack.Patch{{Patch: "- op: add\n  path: /metadata/labels/z\n  value: y\n"}}
				if entry == "CreateLayoutWithResources" {
					_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, nodeOnly)
					return err
				}
				ml, err := layout.WalkCluster(c, nodeOnly)
				if err != nil {
					return err
				}
				return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, nodeOnly)
			})
			if calls < 2 {
				t.Fatalf("the intent was not applied and undone: SetAnnotations was called %d time(s), want 2", calls)
			}
			if err == nil {
				t.Fatalf("%s returned no error, want the refusal that started the undo", entry)
			}
			msg := err.Error()
			if strings.HasPrefix(msg, "PANIC: ") {
				t.Fatalf("%s panicked while it took the intent back: %s", entry, msg)
			}
			if !strings.Contains(msg, "must specify a target for JSON patch") {
				t.Errorf("the original refusal is missing: %s", msg)
			}
			if !strings.Contains(msg, "fluxcd_test.panicObject") || !strings.Contains(msg, "could not be restored") {
				t.Errorf("the undo's refusal must name the Go type and say the object could not be restored: %s", msg)
			}
			if strings.Contains(msg, callerMethodPanicText) {
				t.Errorf("the error carries the text of the panic: %s", msg)
			}
		})
	}
}

// A refusal by the probe puts back what the normalization before it replaced
// (go-kure/kure#1201): the probe is a later pass than the normalization, and
// a call that refuses must leave every caller's object as it found it. The
// control shows the normalization does replace the map of an object that
// holds a plain int, when nothing refuses.
func TestCallerMethodPanic_ProbeRefusalPutsBackWhatNormalizationReplaced(t *testing.T) {
	integrate := func(second client.Object) (*unstructured.Unstructured, error) {
		u := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "plain", "namespace": "default"},
			"data":     map[string]any{"n": int(1)},
		}}
		c, ml, err := walkPanicCluster(second, stack.DeliveryIntent{})
		if err != nil {
			return u, err
		}
		ml.Resources = append(ml.Resources, u)
		err = callEntry(func() error {
			return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, nodeOnly)
		})
		return u, err
	}
	numberType := func(u *unstructured.Unstructured) string {
		return fmt.Sprintf("%T", u.Object["data"].(map[string]any)["n"])
	}

	control := &panicObject{ConfigMap: panicConfigMap(callerKinds[0])}
	u, err := integrate(control)
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	if got := numberType(u); got != "int64" {
		t.Fatalf("control: the normalization left a %s, want int64: the case below would prove nothing", got)
	}

	hostile := &panicObject{ConfigMap: panicConfigMap(callerKinds[0]), panicOn: "GetLabels"}
	u, err = integrate(hostile)
	if err == nil || !strings.Contains(err.Error(), "fluxcd_test.panicObject") {
		t.Fatalf("want the refusal naming fluxcd_test.panicObject, got %v", err)
	}
	if got := numberType(u); got != "int" {
		t.Errorf("the refused call left the unstructured object's number as %s, want the caller's int", got)
	}
}

// A delivery intent is applied to the objects an application's origin record
// retains, which the layouts hold too but a caller can drop from one and keep
// in the other. The probe reads what the integration reads: the retained
// objects as well as the layouts' own resources.
func TestCallerMethodPanic_ObjectsOfAnOriginRecordAreProbed(t *testing.T) {
	for _, method := range callerMethods {
		t.Run(method, func(t *testing.T) {
			obj := &panicObject{ConfigMap: panicConfigMap(callerKinds[0]), panicOn: method}
			c, ml, err := walkPanicCluster(obj, stack.DeliveryIntent{PruneProtection: true})
			if err != nil {
				t.Fatalf("walk: %v", err)
			}
			var drop func(l *layout.ManifestLayout)
			drop = func(l *layout.ManifestLayout) {
				if l == nil {
					return
				}
				l.Resources = nil
				for _, child := range l.Children {
					drop(child)
				}
				drop(l.OriginUnit())
			}
			drop(ml)
			err = callEntry(func() error {
				return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, nodeOnly)
			})
			if err == nil || strings.HasPrefix(err.Error(), "PANIC:") {
				t.Fatalf("want a refusal, got %v", err)
			}
			if !strings.Contains(err.Error(), "fluxcd_test.panicObject") || strings.Contains(err.Error(), callerMethodPanicText) {
				t.Errorf("the refusal %q must name fluxcd_test.panicObject and not the panic text", err)
			}
		})
	}
}

// Two bundles merged into one directory share one Kustomization, and a
// bundle's targeted patch is checked against the objects of every other bundle
// of the unit (checkPatchScope), which calls the metadata methods of a caller's
// object. GenerateFromLayout and GenerateFromCluster probe the objects of every
// bundle of every unit before that check.

// mergedPatchCluster is a root node whose bundle carries a patch that targets
// ConfigMaps by label and annotation, and a child node whose bundle's
// application emits r. A node-flat walk merges both bundles into one directory.
func mergedPatchCluster(r client.Object) *stack.Cluster {
	obj := r
	hostile := stack.NewApplication("hostile", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	owner := srBundle("owner", cmApp("owner-app"))
	owner.Patches = []stack.Patch{{
		Patch:  "- op: add\n  path: /metadata/labels/z\n  value: y\n",
		Target: &stack.PatchSelector{Kind: "ConfigMap", LabelSelector: "a=b", AnnotationSelector: "c=d"},
	}}
	child := &stack.Node{Name: "child", Bundle: srBundle("other", hostile)}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "root", Bundle: owner, Children: []*stack.Node{child}}}
}

var generatesFromMergedPatchCluster = map[string]func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error){
	"GenerateFromLayout": func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error) {
		ml, err := layout.WalkCluster(c, rules)
		if err != nil {
			return nil, err
		}
		return fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
	},
	"GenerateFromCluster": func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error) {
		return fluxstack.NewResourceGenerator().GenerateFromCluster(c, rules)
	},
}

// TestCallerMethodPanic_PatchScopeOfMergedBundlesRefuses: the patch-scope check
// of a unit of merged bundles reads another bundle's caller object with every
// metadata method; a method that panics is a refusal naming the Go type, in each
// shape the object can come in, and not the panic.
func TestCallerMethodPanic_PatchScopeOfMergedBundlesRefuses(t *testing.T) {
	rules := propertyGroupings["nodeFlat"]
	for entry, run := range generatesFromMergedPatchCluster {
		for shape, build := range callerShapes {
			for _, method := range callerMethods {
				t.Run(entry+"/"+shape+"/"+method, func(t *testing.T) {
					obj := build(callerKinds[0], method)
					before := snapshotOf(t, obj)
					err := callEntry(func() error {
						_, err := run(mergedPatchCluster(obj), rules)
						return err
					})
					if after := snapshotOf(t, obj); after != before {
						t.Errorf("the refusal left the caller's object changed:\nbefore %s\nafter  %s", before, after)
					}
					if err == nil {
						t.Fatalf("%s returned no error for an object whose %s panics", entry, method)
					}
					msg := err.Error()
					if strings.HasPrefix(msg, "PANIC: ") {
						t.Fatalf("%s panicked instead of refusing: %s", entry, msg)
					}
					if strings.Contains(msg, callerMethodPanicText) {
						t.Errorf("the refusal carries the text of the panic: %s", msg)
					}
					if !strings.Contains(msg, "fluxcd_test.panic") {
						t.Errorf("the refusal does not name the Go type of the object: %s", msg)
					}
				})
			}
		}
	}
}

// A refusal of GenerateFromLayout puts back what the normalization before it
// replaced (go-kure/kure#1201), also when the probe refuses. The control shows
// the call does replace the map of an object that holds a plain int.
func TestCallerMethodPanic_GenerateFromLayoutRefusalPutsBackWhatNormalizationReplaced(t *testing.T) {
	rules := propertyGroupings["nodeFlat"]
	generate := func(second client.Object) (*unstructured.Unstructured, error) {
		u := &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1", "kind": "ConfigMap",
			"metadata": map[string]any{"name": "plain", "namespace": "default"},
			"data":     map[string]any{"n": int(1)},
		}}
		c := mergedPatchCluster(second)
		ml, err := layout.WalkCluster(c, rules)
		if err != nil {
			return u, err
		}
		ml.Resources = append(ml.Resources, u)
		err = callEntry(func() error {
			_, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
			return err
		})
		return u, err
	}
	numberType := func(u *unstructured.Unstructured) string {
		return fmt.Sprintf("%T", u.Object["data"].(map[string]any)["n"])
	}

	// The control's object matches no label of the patch, so the call succeeds.
	benign := panicConfigMap(callerKinds[0])
	benign.Labels = nil
	u, err := generate(&panicObject{ConfigMap: benign})
	if err != nil {
		t.Fatalf("control: %v", err)
	}
	if got := numberType(u); got != "int64" {
		t.Fatalf("control: the normalization left a %s, want int64: the case below would prove nothing", got)
	}

	u, err = generate(&panicObject{ConfigMap: panicConfigMap(callerKinds[0]), panicOn: "GetLabels"})
	if err == nil || !strings.Contains(err.Error(), "fluxcd_test.panicObject") || strings.HasPrefix(err.Error(), "PANIC:") {
		t.Fatalf("want the refusal naming fluxcd_test.panicObject, got %v", err)
	}
	if got := numberType(u); got != "int" {
		t.Errorf("the refused call left the unstructured object's number as %s, want the caller's int", got)
	}
}

// An object a bundle's application emitted, which a caller took out of the
// layouts' resources and out of the applications' records, is still in the
// bundle's emitted record (OriginBundleObjects), which the integration reads.
// The probe reads it too.
func TestCallerMethodPanic_ObjectsOnlyInABundlesEmittedRecordAreProbed(t *testing.T) {
	entries := map[string]func(c *stack.Cluster, ml *layout.ManifestLayout) error{
		"IntegrateWithLayout": func(c *stack.Cluster, ml *layout.ManifestLayout) error {
			return fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, nodeOnly)
		},
		"UmbrellaTerminals": func(_ *stack.Cluster, ml *layout.ManifestLayout) error {
			_, err := fluxstack.UmbrellaTerminals(ml)
			return err
		},
	}
	for entry, run := range entries {
		for _, method := range callerMethods {
			t.Run(entry+"/"+method, func(t *testing.T) {
				obj := &panicObject{ConfigMap: panicConfigMap(callerKinds[0]), panicOn: method}
				c, ml, err := walkPanicCluster(obj, stack.DeliveryIntent{})
				if err != nil {
					t.Fatalf("walk: %v", err)
				}
				var drop func(l *layout.ManifestLayout)
				drop = func(l *layout.ManifestLayout) {
					if l == nil {
						return
					}
					l.Resources = nil
					recs := l.OriginApplicationObjects()
					for i := range recs {
						recs[i].Objects = nil
					}
					for _, child := range l.Children {
						drop(child)
					}
					drop(l.OriginUnit())
				}
				drop(ml)
				err = callEntry(func() error { return run(c, ml) })
				if err == nil || strings.HasPrefix(err.Error(), "PANIC:") {
					t.Fatalf("want a refusal, got %v", err)
				}
				if !strings.Contains(err.Error(), "fluxcd_test.panicObject") || strings.Contains(err.Error(), callerMethodPanicText) {
					t.Errorf("the refusal %q must name fluxcd_test.panicObject and not the panic text", err)
				}
			})
		}
	}
}
