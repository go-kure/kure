package cilium

import (
	intcilium "github.com/go-kure/kure/internal/cilium"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
)

// CiliumNetworkPolicy converts the config to a CiliumNetworkPolicy object.
func CiliumNetworkPolicy(cfg *CiliumNetworkPolicyConfig) *ciliumv2.CiliumNetworkPolicy {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumNetworkPolicy(cfg.Name, cfg.Namespace)
	if cfg.Spec != nil {
		intcilium.SetCiliumNetworkPolicySpec(obj, cfg.Spec)
	}
	for _, spec := range cfg.Specs {
		intcilium.AddCiliumNetworkPolicySpec(obj, spec)
	}
	return obj
}

// CiliumClusterwideNetworkPolicy converts the config to a
// CiliumClusterwideNetworkPolicy object.
func CiliumClusterwideNetworkPolicy(cfg *CiliumClusterwideNetworkPolicyConfig) *ciliumv2.CiliumClusterwideNetworkPolicy {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumClusterwideNetworkPolicy(cfg.Name)
	if cfg.Spec != nil {
		intcilium.SetCiliumClusterwideNetworkPolicySpec(obj, cfg.Spec)
	}
	for _, spec := range cfg.Specs {
		intcilium.AddCiliumClusterwideNetworkPolicySpec(obj, spec)
	}
	return obj
}

// CiliumCIDRGroup converts the config to a CiliumCIDRGroup object.
func CiliumCIDRGroup(cfg *CiliumCIDRGroupConfig) *ciliumv2.CiliumCIDRGroup {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumCIDRGroup(cfg.Name)
	for _, cidr := range cfg.ExternalCIDRs {
		intcilium.AddCiliumCIDRGroupCIDR(obj, cidr)
	}
	return obj
}

// CiliumEgressGatewayPolicy converts the config to a CiliumEgressGatewayPolicy object.
func CiliumEgressGatewayPolicy(cfg *CiliumEgressGatewayPolicyConfig) *ciliumv2.CiliumEgressGatewayPolicy {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumEgressGatewayPolicy(cfg.Name)
	intcilium.SetCiliumEgressGatewayPolicySpec(obj, cfg.Spec)
	return obj
}

// CiliumLocalRedirectPolicy converts the config to a CiliumLocalRedirectPolicy object.
func CiliumLocalRedirectPolicy(cfg *CiliumLocalRedirectPolicyConfig) *ciliumv2.CiliumLocalRedirectPolicy {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumLocalRedirectPolicy(cfg.Name, cfg.Namespace)
	intcilium.SetCiliumLocalRedirectPolicySpec(obj, cfg.Spec)
	return obj
}

// CiliumLoadBalancerIPPool converts the config to a CiliumLoadBalancerIPPool object.
func CiliumLoadBalancerIPPool(cfg *CiliumLoadBalancerIPPoolConfig) *ciliumv2.CiliumLoadBalancerIPPool {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumLoadBalancerIPPool(cfg.Name)
	intcilium.SetCiliumLoadBalancerIPPoolSpec(obj, cfg.Spec)
	return obj
}

// CiliumEnvoyConfig converts the config to a CiliumEnvoyConfig object.
func CiliumEnvoyConfig(cfg *CiliumEnvoyConfigConfig) *ciliumv2.CiliumEnvoyConfig {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumEnvoyConfig(cfg.Name, cfg.Namespace)
	intcilium.SetCiliumEnvoyConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumClusterwideEnvoyConfig converts the config to a CiliumClusterwideEnvoyConfig object.
func CiliumClusterwideEnvoyConfig(cfg *CiliumClusterwideEnvoyConfigConfig) *ciliumv2.CiliumClusterwideEnvoyConfig {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumClusterwideEnvoyConfig(cfg.Name)
	intcilium.SetCiliumClusterwideEnvoyConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPClusterConfig converts the config to a CiliumBGPClusterConfig object.
func CiliumBGPClusterConfig(cfg *CiliumBGPClusterConfigConfig) *ciliumv2.CiliumBGPClusterConfig {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumBGPClusterConfig(cfg.Name)
	intcilium.SetCiliumBGPClusterConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPPeerConfig converts the config to a CiliumBGPPeerConfig object.
func CiliumBGPPeerConfig(cfg *CiliumBGPPeerConfigConfig) *ciliumv2.CiliumBGPPeerConfig {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumBGPPeerConfig(cfg.Name)
	intcilium.SetCiliumBGPPeerConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPAdvertisement converts the config to a CiliumBGPAdvertisement object.
func CiliumBGPAdvertisement(cfg *CiliumBGPAdvertisementConfig) *ciliumv2.CiliumBGPAdvertisement {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumBGPAdvertisement(cfg.Name)
	intcilium.SetCiliumBGPAdvertisementSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPNodeConfig converts the config to a CiliumBGPNodeConfig object.
func CiliumBGPNodeConfig(cfg *CiliumBGPNodeConfigConfig) *ciliumv2.CiliumBGPNodeConfig {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumBGPNodeConfig(cfg.Name)
	intcilium.SetCiliumBGPNodeConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPNodeConfigOverride converts the config to a CiliumBGPNodeConfigOverride object.
func CiliumBGPNodeConfigOverride(cfg *CiliumBGPNodeConfigOverrideConfig) *ciliumv2.CiliumBGPNodeConfigOverride {
	if cfg == nil {
		return nil
	}
	obj := intcilium.CreateCiliumBGPNodeConfigOverride(cfg.Name)
	intcilium.SetCiliumBGPNodeConfigOverrideSpec(obj, cfg.Spec)
	return obj
}
