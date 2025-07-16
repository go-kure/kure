package k8s

import (
	corev1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// NetworkPolicy helpers

// CreateNetworkPolicy returns a basic NetworkPolicy object with default labels
// and empty rule slices.
func CreateNetworkPolicy(name, namespace string) *netv1.NetworkPolicy {
	obj := &netv1.NetworkPolicy{
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
	return obj
}

func SetNetworkPolicyPodSelector(np *netv1.NetworkPolicy, selector metav1.LabelSelector) {
	np.Spec.PodSelector = selector
}

func AddNetworkPolicyPolicyType(np *netv1.NetworkPolicy, t netv1.PolicyType) {
	np.Spec.PolicyTypes = append(np.Spec.PolicyTypes, t)
}

func SetNetworkPolicyPolicyTypes(np *netv1.NetworkPolicy, types []netv1.PolicyType) {
	np.Spec.PolicyTypes = types
}

func AddNetworkPolicyIngressRule(np *netv1.NetworkPolicy, rule netv1.NetworkPolicyIngressRule) {
	np.Spec.Ingress = append(np.Spec.Ingress, rule)
}

func SetNetworkPolicyIngressRules(np *netv1.NetworkPolicy, rules []netv1.NetworkPolicyIngressRule) {
	np.Spec.Ingress = rules
}

func AddNetworkPolicyEgressRule(np *netv1.NetworkPolicy, rule netv1.NetworkPolicyEgressRule) {
	np.Spec.Egress = append(np.Spec.Egress, rule)
}

func SetNetworkPolicyEgressRules(np *netv1.NetworkPolicy, rules []netv1.NetworkPolicyEgressRule) {
	np.Spec.Egress = rules
}

func AddNetworkPolicyIngressPeer(rule *netv1.NetworkPolicyIngressRule, peer netv1.NetworkPolicyPeer) {
	rule.From = append(rule.From, peer)
}

func SetNetworkPolicyIngressPeers(rule *netv1.NetworkPolicyIngressRule, peers []netv1.NetworkPolicyPeer) {
	rule.From = peers
}

func AddNetworkPolicyIngressPort(rule *netv1.NetworkPolicyIngressRule, port netv1.NetworkPolicyPort) {
	rule.Ports = append(rule.Ports, port)
}

func SetNetworkPolicyIngressPorts(rule *netv1.NetworkPolicyIngressRule, ports []netv1.NetworkPolicyPort) {
	rule.Ports = ports
}

func AddNetworkPolicyEgressPeer(rule *netv1.NetworkPolicyEgressRule, peer netv1.NetworkPolicyPeer) {
	rule.To = append(rule.To, peer)
}

func SetNetworkPolicyEgressPeers(rule *netv1.NetworkPolicyEgressRule, peers []netv1.NetworkPolicyPeer) {
	rule.To = peers
}

func AddNetworkPolicyEgressPort(rule *netv1.NetworkPolicyEgressRule, port netv1.NetworkPolicyPort) {
	rule.Ports = append(rule.Ports, port)
}

func SetNetworkPolicyEgressPorts(rule *netv1.NetworkPolicyEgressRule, ports []netv1.NetworkPolicyPort) {
	rule.Ports = ports
}

// ResourceQuota helpers

// CreateResourceQuota creates a new ResourceQuota object with default metadata.
func CreateResourceQuota(name, namespace string) *corev1.ResourceQuota {
	obj := &corev1.ResourceQuota{
		TypeMeta: metav1.TypeMeta{
			Kind:       "ResourceQuota",
			APIVersion: corev1.SchemeGroupVersion.String(),
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
		Spec: corev1.ResourceQuotaSpec{
			Hard:   corev1.ResourceList{},
			Scopes: []corev1.ResourceQuotaScope{},
		},
	}
	return obj
}

func AddResourceQuotaScope(rq *corev1.ResourceQuota, scope corev1.ResourceQuotaScope) {
	rq.Spec.Scopes = append(rq.Spec.Scopes, scope)
}

func SetResourceQuotaScopes(rq *corev1.ResourceQuota, scopes []corev1.ResourceQuotaScope) {
	rq.Spec.Scopes = scopes
}

func SetResourceQuotaHard(rq *corev1.ResourceQuota, hard corev1.ResourceList) {
	rq.Spec.Hard = hard
}

func AddResourceQuotaHard(rq *corev1.ResourceQuota, name corev1.ResourceName, qty resource.Quantity) {
	if rq.Spec.Hard == nil {
		rq.Spec.Hard = make(corev1.ResourceList)
	}
	rq.Spec.Hard[name] = qty
}

func SetResourceQuotaScopeSelector(rq *corev1.ResourceQuota, selector *corev1.ScopeSelector) {
	rq.Spec.ScopeSelector = selector
}

func AddScopeSelectorExpression(selector *corev1.ScopeSelector, req corev1.ScopedResourceSelectorRequirement) {
	if selector.MatchExpressions == nil {
		selector.MatchExpressions = []corev1.ScopedResourceSelectorRequirement{}
	}
	selector.MatchExpressions = append(selector.MatchExpressions, req)
}

func SetScopeSelectorExpressions(selector *corev1.ScopeSelector, reqs []corev1.ScopedResourceSelectorRequirement) {
	selector.MatchExpressions = reqs
}

// LimitRange helpers

// CreateLimitRange returns a basic LimitRange object with empty limits list.
func CreateLimitRange(name, namespace string) *corev1.LimitRange {
	obj := &corev1.LimitRange{
		TypeMeta: metav1.TypeMeta{
			Kind:       "LimitRange",
			APIVersion: corev1.SchemeGroupVersion.String(),
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
		Spec: corev1.LimitRangeSpec{
			Limits: []corev1.LimitRangeItem{},
		},
	}
	return obj
}

func AddLimitRangeItem(lr *corev1.LimitRange, item corev1.LimitRangeItem) {
	lr.Spec.Limits = append(lr.Spec.Limits, item)
}

func SetLimitRangeItems(lr *corev1.LimitRange, items []corev1.LimitRangeItem) {
	lr.Spec.Limits = items
}

func AddLimitRangeItemMax(item *corev1.LimitRangeItem, name corev1.ResourceName, qty resource.Quantity) {
	if item.Max == nil {
		item.Max = make(corev1.ResourceList)
	}
	item.Max[name] = qty
}

func AddLimitRangeItemMin(item *corev1.LimitRangeItem, name corev1.ResourceName, qty resource.Quantity) {
	if item.Min == nil {
		item.Min = make(corev1.ResourceList)
	}
	item.Min[name] = qty
}

func AddLimitRangeItemDefault(item *corev1.LimitRangeItem, name corev1.ResourceName, qty resource.Quantity) {
	if item.Default == nil {
		item.Default = make(corev1.ResourceList)
	}
	item.Default[name] = qty
}

func AddLimitRangeItemDefaultRequest(item *corev1.LimitRangeItem, name corev1.ResourceName, qty resource.Quantity) {
	if item.DefaultRequest == nil {
		item.DefaultRequest = make(corev1.ResourceList)
	}
	item.DefaultRequest[name] = qty
}

func AddLimitRangeItemMaxLimitRequestRatio(item *corev1.LimitRangeItem, name corev1.ResourceName, qty resource.Quantity) {
	if item.MaxLimitRequestRatio == nil {
		item.MaxLimitRequestRatio = make(corev1.ResourceList)
	}
	item.MaxLimitRequestRatio[name] = qty
}

func SetLimitRangeItemMax(item *corev1.LimitRangeItem, list corev1.ResourceList) {
	item.Max = list
}

func SetLimitRangeItemMin(item *corev1.LimitRangeItem, list corev1.ResourceList) {
	item.Min = list
}

func SetLimitRangeItemDefault(item *corev1.LimitRangeItem, list corev1.ResourceList) {
	item.Default = list
}

func SetLimitRangeItemDefaultRequest(item *corev1.LimitRangeItem, list corev1.ResourceList) {
	item.DefaultRequest = list
}

func SetLimitRangeItemMaxLimitRequestRatio(item *corev1.LimitRangeItem, list corev1.ResourceList) {
	item.MaxLimitRequestRatio = list
}
