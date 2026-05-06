package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumClusterwideNetworkPolicy returns a new
// CiliumClusterwideNetworkPolicy with TypeMeta and ObjectMeta set.
// Spec and Specs are left nil; use the setters to populate them.
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

// SetCiliumClusterwideNetworkPolicySpec sets the single-rule spec on the policy.
func SetCiliumClusterwideNetworkPolicySpec(obj *ciliumv2.CiliumClusterwideNetworkPolicy, spec *api.Rule) {
	obj.Spec = spec
}

// SetCiliumClusterwideNetworkPolicySpecs replaces the multi-rule specs on the policy.
func SetCiliumClusterwideNetworkPolicySpecs(obj *ciliumv2.CiliumClusterwideNetworkPolicy, specs api.Rules) {
	obj.Specs = specs
}

// AddCiliumClusterwideNetworkPolicySpec appends a rule to the multi-rule Specs list.
func AddCiliumClusterwideNetworkPolicySpec(obj *ciliumv2.CiliumClusterwideNetworkPolicy, spec *api.Rule) {
	obj.Specs = append(obj.Specs, spec)
}

// SetCiliumClusterwideNetworkPolicyEndpointSelector sets the endpoint selector
// on obj.Spec, initialising the spec if it is nil.
func SetCiliumClusterwideNetworkPolicyEndpointSelector(obj *ciliumv2.CiliumClusterwideNetworkPolicy, sel api.EndpointSelector) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EndpointSelector = sel
}

// SetCiliumClusterwideNetworkPolicyNodeSelector sets the node selector on
// obj.Spec, initialising the spec if it is nil.
// NodeSelector is only valid on CiliumClusterwideNetworkPolicy.
func SetCiliumClusterwideNetworkPolicyNodeSelector(obj *ciliumv2.CiliumClusterwideNetworkPolicy, sel api.EndpointSelector) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.NodeSelector = sel
}

// AddCiliumClusterwideNetworkPolicyIngressRule appends an ingress allow rule to obj.Spec.
func AddCiliumClusterwideNetworkPolicyIngressRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.IngressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Ingress = append(obj.Spec.Ingress, rule)
}

// AddCiliumClusterwideNetworkPolicyIngressDenyRule appends an ingress deny rule to obj.Spec.
func AddCiliumClusterwideNetworkPolicyIngressDenyRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.IngressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.IngressDeny = append(obj.Spec.IngressDeny, rule)
}

// AddCiliumClusterwideNetworkPolicyEgressRule appends an egress allow rule to obj.Spec.
func AddCiliumClusterwideNetworkPolicyEgressRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.EgressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Egress = append(obj.Spec.Egress, rule)
}

// AddCiliumClusterwideNetworkPolicyEgressDenyRule appends an egress deny rule to obj.Spec.
func AddCiliumClusterwideNetworkPolicyEgressDenyRule(obj *ciliumv2.CiliumClusterwideNetworkPolicy, rule api.EgressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EgressDeny = append(obj.Spec.EgressDeny, rule)
}

// SetCiliumClusterwideNetworkPolicyDescription sets the description on obj.Spec.
func SetCiliumClusterwideNetworkPolicyDescription(obj *ciliumv2.CiliumClusterwideNetworkPolicy, desc string) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Description = desc
}

// SetCiliumClusterwideNetworkPolicyLabels sets the rule labels on obj.Spec. Labels
// are used by tooling such as Hubble to identify and filter policies.
func SetCiliumClusterwideNetworkPolicyLabels(obj *ciliumv2.CiliumClusterwideNetworkPolicy, lbls labels.LabelArray) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Labels = lbls
}

// SetCiliumClusterwideNetworkPolicyEnableDefaultDeny sets the EnableDefaultDeny
// field on obj.Spec, initialising the spec if it is nil.
func SetCiliumClusterwideNetworkPolicyEnableDefaultDeny(obj *ciliumv2.CiliumClusterwideNetworkPolicy, cfg api.DefaultDenyConfig) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EnableDefaultDeny = cfg
}
