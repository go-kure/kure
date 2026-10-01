// Package crdvalidate validates custom resources the way the Kubernetes API
// server validates a create request, in process, against the
// CustomResourceDefinitions it is given and nothing else.
//
// A [Validator] holds one definition per group and kind. For an object it
// runs the server's own routines in the server's own order, as
// k8s.io/apiextensions-apiserver v0.37.1 runs them for a create:
//
//  1. the decode-time coercion, with strict field validation: metadata is
//     decoded, unknown fields are pruned and reported, non-nullable nulls
//     without a default are dropped, and embedded metadata is coerced
//     (pkg/apiserver/customresource_handler.go, unstructuredSchemaCoercer);
//  2. defaulting from the schema (unstructuredDefaulter);
//  3. the request namespace rule (k8s.io/apiserver/pkg/registry/rest
//     BeforeCreate) — see [Validator.Validate] for the one place this
//     package is stricter than the server;
//  4. PrepareForCreate: status is dropped when the definition serves a status
//     subresource, and the generation is set
//     (pkg/registry/customresource/strategy.go);
//  5. Validate: the schema, ObjectMeta, scale paths, embedded ObjectMeta,
//     list set and map invariants, and the x-kubernetes-validations CEL
//     rules unless a blocking schema error already stands (strategy.go,
//     validator.go).
//
// Each served version is compiled on first use as the server compiles it
// when it starts serving the definition: converted to the internal schema,
// made structural, defaults pruned, an OpenAPI validator and a CEL validator
// built from it.
//
// What this cannot know: admission webhooks, the apiserver's per-kind Go
// validation for built-in kinds, and versions the definition does not serve.
package crdvalidate

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"

	"k8s.io/apiextensions-apiserver/pkg/apihelpers"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	structuralschema "k8s.io/apiextensions-apiserver/pkg/apiserver/schema"
	"k8s.io/apiextensions-apiserver/pkg/apiserver/schema/cel"
	structuraldefaulting "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/defaulting"
	structurallisttype "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/listtype"
	schemaobjectmeta "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/objectmeta"
	structuralpruning "k8s.io/apiextensions-apiserver/pkg/apiserver/schema/pruning"
	apiservervalidation "k8s.io/apiextensions-apiserver/pkg/apiserver/validation"
	"k8s.io/apimachinery/pkg/api/operation"
	"k8s.io/apimachinery/pkg/api/validation"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation/field"
	celconfig "k8s.io/apiserver/pkg/apis/cel"
	"k8s.io/apiserver/pkg/features"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
)

// Validator validates objects against a fixed set of definitions.
type Validator struct {
	byKind map[schema.GroupKind]*definition
}

// definition is one CustomResourceDefinition and its compiled versions.
type definition struct {
	crd *apiextensionsv1.CustomResourceDefinition

	mu       sync.Mutex
	versions map[string]*compiled
}

// compiled is one served version, prepared as the server prepares it when it
// starts serving the definition.
type compiled struct {
	structural      *structuralschema.Structural
	schemaValidator apiservervalidation.SchemaValidator
	// cel is nil when the schema carries no x-kubernetes-validations.
	cel *cel.Validator
	// status reports whether the version serves a status subresource.
	status bool
	scale  *apiextensions.CustomResourceSubresourceScale
}

// New indexes the definitions by group and kind. Two definitions for one
// kind are an error, not a merge: the caller decides which one is canonical.
func New(defs []*apiextensionsv1.CustomResourceDefinition) (*Validator, error) {
	v := &Validator{byKind: map[schema.GroupKind]*definition{}}
	for _, crd := range defs {
		gk := schema.GroupKind{Group: crd.Spec.Group, Kind: crd.Spec.Names.Kind}
		if gk.Group == "" || gk.Kind == "" {
			return nil, fmt.Errorf("crdvalidate: definition %q names no group or kind", crd.Name)
		}
		if prev, dup := v.byKind[gk]; dup {
			return nil, fmt.Errorf("crdvalidate: %s is defined twice, by %s and %s", gk, prev.crd.Name, crd.Name)
		}
		v.byKind[gk] = &definition{crd: crd, versions: map[string]*compiled{}}
	}
	return v, nil
}

// Definition returns the definition held for a kind, or nil.
func (v *Validator) Definition(gk schema.GroupKind) *apiextensionsv1.CustomResourceDefinition {
	if def := v.byKind[gk]; def != nil {
		return def.crd
	}
	return nil
}

// Compile prepares the version that objects of gvk are validated against and
// returns what stops it: no definition for the kind, a version the definition
// does not serve, or a schema the server would not serve either.
func (v *Validator) Compile(gvk schema.GroupVersionKind) error {
	def, err := v.definition(gvk)
	if err != nil {
		return err
	}
	_, err = def.compile(gvk.Version)
	return err
}

// Validate validates obj as the server validates a create request for it,
// on a copy; obj is not changed. The list holds the object's errors. The
// error says the validator cannot answer for the object at all: no
// definition for its kind, or one whose schema the server would not serve.
//
// obj must be decoded as the server decodes a request body — through
// [unstructured.UnstructuredJSONScheme] or k8s.io/apimachinery/pkg/util/json,
// which read whole numbers as int64. An integer held as float64 (the
// encoding/json default) is a shape the server never sees, and the scale
// paths and CEL rules report it as a type error.
//
// A namespaced object with no metadata.namespace is validated in the request
// namespace, as kubectl submits it: the server fills "default" in before
// validating. A namespace on a cluster-scoped object is reported as
// forbidden; the server would drop it without a word, and an object that
// carries one is a defect in whatever produced it.
func (v *Validator) Validate(ctx context.Context, obj *unstructured.Unstructured) (field.ErrorList, error) {
	gvk := obj.GroupVersionKind()
	def, err := v.definition(gvk)
	if err != nil {
		return nil, err
	}
	if !apihelpers.HasVersionServed(def.crd, gvk.Version) {
		return field.ErrorList{field.NotSupported(field.NewPath("apiVersion"), obj.GetAPIVersion(), def.servedAPIVersions())}, nil
	}
	c, err := def.compile(gvk.Version)
	if err != nil {
		return nil, err
	}
	namespaced := def.crd.Spec.Scope == apiextensionsv1.NamespaceScoped

	u := obj.DeepCopy()
	unknown, errs := coerce(u, c.structural)
	if len(errs) > 0 {
		return errs, nil
	}
	if len(unknown) > 0 {
		// The server answers a strict request with the unknown fields alone;
		// it never reaches validation.
		for _, path := range unknown {
			errs = append(errs, field.Forbidden(field.NewPath(path), "unknown field"))
		}
		return errs, nil
	}
	structuraldefaulting.Default(u.Object, c.structural)
	requestNamespace(u, namespaced)
	if c.status {
		delete(u.Object, "status")
	}
	u.SetGeneration(1)

	errs = validateResource(ctx, u, c, namespaced)
	errs = append(errs, schemaobjectmeta.Validate(ctx, nil, u.Object, c.structural, false)...)
	errs = append(errs, structurallisttype.ValidateListSetsAndMaps(nil, c.structural, u.Object)...)
	if c.cel != nil {
		if has, err := hasBlockingErr(errs); has {
			errs = append(errs, err)
		} else {
			celErrs, _ := c.cel.Validate(ctx, nil, c.structural, u.Object, nil, celconfig.RuntimeCELCostBudget)
			errs = append(errs, celErrs...)
		}
	}
	return errs, nil
}

// ValidateObjectMeta validates obj's metadata as the server validates it on
// create, for a kind this package holds no definition for. namespaced says
// whether the kind takes a namespace; the request namespace rule of
// [Validator.Validate] applies.
func ValidateObjectMeta(ctx context.Context, obj *unstructured.Unstructured, namespaced bool) field.ErrorList {
	meta, errs := objectMeta(obj)
	if len(errs) > 0 {
		return errs
	}
	requestNamespace(meta, namespaced)
	return validateObjectMeta(ctx, meta, namespaced)
}

func (v *Validator) definition(gvk schema.GroupVersionKind) (*definition, error) {
	def := v.byKind[gvk.GroupKind()]
	if def == nil {
		return nil, fmt.Errorf("crdvalidate: no definition for %s", gvk.GroupKind())
	}
	return def, nil
}

func (d *definition) servedAPIVersions() []string {
	var out []string
	for _, ver := range d.crd.Spec.Versions {
		if ver.Served {
			out = append(out, d.crd.Spec.Group+"/"+ver.Name)
		}
	}
	sort.Strings(out)
	return out
}

// compile prepares a version once; later calls return the same compilation.
func (d *definition) compile(version string) (*compiled, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if c, ok := d.versions[version]; ok {
		return c, nil
	}
	c, err := newCompiled(d.crd, version)
	if err != nil {
		return nil, err
	}
	d.versions[version] = c
	return c, nil
}

// newCompiled follows pkg/apiserver/customresource_handler.go, the
// structuralSchemas loop and the per-version storage setup.
func newCompiled(crd *apiextensionsv1.CustomResourceDefinition, version string) (*compiled, error) {
	if !apihelpers.HasVersionServed(crd, version) {
		return nil, fmt.Errorf("crdvalidate: %s does not serve %s", crd.Name, version)
	}
	val, err := apihelpers.GetSchemaForVersion(crd, version)
	if err != nil {
		return nil, fmt.Errorf("crdvalidate: %w", err)
	}
	if val == nil || val.OpenAPIV3Schema == nil {
		return nil, fmt.Errorf("crdvalidate: %s %s has no schema", crd.Name, version)
	}
	if crd.Spec.PreserveUnknownFields {
		return nil, fmt.Errorf("crdvalidate: %s preserves unknown fields, which a v1 definition cannot", crd.Name)
	}
	internal := &apiextensions.CustomResourceValidation{}
	if err := apiextensionsv1.Convert_v1_CustomResourceValidation_To_apiextensions_CustomResourceValidation(val, internal, nil); err != nil {
		return nil, fmt.Errorf("crdvalidate: %s %s: convert schema: %w", crd.Name, version, err)
	}
	s, err := structuralschema.NewStructural(internal.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("crdvalidate: %s %s: schema is not structural: %w", crd.Name, version, err)
	}
	if errs := structuralschema.ValidateStructural(nil, s); len(errs) > 0 {
		return nil, fmt.Errorf("crdvalidate: %s %s: schema is not structural: %w", crd.Name, version, errs.ToAggregate())
	}
	s = s.DeepCopy()
	if err := structuraldefaulting.PruneDefaults(s); err != nil {
		return nil, fmt.Errorf("crdvalidate: %s %s: prune defaults: %w", crd.Name, version, err)
	}
	schemaValidator, _, err := apiservervalidation.NewSchemaValidator(internal.OpenAPIV3Schema)
	if err != nil {
		return nil, fmt.Errorf("crdvalidate: %s %s: schema validator: %w", crd.Name, version, err)
	}
	c := &compiled{
		structural:      s,
		schemaValidator: schemaValidator,
		cel:             cel.NewValidator(s, true, celconfig.PerCallLimit),
	}
	sub, err := apihelpers.GetSubresourcesForVersion(crd, version)
	if err != nil {
		return nil, fmt.Errorf("crdvalidate: %w", err)
	}
	if sub != nil {
		c.status = sub.Status != nil
		if sub.Scale != nil {
			c.scale = &apiextensions.CustomResourceSubresourceScale{}
			if err := apiextensionsv1.Convert_v1_CustomResourceSubresourceScale_To_apiextensions_CustomResourceSubresourceScale(sub.Scale, c.scale, nil); err != nil {
				return nil, fmt.Errorf("crdvalidate: %s %s: convert scale subresource: %w", crd.Name, version, err)
			}
		}
	}
	return c, nil
}

// coerce is the decode-time pass (customresource_handler.go,
// unstructuredSchemaCoercer.apply) with strict field validation on and
// malformed metadata kept, so it is reported rather than dropped. It returns
// the paths of the unknown fields it pruned.
func coerce(u *unstructured.Unstructured, s *structuralschema.Structural) ([]string, field.ErrorList) {
	kind, foundKind, err := unstructured.NestedString(u.Object, "kind")
	if err != nil {
		return nil, field.ErrorList{field.Invalid(field.NewPath("kind"), u.Object["kind"], err.Error())}
	}
	apiVersion, foundAPIVersion, err := unstructured.NestedString(u.Object, "apiVersion")
	if err != nil {
		return nil, field.ErrorList{field.Invalid(field.NewPath("apiVersion"), u.Object["apiVersion"], err.Error())}
	}
	meta, foundMeta, unknown, err := schemaobjectmeta.GetObjectMetaWithOptions(u.Object, schemaobjectmeta.ObjectMetaOptions{ReturnUnknownFieldPaths: true})
	if err != nil {
		return nil, field.ErrorList{field.Invalid(field.NewPath("metadata"), u.Object["metadata"], err.Error())}
	}
	unknown = append(unknown, structuralpruning.PruneWithOptions(u.Object, s, true, structuralschema.UnknownFieldPathOptions{TrackUnknownFieldPaths: true})...)
	structuraldefaulting.PruneNonNullableNullsWithoutDefaults(u.Object, s)
	ferr, paths := schemaobjectmeta.CoerceWithOptions(nil, u.Object, s, false, schemaobjectmeta.CoerceOptions{ReturnUnknownFieldPaths: true})
	if ferr != nil {
		return nil, field.ErrorList{ferr}
	}
	unknown = append(unknown, paths...)

	// restore the fields the schema does not describe, as the server does
	if foundKind {
		u.SetKind(kind)
	}
	if foundAPIVersion {
		u.SetAPIVersion(apiVersion)
	}
	if foundMeta {
		if err := schemaobjectmeta.SetObjectMeta(u.Object, meta); err != nil {
			return nil, field.ErrorList{field.Invalid(field.NewPath("metadata"), u.Object["metadata"], err.Error())}
		}
	}
	return unknown, nil
}

// requestNamespace is rest.BeforeCreate's namespace rule for a request sent
// the way kubectl sends one: a namespaced object with no namespace lands in
// the default namespace. A namespace on a cluster-scoped object is left for
// ObjectMeta validation to forbid.
func requestNamespace(obj metav1.Object, namespaced bool) {
	if namespaced && obj.GetNamespace() == "" {
		obj.SetNamespace(metav1.NamespaceDefault)
	}
}

// validateResource is customResourceValidator.Validate
// (pkg/registry/customresource/validator.go). The TypeMeta check it opens
// with cannot fail here: the definition and version were found by the very
// kind and apiVersion it would compare against.
func validateResource(ctx context.Context, u *unstructured.Unstructured, c *compiled, namespaced bool) field.ErrorList {
	meta, errs := objectMeta(u)
	if len(errs) > 0 {
		return errs
	}
	errs = validateObjectMeta(ctx, meta, namespaced)
	errs = append(errs, apiservervalidation.ValidateCustomResource(nil, u.Object, c.schemaValidator)...)
	errs = append(errs, validateScale(u, c.scale)...)
	return errs
}

// objectMeta is validator.go's getObjectMeta.
func objectMeta(u *unstructured.Unstructured) (*metav1.ObjectMeta, field.ErrorList) {
	raw := u.Object["metadata"]
	if raw == nil {
		return &metav1.ObjectMeta{}, nil
	}
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, field.ErrorList{field.Invalid(field.NewPath("metadata"), raw, fmt.Sprintf("expected map[string]interface{}, got %T", raw))}
	}
	meta := &metav1.ObjectMeta{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(m, meta); err != nil {
		return nil, field.ErrorList{field.Invalid(field.NewPath("metadata"), raw, err.Error())}
	}
	return meta, nil
}

// validateObjectMeta is validator.go's validateObjectMetaDeclaratively on
// create, under the same feature gate the server reads.
func validateObjectMeta(ctx context.Context, meta *metav1.ObjectMeta, namespaced bool) field.ErrorList {
	return validation.ValidateObjectMetaDeclaratively(ctx, operation.Create, meta, nil, namespaced, validation.NameIsDNSSubdomain, field.NewPath("metadata"), utilfeature.DefaultFeatureGate.Enabled(features.DeclarativeValidationBeta))
}

// validateScale is validator.go's ValidateScaleSpec and ValidateScaleStatus.
func validateScale(u *unstructured.Unstructured, scale *apiextensions.CustomResourceSubresourceScale) field.ErrorList {
	if scale == nil {
		return nil
	}
	var errs field.ErrorList
	for _, path := range []string{scale.SpecReplicasPath, scale.StatusReplicasPath} {
		replicas, _, err := unstructured.NestedInt64(u.Object, strings.Split(strings.TrimPrefix(path, "."), ".")...)
		switch {
		case err != nil:
			errs = append(errs, field.Invalid(field.NewPath(path), replicas, err.Error()))
		case replicas < 0:
			errs = append(errs, field.Invalid(field.NewPath(path), replicas, "should be a non-negative integer"))
		case replicas > math.MaxInt32:
			errs = append(errs, field.Invalid(field.NewPath(path), replicas, fmt.Sprintf("should be less than or equal to %v", math.MaxInt32)))
		}
	}
	if scale.LabelSelectorPath != nil {
		path := *scale.LabelSelectorPath
		selector, _, err := unstructured.NestedString(u.Object, strings.Split(strings.TrimPrefix(path, "."), ".")...)
		if err != nil {
			errs = append(errs, field.Invalid(field.NewPath(path), selector, err.Error()))
		}
	}
	return errs
}

// hasBlockingErr is strategy.go's: a type, required, enum or size violation
// means the object does not have the shape the CEL rules were written for.
func hasBlockingErr(errs field.ErrorList) (bool, *field.Error) {
	for _, err := range errs {
		if err.Type == field.ErrorTypeNotSupported || err.Type == field.ErrorTypeRequired || err.Type == field.ErrorTypeTooLong || err.Type == field.ErrorTypeTooMany || err.Type == field.ErrorTypeTypeInvalid {
			return true, field.Invalid(nil, nil, "some validation rules were not checked because the object was invalid; correct the existing errors to complete validation")
		}
	}
	return false, nil
}
