# Cilium Builders - Cilium Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/cilium.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cilium)

The `cilium` package provides the generated constructors and the admissible sugar for Cilium Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Every kind is built the same way: the generated `Create<Kind>` wrapper gives you an object carrying identity only, and the upstream `ciliumv2` struct (or `policy/api.Rule` for the two policy kinds) is the construction API — set `Spec` fields directly, or through the `Set*`/`Add*` helpers the builder contract admits.

All builders emit `cilium.io/v2`. That requires **Cilium 1.18 or newer** on the target cluster: BGPv2, `CiliumCIDRGroup` and `CiliumLoadBalancerIPPool` were served only as `cilium.io/v2alpha1` through 1.17 and were promoted to `v2` in 1.18. The tested range is tracked in [the compatibility matrix](/api-reference/compatibility/).

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`.

```go
obj := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
cl := cilium.CreateCiliumCIDRGroup("internal-ranges")
```

There is no second construction path. <!-- doc-api-refs:ignore names the retired config-struct layer --> The config-struct layer this package used to carry (`cilium.CiliumNetworkPolicy(&cilium.CiliumNetworkPolicyConfig{...})` and its twelve siblings) was retired by release 2 of the builder contract: every one of the thirteen was `obj.Spec = cfg.Spec` behind a `Name` and a `Namespace`, so it was retired for uniformity rather than reach. The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. No hand-written `Create*` helper for a spec fragment remains either — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

### CiliumNetworkPolicy

Namespace-scoped policy using the full `api.Rule` spec. `Spec` is `*api.Rule`, so the whole-spec setter is admissible sugar (pointer class); `Specs` is the multi-rule alternative and takes an appender:

```go
import (
    "github.com/cilium/cilium/pkg/policy/api"

    "github.com/go-kure/kure/pkg/kubernetes/cilium"
)

policy := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
cilium.SetCiliumNetworkPolicySpec(policy, &api.Rule{
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
})
```

### CiliumClusterwideNetworkPolicy

Cluster-scoped policy — same spec as CNP, plus `NodeSelector` support:

```go
ccnp := cilium.CreateCiliumClusterwideNetworkPolicy("allow-health-checks")
cilium.SetCiliumClusterwideNetworkPolicySpec(ccnp, &api.Rule{
    NodeSelector: api.NewESFromLabels(),
    Ingress: []api.IngressRule{
        {
            IngressCommonRule: api.IngressCommonRule{
                FromEntities: []api.Entity{api.EntityHost},
            },
        },
    },
})
```

### CiliumCIDRGroup

Cluster-scoped CIDR collection, referenced by `toCIDRSet` rules:

```go
group := cilium.CreateCiliumCIDRGroup("internal-ranges")
group.Spec.ExternalCIDRs = []api.CIDR{
    "10.0.0.0/8",
    "192.168.0.0/16",
    "172.16.0.0/12",
}
```

### CiliumEgressGatewayPolicy

Cluster-scoped policy that routes egress traffic through a gateway node:

```go
import ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"

cegp := cilium.CreateCiliumEgressGatewayPolicy("prod-egress")
cegp.Spec = ciliumv2.CiliumEgressGatewayPolicySpec{
    DestinationCIDRs: []ciliumv2.CIDR{"0.0.0.0/0"},
    EgressGateway:    &ciliumv2.EgressGateway{Interface: "eth0"},
}
```

### CiliumLocalRedirectPolicy

Namespace-scoped policy that redirects traffic to a local backend:

```go
lrp := cilium.CreateCiliumLocalRedirectPolicy("dns-redirect", "kube-system")
lrp.Spec = ciliumv2.CiliumLocalRedirectPolicySpec{
    RedirectFrontend: ciliumv2.RedirectFrontend{
        AddressMatcher: &ciliumv2.Frontend{IP: "169.254.20.10", ToPorts: []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}}},
    },
    RedirectBackend: ciliumv2.RedirectBackend{
        LocalEndpointSelector: slimv1.LabelSelector{MatchLabels: map[string]string{"k8s-app": "coredns"}},
        ToPorts:               []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}},
    },
}
```

### CiliumLoadBalancerIPPool

Cluster-scoped pool of IP addresses for LoadBalancer services:

```go
pool := cilium.CreateCiliumLoadBalancerIPPool("public-pool")
pool.Spec = ciliumv2.CiliumLoadBalancerIPPoolSpec{
    Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
}
// Add blocks incrementally:
cilium.AddCiliumLoadBalancerIPPoolBlock(pool, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "198.51.100.0/24"})
```

### CiliumEnvoyConfig

Namespace-scoped Envoy proxy configuration:

```go
cec := cilium.CreateCiliumEnvoyConfig("my-proxy", "default")
cilium.AddCiliumEnvoyConfigService(cec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
cilium.AddCiliumEnvoyConfigResource(cec, xdsResource)
```

### CiliumClusterwideEnvoyConfig

Cluster-scoped Envoy proxy configuration with the same spec shape as `CiliumEnvoyConfig`:

```go
ccec := cilium.CreateCiliumClusterwideEnvoyConfig("cluster-proxy")
cilium.AddCiliumClusterwideEnvoyConfigService(ccec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
```

### CiliumBGPClusterConfig

Cluster-scoped BGP configuration selecting nodes and defining BGP instances:

```go
bgpcc := cilium.CreateCiliumBGPClusterConfig("default-bgp")
cilium.SetCiliumBGPClusterConfigNodeSelector(bgpcc, &slimv1.LabelSelector{
    MatchLabels: map[string]string{"bgp": "enabled"},
})
cilium.AddCiliumBGPClusterConfigBGPInstance(bgpcc, ciliumv2.CiliumBGPInstance{
    Name: "instance-65000",
})
```

### CiliumBGPPeerConfig

Cluster-scoped BGP peer configuration (transport, timers, families):

```go
peer := cilium.CreateCiliumBGPPeerConfig("peer-65001")
cilium.SetCiliumBGPPeerConfigEBGPMultihop(peer, 2)
cilium.AddCiliumBGPPeerConfigFamily(peer, ciliumv2.CiliumBGPFamilyWithAdverts{
    CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
})
```

### CiliumBGPAdvertisement

Cluster-scoped BGP advertisement configuration:

```go
advert := cilium.CreateCiliumBGPAdvertisement("pod-cidr-advert")
cilium.AddCiliumBGPAdvertisementEntry(advert, ciliumv2.BGPAdvertisement{
    AdvertisementType: ciliumv2.BGPPodCIDRAdvert,
})
```

### CiliumBGPNodeConfig

Cluster-scoped per-node BGP configuration (typically managed by the Cilium operator):

```go
nc := cilium.CreateCiliumBGPNodeConfig("node-worker-1")
cilium.AddCiliumBGPNodeConfigBGPInstance(nc, ciliumv2.CiliumBGPNodeInstance{
    Name: "instance-65000",
})
```

### CiliumBGPNodeConfigOverride

Cluster-scoped per-node BGP override for router ID and local AS:

```go
routerID := "10.0.0.1"
override := cilium.CreateCiliumBGPNodeConfigOverride("node-worker-1")
cilium.AddCiliumBGPNodeConfigOverrideBGPInstance(override, ciliumv2.CiliumBGPNodeConfigInstanceOverride{
    Name:     "instance-65000",
    RouterID: &routerID,
})
```

## Modifier Functions

Update existing resources after construction:

```go
// Network policies
cilium.SetCiliumNetworkPolicySpec(policy, newRule)
cilium.AddCiliumNetworkPolicySpec(policy, extraRule)
cilium.SetCiliumClusterwideNetworkPolicyNodeSelector(ccnp, nodeSelector)
cilium.AddCiliumNetworkPolicyIngressRule(policy, ingressRule)
cilium.AddCiliumNetworkPolicyEgressRule(policy, egressRule)
cilium.AddCiliumNetworkPolicyIngressDenyRule(policy, ingressDenyRule)
cilium.AddCiliumNetworkPolicyEgressDenyRule(policy, egressDenyRule)

// CIDR groups
cilium.AddCiliumCIDRGroupCIDR(group, "203.0.113.0/24")
group.Spec.ExternalCIDRs = newCIDRSlice

// Egress gateway
cilium.AddCiliumEgressGatewayPolicySelectorRule(cegp, selectorRule)
cilium.AddCiliumEgressGatewayPolicyDestinationCIDR(cegp, "10.0.0.0/8")
cilium.SetCiliumEgressGatewayPolicyEgressGateway(cegp, &egressGateway)

// LB IP pool
cilium.AddCiliumLoadBalancerIPPoolBlock(pool, block)
cilium.SetCiliumLoadBalancerIPPoolServiceSelector(pool, selector)
pool.Spec.Disabled = true

// Envoy config
cilium.AddCiliumEnvoyConfigService(cec, serviceListener)
cilium.AddCiliumEnvoyConfigBackendService(cec, backendService)
cilium.AddCiliumEnvoyConfigResource(cec, xdsResource)
cilium.SetCiliumEnvoyConfigNodeSelector(cec, nodeSelector)

// BGP
cilium.SetCiliumBGPClusterConfigNodeSelector(bgpcc, nodeSelector)
cilium.AddCiliumBGPClusterConfigBGPInstance(bgpcc, instance)
cilium.SetCiliumBGPPeerConfigTransport(peer, transport)
cilium.SetCiliumBGPPeerConfigTimers(peer, timers)
cilium.AddCiliumBGPPeerConfigFamily(peer, family)
cilium.AddCiliumBGPAdvertisementEntry(advert, advertisement)
cilium.AddCiliumBGPNodeConfigBGPInstance(nc, instance)
cilium.AddCiliumBGPNodeConfigOverrideBGPInstance(override, instanceOverride)
```

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [metallb](/api-reference/metallb-builders/) - MetalLB resource builders
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) - the retired config-struct layer, field by field
