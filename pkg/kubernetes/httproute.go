package kubernetes

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// CreateHTTPRoute returns an HTTPRoute with default labels, annotations,
// and empty rule and hostname slices.
func CreateHTTPRoute(name, namespace string) *gwapiv1.HTTPRoute {
	return &gwapiv1.HTTPRoute{
		TypeMeta: metav1.TypeMeta{
			Kind:       "HTTPRoute",
			APIVersion: gwapiv1.GroupVersion.String(),
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
		Spec: gwapiv1.HTTPRouteSpec{
			Hostnames: []gwapiv1.Hostname{},
			Rules:     []gwapiv1.HTTPRouteRule{},
		},
	}
}

// AddHTTPRouteHostname appends a hostname to the HTTPRoute.
func AddHTTPRouteHostname(route *gwapiv1.HTTPRoute, hostname gwapiv1.Hostname) error {
	if route == nil {
		return errors.ErrNilHTTPRoute
	}
	route.Spec.Hostnames = append(route.Spec.Hostnames, hostname)
	return nil
}

// SetHTTPRouteHostnames replaces all hostnames on the HTTPRoute.
func SetHTTPRouteHostnames(route *gwapiv1.HTTPRoute, hostnames []gwapiv1.Hostname) error {
	if route == nil {
		return errors.ErrNilHTTPRoute
	}
	route.Spec.Hostnames = hostnames
	return nil
}

// AddHTTPRouteParentRef appends a parent reference (typically a Gateway) to the HTTPRoute.
func AddHTTPRouteParentRef(route *gwapiv1.HTTPRoute, ref gwapiv1.ParentReference) error {
	if route == nil {
		return errors.ErrNilHTTPRoute
	}
	route.Spec.ParentRefs = append(route.Spec.ParentRefs, ref)
	return nil
}

// SetHTTPRouteParentRefs replaces the parent references on the HTTPRoute.
func SetHTTPRouteParentRefs(route *gwapiv1.HTTPRoute, refs []gwapiv1.ParentReference) error {
	if route == nil {
		return errors.ErrNilHTTPRoute
	}
	route.Spec.ParentRefs = refs
	return nil
}

// AddHTTPRouteRule appends a routing rule to the HTTPRoute.
func AddHTTPRouteRule(route *gwapiv1.HTTPRoute, rule gwapiv1.HTTPRouteRule) error {
	if route == nil {
		return errors.ErrNilHTTPRoute
	}
	route.Spec.Rules = append(route.Spec.Rules, rule)
	return nil
}

// SetHTTPRouteRules replaces the routing rules on the HTTPRoute.
func SetHTTPRouteRules(route *gwapiv1.HTTPRoute, rules []gwapiv1.HTTPRouteRule) error {
	if route == nil {
		return errors.ErrNilHTTPRoute
	}
	route.Spec.Rules = rules
	return nil
}

// AddHTTPRouteRuleMatch appends a match condition to an HTTPRouteRule.
func AddHTTPRouteRuleMatch(rule *gwapiv1.HTTPRouteRule, match gwapiv1.HTTPRouteMatch) {
	rule.Matches = append(rule.Matches, match)
}

// SetHTTPRouteRuleMatches replaces the match conditions on an HTTPRouteRule.
func SetHTTPRouteRuleMatches(rule *gwapiv1.HTTPRouteRule, matches []gwapiv1.HTTPRouteMatch) {
	rule.Matches = matches
}

// AddHTTPRouteRuleFilter appends a filter to an HTTPRouteRule.
func AddHTTPRouteRuleFilter(rule *gwapiv1.HTTPRouteRule, filter gwapiv1.HTTPRouteFilter) {
	rule.Filters = append(rule.Filters, filter)
}

// SetHTTPRouteRuleFilters replaces the filters on an HTTPRouteRule.
func SetHTTPRouteRuleFilters(rule *gwapiv1.HTTPRouteRule, filters []gwapiv1.HTTPRouteFilter) {
	rule.Filters = filters
}

// AddHTTPRouteRuleBackendRef appends a backend reference to an HTTPRouteRule.
func AddHTTPRouteRuleBackendRef(rule *gwapiv1.HTTPRouteRule, ref gwapiv1.HTTPBackendRef) {
	rule.BackendRefs = append(rule.BackendRefs, ref)
}

// SetHTTPRouteRuleBackendRefs replaces the backend references on an HTTPRouteRule.
func SetHTTPRouteRuleBackendRefs(rule *gwapiv1.HTTPRouteRule, refs []gwapiv1.HTTPBackendRef) {
	rule.BackendRefs = refs
}
