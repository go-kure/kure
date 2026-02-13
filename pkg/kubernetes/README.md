# Kubernetes Builders - Core Resource Helpers

The `kubernetes` package provides GVK utilities, scheme registration, and strongly-typed builder functions for core Kubernetes resources.

## Overview

This package exposes helpers that other Kure packages (and external consumers such as Crane) use to construct and inspect Kubernetes objects without dealing with low-level struct details.

## Import

```go
import "github.com/go-kure/kure/pkg/kubernetes"
```

## GVK Utilities

```go
// Resolve the GVK of any registered runtime.Object
gvk, err := kubernetes.GetGroupVersionKind(myDeployment)

// Check if a GVK is in an allow list
ok := kubernetes.IsGVKAllowed(gvk, allowedGVKs)
```

## Scheme Registration

```go
// Lazily registers all supported API groups (core K8s, FluxCD, cert-manager, etc.)
err := kubernetes.RegisterSchemes()
```

## HPA Builders

```go
// Create a HorizontalPodAutoscaler
hpa := kubernetes.CreateHorizontalPodAutoscaler("my-app", "default")

// Set the scale target
err := kubernetes.SetHPAScaleTargetRef(hpa, "apps/v1", "Deployment", "my-app")

// Set replica bounds
err = kubernetes.SetHPAMinMaxReplicas(hpa, 2, 10)

// Add CPU and memory metrics
err = kubernetes.AddHPACPUMetric(hpa, 80)
err = kubernetes.AddHPAMemoryMetric(hpa, 70)

// Set scaling behavior
window := int32(300)
err = kubernetes.SetHPABehavior(hpa, &autoscalingv2.HorizontalPodAutoscalerBehavior{
    ScaleDown: &autoscalingv2.HPAScalingRules{
        StabilizationWindowSeconds: &window,
    },
})

// Update metadata
err = kubernetes.SetHPALabels(hpa, map[string]string{"env": "prod"})
err = kubernetes.SetHPAAnnotations(hpa, map[string]string{"owner": "platform"})
```

## Related Packages

- [fluxcd](fluxcd/) - FluxCD resource constructors
- [errors](../errors/) - Structured error types used for nil-check sentinels
