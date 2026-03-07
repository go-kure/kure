package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestIPAddressPool_Success(t *testing.T) {
	autoAssign := true
	avoidBuggy := false
	cfg := &IPAddressPoolConfig{
		Name:          "my-pool",
		Namespace:     "metallb-system",
		Addresses:     []string{"192.168.1.0/24", "10.0.0.0/16"},
		AutoAssign:    &autoAssign,
		AvoidBuggyIPs: &avoidBuggy,
	}

	pool := IPAddressPool(cfg)

	if pool == nil {
		t.Fatal("expected non-nil IPAddressPool")
	}

	if pool.Name != "my-pool" {
		t.Errorf("expected Name 'my-pool', got %s", pool.Name)
	}

	if pool.Namespace != "metallb-system" {
		t.Errorf("expected Namespace 'metallb-system', got %s", pool.Namespace)
	}

	if len(pool.Spec.Addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(pool.Spec.Addresses))
	}

	if pool.Spec.Addresses[0] != "192.168.1.0/24" {
		t.Errorf("expected first address '192.168.1.0/24', got %s", pool.Spec.Addresses[0])
	}

	if pool.Spec.AutoAssign == nil || !*pool.Spec.AutoAssign {
		t.Error("expected AutoAssign to be true")
	}

	if pool.Spec.AvoidBuggyIPs {
		t.Error("expected AvoidBuggyIPs to be false")
	}
}

func TestIPAddressPool_NilConfig(t *testing.T) {
	pool := IPAddressPool(nil)
	if pool != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestIPAddressPool_WithAllocateTo(t *testing.T) {
	alloc := &metallbv1beta1.ServiceAllocation{
		Priority: 10,
	}
	cfg := &IPAddressPoolConfig{
		Name:       "alloc-pool",
		Namespace:  "metallb-system",
		Addresses:  []string{"10.0.0.0/8"},
		AllocateTo: alloc,
	}

	pool := IPAddressPool(cfg)

	if pool == nil {
		t.Fatal("expected non-nil IPAddressPool")
	}

	if pool.Spec.AllocateTo == nil {
		t.Fatal("expected non-nil AllocateTo")
	}

	if pool.Spec.AllocateTo.Priority != 10 {
		t.Errorf("expected AllocateTo.Priority 10, got %d", pool.Spec.AllocateTo.Priority)
	}
}

func TestBGPPeer_Success(t *testing.T) {
	holdTime := metav1.Duration{Duration: 90000000000} // 90s
	ebgpMultiHop := true
	cfg := &BGPPeerConfig{
		Name:         "my-peer",
		Namespace:    "metallb-system",
		MyASN:        64500,
		ASN:          64501,
		Address:      "10.0.0.1",
		Port:         179,
		HoldTime:     &holdTime,
		SrcAddress:   "10.0.0.2",
		RouterID:     "10.0.0.2",
		EBGPMultiHop: &ebgpMultiHop,
		Password:     "secret",
		BFDProfile:   "full-bfd",
		NodeSelectors: []metallbv1beta1.NodeSelector{
			{MatchLabels: map[string]string{"role": "worker"}},
		},
	}

	peer := BGPPeer(cfg)

	if peer == nil {
		t.Fatal("expected non-nil BGPPeer")
	}

	if peer.Name != "my-peer" {
		t.Errorf("expected Name 'my-peer', got %s", peer.Name)
	}

	if peer.Spec.MyASN != 64500 {
		t.Errorf("expected MyASN 64500, got %d", peer.Spec.MyASN)
	}

	if peer.Spec.ASN != 64501 {
		t.Errorf("expected ASN 64501, got %d", peer.Spec.ASN)
	}

	if peer.Spec.Address != "10.0.0.1" {
		t.Errorf("expected Address '10.0.0.1', got %s", peer.Spec.Address)
	}

	if peer.Spec.Port != 179 {
		t.Errorf("expected Port 179, got %d", peer.Spec.Port)
	}

	if peer.Spec.SrcAddress != "10.0.0.2" {
		t.Errorf("expected SrcAddress '10.0.0.2', got %s", peer.Spec.SrcAddress)
	}

	if peer.Spec.RouterID != "10.0.0.2" {
		t.Errorf("expected RouterID '10.0.0.2', got %s", peer.Spec.RouterID)
	}

	if !peer.Spec.EBGPMultiHop {
		t.Error("expected EBGPMultiHop to be true")
	}

	if peer.Spec.Password != "secret" {
		t.Errorf("expected Password 'secret', got %s", peer.Spec.Password)
	}

	if peer.Spec.BFDProfile != "full-bfd" {
		t.Errorf("expected BFDProfile 'full-bfd', got %s", peer.Spec.BFDProfile)
	}

	if len(peer.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(peer.Spec.NodeSelectors))
	}
}

func TestBGPPeer_NilConfig(t *testing.T) {
	peer := BGPPeer(nil)
	if peer != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestBGPPeer_MinimalConfig(t *testing.T) {
	cfg := &BGPPeerConfig{
		Name:      "minimal-peer",
		Namespace: "metallb-system",
		MyASN:     64500,
		ASN:       64501,
		Address:   "10.0.0.1",
	}

	peer := BGPPeer(cfg)

	if peer == nil {
		t.Fatal("expected non-nil BGPPeer")
	}

	if peer.Spec.Port != 0 {
		t.Errorf("expected Port 0 (unset), got %d", peer.Spec.Port)
	}

	if peer.Spec.SrcAddress != "" {
		t.Errorf("expected empty SrcAddress, got %s", peer.Spec.SrcAddress)
	}

	if peer.Spec.Password != "" {
		t.Errorf("expected empty Password, got %s", peer.Spec.Password)
	}
}

func TestBGPAdvertisement_Success(t *testing.T) {
	cfg := &BGPAdvertisementConfig{
		Name:           "my-advert",
		Namespace:      "metallb-system",
		IPAddressPools: []string{"pool-1", "pool-2"},
		Peers:          []string{"peer-1"},
		Communities:    []string{"65535:65282"},
		LocalPref:      100,
		NodeSelectors: []metav1.LabelSelector{
			{MatchLabels: map[string]string{"node": "worker"}},
		},
	}

	advert := BGPAdvertisement(cfg)

	if advert == nil {
		t.Fatal("expected non-nil BGPAdvertisement")
	}

	if advert.Name != "my-advert" {
		t.Errorf("expected Name 'my-advert', got %s", advert.Name)
	}

	if len(advert.Spec.IPAddressPools) != 2 {
		t.Fatalf("expected 2 IP address pools, got %d", len(advert.Spec.IPAddressPools))
	}

	if len(advert.Spec.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(advert.Spec.Peers))
	}

	if advert.Spec.Peers[0] != "peer-1" {
		t.Errorf("expected peer 'peer-1', got %s", advert.Spec.Peers[0])
	}

	if len(advert.Spec.Communities) != 1 {
		t.Fatalf("expected 1 community, got %d", len(advert.Spec.Communities))
	}

	if advert.Spec.LocalPref != 100 {
		t.Errorf("expected LocalPref 100, got %d", advert.Spec.LocalPref)
	}

	if len(advert.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(advert.Spec.NodeSelectors))
	}
}

func TestBGPAdvertisement_NilConfig(t *testing.T) {
	advert := BGPAdvertisement(nil)
	if advert != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestL2Advertisement_Success(t *testing.T) {
	cfg := &L2AdvertisementConfig{
		Name:           "my-l2",
		Namespace:      "metallb-system",
		IPAddressPools: []string{"pool-1"},
		Interfaces:     []string{"eth0", "eth1"},
		NodeSelectors: []metav1.LabelSelector{
			{MatchLabels: map[string]string{"node": "worker"}},
		},
	}

	l2 := L2Advertisement(cfg)

	if l2 == nil {
		t.Fatal("expected non-nil L2Advertisement")
	}

	if l2.Name != "my-l2" {
		t.Errorf("expected Name 'my-l2', got %s", l2.Name)
	}

	if l2.Namespace != "metallb-system" {
		t.Errorf("expected Namespace 'metallb-system', got %s", l2.Namespace)
	}

	if len(l2.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 IP address pool, got %d", len(l2.Spec.IPAddressPools))
	}

	if len(l2.Spec.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(l2.Spec.Interfaces))
	}

	if l2.Spec.Interfaces[0] != "eth0" {
		t.Errorf("expected first interface 'eth0', got %s", l2.Spec.Interfaces[0])
	}

	if len(l2.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(l2.Spec.NodeSelectors))
	}
}

func TestL2Advertisement_NilConfig(t *testing.T) {
	l2 := L2Advertisement(nil)
	if l2 != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestBFDProfile_Success(t *testing.T) {
	detectMult := uint32(3)
	echoInterval := uint32(50)
	echoMode := true
	passiveMode := false
	cfg := &BFDProfileConfig{
		Name:             "full-bfd",
		Namespace:        "metallb-system",
		DetectMultiplier: &detectMult,
		EchoInterval:     &echoInterval,
		EchoMode:         &echoMode,
		PassiveMode:      &passiveMode,
	}

	bfd := BFDProfile(cfg)

	if bfd == nil {
		t.Fatal("expected non-nil BFDProfile")
	}

	if bfd.Name != "full-bfd" {
		t.Errorf("expected Name 'full-bfd', got %s", bfd.Name)
	}

	if bfd.Namespace != "metallb-system" {
		t.Errorf("expected Namespace 'metallb-system', got %s", bfd.Namespace)
	}

	if bfd.Spec.DetectMultiplier == nil || *bfd.Spec.DetectMultiplier != 3 {
		t.Error("expected DetectMultiplier 3")
	}

	if bfd.Spec.EchoInterval == nil || *bfd.Spec.EchoInterval != 50 {
		t.Error("expected EchoInterval 50")
	}

	if bfd.Spec.EchoMode == nil || !*bfd.Spec.EchoMode {
		t.Error("expected EchoMode true")
	}

	if bfd.Spec.PassiveMode == nil || *bfd.Spec.PassiveMode {
		t.Error("expected PassiveMode false")
	}
}

func TestBFDProfile_NilConfig(t *testing.T) {
	bfd := BFDProfile(nil)
	if bfd != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestBFDProfile_MinimalConfig(t *testing.T) {
	cfg := &BFDProfileConfig{
		Name:      "minimal-bfd",
		Namespace: "metallb-system",
	}

	bfd := BFDProfile(cfg)

	if bfd == nil {
		t.Fatal("expected non-nil BFDProfile")
	}

	if bfd.Spec.DetectMultiplier != nil {
		t.Error("expected nil DetectMultiplier")
	}

	if bfd.Spec.EchoMode != nil {
		t.Error("expected nil EchoMode")
	}
}

func TestAllConstructorsWithNilConfig(t *testing.T) {
	constructors := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"IPAddressPool", func(t *testing.T) {
			if IPAddressPool(nil) != nil {
				t.Error("IPAddressPool should return nil for nil config")
			}
		}},
		{"BGPPeer", func(t *testing.T) {
			if BGPPeer(nil) != nil {
				t.Error("BGPPeer should return nil for nil config")
			}
		}},
		{"BGPAdvertisement", func(t *testing.T) {
			if BGPAdvertisement(nil) != nil {
				t.Error("BGPAdvertisement should return nil for nil config")
			}
		}},
		{"L2Advertisement", func(t *testing.T) {
			if L2Advertisement(nil) != nil {
				t.Error("L2Advertisement should return nil for nil config")
			}
		}},
		{"BFDProfile", func(t *testing.T) {
			if BFDProfile(nil) != nil {
				t.Error("BFDProfile should return nil for nil config")
			}
		}},
	}

	for _, constructor := range constructors {
		t.Run(constructor.name, constructor.fn)
	}
}
