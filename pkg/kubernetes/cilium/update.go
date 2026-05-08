package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
)

// SetCiliumNetworkPolicySpec sets the single-rule spec on the policy.
func SetCiliumNetworkPolicySpec(obj *ciliumv2.CiliumNetworkPolicy, spec *api.Rule) {
	obj.Spec = spec
}

// SetCiliumNetworkPolicySpecs replaces the multi-rule specs on the policy.
func SetCiliumNetworkPolicySpecs(obj *ciliumv2.CiliumNetworkPolicy, specs api.Rules) {
	obj.Specs = specs
}

// AddCiliumNetworkPolicySpec appends a rule to the multi-rule Specs list.
func AddCiliumNetworkPolicySpec(obj *ciliumv2.CiliumNetworkPolicy, spec *api.Rule) {
	obj.Specs = append(obj.Specs, spec)
}

// SetCiliumNetworkPolicyEndpointSelector sets the endpoint selector on the
// policy, initialising Spec if nil.
func SetCiliumNetworkPolicyEndpointSelector(obj *ciliumv2.CiliumNetworkPolicy, sel api.EndpointSelector) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EndpointSelector = sel
}

// AddCiliumNetworkPolicyIngressRule appends an ingress allow rule, initialising
// Spec if nil.
func AddCiliumNetworkPolicyIngressRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.IngressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Ingress = append(obj.Spec.Ingress, rule)
}

// AddCiliumNetworkPolicyIngressDenyRule appends an ingress deny rule,
// initialising Spec if nil.
func AddCiliumNetworkPolicyIngressDenyRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.IngressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.IngressDeny = append(obj.Spec.IngressDeny, rule)
}

// AddCiliumNetworkPolicyEgressRule appends an egress allow rule, initialising
// Spec if nil.
func AddCiliumNetworkPolicyEgressRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.EgressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Egress = append(obj.Spec.Egress, rule)
}

// AddCiliumNetworkPolicyEgressDenyRule appends an egress deny rule,
// initialising Spec if nil.
func AddCiliumNetworkPolicyEgressDenyRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.EgressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EgressDeny = append(obj.Spec.EgressDeny, rule)
}

// SetCiliumNetworkPolicyDescription sets the description on the policy,
// initialising Spec if nil.
func SetCiliumNetworkPolicyDescription(obj *ciliumv2.CiliumNetworkPolicy, desc string) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Description = desc
}

// SetCiliumNetworkPolicyLabels sets the rule labels on the policy, initialising
// Spec if nil.
func SetCiliumNetworkPolicyLabels(obj *ciliumv2.CiliumNetworkPolicy, lbls labels.LabelArray) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Labels = lbls
}

// SetCiliumNetworkPolicyEnableDefaultDeny sets the EnableDefaultDeny field on
// the policy, initialising Spec if nil.
func SetCiliumNetworkPolicyEnableDefaultDeny(obj *ciliumv2.CiliumNetworkPolicy, cfg api.DefaultDenyConfig) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EnableDefaultDeny = cfg
}

// SetCiliumClusterwideNetworkPolicySpec sets the single-rule spec on the policy.
func SetCiliumClusterwideNetworkPolicySpec(obj *ciliumv2.CiliumClusterwideNetworkPolicy, spec *api.Rule) {
	obj.Spec = spec
}

// SetCiliumClusterwideNetworkPolicySpecs replaces the multi-rule specs on the
// policy.
func SetCiliumClusterwideNetworkPolicySpecs(obj *ciliumv2.CiliumClusterwideNetworkPolicy, specs api.Rules) {
	obj.Specs = specs
}

// AddCiliumClusterwideNetworkPolicySpec appends a rule to the multi-rule Specs
// list.
func AddCiliumClusterwideNetworkPolicySpec(obj *ciliumv2.CiliumClusterwideNetworkPolicy, spec *api.Rule) {
	obj.Specs = append(obj.Specs, spec)
}

// SetCiliumClusterwideNetworkPolicyEndpointSelector sets the endpoint selector
// on the policy, initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyEndpointSelector(obj *ciliumv2.CiliumClusterwideNetworkPolicy, sel api.EndpointSelector) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EndpointSelector = sel
}

// SetCiliumClusterwideNetworkPolicyNodeSelector sets the node selector on the
// policy, initialising Spec if nil. NodeSelector is only valid on
// CiliumClusterwideNetworkPolicy.
func SetCiliumClusterwideNetworkPolicyNodeSelector(obj *ciliumv2.CiliumClusterwideNetworkPolicy, sel api.EndpointSelector) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.NodeSelector = sel
}

// AddCiliumClusterwideNetworkPolicyIngressRule appends an ingress allow rule,
// initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyIngressRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.IngressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Ingress = append(obj.Spec.Ingress, rule)
}

// AddCiliumClusterwideNetworkPolicyIngressDenyRule appends an ingress deny
// rule, initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyIngressDenyRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.IngressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.IngressDeny = append(obj.Spec.IngressDeny, rule)
}

// AddCiliumClusterwideNetworkPolicyEgressRule appends an egress allow rule,
// initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyEgressRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.EgressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Egress = append(obj.Spec.Egress, rule)
}

// AddCiliumClusterwideNetworkPolicyEgressDenyRule appends an egress deny rule,
// initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyEgressDenyRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.EgressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EgressDeny = append(obj.Spec.EgressDeny, rule)
}

// SetCiliumClusterwideNetworkPolicyDescription sets the description on the
// policy, initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyDescription(obj *ciliumv2.CiliumClusterwideNetworkPolicy, desc string) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Description = desc
}

// SetCiliumClusterwideNetworkPolicyLabels sets the rule labels on the policy,
// initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyLabels(obj *ciliumv2.CiliumClusterwideNetworkPolicy, lbls labels.LabelArray) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Labels = lbls
}

// SetCiliumClusterwideNetworkPolicyEnableDefaultDeny sets the EnableDefaultDeny
// field on the policy, initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyEnableDefaultDeny(obj *ciliumv2.CiliumClusterwideNetworkPolicy, cfg api.DefaultDenyConfig) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EnableDefaultDeny = cfg
}

// AddCiliumCIDRGroupCIDR appends a CIDR to the group's ExternalCIDRs list.
func AddCiliumCIDRGroupCIDR(obj *ciliumv2.CiliumCIDRGroup, cidr api.CIDR) {
	obj.Spec.ExternalCIDRs = append(obj.Spec.ExternalCIDRs, cidr)
}

// SetCiliumCIDRGroupCIDRs replaces the ExternalCIDRs list on the group.
func SetCiliumCIDRGroupCIDRs(obj *ciliumv2.CiliumCIDRGroup, cidrs []api.CIDR) {
	obj.Spec.ExternalCIDRs = cidrs
}

// SetCiliumEgressGatewayPolicySpec sets the full spec on the policy.
func SetCiliumEgressGatewayPolicySpec(obj *ciliumv2.CiliumEgressGatewayPolicy, spec ciliumv2.CiliumEgressGatewayPolicySpec) {
	obj.Spec = spec
}

// AddCiliumEgressGatewayPolicySelectorRule appends an egress selector rule.
func AddCiliumEgressGatewayPolicySelectorRule(obj *ciliumv2.CiliumEgressGatewayPolicy, rule ciliumv2.EgressRule) {
	obj.Spec.Selectors = append(obj.Spec.Selectors, rule)
}

// AddCiliumEgressGatewayPolicyDestinationCIDR appends a destination CIDR.
func AddCiliumEgressGatewayPolicyDestinationCIDR(obj *ciliumv2.CiliumEgressGatewayPolicy, cidr ciliumv2.CIDR) {
	obj.Spec.DestinationCIDRs = append(obj.Spec.DestinationCIDRs, cidr)
}

// AddCiliumEgressGatewayPolicyExcludedCIDR appends an excluded CIDR.
func AddCiliumEgressGatewayPolicyExcludedCIDR(obj *ciliumv2.CiliumEgressGatewayPolicy, cidr ciliumv2.CIDR) {
	obj.Spec.ExcludedCIDRs = append(obj.Spec.ExcludedCIDRs, cidr)
}

// SetCiliumEgressGatewayPolicyEgressGateway sets the primary egress gateway.
func SetCiliumEgressGatewayPolicyEgressGateway(obj *ciliumv2.CiliumEgressGatewayPolicy, gw *ciliumv2.EgressGateway) {
	obj.Spec.EgressGateway = gw
}

// AddCiliumEgressGatewayPolicyEgressGateway appends to the multi-gateway list.
func AddCiliumEgressGatewayPolicyEgressGateway(obj *ciliumv2.CiliumEgressGatewayPolicy, gw ciliumv2.EgressGateway) {
	obj.Spec.EgressGateways = append(obj.Spec.EgressGateways, gw)
}

// SetCiliumLocalRedirectPolicySpec sets the full spec on the policy.
func SetCiliumLocalRedirectPolicySpec(obj *ciliumv2.CiliumLocalRedirectPolicy, spec ciliumv2.CiliumLocalRedirectPolicySpec) {
	obj.Spec = spec
}

// SetCiliumLocalRedirectPolicyFrontend sets the redirect frontend.
func SetCiliumLocalRedirectPolicyFrontend(obj *ciliumv2.CiliumLocalRedirectPolicy, frontend ciliumv2.RedirectFrontend) {
	obj.Spec.RedirectFrontend = frontend
}

// SetCiliumLocalRedirectPolicyBackend sets the redirect backend.
func SetCiliumLocalRedirectPolicyBackend(obj *ciliumv2.CiliumLocalRedirectPolicy, backend ciliumv2.RedirectBackend) {
	obj.Spec.RedirectBackend = backend
}

// SetCiliumLocalRedirectPolicyDescription sets the description.
func SetCiliumLocalRedirectPolicyDescription(obj *ciliumv2.CiliumLocalRedirectPolicy, desc string) {
	obj.Spec.Description = desc
}

// SetCiliumLocalRedirectPolicySkipRedirectFromBackend sets the skip-redirect flag.
func SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj *ciliumv2.CiliumLocalRedirectPolicy, skip bool) {
	obj.Spec.SkipRedirectFromBackend = skip
}

// SetCiliumLoadBalancerIPPoolSpec sets the full spec on the pool.
func SetCiliumLoadBalancerIPPoolSpec(obj *ciliumv2.CiliumLoadBalancerIPPool, spec ciliumv2.CiliumLoadBalancerIPPoolSpec) {
	obj.Spec = spec
}

// SetCiliumLoadBalancerIPPoolServiceSelector sets the service selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumLoadBalancerIPPoolServiceSelector(obj *ciliumv2.CiliumLoadBalancerIPPool, sel *slimv1.LabelSelector) {
	obj.Spec.ServiceSelector = sel
}

// AddCiliumLoadBalancerIPPoolBlock appends a CIDR block to the pool.
func AddCiliumLoadBalancerIPPoolBlock(obj *ciliumv2.CiliumLoadBalancerIPPool, block ciliumv2.CiliumLoadBalancerIPPoolIPBlock) {
	obj.Spec.Blocks = append(obj.Spec.Blocks, block)
}

// SetCiliumLoadBalancerIPPoolDisabled enables or disables the pool.
func SetCiliumLoadBalancerIPPoolDisabled(obj *ciliumv2.CiliumLoadBalancerIPPool, disabled bool) {
	obj.Spec.Disabled = disabled
}

// SetCiliumLoadBalancerIPPoolAllowFirstLastIPs controls first/last IP allocation.
func SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj *ciliumv2.CiliumLoadBalancerIPPool, allow ciliumv2.AllowFirstLastIPType) {
	obj.Spec.AllowFirstLastIPs = allow
}

// SetCiliumEnvoyConfigSpec sets the full spec on the config.
func SetCiliumEnvoyConfigSpec(obj *ciliumv2.CiliumEnvoyConfig, spec ciliumv2.CiliumEnvoyConfigSpec) {
	obj.Spec = spec
}

// AddCiliumEnvoyConfigService appends a service listener.
func AddCiliumEnvoyConfigService(obj *ciliumv2.CiliumEnvoyConfig, svc *ciliumv2.ServiceListener) {
	obj.Spec.Services = append(obj.Spec.Services, svc)
}

// AddCiliumEnvoyConfigBackendService appends a backend service.
func AddCiliumEnvoyConfigBackendService(obj *ciliumv2.CiliumEnvoyConfig, svc *ciliumv2.Service) {
	obj.Spec.BackendServices = append(obj.Spec.BackendServices, svc)
}

// AddCiliumEnvoyConfigResource appends an Envoy xDS resource.
func AddCiliumEnvoyConfigResource(obj *ciliumv2.CiliumEnvoyConfig, res ciliumv2.XDSResource) {
	obj.Spec.Resources = append(obj.Spec.Resources, res)
}

// SetCiliumEnvoyConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumEnvoyConfigNodeSelector(obj *ciliumv2.CiliumEnvoyConfig, sel *slimv1.LabelSelector) {
	obj.Spec.NodeSelector = sel
}

// SetCiliumClusterwideEnvoyConfigSpec sets the full spec on the config.
func SetCiliumClusterwideEnvoyConfigSpec(obj *ciliumv2.CiliumClusterwideEnvoyConfig, spec ciliumv2.CiliumEnvoyConfigSpec) {
	obj.Spec = spec
}

// AddCiliumClusterwideEnvoyConfigService appends a service listener.
func AddCiliumClusterwideEnvoyConfigService(obj *ciliumv2.CiliumClusterwideEnvoyConfig, svc *ciliumv2.ServiceListener) {
	obj.Spec.Services = append(obj.Spec.Services, svc)
}

// AddCiliumClusterwideEnvoyConfigBackendService appends a backend service.
func AddCiliumClusterwideEnvoyConfigBackendService(obj *ciliumv2.CiliumClusterwideEnvoyConfig, svc *ciliumv2.Service) {
	obj.Spec.BackendServices = append(obj.Spec.BackendServices, svc)
}

// AddCiliumClusterwideEnvoyConfigResource appends an Envoy xDS resource.
func AddCiliumClusterwideEnvoyConfigResource(obj *ciliumv2.CiliumClusterwideEnvoyConfig, res ciliumv2.XDSResource) {
	obj.Spec.Resources = append(obj.Spec.Resources, res)
}

// SetCiliumClusterwideEnvoyConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumClusterwideEnvoyConfigNodeSelector(obj *ciliumv2.CiliumClusterwideEnvoyConfig, sel *slimv1.LabelSelector) {
	obj.Spec.NodeSelector = sel
}

// SetCiliumBGPClusterConfigSpec sets the full spec on the config.
func SetCiliumBGPClusterConfigSpec(obj *ciliumv2.CiliumBGPClusterConfig, spec ciliumv2.CiliumBGPClusterConfigSpec) {
	obj.Spec = spec
}

// SetCiliumBGPClusterConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumBGPClusterConfigNodeSelector(obj *ciliumv2.CiliumBGPClusterConfig, sel *slimv1.LabelSelector) {
	obj.Spec.NodeSelector = sel
}

// AddCiliumBGPClusterConfigBGPInstance appends a BGP instance.
func AddCiliumBGPClusterConfigBGPInstance(obj *ciliumv2.CiliumBGPClusterConfig, instance ciliumv2.CiliumBGPInstance) {
	obj.Spec.BGPInstances = append(obj.Spec.BGPInstances, instance)
}

// SetCiliumBGPPeerConfigSpec sets the full spec on the config.
func SetCiliumBGPPeerConfigSpec(obj *ciliumv2.CiliumBGPPeerConfig, spec ciliumv2.CiliumBGPPeerConfigSpec) {
	obj.Spec = spec
}

// SetCiliumBGPPeerConfigTransport sets the transport configuration.
func SetCiliumBGPPeerConfigTransport(obj *ciliumv2.CiliumBGPPeerConfig, transport *ciliumv2.CiliumBGPTransport) {
	obj.Spec.Transport = transport
}

// SetCiliumBGPPeerConfigTimers sets the BGP timer configuration.
func SetCiliumBGPPeerConfigTimers(obj *ciliumv2.CiliumBGPPeerConfig, timers *ciliumv2.CiliumBGPTimers) {
	obj.Spec.Timers = timers
}

// SetCiliumBGPPeerConfigAuthSecretRef sets the BGP authentication secret name.
func SetCiliumBGPPeerConfigAuthSecretRef(obj *ciliumv2.CiliumBGPPeerConfig, ref string) {
	obj.Spec.AuthSecretRef = &ref
}

// SetCiliumBGPPeerConfigEBGPMultihop sets the eBGP multihop TTL.
func SetCiliumBGPPeerConfigEBGPMultihop(obj *ciliumv2.CiliumBGPPeerConfig, ttl int32) {
	obj.Spec.EBGPMultihop = &ttl
}

// SetCiliumBGPPeerConfigGracefulRestart sets the graceful restart configuration.
func SetCiliumBGPPeerConfigGracefulRestart(obj *ciliumv2.CiliumBGPPeerConfig, gr *ciliumv2.CiliumBGPNeighborGracefulRestart) {
	obj.Spec.GracefulRestart = gr
}

// AddCiliumBGPPeerConfigFamily appends an address family with advertisements.
func AddCiliumBGPPeerConfigFamily(obj *ciliumv2.CiliumBGPPeerConfig, family ciliumv2.CiliumBGPFamilyWithAdverts) {
	obj.Spec.Families = append(obj.Spec.Families, family)
}

// SetCiliumBGPAdvertisementSpec sets the full spec on the advertisement.
func SetCiliumBGPAdvertisementSpec(obj *ciliumv2.CiliumBGPAdvertisement, spec ciliumv2.CiliumBGPAdvertisementSpec) {
	obj.Spec = spec
}

// AddCiliumBGPAdvertisementEntry appends a BGP advertisement entry.
func AddCiliumBGPAdvertisementEntry(obj *ciliumv2.CiliumBGPAdvertisement, advert ciliumv2.BGPAdvertisement) {
	obj.Spec.Advertisements = append(obj.Spec.Advertisements, advert)
}

// SetCiliumBGPNodeConfigSpec sets the full spec on the node config.
func SetCiliumBGPNodeConfigSpec(obj *ciliumv2.CiliumBGPNodeConfig, spec ciliumv2.CiliumBGPNodeSpec) {
	obj.Spec = spec
}

// AddCiliumBGPNodeConfigBGPInstance appends a BGP node instance.
func AddCiliumBGPNodeConfigBGPInstance(obj *ciliumv2.CiliumBGPNodeConfig, instance ciliumv2.CiliumBGPNodeInstance) {
	obj.Spec.BGPInstances = append(obj.Spec.BGPInstances, instance)
}

// SetCiliumBGPNodeConfigOverrideSpec sets the full spec on the override.
func SetCiliumBGPNodeConfigOverrideSpec(obj *ciliumv2.CiliumBGPNodeConfigOverride, spec ciliumv2.CiliumBGPNodeConfigOverrideSpec) {
	obj.Spec = spec
}

// AddCiliumBGPNodeConfigOverrideBGPInstance appends a BGP instance override.
func AddCiliumBGPNodeConfigOverrideBGPInstance(obj *ciliumv2.CiliumBGPNodeConfigOverride, instance ciliumv2.CiliumBGPNodeConfigInstanceOverride) {
	obj.Spec.BGPInstances = append(obj.Spec.BGPInstances, instance)
}
