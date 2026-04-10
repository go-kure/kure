# Umbrella Cluster Example

Demonstrates the `Bundle.Children` umbrella pattern: one parent Flux
Kustomization that waits on a set of child Kustomizations, each of which
manages a subset of the platform.

## Pattern

An **umbrella** is a parent `Bundle` whose `children:` field nests other
`Bundle`s (containment), as opposed to `dependsOn:` which expresses ordering
between peer bundles.

```yaml
# cluster.yaml
name: umbrella-demo
node:
  name: flux-system
  bundle:
    name: platform
    children:
      - name: platform-infra
      - name: platform-services
      - name: platform-apps
```

Each child is a directory next to `cluster.yaml` containing `AppWorkload`
YAML documents. `cmd/demo` loads them via `loadUmbrellaChildrenApps` so the
child bundles carry their own applications.

## Containment vs ordering

- **`children:`** — the parent *contains* the children. They render as
  sub-layouts under the parent directory and, in Flux, as separate
  `Kustomization` CRs that the parent Kustomization waits on via
  `spec.wait: true` and `spec.healthChecks`.
- **`dependsOn:`** — expresses pure ordering between two peer bundles
  without nesting one inside the other.

See `site/content/concepts/domain-model.md` for the full model.

## Expected output

Running `./bin/demo` produces:

```
out/umbrella-demo-repo/clusters/umbrella-demo/flux-system/
├── flux-system-kustomization-platform.yaml       # parent CR (spec.wait + 3 healthChecks)
├── flux-system-kustomization-platform-apps.yaml  # child CRs
├── flux-system-kustomization-platform-infra.yaml
├── flux-system-kustomization-platform-services.yaml
├── kustomization.yaml                             # references the 4 CRs above
├── platform-apps/                                 # child workloads + own kustomization.yaml
│   ├── kustomization.yaml
│   └── platform-apps-deployment-frontend.yaml
├── platform-infra/
│   ├── kustomization.yaml
│   └── platform-infra-deployment-networking.yaml
└── platform-services/
    ├── kustomization.yaml
    └── platform-services-deployment-cache.yaml
```

The parent `flux-system/kustomization.yaml` lists the four CR filenames
exactly once each — no duplicates, no plain-subdirectory references to the
umbrella children (they are applied through their own `Kustomization` CRs).
