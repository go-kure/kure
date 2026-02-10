# Kure FluxCD Quality Fixes + Features Plan

> **Status:** Completed 2026-02-09
> **Archived from:** `PLAN.md`
> **Related:** crane `docs/development/20260209-HISTORY-fluxcd-integration.md`

## Goal

Fix existing issues in kure's FluxCD workflow engine and add features
needed for crane's integration: source CRD creation, in-memory layout
generation (tar), and clean YAML serialization.

## Status Summary

| Step | Description | Status |
|------|-------------|--------|
| K1 | Deterministic kustomization.yaml ordering | Done |
| K2 | Add missing Bundle fields | Done |
| K3 | Fix findLayoutNode() path matching | Done |
| K4 | EncodeObjectsToYAML cleanup | Done |
| K5 | Implement createSource() | Partial (see GitHub #182) |
| K6 | WriteToTar(io.Writer) | Done |

## Step K1: Fix deterministic ordering in kustomization.yaml - Done

**Problem:** `WriteToDisk()` and `WriteManifest()` iterate over
`map[string][]client.Object`, producing non-deterministic file ordering.

**Files:** `pkg/stack/layout/manifest.go`, `pkg/stack/layout/write.go`

**Fix:** Collect `fileGroups` keys into `sortedFileNames []string`,
`sort.Strings()`, iterate that instead of the map directly -- for both
file writing and kustomization.yaml resource entries.

## Step K2: Add missing fields to stack.Bundle - Done

**Problem:** `stack.Bundle` lacks fields that `v1alpha1.BundleSpec` already
defines. The resource generator can't set them on generated Kustomization CRDs.

**New fields on `Bundle`:**
- `Annotations map[string]string`
- `Description string`
- `Prune *bool` (pointer to distinguish unset from false)
- `Wait *bool`
- `Timeout string`
- `RetryInterval string`

**Files:**
- `pkg/stack/bundle.go` -- Add fields
- `pkg/stack/fluxcd/resource_generator.go:createKustomization()` -- Use new fields
  (Prune defaults to true when nil; set Wait, Timeout, RetryInterval, Annotations)
- `pkg/stack/v1alpha1/converters.go` -- Round-trip new fields in both directions
- `pkg/stack/builders.go:copyBundle()` -- Copy new fields

## Step K3: Fix findLayoutNode() path-based matching - Done

**Problem:** `findLayoutNode()` matches by `ml.Name == node.Name`, which fails
when nodes at different hierarchy levels share the same name.

**File:** `pkg/stack/fluxcd/layout_integrator.go`

**Fix:** Replace with `findLayoutNodeByPath()` that accumulates the layout
path as it recurses and compares against `node.GetPath()`.

## Step K4: Move YAML cleanup into EncodeObjectsToYAML - Done

**Problem:** Crane's `serialize.go` has cleanup logic (strip null
creationTimestamp, empty status) that should be the default.

**File:** `pkg/io/yaml.go`

**Change:** `EncodeObjectsToYAML()` now uses `marshalCleanResource()`:
JSON marshal -> unmarshal to map -> clean -> YAML marshal. The raw
`EncodeObjectsTo()` is preserved for callers who want unmodified output.

Added helpers: `cleanResourceMap`, `cleanMetadata`,
`removeNullCreationTimestamp`, `removeEmptyStatus`, `isDeepEmpty`.

## Step K5: Implement createSource() for OCIRepository/GitRepository - Partial

**Problem:** `ResourceGenerator.createSource()` returns nil for both source
types. When a Bundle's SourceRef has a URL, the generator should create
the actual CRD.

**Remaining work tracked in:** GitHub #182 (FluxCD source CRD generation strategy)

**Files:**
- `pkg/stack/bundle.go` -- Add `URL`, `Tag`, `Branch` to `SourceRef` (done)
- `pkg/stack/fluxcd/resource_generator.go:createSource()` -- Wire to internal helpers (pending)

## Step K6: Add WriteToTar(io.Writer) to ManifestLayout - Done

**File:** `pkg/stack/layout/tar.go`

Mirrors `WriteToDisk` logic but writes tar entries:
1. Compute fullPath from basePath + `ml.FullRepoPath()`
2. Group resources into fileGroups (sorted -- same as WriteToDisk)
3. For each file: encode via `EncodeObjectsToYAML`, write tar header + content
4. Generate kustomization.yaml as tar entry
5. Recurse into children
