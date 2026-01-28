# Task: Add HorizontalPodAutoscaler Builder

**Priority:** 1 - High (Short Term: 2-3 days)
**Category:** library
**Status:** Completed (296dc88)
**Dependencies:** None
**Blocked By:** None

---

## Overview

Add a HorizontalPodAutoscaler (HPA) builder to `internal/kubernetes/` following the existing builder pattern. This is required to support the Scaler trait in the Crane OAM implementation.

## Current State

The Kure library currently has **no HPA support**:
- No builder in `internal/kubernetes/`
- No validation in `internal/validation/validators.go`
- No error constant in `pkg/errors/errors.go`

Compare to PDB which has validation and error but no builder.

## Context

This gap was identified during Crane/Kure evaluation (see `/home/serge/src/autops/wharf/ADR/research/crane-oam-kure/kure-gap-analysis.md`). The Scaler trait from ADR-006 requires both HPA and PDB builders.

## Objectives

1. Add `ErrNilHorizontalPodAutoscaler` error constant
2. Add `ValidateHorizontalPodAutoscaler` validation function
3. Create `internal/kubernetes/hpa.go` with builder functions
4. Add comprehensive unit tests

## Implementation

### 1. Add Error Constant

**File:** `pkg/errors/errors.go`

```go
ErrNilHorizontalPodAutoscaler = ResourceValidationError("HorizontalPodAutoscaler", "", "hpa", "horizontal pod autoscaler cannot be nil", nil)
```

### 2. Add Validation Function

**File:** `internal/validation/validators.go`

```go
func (v *Validator) ValidateHorizontalPodAutoscaler(hpa *autoscalingv2.HorizontalPodAutoscaler) error {
    return v.validateNotNil(hpa, errors.ErrNilHorizontalPodAutoscaler)
}
```

### 3. Create HPA Builder

**File:** `internal/kubernetes/hpa.go`

Follow the pattern established in `deployment.go`:

```go
package kubernetes

import (
    autoscalingv2 "k8s.io/api/autoscaling/v2"
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    "github.com/autops/kure/internal/validation"
)

// CreateHorizontalPodAutoscaler creates an HPA with sensible defaults
func CreateHorizontalPodAutoscaler(name string, namespace string) *autoscalingv2.HorizontalPodAutoscaler {
    obj := &autoscalingv2.HorizontalPodAutoscaler{
        TypeMeta: metav1.TypeMeta{
            Kind:       "HorizontalPodAutoscaler",
            APIVersion: autoscalingv2.SchemeGroupVersion.String(),
        },
        ObjectMeta: metav1.ObjectMeta{
            Name:      name,
            Namespace: namespace,
        },
    }
    return obj
}

// SetScaleTargetRef sets the target reference for the HPA
func SetScaleTargetRef(hpa *autoscalingv2.HorizontalPodAutoscaler, apiVersion, kind, name string) error {
    v := validation.NewValidator()
    if err := v.ValidateHorizontalPodAutoscaler(hpa); err != nil {
        return err
    }
    hpa.Spec.ScaleTargetRef = autoscalingv2.CrossVersionObjectReference{
        APIVersion: apiVersion,
        Kind:       kind,
        Name:       name,
    }
    return nil
}

// SetMinMaxReplicas sets the replica bounds
func SetMinMaxReplicas(hpa *autoscalingv2.HorizontalPodAutoscaler, min, max int32) error {
    v := validation.NewValidator()
    if err := v.ValidateHorizontalPodAutoscaler(hpa); err != nil {
        return err
    }
    hpa.Spec.MinReplicas = &min
    hpa.Spec.MaxReplicas = max
    return nil
}

// AddCPUMetric adds a CPU utilization metric
func AddCPUMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, targetAverageUtilization int32) error {
    v := validation.NewValidator()
    if err := v.ValidateHorizontalPodAutoscaler(hpa); err != nil {
        return err
    }
    metric := autoscalingv2.MetricSpec{
        Type: autoscalingv2.ResourceMetricSourceType,
        Resource: &autoscalingv2.ResourceMetricSource{
            Name: "cpu",
            Target: autoscalingv2.MetricTarget{
                Type:               autoscalingv2.UtilizationMetricType,
                AverageUtilization: &targetAverageUtilization,
            },
        },
    }
    hpa.Spec.Metrics = append(hpa.Spec.Metrics, metric)
    return nil
}

// AddMemoryMetric adds a memory utilization metric
func AddMemoryMetric(hpa *autoscalingv2.HorizontalPodAutoscaler, targetAverageUtilization int32) error {
    v := validation.NewValidator()
    if err := v.ValidateHorizontalPodAutoscaler(hpa); err != nil {
        return err
    }
    metric := autoscalingv2.MetricSpec{
        Type: autoscalingv2.ResourceMetricSourceType,
        Resource: &autoscalingv2.ResourceMetricSource{
            Name: "memory",
            Target: autoscalingv2.MetricTarget{
                Type:               autoscalingv2.UtilizationMetricType,
                AverageUtilization: &targetAverageUtilization,
            },
        },
    }
    hpa.Spec.Metrics = append(hpa.Spec.Metrics, metric)
    return nil
}
```

### 4. Add Unit Tests

**File:** `internal/kubernetes/hpa_test.go`

Test cases:
- CreateHorizontalPodAutoscaler with valid name/namespace
- SetScaleTargetRef with nil HPA (should error)
- SetScaleTargetRef with valid HPA
- SetMinMaxReplicas boundary conditions
- AddCPUMetric / AddMemoryMetric append behavior

## Success Criteria

- [ ] `ErrNilHorizontalPodAutoscaler` added to errors.go
- [ ] `ValidateHorizontalPodAutoscaler` added to validators.go
- [ ] `internal/kubernetes/hpa.go` implements builder pattern
- [ ] Unit tests in `hpa_test.go` with >80% coverage
- [ ] All existing tests pass (`make precommit`)
- [ ] Code follows existing patterns in deployment.go

## Estimated Effort

**1-2 days**
