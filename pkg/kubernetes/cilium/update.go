package cilium

import (
	intcilium "github.com/go-kure/kure/internal/cilium"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
)

// SetCiliumNetworkPolicySpec sets the single-rule spec on the policy.
func SetCiliumNetworkPolicySpec(obj *ciliumv2.CiliumNetworkPolicy, spec *api.Rule) {
	intcilium.SetCiliumNetworkPolicySpec(obj, spec)
}

// SetCiliumNetworkPolicySpecs replaces the multi-rule specs on the policy.
func SetCiliumNetworkPolicySpecs(obj *ciliumv2.CiliumNetworkPolicy, specs api.Rules) {
	intcilium.SetCiliumNetworkPolicySpecs(obj, specs)
}

// AddCiliumNetworkPolicySpec appends a rule to the multi-rule Specs list.
func AddCiliumNetworkPolicySpec(obj *ciliumv2.CiliumNetworkPolicy, spec *api.Rule) {
	intcilium.AddCiliumNetworkPolicySpec(obj, spec)
}

// SetCiliumNetworkPolicyEndpointSelector sets the endpoint selector on the
// policy, initialising Spec if nil.
func SetCiliumNetworkPolicyEndpointSelector(obj *ciliumv2.CiliumNetworkPolicy, sel api.EndpointSelector) {
	intcilium.SetCiliumNetworkPolicyEndpointSelector(obj, sel)
}

// AddCiliumNetworkPolicyIngressRule appends an ingress allow rule, initialising
// Spec if nil.
func AddCiliumNetworkPolicyIngressRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.IngressRule) {
	intcilium.AddCiliumNetworkPolicyIngressRule(obj, rule)
}

// AddCiliumNetworkPolicyIngressDenyRule appends an ingress deny rule,
// initialising Spec if nil.
func AddCiliumNetworkPolicyIngressDenyRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.IngressDenyRule) {
	intcilium.AddCiliumNetworkPolicyIngressDenyRule(obj, rule)
}

// AddCiliumNetworkPolicyEgressRule appends an egress allow rule, initialising
// Spec if nil.
func AddCiliumNetworkPolicyEgressRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.EgressRule) {
	intcilium.AddCiliumNetworkPolicyEgressRule(obj, rule)
}

// AddCiliumNetworkPolicyEgressDenyRule appends an egress deny rule,
// initialising Spec if nil.
func AddCiliumNetworkPolicyEgressDenyRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.EgressDenyRule) {
	intcilium.AddCiliumNetworkPolicyEgressDenyRule(obj, rule)
}

// SetCiliumNetworkPolicyDescription sets the description on the policy,
// initialising Spec if nil.
func SetCiliumNetworkPolicyDescription(obj *ciliumv2.CiliumNetworkPolicy, desc string) {
	intcilium.SetCiliumNetworkPolicyDescription(obj, desc)
}

// SetCiliumNetworkPolicyLabels sets the rule labels on the policy, initialising
// Spec if nil. Labels are used by tooling such as Hubble to identify and filter
// policies.
func SetCiliumNetworkPolicyLabels(obj *ciliumv2.CiliumNetworkPolicy, lbls labels.LabelArray) {
	intcilium.SetCiliumNetworkPolicyLabels(obj, lbls)
}

// SetCiliumNetworkPolicyEnableDefaultDeny sets the EnableDefaultDeny field on
// the policy, initialising Spec if nil.
func SetCiliumNetworkPolicyEnableDefaultDeny(obj *ciliumv2.CiliumNetworkPolicy, cfg api.DefaultDenyConfig) {
	intcilium.SetCiliumNetworkPolicyEnableDefaultDeny(obj, cfg)
}

// SetCiliumClusterwideNetworkPolicySpec sets the single-rule spec on the policy.
func SetCiliumClusterwideNetworkPolicySpec(obj *ciliumv2.CiliumClusterwideNetworkPolicy, spec *api.Rule) {
	intcilium.SetCiliumClusterwideNetworkPolicySpec(obj, spec)
}

// SetCiliumClusterwideNetworkPolicySpecs replaces the multi-rule specs on the
// policy.
func SetCiliumClusterwideNetworkPolicySpecs(obj *ciliumv2.CiliumClusterwideNetworkPolicy, specs api.Rules) {
	intcilium.SetCiliumClusterwideNetworkPolicySpecs(obj, specs)
}

// AddCiliumClusterwideNetworkPolicySpec appends a rule to the multi-rule Specs
// list.
func AddCiliumClusterwideNetworkPolicySpec(obj *ciliumv2.CiliumClusterwideNetworkPolicy, spec *api.Rule) {
	intcilium.AddCiliumClusterwideNetworkPolicySpec(obj, spec)
}

// SetCiliumClusterwideNetworkPolicyEndpointSelector sets the endpoint selector
// on the policy, initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyEndpointSelector(obj *ciliumv2.CiliumClusterwideNetworkPolicy, sel api.EndpointSelector) {
	intcilium.SetCiliumClusterwideNetworkPolicyEndpointSelector(obj, sel)
}

// SetCiliumClusterwideNetworkPolicyNodeSelector sets the node selector on the
// policy, initialising Spec if nil. NodeSelector is only valid on
// CiliumClusterwideNetworkPolicy.
func SetCiliumClusterwideNetworkPolicyNodeSelector(obj *ciliumv2.CiliumClusterwideNetworkPolicy, sel api.EndpointSelector) {
	intcilium.SetCiliumClusterwideNetworkPolicyNodeSelector(obj, sel)
}

// AddCiliumClusterwideNetworkPolicyIngressRule appends an ingress allow rule,
// initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyIngressRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.IngressRule) {
	intcilium.AddCiliumClusterwideNetworkPolicyIngressRule(obj, rule)
}

// AddCiliumClusterwideNetworkPolicyIngressDenyRule appends an ingress deny
// rule, initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyIngressDenyRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.IngressDenyRule) {
	intcilium.AddCiliumClusterwideNetworkPolicyIngressDenyRule(obj, rule)
}

// AddCiliumClusterwideNetworkPolicyEgressRule appends an egress allow rule,
// initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyEgressRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.EgressRule) {
	intcilium.AddCiliumClusterwideNetworkPolicyEgressRule(obj, rule)
}

// AddCiliumClusterwideNetworkPolicyEgressDenyRule appends an egress deny rule,
// initialising Spec if nil.
func AddCiliumClusterwideNetworkPolicyEgressDenyRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.EgressDenyRule) {
	intcilium.AddCiliumClusterwideNetworkPolicyEgressDenyRule(obj, rule)
}

// SetCiliumClusterwideNetworkPolicyDescription sets the description on the
// policy, initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyDescription(obj *ciliumv2.CiliumClusterwideNetworkPolicy, desc string) {
	intcilium.SetCiliumClusterwideNetworkPolicyDescription(obj, desc)
}

// SetCiliumClusterwideNetworkPolicyLabels sets the rule labels on the policy,
// initialising Spec if nil. Labels are used by tooling such as Hubble to
// identify and filter policies.
func SetCiliumClusterwideNetworkPolicyLabels(obj *ciliumv2.CiliumClusterwideNetworkPolicy, lbls labels.LabelArray) {
	intcilium.SetCiliumClusterwideNetworkPolicyLabels(obj, lbls)
}

// SetCiliumClusterwideNetworkPolicyEnableDefaultDeny sets the EnableDefaultDeny
// field on the policy, initialising Spec if nil.
func SetCiliumClusterwideNetworkPolicyEnableDefaultDeny(obj *ciliumv2.CiliumClusterwideNetworkPolicy, cfg api.DefaultDenyConfig) {
	intcilium.SetCiliumClusterwideNetworkPolicyEnableDefaultDeny(obj, cfg)
}

// AddCiliumCIDRGroupCIDR appends a CIDR to the group's ExternalCIDRs list.
func AddCiliumCIDRGroupCIDR(obj *ciliumv2.CiliumCIDRGroup, cidr api.CIDR) {
	intcilium.AddCiliumCIDRGroupCIDR(obj, cidr)
}

// SetCiliumCIDRGroupCIDRs replaces the ExternalCIDRs list on the group.
func SetCiliumCIDRGroupCIDRs(obj *ciliumv2.CiliumCIDRGroup, cidrs []api.CIDR) {
	intcilium.SetCiliumCIDRGroupCIDRs(obj, cidrs)
}

// SetCiliumEgressGatewayPolicySpec sets the full spec on the policy.
func SetCiliumEgressGatewayPolicySpec(obj *ciliumv2.CiliumEgressGatewayPolicy, spec ciliumv2.CiliumEgressGatewayPolicySpec) {
	intcilium.SetCiliumEgressGatewayPolicySpec(obj, spec)
}

// AddCiliumEgressGatewayPolicySelectorRule appends an egress selector rule.
func AddCiliumEgressGatewayPolicySelectorRule(obj *ciliumv2.CiliumEgressGatewayPolicy, rule ciliumv2.EgressRule) {
	intcilium.AddCiliumEgressGatewayPolicySelectorRule(obj, rule)
}

// AddCiliumEgressGatewayPolicyDestinationCIDR appends a destination CIDR.
func AddCiliumEgressGatewayPolicyDestinationCIDR(obj *ciliumv2.CiliumEgressGatewayPolicy, cidr ciliumv2.CIDR) {
	intcilium.AddCiliumEgressGatewayPolicyDestinationCIDR(obj, cidr)
}

// AddCiliumEgressGatewayPolicyExcludedCIDR appends an excluded CIDR.
func AddCiliumEgressGatewayPolicyExcludedCIDR(obj *ciliumv2.CiliumEgressGatewayPolicy, cidr ciliumv2.CIDR) {
	intcilium.AddCiliumEgressGatewayPolicyExcludedCIDR(obj, cidr)
}

// SetCiliumEgressGatewayPolicyEgressGateway sets the primary egress gateway.
func SetCiliumEgressGatewayPolicyEgressGateway(obj *ciliumv2.CiliumEgressGatewayPolicy, gw *ciliumv2.EgressGateway) {
	intcilium.SetCiliumEgressGatewayPolicyEgressGateway(obj, gw)
}

// AddCiliumEgressGatewayPolicyEgressGateway appends to the multi-gateway list.
func AddCiliumEgressGatewayPolicyEgressGateway(obj *ciliumv2.CiliumEgressGatewayPolicy, gw ciliumv2.EgressGateway) {
	intcilium.AddCiliumEgressGatewayPolicyEgressGateway(obj, gw)
}

// SetCiliumLocalRedirectPolicySpec sets the full spec on the policy.
func SetCiliumLocalRedirectPolicySpec(obj *ciliumv2.CiliumLocalRedirectPolicy, spec ciliumv2.CiliumLocalRedirectPolicySpec) {
	intcilium.SetCiliumLocalRedirectPolicySpec(obj, spec)
}

// SetCiliumLocalRedirectPolicyFrontend sets the redirect frontend.
func SetCiliumLocalRedirectPolicyFrontend(obj *ciliumv2.CiliumLocalRedirectPolicy, frontend ciliumv2.RedirectFrontend) {
	intcilium.SetCiliumLocalRedirectPolicyFrontend(obj, frontend)
}

// SetCiliumLocalRedirectPolicyBackend sets the redirect backend.
func SetCiliumLocalRedirectPolicyBackend(obj *ciliumv2.CiliumLocalRedirectPolicy, backend ciliumv2.RedirectBackend) {
	intcilium.SetCiliumLocalRedirectPolicyBackend(obj, backend)
}

// SetCiliumLocalRedirectPolicyDescription sets the description.
func SetCiliumLocalRedirectPolicyDescription(obj *ciliumv2.CiliumLocalRedirectPolicy, desc string) {
	intcilium.SetCiliumLocalRedirectPolicyDescription(obj, desc)
}

// SetCiliumLocalRedirectPolicySkipRedirectFromBackend sets the skip-redirect flag.
func SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj *ciliumv2.CiliumLocalRedirectPolicy, skip bool) {
	intcilium.SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj, skip)
}

// SetCiliumLoadBalancerIPPoolSpec sets the full spec on the pool.
func SetCiliumLoadBalancerIPPoolSpec(obj *ciliumv2.CiliumLoadBalancerIPPool, spec ciliumv2.CiliumLoadBalancerIPPoolSpec) {
	intcilium.SetCiliumLoadBalancerIPPoolSpec(obj, spec)
}

// SetCiliumLoadBalancerIPPoolServiceSelector sets the service selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumLoadBalancerIPPoolServiceSelector(obj *ciliumv2.CiliumLoadBalancerIPPool, sel *slimv1.LabelSelector) {
	intcilium.SetCiliumLoadBalancerIPPoolServiceSelector(obj, sel)
}

// AddCiliumLoadBalancerIPPoolBlock appends a CIDR block to the pool.
func AddCiliumLoadBalancerIPPoolBlock(obj *ciliumv2.CiliumLoadBalancerIPPool, block ciliumv2.CiliumLoadBalancerIPPoolIPBlock) {
	intcilium.AddCiliumLoadBalancerIPPoolBlock(obj, block)
}

// SetCiliumLoadBalancerIPPoolDisabled enables or disables the pool.
func SetCiliumLoadBalancerIPPoolDisabled(obj *ciliumv2.CiliumLoadBalancerIPPool, disabled bool) {
	intcilium.SetCiliumLoadBalancerIPPoolDisabled(obj, disabled)
}

// SetCiliumLoadBalancerIPPoolAllowFirstLastIPs controls first/last IP allocation.
func SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj *ciliumv2.CiliumLoadBalancerIPPool, allow ciliumv2.AllowFirstLastIPType) {
	intcilium.SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj, allow)
}

// SetCiliumEnvoyConfigSpec sets the full spec on the config.
func SetCiliumEnvoyConfigSpec(obj *ciliumv2.CiliumEnvoyConfig, spec ciliumv2.CiliumEnvoyConfigSpec) {
	intcilium.SetCiliumEnvoyConfigSpec(obj, spec)
}

// AddCiliumEnvoyConfigService appends a service listener.
func AddCiliumEnvoyConfigService(obj *ciliumv2.CiliumEnvoyConfig, svc *ciliumv2.ServiceListener) {
	intcilium.AddCiliumEnvoyConfigService(obj, svc)
}

// AddCiliumEnvoyConfigBackendService appends a backend service.
func AddCiliumEnvoyConfigBackendService(obj *ciliumv2.CiliumEnvoyConfig, svc *ciliumv2.Service) {
	intcilium.AddCiliumEnvoyConfigBackendService(obj, svc)
}

// AddCiliumEnvoyConfigResource appends an Envoy xDS resource.
func AddCiliumEnvoyConfigResource(obj *ciliumv2.CiliumEnvoyConfig, res ciliumv2.XDSResource) {
	intcilium.AddCiliumEnvoyConfigResource(obj, res)
}

// SetCiliumEnvoyConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumEnvoyConfigNodeSelector(obj *ciliumv2.CiliumEnvoyConfig, sel *slimv1.LabelSelector) {
	intcilium.SetCiliumEnvoyConfigNodeSelector(obj, sel)
}

// SetCiliumClusterwideEnvoyConfigSpec sets the full spec on the config.
func SetCiliumClusterwideEnvoyConfigSpec(obj *ciliumv2.CiliumClusterwideEnvoyConfig, spec ciliumv2.CiliumEnvoyConfigSpec) {
	intcilium.SetCiliumClusterwideEnvoyConfigSpec(obj, spec)
}

// AddCiliumClusterwideEnvoyConfigService appends a service listener.
func AddCiliumClusterwideEnvoyConfigService(obj *ciliumv2.CiliumClusterwideEnvoyConfig, svc *ciliumv2.ServiceListener) {
	intcilium.AddCiliumClusterwideEnvoyConfigService(obj, svc)
}

// AddCiliumClusterwideEnvoyConfigBackendService appends a backend service.
func AddCiliumClusterwideEnvoyConfigBackendService(obj *ciliumv2.CiliumClusterwideEnvoyConfig, svc *ciliumv2.Service) {
	intcilium.AddCiliumClusterwideEnvoyConfigBackendService(obj, svc)
}

// AddCiliumClusterwideEnvoyConfigResource appends an Envoy xDS resource.
func AddCiliumClusterwideEnvoyConfigResource(obj *ciliumv2.CiliumClusterwideEnvoyConfig, res ciliumv2.XDSResource) {
	intcilium.AddCiliumClusterwideEnvoyConfigResource(obj, res)
}

// SetCiliumClusterwideEnvoyConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumClusterwideEnvoyConfigNodeSelector(obj *ciliumv2.CiliumClusterwideEnvoyConfig, sel *slimv1.LabelSelector) {
	intcilium.SetCiliumClusterwideEnvoyConfigNodeSelector(obj, sel)
}

// SetCiliumBGPClusterConfigSpec sets the full spec on the config.
func SetCiliumBGPClusterConfigSpec(obj *ciliumv2.CiliumBGPClusterConfig, spec ciliumv2.CiliumBGPClusterConfigSpec) {
	intcilium.SetCiliumBGPClusterConfigSpec(obj, spec)
}

// SetCiliumBGPClusterConfigNodeSelector sets the node selector.
// Uses Cilium's slim LabelSelector (github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1).
func SetCiliumBGPClusterConfigNodeSelector(obj *ciliumv2.CiliumBGPClusterConfig, sel *slimv1.LabelSelector) {
	intcilium.SetCiliumBGPClusterConfigNodeSelector(obj, sel)
}

// AddCiliumBGPClusterConfigBGPInstance appends a BGP instance.
func AddCiliumBGPClusterConfigBGPInstance(obj *ciliumv2.CiliumBGPClusterConfig, instance ciliumv2.CiliumBGPInstance) {
	intcilium.AddCiliumBGPClusterConfigBGPInstance(obj, instance)
}

// SetCiliumBGPPeerConfigSpec sets the full spec on the config.
func SetCiliumBGPPeerConfigSpec(obj *ciliumv2.CiliumBGPPeerConfig, spec ciliumv2.CiliumBGPPeerConfigSpec) {
	intcilium.SetCiliumBGPPeerConfigSpec(obj, spec)
}

// SetCiliumBGPPeerConfigTransport sets the transport configuration.
func SetCiliumBGPPeerConfigTransport(obj *ciliumv2.CiliumBGPPeerConfig, transport *ciliumv2.CiliumBGPTransport) {
	intcilium.SetCiliumBGPPeerConfigTransport(obj, transport)
}

// SetCiliumBGPPeerConfigTimers sets the BGP timer configuration.
func SetCiliumBGPPeerConfigTimers(obj *ciliumv2.CiliumBGPPeerConfig, timers *ciliumv2.CiliumBGPTimers) {
	intcilium.SetCiliumBGPPeerConfigTimers(obj, timers)
}

// SetCiliumBGPPeerConfigAuthSecretRef sets the BGP authentication secret name.
func SetCiliumBGPPeerConfigAuthSecretRef(obj *ciliumv2.CiliumBGPPeerConfig, ref string) {
	intcilium.SetCiliumBGPPeerConfigAuthSecretRef(obj, ref)
}

// SetCiliumBGPPeerConfigEBGPMultihop sets the eBGP multihop TTL.
func SetCiliumBGPPeerConfigEBGPMultihop(obj *ciliumv2.CiliumBGPPeerConfig, ttl int32) {
	intcilium.SetCiliumBGPPeerConfigEBGPMultihop(obj, ttl)
}

// SetCiliumBGPPeerConfigGracefulRestart sets the graceful restart configuration.
func SetCiliumBGPPeerConfigGracefulRestart(obj *ciliumv2.CiliumBGPPeerConfig, gr *ciliumv2.CiliumBGPNeighborGracefulRestart) {
	intcilium.SetCiliumBGPPeerConfigGracefulRestart(obj, gr)
}

// AddCiliumBGPPeerConfigFamily appends an address family with advertisements.
func AddCiliumBGPPeerConfigFamily(obj *ciliumv2.CiliumBGPPeerConfig, family ciliumv2.CiliumBGPFamilyWithAdverts) {
	intcilium.AddCiliumBGPPeerConfigFamily(obj, family)
}

// SetCiliumBGPAdvertisementSpec sets the full spec on the advertisement.
func SetCiliumBGPAdvertisementSpec(obj *ciliumv2.CiliumBGPAdvertisement, spec ciliumv2.CiliumBGPAdvertisementSpec) {
	intcilium.SetCiliumBGPAdvertisementSpec(obj, spec)
}

// AddCiliumBGPAdvertisementEntry appends a BGP advertisement entry.
func AddCiliumBGPAdvertisementEntry(obj *ciliumv2.CiliumBGPAdvertisement, advert ciliumv2.BGPAdvertisement) {
	intcilium.AddCiliumBGPAdvertisementEntry(obj, advert)
}

// SetCiliumBGPNodeConfigSpec sets the full spec on the node config.
func SetCiliumBGPNodeConfigSpec(obj *ciliumv2.CiliumBGPNodeConfig, spec ciliumv2.CiliumBGPNodeSpec) {
	intcilium.SetCiliumBGPNodeConfigSpec(obj, spec)
}

// AddCiliumBGPNodeConfigBGPInstance appends a BGP node instance.
func AddCiliumBGPNodeConfigBGPInstance(obj *ciliumv2.CiliumBGPNodeConfig, instance ciliumv2.CiliumBGPNodeInstance) {
	intcilium.AddCiliumBGPNodeConfigBGPInstance(obj, instance)
}

// SetCiliumBGPNodeConfigOverrideSpec sets the full spec on the override.
func SetCiliumBGPNodeConfigOverrideSpec(obj *ciliumv2.CiliumBGPNodeConfigOverride, spec ciliumv2.CiliumBGPNodeConfigOverrideSpec) {
	intcilium.SetCiliumBGPNodeConfigOverrideSpec(obj, spec)
}

// AddCiliumBGPNodeConfigOverrideBGPInstance appends a BGP instance override.
func AddCiliumBGPNodeConfigOverrideBGPInstance(obj *ciliumv2.CiliumBGPNodeConfigOverride, instance ciliumv2.CiliumBGPNodeConfigInstanceOverride) {
	intcilium.AddCiliumBGPNodeConfigOverrideBGPInstance(obj, instance)
}
