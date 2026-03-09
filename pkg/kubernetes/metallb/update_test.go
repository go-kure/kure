package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestSetIPAddressPoolSpec(t *testing.T) {
	cfg := &IPAddressPoolConfig{
		Name:      "test-pool",
		Namespace: "metallb-system",
		Addresses: []string{"10.0.0.0/24"},
	}

	pool := IPAddressPool(cfg)
	if pool == nil {
		t.Fatal("failed to create IPAddressPool")
	}

	newSpec := metallbv1beta1.IPAddressPoolSpec{
		Addresses: []string{"192.168.0.0/16", "172.16.0.0/12"},
	}

	SetIPAddressPoolSpec(pool, newSpec)

	if len(pool.Spec.Addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(pool.Spec.Addresses))
	}

	if pool.Spec.Addresses[0] != "192.168.0.0/16" {
		t.Errorf("expected first address '192.168.0.0/16', got %s", pool.Spec.Addresses[0])
	}
}

func TestSetBGPPeerSpec(t *testing.T) {
	cfg := &BGPPeerConfig{
		Name:      "test-peer",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	}

	peer := BGPPeer(cfg)
	if peer == nil {
		t.Fatal("failed to create BGPPeer")
	}

	newSpec := metallbv1beta1.BGPPeerSpec{
		MyASN:   64600,
		ASN:     64601,
		Address: "10.0.0.2",
		Port:    180,
	}

	SetBGPPeerSpec(peer, newSpec)

	if peer.Spec.MyASN != 64600 {
		t.Errorf("expected MyASN 64600, got %d", peer.Spec.MyASN)
	}

	if peer.Spec.Address != "10.0.0.2" {
		t.Errorf("expected Address '10.0.0.2', got %s", peer.Spec.Address)
	}

	if peer.Spec.Port != 180 {
		t.Errorf("expected Port 180, got %d", peer.Spec.Port)
	}
}

func TestSetBGPAdvertisementSpec(t *testing.T) {
	cfg := &BGPAdvertisementConfig{
		Name:      "test-advert",
		Namespace: "metallb-system",
	}

	advert := BGPAdvertisement(cfg)
	if advert == nil {
		t.Fatal("failed to create BGPAdvertisement")
	}

	newSpec := metallbv1beta1.BGPAdvertisementSpec{
		IPAddressPools: []string{"new-pool"},
		LocalPref:      200,
	}

	SetBGPAdvertisementSpec(advert, newSpec)

	if len(advert.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 IP address pool, got %d", len(advert.Spec.IPAddressPools))
	}

	if advert.Spec.IPAddressPools[0] != "new-pool" {
		t.Errorf("expected pool 'new-pool', got %s", advert.Spec.IPAddressPools[0])
	}

	if advert.Spec.LocalPref != 200 {
		t.Errorf("expected LocalPref 200, got %d", advert.Spec.LocalPref)
	}
}

func TestSetL2AdvertisementSpec(t *testing.T) {
	cfg := &L2AdvertisementConfig{
		Name:      "test-l2",
		Namespace: "metallb-system",
	}

	l2 := L2Advertisement(cfg)
	if l2 == nil {
		t.Fatal("failed to create L2Advertisement")
	}

	newSpec := metallbv1beta1.L2AdvertisementSpec{
		IPAddressPools: []string{"pool-a"},
		Interfaces:     []string{"eth2"},
	}

	SetL2AdvertisementSpec(l2, newSpec)

	if len(l2.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 IP address pool, got %d", len(l2.Spec.IPAddressPools))
	}

	if l2.Spec.Interfaces[0] != "eth2" {
		t.Errorf("expected interface 'eth2', got %s", l2.Spec.Interfaces[0])
	}
}

func TestSetBFDProfileSpec(t *testing.T) {
	cfg := &BFDProfileConfig{
		Name:      "test-bfd",
		Namespace: "metallb-system",
	}

	bfd := BFDProfile(cfg)
	if bfd == nil {
		t.Fatal("failed to create BFDProfile")
	}

	detectMult := uint32(5)
	newSpec := metallbv1beta1.BFDProfileSpec{
		DetectMultiplier: &detectMult,
	}

	SetBFDProfileSpec(bfd, newSpec)

	if bfd.Spec.DetectMultiplier == nil || *bfd.Spec.DetectMultiplier != 5 {
		t.Error("expected DetectMultiplier 5")
	}
}

func TestAddIPAddressPoolAddress(t *testing.T) {
	pool := IPAddressPool(&IPAddressPoolConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	AddIPAddressPoolAddress(pool, "10.0.0.0/24")

	if len(pool.Spec.Addresses) != 1 {
		t.Fatalf("expected 1 address, got %d", len(pool.Spec.Addresses))
	}

	if pool.Spec.Addresses[0] != "10.0.0.0/24" {
		t.Errorf("expected address '10.0.0.0/24', got %s", pool.Spec.Addresses[0])
	}
}

func TestSetIPAddressPoolAutoAssign(t *testing.T) {
	pool := IPAddressPool(&IPAddressPoolConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetIPAddressPoolAutoAssign(pool, false)

	if pool.Spec.AutoAssign == nil || *pool.Spec.AutoAssign {
		t.Error("expected AutoAssign to be false")
	}
}

func TestSetIPAddressPoolAvoidBuggyIPs(t *testing.T) {
	pool := IPAddressPool(&IPAddressPoolConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetIPAddressPoolAvoidBuggyIPs(pool, true)

	if !pool.Spec.AvoidBuggyIPs {
		t.Error("expected AvoidBuggyIPs to be true")
	}
}

func TestSetIPAddressPoolAllocateTo(t *testing.T) {
	pool := IPAddressPool(&IPAddressPoolConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	alloc := &metallbv1beta1.ServiceAllocation{Priority: 5}
	SetIPAddressPoolAllocateTo(pool, alloc)

	if pool.Spec.AllocateTo == nil || pool.Spec.AllocateTo.Priority != 5 {
		t.Error("expected AllocateTo.Priority 5")
	}
}

func TestAddBGPPeerNodeSelector(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	sel := metallbv1beta1.NodeSelector{
		MatchLabels: map[string]string{"role": "worker"},
	}
	AddBGPPeerNodeSelector(peer, sel)

	if len(peer.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(peer.Spec.NodeSelectors))
	}
}

func TestSetBGPPeerPort(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	SetBGPPeerPort(peer, 1179)

	if peer.Spec.Port != 1179 {
		t.Errorf("expected Port 1179, got %d", peer.Spec.Port)
	}
}

func TestSetBGPPeerHoldTime(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	d := metav1.Duration{Duration: 120000000000} // 120s
	SetBGPPeerHoldTime(peer, d)
}

func TestSetBGPPeerKeepaliveTime(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	d := metav1.Duration{Duration: 30000000000} // 30s
	SetBGPPeerKeepaliveTime(peer, d)
}

func TestSetBGPPeerSrcAddress(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	SetBGPPeerSrcAddress(peer, "10.0.0.100")

	if peer.Spec.SrcAddress != "10.0.0.100" {
		t.Errorf("expected SrcAddress '10.0.0.100', got %s", peer.Spec.SrcAddress)
	}
}

func TestSetBGPPeerRouterID(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	SetBGPPeerRouterID(peer, "1.2.3.4")

	if peer.Spec.RouterID != "1.2.3.4" {
		t.Errorf("expected RouterID '1.2.3.4', got %s", peer.Spec.RouterID)
	}
}

func TestSetBGPPeerEBGPMultiHop(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	SetBGPPeerEBGPMultiHop(peer, true)

	if !peer.Spec.EBGPMultiHop {
		t.Error("expected EBGPMultiHop to be true")
	}
}

func TestSetBGPPeerPassword(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	SetBGPPeerPassword(peer, "bgp-secret")

	if peer.Spec.Password != "bgp-secret" {
		t.Errorf("expected Password 'bgp-secret', got %s", peer.Spec.Password)
	}
}

func TestSetBGPPeerBFDProfile(t *testing.T) {
	peer := BGPPeer(&BGPPeerConfig{
		Name:      "test",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	})

	SetBGPPeerBFDProfile(peer, "my-bfd")

	if peer.Spec.BFDProfile != "my-bfd" {
		t.Errorf("expected BFDProfile 'my-bfd', got %s", peer.Spec.BFDProfile)
	}
}

func TestAddBGPAdvertisementIPAddressPool(t *testing.T) {
	advert := BGPAdvertisement(&BGPAdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	AddBGPAdvertisementIPAddressPool(advert, "pool-1")

	if len(advert.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(advert.Spec.IPAddressPools))
	}

	if advert.Spec.IPAddressPools[0] != "pool-1" {
		t.Errorf("expected pool 'pool-1', got %s", advert.Spec.IPAddressPools[0])
	}
}

func TestAddBGPAdvertisementNodeSelector(t *testing.T) {
	advert := BGPAdvertisement(&BGPAdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	sel := metav1.LabelSelector{
		MatchLabels: map[string]string{"zone": "us-east"},
	}
	AddBGPAdvertisementNodeSelector(advert, sel)

	if len(advert.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(advert.Spec.NodeSelectors))
	}
}

func TestAddBGPAdvertisementCommunity(t *testing.T) {
	advert := BGPAdvertisement(&BGPAdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	AddBGPAdvertisementCommunity(advert, "65535:65282")

	if len(advert.Spec.Communities) != 1 {
		t.Fatalf("expected 1 community, got %d", len(advert.Spec.Communities))
	}
}

func TestAddBGPAdvertisementPeer(t *testing.T) {
	advert := BGPAdvertisement(&BGPAdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	AddBGPAdvertisementPeer(advert, "peer-1")

	if len(advert.Spec.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(advert.Spec.Peers))
	}
}

func TestSetBGPAdvertisementLocalPref(t *testing.T) {
	advert := BGPAdvertisement(&BGPAdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetBGPAdvertisementLocalPref(advert, 150)

	if advert.Spec.LocalPref != 150 {
		t.Errorf("expected LocalPref 150, got %d", advert.Spec.LocalPref)
	}
}

func TestAddL2AdvertisementIPAddressPool(t *testing.T) {
	l2 := L2Advertisement(&L2AdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	AddL2AdvertisementIPAddressPool(l2, "pool-1")

	if len(l2.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(l2.Spec.IPAddressPools))
	}
}

func TestAddL2AdvertisementNodeSelector(t *testing.T) {
	l2 := L2Advertisement(&L2AdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	sel := metav1.LabelSelector{
		MatchLabels: map[string]string{"zone": "us-west"},
	}
	AddL2AdvertisementNodeSelector(l2, sel)

	if len(l2.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(l2.Spec.NodeSelectors))
	}
}

func TestAddL2AdvertisementInterface(t *testing.T) {
	l2 := L2Advertisement(&L2AdvertisementConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	AddL2AdvertisementInterface(l2, "eth0")

	if len(l2.Spec.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(l2.Spec.Interfaces))
	}

	if l2.Spec.Interfaces[0] != "eth0" {
		t.Errorf("expected interface 'eth0', got %s", l2.Spec.Interfaces[0])
	}
}

func TestSetBFDProfileDetectMultiplier(t *testing.T) {
	bfd := BFDProfile(&BFDProfileConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetBFDProfileDetectMultiplier(bfd, 5)

	if bfd.Spec.DetectMultiplier == nil || *bfd.Spec.DetectMultiplier != 5 {
		t.Error("expected DetectMultiplier 5")
	}
}

func TestSetBFDProfileEchoInterval(t *testing.T) {
	bfd := BFDProfile(&BFDProfileConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetBFDProfileEchoInterval(bfd, 100)

	if bfd.Spec.EchoInterval == nil || *bfd.Spec.EchoInterval != 100 {
		t.Error("expected EchoInterval 100")
	}
}

func TestSetBFDProfileEchoMode(t *testing.T) {
	bfd := BFDProfile(&BFDProfileConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetBFDProfileEchoMode(bfd, true)

	if bfd.Spec.EchoMode == nil || !*bfd.Spec.EchoMode {
		t.Error("expected EchoMode true")
	}
}

func TestSetBFDProfilePassiveMode(t *testing.T) {
	bfd := BFDProfile(&BFDProfileConfig{
		Name:      "test",
		Namespace: "metallb-system",
	})

	SetBFDProfilePassiveMode(bfd, true)

	if bfd.Spec.PassiveMode == nil || !*bfd.Spec.PassiveMode {
		t.Error("expected PassiveMode true")
	}
}
