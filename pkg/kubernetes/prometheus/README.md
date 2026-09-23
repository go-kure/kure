# Prometheus Builders - Prometheus Operator CRD Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/prometheus.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/prometheus)

The `prometheus` package provides the generated constructors and the admissible sugar for Prometheus Operator Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Every kind is built the same way: the generated `Create<Kind>` wrapper gives you an object carrying identity only, and the upstream `monitoringv1` struct is the construction API — set `Spec` fields directly, or through the few `Set*`/`Add*` helpers the builder contract admits.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`.

```go
obj := prometheus.CreateServiceMonitor("my-app", "monitoring")
```

There is no second construction path. The config-struct layer this package used to carry (`prometheus.ServiceMonitor(&prometheus.ServiceMonitorConfig{...})`, `PodMonitor`, `PrometheusRule`) was retired by release 2 of the builder contract: it reached 6 of a `ServiceMonitor`'s 19 spec fields and 6 of a `PodMonitor`'s 17, and its `Labels` field was `metadata.labels` under another name. The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. One hand-written constructor for a spec fragment remains, `CreateRuleGroup`: `monitoringv1.RuleGroup` is not a `client.Object`, so it gets no generated wrapper, and the helper stays because a rule group is the unit a caller assembles repeatedly. Every other sub-type takes a struct literal.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

### ServiceMonitor

```go
import (
    monitoringv1 "github.com/prometheus-operator/prometheus-operator/pkg/apis/monitoring/v1"

    "github.com/go-kure/kure/pkg/kubernetes"
    "github.com/go-kure/kure/pkg/kubernetes/prometheus"
)

sm := prometheus.CreateServiceMonitor("my-app", "monitoring")
kubernetes.AddLabel(sm, "team", "platform")
sm.Spec = monitoringv1.ServiceMonitorSpec{
    Selector:     metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
    Endpoints:    []monitoringv1.Endpoint{{Path: "/metrics", Port: "http", Interval: "30s"}},
    JobLabel:     "app",
    TargetLabels: []string{"app", "version"},
}
prometheus.SetServiceMonitorSampleLimit(sm, 10000)
```

`spec.endpoints` carries no `omitempty` upstream, so a ServiceMonitor with no endpoints renders `endpoints: null`; the same holds for a PodMonitor's `podMetricsEndpoints`.

### PodMonitor

```go
pm := prometheus.CreatePodMonitor("my-app", "monitoring")
pm.Spec = monitoringv1.PodMonitorSpec{
    Selector:            metav1.LabelSelector{MatchLabels: map[string]string{"app": "my-app"}},
    PodMetricsEndpoints: []monitoringv1.PodMetricsEndpoint{{Path: "/metrics", Port: ptr.To("http"), Interval: "30s"}},
    NamespaceSelector:   monitoringv1.NamespaceSelector{Any: true},
}
```

### PrometheusRule

```go
rule := prometheus.CreatePrometheusRule("alerts", "monitoring")
kubernetes.AddLabel(rule, "role", "alert-rules")

group := prometheus.CreateRuleGroup("my-app.rules")
prometheus.AddRuleGroupRule(&group, monitoringv1.Rule{
    Alert: "HighErrorRate",
    Expr:  intstr.FromString(`rate(http_requests_total{code=~"5.."}[5m]) > 0.1`),
    For:   ptr.To(monitoringv1.Duration("5m")),
})
prometheus.AddPrometheusRuleGroup(rule, group)
```

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
prometheus.AddPodMonitorEndpoint(pm, monitoringv1.PodMetricsEndpoint{Path: "/metrics", Port: ptr.To("http")})
pm.Spec.JobLabel = "app"
prometheus.AddPodMonitorPodTargetLabel(pm, "version")
pm.Spec.NamespaceSelector = monitoringv1.NamespaceSelector{Any: true}
prometheus.SetPodMonitorSampleLimit(pm, 10000)

// PrometheusRule modifiers
prometheus.AddPrometheusRuleGroup(rule, monitoringv1.RuleGroup{Name: "extra.rules"})
prometheus.SetRuleGroupInterval(&group, monitoringv1.Duration("1m"))
```

## Related Packages

- [kubernetes-builders](/api-reference/kubernetes-builders/) - Core Kubernetes resource constructors
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) - the retired config-struct layer, field by field
