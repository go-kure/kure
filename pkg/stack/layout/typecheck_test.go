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

// unwrittenType reports a kind through a TypeMeta it leaves out of what is
// written, and writes the kind and apiVersion fields from two fields of its
// own: what its Go value reports and what is written disagree.
type unwrittenType struct {
	metav1.TypeMeta   `json:"-"`
	WrittenAPIVersion string `json:"apiVersion,omitempty"`
	WrittenKind       string `json:"kind,omitempty"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Items             []corev1.ConfigMap `json:"items"`
}

func (u *unwrittenType) DeepCopyObject() runtime.Object { c := *u; return &c }

// unwrittenTypeOf is an unwrittenType named holder whose Go value reports the
// kind reported, written with the kind written and no apiVersion, holding
// items.
func unwrittenTypeOf(reported, written string, items ...corev1.ConfigMap) *unwrittenType {
	u := &unwrittenType{TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: reported}, WrittenKind: written, Items: items}
	u.Name, u.Namespace = "holder", "default"
	return u
}

// typedConfigMap is a typed ConfigMap with its TypeMeta set.
func typedConfigMap(name string) corev1.ConfigMap {
	cm := untypedConfigMap(name)
	cm.TypeMeta = metav1.TypeMeta{APIVersion: "v1", Kind: "ConfigMap"}
	return *cm
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
			[]string{`ConfigMap "default/cm"`, `held in List without a name (Go type *unstructured.Unstructured)`, "written without an apiVersion:"},
		},
		"a typed object without TypeMeta in a typed List": {
			func(t *testing.T) client.Object {
				return rawListOf("holder", runtime.RawExtension{Object: untypedConfigMap("cm")})
			},
			[]string{`an object "default/cm"`, `held in List "default/holder"`, "written without a kind and an apiVersion"},
		},
		"raw JSON without a kind in a typed List": {
			func(t *testing.T) client.Object {
				return rawListOf("holder", rawJSON(t, withoutField("cm", "kind")))
			},
			[]string{`an object "default/cm"`, `held in List "default/holder"`, "written without a kind:"},
		},
		"an object without an apiVersion two Lists deep": {
			// Named by the resource that holds it, not by the List between.
			func(t *testing.T) client.Object {
				outer := listOf("List", listOf("List", withoutField("cm", "apiVersion")))
				outer.SetName("outer")
				return outer
			},
			[]string{`ConfigMap "default/cm"`, `held in List "outer" (Go type *unstructured.Unstructured)`, "written without an apiVersion:"},
		},
		// The written kind decides whether an object is a List, not the one
		// its Go value reports. A List kind left out of what is written is
		// one object without a kind, refused before the identity check could
		// find its item, a copy of the layout's ConfigMap "fine", twice.
		"a List kind its TypeMeta leaves out of what is written": {
			func(t *testing.T) client.Object { return unwrittenTypeOf("ConfigMapList", "", typedConfigMap("fine")) },
			[]string{`an object "default/holder" (Go type *layout_test.unwrittenType)`, "written without a kind and an apiVersion"},
		},
		"an item without TypeMeta of an object written as a List kind its Go value does not report": {
			func(t *testing.T) client.Object {
				return unwrittenTypeOf("Inventory", "ConfigMapList", *untypedConfigMap("cm"))
			},
			[]string{`an object "default/cm" (Go type *v1.ConfigMap)`, `held in ConfigMapList "default/holder"`, "written without a kind and an apiVersion"},
		},
		// A build reads the kind of a local-config object before it drops it,
		// and refuses one without.
		"a local-config object without a kind": {
			func(t *testing.T) client.Object { return localConfig(withoutField("cm", "kind"), "true") },
			[]string{`an object "default/cm"`, "written without a kind:"},
		},
		// "false" keeps the object in the build.
		"an object without an apiVersion whose local-config annotation is false": {
			func(t *testing.T) client.Object { return localConfig(withoutField("cm", "apiVersion"), "false") },
			[]string{`ConfigMap "default/cm"`, "written without an apiVersion:"},
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

// localConfig gives obj kustomize's local-config annotation at value.
func localConfig(obj *unstructured.Unstructured, value string) *unstructured.Unstructured {
	obj.SetAnnotations(map[string]string{"config.kubernetes.io/local-config": value})
	return obj
}

// TestWriters_LocalConfigWithoutAPIVersion: an object with kustomize's
// local-config annotation at any string value but "false" is not judged on its
// apiVersion. A build drops it without requiring one, so it is written as
// before.
func TestWriters_LocalConfigWithoutAPIVersion(t *testing.T) {
	cases := map[string]func(t *testing.T) client.Object{
		"an unstructured object": func(t *testing.T) client.Object {
			return localConfig(withoutField("held", "apiVersion"), "true")
		},
		"an item of an unstructured List, at a value that is not true": func(t *testing.T) client.Object {
			return listOf("List", localConfig(withoutField("held", "apiVersion"), "x"))
		},
		"a typed object with a kind and no apiVersion": func(t *testing.T) client.Object {
			cm := untypedConfigMap("held")
			cm.Kind = "ConfigMap"
			cm.Annotations = map[string]string{"config.kubernetes.io/local-config": ""}
			return cm
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
					t.Errorf("the local-config object is written %d times, want once", found)
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
		// A List by the kind it is written with, whatever kind its Go value
		// reports.
		"an object written as a List kind its Go value does not report": func(t *testing.T) client.Object {
			return unwrittenTypeOf("Inventory", "ConfigMapList", typedConfigMap("held"))
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
