package crdvalidate

import (
	"context"
	"reflect"
	"strings"
	"testing"

	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	utiljson "k8s.io/apimachinery/pkg/util/json"
	"k8s.io/apimachinery/pkg/util/validation/field"
	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/yaml"
)

// widgetCRD exercises every routine on the create path: enum, required,
// defaults, nullable, list types, scale and status subresources, a CEL
// rule, and an unserved version.
const widgetCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: widgets.example.com
spec:
  group: example.com
  names:
    kind: Widget
    plural: widgets
  scope: Namespaced
  versions:
  - name: v1alpha1
    served: false
    storage: false
    schema:
      openAPIV3Schema:
        type: object
  - name: v1
    served: true
    storage: true
    subresources:
      status: {}
      scale:
        specReplicasPath: .spec.replicas
        statusReplicasPath: .status.replicas
        labelSelectorPath: .status.selector
    schema:
      openAPIV3Schema:
        type: object
        x-kubernetes-validations:
        - rule: "!has(self.spec.size) || self.spec.size <= 10"
          message: size is capped at 10
        - rule: self.spec.mode == 'auto'
          message: mode must be auto
        properties:
          spec:
            type: object
            required: [color]
            properties:
              color:
                type: string
                enum: [red, green]
              size:
                type: integer
                minimum: 1
              mode:
                type: string
                default: auto
              replicas:
                type: integer
              note:
                type: string
                nullable: true
              tags:
                type: array
                x-kubernetes-list-type: set
                items:
                  type: string
              ports:
                type: array
                x-kubernetes-list-type: map
                x-kubernetes-list-map-keys: [name]
                items:
                  type: object
                  required: [name]
                  properties:
                    name:
                      type: string
                    port:
                      type: integer
              template:
                type: object
                x-kubernetes-embedded-resource: true
                x-kubernetes-preserve-unknown-fields: true
          status:
            type: object
            properties:
              replicas:
                type: integer
              selector:
                type: string
`

// gadgetCRD is cluster-scoped, has no subresources and no rules.
const gadgetCRD = `apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: gadgets.example.com
spec:
  group: example.com
  names:
    kind: Gadget
    plural: gadgets
  scope: Cluster
  versions:
  - name: v1
    served: true
    storage: true
    schema:
      openAPIV3Schema:
        type: object
        properties:
          status:
            type: object
            properties:
              ready:
                type: boolean
`

func crd(t *testing.T, doc string) *apiextensionsv1.CustomResourceDefinition {
	t.Helper()
	out := &apiextensionsv1.CustomResourceDefinition{}
	if err := yaml.UnmarshalStrict([]byte(doc), out); err != nil {
		t.Fatalf("test definition does not decode: %v", err)
	}
	return out
}

// object decodes as the server decodes a request body: through
// apimachinery's JSON path, which reads whole numbers as int64. The
// encoding/json default, float64, is a shape the server never sees.
func object(t *testing.T, doc string) *unstructured.Unstructured {
	t.Helper()
	data, err := utilyaml.ToJSON([]byte(doc))
	if err != nil {
		t.Fatalf("test object is not YAML: %v", err)
	}
	out := &unstructured.Unstructured{}
	if err := utiljson.Unmarshal(data, &out.Object); err != nil {
		t.Fatalf("test object does not decode: %v", err)
	}
	return out
}

func validator(t *testing.T) *Validator {
	t.Helper()
	v, err := New([]*apiextensionsv1.CustomResourceDefinition{crd(t, widgetCRD), crd(t, gadgetCRD)})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return v
}

// validate runs Validate and fails the test on the error that says the
// validator could not answer at all.
func validate(t *testing.T, v *Validator, doc string) field.ErrorList {
	t.Helper()
	errs, err := v.Validate(context.Background(), object(t, doc))
	if err != nil {
		t.Fatalf("Validate: %v", err)
	}
	return errs
}

// want asserts that errs holds exactly the given errors, in any order, each
// written as "<type> <field>: <start of detail>" with the type's FieldValue
// prefix dropped.
func want(t *testing.T, errs field.ErrorList, expected ...string) {
	t.Helper()
	var got []string
	for _, e := range errs {
		got = append(got, strings.TrimPrefix(string(e.Type), "FieldValue")+" "+e.Field+": "+e.Detail)
	}
	if len(errs) != len(expected) {
		t.Fatalf("got %d errors, want %d:\n got  %q\n want %q", len(errs), len(expected), got, expected)
	}
	for _, e := range expected {
		found := false
		for _, g := range got {
			if strings.HasPrefix(g, e) {
				found = true
			}
		}
		if !found {
			t.Errorf("no error matching %q in %q", e, got)
		}
	}
}

const validWidget = `apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
  namespace: default
spec:
  color: red
  size: 3
  replicas: 2
`

func TestValidateAcceptsAValidObject(t *testing.T) {
	want(t, validate(t, validator(t), validWidget))
}

func TestValidateDoesNotChangeTheObject(t *testing.T) {
	in := object(t, `apiVersion: example.com/v1
kind: Widget
metadata:
  name: w
spec:
  color: red
  bogus: 1
status:
  replicas: 1
`)
	before := in.DeepCopy()
	if _, err := validator(t).Validate(context.Background(), in); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in, before) {
		t.Errorf("Validate changed its argument:\n before %v\n after  %v", before.Object, in.Object)
	}
}

func TestValidateReportsSchemaViolations(t *testing.T) {
	v := validator(t)
	// an enum, required or type error blocks the CEL rules, and says so
	t.Run("enum", func(t *testing.T) {
		want(t, validate(t, v, strings.Replace(validWidget, "color: red", "color: blue", 1)),
			`NotSupported spec.color`,
			`Invalid <nil>: some validation rules were not checked`)
	})
	t.Run("required", func(t *testing.T) {
		want(t, validate(t, v, strings.Replace(validWidget, "  color: red\n", "", 1)),
			`Required spec.color`,
			`Invalid <nil>: some validation rules were not checked`)
	})
	t.Run("minimum", func(t *testing.T) {
		want(t, validate(t, v, strings.Replace(validWidget, "size: 3", "size: 0", 1)),
			`Invalid spec.size: spec.size in body should be greater than or equal to 1`)
	})
	t.Run("type", func(t *testing.T) {
		want(t, validate(t, v, strings.Replace(validWidget, "size: 3", "size: big", 1)),
			`TypeInvalid spec.size`,
			`Invalid <nil>: some validation rules were not checked`)
	})
}

func TestValidateRunsTheCELRules(t *testing.T) {
	v := validator(t)
	t.Run("a rule that fails is reported with its message", func(t *testing.T) {
		want(t, validate(t, v, strings.Replace(validWidget, "size: 3", "size: 11", 1)),
			`Invalid <nil>: size is capped at 10`)
	})
	t.Run("a blocking schema error leaves the rules unchecked", func(t *testing.T) {
		doc := strings.NewReplacer("size: 3", "size: 11", "color: red", "color: blue").Replace(validWidget)
		want(t, validate(t, v, doc),
			`NotSupported spec.color`,
			`Invalid <nil>: some validation rules were not checked`)
	})
	t.Run("defaults are applied before the rules see the object", func(t *testing.T) {
		// mode is absent; the rule requires it to be auto, which the default supplies
		want(t, validate(t, v, validWidget))
		want(t, validate(t, v, validWidget+"  mode: manual\n"), `Invalid <nil>: mode must be auto`)
	})
}

func TestValidateRejectsUnknownFieldsBeforeAnythingElse(t *testing.T) {
	v := validator(t)
	// the object also breaks the size rule, and that is not reported
	doc := strings.Replace(validWidget, "size: 3", "size: 11\n  bogus: 1\n", 1) + "metadata2: {}\n"
	want(t, validate(t, v, doc),
		`Forbidden spec.bogus: unknown field`,
		`Forbidden metadata2: unknown field`)
	want(t, validate(t, v, strings.Replace(validWidget, "  name: w\n", "  name: w\n  bogus: 1\n", 1)),
		`Forbidden metadata.bogus: unknown field`)
}

func TestValidateDropsStatusWhenTheSubresourceIsServed(t *testing.T) {
	v := validator(t)
	// Widget serves status: a status that violates the schema is dropped, not reported
	want(t, validate(t, v, validWidget+"status:\n  replicas: many\n"))
	// Gadget does not: the same status is validated
	want(t, validate(t, v, "apiVersion: example.com/v1\nkind: Gadget\nmetadata:\n  name: g\nstatus:\n  ready: maybe\n"),
		`TypeInvalid status.ready`)
}

func TestValidatePrunesNullsAsTheServerDoes(t *testing.T) {
	v := validator(t)
	// non-nullable without a default: dropped, so no type error and no enum error
	want(t, validate(t, v, strings.Replace(validWidget, "size: 3", "size: null", 1)))
	// nullable: kept and valid
	want(t, validate(t, v, validWidget+"  note: null\n"))
	// non-nullable with a default: the default replaces it
	want(t, validate(t, v, validWidget+"  mode: null\n"))
}

func TestValidateChecksListTypes(t *testing.T) {
	v := validator(t)
	want(t, validate(t, v, validWidget+"  tags: [a, a]\n"), `Duplicate spec.tags[1]`)
	want(t, validate(t, v, validWidget+"  ports:\n  - name: http\n  - name: http\n"), `Duplicate spec.ports[1]`)
}

func TestValidateChecksScalePaths(t *testing.T) {
	v := validator(t)
	want(t, validate(t, v, strings.Replace(validWidget, "replicas: 2", "replicas: -1", 1)),
		`Invalid .spec.replicas: should be a non-negative integer`)
	want(t, validate(t, v, strings.Replace(validWidget, "replicas: 2", "replicas: 3000000000", 1)),
		`Invalid .spec.replicas: should be less than or equal to 2147483647`)
	// the schema reports the type, and the scale path reports what it could not read
	want(t, validate(t, v, strings.Replace(validWidget, "replicas: 2", "replicas: two", 1)),
		`TypeInvalid spec.replicas`,
		`Invalid .spec.replicas: `,
		`Invalid <nil>: some validation rules were not checked`)
}

func TestValidateReportsMetadataTheServerCannotDecode(t *testing.T) {
	v := validator(t)
	want(t, validate(t, v, strings.Replace(validWidget, "metadata:\n  name: w\n  namespace: default\n", "metadata: w\n", 1)),
		`Invalid metadata`)
	want(t, validate(t, v, strings.Replace(validWidget, "  name: w\n", "  name: 5\n", 1)),
		`Invalid metadata`)
	// an embedded resource's metadata is held to the same rule
	want(t, validate(t, v, validWidget+"  template:\n    apiVersion: v1\n    kind: Pod\n    metadata: nope\n"),
		`Invalid spec.template.metadata`)
}

func TestValidateCannotAnswerWhenTheSchemaWouldNotBeServed(t *testing.T) {
	c := crd(t, gadgetCRD)
	c.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["loose"] = apiextensionsv1.JSONSchemaProps{Description: "no type"}
	v, err := New([]*apiextensionsv1.CustomResourceDefinition{c})
	if err != nil {
		t.Fatal(err)
	}
	_, err = v.Validate(context.Background(), object(t, "apiVersion: example.com/v1\nkind: Gadget\nmetadata:\n  name: g\n"))
	if err == nil || !strings.Contains(err.Error(), "would not accept the definition") || !strings.Contains(err.Error(), "properties[loose].type") {
		t.Errorf("Validate: %v, want the server's refusal naming the untyped property", err)
	}
	if err := v.Compile(context.Background(), schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Gizmo"}); err == nil || !strings.Contains(err.Error(), "no definition for Gizmo.example.com") {
		t.Errorf("Compile of an unknown kind: %v", err)
	}
}

// A rule that does not compile is the server's refusal of the definition,
// not an error on every object that reaches the rule.
func TestCompileRejectsARuleThatDoesNotCompile(t *testing.T) {
	c := crd(t, gadgetCRD)
	c.Spec.Versions[0].Schema.OpenAPIV3Schema.XValidations = apiextensionsv1.ValidationRules{{Rule: "self.nonsense(", Message: "never"}}
	v, err := New([]*apiextensionsv1.CustomResourceDefinition{c})
	if err != nil {
		t.Fatal(err)
	}
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Gadget"}
	err = v.Compile(context.Background(), gvk)
	if err == nil || !strings.Contains(err.Error(), "would not accept the definition") || !strings.Contains(err.Error(), "x-kubernetes-validations[0].rule") {
		t.Errorf("Compile: %v, want the server's refusal naming the rule", err)
	}
	errs, err := v.Validate(context.Background(), object(t, "apiVersion: example.com/v1\nkind: Gadget\nmetadata:\n  name: g\n"))
	if err == nil || len(errs) != 0 {
		t.Errorf("Validate answered with %v, %v; want the same refusal and no object errors", errs, err)
	}
}

// The server generates the name from generateName before it validates
// (Store.create, then BeforeCreate); an object with only generateName is
// not one with no name. One with neither is, and the Required error blocks
// the rules as the server's does.
func TestValidateGeneratesTheName(t *testing.T) {
	v := validator(t)
	want(t, validate(t, v, strings.Replace(validWidget, "name: w", "generateName: w-", 1)))
	want(t, validate(t, v, strings.Replace(validWidget, "  name: w\n", "", 1)),
		`Required metadata.name`, `Invalid <nil>: some validation rules were not checked`)
}

func TestValidateAppliesTheRequestNamespaceRule(t *testing.T) {
	v := validator(t)
	t.Run("a namespaced object without a namespace is in the request namespace", func(t *testing.T) {
		want(t, validate(t, v, strings.Replace(validWidget, "  namespace: default\n", "", 1)))
	})
	t.Run("a cluster-scoped object without a namespace", func(t *testing.T) {
		want(t, validate(t, v, "apiVersion: example.com/v1\nkind: Gadget\nmetadata:\n  name: g\n"))
	})
	t.Run("a cluster-scoped object with a namespace is forbidden", func(t *testing.T) {
		want(t, validate(t, v, "apiVersion: example.com/v1\nkind: Gadget\nmetadata:\n  name: g\n  namespace: default\n"),
			`Forbidden metadata.namespace: not allowed on this type`)
	})
}

func TestValidateChecksObjectMeta(t *testing.T) {
	v := validator(t)
	want(t, validate(t, v, strings.Replace(validWidget, "name: w", "name: Not_A_Subdomain", 1)),
		`Invalid metadata.name`)
	want(t, validate(t, v, strings.Replace(validWidget, "  name: w\n", "  name: w\n  labels:\n    a: 'not valid!'\n", 1)),
		`Invalid metadata.labels: `)
	want(t, validate(t, v, strings.Replace(validWidget, "  name: w\n", "", 1)),
		`Required metadata.name`,
		`Invalid <nil>: some validation rules were not checked`)
}

func TestValidateRejectsAnUnservedVersion(t *testing.T) {
	v := validator(t)
	want(t, validate(t, v, strings.Replace(validWidget, "example.com/v1", "example.com/v1alpha1", 1)),
		`NotSupported apiVersion`)
	if err := v.Compile(context.Background(), schema.GroupVersionKind{Group: "example.com", Version: "v1alpha1", Kind: "Widget"}); err == nil || !strings.Contains(err.Error(), "does not serve v1alpha1") {
		t.Errorf("Compile of an unserved version: %v", err)
	}
	if err := v.Compile(context.Background(), schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Widget"}); err != nil {
		t.Errorf("Compile of a served version: %v", err)
	}
}

func TestValidateCannotAnswerForAnUnknownKind(t *testing.T) {
	v := validator(t)
	_, err := v.Validate(context.Background(), object(t, "apiVersion: example.com/v1\nkind: Gizmo\nmetadata:\n  name: g\n"))
	if err == nil || !strings.Contains(err.Error(), "no definition for Gizmo.example.com") {
		t.Errorf("Validate: %v", err)
	}
	if v.Definition(schema.GroupKind{Group: "example.com", Kind: "Gizmo"}) != nil {
		t.Error("Definition returned something for an unknown kind")
	}
	if d := v.Definition(schema.GroupKind{Group: "example.com", Kind: "Widget"}); d == nil || d.Name != "widgets.example.com" {
		t.Errorf("Definition(Widget) = %v", d)
	}
}

func TestNewRejectsTwoDefinitionsForOneKind(t *testing.T) {
	second := crd(t, widgetCRD)
	second.Name = "widgets.again.example.com"
	_, err := New([]*apiextensionsv1.CustomResourceDefinition{crd(t, widgetCRD), second})
	if err == nil || !strings.Contains(err.Error(), "widgets.example.com") || !strings.Contains(err.Error(), "widgets.again.example.com") {
		t.Errorf("New: %v, want an error naming both definitions", err)
	}
	_, err = New([]*apiextensionsv1.CustomResourceDefinition{{}})
	if err == nil {
		t.Error("New accepted a definition with no group or kind")
	}
}

// Each case is a definition the server refuses on create; the error names
// the field the server names.
func TestCompileRejectsWhatTheServerWouldNotServe(t *testing.T) {
	gvk := schema.GroupVersionKind{Group: "example.com", Version: "v1", Kind: "Gadget"}
	t.Run("no schema", func(t *testing.T) {
		c := crd(t, gadgetCRD)
		c.Spec.Versions[0].Schema = nil
		v, _ := New([]*apiextensionsv1.CustomResourceDefinition{c})
		if err := v.Compile(context.Background(), gvk); err == nil || !strings.Contains(err.Error(), "would not accept the definition") || !strings.Contains(err.Error(), "spec.versions[0].schema.openAPIV3Schema") {
			t.Errorf("Compile: %v", err)
		}
	})
	t.Run("not structural", func(t *testing.T) {
		c := crd(t, gadgetCRD)
		c.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties["loose"] = apiextensionsv1.JSONSchemaProps{Description: "no type"}
		v, _ := New([]*apiextensionsv1.CustomResourceDefinition{c})
		if err := v.Compile(context.Background(), gvk); err == nil || !strings.Contains(err.Error(), "would not accept the definition") || !strings.Contains(err.Error(), "properties[loose].type") {
			t.Errorf("Compile: %v", err)
		}
	})
	t.Run("preserves unknown fields", func(t *testing.T) {
		c := crd(t, gadgetCRD)
		c.Spec.PreserveUnknownFields = true
		v, _ := New([]*apiextensionsv1.CustomResourceDefinition{c})
		if err := v.Compile(context.Background(), gvk); err == nil || !strings.Contains(err.Error(), "would not accept the definition") || !strings.Contains(err.Error(), "spec.preserveUnknownFields") {
			t.Errorf("Compile: %v", err)
		}
	})
	t.Run("name is not plural.group", func(t *testing.T) {
		c := crd(t, gadgetCRD)
		c.Name = "gadget.example.com"
		v, _ := New([]*apiextensionsv1.CustomResourceDefinition{c})
		if err := v.Compile(context.Background(), gvk); err == nil || !strings.Contains(err.Error(), "would not accept the definition") || !strings.Contains(err.Error(), "metadata.name") {
			t.Errorf("Compile: %v", err)
		}
	})
}

func TestValidateObjectMeta(t *testing.T) {
	ctx := context.Background()
	deployment := func(meta string) *unstructured.Unstructured {
		return object(t, "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n"+meta)
	}
	want(t, ValidateObjectMeta(ctx, deployment("  name: d\n"), true, nil))
	want(t, ValidateObjectMeta(ctx, deployment("  name: d\n  namespace: web\n"), true, nil))
	want(t, ValidateObjectMeta(ctx, deployment("  name: Not_A_Subdomain\n"), true, nil), `Invalid metadata.name`)
	want(t, ValidateObjectMeta(ctx, deployment("  name: d\n  namespace: web\n"), false, nil), `Forbidden metadata.namespace`)
	want(t, ValidateObjectMeta(ctx, deployment("  name: d\n  annotations:\n    'bad key!': x\n"), true, nil), `Invalid metadata.annotations`)
	want(t, ValidateObjectMeta(ctx, object(t, "apiVersion: apps/v1\nkind: Deployment\nmetadata: nope\n"), true, nil), `Invalid metadata`)
	want(t, ValidateObjectMeta(ctx, deployment("  name: d\n  labels: nope\n"), true, nil), `Invalid metadata`)
	want(t, ValidateObjectMeta(ctx, object(t, "apiVersion: apps/v1\nkind: Deployment\n"), true, nil), `Required metadata.name`)
	// the server generates the name before it validates
	want(t, ValidateObjectMeta(ctx, deployment("  generateName: d-\n"), true, nil))
	// the kind's own name rule is the caller's: here one that refuses "d"
	noD := func(name string, _ bool) []string {
		if name == "d" {
			return []string{"may not be d"}
		}
		return nil
	}
	want(t, ValidateObjectMeta(ctx, deployment("  name: d\n"), true, noD), `Invalid metadata.name: may not be d`)
	want(t, ValidateObjectMeta(ctx, deployment("  name: not:a:subdomain\n"), true, noD))
}
