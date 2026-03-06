# v1alpha1 API Graduation Path

## Overview

This document defines the graduation criteria for promoting the `stack.gokure.dev/v1alpha1` API to `v1beta1` and eventually `v1`. The v1alpha1 API is kure's serialization layer for stack definitions (ClusterConfig, NodeConfig, BundleConfig) and is consumed by Crane and potentially other Wharf components.

## Current State

- **API Group**: `stack.gokure.dev`
- **Current Version**: `v1alpha1`
- **Release**: `v0.1.0-beta.1` (2026-02-17)
- **v1alpha1 Types**: ClusterConfig, NodeConfig, BundleConfig, StackRegistry
- **Consumers**: Crane (autops/wharf/crane)
- **API Surface**: ~4,200 lines across 6 source files in `pkg/stack/v1alpha1/`

### Completed Prerequisites

- [x] Builder promotion (#241) — 7 internal K8s builders promoted to `pkg/kubernetes/`
- [x] FluxCD 2.8 upgrade (#128) — v1 APIs available
- [x] GVK package promoted to `pkg/gvk/`
- [x] StackRegistry with factory pattern and dynamic registration

## Graduation Criteria

### v1alpha1 → v1beta1

The API graduates to `v1beta1` when **all** of the following are met:

1. **Stability Duration**: v1alpha1 API has been stable (no breaking changes) for at least 3 months after `v0.1.0-rc.1` release.

2. **API Coverage**: All planned resource types are implemented and tested:
   - [x] ClusterConfig with GitOps provider selection
   - [x] NodeConfig with tree hierarchy support
   - [x] BundleConfig with dependency management and validation
   - [x] StackRegistry with factory pattern
   - [x] Bidirectional converters
   - [ ] Workflow interface uses typed parameters (not `any`) — #315

3. **Consumer Validation**: At least 2 consumers actively use the v1alpha1 types:
   - Crane (confirmed — imports `pkg/stack/v1alpha1` for OAM compilation)
   - Barge CLI (expected — will use types for stack definition authoring)

4. **Test Coverage**: ≥80% coverage on all v1alpha1 packages (currently met).

5. **Documentation**: Complete API reference documentation for all public types.

6. **No Known Bugs**: Zero open issues tagged `priority/critical` against v1alpha1 types.

7. **FluxCD v1 Migration Complete**: All Flux builders use v1 GA APIs (#249, #250, #251).

### v1beta1 → v1

The API graduates to `v1` (stable) when **all** of the following are met:

1. **Stability Duration**: v1beta1 API has been stable for at least 6 months.

2. **Consumer Count**: ≥3 consumers actively importing and using the API.

3. **Backward Compatibility**: Conversion webhooks or hub-spoke pattern implemented for v1alpha1 → v1 migration path.

4. **Production Usage**: At least one production deployment using v1beta1 types through Crane.

5. **API Review**: Formal API review completed, covering:
   - Naming consistency across all types
   - Field optionality and defaults
   - Validation completeness
   - Serialization round-trip fidelity

## Proposed Timeline

| Milestone | Target Date | Gate |
|-----------|-------------|------|
| v0.1.0-rc.1 | 2026-Q2 | API surface stabilized, Workflow typed (#315), coverage ≥80% |
| v0.1.0-stable | 2026-Q2 | Flux v1 migrations complete (#249, #250, #251), zero critical issues |
| v1beta1 proposal | 2026-Q3 | 3 months stable after rc.1, 2+ consumers, API review |
| v1beta1 release | 2026-Q4 | All v1beta1 criteria met |
| v1 proposal | 2027-Q2 | 6 months stable v1beta1, 3+ consumers, production usage |

## Breaking Changes Inventory

Changes required when graduating from v1alpha1 to v1beta1:

### Package Path Changes

| Current | Graduated |
|---------|-----------|
| `pkg/stack/v1alpha1` | `pkg/stack/v1beta1` |
| `stack.gokure.dev/v1alpha1` | `stack.gokure.dev/v1beta1` |

### Type Changes Under Consideration

1. **Workflow interface** (#315): Replace `any` parameters with typed `*layout.LayoutRules` and `*layout.ManifestLayout`. Must be resolved before graduation.

2. **BootstrapConfig**: Currently embeds Flux-specific and ArgoCD-specific fields in the same struct. Consider splitting into provider-specific sub-structs for cleaner API boundaries:
   ```
   // Current (v1alpha1)
   BootstrapConfig.FluxMode, FluxVersion, ArgoCDVersion, ArgoCDNamespace

   // Proposed (v1beta1)
   BootstrapConfig.Flux *FluxBootstrapConfig
   BootstrapConfig.ArgoCD *ArgoCDBootstrapConfig
   ```

3. **NodeReference**: Consider adding `Kind` field for cross-kind references (future extensibility).

4. **BundleConfig.Spec**: The `ParentPath` field is an internal tree-management detail. Consider removing from the serialized API and computing it during tree construction.

### Cross-Repo Impact (Crane)

Crane imports the following from kure v1alpha1:
- `pkg/stack/v1alpha1.ClusterConfig`
- `pkg/stack/v1alpha1.NodeConfig`
- `pkg/stack/v1alpha1.BundleConfig`
- Converter functions for serialization/deserialization

**Migration path for Crane**:
1. Kure publishes v1beta1 package alongside v1alpha1 (both available during transition)
2. Kure provides `ConvertV1Alpha1ToV1Beta1()` functions
3. Crane migrates imports to v1beta1
4. v1alpha1 package deprecated (kept for one release cycle)
5. v1alpha1 package removed

## Migration Strategy

### Multi-Version Support

During graduation, kure will support both versions simultaneously:

```
pkg/stack/
├── v1alpha1/    # Deprecated but functional
├── v1beta1/     # New graduated version
└── converters/  # Cross-version conversion utilities
```

### Converter Pattern

Each type will have bidirectional converters:
- `ConvertV1Alpha1ToV1Beta1()`
- `ConvertV1Beta1ToV1Alpha1()`

The StackRegistry will accept both versions and normalize internally.

### Deprecation Policy

- Deprecated versions are maintained for **one minor release cycle** after the graduated version ships.
- Deprecated versions emit a log warning on use.
- Removal is announced one release in advance.
