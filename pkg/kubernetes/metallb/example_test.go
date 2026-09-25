package metallb_test

import (
	"fmt"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"

	"github.com/go-kure/kure/pkg/kubernetes/metallb"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateIPAddressPool() {
	obj := metallb.CreateIPAddressPool("my-pool", "metallb-system")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
	// Output: IPAddressPool metallb-system/my-pool
}

func ExampleAddIPAddressPoolAddress() {
	pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
	metallb.AddIPAddressPoolAddress(pool, "192.168.1.0/24")
	metallb.AddIPAddressPoolAddress(pool, "10.0.0.0/16")
	fmt.Println(pool.Spec.Addresses)
	// Output: [192.168.1.0/24 10.0.0.0/16]
}

func ExampleCreateBGPPeer() {
	peer := metallb.CreateBGPPeer("my-peer", "metallb-system")
	peer.Spec.MyASN = 64500
	peer.Spec.ASN = 64501
	peer.Spec.Address = "10.0.0.1"
	peer.Spec.Port = 179
	fmt.Println(peer.Spec.Address, peer.Spec.ASN)
	// Output: 10.0.0.1 64501
}

func ExampleCreateBGPAdvertisement() {
	advert := metallb.CreateBGPAdvertisement("my-advert", "metallb-system")
	metallb.AddBGPAdvertisementIPAddressPool(advert, "my-pool")
	metallb.AddBGPAdvertisementPeer(advert, "my-peer")
	metallb.AddBGPAdvertisementCommunity(advert, "65535:65282")
	advert.Spec.LocalPref = 100
	fmt.Println(advert.Spec.IPAddressPools, advert.Spec.Peers, advert.Spec.Communities)
	// Output: [my-pool] [my-peer] [65535:65282]
}

func ExampleCreateL2Advertisement() {
	l2 := metallb.CreateL2Advertisement("my-l2", "metallb-system")
	metallb.AddL2AdvertisementIPAddressPool(l2, "my-pool")
	metallb.AddL2AdvertisementInterface(l2, "eth0")
	fmt.Println(l2.Spec.IPAddressPools, l2.Spec.Interfaces)
	// Output: [my-pool] [eth0]
}

func ExampleCreateBFDProfile() {
	bfd := metallb.CreateBFDProfile("my-bfd", "metallb-system")
	metallb.SetBFDProfileDetectMultiplier(bfd, 3)
	fmt.Println(*bfd.Spec.DetectMultiplier)
	// Output: 3
}

func ExampleSetIPAddressPoolAutoAssign() {
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
	// Output: [192.168.1.0/24 172.16.0.0/12] false 1179
}
