+++
title = "OCI Artifact Layout"
weight = 60
+++

A manifest tree that Flux pulls from an OCI registry needs a fixed directory shape. The design of that
shape belongs to the go-kure organisation, and its canonical text is
[OCI Artifact Layout](https://github.com/go-kure/.github/blob/main/docs/design/oci-layout.md). Read it
for the single and split artifact forms, the naming convention and the open items. This page states only
what Kure does to produce the shape.

## What Kure does

Kure builds the tree with a `ManifestLayout` and writes it with `WriteToTar`. The design document names
these conventions:

- The root layout node sets `Namespace: "."`, so the tar root has no extra prefix directory.
- Every node sets `FileNaming: layout.FileNamingKindName`, so each file is named `<Kind>-<name>.yaml`.
- A caller that attaches a per-application payload layout as a child first rewrites that layout's
  `Namespace` to `<group>/<appname>`.
- `WriteToTar` walks the tree depth first and writes a `kustomization.yaml` at each node that lists the
  node's children as resources.

## The three layers

| Layer | Directory | Holds |
|-------|-----------|-------|
| 1 | `flux-system/` | The OCIRepository objects, the root and group Kustomizations, and the Flux components |
| 2 | `flux-system-<group>/` | The Kustomization of each application in one group |
| 3 | `<group>/<appname>/` or `platform/<id>/` | The application or platform manifests |

Flux applies the layer 2 Kustomizations from the layer 1 roots, and those apply the layer 3 payloads.

## Related pages

- [Layout and Flux Naming](/concepts/layout-and-flux-naming/) explains how Kure names directories, files
  and Flux objects.
- [Layout Engine](/api-reference/layout/) documents `ManifestLayout` and `WriteToTar`.
- [Generating Flux Manifests](/guides/flux-workflow/) walks through writing a layout to disk.
