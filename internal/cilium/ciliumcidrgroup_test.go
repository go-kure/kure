package cilium

import (
	"testing"

	"github.com/cilium/cilium/pkg/policy/api"
)

func TestCreateCiliumCIDRGroup(t *testing.T) {
	obj := CreateCiliumCIDRGroup("my-cidrs")
	if obj == nil {
		t.Fatal("expected non-nil CiliumCIDRGroup")
	}
	if obj.Name != "my-cidrs" {
		t.Errorf("expected Name 'my-cidrs', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumCIDRGroup" {
		t.Errorf("expected Kind 'CiliumCIDRGroup', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
	if obj.Spec.ExternalCIDRs == nil {
		t.Error("expected non-nil ExternalCIDRs slice")
	}
}

func TestAddCiliumCIDRGroupCIDR(t *testing.T) {
	obj := CreateCiliumCIDRGroup("g")
	AddCiliumCIDRGroupCIDR(obj, api.CIDR("10.0.0.0/8"))
	AddCiliumCIDRGroupCIDR(obj, api.CIDR("192.168.0.0/16"))
	if len(obj.Spec.ExternalCIDRs) != 2 {
		t.Fatalf("expected 2 CIDRs, got %d", len(obj.Spec.ExternalCIDRs))
	}
	if string(obj.Spec.ExternalCIDRs[0]) != "10.0.0.0/8" {
		t.Errorf("unexpected first CIDR: %s", obj.Spec.ExternalCIDRs[0])
	}
}

func TestSetCiliumCIDRGroupCIDRs(t *testing.T) {
	obj := CreateCiliumCIDRGroup("g")
	AddCiliumCIDRGroupCIDR(obj, api.CIDR("10.0.0.0/8"))
	SetCiliumCIDRGroupCIDRs(obj, []api.CIDR{"172.16.0.0/12"})
	if len(obj.Spec.ExternalCIDRs) != 1 {
		t.Fatalf("expected 1 CIDR after replace, got %d", len(obj.Spec.ExternalCIDRs))
	}
	if string(obj.Spec.ExternalCIDRs[0]) != "172.16.0.0/12" {
		t.Errorf("unexpected CIDR: %s", obj.Spec.ExternalCIDRs[0])
	}
}
