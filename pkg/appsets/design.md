# Kure `appsets` Module — Purpose and Design

## Purpose

The `appsets` module provides the **core abstraction layer** for loading, representing, and mutating Kubernetes resources using **structured patches** — without templates, overlays, or DSLs.

It enables tools like **Crane** and **Kur8** to declaratively define resource configurations and safe modifications using **pure YAML** and **Go-native data structures**. Patches themselves modify an existing **base resource**, which must be provided when a patchable set is created, either loaded from YAML or passed directly as an object.

This forms the foundation for Kure's deterministic, introspectable Kubernetes manifest generation pipeline.

---

## Design Principles

1. **Declarative patching** over templating:
   - Use single-line YAML to express changes to Kubernetes objects.
   - Avoid logic or conditional expressions in YAML.

2. **One patch = one operation**:
   - Patches operate on individual fields or list items.
   - Operations include: `replace`, `delete`, `insertbefore`, `insertafter`, `append`.

3. **Smart patch targeting**:
   - Automatically routes patches to matching resources using metadata.
   - Supports `target:` override when ambiguity exists.

4. **Fully introspectable and validatable**:
   - All paths are parsed and normalized into `PathPart` structs.
   - Path syntax is explicitly validated before application.

5. **Base resource required**:
   - Each patch is applied on top of an existing object loaded from a file or provided programmatically.

---

## Core Types

- `PatchOp`: A single parsed patch line (path, value, operation)
- `PathPart`: A structured representation of a patch path segment
- `PatchableAppSet`: Holds resources and their associated patch operations

---

## Interfaces and Helpers

- `LoadResourcesFromMultiYAML(io.Reader)` — Load 1..n Kubernetes resources
- `LoadPatchFile(io.Reader)` — Load simple or targeted patch syntax
- `NewPatchableAppSet([]*unstructured.Unstructured, []PatchSpec)` — Create a patchable set from in-memory objects
- `LoadPatchableAppSet([]io.Reader, io.Reader)` — Create a full working patchable set
- `NormalizePath()` — Validate and parse patch paths before application

---

## Syntax Highlights

Supports expressive list modification syntax:

```yaml
spec.containers[3].image: nginx:latest        # replace
spec.containers[+=name=web].image: sidecar:1  # insert after matching item
spec.containers[-]: { name: debug }           # append
metadata.labels[delete=app]: ""               # delete label

