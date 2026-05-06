package cilium

import (
	"testing"

	"github.com/cilium/cilium/pkg/labels"
	"github.com/cilium/cilium/pkg/policy/api"
)

func TestCreateCiliumClusterwideNetworkPolicy(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("cluster-policy")
	if obj == nil {
		t.Fatal("expected non-nil CiliumClusterwideNetworkPolicy")
	}
	if obj.Name != "cluster-policy" {
		t.Errorf("expected Name 'cluster-policy', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumClusterwideNetworkPolicy" {
		t.Errorf("expected Kind 'CiliumClusterwideNetworkPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
	if obj.Spec != nil {
		t.Error("expected nil Spec on creation")
	}
}

func TestSetCiliumClusterwideNetworkPolicyNodeSelector(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("p")
	sel := api.NewESFromLabels()
	SetCiliumClusterwideNetworkPolicyNodeSelector(obj, sel)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
}

func TestAddCiliumClusterwideNetworkPolicyIngressRule(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("p")
	AddCiliumClusterwideNetworkPolicyIngressRule(obj, api.IngressRule{})
	AddCiliumClusterwideNetworkPolicyIngressRule(obj, api.IngressRule{})
	if len(obj.Spec.Ingress) != 2 {
		t.Errorf("expected 2 ingress rules, got %d", len(obj.Spec.Ingress))
	}
}

func TestAddCiliumClusterwideNetworkPolicyEgressRule(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("p")
	AddCiliumClusterwideNetworkPolicyEgressRule(obj, api.EgressRule{})
	if len(obj.Spec.Egress) != 1 {
		t.Errorf("expected 1 egress rule, got %d", len(obj.Spec.Egress))
	}
}

func TestAddCiliumClusterwideNetworkPolicyIngressDenyRule(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("p")
	AddCiliumClusterwideNetworkPolicyIngressDenyRule(obj, api.IngressDenyRule{})
	if len(obj.Spec.IngressDeny) != 1 {
		t.Errorf("expected 1 ingress deny rule, got %d", len(obj.Spec.IngressDeny))
	}
}

func TestAddCiliumClusterwideNetworkPolicyEgressDenyRule(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("p")
	AddCiliumClusterwideNetworkPolicyEgressDenyRule(obj, api.EgressDenyRule{})
	if len(obj.Spec.EgressDeny) != 1 {
		t.Errorf("expected 1 egress deny rule, got %d", len(obj.Spec.EgressDeny))
	}
}

func TestSetCiliumClusterwideNetworkPolicyLabels(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("p")
	lbls := labels.LabelArray{labels.NewLabel("env", "prod", labels.LabelSourceK8s)}
	SetCiliumClusterwideNetworkPolicyLabels(obj, lbls)
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if len(obj.Spec.Labels) != 1 {
		t.Fatalf("expected 1 label, got %d", len(obj.Spec.Labels))
	}
}
