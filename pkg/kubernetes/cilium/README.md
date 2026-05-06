# Cilium Builders - Cilium Network Policy Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/cilium.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cilium)

The `cilium` package provides strongly-typed constructor functions for creating Cilium Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each function takes a configuration struct and returns a fully initialized Cilium custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Supported Resources

### CiliumNetworkPolicy

Namespace-scoped policy using the full `api.Rule` spec:

```go
import (
    "github.com/go-kure/kure/pkg/kubernetes/cilium"
    "github.com/cilium/cilium/pkg/policy/api"
)

policy := cilium.CiliumNetworkPolicy(&cilium.CiliumNetworkPolicyConfig{
    Name:      "allow-internal",
    Namespace: "default",
    Spec: &api.Rule{
        EndpointSelector: api.NewESFromLabels(),
        Ingress: []api.IngressRule{
            {
                IngressCommonRule: api.IngressCommonRule{
                    FromEndpoints: []api.EndpointSelector{
                        api.NewESFromLabels(),
                    },
                },
            },
        },
    },
})
```

### CiliumClusterwideNetworkPolicy

Cluster-scoped policy — same spec as CNP, plus `NodeSelector` support:

```go
ccnp := cilium.CiliumClusterwideNetworkPolicy(&cilium.CiliumClusterwideNetworkPolicyConfig{
    Name: "allow-health-checks",
    Spec: &api.Rule{
        NodeSelector: api.NewESFromLabels(),
        Ingress: []api.IngressRule{
            {
                IngressCommonRule: api.IngressCommonRule{
                    FromEntities: []api.Entity{api.EntityHost},
                },
            },
        },
    },
})
```

### CiliumCIDRGroup

Cluster-scoped CIDR collection, referenced by `toCIDRSet` rules:

```go
group := cilium.CiliumCIDRGroup(&cilium.CiliumCIDRGroupConfig{
    Name: "internal-ranges",
    ExternalCIDRs: []api.CIDR{
        "10.0.0.0/8",
        "192.168.0.0/16",
        "172.16.0.0/12",
    },
})
```

## Modifier Functions

Update existing resources after construction:

```go
// Set or replace spec
cilium.SetCiliumNetworkPolicySpec(policy, newRule)
cilium.SetCiliumClusterwideNetworkPolicyNodeSelector(ccnp, nodeSelector)

// Add individual rules (auto-initialise Spec if nil)
cilium.AddCiliumNetworkPolicyIngressRule(policy, ingressRule)
cilium.AddCiliumNetworkPolicyEgressRule(policy, egressRule)
cilium.AddCiliumNetworkPolicyIngressDenyRule(policy, ingressDenyRule)
cilium.AddCiliumNetworkPolicyEgressDenyRule(policy, egressDenyRule)

// Manage CIDR groups
cilium.AddCiliumCIDRGroupCIDR(group, "203.0.113.0/24")
cilium.SetCiliumCIDRGroupCIDRs(group, newCIDRSlice)
```

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [metallb](/api-reference/metallb-builders/) - MetalLB resource builders
