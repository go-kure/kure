package cilium

import (
	intcilium "github.com/go-kure/kure/internal/cilium"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
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

// AddCiliumCIDRGroupCIDR appends a CIDR to the group's ExternalCIDRs list.
func AddCiliumCIDRGroupCIDR(obj *ciliumv2.CiliumCIDRGroup, cidr api.CIDR) {
	intcilium.AddCiliumCIDRGroupCIDR(obj, cidr)
}

// SetCiliumCIDRGroupCIDRs replaces the ExternalCIDRs list on the group.
func SetCiliumCIDRGroupCIDRs(obj *ciliumv2.CiliumCIDRGroup, cidrs []api.CIDR) {
	intcilium.SetCiliumCIDRGroupCIDRs(obj, cidrs)
}
