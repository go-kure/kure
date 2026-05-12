# Demo Input Configurations

These directories contain YAML input files that demonstrate Kure's manifest generation pipeline.

## Subdirectories

- **`app-workloads/`** — `AppWorkload` definitions that generate Deployment
  and Service manifests via the GVK generator system.
- **`bootstrap/`** — Cluster bootstrap configurations for Flux and ArgoCD,
  producing GitOps bootstrap manifests.
- **`clusters/`** — Full cluster definitions with node hierarchies and
  application references, generating complete repo layouts.
- **`multi-oci/`** — Multi-source OCI package configurations demonstrating
  per-node manifest generation.

These YAML files serve as reference inputs and test fixtures for the library's manifest generation packages.
