package fluxcd_test

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for a delivery intent on an object held in a List that carries no
// object metadata (go-kure/kure#1006): the core v1 List has list metadata
// only, and kustomize opens it like any other List.

// listWithoutObjectMetadata is a core v1 List holding items.
func listWithoutObjectMetadata(items ...runtime.RawExtension) *metav1.List {
	return &metav1.List{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"}, Items: items}
}

// heldIn are the ways a typed List can hold the ConfigMap held as a Go value.
var heldIn = map[string]func(held *corev1.ConfigMap) client.Object{
	"the List itself": func(held *corev1.ConfigMap) client.Object {
		return rawListOf(runtime.RawExtension{Object: held})
	},
	"a List without object metadata": func(held *corev1.ConfigMap) client.Object {
		return rawListOf(runtime.RawExtension{Object: listWithoutObjectMetadata(runtime.RawExtension{Object: held})})
	},
	"a List without object metadata in another": func(held *corev1.ConfigMap) client.Object {
		return rawListOf(runtime.RawExtension{Object: listWithoutObjectMetadata(
			runtime.RawExtension{Object: listWithoutObjectMetadata(runtime.RawExtension{Object: held})})})
	},
}

func heldConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta:   metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"},
		ObjectMeta: metav1.ObjectMeta{Name: "held", Namespace: "default"},
	}
}

// listedCluster is one node whose bundle has the application listed, with a
// prune-protection intent and obj as its one object, and the applications
// more beside it.
func listedCluster(obj client.Object, more ...*stack.Application) *stack.Cluster {
	app := stack.NewApplication("listed", "default", &fakeAppConfig{objs: []*client.Object{&obj}})
	app.Delivery = stack.DeliveryIntent{PruneProtection: true}
	return &stack.Cluster{Name: "demo", Node: &stack.Node{Name: "platform", Bundle: srBundle("platform", append([]*stack.Application{app}, more...)...)}}
}

// TestDeliveryIntent_ObjectInListWithoutObjectMetadata: the intent reaches an
// object held in a List that has no object metadata, sets the annotation on
// the application's own Go value, and every writer's output applies the
// object with it.
func TestDeliveryIntent_ObjectInListWithoutObjectMetadata(t *testing.T) {
	want := map[string]string{pruneKey: "disabled"}
	for name, build := range heldIn {
		for _, placement := range placements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				held := heldConfigMap()
				c := listedCluster(build(held), cmApp("other"))
				ml := integrated(t, c, recursiveRules("nodeOnly", placement))
				if got := held.GetAnnotations(); !mapsEqual(got, want) {
					t.Errorf("the object inside the List has annotations %v, want %v", got, want)
				}
				for writer, w := range writeAll(t, ml) {
					applied := appliedObjects(t, w, ml)
					checkApplied(t, writer, applied, []string{"held"}, want)
					checkApplied(t, writer, applied, []string{"other-cm"}, map[string]string{})
				}
			})
		}
	}
}

// TestDeliveryIntent_ObjectInListWithoutObjectMetadataRestored: a refusal
// that comes after the annotation was set, here a Kustomization identity
// present twice in the tree, takes it back from an object held that way too.
func TestDeliveryIntent_ObjectInListWithoutObjectMetadataRestored(t *testing.T) {
	for name, build := range heldIn {
		for _, placement := range placements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				held := heldConfigMap()
				a, b := fluxKustomization("twice", "a"), fluxKustomization("twice", "b")
				dup := stack.NewApplication("dup", "default", &fakeAppConfig{objs: []*client.Object{&a, &b}})
				c := listedCluster(build(held), dup)
				rules := recursiveRules("nodeOnly", placement)
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatal(err)
				}
				err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
				if err == nil || !strings.Contains(err.Error(), `"twice" is present twice`) {
					t.Fatalf("got %v, want the duplicate Kustomization refused", err)
				}
				if held.Annotations != nil {
					t.Errorf("the refusal left annotations %v on the object inside the List", held.Annotations)
				}
			})
		}
	}
}

// TestDeliveryIntent_ItemsNotWrittenAreNotAnnotated: what is written decides
// what a typed List holds. One that writes its items as null, or as an empty
// array, holds nothing for kustomize, so an object its Go value has beside
// that is not an object of the application: the intent leaves it alone.
func TestDeliveryIntent_ItemsNotWrittenAreNotAnnotated(t *testing.T) {
	written := map[string][]corev1.ConfigMap{
		"items written as null":           nil,
		"items written as an empty array": {},
	}
	for name, entries := range written {
		for _, placement := range placements {
			t.Run(name+"/"+string(placement), func(t *testing.T) {
				list := &typedSplitItems{
					TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMapList"},
					Items:    []corev1.ConfigMap{*heldConfigMap()},
					Entries:  entries,
				}
				c := listedCluster(list, cmApp("other"))
				rules := recursiveRules("nodeOnly", placement)
				ml, err := layout.WalkCluster(c, rules)
				if err != nil {
					t.Fatal(err)
				}
				if err := fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules); err != nil {
					t.Fatalf("IntegrateWithLayout: %v", err)
				}
				if got := list.Items[0].GetAnnotations(); len(got) != 0 {
					t.Errorf("an object that is not written has annotations %v", got)
				}
			})
		}
	}
}

// TestDeliveryIntent_RawItemInListWithoutObjectMetadataRefused: an item held
// as raw JSON is written from that JSON and cannot be annotated, in a List
// without object metadata as in any other. The application is refused, and
// the object reached beside it is left as it was.
func TestDeliveryIntent_RawItemInListWithoutObjectMetadataRefused(t *testing.T) {
	raw := []byte(`{"apiVersion":"v1","kind":"ConfigMap","metadata":{"name":"raw-cm","namespace":"default"}}`)
	for _, placement := range placements {
		t.Run(string(placement), func(t *testing.T) {
			held := heldConfigMap()
			c := listedCluster(rawListOf(runtime.RawExtension{Object: listWithoutObjectMetadata(
				runtime.RawExtension{Object: held}, runtime.RawExtension{Raw: raw})}))
			rules := recursiveRules("nodeOnly", placement)
			ml, err := layout.WalkCluster(c, rules)
			if err != nil {
				t.Fatal(err)
			}
			err = fluxstack.NewLayoutIntegrator(fluxstack.NewResourceGenerator()).IntegrateWithLayout(ml, c, rules)
			if err == nil {
				t.Fatal("a List written with a raw JSON item that lacks the annotation was accepted")
			}
			for _, want := range []string{`application "listed"`, `ConfigMap "default/raw-cm"`, pruneKey, "cannot reach"} {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not contain %q", err, want)
				}
			}
			if held.Annotations != nil {
				t.Errorf("the refusal left annotations %v on the object it had reached", held.Annotations)
			}
		})
	}
}
