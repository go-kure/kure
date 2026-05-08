package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/policy/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumNetworkPolicy returns a new CiliumNetworkPolicy with TypeMeta and ObjectMeta set.
func CreateCiliumNetworkPolicy(name, namespace string) *ciliumv2.CiliumNetworkPolicy {
	return &ciliumv2.CiliumNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CNPKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateCiliumClusterwideNetworkPolicy returns a new CiliumClusterwideNetworkPolicy with TypeMeta and ObjectMeta set.
func CreateCiliumClusterwideNetworkPolicy(name string) *ciliumv2.CiliumClusterwideNetworkPolicy {
	return &ciliumv2.CiliumClusterwideNetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CCNPKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumCIDRGroup returns a new CiliumCIDRGroup with TypeMeta and ObjectMeta set.
func CreateCiliumCIDRGroup(name string) *ciliumv2.CiliumCIDRGroup {
	return &ciliumv2.CiliumCIDRGroup{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CCGKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
		Spec: ciliumv2.CiliumCIDRGroupSpec{
			ExternalCIDRs: []api.CIDR{},
		},
	}
}

// CreateCiliumEgressGatewayPolicy returns a new CiliumEgressGatewayPolicy with TypeMeta and ObjectMeta set.
func CreateCiliumEgressGatewayPolicy(name string) *ciliumv2.CiliumEgressGatewayPolicy {
	return &ciliumv2.CiliumEgressGatewayPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CEGPKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumLocalRedirectPolicy returns a new CiliumLocalRedirectPolicy with TypeMeta and ObjectMeta set.
func CreateCiliumLocalRedirectPolicy(name, namespace string) *ciliumv2.CiliumLocalRedirectPolicy {
	return &ciliumv2.CiliumLocalRedirectPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CLRPKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateCiliumLoadBalancerIPPool returns a new CiliumLoadBalancerIPPool with TypeMeta and ObjectMeta set.
func CreateCiliumLoadBalancerIPPool(name string) *ciliumv2.CiliumLoadBalancerIPPool {
	return &ciliumv2.CiliumLoadBalancerIPPool{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.PoolKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumEnvoyConfig returns a new CiliumEnvoyConfig with TypeMeta and ObjectMeta set.
func CreateCiliumEnvoyConfig(name, namespace string) *ciliumv2.CiliumEnvoyConfig {
	return &ciliumv2.CiliumEnvoyConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CECKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
}

// CreateCiliumClusterwideEnvoyConfig returns a new CiliumClusterwideEnvoyConfig with TypeMeta and ObjectMeta set.
func CreateCiliumClusterwideEnvoyConfig(name string) *ciliumv2.CiliumClusterwideEnvoyConfig {
	return &ciliumv2.CiliumClusterwideEnvoyConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.CCECKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumBGPClusterConfig returns a new CiliumBGPClusterConfig with TypeMeta and ObjectMeta set.
func CreateCiliumBGPClusterConfig(name string) *ciliumv2.CiliumBGPClusterConfig {
	return &ciliumv2.CiliumBGPClusterConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPCCKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumBGPPeerConfig returns a new CiliumBGPPeerConfig with TypeMeta and ObjectMeta set.
func CreateCiliumBGPPeerConfig(name string) *ciliumv2.CiliumBGPPeerConfig {
	return &ciliumv2.CiliumBGPPeerConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPPCKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumBGPAdvertisement returns a new CiliumBGPAdvertisement with TypeMeta and ObjectMeta set.
func CreateCiliumBGPAdvertisement(name string) *ciliumv2.CiliumBGPAdvertisement {
	return &ciliumv2.CiliumBGPAdvertisement{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPAKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumBGPNodeConfig returns a new CiliumBGPNodeConfig with TypeMeta and ObjectMeta set.
func CreateCiliumBGPNodeConfig(name string) *ciliumv2.CiliumBGPNodeConfig {
	return &ciliumv2.CiliumBGPNodeConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPNCKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// CreateCiliumBGPNodeConfigOverride returns a new CiliumBGPNodeConfigOverride with TypeMeta and ObjectMeta set.
func CreateCiliumBGPNodeConfigOverride(name string) *ciliumv2.CiliumBGPNodeConfigOverride {
	return &ciliumv2.CiliumBGPNodeConfigOverride{
		TypeMeta: metav1.TypeMeta{
			Kind:       ciliumv2.BGPNCOKindDefinition,
			APIVersion: ciliumv2.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
		},
	}
}

// Config-based constructors (convenience wrappers using Style B for backwards compat)

// CiliumNetworkPolicy converts the config to a CiliumNetworkPolicy object.
func CiliumNetworkPolicy(cfg *CiliumNetworkPolicyConfig) *ciliumv2.CiliumNetworkPolicy {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumNetworkPolicy(cfg.Name, cfg.Namespace)
	if cfg.Spec != nil {
		SetCiliumNetworkPolicySpec(obj, cfg.Spec)
	}
	for _, spec := range cfg.Specs {
		AddCiliumNetworkPolicySpec(obj, spec)
	}
	return obj
}

// CiliumClusterwideNetworkPolicy converts the config to a CiliumClusterwideNetworkPolicy object.
func CiliumClusterwideNetworkPolicy(cfg *CiliumClusterwideNetworkPolicyConfig) *ciliumv2.CiliumClusterwideNetworkPolicy {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumClusterwideNetworkPolicy(cfg.Name)
	if cfg.Spec != nil {
		SetCiliumClusterwideNetworkPolicySpec(obj, cfg.Spec)
	}
	for _, spec := range cfg.Specs {
		AddCiliumClusterwideNetworkPolicySpec(obj, spec)
	}
	return obj
}

// CiliumCIDRGroup converts the config to a CiliumCIDRGroup object.
func CiliumCIDRGroup(cfg *CiliumCIDRGroupConfig) *ciliumv2.CiliumCIDRGroup {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumCIDRGroup(cfg.Name)
	for _, cidr := range cfg.ExternalCIDRs {
		AddCiliumCIDRGroupCIDR(obj, cidr)
	}
	return obj
}

// CiliumEgressGatewayPolicy converts the config to a CiliumEgressGatewayPolicy object.
func CiliumEgressGatewayPolicy(cfg *CiliumEgressGatewayPolicyConfig) *ciliumv2.CiliumEgressGatewayPolicy {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumEgressGatewayPolicy(cfg.Name)
	SetCiliumEgressGatewayPolicySpec(obj, cfg.Spec)
	return obj
}

// CiliumLocalRedirectPolicy converts the config to a CiliumLocalRedirectPolicy object.
func CiliumLocalRedirectPolicy(cfg *CiliumLocalRedirectPolicyConfig) *ciliumv2.CiliumLocalRedirectPolicy {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumLocalRedirectPolicy(cfg.Name, cfg.Namespace)
	SetCiliumLocalRedirectPolicySpec(obj, cfg.Spec)
	return obj
}

// CiliumLoadBalancerIPPool converts the config to a CiliumLoadBalancerIPPool object.
func CiliumLoadBalancerIPPool(cfg *CiliumLoadBalancerIPPoolConfig) *ciliumv2.CiliumLoadBalancerIPPool {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumLoadBalancerIPPool(cfg.Name)
	SetCiliumLoadBalancerIPPoolSpec(obj, cfg.Spec)
	return obj
}

// CiliumEnvoyConfig converts the config to a CiliumEnvoyConfig object.
func CiliumEnvoyConfig(cfg *CiliumEnvoyConfigConfig) *ciliumv2.CiliumEnvoyConfig {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumEnvoyConfig(cfg.Name, cfg.Namespace)
	SetCiliumEnvoyConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumClusterwideEnvoyConfig converts the config to a CiliumClusterwideEnvoyConfig object.
func CiliumClusterwideEnvoyConfig(cfg *CiliumClusterwideEnvoyConfigConfig) *ciliumv2.CiliumClusterwideEnvoyConfig {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumClusterwideEnvoyConfig(cfg.Name)
	SetCiliumClusterwideEnvoyConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPClusterConfig converts the config to a CiliumBGPClusterConfig object.
func CiliumBGPClusterConfig(cfg *CiliumBGPClusterConfigConfig) *ciliumv2.CiliumBGPClusterConfig {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumBGPClusterConfig(cfg.Name)
	SetCiliumBGPClusterConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPPeerConfig converts the config to a CiliumBGPPeerConfig object.
func CiliumBGPPeerConfig(cfg *CiliumBGPPeerConfigConfig) *ciliumv2.CiliumBGPPeerConfig {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumBGPPeerConfig(cfg.Name)
	SetCiliumBGPPeerConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPAdvertisement converts the config to a CiliumBGPAdvertisement object.
func CiliumBGPAdvertisement(cfg *CiliumBGPAdvertisementConfig) *ciliumv2.CiliumBGPAdvertisement {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumBGPAdvertisement(cfg.Name)
	SetCiliumBGPAdvertisementSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPNodeConfig converts the config to a CiliumBGPNodeConfig object.
func CiliumBGPNodeConfig(cfg *CiliumBGPNodeConfigConfig) *ciliumv2.CiliumBGPNodeConfig {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumBGPNodeConfig(cfg.Name)
	SetCiliumBGPNodeConfigSpec(obj, cfg.Spec)
	return obj
}

// CiliumBGPNodeConfigOverride converts the config to a CiliumBGPNodeConfigOverride object.
func CiliumBGPNodeConfigOverride(cfg *CiliumBGPNodeConfigOverrideConfig) *ciliumv2.CiliumBGPNodeConfigOverride {
	if cfg == nil {
		return nil
	}
	obj := CreateCiliumBGPNodeConfigOverride(cfg.Name)
	SetCiliumBGPNodeConfigOverrideSpec(obj, cfg.Spec)
	return obj
}
