package kubernetes

import (
	netv1 "k8s.io/api/networking/v1"
)

// AddNetworkPolicyPolicyType appends a policy type to the NetworkPolicy.
func AddNetworkPolicyPolicyType(np *netv1.NetworkPolicy, t netv1.PolicyType) {
	if np == nil {
		panic("AddNetworkPolicyPolicyType: np must not be nil")
	}
	np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, t)
}

// AddNetworkPolicyIngressRule appends an ingress rule to the NetworkPolicy.
func AddNetworkPolicyIngressRule(np *netv1.NetworkPolicy, rule netv1.NetworkPolicyIngressRule) {
	if np == nil {
		panic("AddNetworkPolicyIngressRule: np must not be nil")
	}
	np.Spec.Ingress = append(np.Spec.Ingress, rule)
}

// AddNetworkPolicyEgressRule appends an egress rule to the NetworkPolicy.
func AddNetworkPolicyEgressRule(np *netv1.NetworkPolicy, rule netv1.NetworkPolicyEgressRule) {
	if np == nil {
		panic("AddNetworkPolicyEgressRule: np must not be nil")
	}
	np.Spec.Egress = append(np.Spec.Egress, rule)
}

// AddNetworkPolicyIngressPeer appends a peer to an ingress rule's From list.
func AddNetworkPolicyIngressPeer(rule *netv1.NetworkPolicyIngressRule, peer netv1.NetworkPolicyPeer) {
	rule.From = append(rule.From, peer)
}

// AddNetworkPolicyIngressPort appends a port to an ingress rule.
func AddNetworkPolicyIngressPort(rule *netv1.NetworkPolicyIngressRule, port netv1.NetworkPolicyPort) {
	rule.Ports = append(rule.Ports, port)
}

// AddNetworkPolicyEgressPeer appends a peer to an egress rule's To list.
func AddNetworkPolicyEgressPeer(rule *netv1.NetworkPolicyEgressRule, peer netv1.NetworkPolicyPeer) {
	rule.To = append(rule.To, peer)
}

// AddNetworkPolicyEgressPort appends a port to an egress rule.
func AddNetworkPolicyEgressPort(rule *netv1.NetworkPolicyEgressRule, port netv1.NetworkPolicyPort) {
	rule.Ports = append(rule.Ports, port)
}
