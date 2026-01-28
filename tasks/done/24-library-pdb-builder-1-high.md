# Task: Add PodDisruptionBudget Builder

**Priority:** 1 - High (Short Term: 1-2 days)
**Category:** library
**Status:** Completed (37bb321)
**Dependencies:** None
**Blocked By:** None

---

## Overview

Add a PodDisruptionBudget (PDB) builder to `internal/kubernetes/` following the existing builder pattern. This is required to support the Scaler trait in the Crane OAM implementation.

## Current State

The Kure library has **partial PDB support**:
- ✅ Error constant exists: `ErrNilPodDisruptionBudget` in `pkg/errors/errors.go:75`
- ✅ Validation exists: `ValidatePodDisruptionBudget` in `internal/validation/validators.go:138-140`
- ❌ **No builder** in `internal/kubernetes/`

Only the builder implementation is needed.

## Context

This gap was identified during Crane/Kure evaluation (see `/home/serge/src/autops/wharf/ADR/research/crane-oam-kure/kure-gap-analysis.md`). The Scaler trait from ADR-006 requires both HPA and PDB builders.

## Objectives

1. Create `internal/kubernetes/pdb.go` with builder functions
2. Add comprehensive unit tests
3. Leverage existing validation infrastructure

## Implementation

### Create PDB Builder

**File:** `internal/kubernetes/pdb.go`

Follow the pattern established in `deployment.go`:

```go
package kubernetes

import (
    policyv1 "k8s.io/api/policy/v1"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "k8s.io/apimachinery/pkg/util/intstr"
    "github.com/autops/kure/internal/validation"
)

// CreatePodDisruptionBudget creates a PDB with sensible defaults
func CreatePodDisruptionBudget(name string, namespace string) *policyv1.PodDisruptionBudget {
    obj := &policyv1.PodDisruptionBudget{
        TypeMeta: metav1.TypeMeta{
            Kind:       "PodDisruptionBudget",
            APIVersion: policyv1.SchemeGroupVersion.String(),
        },
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
    }
    return obj
}

// SetMinAvailable sets the minimum number of pods that must be available
func SetMinAvailable(pdb *policyv1.PodDisruptionBudget, minAvailable intstr.IntOrString) error {
    v := validation.NewValidator()
    if err := v.ValidatePodDisruptionBudget(pdb); err != nil {
        return err
    }
    pdb.Spec.MinAvailable = &minAvailable
    pdb.Spec.MaxUnavailable = nil // mutually exclusive
    return nil
}

// SetMaxUnavailable sets the maximum number of pods that can be unavailable
func SetMaxUnavailable(pdb *policyv1.PodDisruptionBudget, maxUnavailable intstr.IntOrString) error {
    v := validation.NewValidator()
    if err := v.ValidatePodDisruptionBudget(pdb); err != nil {
        return err
    }
    pdb.Spec.MaxUnavailable = &maxUnavailable
    pdb.Spec.MinAvailable = nil // mutually exclusive
    return nil
}

// SetSelector sets the label selector for the PDB
func SetPDBSelector(pdb *policyv1.PodDisruptionBudget, matchLabels map[string]string) error {
    v := validation.NewValidator()
    if err := v.ValidatePodDisruptionBudget(pdb); err != nil {
        return err
    }
    pdb.Spec.Selector = &metav1.LabelSelector{
        MatchLabels: matchLabels,
    }
    return nil
}

// SetPDBLabels sets the labels on the PDB object
func SetPDBLabels(pdb *policyv1.PodDisruptionBudget, labels map[string]string) error {
    v := validation.NewValidator()
    if err := v.ValidatePodDisruptionBudget(pdb); err != nil {
        return err
    }
    pdb.Labels = labels
    return nil
}

// SetPDBAnnotations sets the annotations on the PDB object
func SetPDBAnnotations(pdb *policyv1.PodDisruptionBudget, annotations map[string]string) error {
    v := validation.NewValidator()
    if err := v.ValidatePodDisruptionBudget(pdb); err != nil {
        return err
    }
    pdb.Annotations = annotations
    return nil
}
```

### Add Unit Tests

**File:** `internal/kubernetes/pdb_test.go`

Test cases:
- CreatePodDisruptionBudget with valid name/namespace
- SetMinAvailable with nil PDB (should error)
- SetMinAvailable with valid PDB
- SetMaxUnavailable clears MinAvailable (mutual exclusivity)
- SetMinAvailable clears MaxUnavailable (mutual exclusivity)
- SetPDBSelector with valid labels
- Integer vs percentage values for MinAvailable/MaxUnavailable

## Success Criteria

- [ ] `internal/kubernetes/pdb.go` implements builder pattern
- [ ] Uses existing `ValidatePodDisruptionBudget` from validators.go
- [ ] Uses existing `ErrNilPodDisruptionBudget` from errors.go
- [ ] Unit tests in `pdb_test.go` with >80% coverage
- [ ] All existing tests pass (`make precommit`)
- [ ] Code follows existing patterns in deployment.go

## Estimated Effort

**1 day** (simpler than HPA - validation/error already exist)
