package kubernetes

import (
	"reflect"
	"testing"

	gwapiv1 "sigs.k8s.io/gateway-api/apis/v1"
)

func TestCreateHTTPRoute(t *testing.T) {
	route := CreateHTTPRoute("web", "ns")
	if route.Name != "web" || route.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", route.Namespace, route.Name)
	}
	if route.Kind != "HTTPRoute" {
		t.Errorf("unexpected kind %q", route.Kind)
	}
	if route.Labels["app"] != "web" {
		t.Errorf("expected label app=web, got %v", route.Labels)
	}
	if route.Annotations["app"] != "web" {
		t.Errorf("expected annotation app=web, got %v", route.Annotations)
	}
	if len(route.Spec.Hostnames) != 0 {
		t.Errorf("expected empty hostnames, got %v", route.Spec.Hostnames)
	}
	if len(route.Spec.Rules) != 0 {
		t.Errorf("expected empty rules, got %v", route.Spec.Rules)
	}
}

func TestHTTPRouteNilErrors(t *testing.T) {
	// All HTTPRoute functions now panic on nil receiver
	assertPanics(t, func() { AddHTTPRouteHostname(nil, "example.com") })
	assertPanics(t, func() { SetHTTPRouteHostnames(nil, nil) })
	assertPanics(t, func() { AddHTTPRouteParentRef(nil, gwapiv1.ParentReference{}) })
	assertPanics(t, func() { SetHTTPRouteParentRefs(nil, nil) })
	assertPanics(t, func() { AddHTTPRouteRule(nil, gwapiv1.HTTPRouteRule{}) })
	assertPanics(t, func() { SetHTTPRouteRules(nil, nil) })
}

func TestHTTPRouteFunctions(t *testing.T) {
	route := CreateHTTPRoute("web", "ns")

	AddHTTPRouteHostname(route, "example.com")
	if len(route.Spec.Hostnames) != 1 || route.Spec.Hostnames[0] != "example.com" {
		t.Errorf("hostname not added")
	}

	hostnames := []gwapiv1.Hostname{"a.example.com", "b.example.com"}
	SetHTTPRouteHostnames(route, hostnames)
	if !reflect.DeepEqual(route.Spec.Hostnames, hostnames) {
		t.Errorf("hostnames not set")
	}

	gwName := gwapiv1.ObjectName("my-gw")
	ref := gwapiv1.ParentReference{Name: gwName}
	AddHTTPRouteParentRef(route, ref)
	if len(route.Spec.ParentRefs) != 1 || route.Spec.ParentRefs[0].Name != gwName {
		t.Errorf("parent ref not added")
	}

	refs := []gwapiv1.ParentReference{{Name: "gw-1"}, {Name: "gw-2"}}
	SetHTTPRouteParentRefs(route, refs)
	if len(route.Spec.ParentRefs) != 2 {
		t.Errorf("parent refs not set")
	}

	rule := gwapiv1.HTTPRouteRule{}
	AddHTTPRouteRule(route, rule)
	if len(route.Spec.Rules) != 1 {
		t.Errorf("rule not added")
	}

	rules := []gwapiv1.HTTPRouteRule{{}, {}}
	SetHTTPRouteRules(route, rules)
	if len(route.Spec.Rules) != 2 {
		t.Errorf("rules not set")
	}
}

func TestHTTPRouteRuleHelpers(t *testing.T) {
	t.Run("matches", func(t *testing.T) {
		rule := gwapiv1.HTTPRouteRule{}

		pathType := gwapiv1.PathMatchPathPrefix
		match := gwapiv1.HTTPRouteMatch{
			Path: &gwapiv1.HTTPPathMatch{
				Type:  &pathType,
				Value: ptrStr("/api"),
			},
		}
		AddHTTPRouteRuleMatch(&rule, match)
		if len(rule.Matches) != 1 {
			t.Fatalf("expected 1 match, got %d", len(rule.Matches))
		}
		if *rule.Matches[0].Path.Value != "/api" {
			t.Errorf("match path mismatch")
		}

		matches := []gwapiv1.HTTPRouteMatch{{}, {}}
		SetHTTPRouteRuleMatches(&rule, matches)
		if len(rule.Matches) != 2 {
			t.Errorf("matches not set")
		}
	})

	t.Run("filters", func(t *testing.T) {
		rule := gwapiv1.HTTPRouteRule{}

		filter := gwapiv1.HTTPRouteFilter{
			Type: gwapiv1.HTTPRouteFilterRequestHeaderModifier,
			RequestHeaderModifier: &gwapiv1.HTTPHeaderFilter{
				Set: []gwapiv1.HTTPHeader{{Name: "X-Custom", Value: "val"}},
			},
		}
		AddHTTPRouteRuleFilter(&rule, filter)
		if len(rule.Filters) != 1 {
			t.Fatalf("expected 1 filter, got %d", len(rule.Filters))
		}
		if rule.Filters[0].Type != gwapiv1.HTTPRouteFilterRequestHeaderModifier {
			t.Errorf("filter type mismatch")
		}

		filters := []gwapiv1.HTTPRouteFilter{filter, filter}
		SetHTTPRouteRuleFilters(&rule, filters)
		if len(rule.Filters) != 2 {
			t.Errorf("filters not set")
		}
	})

	t.Run("backend refs", func(t *testing.T) {
		rule := gwapiv1.HTTPRouteRule{}

		weight := int32(100)
		ref := gwapiv1.HTTPBackendRef{
			BackendRef: gwapiv1.BackendRef{
				BackendObjectReference: gwapiv1.BackendObjectReference{
					Name: "my-svc",
					Port: ptrPort(8080),
				},
				Weight: &weight,
			},
		}
		AddHTTPRouteRuleBackendRef(&rule, ref)
		if len(rule.BackendRefs) != 1 {
			t.Fatalf("expected 1 backend ref, got %d", len(rule.BackendRefs))
		}
		if rule.BackendRefs[0].Name != "my-svc" {
			t.Errorf("backend ref name mismatch")
		}

		refs := []gwapiv1.HTTPBackendRef{ref, ref}
		SetHTTPRouteRuleBackendRefs(&rule, refs)
		if len(rule.BackendRefs) != 2 {
			t.Errorf("backend refs not set")
		}
	})
}

func ptrStr(s string) *string           { return &s }
func ptrPort(p int) *gwapiv1.PortNumber { n := gwapiv1.PortNumber(p); return &n }
