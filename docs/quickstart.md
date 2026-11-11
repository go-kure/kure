# Quickstart Guide

This guide walks you through installing Kure, generating one Flux Kustomization from Go, and handing it to Flux. It stays at the level of a single Kubernetes object. To build a whole cluster (Cluster, Node, Bundle, Application) and write the repository layout to disk, continue with the [cluster-to-disk example](/examples/getting-started/).

## Installation

You need the Go version that Kure's `go.mod` declares. Create a module and add Kure:

```bash
mkdir hello-kure && cd hello-kure
go mod init example.com/hello-kure
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
against the Example's own `// Output:` comment. The output below reproduces that comment by hand;
it is not generated.

Fetch the modules the program imports, then run it to see the generated YAML:

```bash
go mod tidy
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

The program prints the Kustomization to stdout. To deploy it you need a Flux installation that already watches your Git repository through a `GitRepository` named `flux-system`, and a `./clusters/production` directory in that repository that holds the manifests the Kustomization should apply.

1. **Save the output into the repository Flux watches, then commit it**

```bash
go run main.go > /path/to/your-repo/clusters/flux-system/hello-world.yaml
cd /path/to/your-repo
git add clusters/
git commit -m "Add hello-world kustomization"
git push
```

2. **Flux reconciles automatically**

If a Flux Kustomization already applies the directory you saved the file in, Flux picks up the new Kustomization on its next sync. Check the status:

```bash
flux get kustomizations
```

3. **Or trigger manually**

The `hello-world` Kustomization does not exist until Flux applies the file you pushed. First reconcile the Flux Kustomization that applies the directory you saved the file in. In a default Flux bootstrap, that Kustomization is named `flux-system`.

```bash
flux reconcile kustomization flux-system --with-source
```

When `flux get kustomizations` lists `hello-world`, reconcile it directly:

```bash
flux reconcile kustomization hello-world --with-source
```

## Next Steps

- **A whole cluster**: The [cluster-to-disk example](/examples/getting-started/) builds a Cluster with Nodes, Bundles and Applications and writes the repository layout
- **Flux workflow**: The [Flux workflow guide](/guides/flux-workflow/) covers the generated Flux resources in depth
- **Architecture**: Read the [Design Philosophy](/concepts/design-philosophy/) page for a deep dive into Kure's design
- **Building objects**: The [Builder Contract](/concepts/builder-contract/) page explains what a constructor does and does not set, when a `Set*`/`Add*` helper exists, and how to read the maturity table
- **Examples**: The [examples page](/examples/) lists the runnable examples and sample data
- **API Reference**: See the full API at [pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure)
- **Patching**: The patch engine moved to [go-kure/launcher](https://github.com/go-kure/launcher) in the launcher extraction (ADR-018); see that repository for current documentation
