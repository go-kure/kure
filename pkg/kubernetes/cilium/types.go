package cilium

import (
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
