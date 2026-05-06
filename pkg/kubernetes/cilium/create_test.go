package cilium

import (
	"testing"

	"github.com/cilium/cilium/pkg/policy/api"
)

func TestCiliumNetworkPolicy_Success(t *testing.T) {
	cfg := &CiliumNetworkPolicyConfig{
		Name:      "allow-internal",
		Namespace: "default",
	}

	obj := CiliumNetworkPolicy(cfg)

	if obj == nil {
		t.Fatal("expected non-nil CiliumNetworkPolicy")
	}
	if obj.Name != "allow-internal" {
		t.Errorf("expected Name 'allow-internal', got %s", obj.Name)
	}
	if obj.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumNetworkPolicy" {
		t.Errorf("expected Kind 'CiliumNetworkPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
	if obj.Spec != nil {
		t.Error("expected nil Spec when not configured")
	}
}

func TestCiliumNetworkPolicy_NilConfig(t *testing.T) {
	obj := CiliumNetworkPolicy(nil)
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestCiliumNetworkPolicy_WithSpec(t *testing.T) {
	cfg := &CiliumNetworkPolicyConfig{
		Name:      "p",
		Namespace: "ns",
		Spec:      &api.Rule{Description: "single rule"},
	}

	obj := CiliumNetworkPolicy(cfg)

	if obj.Spec == nil {
		t.Fatal("expected non-nil Spec")
	}
	if obj.Spec.Description != "single rule" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestCiliumNetworkPolicy_WithSpecs(t *testing.T) {
	cfg := &CiliumNetworkPolicyConfig{
		Name:      "p",
		Namespace: "ns",
		Specs:     api.Rules{&api.Rule{Description: "r1"}, &api.Rule{Description: "r2"}},
	}

	obj := CiliumNetworkPolicy(cfg)

	if len(obj.Specs) != 2 {
		t.Fatalf("expected 2 Specs, got %d", len(obj.Specs))
	}
}

func TestCiliumClusterwideNetworkPolicy_Success(t *testing.T) {
	cfg := &CiliumClusterwideNetworkPolicyConfig{
		Name: "cluster-policy",
	}

	obj := CiliumClusterwideNetworkPolicy(cfg)

	if obj == nil {
		t.Fatal("expected non-nil CiliumClusterwideNetworkPolicy")
	}
	if obj.Name != "cluster-policy" {
		t.Errorf("expected Name 'cluster-policy', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumClusterwideNetworkPolicy" {
		t.Errorf("expected Kind 'CiliumClusterwideNetworkPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumClusterwideNetworkPolicy_NilConfig(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(nil)
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestCiliumClusterwideNetworkPolicy_WithSpec(t *testing.T) {
	cfg := &CiliumClusterwideNetworkPolicyConfig{
		Name: "p",
		Spec: &api.Rule{Description: "cluster rule"},
	}

	obj := CiliumClusterwideNetworkPolicy(cfg)

	if obj.Spec == nil {
		t.Fatal("expected non-nil Spec")
	}
	if obj.Spec.Description != "cluster rule" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestCiliumClusterwideNetworkPolicy_WithSpecs(t *testing.T) {
	cfg := &CiliumClusterwideNetworkPolicyConfig{
		Name:  "p",
		Specs: api.Rules{&api.Rule{Description: "r1"}},
	}

	obj := CiliumClusterwideNetworkPolicy(cfg)

	if len(obj.Specs) != 1 {
		t.Fatalf("expected 1 Specs, got %d", len(obj.Specs))
	}
}

func TestCiliumCIDRGroup_Success(t *testing.T) {
	cfg := &CiliumCIDRGroupConfig{
		Name:          "my-cidrs",
		ExternalCIDRs: []api.CIDR{"10.0.0.0/8", "192.168.0.0/16"},
	}

	obj := CiliumCIDRGroup(cfg)

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
	if len(obj.Spec.ExternalCIDRs) != 2 {
		t.Fatalf("expected 2 ExternalCIDRs, got %d", len(obj.Spec.ExternalCIDRs))
	}
	if string(obj.Spec.ExternalCIDRs[0]) != "10.0.0.0/8" {
		t.Errorf("unexpected first CIDR: %s", obj.Spec.ExternalCIDRs[0])
	}
}

func TestCiliumCIDRGroup_NilConfig(t *testing.T) {
	obj := CiliumCIDRGroup(nil)
	if obj != nil {
		t.Error("expected nil result for nil config")
	}
}

func TestCiliumCIDRGroup_EmptyCIDRs(t *testing.T) {
	cfg := &CiliumCIDRGroupConfig{
		Name: "empty-group",
	}

	obj := CiliumCIDRGroup(cfg)

	if obj == nil {
		t.Fatal("expected non-nil CiliumCIDRGroup")
	}
	if obj.Spec.ExternalCIDRs == nil {
		t.Error("expected non-nil ExternalCIDRs slice")
	}
	if len(obj.Spec.ExternalCIDRs) != 0 {
		t.Errorf("expected 0 ExternalCIDRs, got %d", len(obj.Spec.ExternalCIDRs))
	}
}

func TestAllConstructorsWithNilConfig(t *testing.T) {
	constructors := []struct {
		name string
		fn   func(t *testing.T)
	}{
		{"CiliumNetworkPolicy", func(t *testing.T) {
			if CiliumNetworkPolicy(nil) != nil {
				t.Error("CiliumNetworkPolicy should return nil for nil config")
			}
		}},
		{"CiliumClusterwideNetworkPolicy", func(t *testing.T) {
			if CiliumClusterwideNetworkPolicy(nil) != nil {
				t.Error("CiliumClusterwideNetworkPolicy should return nil for nil config")
			}
		}},
		{"CiliumCIDRGroup", func(t *testing.T) {
			if CiliumCIDRGroup(nil) != nil {
				t.Error("CiliumCIDRGroup should return nil for nil config")
			}
		}},
	}

	for _, constructor := range constructors {
		t.Run(constructor.name, constructor.fn)
	}
}
