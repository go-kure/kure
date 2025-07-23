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


## License

This project is licensed under the Apache License 2.0. See the [LICENSE](LICENSE) file for details.
