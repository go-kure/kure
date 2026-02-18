# Quickstart Guide

This guide walks you through installing Kure, generating your first cluster configuration, and deploying with Flux.

## Installation

Install the Kure CLI tools using Go:

```bash
go install github.com/go-kure/kure/cmd/kure@latest
go install github.com/go-kure/kure/cmd/kurel@latest
```

Verify the installation:

```bash
kure version
kurel version
```

## Hello World: Generate a Simple Cluster Config

Create a minimal Go program that generates Kubernetes manifests:

```go
package main

import (
    "os"

    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
    "github.com/go-kure/kure/pkg/io"
    kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
)

func main() {
    // Create a Flux Kustomization
    ks := fluxcd.NewKustomization(&fluxcd.KustomizationConfig{
        Name:      "hello-world",
        Namespace: "flux-system",
        Interval:  "5m",
        Path:      "./clusters/production",
        SourceRef: kustv1.CrossNamespaceSourceReference{
            Kind: "GitRepository",
            Name: "flux-system",
        },
    })

    // Print YAML to stdout
    printer := io.NewYAMLPrinter()
    printer.PrintObj(ks, os.Stdout)
}
```

Run the program to see the generated YAML:

```bash
go run main.go
```

Output:

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: hello-world
  namespace: flux-system
spec:
  interval: 5m
  path: ./clusters/production
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
```

## Build a Kurel Package

Kurel packages provide a structured way to define reusable Kubernetes applications. Try building the example Frigate package:

```bash
cd examples/kurel/frigate
kurel build .
```

This generates Kubernetes manifests from the package definition. Inspect the package structure:

```
frigate/
  kurel.yaml         # Package metadata
  parameters.yaml    # Configurable parameters
  resources/         # Base Kubernetes resources
  patches/           # Optional patches
```

Validate a package before deployment:

```bash
kurel validate .
```

View package information:

```bash
kurel info .
```

## Deploy with Flux

Once you have generated manifests, deploy them using Flux:

1. **Commit the manifests to your Git repository**

```bash
git add clusters/
git commit -m "Add hello-world kustomization"
git push
```

2. **Flux reconciles automatically**

If Flux is already watching your repository, it will automatically apply the new Kustomization. Check the status:

```bash
flux get kustomizations
```

3. **Or trigger manually**

```bash
flux reconcile kustomization flux-system --with-source
```

## Next Steps

- **Architecture**: Read the [Architecture](/concepts/architecture/) page for a deep dive into Kure's design
- **Examples**: Explore the `examples/` directory for more complex configurations
- **API Reference**: See the full API at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure)
- **Patching**: Learn about declarative patching in the [README](https://github.com/go-kure/kure#declarative-patching)
- **CLI Reference**: Run `kure --help` and `kurel --help` for all available commands
