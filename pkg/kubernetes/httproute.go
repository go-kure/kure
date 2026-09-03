package kubernetes

import (
	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

// AddHTTPRouteHostname appends a hostname to the HTTPRoute.
func AddHTTPRouteHostname(route *gwapiv1.HTTPRoute, hostname gwapiv1.Hostname) {
	if route == nil {
		panic("AddHTTPRouteHostname: route must not be nil")
	}
	route.Spec.Hostnames = append(route.Spec.Hostnames, hostname)
}

// AddHTTPRouteParentRef appends a parent reference (typically a Gateway) to the HTTPRoute.
func AddHTTPRouteParentRef(route *gwapiv1.HTTPRoute, ref gwapiv1.ParentReference) {
	if route == nil {
		panic("AddHTTPRouteParentRef: route must not be nil")
	}
	route.Spec.ParentRefs = append(route.Spec.ParentRefs, ref)
}

// AddHTTPRouteRule appends a routing rule to the HTTPRoute.
func AddHTTPRouteRule(route *gwapiv1.HTTPRoute, rule gwapiv1.HTTPRouteRule) {
	if route == nil {
		panic("AddHTTPRouteRule: route must not be nil")
	}
	route.Spec.Rules = append(route.Spec.Rules, rule)
}

// AddHTTPRouteRuleMatch appends a match condition to an HTTPRouteRule.
func AddHTTPRouteRuleMatch(rule *gwapiv1.HTTPRouteRule, match gwapiv1.HTTPRouteMatch) {
	rule.Matches = append(rule.Matches, match)
}

// AddHTTPRouteRuleFilter appends a filter to an HTTPRouteRule.
func AddHTTPRouteRuleFilter(rule *gwapiv1.HTTPRouteRule, filter gwapiv1.HTTPRouteFilter) {
	rule.Filters = append(rule.Filters, filter)
}

// AddHTTPRouteRuleBackendRef appends a backend reference to an HTTPRouteRule.
func AddHTTPRouteRuleBackendRef(rule *gwapiv1.HTTPRouteRule, ref gwapiv1.HTTPBackendRef) {
	rule.BackendRefs = append(rule.BackendRefs, ref)
}
