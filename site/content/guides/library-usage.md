+++
title = "Using Kure as a Library"
weight = 10
+++

# Using Kure as a Library

Kure is primarily a Go library. This guide covers the basics of importing it, creating resources, and generating YAML output.

Each Go block below that is not an import block is the body of an `Example` function in
`pkg/kubernetes/library_usage_example_test.go`, generated from it, so the page cannot show code
that no longer compiles. `go test` runs every one of them and compares what it prints, except the
maturity example, which is compiled only: its output follows the pinned upstream modules. Each
import block lists what the examples after it import, less `fmt`: the examples add `fmt.Println`
lines to print what they built. They use `panic(err)` where a program would return the error, and
two things declared in the same file: `frontendConfig`, a `stack.ApplicationConfig` that emits one
Deployment, and `exportedObjects()`, a ConfigMap carrying the `resourceVersion`, `uid` and
`creationTimestamp` an API server sets.

## Installation

```bash
go get github.com/go-kure/kure
```

## Creating Resources

A constructor gives you an object with an identity and nothing else: its
`apiVersion` and `kind` from the scheme, its `metadata.name`, and its
`metadata.namespace` for a namespaced kind. From there the upstream Go struct is
the API, so you set fields on it directly.

<!-- doc-example:excerpt the import block alone, which the two examples below use -->
```go
import (
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/utils/ptr"
    "github.com/go-kure/kure/pkg/kubernetes"
)
```

<!-- doc-example: pkg/kubernetes Example_libraryUsageCreate -->
```go
dep := kubernetes.CreateDeployment("web", "default") // identity only
dep.Spec.Replicas = ptr.To[int32](3)
dep.Spec.Selector = &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
dep.Spec.Template.Spec.ServiceAccountName = "web"
fmt.Println(dep.Kind, dep.Name, *dep.Spec.Replicas, dep.Spec.Selector.MatchLabels["app"])
```
<!-- doc-example:end -->

`kubernetes.Create[appsv1.Deployment]("web", "default")` is the generic form;
the per-kind wrappers are generated from the scheme and carry the scope in
their signature, so a cluster-scoped kind takes only a name
(`kubernetes.CreateNamespace("platform")`).

Kure adds a helper only for one of a few write shapes: appending to a list,
inserting into a map, setting a pointer field, or composing a small upstream
struct. A helper never defaults, never validates, and never touches a field you
did not name.

<!-- doc-example: pkg/kubernetes Example_libraryUsageHelpers -->
```go
dep := kubernetes.CreateDeployment("web", "default")

kubernetes.AddLabel(dep, "tier", "frontend") // works on any kind
kubernetes.SetDeploymentReplicas(dep, 3)
fmt.Println(dep.Labels["tier"], *dep.Spec.Replicas)
```
<!-- doc-example:end -->

The [Kubernetes Builders](/api-reference/kubernetes-builders) page is the
normative contract: what constructors emit, which helpers exist and why, and
the [migration notes](/concepts/builder-contract-release-1/) for the
constructor defaults that earlier releases injected and no longer do. If you
upgraded and a field you relied on is now empty, that page lists it.

The per-kind config-struct builders (`certmanager.Certificate(&CertificateConfig{...})`, <!-- doc-api-refs:ignore retired config-struct builder, named as gone -->
`cnpg.Cluster(&ClusterConfig{...})` and the like in `certmanager`, `cnpg`, <!-- doc-api-refs:ignore retired config-struct builder, named as gone -->
`externalsecrets`, `prometheus`, `cilium` and `volsync`) are gone too: build
those kinds with `Create<Kind>` and the upstream struct, exactly as above. The
[release 2 migration notes](/concepts/builder-contract-release-2/) map every
removed function, type and field to the upstream field that replaces it, and
list the values the CNPG builders used to inject that you now write yourself.

### FluxCD Resources

<!-- doc-example:excerpt the import block alone, which the example below uses -->
```go
import (
    "time"

    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
    sourcev1 "github.com/fluxcd/source-controller/api/v1"
    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
)
```

<!-- doc-example: pkg/kubernetes Example_libraryUsageFlux -->
```go
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
fmt.Println(repo.Spec.Reference.Branch, ks.Spec.SourceRef.Kind+"/"+ks.Spec.SourceRef.Name, ks.Spec.Path)
```
<!-- doc-example:end -->

Helm values given as a map go through `fluxcd.SetHelmReleaseValuesFromMap`, which marshals them to
JSON and panics on a value `encoding/json` refuses. Decoded YAML can carry one: `gopkg.in/yaml.v3`
decodes `.nan` and `.inf` into a float NaN or infinity without error, while `sigs.k8s.io/yaml`
returns that error at decode time. Decode with the latter, or marshal the values yourself and pass
them to `fluxcd.SetHelmReleaseValues`.

See the [FluxCD Builders reference](/api-reference/fluxcd-builders) for all available resource types.

Beyond FluxCD, the [Kubernetes Builders](/api-reference/kubernetes-builders) package provides typed constructors for core resources (Deployment, Service, Ingress, CronJob, NetworkPolicy, HTTPRoute), PSA security context helpers, ResourceRequirements builders, and more. The constructors write identity only, so fields the API server requires in context are yours to set: a Job from `kubernetes.CreateJob` needs `Spec.Template.Spec.RestartPolicy` set to `Never` or `OnFailure`, or the server defaults it to `Always` and rejects the Job. The [Prometheus Builders](/api-reference/prometheus-builders) sub-package covers ServiceMonitor, PodMonitor, and PrometheusRule CRDs.

## Generating YAML

Use the `io` package to serialize resources. `io.Marshal` and `io.PrintObjectsAsYAML` write to
the `io.Writer` you pass; `io.SaveFile` writes one object to a path:

<!-- doc-example:excerpt the import block alone, which the examples in this section use -->
```go
import (
    "os"
    "path/filepath"

    "sigs.k8s.io/controller-runtime/pkg/client"
    "github.com/go-kure/kure/pkg/io"
    "github.com/go-kure/kure/pkg/kubernetes"
)
```

<!-- doc-example: pkg/kubernetes Example_libraryUsageYAML -->
```go
cm := kubernetes.CreateConfigMap("app-config", "default")
cm.Data = map[string]string{"LOG_LEVEL": "info"}
var obj client.Object = cm
objects := []*client.Object{&obj}
dir, err := os.MkdirTemp("", "kure-library-usage")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(dir) }()

// Serialize a single object
if err := io.Marshal(os.Stdout, cm); err != nil {
    panic(err)
}

// Write multiple objects to stdout as YAML
if err := io.PrintObjectsAsYAML(objects, os.Stdout); err != nil {
    panic(err)
}

// Save to file
if err := io.SaveFile(filepath.Join(dir, "output.yaml"), cm); err != nil {
    panic(err)
}
```
<!-- doc-example:end -->

### Clean YAML encoding

When encoding resources exported from a cluster, server-managed metadata fields (`managedFields`, `resourceVersion`, `uid`, `generation`, `selfLink` and the `kubectl.kubernetes.io/last-applied-configuration` annotation) clutter the output. The default encoding strips all of these automatically, together with a null `creationTimestamp` and an empty `status`; a `creationTimestamp` that is set is kept:

<!-- doc-example: pkg/kubernetes Example_libraryUsageEncodeDefault -->
```go
objects := exportedObjects()

// Default: strips server-managed fields and uses standard key order
data, err := io.EncodeObjectsToYAMLWithOptions(objects, io.EncodeOptions{
    KubernetesFieldOrder: true,
})
if err != nil {
    panic(err)
}
fmt.Print(string(data))
```
<!-- doc-example:end -->

Use `ServerFieldStripping` to control the level of stripping:

<!-- doc-example: pkg/kubernetes Example_libraryUsageEncodeNone -->
```go
objects := exportedObjects()

// Preserve server fields (e.g. for debugging)
data, err := io.EncodeObjectsToYAMLWithOptions(objects, io.EncodeOptions{
    ServerFieldStripping: io.StripServerFieldsNone,
})
if err != nil {
    panic(err)
}
fmt.Print(string(data))
```
<!-- doc-example:end -->

See the [IO reference](/api-reference/io) for all output formats and stripping options.

## Asking What Kure Knows About a Kind

Every kind kure registers is described in a table generated from the pinned upstream
module sources — its scope, the module and version it came from, and the fields
upstream documents as gated, alpha, beta or deprecated. The lookups are in
`pkg/kubernetes`, and they answer from that table rather than from a hand-kept list.

<!-- doc-example:excerpt the import block alone, which the examples in this section use -->
```go
import (
    "github.com/go-kure/kure/pkg/kubernetes"
    "github.com/go-kure/kure/pkg/manifest"
)
```

<!-- doc-example: pkg/kubernetes Example_libraryUsageKindFor -->
```go
k, ok := kubernetes.KindFor("apps/v1", "Deployment") // exact group/version/kind
if ok {
    fmt.Println(k.Namespaced, k.Module, k.ScopeSource)
}

namespaced, known := kubernetes.IsNamespaced("autoscaling/v1", "HorizontalPodAutoscaler")
fmt.Println(namespaced, known)
```
<!-- doc-example:end -->

The row also carries `k.ModuleVersion`, the pinned version of that module; the example leaves it
out of what it prints because it changes with every dependency bump.

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

<!-- doc-example: pkg/kubernetes Example_libraryUsageScopeSource -->
```go
k, _ := kubernetes.KindForAnyVersion("cilium.io/v2", "CiliumNetworkPolicy")
fmt.Println(k.ScopeSource == kubernetes.ScopeSourceBuiltin) // false: declared by a marker

// pkg/manifest asks both forms of the scope question:
fmt.Println(manifest.IsNamespacedKind("cilium.io/v2", "CiliumNetworkPolicy"))        // true
fmt.Println(manifest.IsNamespacedBuiltinKind("cilium.io/v2", "CiliumNetworkPolicy")) // false, not a built-in
```
<!-- doc-example:end -->

The three sources are `ScopeSourceMarker` (the kind's own `+kubebuilder:resource`
marker), `ScopeSourceShippedCRD` (a `CustomResourceDefinition` the module ships) and
`ScopeSourceBuiltin` (the Kubernetes API itself). Only the last is a built-in, and
that distinction decides who wins in `manifest.Scope`: a built-in's scope comes from
the table, while for a custom resource a `CustomResourceDefinition` in the same
context governs — it names the scope the target cluster will serve, where the table
only records what the pinned module declared at build time.

Maturity is reported, never enforced (this example is compiled, not run, because what it prints
follows the pinned upstream modules):

<!-- doc-example: pkg/kubernetes Example_libraryUsageMaturity -->
```go
for _, f := range kubernetes.MaturityForType("k8s.io/api/core/v1", "PodSpec") {
    fmt.Println(f.Field, f.Stability, f.Gates)
}
gated := kubernetes.GatedFields() // every field behind a feature gate
fmt.Println(len(gated) > 0)
```
<!-- doc-example:end -->

kure does not warn, reject or filter on any of it — a consumer with cluster knowledge
decides. The table exists because the failure it describes is silent: the API server
does not reject a field whose feature gate is off, it clears the field and admits the
object, so the manifest reads as applied and is not. The same data is published as
`docs/api-tables.json` and [API Tables](/api-reference/api-tables) for readers outside
Go. See the [Kubernetes Builders](/api-reference/kubernetes-builders) page for what
each column means and how it is derived.

## Working with the Domain Model

For more complex scenarios, use the [Stack](/api-reference/stack) package to define cluster topologies:

<!-- doc-example:excerpt the import line alone, which the example below uses -->
```go
import "github.com/go-kure/kure/pkg/stack"
```

<!-- doc-example: pkg/kubernetes Example_libraryUsageDomainModel -->
```go
cluster, err := stack.NewClusterBuilder("production").
    WithNode("apps").
    WithBundle("web").
    WithApplication("frontend", frontendConfig).
    End().
    End().
    Build()
if err != nil {
    panic(err)
}
fmt.Println(cluster.Name, cluster.Node.Name, cluster.Node.Bundle.Name)
```
<!-- doc-example:end -->

`Build` returns the cluster and an error, which reports an invalid tree.

Then use the [Flux Engine](/api-reference/flux-engine) and [Layout Engine](/api-reference/layout) to generate a complete GitOps repository structure. See the [Generating Flux Manifests](/guides/flux-workflow/) guide for the full workflow.

## Error Handling

All Kure packages use the [errors](/api-reference/errors) package:

<!-- doc-example:excerpt the import block alone, which the example below uses -->
```go
import (
    "github.com/go-kure/kure/pkg/errors"
    "github.com/go-kure/kure/pkg/io"
)
```

<!-- doc-example: pkg/kubernetes Example_libraryUsageErrors -->
```go
load := func(path string) error {
    _, err := io.ParseFile(path)
    if err != nil {
        return errors.Wrap(err, "failed to generate manifests")
    }
    return nil
}
fmt.Println(load("does-not-exist.yaml"))
```
<!-- doc-example:end -->

## Next Steps

- [Generating Flux Manifests](/guides/flux-workflow/) for the complete workflow
- [API Reference](/api-reference) for all package documentation
- [Examples](/examples) for working code samples
