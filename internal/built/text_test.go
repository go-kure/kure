package built

import (
	"maps"
	"reflect"
	"testing"
)

func TestLabels(t *testing.T) {
	t.Run("unstructured keeps non-string values", func(t *testing.T) {
		obj := configMap("app")
		obj.Object["metadata"].(map[string]any)["labels"] = map[string]any{
			"team": "web", "count": int64(1), "flag": true, "none": nil,
		}
		got := Labels(obj)
		want := map[string]any{"team": "web", "count": int64(1), "flag": true, "none": nil}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("Labels = %#v, want %#v", got, want)
		}
		got["team"] = "edited"
		if v := obj.Object["metadata"].(map[string]any)["labels"].(map[string]any)["team"]; v != "web" {
			t.Errorf("editing the result reached the object: team = %v", v)
		}
	})
	t.Run("unstructured without labels", func(t *testing.T) {
		if got := Labels(configMap("app")); got != nil {
			t.Errorf("Labels = %#v, want nil", got)
		}
	})
	t.Run("typed", func(t *testing.T) {
		obj := typedConfigMap("app")
		obj.Labels = map[string]string{"team": "web"}
		got := Labels(obj)
		if want := map[string]any{"team": "web"}; !reflect.DeepEqual(got, want) {
			t.Fatalf("Labels = %#v, want %#v", got, want)
		}
		got["team"] = "edited"
		if obj.Labels["team"] != "web" {
			t.Errorf("editing the result reached the object: team = %q", obj.Labels["team"])
		}
	})
	t.Run("typed without labels", func(t *testing.T) {
		if got := Labels(typedConfigMap("app")); got != nil {
			t.Errorf("Labels = %#v, want nil", got)
		}
	})
}

// TestText pins the text a kustomize build reads each label and annotation
// as from what the writers write, for a value of each type an object decoded
// from YAML holds, and that what the writers leave out is not read.
func TestText(t *testing.T) {
	const lastApplied = "kubectl.kubernetes.io/last-applied-configuration"
	values := map[string]any{
		"string":       "web",
		"string true":  "true",
		"true":         true,
		"false":        false,
		"int":          int64(3),
		"negative int": int64(-3),
		"float":        1.5,
		"whole float":  float64(2),
		"null":         nil,
		"map":          map[string]any{"a": "b"},
		"list":         []any{"a"},
	}
	want := map[string]string{
		"string":       "web",
		"string true":  "true",
		"true":         "true",
		"false":        "false",
		"int":          "3",
		"negative int": "-3",
		"float":        "1.5",
		"whole float":  "2",
		"null":         "null",
		"map":          "",
		"list":         "",
	}
	check := func(t *testing.T, gotLabels, gotAnnotations, wantLabels, wantAnnotations map[string]string, err error) {
		t.Helper()
		if err != nil {
			t.Fatalf("Text: %v", err)
		}
		if !reflect.DeepEqual(gotLabels, wantLabels) {
			t.Errorf("labels = %#v\nwant %#v", gotLabels, wantLabels)
		}
		if !reflect.DeepEqual(gotAnnotations, wantAnnotations) {
			t.Errorf("annotations = %#v\nwant %#v", gotAnnotations, wantAnnotations)
		}
	}

	t.Run("every type", func(t *testing.T) {
		obj := configMap("app")
		meta := obj.Object["metadata"].(map[string]any)
		meta["labels"] = maps.Clone(values)
		meta["annotations"] = maps.Clone(values)
		l, a, err := Text(obj)
		check(t, l, a, want, want, err)
	})
	t.Run("last-applied-configuration annotation is not written", func(t *testing.T) {
		for name, tc := range map[string]struct {
			value any
			text  string
		}{
			"string":  {"{}", "{}"},
			"boolean": {true, "true"},
		} {
			t.Run(name, func(t *testing.T) {
				obj := configMap("app")
				meta := obj.Object["metadata"].(map[string]any)
				// The writers drop the annotation and keep a label of the
				// same key.
				meta["labels"] = map[string]any{lastApplied: tc.value}
				meta["annotations"] = map[string]any{lastApplied: tc.value, "team": "web"}
				l, a, err := Text(obj)
				check(t, l, a, map[string]string{lastApplied: tc.text}, map[string]string{"team": "web"}, err)
			})
		}
	})
	t.Run("typed", func(t *testing.T) {
		obj := typedConfigMap("app")
		obj.Labels = map[string]string{"team": "web", "empty": ""}
		obj.Annotations = map[string]string{"note": "x"}
		l, a, err := Text(obj)
		check(t, l, a, map[string]string{"team": "web", "empty": ""}, map[string]string{"note": "x"}, err)
	})
	t.Run("none", func(t *testing.T) {
		l, a, err := Text(configMap("app"))
		check(t, l, a, map[string]string{}, map[string]string{}, err)
	})
}
