# Cilium Builders - Cilium Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/cilium.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/cilium)

The `cilium` package provides the generated constructors and the admissible sugar for Cilium Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Every kind is built the same way: the generated `Create<Kind>` wrapper gives you an object carrying identity only, and the upstream `ciliumv2` struct (or `policy/api.Rule` for the two policy kinds) is the construction API — set `Spec` fields directly, or through the `Set*`/`Add*` helpers the builder contract admits.

Each block on this page is the body of an `Example` function in `example_test.go`, which `go test`
runs: it imports this package as `cilium`, the upstream APIs as
`ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"`,
`slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"` and
`github.com/cilium/cilium/pkg/policy/api`, and `fmt` for the line that prints what the example
built.

All builders emit `cilium.io/v2`. That requires **Cilium 1.18 or newer** on the target cluster: BGPv2, `CiliumCIDRGroup` and `CiliumLoadBalancerIPPool` were served only as `cilium.io/v2alpha1` through 1.17 and were promoted to `v2` in 1.18. The tested range is tracked in [the compatibility matrix](/api-reference/compatibility/).

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`.

<!-- doc-example: pkg/kubernetes/cilium ExampleCreateCiliumNetworkPolicy -->
```go
obj := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
cl := cilium.CreateCiliumCIDRGroup("internal-ranges")
fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name, cl.Kind, cl.Name)
```
<!-- doc-example:end -->

There is no second construction path.
The config-struct layer this package used to carry (`cilium.CiliumNetworkPolicy(&cilium.CiliumNetworkPolicyConfig{...})` and its twelve siblings) was retired by release 2 of the builder contract: every one of the thirteen was `obj.Spec = cfg.Spec` behind a `Name` and a `Namespace`, so it was retired for uniformity rather than reach. <!-- doc-api-refs:ignore names the retired config-struct layer -->
The [release 2 migration notes](/concepts/builder-contract-release-2/) map every removed field to the upstream field that replaces it. No hand-written `Create*` helper for a spec fragment remains either — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the migration ledgers.

## Supported Resources

### CiliumNetworkPolicy

Namespace-scoped policy using the full `api.Rule` spec. `Spec` is `*api.Rule`, so the whole-spec setter is admissible sugar (pointer class); `Specs` is the multi-rule alternative and takes an appender:

<!-- doc-example: pkg/kubernetes/cilium ExampleSetCiliumNetworkPolicySpec -->
```go
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
fmt.Println(len(policy.Spec.Ingress))
```
<!-- doc-example:end -->

### CiliumClusterwideNetworkPolicy

Cluster-scoped policy — same spec as CNP, plus `NodeSelector` support:

<!-- doc-example: pkg/kubernetes/cilium ExampleSetCiliumClusterwideNetworkPolicySpec -->
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
fmt.Println(ccnp.Spec.Ingress[0].FromEntities)
```
<!-- doc-example:end -->

### CiliumCIDRGroup

Cluster-scoped CIDR collection, referenced by `toCIDRSet` rules:

<!-- doc-example: pkg/kubernetes/cilium ExampleCreateCiliumCIDRGroup -->
```go
group := cilium.CreateCiliumCIDRGroup("internal-ranges")
group.Spec.ExternalCIDRs = []api.CIDR{
    "10.0.0.0/8",
    "192.168.0.0/16",
    "172.16.0.0/12",
}
fmt.Println(group.Spec.ExternalCIDRs)
```
<!-- doc-example:end -->

### CiliumEgressGatewayPolicy

Cluster-scoped policy that routes egress traffic through a gateway node:

<!-- doc-example: pkg/kubernetes/cilium ExampleCreateCiliumEgressGatewayPolicy -->
```go
cegp := cilium.CreateCiliumEgressGatewayPolicy("prod-egress")
cegp.Spec = ciliumv2.CiliumEgressGatewayPolicySpec{
    DestinationCIDRs: []ciliumv2.CIDR{"0.0.0.0/0"},
    EgressGateway:    &ciliumv2.EgressGateway{Interface: "eth0"},
}
fmt.Println(cegp.Spec.DestinationCIDRs, cegp.Spec.EgressGateway.Interface)
```
<!-- doc-example:end -->

### CiliumLocalRedirectPolicy

Namespace-scoped policy that redirects traffic to a local backend:

<!-- doc-example: pkg/kubernetes/cilium ExampleCreateCiliumLocalRedirectPolicy -->
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
fmt.Println(lrp.Spec.RedirectFrontend.AddressMatcher.IP, lrp.Spec.RedirectBackend.LocalEndpointSelector.MatchLabels["k8s-app"])
```
<!-- doc-example:end -->

### CiliumLoadBalancerIPPool

Cluster-scoped pool of IP addresses for LoadBalancer services:

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumLoadBalancerIPPoolBlock -->
```go
pool := cilium.CreateCiliumLoadBalancerIPPool("public-pool")
pool.Spec = ciliumv2.CiliumLoadBalancerIPPoolSpec{
    Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
}
// Add blocks incrementally:
cilium.AddCiliumLoadBalancerIPPoolBlock(pool, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "198.51.100.0/24"})
fmt.Println(pool.Spec.Blocks[0].Cidr, pool.Spec.Blocks[1].Cidr)
```
<!-- doc-example:end -->

### CiliumEnvoyConfig

Namespace-scoped Envoy proxy configuration:

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumEnvoyConfigService -->
```go
// An Envoy listener, cluster or route, wrapped in its protobuf Any.
var xdsResource ciliumv2.XDSResource

cec := cilium.CreateCiliumEnvoyConfig("my-proxy", "default")
cilium.AddCiliumEnvoyConfigService(cec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
cilium.AddCiliumEnvoyConfigResource(cec, xdsResource)
fmt.Println(cec.Spec.Services[0].Name, len(cec.Spec.Resources))
```
<!-- doc-example:end -->

### CiliumClusterwideEnvoyConfig

Cluster-scoped Envoy proxy configuration with the same spec shape as `CiliumEnvoyConfig`:

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumClusterwideEnvoyConfigService -->
```go
ccec := cilium.CreateCiliumClusterwideEnvoyConfig("cluster-proxy")
cilium.AddCiliumClusterwideEnvoyConfigService(ccec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
fmt.Println(ccec.Spec.Services[0].Namespace + "/" + ccec.Spec.Services[0].Name)
```
<!-- doc-example:end -->

### CiliumBGPClusterConfig

Cluster-scoped BGP configuration selecting nodes and defining BGP instances:

<!-- doc-example: pkg/kubernetes/cilium ExampleSetCiliumBGPClusterConfigNodeSelector -->
```go
bgpcc := cilium.CreateCiliumBGPClusterConfig("default-bgp")
cilium.SetCiliumBGPClusterConfigNodeSelector(bgpcc, &slimv1.LabelSelector{
    MatchLabels: map[string]string{"bgp": "enabled"},
})
cilium.AddCiliumBGPClusterConfigBGPInstance(bgpcc, ciliumv2.CiliumBGPInstance{
    Name: "instance-65000",
})
fmt.Println(bgpcc.Spec.NodeSelector.MatchLabels["bgp"], bgpcc.Spec.BGPInstances[0].Name)
```
<!-- doc-example:end -->

### CiliumBGPPeerConfig

Cluster-scoped BGP peer configuration (transport, timers, families):

<!-- doc-example: pkg/kubernetes/cilium ExampleSetCiliumBGPPeerConfigEBGPMultihop -->
```go
peer := cilium.CreateCiliumBGPPeerConfig("peer-65001")
cilium.SetCiliumBGPPeerConfigEBGPMultihop(peer, 2)
cilium.AddCiliumBGPPeerConfigFamily(peer, ciliumv2.CiliumBGPFamilyWithAdverts{
    CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
})
fmt.Println(*peer.Spec.EBGPMultihop, peer.Spec.Families[0].Afi)
```
<!-- doc-example:end -->

### CiliumBGPAdvertisement

Cluster-scoped BGP advertisement configuration:

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumBGPAdvertisementEntry -->
```go
advert := cilium.CreateCiliumBGPAdvertisement("pod-cidr-advert")
cilium.AddCiliumBGPAdvertisementEntry(advert, ciliumv2.BGPAdvertisement{
    AdvertisementType: ciliumv2.BGPPodCIDRAdvert,
})
fmt.Println(advert.Spec.Advertisements[0].AdvertisementType)
```
<!-- doc-example:end -->

### CiliumBGPNodeConfig

Cluster-scoped per-node BGP configuration (typically managed by the Cilium operator):

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumBGPNodeConfigBGPInstance -->
```go
nc := cilium.CreateCiliumBGPNodeConfig("node-worker-1")
cilium.AddCiliumBGPNodeConfigBGPInstance(nc, ciliumv2.CiliumBGPNodeInstance{
    Name: "instance-65000",
})
fmt.Println(nc.Spec.BGPInstances[0].Name)
```
<!-- doc-example:end -->

### CiliumBGPNodeConfigOverride

Cluster-scoped per-node BGP override for router ID and local AS:

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumBGPNodeConfigOverrideBGPInstance -->
```go
routerID := "10.0.0.1"
override := cilium.CreateCiliumBGPNodeConfigOverride("node-worker-1")
cilium.AddCiliumBGPNodeConfigOverrideBGPInstance(override, ciliumv2.CiliumBGPNodeConfigInstanceOverride{
    Name:     "instance-65000",
    RouterID: &routerID,
})
fmt.Println(*override.Spec.BGPInstances[0].RouterID)
```
<!-- doc-example:end -->

## Modifier Functions

Update existing resources after construction:

<!-- doc-example: pkg/kubernetes/cilium ExampleAddCiliumNetworkPolicySpec -->
```go
policy := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
ccnp := cilium.CreateCiliumClusterwideNetworkPolicy("allow-health-checks")
group := cilium.CreateCiliumCIDRGroup("internal-ranges")
cegp := cilium.CreateCiliumEgressGatewayPolicy("prod-egress")
pool := cilium.CreateCiliumLoadBalancerIPPool("public-pool")
cec := cilium.CreateCiliumEnvoyConfig("my-proxy", "default")
bgpcc := cilium.CreateCiliumBGPClusterConfig("default-bgp")
peer := cilium.CreateCiliumBGPPeerConfig("peer-65001")
advert := cilium.CreateCiliumBGPAdvertisement("pod-cidr-advert")
nc := cilium.CreateCiliumBGPNodeConfig("node-worker-1")
override := cilium.CreateCiliumBGPNodeConfigOverride("node-worker-1")

selector := &slimv1.LabelSelector{MatchLabels: map[string]string{"bgp": "enabled"}}
multihop := int32(2)

// Network policies
cilium.SetCiliumNetworkPolicySpec(policy, &api.Rule{EndpointSelector: api.NewESFromLabels()})
cilium.AddCiliumNetworkPolicySpec(policy, &api.Rule{EndpointSelector: api.NewESFromLabels()})
cilium.SetCiliumClusterwideNetworkPolicyNodeSelector(ccnp, api.NewESFromLabels())
cilium.AddCiliumNetworkPolicyIngressRule(policy, api.IngressRule{
    IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityCluster}},
})
cilium.AddCiliumNetworkPolicyEgressRule(policy, api.EgressRule{
    EgressCommonRule: api.EgressCommonRule{ToEntities: []api.Entity{api.EntityWorld}},
})
cilium.AddCiliumNetworkPolicyIngressDenyRule(policy, api.IngressDenyRule{
    IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityWorld}},
})
cilium.AddCiliumNetworkPolicyEgressDenyRule(policy, api.EgressDenyRule{
    EgressCommonRule: api.EgressCommonRule{ToEntities: []api.Entity{api.EntityHost}},
})

// CIDR groups
cilium.AddCiliumCIDRGroupCIDR(group, "203.0.113.0/24")
group.Spec.ExternalCIDRs = []api.CIDR{"10.0.0.0/8"}

// Egress gateway
cilium.AddCiliumEgressGatewayPolicySelectorRule(cegp, ciliumv2.EgressRule{PodSelector: selector})
cilium.AddCiliumEgressGatewayPolicyDestinationCIDR(cegp, "10.0.0.0/8")
cilium.SetCiliumEgressGatewayPolicyEgressGateway(cegp, &ciliumv2.EgressGateway{Interface: "eth0"})

// LB IP pool
cilium.AddCiliumLoadBalancerIPPoolBlock(pool, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "198.51.100.0/24"})
cilium.SetCiliumLoadBalancerIPPoolServiceSelector(pool, selector)
pool.Spec.Disabled = true

// Envoy config
cilium.AddCiliumEnvoyConfigService(cec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
cilium.AddCiliumEnvoyConfigBackendService(cec, &ciliumv2.Service{Name: "backend", Namespace: "default"})
cilium.SetCiliumEnvoyConfigNodeSelector(cec, selector)

// BGP
cilium.SetCiliumBGPClusterConfigNodeSelector(bgpcc, selector)
cilium.AddCiliumBGPClusterConfigBGPInstance(bgpcc, ciliumv2.CiliumBGPInstance{Name: "instance-65000"})
cilium.SetCiliumBGPPeerConfigTransport(peer, &ciliumv2.CiliumBGPTransport{})
cilium.SetCiliumBGPPeerConfigTimers(peer, &ciliumv2.CiliumBGPTimers{})
cilium.SetCiliumBGPPeerConfigEBGPMultihop(peer, multihop)
cilium.AddCiliumBGPPeerConfigFamily(peer, ciliumv2.CiliumBGPFamilyWithAdverts{
    CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv6", Safi: "unicast"},
})
cilium.AddCiliumBGPAdvertisementEntry(advert, ciliumv2.BGPAdvertisement{AdvertisementType: ciliumv2.BGPServiceAdvert})
cilium.AddCiliumBGPNodeConfigBGPInstance(nc, ciliumv2.CiliumBGPNodeInstance{Name: "instance-65000"})
cilium.AddCiliumBGPNodeConfigOverrideBGPInstance(override, ciliumv2.CiliumBGPNodeConfigInstanceOverride{Name: "instance-65000"})

fmt.Println(len(policy.Specs), len(policy.Spec.Ingress), len(policy.Spec.EgressDeny), cegp.Spec.DestinationCIDRs, pool.Spec.Disabled)
```
<!-- doc-example:end -->

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [metallb](/api-reference/metallb-builders/) - MetalLB resource builders
- [Builder contract: release 2 migration notes](/concepts/builder-contract-release-2/) - the retired config-struct layer, field by field
