# Manifest Classification

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/manifest.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/manifest)

The `manifest` package provides shared classification of Kubernetes manifests:
recognizing `CustomResourceDefinition`s and determining the namespacing (scope) of
an arbitrary object. It exists so that independent consumers — a staging engine
that must order CRDs ahead of the resources that depend on them, and source
components that emit and namespace-stamp raw manifests — classify objects
identically rather than maintaining divergent copies.

The package depends only on Kubernetes API machinery
(`k8s.io/apiextensions-apiserver`, `k8s.io/apimachinery`,
`sigs.k8s.io/controller-runtime`) and has no dependency on any other Kure package.

## CRD recognition

`IsCRD` reports whether an object is a `CustomResourceDefinition`, by type or GVK,
without requiring `spec.group`/`spec.names` to be populated. `CRDDefinedGroupKind`
returns the `GroupKind` a CRD defines (`spec.group` + `spec.names.kind`), and
`CRDScope` additionally returns its declared scope, defaulting to `NamespaceScoped`
when `spec.scope` is absent (matching Kubernetes):

```go
gk, scope, ok := manifest.CRDScope(obj)
```

## Scope determination

`Scope` determines whether an object is namespaced, cluster-scoped, or unknown.
CRDs are cluster-scoped; well-known built-in kinds are resolved from internal
namespaced/cluster maps; custom resources are resolved from a caller-supplied map
of CRD scopes (the `spec.scope` of CRDs known in the same context). Anything else
is `ScopeUnknown` — callers are expected to fail closed rather than guess:

```go
switch manifest.Scope(obj, crdScopes) {
case manifest.ScopeNamespaced:
    // must declare metadata.namespace
case manifest.ScopeCluster:
    // cluster-scoped
case manifest.ScopeUnknown:
    // unknown custom resource with no defining CRD in scope
}
```

## API overview

| Function | Purpose |
|----------|---------|
| `IsCRD(o)` | Report whether an object is a `CustomResourceDefinition` (by type or GVK). |
| `CRDDefinedGroupKind(o)` | The `GroupKind` a CRD defines, and whether `o` is a CRD. |
| `CRDScope(o)` | A CRD's defined `GroupKind` and declared scope (defaults to namespaced). |
| `ObjectGroupKind(o)` | The `GroupKind` of an emitted object. |
| `Scope(o, crdScopes)` | Classify an object as namespaced, cluster-scoped, or unknown. |
| `IsNamespacedBuiltinKind(apiVersion, kind)` | Whether a built-in kind is a known namespaced type. |
