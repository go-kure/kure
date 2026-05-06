package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
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

func TestCiliumEgressGatewayPolicyConfig_Fields(t *testing.T) {
	cfg := CiliumEgressGatewayPolicyConfig{
		Name: "egress-gw",
		Spec: ciliumv2.CiliumEgressGatewayPolicySpec{
			DestinationCIDRs: []ciliumv2.CIDR{"10.0.0.0/8"},
		},
	}
	if cfg.Name != "egress-gw" {
		t.Errorf("expected Name 'egress-gw', got %s", cfg.Name)
	}
	if len(cfg.Spec.DestinationCIDRs) != 1 {
		t.Errorf("expected 1 DestinationCIDR, got %d", len(cfg.Spec.DestinationCIDRs))
	}
}

func TestCiliumLocalRedirectPolicyConfig_Fields(t *testing.T) {
	cfg := CiliumLocalRedirectPolicyConfig{
		Name:      "lrp",
		Namespace: "kube-system",
	}
	if cfg.Name != "lrp" {
		t.Errorf("expected Name 'lrp', got %s", cfg.Name)
	}
	if cfg.Namespace != "kube-system" {
		t.Errorf("expected Namespace 'kube-system', got %s", cfg.Namespace)
	}
}

func TestCiliumLoadBalancerIPPoolConfig_Fields(t *testing.T) {
	cfg := CiliumLoadBalancerIPPoolConfig{
		Name: "my-pool",
		Spec: ciliumv2.CiliumLoadBalancerIPPoolSpec{
			Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "10.0.0.0/8"}},
		},
	}
	if cfg.Name != "my-pool" {
		t.Errorf("expected Name 'my-pool', got %s", cfg.Name)
	}
	if len(cfg.Spec.Blocks) != 1 {
		t.Errorf("expected 1 block, got %d", len(cfg.Spec.Blocks))
	}
}

func TestCiliumEnvoyConfigConfig_Fields(t *testing.T) {
	cfg := CiliumEnvoyConfigConfig{
		Name:      "my-cec",
		Namespace: "default",
	}
	if cfg.Name != "my-cec" {
		t.Errorf("expected Name 'my-cec', got %s", cfg.Name)
	}
	if cfg.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", cfg.Namespace)
	}
}

func TestCiliumClusterwideEnvoyConfigConfig_Fields(t *testing.T) {
	cfg := CiliumClusterwideEnvoyConfigConfig{Name: "my-ccec"}
	if cfg.Name != "my-ccec" {
		t.Errorf("expected Name 'my-ccec', got %s", cfg.Name)
	}
}

func TestCiliumBGPClusterConfigConfig_Fields(t *testing.T) {
	cfg := CiliumBGPClusterConfigConfig{
		Name: "bgp-cluster",
		Spec: ciliumv2.CiliumBGPClusterConfigSpec{
			BGPInstances: []ciliumv2.CiliumBGPInstance{{Name: "inst"}},
		},
	}
	if cfg.Name != "bgp-cluster" {
		t.Errorf("expected Name 'bgp-cluster', got %s", cfg.Name)
	}
	if len(cfg.Spec.BGPInstances) != 1 {
		t.Errorf("expected 1 BGP instance, got %d", len(cfg.Spec.BGPInstances))
	}
}

func TestCiliumBGPPeerConfigConfig_Fields(t *testing.T) {
	cfg := CiliumBGPPeerConfigConfig{Name: "bgp-peer"}
	if cfg.Name != "bgp-peer" {
		t.Errorf("expected Name 'bgp-peer', got %s", cfg.Name)
	}
}

func TestCiliumBGPAdvertisementConfig_Fields(t *testing.T) {
	cfg := CiliumBGPAdvertisementConfig{
		Name: "bgp-advert",
		Spec: ciliumv2.CiliumBGPAdvertisementSpec{
			Advertisements: []ciliumv2.BGPAdvertisement{
				{AdvertisementType: ciliumv2.BGPServiceAdvert},
			},
		},
	}
	if cfg.Name != "bgp-advert" {
		t.Errorf("expected Name 'bgp-advert', got %s", cfg.Name)
	}
	if len(cfg.Spec.Advertisements) != 1 {
		t.Errorf("expected 1 advertisement, got %d", len(cfg.Spec.Advertisements))
	}
}

func TestCiliumBGPNodeConfigConfig_Fields(t *testing.T) {
	cfg := CiliumBGPNodeConfigConfig{Name: "bgp-node"}
	if cfg.Name != "bgp-node" {
		t.Errorf("expected Name 'bgp-node', got %s", cfg.Name)
	}
}

func TestCiliumBGPNodeConfigOverrideConfig_Fields(t *testing.T) {
	cfg := CiliumBGPNodeConfigOverrideConfig{Name: "bgp-override"}
	if cfg.Name != "bgp-override" {
		t.Errorf("expected Name 'bgp-override', got %s", cfg.Name)
	}
}
