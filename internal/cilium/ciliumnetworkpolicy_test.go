package cilium

import (
	"testing"

	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
)

func TestCreateCiliumNetworkPolicy(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("test-policy", "default")
	if obj == nil {
		t.Fatal("expected non-nil CiliumNetworkPolicy")
	}
	if obj.Name != "test-policy" {
		t.Errorf("expected Name 'test-policy', got %s", obj.Name)
	}
	if obj.Namespace != "default" {
		t.Errorf("expected Namespace 'default', got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumNetworkPolicy" {
		t.Errorf("expected Kind 'CiliumNetworkPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
	if obj.Spec != nil {
		t.Error("expected nil Spec on creation")
	}
}

func TestSetCiliumNetworkPolicySpec(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	spec := &api.Rule{Description: "test rule"}
	SetCiliumNetworkPolicySpec(obj, spec)
	if obj.Spec == nil {
		t.Fatal("expected non-nil Spec after set")
	}
	if obj.Spec.Description != "test rule" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestAddCiliumNetworkPolicySpec(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	AddCiliumNetworkPolicySpec(obj, &api.Rule{Description: "r1"})
	AddCiliumNetworkPolicySpec(obj, &api.Rule{Description: "r2"})
	if len(obj.Specs) != 2 {
		t.Fatalf("expected 2 specs, got %d", len(obj.Specs))
	}
}

func TestSetCiliumNetworkPolicyEndpointSelector(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	sel := api.NewESFromLabels()
	SetCiliumNetworkPolicyEndpointSelector(obj, sel)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
}

func TestAddCiliumNetworkPolicyIngressRule(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	AddCiliumNetworkPolicyIngressRule(obj, api.IngressRule{})
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if len(obj.Spec.Ingress) != 1 {
		t.Errorf("expected 1 ingress rule, got %d", len(obj.Spec.Ingress))
	}
}

func TestAddCiliumNetworkPolicyIngressDenyRule(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	AddCiliumNetworkPolicyIngressDenyRule(obj, api.IngressDenyRule{})
	if len(obj.Spec.IngressDeny) != 1 {
		t.Errorf("expected 1 ingress deny rule, got %d", len(obj.Spec.IngressDeny))
	}
}

func TestAddCiliumNetworkPolicyEgressRule(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	AddCiliumNetworkPolicyEgressRule(obj, api.EgressRule{})
	if len(obj.Spec.Egress) != 1 {
		t.Errorf("expected 1 egress rule, got %d", len(obj.Spec.Egress))
	}
}

func TestAddCiliumNetworkPolicyEgressDenyRule(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	AddCiliumNetworkPolicyEgressDenyRule(obj, api.EgressDenyRule{})
	if len(obj.Spec.EgressDeny) != 1 {
		t.Errorf("expected 1 egress deny rule, got %d", len(obj.Spec.EgressDeny))
	}
}

func TestSetCiliumNetworkPolicyDescription(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	SetCiliumNetworkPolicyDescription(obj, "allow internal traffic")
	if obj.Spec.Description != "allow internal traffic" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumNetworkPolicyLabels(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	lbls := labels.LabelArray{labels.NewLabel("env", "prod", labels.LabelSourceK8s)}
	SetCiliumNetworkPolicyLabels(obj, lbls)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if len(obj.Spec.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(obj.Spec.Labels))
	}
}

func TestSetCiliumNetworkPolicyEnableDefaultDeny(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("p", "ns")
	ingress := true
	egress := false
	SetCiliumNetworkPolicyEnableDefaultDeny(obj, api.DefaultDenyConfig{Ingress: &ingress, Egress: &egress})
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if obj.Spec.EnableDefaultDeny.Ingress == nil || !*obj.Spec.EnableDefaultDeny.Ingress {
		t.Error("expected EnableDefaultDeny.Ingress to be true")
	}
	if obj.Spec.EnableDefaultDeny.Egress == nil || *obj.Spec.EnableDefaultDeny.Egress {
		t.Error("expected EnableDefaultDeny.Egress to be false")
	}
}
