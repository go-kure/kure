# MetalLB Builders - MetalLB Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/metallb.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/metallb)

The `metallb` package provides strongly-typed constructor functions for creating MetalLB Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each generated constructor returns a MetalLB custom resource carrying its API version, kind and identity, and nothing else. You set the spec yourself, by assignment or through the admissible sugar this package exports.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

```go
obj := metallb.CreateIPAddressPool("my-pool", "metallb-system")
```

This package has no config-struct layer. Unlike `cilium` and `prometheus`, it exports no `metallb.<Kind>(&metallb.<Kind>Config{...})` builder, and never has — the generated constructor plus field assignment is the whole construction API here. No hand-written `Create*` helper for a spec fragment remains either: a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

## Supported Resources

### IP Address Pools

```go
import "github.com/go-kure/kure/pkg/kubernetes/metallb"

pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
metallb.AddIPAddressPoolAddress(pool, "192.168.1.0/24")
metallb.AddIPAddressPoolAddress(pool, "10.0.0.0/16")
```

### BGP Peers

```go
peer := metallb.CreateBGPPeer("my-peer", "metallb-system")
peer.Spec.MyASN = 64500
peer.Spec.ASN = 64501
peer.Spec.Address = "10.0.0.1"
peer.Spec.Port = 179
```

### BGP Advertisements

```go
advert := metallb.CreateBGPAdvertisement("my-advert", "metallb-system")
metallb.AddBGPAdvertisementIPAddressPool(advert, "my-pool")
metallb.AddBGPAdvertisementPeer(advert, "my-peer")
metallb.AddBGPAdvertisementCommunity(advert, "65535:65282")
advert.Spec.LocalPref = 100
```

### L2 Advertisements

```go
l2 := metallb.CreateL2Advertisement("my-l2", "metallb-system")
metallb.AddL2AdvertisementIPAddressPool(l2, "my-pool")
metallb.AddL2AdvertisementInterface(l2, "eth0")
```

### BFD Profiles

```go
bfd := metallb.CreateBFDProfile("my-bfd", "metallb-system")
metallb.SetBFDProfileDetectMultiplier(bfd, 3)
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
