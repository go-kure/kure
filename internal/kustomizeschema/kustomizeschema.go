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
// kyaml's API leaves an entry. It does not cover code that registers a
// definition and then deletes its entry by hand: kyaml's index by resource
// type is unexported, still serves the schema, and hiding a registration that
// way is deliberate concealment, which no check in the same process can stop.
package kustomizeschema

import (
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
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
const builtInDefinitions = "sha256:2c1a479454ccf7295f025e058bf2844292d05eefe187206b84d064b32115b894"

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
// the SHA-256 of a walk over every field of them, and whether it is the
// built-in schema's (fingerprint). Any definition added, replaced or edited
// in place (openapi.AddSchema, openapi.AddDefinitions, a write through
// openapi.Schema()) changes it, and so does one removed, or a value moved to
// another field, such as an extension to ExtraProps, which kyaml does not
// read and which marshals to the same JSON; a definition equal to the
// built-in one does not. It parses the schema first, as Parse does, and
// costs a walk of it, under 100 ms for the 623 definitions of kyaml
// v0.21.1, so a caller asks once per run of builds, not once per build. It
// does not read the version, so a custom schema is told by BuiltIn.
func Definitions() (string, bool) {
	got := "sha256:" + fingerprint(openapi.Schema().Definitions)
	return got, got == builtInDefinitions
}
