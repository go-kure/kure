# Cilium Builders - Cilium Resource Constructors

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

### CiliumEgressGatewayPolicy

Cluster-scoped policy that routes egress traffic through a gateway node:

```go
import ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"

cegp := cilium.CiliumEgressGatewayPolicy(&cilium.CiliumEgressGatewayPolicyConfig{
    Name: "prod-egress",
    Spec: ciliumv2.CiliumEgressGatewayPolicySpec{
        DestinationCIDRs: []ciliumv2.CIDR{"0.0.0.0/0"},
        EgressGateway:    &ciliumv2.EgressGateway{Interface: "eth0"},
    },
})
```

### CiliumLocalRedirectPolicy

Namespace-scoped policy that redirects traffic to a local backend:

```go
lrp := cilium.CiliumLocalRedirectPolicy(&cilium.CiliumLocalRedirectPolicyConfig{
    Name:      "dns-redirect",
    Namespace: "kube-system",
    Spec: ciliumv2.CiliumLocalRedirectPolicySpec{
        RedirectFrontend: ciliumv2.RedirectFrontend{
            AddressMatcher: &ciliumv2.Frontend{IP: "169.254.20.10", ToPorts: []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}}},
        },
        RedirectBackend: ciliumv2.RedirectBackend{
            LocalEndpointSelector: slimv1.LabelSelector{MatchLabels: map[string]string{"k8s-app": "coredns"}},
            ToPorts:               []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}},
        },
    },
})
```

### CiliumLoadBalancerIPPool

Cluster-scoped pool of IP addresses for LoadBalancer services:

```go
pool := cilium.CiliumLoadBalancerIPPool(&cilium.CiliumLoadBalancerIPPoolConfig{
    Name: "public-pool",
    Spec: ciliumv2.CiliumLoadBalancerIPPoolSpec{
        Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
    },
})
// Add blocks incrementally:
cilium.AddCiliumLoadBalancerIPPoolBlock(pool, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "198.51.100.0/24"})
```

### CiliumEnvoyConfig

Namespace-scoped Envoy proxy configuration:

```go
cec := cilium.CiliumEnvoyConfig(&cilium.CiliumEnvoyConfigConfig{
    Name:      "my-proxy",
    Namespace: "default",
})
cilium.AddCiliumEnvoyConfigService(cec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
cilium.AddCiliumEnvoyConfigResource(cec, xdsResource)
```

### CiliumClusterwideEnvoyConfig

Cluster-scoped Envoy proxy configuration with the same spec shape as `CiliumEnvoyConfig`:

```go
ccec := cilium.CiliumClusterwideEnvoyConfig(&cilium.CiliumClusterwideEnvoyConfigConfig{
    Name: "cluster-proxy",
})
```

### CiliumBGPClusterConfig

Cluster-scoped BGP configuration selecting nodes and defining BGP instances:

```go
bgpcc := cilium.CiliumBGPClusterConfig(&cilium.CiliumBGPClusterConfigConfig{
    Name: "default-bgp",
})
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
peer := cilium.CiliumBGPPeerConfig(&cilium.CiliumBGPPeerConfigConfig{
    Name: "peer-65001",
})
cilium.SetCiliumBGPPeerConfigEBGPMultihop(peer, 2)
cilium.AddCiliumBGPPeerConfigFamily(peer, ciliumv2.CiliumBGPFamilyWithAdverts{
    CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
})
```

### CiliumBGPAdvertisement

Cluster-scoped BGP advertisement configuration:

```go
advert := cilium.CiliumBGPAdvertisement(&cilium.CiliumBGPAdvertisementConfig{
    Name: "pod-cidr-advert",
})
cilium.AddCiliumBGPAdvertisementEntry(advert, ciliumv2.BGPAdvertisement{
    AdvertisementType: ciliumv2.BGPPodCIDRAdvert,
})
```

### CiliumBGPNodeConfig

Cluster-scoped per-node BGP configuration (typically managed by the Cilium operator):

```go
nc := cilium.CiliumBGPNodeConfig(&cilium.CiliumBGPNodeConfigConfig{
    Name: "node-worker-1",
})
cilium.AddCiliumBGPNodeConfigBGPInstance(nc, ciliumv2.CiliumBGPNodeInstance{
    Name: "instance-65000",
})
```

### CiliumBGPNodeConfigOverride

Cluster-scoped per-node BGP override for router ID and local AS:

```go
routerID := "10.0.0.1"
override := cilium.CiliumBGPNodeConfigOverride(&cilium.CiliumBGPNodeConfigOverrideConfig{
    Name: "node-worker-1",
})
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
cilium.SetCiliumClusterwideNetworkPolicyNodeSelector(ccnp, nodeSelector)
cilium.AddCiliumNetworkPolicyIngressRule(policy, ingressRule)
cilium.AddCiliumNetworkPolicyEgressRule(policy, egressRule)
cilium.AddCiliumNetworkPolicyIngressDenyRule(policy, ingressDenyRule)
cilium.AddCiliumNetworkPolicyEgressDenyRule(policy, egressDenyRule)

// CIDR groups
cilium.AddCiliumCIDRGroupCIDR(group, "203.0.113.0/24")
cilium.SetCiliumCIDRGroupCIDRs(group, newCIDRSlice)

// Egress gateway
cilium.AddCiliumEgressGatewayPolicySelectorRule(cegp, selectorRule)
cilium.AddCiliumEgressGatewayPolicyDestinationCIDR(cegp, "10.0.0.0/8")
cilium.SetCiliumEgressGatewayPolicyEgressGateway(cegp, &egressGateway)

// LB IP pool
cilium.AddCiliumLoadBalancerIPPoolBlock(pool, block)
cilium.SetCiliumLoadBalancerIPPoolServiceSelector(pool, selector)
cilium.SetCiliumLoadBalancerIPPoolDisabled(pool, true)

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
