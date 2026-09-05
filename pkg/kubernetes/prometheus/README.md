# Prometheus Builders - Prometheus Operator CRD Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/prometheus.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/prometheus)

The `prometheus` package provides strongly-typed constructor functions for creating Prometheus Operator Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each config-struct builder takes a configuration struct and returns a populated Prometheus Operator custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := prometheus.CreateServiceMonitor("my-app", "monitoring")
```

The config-struct builders (`prometheus.ServiceMonitor(&prometheus.ServiceMonitorConfig{...})`) are a separate, opinionated layer on top of the same upstream types; they are unchanged by the generated constructors. One hand-written constructor for a spec fragment remains, `CreateRuleGroup`: `monitoringv1.RuleGroup` is not a `client.Object`, so it gets no generated wrapper, and the helper stays because a rule group is the unit a caller assembles repeatedly. Every other sub-type takes a struct literal.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

## Supported Resources

### ServiceMonitor

```go
import "github.com/go-kure/kure/pkg/kubernetes/prometheus"

sm := prometheus.ServiceMonitor(&prometheus.ServiceMonitorConfig{
    Name:      "my-app",
    Namespace: "monitoring",
    Selector:  metav1.LabelSelector{
        MatchLabels: map[string]string{"app": "my-app"},
    },
    Endpoints: []monitoringv1.Endpoint{
        {Path: "/metrics", Port: "http", Interval: "30s"},
    },
    JobLabel:     "app",
    TargetLabels: []string{"app", "version"},
})
```

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Resource name |
| `Namespace` | `string` | Resource namespace |
| `Selector` | `metav1.LabelSelector` | Label selector for target Services |
| `Endpoints` | `[]monitoringv1.Endpoint` | Scrape endpoint configurations |
| `JobLabel` | `string` | Label to use as the Prometheus `job` label |
| `TargetLabels` | `[]string` | Service labels to transfer to scraped metrics |
| `NamespaceSelector` | `*monitoringv1.NamespaceSelector` | Namespaces to select Services from |
| `SampleLimit` | `*int64` | Per-scrape sample limit |
| `Labels` | `map[string]string` | Additional labels for the resource |

### PodMonitor

```go
pm := prometheus.PodMonitor(&prometheus.PodMonitorConfig{
    Name:      "my-app",
    Namespace: "monitoring",
    Selector:  metav1.LabelSelector{
        MatchLabels: map[string]string{"app": "my-app"},
    },
    PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{
        {Path: "/metrics", Port: "http", Interval: "30s"},
    },
})
```

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Resource name |
| `Namespace` | `string` | Resource namespace |
| `Selector` | `metav1.LabelSelector` | Label selector for target Pods |
| `PodMetricsEndpoints` | `[]monitoringv1.PodMetricsEndpoint` | Scrape endpoint configurations |
| `JobLabel` | `string` | Label to use as the Prometheus `job` label |
| `PodTargetLabels` | `[]string` | Pod labels to transfer to scraped metrics |
| `NamespaceSelector` | `*monitoringv1.NamespaceSelector` | Namespaces to select Pods from |
| `SampleLimit` | `*int64` | Per-scrape sample limit |
| `Labels` | `map[string]string` | Additional labels for the resource |

### PrometheusRule

```go
rule := prometheus.PrometheusRule(&prometheus.PrometheusRuleConfig{
    Name:      "alerts",
    Namespace: "monitoring",
    Labels:    map[string]string{"role": "alert-rules"},
    Groups: []monitoringv1.RuleGroup{
        {
            Name: "my-app.rules",
            Rules: []monitoringv1.Rule{
                {
                    Alert: "HighErrorRate",
                    Expr:  intstr.FromString(`rate(http_requests_total{code=~"5.."}[5m]) > 0.1`),
                    For:   (*monitoringv1.Duration)(ptr("5m")),
                },
            },
        },
    },
})
```

| Field | Type | Description |
|-------|------|-------------|
| `Name` | `string` | Resource name |
| `Namespace` | `string` | Resource namespace |
| `Groups` | `[]monitoringv1.RuleGroup` | Rule groups containing alert and recording rules |
| `Labels` | `map[string]string` | Additional labels for the resource |

## Modifier Functions

Update existing resources after construction:

```go
// ServiceMonitor modifiers
prometheus.AddServiceMonitorEndpoint(sm, monitoringv1.Endpoint{Path: "/metrics", Port: "http"})
sm.Spec.JobLabel = "app"
prometheus.AddServiceMonitorTargetLabel(sm, "version")
sm.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{Any: true}
prometheus.SetServiceMonitorSampleLimit(sm, 10000)

// PodMonitor modifiers
prometheus.AddPodMonitorEndpoint(pm, monitoringv1.PodMetricsEndpoint{Path: "/metrics", Port: "http"})
pm.Spec.JobLabel = "app"
prometheus.AddPodMonitorPodTargetLabel(pm, "version")
pm.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{Any: true}
prometheus.SetPodMonitorSampleLimit(pm, 10000)

// PrometheusRule modifiers
prometheus.AddPrometheusRuleGroup(rule, monitoringv1.RuleGroup{Name: "extra.rules"})
```

## Related Packages

- [kubernetes-builders](/api-reference/kubernetes-builders/) - Core Kubernetes resource constructors
