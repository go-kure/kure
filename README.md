# Kure

Kure provides helper functions for building Kubernetes objects and resources used by
Flux, cert-manager, MetalLB and External Secrets. The library focuses on creating
strongly typed objects that can be modified through small helper functions.

The repository includes an extensive example in [cmd/demo/main.go](cmd/demo/main.go) that constructs
several resources and prints them as YAML. A short excerpt is shown below:

```go
y := printers.YAMLPrinter{}

ns := k8s.CreateNamespace("demo")
k8s.AddNamespaceLabel(ns, "env", "demo")

if err := y.PrintObj(ns, os.Stdout); err != nil {
    fmt.Fprintf(os.Stderr, "failed to print YAML: %v\n", err)
}
```

Run the full example locally using:

```bash
go run ./cmd/demo
```

To use the helpers in your own code, import the desired package from
`github.com/go-kure/kure`.


## Running tests

All packages include unit tests. Run them with:

```bash
go test ./...
```

The `go test` command will discover and execute tests across all packages.

## Documentation

API reference documentation is available at
[pkg.go.dev](https://pkg.go.dev/github.com/go-kure/kure). Packages like
`k8s`, `fluxcd`, `certmanager`, and `metallb` are located under the
`internal/` directory and include helpers for constructing related
resources.

## Patching manifests

The `kure` CLI can patch a base manifest using a file of patch operations. Example files are located in `examples/patch`:

```bash
kure patch --base examples/patch/base-config.yaml --patch examples/patch/patch.yaml
```

The command reads the base resource(s) and applies the patches, printing the resulting YAML to stdout.

## Cluster example

The repository also provides an example for building a cluster configuration
programmatically. Running the following command will print the generated YAML
to stdout:

```bash
go run ./cmd/cluster
```


## Flux vs ArgoCD paths

Flux Kustomizations reference directories in a Git repository using
`spec.path`. The value must begin with `./` and is interpreted relative to the
repository root. ArgoCD Applications use `spec.source.path` without the `./`
prefix but with the same relative semantics.

When nodes or bundles are stored in subfolders, the path has to point directly
to that folder unless the directory tree only contains files for a single
node or bundle. Flux will recursively auto-generate a `kustomization.yaml` when
one is missing and include every manifest under the specified path. ArgoCD does
not auto-generate a `kustomization.yaml` and therefore ignores nested
directories unless they are referenced from a `kustomization.yaml` at the
target path.

For example:

```text
repo/
  clusters/
    prod/
      nodes/
        cp/
          kustomization.yaml
      bundles/
        monitoring/
          kustomization.yaml
```

Flux Kustomization for the control-plane node:

```yaml
spec:
  path: ./clusters/prod/nodes/cp
```

Equivalent ArgoCD Application:

```yaml
spec:
  source:
    path: clusters/prod/nodes/cp
```

With this layout, each node or bundle is targeted individually. Pointing a Flux
Kustomization to the parent directory (`./clusters/prod`) would combine the
`cp` and `monitoring` manifests into a single deployment because it would
auto-generate a `kustomization.yaml` for the entire tree. ArgoCD will only
process the manifests under `clusters/prod` itself unless a
`kustomization.yaml` aggregates the subdirectories, so each subfolder must be
referenced separately.



## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
