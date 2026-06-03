// Package manifest provides shared classification of Kubernetes manifests:
// recognizing CustomResourceDefinitions and determining the namespacing
// (scope) of an arbitrary object. It exists so that independent consumers —
// a staging engine that must order CRDs ahead of the resources that depend on
// them, and source components that emit and namespace-stamp raw manifests —
// classify objects identically rather than maintaining divergent copies.
//
// # CRD recognition
//
// IsCRD reports whether an object is a CustomResourceDefinition by type or GVK,
// without requiring spec.group/spec.names to be populated. CRDDefinedGroupKind
// returns the GroupKind a CRD defines (spec.group + spec.names.kind), and
// CRDScope additionally returns its declared scope, defaulting to
// NamespaceScoped when spec.scope is absent (matching Kubernetes):
//
//	gk, scope, ok := manifest.CRDScope(obj)
//
// # Scope determination
//
// Scope determines whether an object is namespaced, cluster-scoped, or unknown.
// CRDs are cluster-scoped; well-known built-in kinds are resolved from internal
// namespaced/cluster maps; custom resources are resolved from a caller-supplied
// map of CRD scopes (the spec.scope of CRDs known in the same context).
// Anything else is ScopeUnknown — callers are expected to fail closed rather
// than guess:
//
//	switch manifest.Scope(obj, crdScopes) {
//	case manifest.ScopeNamespaced:
//	    // must declare metadata.namespace
//	case manifest.ScopeCluster:
//	    // cluster-scoped
//	case manifest.ScopeUnknown:
//	    // unknown custom resource with no defining CRD in scope
//	}
//
// The package depends only on the Kubernetes API machinery
// (k8s.io/apiextensions-apiserver, k8s.io/apimachinery,
// sigs.k8s.io/controller-runtime) and has no dependency on any other Kure
// package.
package manifest
