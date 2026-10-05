package layout_test

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for which objects the pre-write checks take a resource to hold
// (go-kure/kure#1006): the ones kustomize builds from it, a List opened
// however deep, and an item of a typed List read whether or not it carries
// object metadata.

func configMapNamed(name string) *unstructured.Unstructured {
	return testObj("v1", "ConfigMap", name).(*unstructured.Unstructured)
}

// listWithoutObjectMetadata is a core v1 List, which has list metadata only,
// holding items.
func listWithoutObjectMetadata(items ...runtime.RawExtension) *metav1.List {
	return &metav1.List{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "List"}, Items: items}
}

// TestWriters_TypedListItemsWithoutObjectMetadata: a typed List whose direct
// item is raw JSON, or a List without object metadata, is a List the writers
// serialize, so every writer writes it, with the object it holds.
func TestWriters_TypedListItemsWithoutObjectMetadata(t *testing.T) {
	cases := map[string]func(t *testing.T) client.Object{
		"an object": func(t *testing.T) client.Object {
			return rawListOf("holder", runtime.RawExtension{Object: configMapNamed("held")})
		},
		"raw JSON": func(t *testing.T) client.Object {
			return rawListOf("holder", rawJSON(t, configMapNamed("held")))
		},
		"a List without object metadata": func(t *testing.T) client.Object {
			return rawListOf("holder", runtime.RawExtension{Object: listWithoutObjectMetadata(
				runtime.RawExtension{Object: configMapNamed("held")})})
		},
		"a List without object metadata holding raw JSON": func(t *testing.T) client.Object {
			return rawListOf("holder", runtime.RawExtension{Object: listWithoutObjectMetadata(
				rawJSON(t, configMapNamed("held")))})
		},
	}
	for name, build := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				ml := &layout.ManifestLayout{Name: "p", Namespace: ".", Resources: []client.Object{build(t)}}
				found := 0
				for _, content := range writtenFiles(t, writer, layout.DefaultLayoutConfig(), ml) {
					found += strings.Count(content, "name: held")
				}
				if found != 1 {
					t.Errorf("the ConfigMap the List holds is written %d times, want once", found)
				}
			})
		}
	}
}

// TestWriters_RefuseDuplicateNestedInLists: two objects of one identity in
// what one kustomize build takes in are refused however many Lists deep one
// of them sits, since kustomize opens a List inside a List and then refuses
// the build. The same Lists holding an object of another name are written.
func TestWriters_RefuseDuplicateNestedInLists(t *testing.T) {
	// Each case holds the ConfigMap held somewhere.
	cases := map[string]func(t *testing.T, held *unstructured.Unstructured) client.Object{
		"beside it": func(t *testing.T, held *unstructured.Unstructured) client.Object { return held },
		"in a List": func(t *testing.T, held *unstructured.Unstructured) client.Object { return listOf("List", held) },
		"in a List in a List": func(t *testing.T, held *unstructured.Unstructured) client.Object {
			return listOf("List", listOf("List", held))
		},
		"in a typed List in a typed List": func(t *testing.T, held *unstructured.Unstructured) client.Object {
			return rawListOf("o", runtime.RawExtension{Object: rawListOf("i", runtime.RawExtension{Object: held})})
		},
		"as raw JSON in a typed List in a typed List": func(t *testing.T, held *unstructured.Unstructured) client.Object {
			return rawListOf("o", runtime.RawExtension{Object: rawListOf("i", rawJSON(t, held))})
		},
		"in a List without object metadata in a typed List": func(t *testing.T, held *unstructured.Unstructured) client.Object {
			return rawListOf("o", runtime.RawExtension{Object: listWithoutObjectMetadata(runtime.RawExtension{Object: held})})
		},
		"in an unstructured List in a typed List": func(t *testing.T, held *unstructured.Unstructured) client.Object {
			return rawListOf("o", runtime.RawExtension{Object: listOf("List", held)})
		},
	}
	for name, holding := range cases {
		for _, writer := range allWriters {
			t.Run("one layout/"+name+"/"+writer, func(t *testing.T) {
				ml := &layout.ManifestLayout{Name: "p", Namespace: ".", Resources: []client.Object{
					configMapNamed("twice"), holding(t, configMapNamed("twice")),
				}}
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), ml)
				want := `layout "p" holds the same object /ConfigMap default/twice twice`
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to contain %q", err, want)
				}
			})
			// The parent's kustomization.yaml lists the child's directory,
			// so one build takes in both layouts.
			t.Run("two layouts in one build/"+name+"/"+writer, func(t *testing.T) {
				p := fluxParent()
				p.Resources = append(p.Resources, configMapNamed("twice"))
				p.Children[0].Resources = append(p.Children[0].Resources, holding(t, configMapNamed("twice")))
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), p)
				want := `layouts "p" and "p/c" both hold the object v1 ConfigMap default/twice`
				if err == nil || !strings.Contains(err.Error(), want) {
					t.Errorf("err = %v, want it to contain %q", err, want)
				}
			})
			t.Run("control/"+name+"/"+writer, func(t *testing.T) {
				ml := &layout.ManifestLayout{Name: "p", Namespace: ".", Resources: []client.Object{
					configMapNamed("twice"), holding(t, configMapNamed("once")),
				}}
				writtenFiles(t, writer, layout.DefaultLayoutConfig(), ml)
			})
		}
	}
}
