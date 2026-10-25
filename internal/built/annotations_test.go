package built

import (
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestAnnotations(t *testing.T) {
	t.Run("unstructured keeps non-string values", func(t *testing.T) {
		obj := configMap("app")
		obj.Object["metadata"].(map[string]any)["annotations"] = map[string]any{
			"team": "web", "count": int64(1), "flag": true, "none": nil,
		}
		got := Annotations(obj)
		want := map[string]any{"team": "web", "count": int64(1), "flag": true, "none": nil}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Annotations = %#v, want %#v", got, want)
		}
		got["team"] = "edited"
		if v := obj.Object["metadata"].(map[string]any)["annotations"].(map[string]any)["team"]; v != "web" {
			t.Errorf("editing the result reached the object: team = %v", v)
		}
	})
	t.Run("unstructured without annotations", func(t *testing.T) {
		if got := Annotations(configMap("app")); got != nil {
			t.Errorf("Annotations = %#v, want nil", got)
		}
	})
	t.Run("unstructured without metadata", func(t *testing.T) {
		if got := Annotations(&unstructured.Unstructured{Object: map[string]any{"kind": "ConfigMap"}}); got != nil {
			t.Errorf("Annotations = %#v, want nil", got)
		}
	})
	t.Run("typed", func(t *testing.T) {
		obj := typedConfigMap("app")
		obj.Annotations = map[string]string{"team": "web"}
		got := Annotations(obj)
		if want := map[string]any{"team": "web"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Annotations = %#v, want %#v", got, want)
		}
		got["team"] = "edited"
		if obj.Annotations["team"] != "web" {
			t.Errorf("editing the result reached the object: team = %q", obj.Annotations["team"])
		}
	})
	t.Run("typed without annotations", func(t *testing.T) {
		if got := Annotations(typedConfigMap("app")); got != nil {
			t.Errorf("Annotations = %#v, want nil", got)
		}
	})
}

func TestAddAnnotations_Unstructured(t *testing.T) {
	want := map[string]string{"prune": "disabled", "team": "ops"}
	cases := []struct {
		name   string
		object func() *unstructured.Unstructured
		after  map[string]any // metadata.annotations after the call
	}{
		{
			name: "existing annotations, non-string value kept",
			object: func() *unstructured.Unstructured {
				obj := configMap("app")
				obj.Object["metadata"].(map[string]any)["annotations"] = map[string]any{"team": "web", "count": int64(1)}
				return obj
			},
			after: map[string]any{"team": "web", "count": int64(1), "prune": "disabled"},
		},
		{
			name:   "no annotations",
			object: func() *unstructured.Unstructured { return configMap("app") },
			after:  map[string]any{"team": "ops", "prune": "disabled"},
		},
		{
			name: "no metadata",
			object: func() *unstructured.Unstructured {
				return &unstructured.Unstructured{Object: map[string]any{"apiVersion": "v1", "kind": "ConfigMap"}}
			},
			after: map[string]any{"team": "ops", "prune": "disabled"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			obj := tc.object()
			before := runtime.DeepCopyJSON(obj.Object)
			undo := AddAnnotations(obj, want)
			if undo == nil {
				t.Fatal("AddAnnotations returned no undo for a change")
			}
			got, _, _ := unstructured.NestedFieldNoCopy(obj.Object, "metadata", "annotations")
			if !reflect.DeepEqual(got, tc.after) {
				t.Errorf("annotations = %#v, want %#v", got, tc.after)
			}
			undo()
			if !reflect.DeepEqual(obj.Object, before) {
				t.Errorf("undo left %#v, want %#v", obj.Object, before)
			}
		})
	}
}

func TestAddAnnotations_Typed(t *testing.T) {
	t.Run("existing annotations", func(t *testing.T) {
		obj := typedConfigMap("app")
		obj.Annotations = map[string]string{"team": "web"}
		undo := AddAnnotations(obj, map[string]string{"prune": "disabled", "team": "ops"})
		if want := map[string]string{"team": "web", "prune": "disabled"}; !reflect.DeepEqual(obj.Annotations, want) {
			t.Errorf("annotations = %#v, want %#v", obj.Annotations, want)
		}
		undo()
		if want := map[string]string{"team": "web"}; !reflect.DeepEqual(obj.Annotations, want) {
			t.Errorf("undo left %#v, want %#v", obj.Annotations, want)
		}
	})
	t.Run("no annotations", func(t *testing.T) {
		obj := typedConfigMap("app")
		undo := AddAnnotations(obj, map[string]string{"prune": "disabled"})
		if want := map[string]string{"prune": "disabled"}; !reflect.DeepEqual(obj.Annotations, want) {
			t.Errorf("annotations = %#v, want %#v", obj.Annotations, want)
		}
		undo()
		if obj.Annotations != nil {
			t.Errorf("undo left %#v, want nil", obj.Annotations)
		}
	})
}

func TestAddAnnotations_NoChange(t *testing.T) {
	obj := configMap("app")
	obj.Object["metadata"].(map[string]any)["annotations"] = map[string]any{"prune": false}
	before := runtime.DeepCopyJSON(obj.Object)
	if undo := AddAnnotations(obj, map[string]string{"prune": "disabled"}); undo != nil {
		t.Error("AddAnnotations returned an undo though every key was present")
	}
	if !reflect.DeepEqual(obj.Object, before) {
		t.Errorf("object changed: %#v, want %#v", obj.Object, before)
	}
	if undo := AddAnnotations(typedConfigMap("app"), nil); undo != nil {
		t.Error("AddAnnotations returned an undo for an empty want")
	}
}
