+++
title = "Using Kure as a Library"
weight = 10
+++

# Using Kure as a Library

Kure is primarily a Go library. This guide covers the basics of importing it, creating resources, and generating YAML output.

## Installation

```bash
go get github.com/go-kure/kure
```

## Creating Resources

A constructor gives you an object with an identity and nothing else: its
`apiVersion` and `kind` from the scheme, its `metadata.name`, and its
`metadata.namespace` for a namespaced kind. From there the upstream Go struct is
the API, so you set fields on it directly.

```go
import (
    appsv1 "k8s.io/api/apps/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/utils/ptr"
    "github.com/go-kure/kure/pkg/kubernetes"
)

dep := kubernetes.CreateDeployment("web", "default")   // identity only
dep.Spec.Replicas = ptr.To[int32](3)
dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
dep.Spec.Template.Spec.ServiceAccountName = "web"
```

`kubernetes.Create[appsv1.Deployment]("web", "default")` is the generic form;
the per-kind wrappers are generated from the scheme and carry the scope in
their signature, so a cluster-scoped kind takes only a name
(`kubernetes.CreateNamespace("platform")`).

Kure adds a helper only for one of a few write shapes: appending to a list,
inserting into a map, setting a pointer field, or composing a small upstream
struct. A helper never defaults, never validates, and never touches a field you
did not name.

```go
kubernetes.AddLabel(dep, "tier", "frontend")       // works on any kind
kubernetes.SetDeploymentReplicas(dep, 3)
```

The [Kubernetes Builders](/api-reference/kubernetes-builders) page is the
normative contract: what constructors emit, which helpers exist and why, and
the [migration notes](/concepts/builder-contract-release-1/) for the
constructor defaults that earlier releases injected and no longer do. If you
upgraded and a field you relied on is now empty, that page lists it.

### FluxCD Resources

```go
import (
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
    sourcev1 "github.com/fluxcd/source-controller/api/v1"
    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
)

// Create a GitRepository source
repo := fluxcd.CreateGitRepository("my-repo", "flux-system")
repo.Spec.URL = "https://github.com/org/repo"
fluxcd.SetGitRepositoryReference(repo, &sourcev1.GitRepositoryRef{Branch: "main"})
repo.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}

// Create a Kustomization that references the source
ks := fluxcd.CreateKustomization("my-app", "flux-system")
ks.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
    Kind: "GitRepository",
    Name: "my-repo",
}
ks.Spec.Path = "./clusters/production"
ks.Spec.Interval = metav1.Duration{Duration: 10 * time.Minute}
ks.Spec.Prune = true
```

See the [FluxCD Builders reference](/api-reference/fluxcd-builders) for all available resource types.

Beyond FluxCD, the [Kubernetes Builders](/api-reference/kubernetes-builders) package provides typed constructors for core resources (Deployment, Service, Ingress, CronJob, NetworkPolicy, HTTPRoute), PSA security context helpers, ResourceRequirements builders, and more. The [Prometheus Builders](/api-reference/prometheus-builders) sub-package covers ServiceMonitor, PodMonitor, and PrometheusRule CRDs.

## Generating YAML

Use the `io` package to serialize resources:

```go
import "github.com/go-kure/kure/pkg/io"

// Serialize a single object
data, err := io.Marshal(deployment)

// Write multiple objects to stdout as YAML
err := io.PrintObjectsAsYAML(objects, os.Stdout)

// Save to file
err := io.SaveFile("output.yaml", deployment)
```

### Clean YAML encoding

When encoding resources exported from a cluster, server-managed metadata fields (`managedFields`, `resourceVersion`, `uid`, etc.) clutter the output. The default encoding strips all of these automatically:

```go
// Default: strips all server-set fields and uses standard key order
data, err := io.EncodeObjectsToYAMLWithOptions(objects, io.EncodeOptions{
    KubernetesFieldOrder: true,
})
```

Use `ServerFieldStripping` to control the level of stripping:

```go
// Preserve server fields (e.g. for debugging)
data, err := io.EncodeObjectsToYAMLWithOptions(objects, io.EncodeOptions{
    ServerFieldStripping: io.StripServerFieldsNone,
})
```

See the [IO reference](/api-reference/io) for all output formats and stripping options.

## Asking What Kure Knows About a Kind

Every kind kure registers is described in a table generated from the pinned upstream
module sources — its scope, the module and version it came from, and the fields
upstream documents as gated, alpha, beta or deprecated. The lookups are in
`pkg/kubernetes`, and they answer from that table rather than from a hand-kept list.

```go
import "github.com/go-kure/kure/pkg/kubernetes"

k, ok := kubernetes.KindFor("apps/v1", "Deployment")   // exact group/version/kind
if ok {
    fmt.Println(k.Namespaced, k.Module, k.ModuleVersion, k.ScopeSource)
}

namespaced, known := kubernetes.IsNamespaced("autoscaling/v1", "HorizontalPodAutoscaler")
```

The two answer deliberately different questions. `KindFor` returns the row for one
exact `group/version/kind`, because `GoType`, `ImportPath` and `ModuleVersion` are
properties of that version. `IsNamespaced` matches on group and kind only: scope is a
property of the resource, not of the version, so it still answers for a manifest at a
version kure does not register — `autoscaling/v1` above, where the scheme carries
`autoscaling/v2`. `KindForAnyVersion` is that same version-insensitive match
returning the whole row. `pkg/manifest`'s `Scope` builds on the version-insensitive
form, which is why a manifest at an unregistered version is classified rather than
left unknown.

`ScopeSource` says what declared the scope, and distinguishes a built-in kind from a
custom resource without keeping a list:

```go
k, _ := kubernetes.KindForAnyVersion("cilium.io/v2", "CiliumNetworkPolicy")
k.ScopeSource == kubernetes.ScopeSourceBuiltin   // false: declared by a marker

// pkg/manifest asks both forms of the scope question:
manifest.IsNamespacedKind("cilium.io/v2", "CiliumNetworkPolicy")        // true
manifest.IsNamespacedBuiltinKind("cilium.io/v2", "CiliumNetworkPolicy") // false, not a built-in
```

The three sources are `ScopeSourceMarker` (the kind's own `+kubebuilder:resource`
marker), `ScopeSourceShippedCRD` (a `CustomResourceDefinition` the module ships) and
`ScopeSourceBuiltin` (the Kubernetes API itself). Only the last is a built-in, and
that distinction decides who wins in `manifest.Scope`: a built-in's scope comes from
the table, while for a custom resource a `CustomResourceDefinition` in the same
context governs — it names the scope the target cluster will serve, where the table
only records what the pinned module declared at build time.

Maturity is reported, never enforced:

```go
for _, f := range kubernetes.MaturityForType("k8s.io/api/core/v1", "PodSpec") {
    fmt.Println(f.Field, f.Stability, f.Gates)
}
gated := kubernetes.GatedFields()   // every field behind a feature gate
```

kure does not warn, reject or filter on any of it — a consumer with cluster knowledge
decides. The table exists because the failure it describes is silent: the API server
does not reject a field whose feature gate is off, it clears the field and admits the
object, so the manifest reads as applied and is not. The same data is published as
`docs/api-tables.json` and [API Tables](/api-reference/api-tables) for readers outside
Go. See the [Kubernetes Builders](/api-reference/kubernetes-builders) page for what
each column means and how it is derived.

## Working with the Domain Model

For more complex scenarios, use the [Stack](/api-reference/stack) package to define cluster topologies:

```go
import "github.com/go-kure/kure/pkg/stack"

cluster := stack.NewClusterBuilder("production").
    WithNode("apps").
        WithBundle("web").
            WithApplication("frontend", frontendConfig).
        End().
    End().
    Build()
```

Then use the [Flux Engine](/api-reference/flux-engine) and [Layout Engine](/api-reference/layout) to generate a complete GitOps repository structure. See the [Generating Flux Manifests](/guides/flux-workflow/) guide for the full workflow.

## Error Handling

All Kure packages use the [errors](/api-reference/errors) package:

```go
import "github.com/go-kure/kure/pkg/errors"

if err != nil {
    return errors.Wrap(err, "failed to generate manifests")
}
```

## Next Steps

- [Generating Flux Manifests](/guides/flux-workflow/) for the complete workflow
- [API Reference](/api-reference) for all package documentation
- [Examples](/examples) for working code samples
