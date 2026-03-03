package kubernetes

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestNetworkPolicyFunctions(t *testing.T) {
	np := CreateNetworkPolicy("net", "ns")
	if np.Name != "net" || np.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", np.Namespace, np.Name)
	}
	if np.Kind != "NetworkPolicy" {
		t.Errorf("unexpected kind %q", np.Kind)
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
	AddNetworkPolicyPolicyType(np, netv1.PolicyTypeIngress)
	if len(np.Spec.PolicyTypes) != 1 || np.Spec.PolicyTypes[0] != netv1.PolicyTypeIngress {
		t.Errorf("policy type not added")
	}
}

func TestResourceQuotaFunctions(t *testing.T) {
	rq := CreateResourceQuota("quota", "ns")
	if rq.Kind != "ResourceQuota" {
		t.Errorf("unexpected kind %q", rq.Kind)
	}

	AddResourceQuotaScope(rq, corev1.ResourceQuotaScopeBestEffort)
	if len(rq.Spec.Scopes) != 1 || rq.Spec.Scopes[0] != corev1.ResourceQuotaScopeBestEffort {
		t.Errorf("scope not added")
	}

	qty := resource.MustParse("1Gi")
	AddResourceQuotaHard(rq, corev1.ResourceRequestsStorage, qty)
	if rq.Spec.Hard[corev1.ResourceRequestsStorage] != qty {
		t.Errorf("hard resource not set")
	}

	sel := &corev1.ScopeSelector{}
	expr := corev1.ScopedResourceSelectorRequirement{ScopeName: corev1.ResourceQuotaScopePriorityClass, Operator: corev1.ScopeSelectorOpExists}
	AddScopeSelectorExpression(sel, expr)
	SetResourceQuotaScopeSelector(rq, sel)
	if rq.Spec.ScopeSelector == nil || !reflect.DeepEqual(rq.Spec.ScopeSelector.MatchExpressions[0], expr) {
		t.Errorf("scope selector not set")
	}
}

func TestLimitRangeFunctions(t *testing.T) {
	lr := CreateLimitRange("limits", "ns")
	if lr.Kind != "LimitRange" {
		t.Errorf("unexpected kind %q", lr.Kind)
	}

	item := corev1.LimitRangeItem{Type: corev1.LimitTypeContainer}
	qty := resource.MustParse("100m")
	AddLimitRangeItemMax(&item, corev1.ResourceCPU, qty)
	if item.Max[corev1.ResourceCPU] != qty {
		t.Errorf("max not set")
	}

	AddLimitRangeItem(lr, item)
	if len(lr.Spec.Limits) != 1 {
		t.Errorf("item not added")
	}
}

func TestNetworkPolicySetters(t *testing.T) {
	np := CreateNetworkPolicy("net", "ns")

	sel := metav1.LabelSelector{MatchLabels: map[string]string{"tier": "frontend"}}
	SetNetworkPolicyPodSelector(np, sel)
	if !reflect.DeepEqual(np.Spec.PodSelector, sel) {
		t.Errorf("pod selector not set")
	}

	types := []netv1.PolicyType{netv1.PolicyTypeIngress, netv1.PolicyTypeEgress}
	SetNetworkPolicyPolicyTypes(np, types)
	if !reflect.DeepEqual(np.Spec.PolicyTypes, types) {
		t.Errorf("policy types not set")
	}

	ingressRules := []netv1.NetworkPolicyIngressRule{{}}
	SetNetworkPolicyIngressRules(np, ingressRules)
	if len(np.Spec.Ingress) != 1 {
		t.Errorf("ingress rules not set")
	}

	egressRule := netv1.NetworkPolicyEgressRule{}
	AddNetworkPolicyEgressRule(np, egressRule)
	if len(np.Spec.Egress) != 1 {
		t.Errorf("egress rule not added")
	}

	egressRules := []netv1.NetworkPolicyEgressRule{{}, {}}
	SetNetworkPolicyEgressRules(np, egressRules)
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

func TestResourceQuotaSetters(t *testing.T) {
	rq := CreateResourceQuota("quota", "ns")

	scopes := []corev1.ResourceQuotaScope{corev1.ResourceQuotaScopeBestEffort}
	SetResourceQuotaScopes(rq, scopes)
	if !reflect.DeepEqual(rq.Spec.Scopes, scopes) {
		t.Errorf("scopes not set")
	}

	hard := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("4")}
	SetResourceQuotaHard(rq, hard)
	if !reflect.DeepEqual(rq.Spec.Hard, hard) {
		t.Errorf("hard resources not set")
	}

	sel := &corev1.ScopeSelector{}
	reqs := []corev1.ScopedResourceSelectorRequirement{
		{ScopeName: corev1.ResourceQuotaScopePriorityClass, Operator: corev1.ScopeSelectorOpExists},
	}
	SetScopeSelectorExpressions(sel, reqs)
	if !reflect.DeepEqual(sel.MatchExpressions, reqs) {
		t.Errorf("scope selector expressions not set")
	}
}

func TestLimitRangeSettersAndAdders(t *testing.T) {
	lr := CreateLimitRange("lr", "ns")

	items := []corev1.LimitRangeItem{{Type: corev1.LimitTypeContainer}}
	SetLimitRangeItems(lr, items)
	if len(lr.Spec.Limits) != 1 {
		t.Errorf("limit range items not set")
	}

	item := corev1.LimitRangeItem{Type: corev1.LimitTypeContainer}
	qty := resource.MustParse("256Mi")

	AddLimitRangeItemMin(&item, corev1.ResourceMemory, qty)
	if item.Min[corev1.ResourceMemory] != qty {
		t.Errorf("min not set")
	}

	AddLimitRangeItemDefault(&item, corev1.ResourceMemory, qty)
	if item.Default[corev1.ResourceMemory] != qty {
		t.Errorf("default not set")
	}

	AddLimitRangeItemDefaultRequest(&item, corev1.ResourceMemory, qty)
	if item.DefaultRequest[corev1.ResourceMemory] != qty {
		t.Errorf("default request not set")
	}

	AddLimitRangeItemMaxLimitRequestRatio(&item, corev1.ResourceMemory, qty)
	if item.MaxLimitRequestRatio[corev1.ResourceMemory] != qty {
		t.Errorf("max limit request ratio not set")
	}

	list := corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("1")}

	SetLimitRangeItemMax(&item, list)
	if !reflect.DeepEqual(item.Max, list) {
		t.Errorf("SetLimitRangeItemMax failed")
	}

	SetLimitRangeItemMin(&item, list)
	if !reflect.DeepEqual(item.Min, list) {
		t.Errorf("SetLimitRangeItemMin failed")
	}

	SetLimitRangeItemDefault(&item, list)
	if !reflect.DeepEqual(item.Default, list) {
		t.Errorf("SetLimitRangeItemDefault failed")
	}

	SetLimitRangeItemDefaultRequest(&item, list)
	if !reflect.DeepEqual(item.DefaultRequest, list) {
		t.Errorf("SetLimitRangeItemDefaultRequest failed")
	}

	SetLimitRangeItemMaxLimitRequestRatio(&item, list)
	if !reflect.DeepEqual(item.MaxLimitRequestRatio, list) {
		t.Errorf("SetLimitRangeItemMaxLimitRequestRatio failed")
	}
}
