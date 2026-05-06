# Kubernetes Builders - Core Resource Helpers

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes)

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

## NetworkPolicy Builders

```go
// Create a NetworkPolicy
np := kubernetes.CreateNetworkPolicy("my-app", "default")

// Set pod selector and policy types
err := kubernetes.SetNetworkPolicyPodSelector(np, metav1.LabelSelector{
    MatchLabels: map[string]string{"app": "my-app"},
})
err = kubernetes.AddNetworkPolicyPolicyType(np, netv1.PolicyTypeIngress)
err = kubernetes.AddNetworkPolicyPolicyType(np, netv1.PolicyTypeEgress)

// Build an ingress rule with peers and ports
ingressRule := netv1.NetworkPolicyIngressRule{}
kubernetes.AddNetworkPolicyIngressPeer(&ingressRule, netv1.NetworkPolicyPeer{
    PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "frontend"}},
})
kubernetes.AddNetworkPolicyIngressPort(&ingressRule, netv1.NetworkPolicyPort{})
err = kubernetes.AddNetworkPolicyIngressRule(np, ingressRule)
```

## HTTPRoute Builders

```go
// Create an HTTPRoute
route := kubernetes.CreateHTTPRoute("my-app", "default")

// Add parent gateway reference and hostname
err := kubernetes.AddHTTPRouteParentRef(route, gwapiv1.ParentReference{Name: "my-gateway"})
err = kubernetes.AddHTTPRouteHostname(route, "app.example.com")

// Build a rule with match, filter, and backend ref
rule := gwapiv1.HTTPRouteRule{}
pathType := gwapiv1.PathMatchPathPrefix
kubernetes.AddHTTPRouteRuleMatch(&rule, gwapiv1.HTTPRouteMatch{
    Path: &gwapiv1.HTTPPathMatch{Type: &pathType, Value: ptrStr("/api")},
})
kubernetes.AddHTTPRouteRuleFilter(&rule, gwapiv1.HTTPRouteFilter{
    Type: gwapiv1.HTTPRouteFilterRequestHeaderModifier,
    RequestHeaderModifier: &gwapiv1.HTTPHeaderFilter{
        Set: []gwapiv1.HTTPHeader{{Name: "X-Custom", Value: "val"}},
    },
})
kubernetes.AddHTTPRouteRuleBackendRef(&rule, gwapiv1.HTTPBackendRef{
    BackendRef: gwapiv1.BackendRef{
        BackendObjectReference: gwapiv1.BackendObjectReference{Name: "my-svc"},
    },
})
err = kubernetes.AddHTTPRouteRule(route, rule)
```

## Namespace Builder

Create and configure Kubernetes Namespaces, including Pod Security Admission (PSA) label management.

```go
// Create a Namespace with default app label and annotation
ns := kubernetes.CreateNamespace("my-app")

// Add or replace labels and annotations
kubernetes.AddNamespaceLabel(ns, "env", "prod")
kubernetes.AddNamespaceAnnotation(ns, "owner", "platform-team")
kubernetes.SetNamespaceLabels(ns, map[string]string{"app": "my-app", "env": "prod"})
kubernetes.SetNamespaceAnnotations(ns, map[string]string{"owner": "platform-team"})

// Manage finalizers
kubernetes.AddNamespaceFinalizer(ns, corev1.FinalizerKubernetes)
kubernetes.SetNamespaceFinalizers(ns, []corev1.FinalizerName{"custom-finalizer"})

// Apply PSA admission labels (enforce, warn, audit modes)
kubernetes.SetNamespacePSALabels(ns,
    kubernetes.PSARestricted,  // enforce
    kubernetes.PSARestricted,  // warn
    kubernetes.PSARestricted,  // audit
    "v1.28",                   // version (pass "" to omit version labels)
)

// Skip a mode by passing an empty string
kubernetes.SetNamespacePSALabels(ns, kubernetes.PSARestricted, "", "", "latest")
```

## PSA Security Context Helpers

Helpers for Pod Security Admission (PSA) compliance at Restricted, Baseline, and Privileged levels.

```go
// Get a security context matching the Restricted PSA level
sc := kubernetes.RestrictedSecurityContext()

// Get a pod-level security context
psc := kubernetes.RestrictedPodSecurityContext()

// Level-based selection
sc := kubernetes.SecurityContextForLevel(kubernetes.PSALevelBaseline)
psc := kubernetes.PodSecurityContextForLevel(kubernetes.PSALevelRestricted)

// Validate a container against a PSA level
err := kubernetes.ValidateContainerPSA(container, kubernetes.PSALevelRestricted)

// Validate an entire PodSpec
violations := kubernetes.ValidatePodSpecPSA(podSpec, kubernetes.PSALevelRestricted)
```

## ResourceRequirements Builder

Build Kubernetes resource requirements for containers.

```go
reqs := kubernetes.CreateResourceRequirements()
kubernetes.SetResourceRequestCPU(reqs, "100m")
kubernetes.SetResourceRequestMemory(reqs, "128Mi")
kubernetes.SetResourceLimitCPU(reqs, "500m")
kubernetes.SetResourceLimitMemory(reqs, "512Mi")
kubernetes.SetResourceRequestEphemeralStorage(reqs, "1Gi")
kubernetes.SetResourceLimitEphemeralStorage(reqs, "2Gi")
```

## Prometheus Builders

Builders for Prometheus Operator CRDs are in the `pkg/kubernetes/prometheus/` sub-package:

```go
import "github.com/go-kure/kure/pkg/kubernetes/prometheus"

sm := prometheus.CreateServiceMonitor("my-app", "monitoring")
prometheus.AddServiceMonitorEndpoint(sm, "/metrics", "http", "30s")
prometheus.SetServiceMonitorSelector(sm, map[string]string{"app": "my-app"})

pm := prometheus.CreatePodMonitor("my-app", "monitoring")
rule := prometheus.CreatePrometheusRule("alerts", "monitoring")
```

## ConfigMap Builders

```go
// Create a ConfigMap
cm := kubernetes.CreateConfigMap("my-config", "default")

// Add or replace string data
kubernetes.AddConfigMapData(cm, "key", "value")
kubernetes.AddConfigMapDataMap(cm, map[string]string{"a": "1", "b": "2"})
kubernetes.SetConfigMapData(cm, map[string]string{"x": "y"})

// Add or replace binary data
kubernetes.AddConfigMapBinaryData(cm, "cert", certBytes)
kubernetes.AddConfigMapBinaryDataMap(cm, map[string][]byte{"p12": p12Bytes})
kubernetes.SetConfigMapBinaryData(cm, map[string][]byte{"tls.key": keyBytes})

// Mark as immutable
kubernetes.SetConfigMapImmutable(cm, true)

// Update metadata
kubernetes.AddConfigMapLabel(cm, "env", "prod")
kubernetes.AddConfigMapAnnotation(cm, "owner", "platform")
kubernetes.SetConfigMapLabels(cm, map[string]string{"app": "my-config"})
kubernetes.SetConfigMapAnnotations(cm, map[string]string{"managed-by": "crane"})
```

## Related Packages

- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource constructors
- [prometheus](/api-reference/prometheus-builders/) - Prometheus Operator CRD builders
- [errors](../errors/) - Structured error types used for nil-check sentinels
