# MetalLB Builders - MetalLB Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/metallb.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/metallb)

The `metallb` package provides strongly-typed constructor functions for creating MetalLB Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each config-struct builder takes a configuration struct and returns a populated MetalLB custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := metallb.CreateIPAddressPool("my-pool", "metallb-system")
```

The config-struct builders (`metallb.IPAddressPool(&metallb.IPAddressPoolConfig{...})`) are a separate, opinionated layer on top of the same upstream types; they are unchanged by the generated constructors. No hand-written `Create*` helper for a spec fragment remains — a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

## Supported Resources

### IP Address Pools

```go
import "github.com/go-kure/kure/pkg/kubernetes/metallb"

pool := metallb.IPAddressPool(&metallb.IPAddressPoolConfig{
    Name:      "my-pool",
    Namespace: "metallb-system",
    Addresses: []string{"192.168.1.0/24", "10.0.0.0/16"},
})
```

### BGP Peers

```go
peer := metallb.BGPPeer(&metallb.BGPPeerConfig{
    Name:      "my-peer",
    Namespace: "metallb-system",
    MyASN:     64500,
    ASN:       64501,
    Address:   "10.0.0.1",
    Port:      179,
})
```

### BGP Advertisements

```go
advert := metallb.BGPAdvertisement(&metallb.BGPAdvertisementConfig{
    Name:           "my-advert",
    Namespace:      "metallb-system",
    IPAddressPools: []string{"my-pool"},
    Peers:          []string{"my-peer"},
    Communities:    []string{"65535:65282"},
    LocalPref:      100,
})
```

### L2 Advertisements

```go
l2 := metallb.L2Advertisement(&metallb.L2AdvertisementConfig{
    Name:           "my-l2",
    Namespace:      "metallb-system",
    IPAddressPools: []string{"my-pool"},
    Interfaces:     []string{"eth0"},
})
```

### BFD Profiles

```go
detectMult := uint32(3)
bfd := metallb.BFDProfile(&metallb.BFDProfileConfig{
    Name:             "my-bfd",
    Namespace:        "metallb-system",
    DetectMultiplier: &detectMult,
})
```

## Modifier Functions

Update existing resources:

```go
// Replace full spec
pool.Spec = newSpec
peer.Spec = newSpec

// Plain fields are assigned directly — there is no Set<Kind><Field> helper
peer.Spec.Port = 1179

// Slice fields keep an appender, and pointer fields a setter
metallb.AddIPAddressPoolAddress(pool, "172.16.0.0/12")
metallb.AddBGPAdvertisementPeer(advert, "peer-2")
metallb.AddL2AdvertisementInterface(l2, "eth1")
metallb.SetIPAddressPoolAutoAssign(pool, false)
```

The helpers are void. A nil object is a programming error and panics, like
every other builder in this family.

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource builders
