# MetalLB Builders - MetalLB Resource Constructors

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/kubernetes/metallb.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/kubernetes/metallb)

The `metallb` package provides strongly-typed constructor functions for creating MetalLB Kubernetes resources. These are the low-level building blocks used by Kure's higher-level stack and workflow packages.

## Overview

Each function takes a configuration struct and returns a fully initialized MetalLB custom resource. The builders handle API version and kind metadata, letting you focus on the resource specification.

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
metallb.SetIPAddressPoolSpec(pool, newSpec)
metallb.SetBGPPeerSpec(peer, newSpec)

// Granular updates
err := metallb.AddIPAddressPoolAddress(pool, "172.16.0.0/12")
err := metallb.SetBGPPeerPort(peer, 1179)
err := metallb.AddBGPAdvertisementPeer(advert, "peer-2")
err := metallb.AddL2AdvertisementInterface(l2, "eth1")
err := metallb.SetBFDProfileDetectMultiplier(bfd, 5)
```

## Related Packages

- [kubernetes](/api-reference/kubernetes-builders/) - Core Kubernetes resource builders
- [fluxcd](/api-reference/fluxcd-builders/) - FluxCD resource builders
