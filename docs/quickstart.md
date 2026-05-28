# Quickstart Guide

This guide walks you through installing Kure, generating your first cluster configuration, and deploying with Flux.

## Installation

Add Kure as a Go module dependency:

```bash
go get github.com/go-kure/kure@latest
```

## Hello World: Generate a Simple Cluster Config

Create a minimal Go program that generates Kubernetes manifests:

```go
package main

import (
    "os"
    "time"

    "github.com/go-kure/kure/pkg/io"
    "github.com/go-kure/kure/pkg/kubernetes/fluxcd"
    kustv1 "github.com/fluxcd/kustomize-controller/api/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func main() {
    ks := fluxcd.CreateKustomization("hello-world", "flux-system")
    fluxcd.SetKustomizationSourceRef(ks, kustv1.CrossNamespaceSourceReference{
        Kind: "GitRepository",
        Name: "flux-system",
    })
    fluxcd.SetKustomizationPath(ks, "./clusters/production")
    fluxcd.SetKustomizationInterval(ks, metav1.Duration{Duration: 5 * time.Minute})
    fluxcd.SetKustomizationPrune(ks, true)

    io.Marshal(os.Stdout, ks)
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

- **Architecture**: Read the [Design Philosophy](/concepts/design-philosophy/) page for a deep dive into Kure's design
- **Examples**: Explore the `examples/` directory for more complex configurations
- **API Reference**: See the full API at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure)
- **Patching**: Learn about declarative patching in the [README](https://github.com/go-kure/kure#declarative-patching)
