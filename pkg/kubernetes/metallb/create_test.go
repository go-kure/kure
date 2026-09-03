package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func TestCreateL2Advertisement(t *testing.T) {
	obj := CreateL2Advertisement("my-l2", "metallb-system")
	if obj == nil {
		t.Fatal("expected non-nil L2Advertisement")
	}
	if obj.Name != "my-l2" {
		t.Errorf("expected Name 'my-l2', got %s", obj.Name)
	}
	if obj.Namespace != "metallb-system" {
		t.Errorf("expected Namespace 'metallb-system', got %s", obj.Namespace)
	}
}

// Tests for setters to verify the Style A pattern works end-to-end.

func TestIPAddressPoolSetters(t *testing.T) {
	obj := CreateIPAddressPool("my-pool", "metallb-system")
	AddIPAddressPoolAddress(obj, "192.168.1.0/24")
	AddIPAddressPoolAddress(obj, "10.0.0.0/16")
	SetIPAddressPoolAutoAssign(obj, true)

	if len(obj.Spec.Addresses) != 2 {
		t.Fatalf("expected 2 addresses, got %d", len(obj.Spec.Addresses))
	}
	if obj.Spec.Addresses[0] != "192.168.1.0/24" {
		t.Errorf("expected first address '192.168.1.0/24', got %s", obj.Spec.Addresses[0])
	}
	if obj.Spec.AutoAssign == nil || !*obj.Spec.AutoAssign {
		t.Error("expected AutoAssign true")
	}
}

func TestIPAddressPoolAllocateTo(t *testing.T) {
	obj := CreateIPAddressPool("alloc-pool", "metallb-system")
	alloc := &metallbv1beta1.ServiceAllocation{Priority: 10}
	SetIPAddressPoolAllocateTo(obj, alloc)

	if obj.Spec.AllocateTo == nil {
		t.Fatal("expected non-nil AllocateTo")
	}
	if obj.Spec.AllocateTo.Priority != 10 {
		t.Errorf("expected AllocateTo.Priority 10, got %d", obj.Spec.AllocateTo.Priority)
	}
}

func TestBGPPeerSetters(t *testing.T) {
	obj := CreateBGPPeer("my-peer", "metallb-system")
	obj.Spec.MyASN = 64500
	obj.Spec.ASN = 64501
	obj.Spec.Address = "10.0.0.1"
	AddBGPPeerNodeSelector(obj, metallbv1beta1.NodeSelector{
		MatchLabels: map[string]string{"role": "worker"},
	})

	if obj.Spec.MyASN != 64500 {
		t.Errorf("expected MyASN 64500, got %d", obj.Spec.MyASN)
	}
	if len(obj.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(obj.Spec.NodeSelectors))
	}
}

func TestBGPAdvertisementSetters(t *testing.T) {
	obj := CreateBGPAdvertisement("my-advert", "metallb-system")
	AddBGPAdvertisementIPAddressPool(obj, "pool-1")
	AddBGPAdvertisementIPAddressPool(obj, "pool-2")
	AddBGPAdvertisementPeer(obj, "peer-1")
	AddBGPAdvertisementCommunity(obj, "65535:65282")
	if len(obj.Spec.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(obj.Spec.Peers))
	}
}

func TestL2AdvertisementSetters(t *testing.T) {
	obj := CreateL2Advertisement("my-l2", "metallb-system")
	AddL2AdvertisementIPAddressPool(obj, "pool-1")
	AddL2AdvertisementInterface(obj, "eth0")
	AddL2AdvertisementInterface(obj, "eth1")

	if len(obj.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(obj.Spec.IPAddressPools))
	}
	if len(obj.Spec.Interfaces) != 2 {
		t.Fatalf("expected 2 interfaces, got %d", len(obj.Spec.Interfaces))
	}
	if obj.Spec.Interfaces[0] != "eth0" {
		t.Errorf("expected first interface 'eth0', got %s", obj.Spec.Interfaces[0])
	}
}

func TestBFDProfileSetters(t *testing.T) {
	obj := CreateBFDProfile("full-bfd", "metallb-system")
	SetBFDProfileDetectMultiplier(obj, 3)
	SetBFDProfileEchoInterval(obj, 50)
	SetBFDProfileEchoMode(obj, true)
	SetBFDProfilePassiveMode(obj, false)

	if obj.Spec.DetectMultiplier == nil || *obj.Spec.DetectMultiplier != 3 {
		t.Error("expected DetectMultiplier 3")
	}
	if obj.Spec.EchoInterval == nil || *obj.Spec.EchoInterval != 50 {
		t.Error("expected EchoInterval 50")
	}
	if obj.Spec.EchoMode == nil || !*obj.Spec.EchoMode {
		t.Error("expected EchoMode true")
	}
	if obj.Spec.PassiveMode == nil || *obj.Spec.PassiveMode {
		t.Error("expected PassiveMode false")
	}
}

func TestBFDProfileMinimal(t *testing.T) {
	obj := CreateBFDProfile("minimal-bfd", "metallb-system")
	if obj.Spec.DetectMultiplier != nil {
		t.Error("expected nil DetectMultiplier")
	}
	if obj.Spec.EchoMode != nil {
		t.Error("expected nil EchoMode")
	}
}
