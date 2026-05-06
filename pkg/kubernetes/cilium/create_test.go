package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
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
		{"CiliumEgressGatewayPolicy", func(t *testing.T) {
			if CiliumEgressGatewayPolicy(nil) != nil {
				t.Error("CiliumEgressGatewayPolicy should return nil for nil config")
			}
		}},
		{"CiliumLocalRedirectPolicy", func(t *testing.T) {
			if CiliumLocalRedirectPolicy(nil) != nil {
				t.Error("CiliumLocalRedirectPolicy should return nil for nil config")
			}
		}},
		{"CiliumLoadBalancerIPPool", func(t *testing.T) {
			if CiliumLoadBalancerIPPool(nil) != nil {
				t.Error("CiliumLoadBalancerIPPool should return nil for nil config")
			}
		}},
		{"CiliumEnvoyConfig", func(t *testing.T) {
			if CiliumEnvoyConfig(nil) != nil {
				t.Error("CiliumEnvoyConfig should return nil for nil config")
			}
		}},
		{"CiliumClusterwideEnvoyConfig", func(t *testing.T) {
			if CiliumClusterwideEnvoyConfig(nil) != nil {
				t.Error("CiliumClusterwideEnvoyConfig should return nil for nil config")
			}
		}},
		{"CiliumBGPClusterConfig", func(t *testing.T) {
			if CiliumBGPClusterConfig(nil) != nil {
				t.Error("CiliumBGPClusterConfig should return nil for nil config")
			}
		}},
		{"CiliumBGPPeerConfig", func(t *testing.T) {
			if CiliumBGPPeerConfig(nil) != nil {
				t.Error("CiliumBGPPeerConfig should return nil for nil config")
			}
		}},
		{"CiliumBGPAdvertisement", func(t *testing.T) {
			if CiliumBGPAdvertisement(nil) != nil {
				t.Error("CiliumBGPAdvertisement should return nil for nil config")
			}
		}},
		{"CiliumBGPNodeConfig", func(t *testing.T) {
			if CiliumBGPNodeConfig(nil) != nil {
				t.Error("CiliumBGPNodeConfig should return nil for nil config")
			}
		}},
		{"CiliumBGPNodeConfigOverride", func(t *testing.T) {
			if CiliumBGPNodeConfigOverride(nil) != nil {
				t.Error("CiliumBGPNodeConfigOverride should return nil for nil config")
			}
		}},
	}

	for _, constructor := range constructors {
		t.Run(constructor.name, constructor.fn)
	}
}

func TestCiliumEgressGatewayPolicy_Success(t *testing.T) {
	cfg := &CiliumEgressGatewayPolicyConfig{Name: "egress-gw"}
	obj := CiliumEgressGatewayPolicy(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumEgressGatewayPolicy")
	}
	if obj.Name != "egress-gw" {
		t.Errorf("expected Name 'egress-gw', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumEgressGatewayPolicy" {
		t.Errorf("expected Kind 'CiliumEgressGatewayPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumEgressGatewayPolicy_WithSpec(t *testing.T) {
	cfg := &CiliumEgressGatewayPolicyConfig{
		Name: "egress-gw",
		Spec: ciliumv2.CiliumEgressGatewayPolicySpec{
			DestinationCIDRs: []ciliumv2.CIDR{"10.0.0.0/8"},
		},
	}
	obj := CiliumEgressGatewayPolicy(cfg)
	if len(obj.Spec.DestinationCIDRs) != 1 {
		t.Fatalf("expected 1 DestinationCIDR, got %d", len(obj.Spec.DestinationCIDRs))
	}
}

func TestCiliumLocalRedirectPolicy_Success(t *testing.T) {
	cfg := &CiliumLocalRedirectPolicyConfig{Name: "lrp", Namespace: "kube-system"}
	obj := CiliumLocalRedirectPolicy(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumLocalRedirectPolicy")
	}
	if obj.Name != "lrp" {
		t.Errorf("expected Name 'lrp', got %s", obj.Name)
	}
	if obj.Namespace != "kube-system" {
		t.Errorf("expected Namespace 'kube-system', got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumLocalRedirectPolicy" {
		t.Errorf("expected Kind 'CiliumLocalRedirectPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumLoadBalancerIPPool_Success(t *testing.T) {
	cfg := &CiliumLoadBalancerIPPoolConfig{Name: "my-pool"}
	obj := CiliumLoadBalancerIPPool(cfg)
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

func TestCiliumEnvoyConfig_Success(t *testing.T) {
	cfg := &CiliumEnvoyConfigConfig{Name: "my-cec", Namespace: "default"}
	obj := CiliumEnvoyConfig(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumEnvoyConfig")
	}
	if obj.Name != "my-cec" {
		t.Errorf("expected Name 'my-cec', got %s", obj.Name)
	}
	if obj.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumEnvoyConfig" {
		t.Errorf("expected Kind 'CiliumEnvoyConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumClusterwideEnvoyConfig_Success(t *testing.T) {
	cfg := &CiliumClusterwideEnvoyConfigConfig{Name: "my-ccec"}
	obj := CiliumClusterwideEnvoyConfig(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumClusterwideEnvoyConfig")
	}
	if obj.Name != "my-ccec" {
		t.Errorf("expected Name 'my-ccec', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumClusterwideEnvoyConfig" {
		t.Errorf("expected Kind 'CiliumClusterwideEnvoyConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumBGPClusterConfig_Success(t *testing.T) {
	cfg := &CiliumBGPClusterConfigConfig{Name: "bgp-cluster"}
	obj := CiliumBGPClusterConfig(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPClusterConfig")
	}
	if obj.Name != "bgp-cluster" {
		t.Errorf("expected Name 'bgp-cluster', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumBGPClusterConfig" {
		t.Errorf("expected Kind 'CiliumBGPClusterConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumBGPPeerConfig_Success(t *testing.T) {
	cfg := &CiliumBGPPeerConfigConfig{Name: "bgp-peer"}
	obj := CiliumBGPPeerConfig(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPPeerConfig")
	}
	if obj.Name != "bgp-peer" {
		t.Errorf("expected Name 'bgp-peer', got %s", obj.Name)
	}
	if obj.Kind != "CiliumBGPPeerConfig" {
		t.Errorf("expected Kind 'CiliumBGPPeerConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumBGPAdvertisement_Success(t *testing.T) {
	cfg := &CiliumBGPAdvertisementConfig{Name: "bgp-advert"}
	obj := CiliumBGPAdvertisement(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPAdvertisement")
	}
	if obj.Name != "bgp-advert" {
		t.Errorf("expected Name 'bgp-advert', got %s", obj.Name)
	}
	if obj.Kind != "CiliumBGPAdvertisement" {
		t.Errorf("expected Kind 'CiliumBGPAdvertisement', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumBGPNodeConfig_Success(t *testing.T) {
	cfg := &CiliumBGPNodeConfigConfig{Name: "bgp-node"}
	obj := CiliumBGPNodeConfig(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPNodeConfig")
	}
	if obj.Name != "bgp-node" {
		t.Errorf("expected Name 'bgp-node', got %s", obj.Name)
	}
	if obj.Kind != "CiliumBGPNodeConfig" {
		t.Errorf("expected Kind 'CiliumBGPNodeConfig', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestCiliumBGPNodeConfigOverride_Success(t *testing.T) {
	cfg := &CiliumBGPNodeConfigOverrideConfig{Name: "bgp-override"}
	obj := CiliumBGPNodeConfigOverride(cfg)
	if obj == nil {
		t.Fatal("expected non-nil CiliumBGPNodeConfigOverride")
	}
	if obj.Name != "bgp-override" {
		t.Errorf("expected Name 'bgp-override', got %s", obj.Name)
	}
	if obj.Kind != "CiliumBGPNodeConfigOverride" {
		t.Errorf("expected Kind 'CiliumBGPNodeConfigOverride', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}
