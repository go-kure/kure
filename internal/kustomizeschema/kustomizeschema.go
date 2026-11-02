// Package kustomizeschema parses kustomize's process-global OpenAPI schema
// before any of kure's uses of kustomize reads it. kyaml parses that
// schema lazily, the first time a strategic-merge patch or a build needs it
// (kyaml v0.21.1 openapi/openapi.go:197-199, :666-698), and writes its tables
// under its lock, while a scope lookup (resid.NewGvk, :403-410) reads its
// scope table after releasing the lock (:410). Two of kure's goroutines, one building and one looking up a
// scope, would then race on a cold start. Every kure use of kustomize calls
// Parse first, so the parse is done, by whichever came first, before any of
// them reads the schema. Parse asks kyaml every time rather than once: a
// reset (openapi.ResetOpenAPI, :308-314, which fluxcd/pkg/kustomize's Build
// defers) leaves the schema unparsed again, and the next kure use parses it.
//
// It orders only kure's own uses: another use of kustomize in the process
// that does not call Parse can still race with kyaml's parse, and one that
// sets or resets the schema races with every reader (fluxcd/pkg/kustomize's
// Build does both). What it can tell, from the schema as kyaml holds it, is
// whether the schema is still the built-in one: BuiltIn reads the version,
// Definitions the definitions (go-kure/kure#1166). Definitions covers what
// kyaml exposes in Schema().Definitions, where every registration through
// kyaml's API leaves an entry, and the schema kyaml's index by resource type
// serves for each kind those declare. It does not cover code that registers a
// definition and then deletes its entry by hand: the index is unexported,
// still serves the schema, and no definition declares its kind any more.
// Hiding a registration that way is deliberate concealment, which no check in
// the same process can stop.
package kustomizeschema

import (
	"sort"

	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

// builtInDefinitions is Definitions' fingerprint of kustomize's built-in
// schema, the Kubernetes definitions of kubernetesapi.DefaultOpenAPI and
// kustomize's own kustomization definitions, as kyaml parses them. It holds
// for the kyaml this module builds with and for the layout of the types the
// walk reads (kyaml's schema type is k8s.io/kube-openapi's spec.Schema, and
// the walk reads its fields by name and order): a bump of either that changes
// the definitions, or those types, fails TestDefinitions_BuiltInMatchesThePin,
// which prints the new value, and the bump's change sets it. A stale pin
// would refuse every build.
const builtInDefinitions = "sha256:293b16ad12a80ade01e9c4dbc10674e0b76444c8fb078127b59b08b7f2bf9844"

// Parse parses kustomize's schema unless it is parsed already; a concurrent
// caller waits until it is. Once parsed, it costs one lock.
func Parse() {
	_ = openapi.Schema()
}

// BuiltIn reports the schema kustomize uses, and whether it is its default
// built-in one, as kustomize-controller builds with it: a custom schema, or
// another built-in version, set by a kustomization's openapi field or by
// another use of kustomize in the process, holds for every later build until
// it is reset. It reads only the version: definitions added to the built-in
// schema (openapi.AddSchema, openapi.AddDefinitions, :302-350) leave it
// unchanged, and Definitions detects them.
func BuiltIn() (string, bool) {
	v := openapi.GetSchemaVersion()
	return v, v == kubernetesapi.DefaultOpenAPI
}

// Definitions returns a fingerprint of the definitions the schema holds now,
// the SHA-256 of a walk over every field of them and of the schemas kyaml's
// index serves, and whether it is the built-in schema's (fingerprint). Any
// definition added, replaced or edited
// in place (openapi.AddSchema, openapi.AddDefinitions, a write through
// openapi.Schema()) changes it, and so does one removed, or a value moved to
// another field, such as an extension to ExtraProps, which kyaml does not
// read and which marshals to the same JSON; a definition equal to the
// built-in one does not. It parses the schema first, as Parse does, and
// costs a walk of it, about 15 to 25 ms on a quiet host (about 0.1 s under the
// race detector) for the 623 definitions of kyaml v0.21.1, so a caller asks
// once per run of builds, not once per build. The index is read for each kind
// the definitions declare, since a write through the schema that
// openapi.SchemaForResourceType returns changes a build and leaves the
// definitions alone. It does not read the version, so a custom schema is told
// by BuiltIn.
func Definitions() (string, bool) {
	kinds, schemas := indexedSchemas()
	got := "sha256:" + fingerprint(state{Definitions: openapi.Schema().Definitions, Kinds: kinds, Schemas: schemas})
	return got, got == builtInDefinitions
}

// state is what Definitions fingerprints: the definitions, and the schemas
// kyaml's index by resource type serves for the kinds they declare.
type state struct {
	Definitions any
	Kinds       []indexedKind
	Schemas     []any
}

// indexedKind is a kind a definition declares, and which of state.Schemas is
// the schema kyaml serves for it, -1 if the index has none. Registering a
// definition indexes a struct copy of it, apart from the one in Definitions,
// and openapi.SchemaForResourceType returns a pointer to that copy, so a write
// through it (replacing its Properties) changes what a build reads and leaves
// Definitions as it was. A definition that declares many kinds (an options
// type has dozens) is indexed once, so its kinds share one entry of Schemas,
// each a *spec.Schema, which keeps the walk from reading it once per kind.
type indexedKind struct {
	APIVersion string
	Kind       string
	Schema     int
}

// gvkExtension is the extension a definition declares its kinds with, a list
// of maps holding a group, a version and a kind (kyaml openapi.go:586-588).
const gvkExtension = "x-kubernetes-group-version-kind"

// indexedSchemas returns, for every kind the definitions declare, in a fixed
// order, which of the returned schemas kyaml's index serves for it; the
// schemas are the distinct ones, in the order the kinds first name them. A
// kind is read as kyaml's index reads it (openapi.go:323-365); one it would not
// index is skipped. A schema the index holds for a kind no definition declares
// is not read: only code that hides its registration leaves one.
func indexedSchemas() ([]indexedKind, []any) {
	kinds := map[yaml.TypeMeta]bool{}
	for _, d := range openapi.Schema().Definitions {
		list, _ := d.Extensions[gvkExtension].([]any)
		for _, e := range list {
			m, _ := e.(map[string]any)
			version, vok := m["version"].(string)
			kind, kok := m["kind"].(string)
			if !vok || !kok {
				continue
			}
			if group, _ := m["group"].(string); group != "" {
				version = group + "/" + version
			}
			kinds[yaml.TypeMeta{APIVersion: version, Kind: kind}] = true
		}
	}
	sorted := make([]yaml.TypeMeta, 0, len(kinds))
	for tm := range kinds {
		sorted = append(sorted, tm)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].APIVersion != sorted[j].APIVersion {
			return sorted[i].APIVersion < sorted[j].APIVersion
		}
		return sorted[i].Kind < sorted[j].Kind
	})
	out := make([]indexedKind, 0, len(sorted))
	var schemas []any
	seen := map[any]int{}
	for _, tm := range sorted {
		k := indexedKind{APIVersion: tm.APIVersion, Kind: tm.Kind, Schema: -1}
		if rs := openapi.SchemaForResourceType(tm); rs != nil && rs.Schema != nil {
			i, ok := seen[rs.Schema]
			if !ok {
				i = len(schemas)
				seen[rs.Schema] = i
				schemas = append(schemas, rs.Schema)
			}
			k.Schema = i
		}
		out = append(out, k)
	}
	return out, schemas
}
