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

Kure adds a helper only where a plain assignment is awkward: appending to a
list, inserting into a map, setting a pointer field, or composing a small
upstream struct. A helper never defaults, never validates, and never touches a
field you did not name.

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
fluxcd.SetGitRepositoryURL(repo, "https://github.com/org/repo")
fluxcd.SetGitRepositoryReference(repo, &sourcev1.GitRepositoryRef{Branch: "main"})
fluxcd.SetGitRepositoryInterval(repo, metav1.Duration{Duration: 5 * time.Minute})

// Create a Kustomization that references the source
ks := fluxcd.CreateKustomization("my-app", "flux-system")
fluxcd.SetKustomizationSourceRef(ks, kustv1.CrossNamespaceSourceReference{
    Kind: "GitRepository",
    Name: "my-repo",
})
fluxcd.SetKustomizationPath(ks, "./clusters/production")
fluxcd.SetKustomizationInterval(ks, metav1.Duration{Duration: 10 * time.Minute})
fluxcd.SetKustomizationPrune(ks, true)
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
