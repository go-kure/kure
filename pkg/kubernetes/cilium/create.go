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
