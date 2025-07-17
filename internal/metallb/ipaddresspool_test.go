package metallb

import (
	"testing"

	metallbv1beta1 "go.universe.tf/metallb/api/v1beta1"
)

func TestCreateIPAddressPool(t *testing.T) {
	spec := metallbv1beta1.IPAddressPoolSpec{
		Addresses: []string{"10.0.0.0/24"},
	}
	pool := CreateIPAddressPool("pool", "default", spec)
	if pool.Name != "pool" || pool.Namespace != "default" {
		t.Fatalf("unexpected metadata: %s %s", pool.Name, pool.Namespace)
	}
	if len(pool.Spec.Addresses) != 1 || pool.Spec.Addresses[0] != "10.0.0.0/24" {
		t.Fatalf("addresses not set")
	}
}

func TestIPAddressPoolHelpers(t *testing.T) {
	pool := CreateIPAddressPool("pool", "ns", metallbv1beta1.IPAddressPoolSpec{})
	if err := AddIPAddressPoolAddress(pool, "10.0.0.1"); err != nil {
		t.Fatalf("AddIPAddressPoolAddress returned error: %v", err)
	}
	if err := SetIPAddressPoolAutoAssign(pool, false); err != nil {
		t.Fatalf("SetIPAddressPoolAutoAssign returned error: %v", err)
	}
	if err := SetIPAddressPoolAvoidBuggyIPs(pool, true); err != nil {
		t.Fatalf("SetIPAddressPoolAvoidBuggyIPs returned error: %v", err)
	}
	if err := SetIPAddressPoolAllocateTo(pool, &metallbv1beta1.ServiceAllocation{Priority: 1}); err != nil {
		t.Fatalf("SetIPAddressPoolAllocateTo returned error: %v", err)
	}

	if len(pool.Spec.Addresses) != 1 || pool.Spec.Addresses[0] != "10.0.0.1" {
		t.Errorf("address not added")
	}
	if pool.Spec.AutoAssign == nil || *pool.Spec.AutoAssign != false {
		t.Errorf("autoAssign not set")
	}
	if !pool.Spec.AvoidBuggyIPs {
		t.Errorf("avoidBuggyIPs not set")
	}
	if pool.Spec.AllocateTo == nil || pool.Spec.AllocateTo.Priority != 1 {
		t.Errorf("allocateTo not set")
	}

	if err := AddIPAddressPoolAddress(nil, "x"); err == nil {
		t.Errorf("expected error when pool nil")
	}
	if err := SetIPAddressPoolAutoAssign(nil, true); err == nil {
		t.Errorf("expected error when pool nil")
	}
	if err := SetIPAddressPoolAvoidBuggyIPs(nil, true); err == nil {
		t.Errorf("expected error when pool nil")
	}
	if err := SetIPAddressPoolAllocateTo(nil, nil); err == nil {
		t.Errorf("expected error when pool nil")
	}
}
