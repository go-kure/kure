package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
)

func TestCreateCiliumLoadBalancerIPPool(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("my-pool")
	if obj == nil {
		t.Fatal("expected non-nil CiliumLoadBalancerIPPool")
	}
	if obj.Name != "my-pool" {
		t.Errorf("expected Name 'my-pool', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumLoadBalancerIPPool" {
		t.Errorf("expected Kind 'CiliumLoadBalancerIPPool', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumLoadBalancerIPPoolSpec(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("p")
	spec := ciliumv2.CiliumLoadBalancerIPPoolSpec{
		Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{
			{Cidr: "10.0.0.0/8"},
		},
	}
	SetCiliumLoadBalancerIPPoolSpec(obj, spec)
	if len(obj.Spec.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(obj.Spec.Blocks))
	}
}

func TestSetCiliumLoadBalancerIPPoolServiceSelector(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("p")
	sel := &slimv1.LabelSelector{
		MatchLabels: map[string]string{"service": "lb"},
	}
	SetCiliumLoadBalancerIPPoolServiceSelector(obj, sel)
	if obj.Spec.ServiceSelector == nil {
		t.Fatal("expected non-nil ServiceSelector")
	}
}

func TestAddCiliumLoadBalancerIPPoolBlock(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("p")
	AddCiliumLoadBalancerIPPoolBlock(obj, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "192.168.0.0/16"})
	AddCiliumLoadBalancerIPPoolBlock(obj, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "10.0.0.0/8"})
	if len(obj.Spec.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(obj.Spec.Blocks))
	}
}

func TestSetCiliumLoadBalancerIPPoolDisabled(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("p")
	SetCiliumLoadBalancerIPPoolDisabled(obj, true)
	if !obj.Spec.Disabled {
		t.Error("expected Disabled to be true")
	}
}

func TestSetCiliumLoadBalancerIPPoolAllowFirstLastIPs(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("p")
	SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj, ciliumv2.AllowFirstLastIPYes)
	if obj.Spec.AllowFirstLastIPs != ciliumv2.AllowFirstLastIPYes {
		t.Errorf("unexpected AllowFirstLastIPs: %v", obj.Spec.AllowFirstLastIPs)
	}
}
