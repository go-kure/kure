package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/policy/api"
)

// CiliumNetworkPolicyConfig holds the configuration for a CiliumNetworkPolicy.
// Spec and Specs are mutually exclusive: use Spec for a single rule and Specs
// for multiple rules. Both are optional; rules can also be added after
// construction via the Set*/Add* update helpers.
type CiliumNetworkPolicyConfig struct {
	Name      string    `yaml:"name"`
	Namespace string    `yaml:"namespace"`
	Spec      *api.Rule `yaml:"spec,omitempty"`
	Specs     api.Rules `yaml:"specs,omitempty"`
}

// CiliumClusterwideNetworkPolicyConfig holds the configuration for a
// CiliumClusterwideNetworkPolicy. The resource is cluster-scoped, so no
// Namespace field is present. Spec and Specs are mutually exclusive.
type CiliumClusterwideNetworkPolicyConfig struct {
	Name  string    `yaml:"name"`
	Spec  *api.Rule `yaml:"spec,omitempty"`
	Specs api.Rules `yaml:"specs,omitempty"`
}

// CiliumCIDRGroupConfig holds the configuration for a CiliumCIDRGroup.
// The resource is cluster-scoped.
type CiliumCIDRGroupConfig struct {
	Name          string     `yaml:"name"`
	ExternalCIDRs []api.CIDR `yaml:"externalCIDRs,omitempty"`
}

// CiliumEgressGatewayPolicyConfig holds the configuration for a
// CiliumEgressGatewayPolicy. The resource is cluster-scoped.
type CiliumEgressGatewayPolicyConfig struct {
	Name string                                 `yaml:"name"`
	Spec ciliumv2.CiliumEgressGatewayPolicySpec `yaml:"spec,omitempty"`
}

// CiliumLocalRedirectPolicyConfig holds the configuration for a
// CiliumLocalRedirectPolicy. The resource is namespace-scoped.
type CiliumLocalRedirectPolicyConfig struct {
	Name      string                                 `yaml:"name"`
	Namespace string                                 `yaml:"namespace"`
	Spec      ciliumv2.CiliumLocalRedirectPolicySpec `yaml:"spec,omitempty"`
}

// CiliumLoadBalancerIPPoolConfig holds the configuration for a
// CiliumLoadBalancerIPPool. The resource is cluster-scoped.
type CiliumLoadBalancerIPPoolConfig struct {
	Name string                                `yaml:"name"`
	Spec ciliumv2.CiliumLoadBalancerIPPoolSpec `yaml:"spec,omitempty"`
}

// CiliumEnvoyConfigConfig holds the configuration for a CiliumEnvoyConfig.
// The resource is namespace-scoped.
type CiliumEnvoyConfigConfig struct {
	Name      string                         `yaml:"name"`
	Namespace string                         `yaml:"namespace"`
	Spec      ciliumv2.CiliumEnvoyConfigSpec `yaml:"spec,omitempty"`
}

// CiliumClusterwideEnvoyConfigConfig holds the configuration for a
// CiliumClusterwideEnvoyConfig. The resource is cluster-scoped.
type CiliumClusterwideEnvoyConfigConfig struct {
	Name string                         `yaml:"name"`
	Spec ciliumv2.CiliumEnvoyConfigSpec `yaml:"spec,omitempty"`
}

// CiliumBGPClusterConfigConfig holds the configuration for a
// CiliumBGPClusterConfig. The resource is cluster-scoped.
type CiliumBGPClusterConfigConfig struct {
	Name string                              `yaml:"name"`
	Spec ciliumv2.CiliumBGPClusterConfigSpec `yaml:"spec,omitempty"`
}

// CiliumBGPPeerConfigConfig holds the configuration for a CiliumBGPPeerConfig.
// The resource is cluster-scoped.
type CiliumBGPPeerConfigConfig struct {
	Name string                           `yaml:"name"`
	Spec ciliumv2.CiliumBGPPeerConfigSpec `yaml:"spec,omitempty"`
}

// CiliumBGPAdvertisementConfig holds the configuration for a
// CiliumBGPAdvertisement. The resource is cluster-scoped.
type CiliumBGPAdvertisementConfig struct {
	Name string                              `yaml:"name"`
	Spec ciliumv2.CiliumBGPAdvertisementSpec `yaml:"spec,omitempty"`
}

// CiliumBGPNodeConfigConfig holds the configuration for a CiliumBGPNodeConfig.
// The resource is cluster-scoped.
type CiliumBGPNodeConfigConfig struct {
	Name string                     `yaml:"name"`
	Spec ciliumv2.CiliumBGPNodeSpec `yaml:"spec,omitempty"`
}

// CiliumBGPNodeConfigOverrideConfig holds the configuration for a
// CiliumBGPNodeConfigOverride. The resource is cluster-scoped.
type CiliumBGPNodeConfigOverrideConfig struct {
	Name string                                   `yaml:"name"`
	Spec ciliumv2.CiliumBGPNodeConfigOverrideSpec `yaml:"spec,omitempty"`
}
