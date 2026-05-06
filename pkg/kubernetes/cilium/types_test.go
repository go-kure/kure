package cilium

import (
	"testing"

	"github.com/cilium/cilium/pkg/policy/api"
)

func TestCiliumNetworkPolicyConfig_Fields(t *testing.T) {
	spec := &api.Rule{Description: "test"}
	cfg := CiliumNetworkPolicyConfig{
		Name:      "policy",
		Namespace: "default",
		Spec:      spec,
		Specs:     api.Rules{spec},
	}
	if cfg.Name != "policy" {
		t.Errorf("expected Name 'policy', got %s", cfg.Name)
	}
	if cfg.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", cfg.Namespace)
	}
	if cfg.Spec == nil {
		t.Error("expected non-nil Spec")
	}
	if len(cfg.Specs) != 1 {
		t.Errorf("expected 1 Specs entry, got %d", len(cfg.Specs))
	}
}

func TestCiliumClusterwideNetworkPolicyConfig_Fields(t *testing.T) {
	spec := &api.Rule{Description: "cluster"}
	cfg := CiliumClusterwideNetworkPolicyConfig{
		Name:  "cluster-policy",
		Spec:  spec,
		Specs: api.Rules{spec},
	}
	if cfg.Name != "cluster-policy" {
		t.Errorf("expected Name 'cluster-policy', got %s", cfg.Name)
	}
	if cfg.Spec == nil {
		t.Error("expected non-nil Spec")
	}
	if len(cfg.Specs) != 1 {
		t.Errorf("expected 1 Specs entry, got %d", len(cfg.Specs))
	}
}

func TestCiliumCIDRGroupConfig_Fields(t *testing.T) {
	cfg := CiliumCIDRGroupConfig{
		Name:          "internal-ranges",
		ExternalCIDRs: []api.CIDR{"10.0.0.0/8", "192.168.0.0/16"},
	}
	if cfg.Name != "internal-ranges" {
		t.Errorf("expected Name 'internal-ranges', got %s", cfg.Name)
	}
	if len(cfg.ExternalCIDRs) != 2 {
		t.Errorf("expected 2 ExternalCIDRs, got %d", len(cfg.ExternalCIDRs))
	}
	if string(cfg.ExternalCIDRs[0]) != "10.0.0.0/8" {
		t.Errorf("unexpected first CIDR: %s", cfg.ExternalCIDRs[0])
	}
}
