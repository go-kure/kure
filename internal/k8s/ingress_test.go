package k8s

import (
	"testing"

	netv1 "k8s.io/api/networking/v1"
)

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
}
