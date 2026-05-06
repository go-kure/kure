package cilium

import (
	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateCiliumEgressGatewayPolicy returns a new CiliumEgressGatewayPolicy with
// TypeMeta and ObjectMeta set. The resource is cluster-scoped.
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
