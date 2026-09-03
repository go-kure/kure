package kubernetes

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNetworkPolicyNilErrors(t *testing.T) {
	// All NetworkPolicy functions now panic on nil receiver
	assertPanics(t, func() { AddNetworkPolicyPolicyType(nil, netv1.PolicyTypeIngress) })
	assertPanics(t, func() { AddNetworkPolicyIngressRule(nil, netv1.NetworkPolicyIngressRule{}) })
	assertPanics(t, func() { AddNetworkPolicyEgressRule(nil, netv1.NetworkPolicyEgressRule{}) })
}

func TestNetworkPolicyFunctions(t *testing.T) {
	np := CreateNetworkPolicy("app", "ns")

	AddNetworkPolicyPolicyType(np, netv1.PolicyTypeIngress)
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != netv1.PolicyTypeIngress {
		t.Errorf("policy type not added")
	}

	rule := netv1.NetworkPolicyIngressRule{}
	peer := netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	AddNetworkPolicyIngressPeer(&rule, peer)
	port := netv1.NetworkPolicyPort{}
	AddNetworkPolicyIngressPort(&rule, port)
	if len(rule.From) != 1 || len(rule.Ports) != 1 {
		t.Errorf("rule not populated correctly")
	}

	AddNetworkPolicyIngressRule(np, rule)
	if len(np.Spec.Ingress) != 1 {
		t.Errorf("ingress rule not added")
	}

	egressRule := netv1.NetworkPolicyEgressRule{}
	AddNetworkPolicyEgressRule(np, egressRule)
	if len(np.Spec.Egress) != 1 {
		t.Errorf("egress rule not added")
	}
}

func TestNetworkPolicyEgressRuleHelpers(t *testing.T) {
	rule := netv1.NetworkPolicyEgressRule{}

	peer := netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{}}
	AddNetworkPolicyEgressPeer(&rule, peer)
	if len(rule.To) != 1 {
		t.Errorf("egress peer not added")
	}

	port := netv1.NetworkPolicyPort{}
	AddNetworkPolicyEgressPort(&rule, port)
	if len(rule.Ports) != 1 {
		t.Errorf("egress port not added")
	}
}
