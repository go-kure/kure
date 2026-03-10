# Demo Input Configurations

These directories contain YAML input files consumed by `cmd/demo` to
demonstrate Kure's manifest generation pipeline.

## Subdirectories

- **`app-workloads/`** — `AppWorkload` definitions that generate Deployment
  and Service manifests via the GVK generator system.
- **`bootstrap/`** — Cluster bootstrap configurations for Flux and ArgoCD,
  producing GitOps bootstrap manifests.
- **`clusters/`** — Full cluster definitions with node hierarchies and
  application references, generating complete repo layouts.
- **`multi-oci/`** — Multi-source OCI package configurations demonstrating
  per-node manifest generation.

## Running

From the repository root:

```bash
make build-demo && ./bin/demo
```

Output is written to `out/` with one subdirectory per demo section.
