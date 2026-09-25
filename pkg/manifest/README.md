# Manifest Classification

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/manifest.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/manifest)

The `manifest` package provides shared classification of Kubernetes manifests:
recognizing `CustomResourceDefinition`s and determining the namespacing (scope) of
an arbitrary object. It exists so that independent consumers — a staging engine
that must order CRDs ahead of the resources that depend on them, and source
components that emit and namespace-stamp raw manifests — classify objects
identically rather than maintaining divergent copies.

Besides Kubernetes API machinery (`k8s.io/apiextensions-apiserver`,
`k8s.io/apimachinery`, `sigs.k8s.io/controller-runtime`), the package depends on
exactly one other Kure package: `pkg/kubernetes`, for the generated scope table
`Scope` consults. That dependency replaced two scope maps kept by hand here, which
had to be edited whenever the scheme gained a kind and were silently wrong until
someone noticed. It is one-directional and data-only — this package calls a lookup
and reads a bool; nothing in `pkg/kubernetes` imports `manifest`.

Each Go block on this page is the body of an `Example` function in `example_test.go`, which
`go test` runs: it imports this package as `manifest`,
`apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"`,
`k8s.io/apimachinery/pkg/apis/meta/v1/unstructured`, `k8s.io/apimachinery/pkg/runtime/schema`,
and `fmt` for the line that prints what the example found.

## CRD recognition

`IsCRD` reports whether an object is a `CustomResourceDefinition`, by type or GVK,
without requiring `spec.group`/`spec.names` to be populated. `CRDDefinedGroupKind`
returns the `GroupKind` a CRD defines (`spec.group` + `spec.names.kind`), and
`CRDScope` additionally returns its declared scope, defaulting to `NamespaceScoped`
when `spec.scope` is absent (matching Kubernetes):

<!-- doc-example: pkg/manifest ExampleCRDScope -->
```go
obj := &apiextv1.CustomResourceDefinition{}
obj.SetName("widgets.example.com")
obj.Spec.Group = "example.com"
obj.Spec.Names.Kind = "Widget"
obj.Spec.Scope = apiextv1.ClusterScoped

gk, scope, ok := manifest.CRDScope(obj)
fmt.Println(gk, scope, ok)
```
<!-- doc-example:end -->

## Scope determination

`Scope` determines whether an object is namespaced, cluster-scoped, or unknown, in
this order:

1. A `CustomResourceDefinition` is cluster-scoped.
2. A **built-in** kind takes its scope from the generated table
   (`ScopeSourceBuiltin`), derived from the pinned upstream sources. See
   [Kubernetes Builders](/api-reference/kubernetes-builders/) § 9. The lookup
   matches on group and kind only: a manifest at a version kure does not register
   (`autoscaling/v1` where the scheme has `autoscaling/v2`) is still answered,
   because a resource has one scope across all its versions.
3. A short residual list covers the cluster-scoped kinds kure does **not** register
   and so cannot derive: `PriorityClass`, `APIService` and the two webhook
   configurations. These are built-ins too. It only shrinks — registering one of
   them moves it to the derived table, and a test fires so the entry is removed
   rather than left as a second, competing answer.
4. A **custom resource** takes its scope from the caller-supplied map of CRD scopes
   (the `spec.scope` of CRDs known in the same context) when that map defines it —
   including for a kind kure registers. The CRD in scope names the scope the target
   cluster will actually serve; the generated table only records what the module
   pinned at build time declared, so a consumer shipping a different release of
   that operator is entitled to differ. Steps 2 and 3 are the other way round on
   purpose: the Kubernetes API defines a built-in's scope and no manifest can
   redefine it.
5. Failing that, a kind kure registers falls back to the generated table.

Anything else is `ScopeUnknown` — callers are expected to fail closed rather than
guess:

<!-- doc-example: pkg/manifest ExampleScope -->
```go
obj := &unstructured.Unstructured{Object: map[string]any{
    "apiVersion": "example.com/v1",
    "kind":       "Widget",
    "metadata":   map[string]any{"name": "my-widget"},
}}

// The spec.scope of the CRDs known in the same context
crdScopes := map[schema.GroupKind]apiextv1.ResourceScope{
    {Group: "example.com", Kind: "Widget"}: apiextv1.NamespaceScoped,
}

switch manifest.Scope(obj, crdScopes) {
case manifest.ScopeNamespaced:
    // must declare metadata.namespace
    fmt.Println("namespaced")
case manifest.ScopeCluster:
    // cluster-scoped
    fmt.Println("cluster-scoped")
case manifest.ScopeUnknown:
    // unknown custom resource with no defining CRD in scope
    fmt.Println("unknown")
}
```
<!-- doc-example:end -->

## API overview

| Function | Purpose |
|----------|---------|
| `IsCRD(o)` | Report whether an object is a `CustomResourceDefinition` (by type or GVK). |
| `CRDDefinedGroupKind(o)` | The `GroupKind` a CRD defines, and whether `o` is a CRD. |
| `CRDScope(o)` | A CRD's defined `GroupKind` and declared scope (defaults to namespaced). |
| `ObjectGroupKind(o)` | The `GroupKind` of an emitted object. |
| `Scope(o, crdScopes)` | Classify an object as namespaced, cluster-scoped, or unknown. |
| `IsNamespacedKind(apiVersion, kind)` | Whether a kind kure registers is namespaced, built-in or custom resource alike. Answered from the generated table; version-insensitive. An unregistered kind answers `false`, which means "not known to be namespaced", never "cluster-scoped" — use `Scope` to tell those apart. |
| `IsNamespacedBuiltinKind(apiVersion, kind)` | The same question restricted to built-ins: a kind whose scope the Kubernetes API itself defines, which the generated table records as `ScopeSourceBuiltin`. A CRD kind answers `false` whatever its scope. |
