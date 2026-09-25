# Quickstart Guide

This guide walks you through installing Kure, generating your first cluster configuration, and deploying with Flux.

## Installation

Add Kure as a Go module dependency:

```bash
go get github.com/go-kure/kure@latest
```

## Hello World: Generate a Simple Cluster Config

Create a minimal Go program that generates Kubernetes manifests. Start `main.go` with the package
clause and imports:

<!-- doc-example:excerpt the file header alone; the body of main follows as a generated block -->
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
```

and put this in `func main() { ... }`:

<!-- doc-example: pkg/kubernetes/fluxcd Example_quickstart -->
```go
ks := fluxcd.CreateKustomization("hello-world", "flux-system")
ks.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
    Kind: "GitRepository",
    Name: "flux-system",
}
ks.Spec.Path = "./clusters/production"
ks.Spec.Interval = metav1.Duration{Duration: 5 * time.Minute}
ks.Spec.Prune = true

if err := io.Marshal(os.Stdout, ks); err != nil {
    panic(err)
}
```
<!-- doc-example:end -->

That body is `Example_quickstart` in `pkg/kubernetes/fluxcd`, which `go test` runs and checks
against the output below.

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
  interval: 5m0s
  path: ./clusters/production
  prune: true
  sourceRef:
    kind: GitRepository
    name: flux-system
status: {}
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
- **Building objects**: The [Builder Contract](/concepts/builder-contract/) page explains what a constructor does and does not set, when a `Set*`/`Add*` helper exists, and how to read the maturity table
- **Examples**: Explore the `examples/` directory for more complex configurations
- **API Reference**: See the full API at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure)
- **Patching**: The patch engine moved to [go-kure/launcher](https://github.com/go-kure/launcher) in the launcher extraction (ADR-018); see that repository for current documentation
