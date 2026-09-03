package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAddIPAddressPoolAddress(t *testing.T) {
	pool := CreateIPAddressPool("test", "metallb-system")

	AddIPAddressPoolAddress(pool, "10.0.0.0/24")

	if len(pool.Spec.Addresses) != 1 {
		t.Fatalf("expected 1 address, got %d", len(pool.Spec.Addresses))
	}

	if pool.Spec.Addresses[0] != "10.0.0.0/24" {
		t.Errorf("expected address '10.0.0.0/24', got %s", pool.Spec.Addresses[0])
	}
}

func TestSetIPAddressPoolAutoAssign(t *testing.T) {
	pool := CreateIPAddressPool("test", "metallb-system")

	SetIPAddressPoolAutoAssign(pool, false)

	if pool.Spec.AutoAssign == nil || *pool.Spec.AutoAssign {
		t.Error("expected AutoAssign to be false")
	}
}

func TestSetIPAddressPoolAllocateTo(t *testing.T) {
	pool := CreateIPAddressPool("test", "metallb-system")

	alloc := &metallbv1beta1.ServiceAllocation{Priority: 5}
	SetIPAddressPoolAllocateTo(pool, alloc)

	if pool.Spec.AllocateTo == nil || pool.Spec.AllocateTo.Priority != 5 {
		t.Error("expected AllocateTo.Priority 5")
	}
}

func TestAddBGPPeerNodeSelector(t *testing.T) {
	peer := CreateBGPPeer("test", "metallb-system")

	sel := metallbv1beta1.NodeSelector{
		MatchLabels: map[string]string{"role": "worker"},
	}
	AddBGPPeerNodeSelector(peer, sel)

	if len(peer.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(peer.Spec.NodeSelectors))
	}
}

func TestAddBGPAdvertisementIPAddressPool(t *testing.T) {
	advert := CreateBGPAdvertisement("test", "metallb-system")

	AddBGPAdvertisementIPAddressPool(advert, "pool-1")

	if len(advert.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(advert.Spec.IPAddressPools))
	}

	if advert.Spec.IPAddressPools[0] != "pool-1" {
		t.Errorf("expected pool 'pool-1', got %s", advert.Spec.IPAddressPools[0])
	}
}

func TestAddBGPAdvertisementNodeSelector(t *testing.T) {
	advert := CreateBGPAdvertisement("test", "metallb-system")

	sel := metav1.LabelSelector{
		MatchLabels: map[string]string{"zone": "us-east"},
	}
	AddBGPAdvertisementNodeSelector(advert, sel)

	if len(advert.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(advert.Spec.NodeSelectors))
	}
}

func TestAddBGPAdvertisementCommunity(t *testing.T) {
	advert := CreateBGPAdvertisement("test", "metallb-system")

	AddBGPAdvertisementCommunity(advert, "65535:65282")

	if len(advert.Spec.Communities) != 1 {
		t.Fatalf("expected 1 community, got %d", len(advert.Spec.Communities))
	}
}

func TestAddBGPAdvertisementPeer(t *testing.T) {
	advert := CreateBGPAdvertisement("test", "metallb-system")

	AddBGPAdvertisementPeer(advert, "peer-1")

	if len(advert.Spec.Peers) != 1 {
		t.Fatalf("expected 1 peer, got %d", len(advert.Spec.Peers))
	}
}

func TestAddL2AdvertisementIPAddressPool(t *testing.T) {
	l2 := CreateL2Advertisement("test", "metallb-system")

	AddL2AdvertisementIPAddressPool(l2, "pool-1")

	if len(l2.Spec.IPAddressPools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(l2.Spec.IPAddressPools))
	}
}

func TestAddL2AdvertisementNodeSelector(t *testing.T) {
	l2 := CreateL2Advertisement("test", "metallb-system")

	sel := metav1.LabelSelector{
		MatchLabels: map[string]string{"zone": "us-west"},
	}
	AddL2AdvertisementNodeSelector(l2, sel)

	if len(l2.Spec.NodeSelectors) != 1 {
		t.Fatalf("expected 1 node selector, got %d", len(l2.Spec.NodeSelectors))
	}
}

func TestAddL2AdvertisementInterface(t *testing.T) {
	l2 := CreateL2Advertisement("test", "metallb-system")

	AddL2AdvertisementInterface(l2, "eth0")

	if len(l2.Spec.Interfaces) != 1 {
		t.Fatalf("expected 1 interface, got %d", len(l2.Spec.Interfaces))
	}

	if l2.Spec.Interfaces[0] != "eth0" {
		t.Errorf("expected interface 'eth0', got %s", l2.Spec.Interfaces[0])
	}
}

func TestSetBFDProfileDetectMultiplier(t *testing.T) {
	bfd := CreateBFDProfile("test", "metallb-system")

	SetBFDProfileDetectMultiplier(bfd, 5)

	if bfd.Spec.DetectMultiplier == nil || *bfd.Spec.DetectMultiplier != 5 {
		t.Error("expected DetectMultiplier 5")
	}
}

func TestSetBFDProfileEchoInterval(t *testing.T) {
	bfd := CreateBFDProfile("test", "metallb-system")

	SetBFDProfileEchoInterval(bfd, 100)

	if bfd.Spec.EchoInterval == nil || *bfd.Spec.EchoInterval != 100 {
		t.Error("expected EchoInterval 100")
	}
}

func TestSetBFDProfileEchoMode(t *testing.T) {
	bfd := CreateBFDProfile("test", "metallb-system")

	SetBFDProfileEchoMode(bfd, true)

	if bfd.Spec.EchoMode == nil || !*bfd.Spec.EchoMode {
		t.Error("expected EchoMode true")
	}
}

func TestSetBFDProfilePassiveMode(t *testing.T) {
	bfd := CreateBFDProfile("test", "metallb-system")

	SetBFDProfilePassiveMode(bfd, true)

	if bfd.Spec.PassiveMode == nil || !*bfd.Spec.PassiveMode {
		t.Error("expected PassiveMode true")
	}
}
