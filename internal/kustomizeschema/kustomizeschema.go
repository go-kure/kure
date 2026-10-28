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
// Build does both).
package kustomizeschema

import (
	"sigs.k8s.io/kustomize/kyaml/openapi"
	"sigs.k8s.io/kustomize/kyaml/openapi/kubernetesapi"
)

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
// unchanged and are not detected.
func BuiltIn() (string, bool) {
	v := openapi.GetSchemaVersion()
	return v, v == kubernetesapi.DefaultOpenAPI
}
