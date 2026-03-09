package metallb_test

import (
	"fmt"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	"github.com/go-kure/kure/internal/metallb"
)

// This example demonstrates composing a full MetalLB BGP setup:
// an IPAddressPool with a public range, a BGPPeer for the upstream
// router, and a BGPAdvertisement tying them together.
func Example_composeBGPSetup() {
	// --- IPAddressPool ---
	pool := metallb.CreateIPAddressPool("public-pool", "metallb-system", metallbv1beta1.IPAddressPoolSpec{})
	metallb.AddIPAddressPoolAddress(pool, "203.0.113.0/24")
	metallb.SetIPAddressPoolAutoAssign(pool, true)

	// --- BGPPeer ---
	peer := metallb.CreateBGPPeer("upstream-router", "metallb-system", metallbv1beta1.BGPPeerSpec{
		MyASN:   64512,
		ASN:     64513,
		Address: "10.0.0.1",
	})
	metallb.SetBGPPeerPort(peer, 179)
	metallb.SetBGPPeerEBGPMultiHop(peer, true)
	metallb.SetBGPPeerBFDProfile(peer, "default-bfd")

	// --- BGPAdvertisement linking the pool and the peer ---
	advert := metallb.CreateBGPAdvertisement("public-advert", "metallb-system", metallbv1beta1.BGPAdvertisementSpec{})
	metallb.AddBGPAdvertisementIPAddressPool(advert, pool.Name)
	metallb.AddBGPAdvertisementPeer(advert, peer.Name)
	metallb.SetBGPAdvertisementLocalPref(advert, 100)
	metallb.AddBGPAdvertisementCommunity(advert, "65535:65281")

	fmt.Println("Pool:", pool.Name)
	fmt.Println("Pool Namespace:", pool.Namespace)
	fmt.Println("Pool Addresses:", pool.Spec.Addresses)
	fmt.Println("Peer:", peer.Name)
	fmt.Println("Peer ASN:", peer.Spec.ASN)
	fmt.Println("Peer Address:", peer.Spec.Address)
	fmt.Println("Peer Port:", peer.Spec.Port)
	fmt.Println("Advertisement:", advert.Name)
	fmt.Println("Advertisement Pools:", advert.Spec.IPAddressPools)
	fmt.Println("Advertisement Peers:", advert.Spec.Peers)
	fmt.Println("Advertisement LocalPref:", advert.Spec.LocalPref)
	// Output:
	// Pool: public-pool
	// Pool Namespace: metallb-system
	// Pool Addresses: [203.0.113.0/24]
	// Peer: upstream-router
	// Peer ASN: 64513
	// Peer Address: 10.0.0.1
	// Peer Port: 179
	// Advertisement: public-advert
	// Advertisement Pools: [public-pool]
	// Advertisement Peers: [upstream-router]
	// Advertisement LocalPref: 100
}
