package cilium

import (
	"testing"

	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
)

func TestSetCiliumNetworkPolicySpec(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	SetCiliumNetworkPolicySpec(obj, &api.Rule{Description: "replaced"})
	if obj.Spec == nil {
		t.Fatal("expected non-nil Spec after set")
	}
	if obj.Spec.Description != "replaced" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumNetworkPolicySpecs(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	SetCiliumNetworkPolicySpecs(obj, api.Rules{&api.Rule{Description: "r1"}, &api.Rule{Description: "r2"}})
	if len(obj.Specs) != 2 {
		t.Fatalf("expected 2 Specs, got %d", len(obj.Specs))
	}
}

func TestAddCiliumNetworkPolicySpec(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	AddCiliumNetworkPolicySpec(obj, &api.Rule{Description: "r1"})
	AddCiliumNetworkPolicySpec(obj, &api.Rule{Description: "r2"})
	if len(obj.Specs) != 2 {
		t.Fatalf("expected 2 Specs, got %d", len(obj.Specs))
	}
}

func TestSetCiliumNetworkPolicyEndpointSelector(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	sel := api.NewESFromLabels()
	SetCiliumNetworkPolicyEndpointSelector(obj, sel)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
}

func TestAddCiliumNetworkPolicyIngressRule(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	AddCiliumNetworkPolicyIngressRule(obj, api.IngressRule{})
	if len(obj.Spec.Ingress) != 1 {
		t.Errorf("expected 1 ingress rule, got %d", len(obj.Spec.Ingress))
	}
}

func TestAddCiliumNetworkPolicyIngressDenyRule(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	AddCiliumNetworkPolicyIngressDenyRule(obj, api.IngressDenyRule{})
	if len(obj.Spec.IngressDeny) != 1 {
		t.Errorf("expected 1 ingress deny rule, got %d", len(obj.Spec.IngressDeny))
	}
}

func TestAddCiliumNetworkPolicyEgressRule(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	AddCiliumNetworkPolicyEgressRule(obj, api.EgressRule{})
	if len(obj.Spec.Egress) != 1 {
		t.Errorf("expected 1 egress rule, got %d", len(obj.Spec.Egress))
	}
}

func TestAddCiliumNetworkPolicyEgressDenyRule(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	AddCiliumNetworkPolicyEgressDenyRule(obj, api.EgressDenyRule{})
	if len(obj.Spec.EgressDeny) != 1 {
		t.Errorf("expected 1 egress deny rule, got %d", len(obj.Spec.EgressDeny))
	}
}

func TestSetCiliumNetworkPolicyDescription(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	SetCiliumNetworkPolicyDescription(obj, "allow internal traffic")
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if obj.Spec.Description != "allow internal traffic" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumNetworkPolicyLabels(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
	lbls := labels.LabelArray{labels.NewLabel("env", "prod", labels.LabelSourceK8s)}
	SetCiliumNetworkPolicyLabels(obj, lbls)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if len(obj.Spec.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(obj.Spec.Labels))
	}
}

func TestSetCiliumClusterwideNetworkPolicySpecs(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	SetCiliumClusterwideNetworkPolicySpecs(obj, api.Rules{&api.Rule{Description: "r1"}, &api.Rule{Description: "r2"}})
	if len(obj.Specs) != 2 {
		t.Fatalf("expected 2 Specs, got %d", len(obj.Specs))
	}
}

func TestSetCiliumClusterwideNetworkPolicySpec(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	SetCiliumClusterwideNetworkPolicySpec(obj, &api.Rule{Description: "cluster rule"})
	if obj.Spec == nil {
		t.Fatal("expected non-nil Spec after set")
	}
	if obj.Spec.Description != "cluster rule" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumClusterwideNetworkPolicyEndpointSelector(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	sel := api.NewESFromLabels()
	SetCiliumClusterwideNetworkPolicyEndpointSelector(obj, sel)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
}

func TestSetCiliumClusterwideNetworkPolicyNodeSelector(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	sel := api.NewESFromLabels()
	SetCiliumClusterwideNetworkPolicyNodeSelector(obj, sel)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
}

func TestAddCiliumClusterwideNetworkPolicyIngressRule(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	AddCiliumClusterwideNetworkPolicyIngressRule(obj, api.IngressRule{})
	AddCiliumClusterwideNetworkPolicyIngressRule(obj, api.IngressRule{})
	if len(obj.Spec.Ingress) != 2 {
		t.Errorf("expected 2 ingress rules, got %d", len(obj.Spec.Ingress))
	}
}

func TestAddCiliumClusterwideNetworkPolicyIngressDenyRule(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	AddCiliumClusterwideNetworkPolicyIngressDenyRule(obj, api.IngressDenyRule{})
	if len(obj.Spec.IngressDeny) != 1 {
		t.Errorf("expected 1 ingress deny rule, got %d", len(obj.Spec.IngressDeny))
	}
}

func TestAddCiliumClusterwideNetworkPolicyEgressRule(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	AddCiliumClusterwideNetworkPolicyEgressRule(obj, api.EgressRule{})
	if len(obj.Spec.Egress) != 1 {
		t.Errorf("expected 1 egress rule, got %d", len(obj.Spec.Egress))
	}
}

func TestAddCiliumClusterwideNetworkPolicyEgressDenyRule(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	AddCiliumClusterwideNetworkPolicyEgressDenyRule(obj, api.EgressDenyRule{})
	if len(obj.Spec.EgressDeny) != 1 {
		t.Errorf("expected 1 egress deny rule, got %d", len(obj.Spec.EgressDeny))
	}
}

func TestSetCiliumClusterwideNetworkPolicyDescription(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	SetCiliumClusterwideNetworkPolicyDescription(obj, "cluster wide policy")
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if obj.Spec.Description != "cluster wide policy" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumClusterwideNetworkPolicyLabels(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	lbls := labels.LabelArray{labels.NewLabel("env", "prod", labels.LabelSourceK8s)}
	SetCiliumClusterwideNetworkPolicyLabels(obj, lbls)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if len(obj.Spec.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(obj.Spec.Labels))
	}
}

func TestAddCiliumCIDRGroupCIDR(t *testing.T) {
	obj := CiliumCIDRGroup(&CiliumCIDRGroupConfig{Name: "g"})
	AddCiliumCIDRGroupCIDR(obj, api.CIDR("10.0.0.0/8"))
	AddCiliumCIDRGroupCIDR(obj, api.CIDR("192.168.0.0/16"))
	if len(obj.Spec.ExternalCIDRs) != 2 {
		t.Fatalf("expected 2 CIDRs, got %d", len(obj.Spec.ExternalCIDRs))
	}
	if string(obj.Spec.ExternalCIDRs[0]) != "10.0.0.0/8" {
		t.Errorf("unexpected first CIDR: %s", obj.Spec.ExternalCIDRs[0])
	}
}

func TestSetCiliumCIDRGroupCIDRs(t *testing.T) {
	obj := CiliumCIDRGroup(&CiliumCIDRGroupConfig{
		Name:          "g",
		ExternalCIDRs: []api.CIDR{"10.0.0.0/8"},
	})
	SetCiliumCIDRGroupCIDRs(obj, []api.CIDR{"172.16.0.0/12"})
	if len(obj.Spec.ExternalCIDRs) != 1 {
		t.Fatalf("expected 1 CIDR after replace, got %d", len(obj.Spec.ExternalCIDRs))
	}
	if string(obj.Spec.ExternalCIDRs[0]) != "172.16.0.0/12" {
		t.Errorf("unexpected CIDR: %s", obj.Spec.ExternalCIDRs[0])
	}
}
