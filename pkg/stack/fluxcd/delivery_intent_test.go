package fluxcd_test

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for an application's delivery intent under the Flux workflow
// (go-kure/kure#974): the integrator turns it into Flux's per-object
// annotations on that application's objects, and on nothing else.

const (
	pruneKey = "kustomize.toolkit.fluxcd.io/prune"
	forceKey = "kustomize.toolkit.fluxcd.io/force"
)

// deliveryCluster is platform (bundle platform: core) -> web (bundle web).
// Bundle web has two applications carrying intent — web-app, a plain one, and
// web-chart, an augmenter that adds a child layout and a configMapGenerator —
// and one, other, that carries none.
func deliveryCluster(intent stack.DeliveryIntent) *stack.Cluster {
	app := cmApp("web-app")
	app.Delivery = intent
	chart := stack.NewApplication("web-chart", "default", &hookAugmenter{app: "web-chart"})
	chart.Delivery = intent
	web := &stack.Node{Name: "web", Bundle: srBundle("web", app, chart, cmApp("other"))}
	root := &stack.Node{Name: "platform", Bundle: srBundle("platform", cmApp("core")), Children: []*stack.Node{web}}
	web.SetParent(root)
	return &stack.Cluster{Name: "demo", Node: root}
}

// deliveryObjects are the ConfigMaps deliveryCluster's intent applications
// render (the last by its generator, with a content-hash suffix), and
// plainObjects those of the applications without intent.
var (
	deliveryObjects = []string{"web-app-cm", "web-chart-cm", "web-chart-hook", "web-chart-values-"}
	plainObjects    = []string{"core-cm", "other-cm"}
)

func wantAnnotations(intent stack.DeliveryIntent) map[string]string {
	want := map[string]string{}
	if intent.PruneProtection {
		want[pruneKey] = "disabled"
	}
	if intent.ForceReplace {
		want[forceKey] = "enabled"
	}
	return want
}

// appliedObjects builds every directory a Flux Kustomization of ml builds, as
// kustomize-controller does, and returns every object those builds apply.
func appliedObjects(t *testing.T, w writtenTree, ml *layout.ManifestLayout) []unstructured.Unstructured {
	t.Helper()
	var out []unstructured.Unstructured
	for dir, kust := range fluxBuilds(t, w, ml) {
		for _, y := range fluxBuild(t, w.root, dir, kust) {
			var u unstructured.Unstructured
			if err := yaml.Unmarshal([]byte(y), &u.Object); err != nil {
				t.Fatal(err)
			}
			out = append(out, u)
		}
	}
	return out
}

// deliveryAnnotations returns the two delivery annotations o carries.
func deliveryAnnotations(o client.Object) map[string]string {
	got := map[string]string{}
	for _, k := range []string{pruneKey, forceKey} {
		if v, ok := o.GetAnnotations()[k]; ok {
			got[k] = v
		}
	}
	return got
}

// checkApplied fails unless every object named in names (a trailing "-" is a
// prefix: a generated name) is applied at least once, and every time with
// exactly the delivery annotations want.
func checkApplied(t *testing.T, where string, applied []unstructured.Unstructured, names []string, want map[string]string) {
	t.Helper()
	for _, name := range names {
		found := 0
		for i := range applied {
			o := &applied[i]
			if o.GetKind() != "ConfigMap" || (o.GetName() != name && (!strings.HasSuffix(name, "-") || !strings.HasPrefix(o.GetName(), name))) {
				continue
			}
			found++
			if got := deliveryAnnotations(o); !mapsEqual(got, want) {
				t.Errorf("%s: ConfigMap %q is applied with delivery annotations %v, want %v", where, o.GetName(), got, want)
			}
		}
		if found == 0 {
			t.Errorf("%s: no ConfigMap %q is applied", where, name)
		}
	}
}

// TestDeliveryIntent_AnnotatesTheApplicationsObjects: every object of an
// application with intent — emitted, added by its augmenter, or built by its
// configMapGenerator — is applied with the matching annotation, in every
// placement and grouping and from every writer's output; no other object is.
func TestDeliveryIntent_AnnotatesTheApplicationsObjects(t *testing.T) {
	intents := map[string]stack.DeliveryIntent{
		"prune": {PruneProtection: true},
		"force": {ForceReplace: true},
		"both":  {PruneProtection: true, ForceReplace: true},
	}
	for iname, intent := range intents {
		for _, placement := range placements {
			for grouping := range propertyGroupings {
				t.Run(iname+"/"+string(placement)+"/"+grouping, func(t *testing.T) {
					ml := integrated(t, deliveryCluster(intent), recursiveRules(grouping, placement))
					for writer, w := range writeAll(t, ml) {
						applied := appliedObjects(t, w, ml)
						checkApplied(t, writer, applied, deliveryObjects, wantAnnotations(intent))
						checkApplied(t, writer, applied, plainObjects, map[string]string{})
						for i := range applied {
							if o := &applied[i]; o.GetKind() != "ConfigMap" && len(deliveryAnnotations(o)) > 0 {
								t.Errorf("%s: %s %q carries %v", writer, o.GetKind(), o.GetName(), deliveryAnnotations(o))
							}
						}
					}
				})
			}
		}
	}
}

// layoutObjects returns every object in the tree, List items included, and
// every configMapGenerator entry.
func layoutObjects(t *testing.T, ml *layout.ManifestLayout) (objs []*unstructured.Unstructured, gens []layout.ConfigMapGeneratorSpec) {
	t.Helper()
	var visit func(l *layout.ManifestLayout)
	visit = func(l *layout.ManifestLayout) {
		for _, r := range l.Resources {
			u, ok := r.(*unstructured.Unstructured)
			if !ok {
				continue // a generated Flux object
			}
			if u.IsList() {
				_ = u.EachListItem(func(item runtime.Object) error {
					objs = append(objs, item.(*unstructured.Unstructured))
					return nil
				})
				continue
			}
			objs = append(objs, u)
		}
		gens = append(gens, l.ConfigMapGenerators...)
		for _, c := range l.Children {
			visit(c)
		}
	}
	visit(ml)
	return objs, gens
}

// checkUntouched fails when an object in the tree has an annotations field at
// all, or a generator has annotations: the integrator wrote nothing.
func checkUntouched(t *testing.T, ml *layout.ManifestLayout) {
	t.Helper()
	objs, gens := layoutObjects(t, ml)
	if len(objs) == 0 {
		t.Fatal("no object in the tree")
	}
	for _, o := range objs {
		if _, found, _ := unstructured.NestedFieldNoCopy(o.Object, "metadata", "annotations"); found {
			t.Errorf("%s %q has an annotations field: %v", o.GetKind(), o.GetName(), o.GetAnnotations())
		}
	}
	if len(gens) == 0 {
		t.Error("no generator in the tree")
	}
	for _, g := range gens {
		if g.Annotations != nil {
			t.Errorf("generator %q has annotations %v", g.Name, g.Annotations)
		}
	}
}

// TestDeliveryIntent_UnsetChangesNothing: with no intent set the integrator
// touches no object and no generator, and no written kustomization.yaml gains
// generator options — the only two inputs through which an intent reaches the
// written output — so the output is what it was before the intent existed.
func TestDeliveryIntent_UnsetChangesNothing(t *testing.T) {
	for _, placement := range placements {
		for grouping := range propertyGroupings {
			t.Run(string(placement)+"/"+grouping, func(t *testing.T) {
				ml := integrated(t, deliveryCluster(stack.DeliveryIntent{}), recursiveRules(grouping, placement))
				checkUntouched(t, ml)
				for writer, w := range writeAll(t, ml) {
					files := 0
					err := filepath.WalkDir(w.root, func(p string, d os.DirEntry, err error) error {
						if err != nil || d.IsDir() {
							return err
						}
						data, err := os.ReadFile(p)
						if err != nil {
							return err
						}
						files++
						for _, s := range []string{"options:", "annotations:", pruneKey, forceKey} {
							if strings.Contains(string(data), s) {
								t.Errorf("%s: %s contains %q", writer, p, s)
							}
						}
						return nil
					})
					if err != nil {
						t.Fatal(err)
					}
					if files == 0 {
						t.Errorf("%s: no file written", writer)
					}
				}
			})
		}
	}
}

// annotatedCM is a ConfigMap carrying one annotation.
func annotatedCM(name, key, value string) *client.Object {
	o := cmObj(name)
	(*o).SetAnnotations(map[string]string{key: value})
	return o
}

// conflictCluster is deliveryCluster with one more intent application on
// bundle web, after the others: its second object already carries key with
// value.
func conflictCluster(intent stack.DeliveryIntent, key, value string) (*stack.Cluster, *stack.Application) {
	c := deliveryCluster(intent)
	late := stack.NewApplication("late", "default", &fakeAppConfig{objs: []*client.Object{cmObj("late-first"), annotatedCM("late-second", key, value)}})
	late.Delivery = intent
	web := c.Node.Children[0].Bundle
	web.Applications = append(web.Applications, late)
	return c, late
}

// TestDeliveryIntent_ConflictingAnnotationRefused: an object that already
// carries the annotation with another value makes the integration fail,
// naming the object and its application; and the refusal leaves no annotation
// the integrator added, on the objects handled before it or on a generator.
func TestDeliveryIntent_ConflictingAnnotationRefused(t *testing.T) {
	cases := map[string]struct {
		intent     stack.DeliveryIntent
		key, value string
	}{
		"prune": {stack.DeliveryIntent{PruneProtection: true}, pruneKey, "enabled"},
		"force": {stack.DeliveryIntent{ForceReplace: true}, forceKey, "disabled"},
		"both":  {stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}, forceKey, "disabled"},
	}
	for name, tc := range cases {
		for _, placement := range placements {
			for grouping := range propertyGroupings {
				t.Run(name+"/"+string(placement)+"/"+grouping, func(t *testing.T) {
					c, _ := conflictCluster(tc.intent, tc.key, tc.value)
					rules := recursiveRules(grouping, placement)
					ml, err := layout.WalkCluster(c, rules)
					if err != nil {
						t.Fatal(err)
					}
					err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
					if err == nil {
						t.Fatal("the integration was not refused")
					}
					for _, want := range []string{`application "late"`, `ConfigMap "default/late-second"`, tc.key, `"` + tc.value + `"`} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("error %q does not contain %q", err, want)
						}
					}
					checkRestored(t, ml, "late-second", map[string]string{tc.key: tc.value})
				})
			}
		}
	}
}

// checkRestored fails unless the tree holds no Flux object, the object named
// keep still has exactly the annotations it came with, and nothing else has an
// annotations field.
func checkRestored(t *testing.T, ml *layout.ManifestLayout, keep string, keepAnnotations map[string]string) {
	t.Helper()
	if n := len(kustomizations(ml)); n != 0 {
		t.Errorf("the refused integration left %d Kustomizations", n)
	}
	objs, gens := layoutObjects(t, ml)
	kept := false
	for _, o := range objs {
		if o.GetName() == keep {
			kept = true
			if got := o.GetAnnotations(); !mapsEqual(got, keepAnnotations) {
				t.Errorf("%q has annotations %v after the refusal, want its own %v", keep, got, keepAnnotations)
			}
			continue
		}
		if _, found, _ := unstructured.NestedFieldNoCopy(o.Object, "metadata", "annotations"); found {
			t.Errorf("%s %q has an annotations field after the refusal: %v", o.GetKind(), o.GetName(), o.GetAnnotations())
		}
	}
	if !kept {
		t.Errorf("no object %q in the tree", keep)
	}
	if len(gens) == 0 {
		t.Error("no generator in the tree")
	}
	for _, g := range gens {
		if g.Annotations != nil {
			t.Errorf("generator %q has annotations %v after the refusal", g.Name, g.Annotations)
		}
	}
}

// TestDeliveryIntent_LaterRefusalRestoresObjects: a refusal that has nothing
// to do with the intent — here a Kustomization identity present twice in the
// tree — also leaves no annotation behind.
func TestDeliveryIntent_LaterRefusalRestoresObjects(t *testing.T) {
	intent := stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}
	for _, placement := range placements {
		for grouping := range propertyGroupings {
			t.Run(string(placement)+"/"+grouping, func(t *testing.T) {
				c := deliveryCluster(intent)
				// An object that already carries the wanted value: the
				// integrator did not add it, so it stays.
				own := stack.NewApplication("own", "default", &fakeAppConfig{objs: []*client.Object{annotatedCM("own-cm", pruneKey, "disabled")}})
				own.Delivery = stack.DeliveryIntent{PruneProtection: true}
				c.Node.Bundle.Applications = append(c.Node.Bundle.Applications, own)
				dup := stack.NewApplication("dup", "default", &fakeAppConfig{objs: func() []*client.Object {
					a, b := fluxKustomization("twice", "a"), fluxKustomization("twice", "b")
					return []*client.Object{&a, &b}
				}()})
				c.Node.Bundle.Applications = append(c.Node.Bundle.Applications, dup)

				rules := recursiveRules(grouping, placement)
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatal(err)
				}
				err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
				if err == nil || !strings.Contains(err.Error(), `"twice" is present twice`) {
					t.Fatalf("got %v, want the duplicate Kustomization refused", err)
				}
				objs, gens := layoutObjects(t, ml)
				kept := false
				for _, o := range objs {
					_, found, _ := unstructured.NestedFieldNoCopy(o.Object, "metadata", "annotations")
					switch {
					case o.GetName() == "own-cm":
						kept = true
						if got := o.GetAnnotations(); !mapsEqual(got, map[string]string{pruneKey: "disabled"}) {
							t.Errorf("own-cm has annotations %v after the refusal, want its own", got)
						}
					case found:
						t.Errorf("%s %q has an annotations field after the refusal: %v", o.GetKind(), o.GetName(), o.GetAnnotations())
					}
				}
				if !kept {
					t.Error(`no object "own-cm" in the tree`)
				}
				if len(gens) == 0 {
					t.Error("no generator in the tree")
				}
				for _, g := range gens {
					if g.Annotations != nil {
						t.Errorf("generator %q has annotations %v after the refusal", g.Name, g.Annotations)
					}
				}
			})
		}
	}
}

// TestDeliveryIntent_GeneratorConflictRefused: a generator that already sets
// the annotation to another value is refused like an object, naming it.
func TestDeliveryIntent_GeneratorConflictRefused(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			c := deliveryCluster(stack.DeliveryIntent{PruneProtection: true})
			rules := recursiveRules("GroupByName", placement)
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			chart := layoutAtPath(t, ml, "prod/platform/web/web/web-chart")
			chart.ConfigMapGenerators[0].Annotations = map[string]string{pruneKey: "enabled"}
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			if err == nil {
				t.Fatal("the integration was not refused")
			}
			for _, want := range []string{`application "web-chart"`, `configMapGenerator "web-chart-values"`, `"prod/platform/web/web/web-chart"`, pruneKey} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			if got := chart.ConfigMapGenerators[0].Annotations; !mapsEqual(got, map[string]string{pruneKey: "enabled"}) {
				t.Errorf("the generator's own annotations became %v", got)
			}
			objs, _ := layoutObjects(t, ml)
			if len(objs) == 0 {
				t.Error("no object in the tree")
			}
			for _, o := range objs {
				if _, found, _ := unstructured.NestedFieldNoCopy(o.Object, "metadata", "annotations"); found {
					t.Errorf("%s %q has an annotations field after the refusal", o.GetKind(), o.GetName())
				}
			}
		})
	}
}

// annotatedGenerator is hookAugmenter with annotations of its own on its
// configMapGenerator.
type annotatedGenerator struct {
	hookAugmenter
	annotations map[string]string
}

func (a *annotatedGenerator) AugmentLayout(ml *layout.ManifestLayout) error {
	if err := a.hookAugmenter.AugmentLayout(ml); err != nil {
		return err
	}
	ml.ConfigMapGenerators[0].Annotations = a.annotations
	return nil
}

// childGenerator is hookAugmenter with a configMapGenerator in the child
// layout it appends, too.
type childGenerator struct{ hookAugmenter }

func (c *childGenerator) AugmentLayout(ml *layout.ManifestLayout) error {
	if err := c.hookAugmenter.AugmentLayout(ml); err != nil {
		return err
	}
	hooks := ml.Children[len(ml.Children)-1]
	hooks.ExtraFiles = append(hooks.ExtraFiles, layout.ExtraFile{Name: "child.yaml", Content: []byte("k: v\n")})
	hooks.ConfigMapGenerators = append(hooks.ConfigMapGenerators, layout.ConfigMapGeneratorSpec{Name: c.app + "-child-values", Files: []string{"child.yaml"}})
	return nil
}

// TestDeliveryIntent_PatchScopeSeesGeneratedConfigMaps: in a directory several
// bundles share, another bundle's patch that reaches a ConfigMap a
// configMapGenerator builds is refused as for a written object. That holds
// for a generator in a child layout the augmenter appended, and for a target
// that selects by an annotation: kustomize puts a generator's
// options.annotations on the ConfigMap, whether the delivery intent or the
// generator itself set them.
func TestDeliveryIntent_PatchScopeSeesGeneratedConfigMaps(t *testing.T) {
	prune := stack.DeliveryIntent{PruneProtection: true}
	hook := func(intent stack.DeliveryIntent, config stack.ApplicationConfig) func() *stack.Application {
		return func() *stack.Application {
			app := stack.NewApplication("hook", "default", config)
			app.Delivery = intent
			return app
		}
	}
	plain := func() stack.ApplicationConfig { return &hookAugmenter{app: "hook"} }
	child := func() stack.ApplicationConfig { return &childGenerator{hookAugmenter{app: "hook"}} }
	// Every target names the generated ConfigMap, which keeps it off hook's
	// written objects.
	tests := map[string]struct {
		app     func() *stack.Application
		target  stack.PatchSelector
		reaches string // the ConfigMap the refusal names; "" when accepted
	}{
		"annotation set by the delivery intent": {
			app:     hook(prune, plain()),
			target:  stack.PatchSelector{Name: "hook-values", AnnotationSelector: pruneKey + "=disabled"},
			reaches: "hook-values",
		},
		"no intent, no annotation to select": {
			app:    hook(stack.DeliveryIntent{}, plain()),
			target: stack.PatchSelector{Name: "hook-values", AnnotationSelector: pruneKey + "=disabled"},
		},
		"annotation set by the generator": {
			app: hook(stack.DeliveryIntent{}, &annotatedGenerator{
				hookAugmenter: hookAugmenter{app: "hook"},
				annotations:   map[string]string{"tier": "backend"},
			}),
			target:  stack.PatchSelector{Name: "hook-values", AnnotationSelector: "tier=backend"},
			reaches: "hook-values",
		},
		"child layout's generator, by name": {
			app:     hook(stack.DeliveryIntent{}, child()),
			target:  stack.PatchSelector{Kind: "ConfigMap", Name: "hook-child-values"},
			reaches: "hook-child-values",
		},
		"child layout's generator, by the intent's annotation": {
			app:     hook(prune, child()),
			target:  stack.PatchSelector{Name: "hook-child-values", AnnotationSelector: pruneKey + "=disabled"},
			reaches: "hook-child-values",
		},
		"child layout's generator, no intent, no annotation to select": {
			app:    hook(stack.DeliveryIntent{}, child()),
			target: stack.PatchSelector{Name: "hook-child-values", AnnotationSelector: pruneKey + "=disabled"},
		},
	}
	for name, tt := range tests {
		for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerBundle} {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				c := mergedCluster(func(_, b1, b2 *stack.Bundle) {
					b2.Applications = append(b2.Applications, tt.app())
					target := tt.target
					b1.Patches = []stack.Patch{{Patch: "- op: add", Target: &target}}
				})
				rules := allFlat
				rules.FluxPlacement = placement
				_, err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).CreateLayoutWithResources(c, rules)
				if tt.reaches == "" {
					if err != nil {
						t.Fatalf("CreateLayoutWithResources: %v", err)
					}
					return
				}
				if err == nil {
					t.Fatal("the integration accepted a patch that reaches another bundle's generated ConfigMap")
				}
				for _, want := range []string{`"b1"`, `"b2"`, `ConfigMap "` + tt.reaches + `"`} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not contain %q", err, want)
					}
				}
			})
		}
	}
}

// TestDeliveryIntent_KeepsOtherAnnotations: an object's own annotations stay
// beside the ones the intent adds, and a value that is already the wanted one
// is accepted.
func TestDeliveryIntent_KeepsOtherAnnotations(t *testing.T) {
	app := stack.NewApplication("app", "default", &fakeAppConfig{objs: []*client.Object{
		annotatedCM("noted", "example.com/note", "x"),
		annotatedCM("already", pruneKey, "disabled"),
	}})
	app.Delivery = stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
	ml := integrated(t, c, recursiveRules("nodeOnly", layout.FluxSeparate))
	objs, _ := layoutObjects(t, ml)
	want := map[string]map[string]string{
		"noted":   {"example.com/note": "x", pruneKey: "disabled", forceKey: "enabled"},
		"already": {pruneKey: "disabled", forceKey: "enabled"},
	}
	found := map[string]bool{}
	for _, o := range objs {
		w, ok := want[o.GetName()]
		if !ok {
			continue
		}
		found[o.GetName()] = true
		if !mapsEqual(o.GetAnnotations(), w) {
			t.Errorf("%q has annotations %v, want %v", o.GetName(), o.GetAnnotations(), w)
		}
	}
	for name := range want {
		if !found[name] {
			t.Errorf("%q is not in the layout", name)
		}
	}
}

// TestDeliveryIntent_ListItems: an application that emits a List has its
// items annotated, a List inside the List included; kustomize builds the
// items, not the envelopes.
func TestDeliveryIntent_ListItems(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			a, _ := (*cmObj("item-a")).(*unstructured.Unstructured)
			b, _ := (*cmObj("item-b")).(*unstructured.Unstructured)
			c1, _ := (*cmObj("item-c")).(*unstructured.Unstructured)
			var list client.Object = wrapInList(a, b, wrapInList(c1))
			app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&list}})
			app.Delivery = stack.DeliveryIntent{PruneProtection: true}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app, cmApp("other"))}}
			ml := integrated(t, c, recursiveRules("nodeOnly", placement))
			for writer, w := range writeAll(t, ml) {
				applied := appliedObjects(t, w, ml)
				checkApplied(t, writer, applied, []string{"item-a", "item-b", "item-c"}, map[string]string{pruneKey: "disabled"})
				checkApplied(t, writer, applied, []string{"other-cm"}, map[string]string{})
				for i := range applied {
					if o := &applied[i]; strings.HasSuffix(o.GetKind(), "List") {
						t.Errorf("%s: a %s envelope is applied", writer, o.GetKind())
					}
				}
			}
		})
	}
}

// TestDeliveryIntent_NestedListConflictRefused: a conflicting value on an
// object inside a List inside a List is found like any other.
func TestDeliveryIntent_NestedListConflictRefused(t *testing.T) {
	deep, _ := (*annotatedCM("deep", pruneKey, "enabled")).(*unstructured.Unstructured)
	var list client.Object = wrapInList(wrapInList(deep))
	app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&list}})
	app.Delivery = stack.DeliveryIntent{PruneProtection: true}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
	rules := recursiveRules("nodeOnly", layout.FluxSeparate)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil {
		t.Fatal("a conflicting annotation inside a nested List was accepted")
	}
	for _, want := range []string{`application "listed"`, `ConfigMap "default/deep"`, pruneKey} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

// TestDeliveryIntent_ResourceWithItemsField: only a kind ending in "List" is
// an envelope, as for kustomize. A resource of another kind that has a
// top-level items array is one object and carries the annotation itself; what
// its items hold is its own data and stays as it is.
func TestDeliveryIntent_ResourceWithItemsField(t *testing.T) {
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			inv := &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "Inventory",
				"metadata":   map[string]any{"name": "inventory", "namespace": "default"},
				"items":      []any{map[string]any{"sku": "a"}},
			}}
			var obj client.Object = inv
			app := stack.NewApplication("stock", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
			app.Delivery = stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
			ml := integrated(t, c, recursiveRules("nodeOnly", placement))
			for writer, w := range writeAll(t, ml) {
				found := 0
				for _, o := range appliedObjects(t, w, ml) {
					if o.GetKind() != "Inventory" {
						continue
					}
					found++
					want := map[string]string{pruneKey: "disabled", forceKey: "enabled"}
					if got := deliveryAnnotations(&o); !mapsEqual(got, want) {
						t.Errorf("%s: Inventory is applied with delivery annotations %v, want %v", writer, got, want)
					}
					items, _, _ := unstructured.NestedSlice(o.Object, "items")
					if len(items) != 1 || !reflect.DeepEqual(items[0], map[string]any{"sku": "a"}) {
						t.Errorf("%s: Inventory items = %v, want them unchanged", writer, items)
					}
				}
				if found != 1 {
					t.Errorf("%s: Inventory is applied %d times, want once", writer, found)
				}
			}
		})
	}
}

// typedConfigMapItems is a typed object with an Items field: a list type for
// apimachinery, whatever its kind.
type typedConfigMapItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Items             []corev1.ConfigMap `json:"items"`
}

func (l *typedConfigMapItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

// typedOptionalItems is typedConfigMapItems with an items field that is left out of
// the written file when it is empty.
type typedOptionalItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Items             []corev1.ConfigMap `json:"items,omitempty"`
}

func (l *typedOptionalItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	return &c
}

// typedHiddenItems writes an items array from a field apimachinery does not
// take for a list's items.
type typedHiddenItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Entries           []corev1.ConfigMap `json:"items"`
}

func (l *typedHiddenItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Entries = slices.Clone(l.Entries)
	return &c
}

// typedSplitItems has the shape of a list for apimachinery, but writes its
// items from another field.
type typedSplitItems struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata"`
	Items             []corev1.ConfigMap `json:"-"`
	Entries           []corev1.ConfigMap `json:"items"`
}

func (l *typedSplitItems) DeepCopyObject() runtime.Object {
	c := *l
	c.Items = slices.Clone(l.Items)
	c.Entries = slices.Clone(l.Entries)
	return &c
}

// TestDeliveryIntent_WrittenWithoutAnnotationRefused: what counts is the
// written file. A typed object whose written form still holds an object
// without the annotation, after the integrator set it on everything it could
// reach, is refused, and what was set is taken back.
func TestDeliveryIntent_WrittenWithoutAnnotationRefused(t *testing.T) {
	cm := func(name string) corev1.ConfigMap {
		return corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		}
	}
	list := &typedSplitItems{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
		Items:    []corev1.ConfigMap{cm("reached")},
		Entries:  []corev1.ConfigMap{cm("written")},
	}
	var obj client.Object = list
	app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	app.Delivery = stack.DeliveryIntent{PruneProtection: true}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
	rules := recursiveRules("nodeOnly", layout.FluxSeparate)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil {
		t.Fatal("a List written with an item that lacks the annotation was accepted")
	}
	for _, want := range []string{`application "listed"`, "ConfigMapList", `ConfigMap "default/written"`, stack.AnnotationFluxPruneKey} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
	if got := list.Items[0].GetAnnotations(); len(got) != 0 {
		t.Errorf("the refusal left annotations %v on the object it had reached", got)
	}
}

// TestDeliveryIntent_UnreachableListItemsRefused: a typed object that is
// written as a List, but whose items the integrator cannot reach to annotate
// them, is refused rather than built without the annotation.
func TestDeliveryIntent_UnreachableListItemsRefused(t *testing.T) {
	var obj client.Object = &typedHiddenItems{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
		Entries: []corev1.ConfigMap{{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: "hidden", Namespace: "default"},
		}},
	}
	app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	app.Delivery = stack.DeliveryIntent{PruneProtection: true}
	c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
	rules := recursiveRules("nodeOnly", layout.FluxSeparate)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil {
		t.Fatal("a List whose items cannot be annotated was accepted")
	}
	for _, want := range []string{`application "listed"`, "ConfigMapList", `ConfigMap "default/hidden"`, "cannot reach"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

// TestDeliveryIntent_ListIsDecidedByKind: what is a List is what kustomize
// takes for one when it builds the written file — a kind ending in "List"
// that has an items field — for typed and unstructured objects alike. A kind
// ending in "List" without items, and another kind with items, are each one
// object and carry the annotation themselves.
func TestDeliveryIntent_ListIsDecidedByKind(t *testing.T) {
	want := map[string]string{pruneKey: "disabled"}
	typedCM := func(name string) corev1.ConfigMap {
		return corev1.ConfigMap{
			TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		}
	}
	cases := map[string]struct {
		obj client.Object
		// self is the kind applied as one object, carrying the annotation,
		// and held the number of entries its own items array still has;
		// items are the ConfigMaps applied from inside it, carrying it.
		self  string
		held  int
		items []string
	}{
		"unstructured kind ending in List without items": {
			obj: &unstructured.Unstructured{Object: map[string]any{
				"apiVersion": "example.com/v1",
				"kind":       "ShoppingList",
				"metadata":   map[string]any{"name": "shopping", "namespace": "default"},
				"spec":       map[string]any{"buy": "milk"},
			}},
			self: "ShoppingList",
		},
		"typed kind ending in List whose items are left out": {
			obj: &typedOptionalItems{
				TypeMeta:   metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "ShoppingList"},
				ObjectMeta: metav1.ObjectMeta{Name: "shopping", Namespace: "default"},
			},
			self: "ShoppingList",
		},
		"typed list": {
			obj: &typedConfigMapItems{
				TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
				Items:    []corev1.ConfigMap{typedCM("l1"), typedCM("l2")},
			},
			items: []string{"l1", "l2"},
		},
		"typed object of another kind with items": {
			obj: &typedConfigMapItems{
				TypeMeta:   metav1.TypeMeta{APIVersion: "example.com/v1", Kind: "Inventory"},
				ObjectMeta: metav1.ObjectMeta{Name: "inventory", Namespace: "default"},
				Items:      []corev1.ConfigMap{typedCM("held")},
			},
			self: "Inventory",
			held: 1,
		},
	}
	for name, tc := range cases {
		for _, placement := range placements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				obj := tc.obj.DeepCopyObject().(client.Object)
				app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
				app.Delivery = stack.DeliveryIntent{PruneProtection: true}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
				ml := integrated(t, c, recursiveRules("nodeOnly", placement))
				for writer, w := range writeAll(t, ml) {
					applied := appliedObjects(t, w, ml)
					checkApplied(t, writer, applied, tc.items, want)
					found := 0
					for i := range applied {
						o := &applied[i]
						if o.GetKind() != tc.self {
							if tc.self != "" && o.GetKind() == "ConfigMap" {
								t.Errorf("%s: ConfigMap %q is applied on its own; it is the %s's data", writer, o.GetName(), tc.self)
							}
							continue
						}
						found++
						if got := deliveryAnnotations(o); !mapsEqual(got, want) {
							t.Errorf("%s: %s is applied with delivery annotations %v, want %v", writer, tc.self, got, want)
						}
						held, _, _ := unstructured.NestedSlice(o.Object, "items")
						if len(held) != tc.held {
							t.Errorf("%s: the %s holds %d items, want %d", writer, tc.self, len(held), tc.held)
						}
						for _, h := range held {
							if m, _ := h.(map[string]any); m != nil {
								if ann, ok, _ := unstructured.NestedMap(m, "metadata", "annotations"); ok {
									t.Errorf("%s: the %s's own data carries annotations %v", writer, tc.self, ann)
								}
							}
						}
					}
					if tc.self != "" && found != 1 {
						t.Errorf("%s: %s is applied %d times, want once", writer, tc.self, found)
					}
				}
			})
		}
	}
}

// TestDeliveryIntent_TypedObjects: typed objects are annotated like
// unstructured ones, and a typed object without a kind is named by its Go
// type in a refusal.
func TestDeliveryIntent_TypedObjects(t *testing.T) {
	cluster := func(objs ...client.Object) *stack.Cluster {
		var ptrs []*client.Object
		for i := range objs {
			ptrs = append(ptrs, &objs[i])
		}
		app := stack.NewApplication("typed", "default", &fakeAppConfig{objs: ptrs})
		app.Delivery = stack.DeliveryIntent{PruneProtection: true}
		return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
	}
	rules := recursiveRules("nodeOnly", layout.FluxSeparate)

	plain := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "a", Namespace: "default"}}
	noted := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "b", Namespace: "default", Annotations: map[string]string{"example.com/note": "x"}}}
	integrated(t, cluster(plain, noted), rules)
	for name, got := range map[string]map[string]string{"a": plain.Annotations, "b": noted.Annotations} {
		if got[pruneKey] != "disabled" {
			t.Errorf("typed ConfigMap %q has annotations %v, want the prune annotation", name, got)
		}
	}
	if noted.Annotations["example.com/note"] != "x" {
		t.Errorf("b lost its own annotation: %v", noted.Annotations)
	}

	clash := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "d", Namespace: "default", Annotations: map[string]string{pruneKey: "enabled"}}}
	first := &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "e", Namespace: "default"}}
	c := cluster(first, clash)
	ml, err := layout.WalkCluster(c, rules)
	if err != nil {
		t.Fatal(err)
	}
	err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
	if err == nil || !strings.Contains(err.Error(), `*v1.ConfigMap "default/d"`) {
		t.Fatalf("got %v, want the typed ConfigMap named", err)
	}
	if first.Annotations != nil {
		t.Errorf("the refusal left annotations %v on the object handled before it", first.Annotations)
	}
}

// TestDeliveryIntent_IntegratingAgainChangesNothing: a second integration of
// the same tree finds the annotations it set and adds nothing.
func TestDeliveryIntent_IntegratingAgainChangesNothing(t *testing.T) {
	intent := stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			rules := recursiveRules("GroupByName", placement)
			checkIdempotent(t, deliveryCluster(intent), rules)

			c := deliveryCluster(intent)
			ml := integrated(t, c, rules)
			first := writtenFiles(t, ml)
			if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
				t.Fatalf("second IntegrateWithLayout: %v", err)
			}
			if second := writtenFiles(t, ml); !mapsEqual(first, second) {
				t.Errorf("the second integration changed the written tree")
			}
		})
	}
}

// TestDeliveryIntent_RawItemReadAsWritten: an item a typed List holds as a
// runtime.RawExtension is written from its raw JSON when it has any, so that
// is the object the intent is checked on. An object held beside the raw JSON
// is not written: it refuses nothing, whatever it carries, and stays as it is.
func TestDeliveryIntent_RawItemReadAsWritten(t *testing.T) {
	raw := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"raw-cm","namespace":"default","annotations":{"` + pruneKey + `":"disabled"}}}`)
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			unwritten := *annotatedCM("raw-cm", pruneKey, "enabled")
			var obj client.Object = rawListOf(runtime.RawExtension{Raw: raw, Object: unwritten})
			app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
			app.Delivery = stack.DeliveryIntent{PruneProtection: true}
			c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", app)}}
			ml := integrated(t, c, recursiveRules("nodeOnly", placement))
			for writer, w := range writeAll(t, ml) {
				checkApplied(t, writer, appliedObjects(t, w, ml), []string{"raw-cm"}, map[string]string{pruneKey: "disabled"})
			}
			if got := unwritten.GetAnnotations(); !mapsEqual(got, map[string]string{pruneKey: "enabled"}) {
				t.Errorf("the object beside the raw JSON has annotations %v, want it unchanged", got)
			}
		})
	}
}

// TestDeliveryIntent_SourceAlsoDerivedFromSourceRef: an application may emit
// the Source a bundle's SourceRef derives; the integrated placements take the
// two for one object. They still do when the application carries an intent:
// the derived copy carries the same annotations, so every copy that is applied
// is the same object, and integrating again changes nothing.
func TestDeliveryIntent_SourceAlsoDerivedFromSourceRef(t *testing.T) {
	webRef := &stack.SourceRef{Kind: "GitRepository", Name: "web-git", Namespace: "flux-system", URL: "https://example.com/web.git", Branch: "main"}
	want := map[string]string{pruneKey: "disabled", forceKey: "enabled"}
	emitted := func(t *testing.T, form string) client.Object {
		t.Helper()
		gen, err := fluxstack.NewResourceGenerator().GenerateForBundle(&stack.Bundle{Name: "web", SourceRef: webRef}, "x")
		if err != nil {
			t.Fatal(err)
		}
		if len(gen) != 2 {
			t.Fatalf("got %d generated objects, want the Kustomization and its GitRepository", len(gen))
		}
		if form == "typed" {
			return gen[1]
		}
		content, err := runtime.DefaultUnstructuredConverter.ToUnstructured(gen[1])
		if err != nil {
			t.Fatal(err)
		}
		u := &unstructured.Unstructured{Object: content}
		u.SetGroupVersionKind(gen[1].GetObjectKind().GroupVersionKind())
		if form == "in a List" {
			return wrapInList(u)
		}
		// Raw JSON is written as it is, so it carries the annotations
		// already. The object beside it is not written; the writers' own
		// duplicate check reads it.
		u.SetAnnotations(want)
		raw, err := u.MarshalJSON()
		if err != nil {
			t.Fatal(err)
		}
		return rawListOf(runtime.RawExtension{Raw: raw, Object: u})
	}
	for _, form := range []string{"typed", "in a List", "raw JSON in a typed List"} {
		for _, placement := range []layout.FluxPlacement{layout.FluxIntegratedPerLayout, layout.FluxIntegratedPerBundle} {
			t.Run(form+"/"+string(placement), func(t *testing.T) {
				obj := emitted(t, form)
				sources := stack.NewApplication("sources", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
				sources.Delivery = stack.DeliveryIntent{PruneProtection: true, ForceReplace: true}
				web := &stack.Node{Name: "web", Bundle: &stack.Bundle{Name: "web", SourceRef: webRef, Applications: []*stack.Application{cmApp("web-app")}}}
				c := &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", sources), Children: []*stack.Node{web}}}
				rules := propertyGroupings["nodeOnly"]
				rules.FluxPlacement = placement
				ml := integrated(t, c, rules)

				// Every copy of the Source that is applied, the application's
				// and any the integration hosts, carries the intent.
				for writer, w := range writeAll(t, ml) {
					found := 0
					for _, o := range appliedObjects(t, w, ml) {
						if o.GetKind() != "GitRepository" {
							continue
						}
						found++
						if got := deliveryAnnotations(&o); !mapsEqual(got, want) {
							t.Errorf("%s: GitRepository %q is applied with delivery annotations %v, want %v", writer, o.GetName(), got, want)
						}
					}
					if found == 0 {
						t.Errorf("%s: no GitRepository is applied", writer)
					}
				}

				first := writtenFiles(t, ml)
				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
					t.Fatalf("second IntegrateWithLayout: %v", err)
				}
				if second := writtenFiles(t, ml); !mapsEqual(first, second) {
					t.Errorf("the second integration changed the written tree")
				}
			})
		}
	}
}

// TestDeliveryIntent_GenerateFromClusterRefused: GenerateFromCluster returns
// Kustomizations and Sources and none of an application's objects, so it has
// nowhere to set a delivery intent. The generator and the engine refuse a set
// one under the three grouping rule sets and both placements the call accepts,
// naming the application and CreateLayoutWithResources; without an intent the
// same cluster generates. Under FluxIntegratedPerLayout the placement refusal
// comes first. GenerateFromLayout does not refuse an intent: its caller holds
// the layout.
func TestDeliveryIntent_GenerateFromClusterRefused(t *testing.T) {
	intents := map[string]stack.DeliveryIntent{
		"prune": {PruneProtection: true},
		"force": {ForceReplace: true},
		"both":  {PruneProtection: true, ForceReplace: true},
	}
	calls := map[string]func(*stack.Cluster, layout.LayoutRules) ([]client.Object, error){
		"generator": func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error) {
			return fluxstack.NewResourceGenerator().GenerateFromCluster(c, rules)
		},
		"engine": func(c *stack.Cluster, rules layout.LayoutRules) ([]client.Object, error) {
			return fluxstack.NewWorkflowEngine().GenerateFromCluster(c, rules)
		},
	}
	for _, placement := range []layout.FluxPlacement{layout.FluxSeparate, layout.FluxIntegratedPerBundle} {
		for grouping := range propertyGroupings {
			rules := recursiveRules(grouping, placement)
			for cname, call := range calls {
				where := cname + "/" + string(placement) + "/" + grouping
				for iname, intent := range intents {
					objs, err := call(deliveryCluster(intent), rules)
					if err == nil {
						t.Errorf("%s/%s: accepted a delivery intent", where, iname)
						continue
					}
					for _, want := range []string{`application "web-app"`, "delivery intent", "CreateLayoutWithResources"} {
						if !strings.Contains(err.Error(), want) {
							t.Errorf("%s/%s: error %q does not contain %q", where, iname, err, want)
						}
					}
					if objs != nil {
						t.Errorf("%s/%s: got %d objects with the refusal", where, iname, len(objs))
					}
				}
				objs, err := call(deliveryCluster(stack.DeliveryIntent{}), rules)
				if err != nil {
					t.Errorf("%s: without an intent: %v", where, err)
				} else if len(objs) == 0 {
					t.Errorf("%s: without an intent: no object generated", where)
				}
			}
		}
	}

	prune := stack.DeliveryIntent{PruneProtection: true}
	for grouping := range propertyGroupings {
		rules := recursiveRules(grouping, layout.FluxIntegratedPerLayout)
		_, err := fluxstack.NewResourceGenerator().GenerateFromCluster(deliveryCluster(prune), rules)
		if err == nil || !strings.Contains(err.Error(), "FluxPlacement") || strings.Contains(err.Error(), "delivery intent") {
			t.Errorf("per-layout/%s: got %v, want the placement refusal, which comes first", grouping, err)
		}

		separate := recursiveRules(grouping, layout.FluxSeparate)
		c := deliveryCluster(prune)
		ml, err := layout.WalkCluster(c, separate)
		if err != nil {
			t.Fatal(err)
		}
		objs, err := fluxstack.NewResourceGenerator().GenerateFromLayout(ml, c)
		if err != nil {
			t.Errorf("GenerateFromLayout/%s: %v", grouping, err)
		} else if len(objs) == 0 {
			t.Errorf("GenerateFromLayout/%s: no object generated", grouping)
		}
	}
}

// writtenFiles writes ml to disk and returns every file's content by path.
func writtenFiles(t *testing.T, ml *layout.ManifestLayout) map[string]string {
	t.Helper()
	root := t.TempDir()
	if err := ml.WriteToDisk(root); err != nil {
		t.Fatalf("WriteToDisk: %v", err)
	}
	out := map[string]string{}
	err := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out[rel] = string(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}
