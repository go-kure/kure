package kubernetes

import (
	"reflect"
	"testing"

	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreateNetworkPolicy(t *testing.T) {
	np := CreateNetworkPolicy("net", "ns")
	if np.Name != "net" || np.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", np.Namespace, np.Name)
	}
	if np.Kind != "NetworkPolicy" {
		t.Errorf("unexpected kind %q", np.Kind)
	}
	if np.Labels["app"] != "net" {
		t.Errorf("expected label app=net, got %v", np.Labels)
	}
	if np.Annotations["app"] != "net" {
		t.Errorf("expected annotation app=net, got %v", np.Annotations)
	}
	if len(np.Spec.PolicyTypes) != 0 {
		t.Errorf("expected empty policy types, got %v", np.Spec.PolicyTypes)
	}
	if len(np.Spec.Ingress) != 0 {
		t.Errorf("expected empty ingress, got %v", np.Spec.Ingress)
	}
	if len(np.Spec.Egress) != 0 {
		t.Errorf("expected empty egress, got %v", np.Spec.Egress)
	}
}

func TestNetworkPolicyNilErrors(t *testing.T) {
	if err := SetNetworkPolicyPodSelector(nil, metav1.LabelSelector{}); err == nil {
		t.Error("expected error for nil NetworkPolicy on SetNetworkPolicyPodSelector")
	}
	if err := AddNetworkPolicyPolicyType(nil, netv1.PolicyTypeIngress); err == nil {
		t.Error("expected error for nil NetworkPolicy on AddNetworkPolicyPolicyType")
	}
	if err := SetNetworkPolicyPolicyTypes(nil, nil); err == nil {
		t.Error("expected error for nil NetworkPolicy on SetNetworkPolicyPolicyTypes")
	}
	if err := AddNetworkPolicyIngressRule(nil, netv1.NetworkPolicyIngressRule{}); err == nil {
		t.Error("expected error for nil NetworkPolicy on AddNetworkPolicyIngressRule")
	}
	if err := SetNetworkPolicyIngressRules(nil, nil); err == nil {
		t.Error("expected error for nil NetworkPolicy on SetNetworkPolicyIngressRules")
	}
	if err := AddNetworkPolicyEgressRule(nil, netv1.NetworkPolicyEgressRule{}); err == nil {
		t.Error("expected error for nil NetworkPolicy on AddNetworkPolicyEgressRule")
	}
	if err := SetNetworkPolicyEgressRules(nil, nil); err == nil {
		t.Error("expected error for nil NetworkPolicy on SetNetworkPolicyEgressRules")
	}
}

func TestNetworkPolicyFunctions(t *testing.T) {
	np := CreateNetworkPolicy("app", "ns")

	sel := metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}}
	if err := SetNetworkPolicyPodSelector(np, sel); err != nil {
		t.Fatalf("SetNetworkPolicyPodSelector returned error: %v", err)
	}
	if !reflect.DeepEqual(np.Spec.PodSelector, sel) {
		t.Errorf("pod selector not set")
	}

	if err := AddNetworkPolicyPolicyType(np, netv1.PolicyTypeIngress); err != nil {
		t.Fatalf("AddNetworkPolicyPolicyType returned error: %v", err)
	}
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != netv1.PolicyTypeIngress {
		t.Errorf("policy type not added")
	}

	types := []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress}
	if err := SetNetworkPolicyPolicyTypes(np, types); err != nil {
		t.Fatalf("SetNetworkPolicyPolicyTypes returned error: %v", err)
	}
	if !reflect.DeepEqual(np.Spec.PolicyTypes, types) {
		t.Errorf("policy types not set")
	}

	rule := netv1.NetworkPolicyIngressRule{}
	peer := netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "demo"}}}
	AddNetworkPolicyIngressPeer(&rule, peer)
	port := netv1.NetworkPolicyPort{}
	AddNetworkPolicyIngressPort(&rule, port)
	if len(rule.From) != 1 || len(rule.Ports) != 1 {
		t.Errorf("rule not populated correctly")
	}

	if err := AddNetworkPolicyIngressRule(np, rule); err != nil {
		t.Fatalf("AddNetworkPolicyIngressRule returned error: %v", err)
	}
	if len(np.Spec.Ingress) != 1 {
		t.Errorf("ingress rule not added")
	}

	ingressRules := []netv1.NetworkPolicyIngressRule{{}, {}}
	if err := SetNetworkPolicyIngressRules(np, ingressRules); err != nil {
		t.Fatalf("SetNetworkPolicyIngressRules returned error: %v", err)
	}
	if len(np.Spec.Ingress) != 2 {
		t.Errorf("ingress rules not set")
	}

	egressRule := netv1.NetworkPolicyEgressRule{}
	if err := AddNetworkPolicyEgressRule(np, egressRule); err != nil {
		t.Fatalf("AddNetworkPolicyEgressRule returned error: %v", err)
	}
	if len(np.Spec.Egress) != 1 {
		t.Errorf("egress rule not added")
	}

	egressRules := []netv1.NetworkPolicyEgressRule{{}, {}}
	if err := SetNetworkPolicyEgressRules(np, egressRules); err != nil {
		t.Fatalf("SetNetworkPolicyEgressRules returned error: %v", err)
	}
	if len(np.Spec.Egress) != 2 {
		t.Errorf("egress rules not set")
	}
}

func TestNetworkPolicyIngressRuleSetters(t *testing.T) {
	rule := netv1.NetworkPolicyIngressRule{}

	peers := []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}}
	SetNetworkPolicyIngressPeers(&rule, peers)
	if len(rule.From) != 1 {
		t.Errorf("ingress peers not set")
	}

	ports := []netv1.NetworkPolicyPort{{}}
	SetNetworkPolicyIngressPorts(&rule, ports)
	if len(rule.Ports) != 1 {
		t.Errorf("ingress ports not set")
	}
}

func TestNetworkPolicyEgressRuleHelpers(t *testing.T) {
	rule := netv1.NetworkPolicyEgressRule{}

	peer := netv1.NetworkPolicyPeer{PodSelector: &metav1.LabelSelector{}}
	AddNetworkPolicyEgressPeer(&rule, peer)
	if len(rule.To) != 1 {
		t.Errorf("egress peer not added")
	}

	peers := []netv1.NetworkPolicyPeer{{PodSelector: &metav1.LabelSelector{}}, {PodSelector: &metav1.LabelSelector{}}}
	SetNetworkPolicyEgressPeers(&rule, peers)
	if len(rule.To) != 2 {
		t.Errorf("egress peers not set")
	}

	port := netv1.NetworkPolicyPort{}
	AddNetworkPolicyEgressPort(&rule, port)
	if len(rule.Ports) != 1 {
		t.Errorf("egress port not added")
	}

	ports := []netv1.NetworkPolicyPort{{}, {}}
	SetNetworkPolicyEgressPorts(&rule, ports)
	if len(rule.Ports) != 2 {
		t.Errorf("egress ports not set")
	}
}
