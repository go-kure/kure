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

## PDB Builders

```go
// Create a PodDisruptionBudget
pdb := kubernetes.CreatePodDisruptionBudget("my-app", "default")

// Set disruption budget (MinAvailable and MaxUnavailable are mutually exclusive)
err := kubernetes.SetPDBMinAvailable(pdb, intstr.FromInt32(2))
// or: err = kubernetes.SetPDBMaxUnavailable(pdb, intstr.FromString("25%"))

// Set the label selector
err = kubernetes.SetPDBSelector(pdb, &metav1.LabelSelector{
    MatchLabels: map[string]string{"app": "my-app"},
})

// Update metadata
err = kubernetes.SetPDBLabels(pdb, map[string]string{"env": "prod"})
err = kubernetes.SetPDBAnnotations(pdb, map[string]string{"owner": "platform"})
```

## Deployment Builders

```go
// Create a Deployment
dep := kubernetes.CreateDeployment("my-app", "default")

// Add a container
container := &corev1.Container{Name: "app", Image: "nginx:1.25"}
err := kubernetes.AddDeploymentContainer(dep, container)

// Set replicas and strategy
err = kubernetes.SetDeploymentReplicas(dep, 3)
err = kubernetes.SetDeploymentStrategy(dep, appsv1.DeploymentStrategy{
    Type: appsv1.RollingUpdateDeploymentStrategyType,
})

// Configure pod template
err = kubernetes.SetDeploymentServiceAccountName(dep, "my-sa")
err = kubernetes.SetDeploymentNodeSelector(dep, map[string]string{"role": "web"})
err = kubernetes.AddDeploymentToleration(dep, &corev1.Toleration{Key: "dedicated", Value: "web"})
```

## CronJob Builders

```go
// Create a CronJob
cj := kubernetes.CreateCronJob("my-job", "default", "*/5 * * * *")

// Add a container
container := &corev1.Container{Name: "worker", Image: "busybox:1.36"}
err := kubernetes.AddCronJobContainer(cj, container)

// Configure schedule and policies
err = kubernetes.SetCronJobConcurrencyPolicy(cj, batchv1.ForbidConcurrent)
err = kubernetes.SetCronJobSuccessfulJobsHistoryLimit(cj, 3)
err = kubernetes.SetCronJobFailedJobsHistoryLimit(cj, 1)

// Configure pod template
err = kubernetes.SetCronJobServiceAccountName(cj, "my-sa")
err = kubernetes.SetCronJobNodeSelector(cj, map[string]string{"role": "batch"})
err = kubernetes.AddCronJobToleration(cj, &corev1.Toleration{Key: "dedicated", Value: "batch"})
```

## Service Builders

```go
// Create a Service
svc := kubernetes.CreateService("my-app", "default")

// Configure the service
err := kubernetes.SetServiceSelector(svc, map[string]string{"app": "my-app"})
err = kubernetes.AddServicePort(svc, corev1.ServicePort{
    Name:       "http",
    Port:       80,
    TargetPort: intstr.FromInt32(8080),
})
err = kubernetes.SetServiceType(svc, corev1.ServiceTypeLoadBalancer)

// Update metadata
err = kubernetes.AddServiceLabel(svc, "env", "prod")
err = kubernetes.AddServiceAnnotation(svc, "external-dns.alpha.kubernetes.io/hostname", "app.example.com")
```

## Ingress Builders

```go
// Create an Ingress
ing := kubernetes.CreateIngress("my-app", "default", "nginx")

// Build a rule with paths
rule := kubernetes.CreateIngressRule("app.example.com")
pt := netv1.PathTypePrefix
path := kubernetes.CreateIngressPath("/", &pt, "my-app", "http")
kubernetes.AddIngressRulePath(rule, path)
err := kubernetes.AddIngressRule(ing, rule)

// Add TLS
err = kubernetes.AddIngressTLS(ing, netv1.IngressTLS{
    Hosts:      []string{"app.example.com"},
    SecretName: "my-app-tls",
})
```

## Related Packages

- [fluxcd](fluxcd/) - FluxCD resource constructors
- [errors](../errors/) - Structured error types used for nil-check sentinels
