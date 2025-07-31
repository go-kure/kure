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
