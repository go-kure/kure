package layout_test

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/stack/layout"
)

// Tests for the refusal of an object written without a kind or without an
// apiVersion (go-kure/kure#1020): every object a build reads, a resource or
// a List item at any depth, typed or unstructured, judged on its written form.
// A List's own apiVersion is exempt.

// untypedConfigMap is a typed ConfigMap whose TypeMeta is unset: it is
// written with neither a kind nor an apiVersion.
func untypedConfigMap(name string) *corev1.ConfigMap {
	return &corev1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}}
}

// withoutField is a ConfigMap named name without the top-level field.
func withoutField(name, field string) *unstructured.Unstructured {
	cm := configMapNamed(name)
	delete(cm.Object, field)
	return cm
}

func TestWriters_RefuseObjectWithoutKindOrAPIVersion(t *testing.T) {
	cases := map[string]struct {
		build func(t *testing.T) client.Object
		wants []string
	}{
		"a typed object without TypeMeta": {
			func(t *testing.T) client.Object { return untypedConfigMap("cm") },
			[]string{`an object "default/cm" (Go type *v1.ConfigMap)`, "written without a kind and an apiVersion"},
		},
		"an unstructured object without a kind": {
			func(t *testing.T) client.Object { return withoutField("cm", "kind") },
			[]string{`an object "default/cm" (Go type *unstructured.Unstructured)`, "written without a kind:"},
		},
		"an unstructured object without an apiVersion": {
			func(t *testing.T) client.Object { return withoutField("cm", "apiVersion") },
			[]string{`ConfigMap "default/cm" (Go type *unstructured.Unstructured)`, "written without an apiVersion:"},
		},
		"an item of an unstructured List without an apiVersion": {
			func(t *testing.T) client.Object { return listOf("List", withoutField("cm", "apiVersion")) },
			[]string{`ConfigMap "default/cm"`, `an item of List without a name (Go type *unstructured.Unstructured)`, "written without an apiVersion:"},
		},
		"a typed object without TypeMeta in a typed List": {
			func(t *testing.T) client.Object {
				return rawListOf("holder", runtime.RawExtension{Object: untypedConfigMap("cm")})
			},
			[]string{`an object "default/cm"`, `an item of List "default/holder"`, "written without a kind and an apiVersion"},
		},
		"raw JSON without a kind in a typed List": {
			func(t *testing.T) client.Object {
				return rawListOf("holder", rawJSON(t, withoutField("cm", "kind")))
			},
			[]string{`an object "default/cm"`, `an item of List "default/holder"`, "written without a kind:"},
		},
		"an object without an apiVersion two Lists deep": {
			func(t *testing.T) client.Object {
				return listOf("List", listOf("List", withoutField("cm", "apiVersion")))
			},
			[]string{`ConfigMap "default/cm"`, "an item of List", "written without an apiVersion:"},
		},
	}
	for name, tc := range cases {
		for _, writer := range allWriters {
			t.Run(name+"/"+writer, func(t *testing.T) {
				ml := &layout.ManifestLayout{Name: "p", Namespace: ".", Resources: []client.Object{configMapNamed("fine"), tc.build(t)}}
				err := writeRefused(t, writer, layout.DefaultLayoutConfig(), ml)
				for _, want := range append([]string{`layout "p" holds`, "TypeMeta"}, tc.wants...) {
					if err == nil || !strings.Contains(err.Error(), want) {
						t.Errorf("err = %v, want it to contain %q", err, want)
					}
				}
			})
		}
	}
}

// TestWriters_ListWithoutAPIVersion: a List's own apiVersion is not judged.
// A build drops the envelope without reading it, so a List without one whose
// items are complete is written as before, with its items.
func TestWriters_ListWithoutAPIVersion(t *testing.T) {
	cases := map[string]func(t *testing.T) client.Object{
		"an unstructured List": func(t *testing.T) client.Object {
			l := listOf("List", configMapNamed("held"))
			delete(l.Object, "apiVersion")
			return l
		},
		"a typed List": func(t *testing.T) client.Object {
			l := rawListOf("holder", runtime.RawExtension{Object: configMapNamed("held")})
			l.APIVersion = ""
			return l
		},
		"an unstructured List in a List": func(t *testing.T) client.Object {
			inner := listOf("List", configMapNamed("held"))
			delete(inner.Object, "apiVersion")
			return listOf("List", inner)
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
