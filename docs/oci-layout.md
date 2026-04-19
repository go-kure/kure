# OCI Artifact Layout

This document describes the folder structure inside a Wharf stack OCI artifact,
how Flux Kustomization objects reference that structure, and how the layout
changes when the artifact is split across multiple OCIs.

Kure's `ManifestLayout` and `WriteToTar` are the primitives used to produce
this layout. The structure described here is what crane emits; kure enforces it
via the layout tree.

---

## Single OCI (monolithic)

All directories are siblings at the same level. Either all at the OCI root, or all
under a `<clustername>/` prefix — the nesting depth is consistent throughout.

```
cluster-prod/
├── flux-system/                         Layer 1 — bootstrap root
│   ├── gotk-components.yaml             Flux controller manifests (omitted when using flux-operator)
│   ├── OCIRepository  stack-prod        url: oci://<registry>/<repository>:<tag>
│   ├── Kustomization  flux-system       path: ./cluster-prod/flux-system
│   │                                    sourceRef: stack-prod
│   ├── Kustomization  flux-system-platform
│   │                                    path: ./cluster-prod/flux-system-platform
│   │                                    sourceRef: stack-prod
│   └── Kustomization  flux-system-<group>
│                                        path: ./cluster-prod/flux-system-<group>
│                                        sourceRef: stack-prod
│
├── flux-system-platform/          Layer 2 — platform group
│   └── Kustomization  platform-<id>        path: ./cluster-prod/platform/<id>
│                                        sourceRef: stack-prod
│
├── flux-system-<group>/                 Layer 2 — application group
│   └── Kustomization  <appname>         path: ./cluster-prod/<group>/<appname>
│                                        sourceRef: stack-prod
│
├── platform/<id>/                 Layer 3 — platform component payloads
│   └── helmrelease.yaml, issuer.yaml …
│
└── <group>/<appname>/                   Layer 3 — application payloads
    └── deployment.yaml, service.yaml …
```

**Bootstrap** applies exactly two objects: the `OCIRepository stack-prod` and the
`Kustomization flux-system`. Everything else is reconciled from those two roots —
Flux applies the Layer 2 Kustomization CRs which in turn apply Layer 3 payloads.

**All Kustomization objects carry `sourceRef: stack-prod`** — the single
OCIRepository defined in `flux-system/`.

---

## Split OCI (platform + per-group)

The split layout is a packaging concern only. Directory structure, file names,
Kustomization paths, and depends-on relationships are **identical** to the
monolithic layout. Only `sourceRef` values change in affected Kustomization objects.

### Rule

- `flux-system/` always lives in the **platform OCI** (the bootstrap root).
- Each `flux-system-<group>/` lives in the **same OCI as that group's payloads**.
- `flux-system/` gains one additional `OCIRepository` CR per split group OCI.
- The `Kustomization flux-system-<group>` CR inside `flux-system/` gets its
  `sourceRef` updated to point at the group's own OCIRepository.

### OCI naming convention

Derived from the base repository path by appending the set name:

| Set        | Repository suffix  | Example                           |
|------------|--------------------|-----------------------------------|
| Platform      | `-platform`           | `wharf/prod/stack-cluster-platform`  |
| App group  | `-<groupname>`     | `wharf/prod/stack-cluster-frontend` |
| Monolithic | *(none)*           | `wharf/prod/stack-cluster`        |

### Example: platform + frontend split

**OCI 1 — `stack-prod-platform`** (bootstrap root):
```
cluster-prod/
├── flux-system/
│   ├── gotk-components.yaml
│   ├── OCIRepository  stack-prod-platform      url: oci://<registry>/…-platform:<tag>
│   ├── OCIRepository  stack-prod-frontend   url: oci://<registry>/…-frontend:<tag>
│   ├── Kustomization  flux-system           path: ./cluster-prod/flux-system
│   │                                        sourceRef: stack-prod-platform
│   ├── Kustomization  flux-system-platform
│   │                                        path: ./cluster-prod/flux-system-platform
│   │                                        sourceRef: stack-prod-platform
│   └── Kustomization  flux-system-frontend  path: ./cluster-prod/flux-system-frontend
│                                            sourceRef: stack-prod-frontend    ← changed
├── flux-system-platform/
│   └── Kustomization  platform-cert-manager    path: ./cluster-prod/platform/cert-manager
│                                            sourceRef: stack-prod-platform
└── platform/
    └── cert-manager/
        └── helmrelease.yaml …
```

**OCI 2 — `stack-prod-frontend`**:
```
cluster-prod/
├── flux-system-frontend/                    ← lives here, not in platform OCI
│   ├── Kustomization  storefront            path: ./cluster-prod/frontend/storefront
│   │                                        sourceRef: stack-prod-frontend    ← changed
│   └── Kustomization  cart                  path: ./cluster-prod/frontend/cart
│                                            sourceRef: stack-prod-frontend    ← changed
└── frontend/
    ├── storefront/
    │   └── deployment.yaml …
    └── cart/
        └── deployment.yaml …
```

Bootstrap still applies only two objects from OCI 1: `OCIRepository stack-prod-platform`
and `Kustomization flux-system`. Flux discovers OCI 2 when it reconciles
`flux-system/` and finds the `OCIRepository stack-prod-frontend` CR there.

---

## Layer reference

| Layer | Directory pattern          | Contents                                       |
|-------|---------------------------|------------------------------------------------|
| 1     | `flux-system/`             | OCIRepository CRs, root + group Kustomization CRs, gotk-components |
| 2     | `flux-system-<group>/`     | Per-app Kustomization CRs for one group         |
| 3     | `<group>/<appname>/`       | Application manifests (Deployment, Service, …)  |
| 3     | `platform/<id>/`     | Platform component manifests (HelmRelease, …)   |

---

## Kure responsibilities

Kure's `ManifestLayout` tree represents the above structure. Key conventions:

- `Namespace: "."` on the root layout node → tar root, no extra prefix directory
- `FileNaming: layout.FileNamingKindName` on all nodes → `<Kind>-<name>.yaml` filenames
- Per-app payload layouts have their `Namespace` rewritten to `<group>/<appname>`
  by the caller (crane) before being attached as children
- `WriteToTar` walks the tree depth-first and emits `kustomization.yaml` at each
  node listing its children as resources

---

## Known pending changes

- **Cluster prefix**: whether to use a `<clustername>/` prefix or place everything
  at OCI root is not yet decided. Current crane output uses OCI root (no prefix).
- **App group paths**: current crane emits `apps/<appname>/` for Layer 3 app
  payloads. The correct pattern is `<group>/<appname>/`. This rename lands with
  applicationGroups support (crane#155).
- **Per-group split**: splitting individual app groups into their own OCIs is
  deferred until crane#155. The current `splitInfra bool` only separates the
  platform set from the combined apps set.
