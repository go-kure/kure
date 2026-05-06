package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
)

func TestCreateCiliumEgressGatewayPolicy(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("my-egress")
	if obj == nil {
		t.Fatal("expected non-nil CiliumEgressGatewayPolicy")
	}
	if obj.Name != "my-egress" {
		t.Errorf("expected Name 'my-egress', got %s", obj.Name)
	}
	if obj.Namespace != "" {
		t.Errorf("expected empty Namespace for cluster-scoped resource, got %s", obj.Namespace)
	}
	if obj.Kind != "CiliumEgressGatewayPolicy" {
		t.Errorf("expected Kind 'CiliumEgressGatewayPolicy', got %s", obj.Kind)
	}
	if obj.APIVersion != "cilium.io/v2" {
		t.Errorf("expected APIVersion 'cilium.io/v2', got %s", obj.APIVersion)
	}
}

func TestSetCiliumEgressGatewayPolicySpec(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("p")
	spec := ciliumv2.CiliumEgressGatewayPolicySpec{
		DestinationCIDRs: []ciliumv2.CIDR{"10.0.0.0/8"},
	}
	SetCiliumEgressGatewayPolicySpec(obj, spec)
	if len(obj.Spec.DestinationCIDRs) != 1 {
		t.Fatalf("expected 1 destination CIDR, got %d", len(obj.Spec.DestinationCIDRs))
	}
}

func TestAddCiliumEgressGatewayPolicySelectorRule(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("p")
	rule := ciliumv2.EgressRule{
		PodSelector: &slimv1.LabelSelector{
			MatchLabels: map[string]string{"app": "frontend"},
		},
	}
	AddCiliumEgressGatewayPolicySelectorRule(obj, rule)
	if len(obj.Spec.Selectors) != 1 {
		t.Fatalf("expected 1 selector rule, got %d", len(obj.Spec.Selectors))
	}
}

func TestAddCiliumEgressGatewayPolicyDestinationCIDR(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("p")
	AddCiliumEgressGatewayPolicyDestinationCIDR(obj, ciliumv2.CIDR("10.0.0.0/8"))
	AddCiliumEgressGatewayPolicyDestinationCIDR(obj, ciliumv2.CIDR("192.168.0.0/16"))
	if len(obj.Spec.DestinationCIDRs) != 2 {
		t.Fatalf("expected 2 destination CIDRs, got %d", len(obj.Spec.DestinationCIDRs))
	}
}

func TestAddCiliumEgressGatewayPolicyExcludedCIDR(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("p")
	AddCiliumEgressGatewayPolicyExcludedCIDR(obj, ciliumv2.CIDR("10.0.1.0/24"))
	if len(obj.Spec.ExcludedCIDRs) != 1 {
		t.Fatalf("expected 1 excluded CIDR, got %d", len(obj.Spec.ExcludedCIDRs))
	}
}

func TestSetCiliumEgressGatewayPolicyEgressGateway(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("p")
	gw := &ciliumv2.EgressGateway{
		NodeSelector: &slimv1.LabelSelector{
			MatchLabels: map[string]string{"role": "egress"},
		},
	}
	SetCiliumEgressGatewayPolicyEgressGateway(obj, gw)
	if obj.Spec.EgressGateway == nil {
		t.Fatal("expected non-nil EgressGateway")
	}
}

func TestAddCiliumEgressGatewayPolicyEgressGateway(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("p")
	gw := ciliumv2.EgressGateway{
		NodeSelector: &slimv1.LabelSelector{
			MatchLabels: map[string]string{"role": "egress"},
		},
	}
	AddCiliumEgressGatewayPolicyEgressGateway(obj, gw)
	AddCiliumEgressGatewayPolicyEgressGateway(obj, gw)
	if len(obj.Spec.EgressGateways) != 2 {
		t.Fatalf("expected 2 egress gateways, got %d", len(obj.Spec.EgressGateways))
	}
}
