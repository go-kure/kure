# Demo Input Configurations

These directories contain sample YAML files shaped like cluster, bootstrap and workload
definitions. Kure ships no command that reads them (there is no `bin/demo` program), so treat
them as sample data. The Go program that runs the pipeline is
[`examples/getting-started`](../getting-started/).

## Subdirectories

- **`app-workloads/`** — Sample `AppWorkload` definitions (a Deployment and Service per
  application). `AppWorkload` is an example kind; kure has no generator for it.
- **`bootstrap/`** — Cluster bootstrap configurations for Flux and ArgoCD,
  producing GitOps bootstrap manifests.
- **`clusters/`** — Full cluster definitions with node hierarchies and
  application references, generating complete repo layouts.
- **`multi-oci/`** — Multi-source OCI package configurations demonstrating
  per-node manifest generation.

Use them for the shape of an input, not as a tested interface.
