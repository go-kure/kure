package kubernetes

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
)

func TestCreateIngress(t *testing.T) {
	ing := CreateIngress("my-ing", "default", "nginx")
	if ing.Name != "my-ing" || ing.Namespace != "default" {
		t.Fatalf("metadata mismatch: %s/%s", ing.Namespace, ing.Name)
	}
	if ing.Kind != "Ingress" {
		t.Errorf("unexpected kind %q", ing.Kind)
	}
	if ing.Labels["app"] != "my-ing" {
		t.Errorf("expected label app=my-ing, got %v", ing.Labels)
	}
	if *ing.Spec.IngressClassName != "nginx" {
		t.Errorf("unexpected class name %q", *ing.Spec.IngressClassName)
	}
}

func TestIngressNilErrors(t *testing.T) {
	rule := CreateIngressRule("example.com")
	// All Ingress functions now panic on nil receiver
	assertPanics(t, func() { AddIngressRule(nil, rule) })
	assertPanics(t, func() { AddIngressTLS(nil, netv1.IngressTLS{}) })
	assertPanics(t, func() { SetIngressDefaultBackend(nil, netv1.IngressBackend{}) })
	assertPanics(t, func() { SetIngressClassName(nil, "nginx") })
}

func TestIngressFunctions(t *testing.T) {
	ing := CreateIngress("ing", "ns", "class")
	if ing.Name != "ing" || ing.Namespace != "ns" {
		t.Fatalf("metadata mismatch: %s/%s", ing.Namespace, ing.Name)
	}
	if ing.Kind != "Ingress" {
		t.Errorf("unexpected kind %q", ing.Kind)
	}
	if *ing.Spec.IngressClassName != "class" {
		t.Errorf("unexpected class name %q", *ing.Spec.IngressClassName)
	}

	rule := CreateIngressRule("example.com")
	if rule.Host != "example.com" {
		t.Errorf("rule host mismatch")
	}

	pt := netv1.PathTypePrefix
	path := CreateIngressPath("/", &pt, "svc", "http")
	if path.Path != "/" || path.Backend.Service.Name != "svc" {
		t.Errorf("unexpected path")
	}

	AddIngressRulePath(rule, path)
	if len(rule.IngressRuleValue.HTTP.Paths) != 1 {
		t.Errorf("path not added")
	}

	AddIngressRule(ing, rule)
	if len(ing.Spec.Rules) != 1 {
		t.Errorf("rule not added")
	}

	tls := netv1.IngressTLS{Hosts: []string{"example.com"}}
	AddIngressTLS(ing, tls)
	if len(ing.Spec.TLS) != 1 || ing.Spec.TLS[0].Hosts[0] != "example.com" {
		t.Errorf("tls not added")
	}

	backend := netv1.IngressBackend{Service: &netv1.IngressServiceBackend{Name: "svc", Port: netv1.ServiceBackendPort{Number: 80}}}
	SetIngressDefaultBackend(ing, backend)
	if ing.Spec.DefaultBackend == nil || ing.Spec.DefaultBackend.Service.Name != "svc" {
		t.Errorf("default backend not set")
	}

	SetIngressClassName(ing, "newclass")
	if ing.Spec.IngressClassName == nil || *ing.Spec.IngressClassName != "newclass" {
		t.Errorf("ingress class name not set")
	}
}

func TestAddIngressRulePath_NilHTTP(t *testing.T) {
	rule := &netv1.IngressRule{Host: "example.com"}
	pathType := netv1.PathTypePrefix
	path := netv1.HTTPIngressPath{
		Path:     "/",
		PathType: &pathType,
		Backend: netv1.IngressBackend{
			Service: &netv1.IngressServiceBackend{
				Name: "svc",
				Port: netv1.ServiceBackendPort{Number: 80},
			},
		},
	}
	AddIngressRulePath(rule, path)
	if len(rule.HTTP.Paths) != 1 {
		t.Fatal("expected path to be added")
	}
}
