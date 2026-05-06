package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumNetworkPolicy returns a new CiliumNetworkPolicy with TypeMeta
// and ObjectMeta set. Spec and Specs are left nil; use the setters to populate them.
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

// SetCiliumNetworkPolicyEndpointSelector sets the endpoint selector on obj.Spec,
// initialising the spec if it is nil.
func SetCiliumNetworkPolicyEndpointSelector(obj *ciliumv2.CiliumNetworkPolicy, sel api.EndpointSelector) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EndpointSelector = sel
}

// AddCiliumNetworkPolicyIngressRule appends an ingress allow rule to obj.Spec.
func AddCiliumNetworkPolicyIngressRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.IngressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Ingress = append(obj.Spec.Ingress, rule)
}

// AddCiliumNetworkPolicyIngressDenyRule appends an ingress deny rule to obj.Spec.
func AddCiliumNetworkPolicyIngressDenyRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.IngressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.IngressDeny = append(obj.Spec.IngressDeny, rule)
}

// AddCiliumNetworkPolicyEgressRule appends an egress allow rule to obj.Spec.
func AddCiliumNetworkPolicyEgressRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.EgressRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Egress = append(obj.Spec.Egress, rule)
}

// AddCiliumNetworkPolicyEgressDenyRule appends an egress deny rule to obj.Spec.
func AddCiliumNetworkPolicyEgressDenyRule(obj *ciliumv2.CiliumNetworkPolicy, rule api.EgressDenyRule) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.EgressDeny = append(obj.Spec.EgressDeny, rule)
}

// SetCiliumNetworkPolicyDescription sets the description on obj.Spec.
func SetCiliumNetworkPolicyDescription(obj *ciliumv2.CiliumNetworkPolicy, desc string) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Description = desc
}

// SetCiliumNetworkPolicyLabels sets the rule labels on obj.Spec. Labels are
// used by tooling such as Hubble to identify and filter policies.
func SetCiliumNetworkPolicyLabels(obj *ciliumv2.CiliumNetworkPolicy, lbls labels.LabelArray) {
	if obj.Spec == nil {
		obj.Spec = &api.Rule{}
	}
	obj.Spec.Labels = lbls
}
