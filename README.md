# Kure

Kure provides helper functions for building Kubernetes objects and resources used by
Flux, cert-manager, MetalLB and External Secrets. The library focuses on creating
strongly typed objects that can be modified through small helper functions.

The repository includes an extensive example in [main.go](main.go) that constructs
several resources and prints them as YAML. A short excerpt is shown below:

```go
y := printers.YAMLPrinter{}

ns := k8s.CreateNamespace("demo")
k8s.AddNamespaceLabel(ns, "env", "demo")

if err := y.PrintObj(ns, os.Stdout); err != nil {
    fmt.Fprintf(os.Stderr, "failed to print YAML: %v\n", err)
}
```

To use the helpers in your own code, import the desired package from
`github.com/go-kure/kure`:

```go
import "github.com/go-kure/kure/internal/k8s"
```

## Running tests

All packages include unit tests. Run them with:

```bash
go test ./...
```

The `go test` command will discover and execute tests across all packages.
