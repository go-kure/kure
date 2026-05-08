package kubernetes

import (
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CreateNetworkPolicy returns a NetworkPolicy with default labels, annotations,
// and empty rule slices.
func CreateNetworkPolicy(name, namespace string) *netv1.NetworkPolicy {
	return &netv1.NetworkPolicy{
		TypeMeta: metav1.TypeMeta{
			Kind:       "NetworkPolicy",
			APIVersion: netv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
			Labels: map[string]string{
				"app": name,
			},
			Annotations: map[string]string{
				"app": name,
			},
		},
		Spec: netv1.NetworkPolicySpec{
			PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{"app": name}},
			PolicyTypes: []netv1.PolicyType{},
			Ingress:     []netv1.NetworkPolicyIngressRule{},
			Egress:      []netv1.NetworkPolicyEgressRule{},
		},
	}
}

// SetNetworkPolicyPodSelector sets the pod selector on the NetworkPolicy.
func SetNetworkPolicyPodSelector(np *netv1.NetworkPolicy, selector metav1.LabelSelector) {
	if np == nil {
		panic("SetNetworkPolicyPodSelector: np must not be nil")
	}
	np.Spec.PodSelector = selector
}

// AddNetworkPolicyPolicyType appends a policy type to the NetworkPolicy.
func AddNetworkPolicyPolicyType(np *netv1.NetworkPolicy, t netv1.PolicyType) {
	if np == nil {
		panic("AddNetworkPolicyPolicyType: np must not be nil")
	}
	np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, t)
}

// SetNetworkPolicyPolicyTypes replaces the policy types on the NetworkPolicy.
func SetNetworkPolicyPolicyTypes(np *netv1.NetworkPolicy, types []netv1.PolicyType) {
	if np == nil {
		panic("SetNetworkPolicyPolicyTypes: np must not be nil")
	}
	np.Spec.PolicyTypes = types
}

// AddNetworkPolicyIngressRule appends an ingress rule to the NetworkPolicy.
func AddNetworkPolicyIngressRule(np *netv1.NetworkPolicy, rule netv1.NetworkPolicyIngressRule) {
	if np == nil {
		panic("AddNetworkPolicyIngressRule: np must not be nil")
	}
	np.Spec.Ingress = append(np.Spec.Ingress, rule)
}

// SetNetworkPolicyIngressRules replaces the ingress rules on the NetworkPolicy.
func SetNetworkPolicyIngressRules(np *netv1.NetworkPolicy, rules []netv1.NetworkPolicyIngressRule) {
	if np == nil {
		panic("SetNetworkPolicyIngressRules: np must not be nil")
	}
	np.Spec.Ingress = rules
}

// AddNetworkPolicyEgressRule appends an egress rule to the NetworkPolicy.
func AddNetworkPolicyEgressRule(np *netv1.NetworkPolicy, rule netv1.NetworkPolicyEgressRule) {
	if np == nil {
		panic("AddNetworkPolicyEgressRule: np must not be nil")
	}
	np.Spec.Egress = append(np.Spec.Egress, rule)
}

// SetNetworkPolicyEgressRules replaces the egress rules on the NetworkPolicy.
func SetNetworkPolicyEgressRules(np *netv1.NetworkPolicy, rules []netv1.NetworkPolicyEgressRule) {
	if np == nil {
		panic("SetNetworkPolicyEgressRules: np must not be nil")
	}
	np.Spec.Egress = rules
}

// AddNetworkPolicyIngressPeer appends a peer to an ingress rule's From list.
func AddNetworkPolicyIngressPeer(rule *netv1.NetworkPolicyIngressRule, peer netv1.NetworkPolicyPeer) {
	rule.From = append(rule.From, peer)
}

// SetNetworkPolicyIngressPeers replaces the peers on an ingress rule.
func SetNetworkPolicyIngressPeers(rule *netv1.NetworkPolicyIngressRule, peers []netv1.NetworkPolicyPeer) {
	rule.From = peers
}

// AddNetworkPolicyIngressPort appends a port to an ingress rule.
func AddNetworkPolicyIngressPort(rule *netv1.NetworkPolicyIngressRule, port netv1.NetworkPolicyPort) {
	rule.Ports = append(rule.Ports, port)
}

// SetNetworkPolicyIngressPorts replaces the ports on an ingress rule.
func SetNetworkPolicyIngressPorts(rule *netv1.NetworkPolicyIngressRule, ports []netv1.NetworkPolicyPort) {
	rule.Ports = ports
}

// AddNetworkPolicyEgressPeer appends a peer to an egress rule's To list.
func AddNetworkPolicyEgressPeer(rule *netv1.NetworkPolicyEgressRule, peer netv1.NetworkPolicyPeer) {
	rule.To = append(rule.To, peer)
}

// SetNetworkPolicyEgressPeers replaces the peers on an egress rule.
func SetNetworkPolicyEgressPeers(rule *netv1.NetworkPolicyEgressRule, peers []netv1.NetworkPolicyPeer) {
	rule.To = peers
}

// AddNetworkPolicyEgressPort appends a port to an egress rule.
func AddNetworkPolicyEgressPort(rule *netv1.NetworkPolicyEgressRule, port netv1.NetworkPolicyPort) {
	rule.Ports = append(rule.Ports, port)
}

// SetNetworkPolicyEgressPorts replaces the ports on an egress rule.
func SetNetworkPolicyEgressPorts(rule *netv1.NetworkPolicyEgressRule, ports []netv1.NetworkPolicyPort) {
	rule.Ports = ports
}
