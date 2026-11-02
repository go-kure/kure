package fluxcd

import (
	"encoding/json"
	"slices"
	"testing"

	kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/stack"
)

// kustomizationWithSpec is a Flux Kustomization whose spec is spec, as a
// tree's hand-written file holds it.
func kustomizationWithSpec(spec map[string]any) *unstructured.Unstructured {
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": kustv1.GroupVersion.String(),
		"kind":       "Kustomization",
		"metadata":   map[string]any{"name": "x", "namespace": "flux-system"},
		"spec": map[string]any{
			"path":      "./x",
			"sourceRef": map[string]any{"kind": "GitRepository", "name": "flux-system"},
		},
	}}
	for k, v := range spec {
		_ = unstructured.SetNestedField(u.Object, v, "spec", k)
	}
	return u
}

// TestSpecFields_ZeroValueEqualsOmitted: a field of zeroAsOmitted set to its
// zero value, of the type its CRD gives it, reads as the field omitted
// (go-kure/kure#1169). Every other value of every other field, a value of
// another type and a field named in another case compare as written.
func TestSpecFields_ZeroValueEqualsOmitted(t *testing.T) {
	cases := map[string]struct {
		a, b   map[string]any
		differ bool
	}{
		"wait true against omitted":       {a: map[string]any{"wait": true}, b: map[string]any{}, differ: true},
		"wait true against Wait true":     {a: map[string]any{"wait": true}, b: map[string]any{"Wait": true}, differ: true},
		"wait true against wait true":     {a: map[string]any{"wait": true}, b: map[string]any{"wait": true}},
		"wait false against wait true":    {a: map[string]any{"wait": false}, b: map[string]any{"wait": true}, differ: true},
		"nested name in another case":     {a: map[string]any{"sourceRef": map[string]any{"kind": "GitRepository", "name": "a"}}, b: map[string]any{"sourceRef": map[string]any{"kind": "GitRepository", "Name": "a"}}, differ: true},
		"wrong-typed value against equal": {a: map[string]any{"wait": "yes"}, b: map[string]any{"wait": "yes"}},
		"a null map entry against an empty string": {
			a:      map[string]any{"postBuild": map[string]any{"substitute": map[string]any{"X": nil}}},
			b:      map[string]any{"postBuild": map[string]any{"substitute": map[string]any{"X": ""}}},
			differ: true,
		},
		"an empty postBuild against omitted": {a: map[string]any{"postBuild": map[string]any{}}, b: map[string]any{}, differ: true},
		// The CRD requires a non-empty value for these, so "" is no omission.
		"namePrefix empty against omitted":      {a: map[string]any{"namePrefix": ""}, b: map[string]any{}, differ: true},
		"nameSuffix empty against omitted":      {a: map[string]any{"nameSuffix": ""}, b: map[string]any{}, differ: true},
		"targetNamespace empty against omitted": {a: map[string]any{"targetNamespace": ""}, b: map[string]any{}, differ: true},
		"path empty against omitted":            {a: map[string]any{"path": ""}, b: map[string]any{}, differ: true},
		"namePrefix null against omitted":       {a: map[string]any{"namePrefix": nil}, b: map[string]any{}, differ: true},
		// A zero value of another type than the CRD gives the field is no omission.
		"wait empty string against omitted":      {a: map[string]any{"wait": ""}, b: map[string]any{}, differ: true},
		"wait empty list against omitted":        {a: map[string]any{"wait": []any{}}, b: map[string]any{}, differ: true},
		"wait zero against omitted":              {a: map[string]any{"wait": int64(0)}, b: map[string]any{}, differ: true},
		"images false against omitted":           {a: map[string]any{"images": false}, b: map[string]any{}, differ: true},
		"images empty string against omitted":    {a: map[string]any{"images": ""}, b: map[string]any{}, differ: true},
		"dependsOn empty object against omitted": {a: map[string]any{"dependsOn": map[string]any{}}, b: map[string]any{}, differ: true},
	}
	for field, kind := range zeroAsOmitted {
		zero := any(false)
		if kind == "array" {
			zero = []any{}
		}
		cases[field+" zero value against omitted"] = struct {
			a, b   map[string]any
			differ bool
		}{a: map[string]any{field: zero}, b: map[string]any{}}
		cases[field+" null against omitted"] = struct {
			a, b   map[string]any
			differ bool
		}{a: map[string]any{field: nil}, b: map[string]any{}}
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			differ, err := differingFields(
				occurrence{obj: kustomizationWithSpec(c.a)},
				occurrence{obj: kustomizationWithSpec(c.b)},
			)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(differ) > 0; got != c.differ {
				t.Fatalf("got differing fields %v, want differ=%t", differ, c.differ)
			}
		})
	}
}

// TestZeroAsOmittedMatchesVendoredCRD holds zeroAsOmitted to the Flux
// Kustomization CRD the package vendors, in every version it serves: each
// entry is a graph field of the type the table names, and has no schema
// default, so omitting it is not defaulted to a non-zero value. namePrefix,
// nameSuffix and targetNamespace require a non-empty value, which is why the
// table leaves "" alone. A Flux release that changes any of this fails here.
func TestZeroAsOmittedMatchesVendoredCRD(t *testing.T) {
	gotk, err := NewBootstrapGenerator().generateGotkComponents(&stack.BootstrapConfig{Enabled: true, FluxMode: ModeGotk})
	if err != nil {
		t.Fatalf("generateGotkComponents: %v", err)
	}
	const name = "kustomizations.kustomize.toolkit.fluxcd.io"
	var crd *apiextensionsv1.CustomResourceDefinition
	for _, o := range gotk {
		if o.GetName() != name || o.GetObjectKind().GroupVersionKind().Kind != "CustomResourceDefinition" {
			continue
		}
		raw, err := json.Marshal(o)
		if err != nil {
			t.Fatal(err)
		}
		crd = &apiextensionsv1.CustomResourceDefinition{}
		if err := json.Unmarshal(raw, crd); err != nil {
			t.Fatalf("the vendored %s is not a CustomResourceDefinition: %v", name, err)
		}
	}
	if crd == nil {
		t.Fatalf("no CustomResourceDefinition %q in the vendored flux2 bundle (%s)", name, GotkVersion)
	}

	checked := 0
	for _, version := range crd.Spec.Versions {
		if !version.Served || version.Schema == nil || version.Schema.OpenAPIV3Schema == nil {
			continue
		}
		spec := version.Schema.OpenAPIV3Schema.Properties["spec"].Properties
		for field, want := range zeroAsOmitted {
			if !slices.Contains(graphFields, field) {
				t.Errorf("zeroAsOmitted lists %q, which is no graph field", field)
			}
			prop, ok := spec[field]
			if !ok {
				t.Errorf("%s %s has no spec.%s", name, version.Name, field)
				continue
			}
			if prop.Type != want {
				t.Errorf("%s %s spec.%s has type %q, zeroAsOmitted says %q", name, version.Name, field, prop.Type, want)
			}
			if prop.Default != nil {
				t.Errorf("%s %s spec.%s has a default (%s): omitting it is not the zero value", name, version.Name, field, prop.Default.Raw)
			}
			checked++
		}
		for _, field := range []string{"namePrefix", "nameSuffix", "targetNamespace"} {
			if min := spec[field].MinLength; min == nil || *min < 1 {
				t.Errorf("%s %s spec.%s no longer requires a non-empty value: \"\" may now be an omission for it", name, version.Name, field)
			}
			checked++
		}
	}
	if checked == 0 {
		t.Fatalf("the vendored %s (%s) serves no version with a schema: nothing was compared", name, GotkVersion)
	}
}
