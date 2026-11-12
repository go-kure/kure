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

- The root layout node sets `Namespace: "."` and an empty `Name`, so the tar root has no extra prefix
  directory. The writer joins `Name` onto `Namespace`, so a root with a name writes under that name.
- Every node sets `FileNaming: layout.FileNamingKindName`, so each file is named `<Kind>-<name>.yaml`.
- A caller that attaches a per-application payload layout as a child sets that layout's `Namespace` to
  the parent path `<group>` and its `Name` to `<appname>`. The layout directory is `Namespace` joined
  with `Name`, so a `Namespace` that already holds the application name nests one level too deep.
- `WriteToTar` walks the tree depth first. A node in explicit mode (the default) gets a
  `kustomization.yaml` that lists the children with a directory entry. A node in recursive mode gets
  no such file, and neither does a single-file application child. A bundle that renders itself, an
  umbrella child and an integrated per-layout directory have no entry in the parent's list.

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
