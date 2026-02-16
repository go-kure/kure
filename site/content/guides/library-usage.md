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

Kure provides typed builder functions for Kubernetes and FluxCD resources.

### FluxCD Resources

```go
import "github.com/go-kure/kure/pkg/kubernetes/fluxcd"

// Create a GitRepository source
repo := fluxcd.GitRepository(&fluxcd.GitRepositoryConfig{
    Name:      "my-repo",
    Namespace: "flux-system",
    URL:       "https://github.com/org/repo",
    Branch:    "main",
    Interval:  "5m",
})

// Create a Kustomization that references the source
ks := fluxcd.Kustomization(&fluxcd.KustomizationConfig{
    Name:      "my-app",
    Namespace: "flux-system",
    Path:      "./clusters/production",
    Interval:  "10m",
    Prune:     true,
    SourceRef: kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "my-repo",
    },
})
```

See the [FluxCD Builders reference](/api-reference/fluxcd-builders) for all available resource types.

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

Then use the [Flux Engine](/api-reference/flux-engine) and [Layout Engine](/api-reference/layout) to generate a complete GitOps repository structure. See the [Generating Flux Manifests](flux-workflow) guide for the full workflow.

## Error Handling

All Kure packages use the [errors](/api-reference/errors) package:

```go
import "github.com/go-kure/kure/pkg/errors"

if err != nil {
    return errors.Wrap(err, "failed to generate manifests")
}
```

## Next Steps

- [Generating Flux Manifests](flux-workflow) for the complete workflow
- [API Reference](/api-reference) for all package documentation
- [Examples](/examples) for working code samples
