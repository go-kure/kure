# MetalLB Builders - MetalLB Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/metallb.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/metallb)

The `metallb` package provides strongly-typed constructor functions for creating MetalLB Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each generated constructor returns a MetalLB custom resource carrying its API version, kind and identity, and nothing else. You set the spec yourself, by assignment or through the admissible sugar this package exports.

Each block on this page is the body of an `Example` function in `example_test.go`, which `go test`
runs: it imports this package as `metallb`, the upstream API as
`metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"`, and `fmt` for the line that prints what the
example built.

## Constructors

Every kind this package registers has a generated `Create<Kind>` wrapper in `zz_generated_create.go`, produced from the scheme by `pkg/kubernetes/internal/gen` (`make gen-builders`, checked by `make check-builders` in CI). A wrapper delegates to `kubernetes.Create[T]` and emits **TypeMeta and identity only**: no default, no label, no spec value. Namespaced kinds take `(name, namespace)`, cluster-scoped kinds take `(name)`. The upstream struct is the construction API; set spec fields directly or through the admissible `Set*`/`Add*` sugar below.

<!-- doc-example: pkg/kubernetes/metallb ExampleCreateIPAddressPool -->
```go
obj := metallb.CreateIPAddressPool("my-pool", "metallb-system")
fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
```
<!-- doc-example:end -->

This package never had a config-struct layer: no `metallb.<Kind>(&metallb.<Kind>Config{...})` builder existed here even before release 2 of the builder contract retired that layer everywhere else — the generated constructor plus field assignment has always been the whole construction API. No hand-written `Create*` helper for a spec fragment remains either: a sub-type that is not a `client.Object` takes a struct literal, which is shorter and shows every field being set.

The kinds this package registers, their scope, and what stated that scope are rows in the generated [Supported kinds and field maturity](/api-reference/api-tables/) tables. The sections below are worked examples, not the coverage list.

See the [Kubernetes Builders](/api-reference/kubernetes-builders/) page for the full builder contract: construction, sugar admission classes, purity and the release-1 migration ledger.

## Supported Resources

### IP Address Pools

<!-- doc-example: pkg/kubernetes/metallb ExampleAddIPAddressPoolAddress -->
```go
pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
metallb.AddIPAddressPoolAddress(pool, "192.168.1.0/24")
metallb.AddIPAddressPoolAddress(pool, "10.0.0.0/16")
fmt.Println(pool.Spec.Addresses)
```
<!-- doc-example:end -->

### BGP Peers

<!-- doc-example: pkg/kubernetes/metallb ExampleCreateBGPPeer -->
```go
peer := metallb.CreateBGPPeer("my-peer", "metallb-system")
peer.Spec.MyASN = 64500
peer.Spec.ASN = 64501
peer.Spec.Address = "10.0.0.1"
peer.Spec.Port = 179
fmt.Println(peer.Spec.Address, peer.Spec.ASN)
```
<!-- doc-example:end -->

### BGP Advertisements

<!-- doc-example: pkg/kubernetes/metallb ExampleCreateBGPAdvertisement -->
```go
advert := metallb.CreateBGPAdvertisement("my-advert", "metallb-system")
metallb.AddBGPAdvertisementIPAddressPool(advert, "my-pool")
metallb.AddBGPAdvertisementPeer(advert, "my-peer")
metallb.AddBGPAdvertisementCommunity(advert, "65535:65282")
advert.Spec.LocalPref = 100
fmt.Println(advert.Spec.IPAddressPools, advert.Spec.Peers, advert.Spec.Communities)
```
<!-- doc-example:end -->

### L2 Advertisements

<!-- doc-example: pkg/kubernetes/metallb ExampleCreateL2Advertisement -->
```go
l2 := metallb.CreateL2Advertisement("my-l2", "metallb-system")
metallb.AddL2AdvertisementIPAddressPool(l2, "my-pool")
metallb.AddL2AdvertisementInterface(l2, "eth0")
fmt.Println(l2.Spec.IPAddressPools, l2.Spec.Interfaces)
```
<!-- doc-example:end -->

### BFD Profiles

<!-- doc-example: pkg/kubernetes/metallb ExampleCreateBFDProfile -->
```go
bfd := metallb.CreateBFDProfile("my-bfd", "metallb-system")
metallb.SetBFDProfileDetectMultiplier(bfd, 3)
fmt.Println(*bfd.Spec.DetectMultiplier)
```
<!-- doc-example:end -->

## Modifier Functions

Update existing resources:

<!-- doc-example: pkg/kubernetes/metallb ExampleSetIPAddressPoolAutoAssign -->
```go
pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
peer := metallb.CreateBGPPeer("my-peer", "metallb-system")

// Replace the full spec
pool.Spec = metallbv1beta1.IPAddressPoolSpec{Addresses: []string{"192.168.1.0/24"}}

// Plain fields are assigned directly — there is no Set<Kind><Field> helper
peer.Spec.Port = 1179

// Slice fields keep an appender, and pointer fields a setter
metallb.AddIPAddressPoolAddress(pool, "172.16.0.0/12")
metallb.SetIPAddressPoolAutoAssign(pool, false)

fmt.Println(pool.Spec.Addresses, *pool.Spec.AutoAssign, peer.Spec.Port)
```
<!-- doc-example:end -->

The helpers are void. A nil object is a programming error and panics, like
every other builder in this family.

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource builders
