package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
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

func TestSetCiliumNetworkPolicyEnableDefaultDeny(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{Name: "p", Namespace: "ns"})
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

func TestSetCiliumClusterwideNetworkPolicyEnableDefaultDeny(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{Name: "p"})
	ingress := true
	egress := true
	SetCiliumClusterwideNetworkPolicyEnableDefaultDeny(obj, api.DefaultDenyConfig{Ingress: &ingress, Egress: &egress})
	if obj.Spec == nil {
		t.Fatal("expected Spec to be auto-initialised")
	}
	if obj.Spec.EnableDefaultDeny.Ingress == nil || !*obj.Spec.EnableDefaultDeny.Ingress {
		t.Error("expected EnableDefaultDeny.Ingress to be true")
	}
	if obj.Spec.EnableDefaultDeny.Egress == nil || !*obj.Spec.EnableDefaultDeny.Egress {
		t.Error("expected EnableDefaultDeny.Egress to be true")
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

func TestSetCiliumEgressGatewayPolicySpec(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{Name: "p"})
	spec := ciliumv2.CiliumEgressGatewayPolicySpec{
		DestinationCIDRs: []ciliumv2.CIDR{"10.0.0.0/8"},
	}
	SetCiliumEgressGatewayPolicySpec(obj, spec)
	if len(obj.Spec.DestinationCIDRs) != 1 {
		t.Fatalf("expected 1 DestinationCIDR, got %d", len(obj.Spec.DestinationCIDRs))
	}
}

func TestAddCiliumEgressGatewayPolicySelectorRule(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{Name: "p"})
	rule := ciliumv2.EgressRule{}
	AddCiliumEgressGatewayPolicySelectorRule(obj, rule)
	AddCiliumEgressGatewayPolicySelectorRule(obj, rule)
	if len(obj.Spec.Selectors) != 2 {
		t.Fatalf("expected 2 selectors, got %d", len(obj.Spec.Selectors))
	}
}

func TestAddCiliumEgressGatewayPolicyDestinationCIDR(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{Name: "p"})
	AddCiliumEgressGatewayPolicyDestinationCIDR(obj, "10.0.0.0/8")
	AddCiliumEgressGatewayPolicyDestinationCIDR(obj, "192.168.0.0/16")
	if len(obj.Spec.DestinationCIDRs) != 2 {
		t.Fatalf("expected 2 DestinationCIDRs, got %d", len(obj.Spec.DestinationCIDRs))
	}
}

func TestAddCiliumEgressGatewayPolicyExcludedCIDR(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{Name: "p"})
	AddCiliumEgressGatewayPolicyExcludedCIDR(obj, "10.1.0.0/24")
	if len(obj.Spec.ExcludedCIDRs) != 1 {
		t.Fatalf("expected 1 ExcludedCIDR, got %d", len(obj.Spec.ExcludedCIDRs))
	}
}

func TestSetCiliumEgressGatewayPolicyEgressGateway(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{Name: "p"})
	gw := &ciliumv2.EgressGateway{Interface: "eth0"}
	SetCiliumEgressGatewayPolicyEgressGateway(obj, gw)
	if obj.Spec.EgressGateway == nil || obj.Spec.EgressGateway.Interface != "eth0" {
		t.Errorf("unexpected EgressGateway: %v", obj.Spec.EgressGateway)
	}
}

func TestAddCiliumEgressGatewayPolicyEgressGateway(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{Name: "p"})
	AddCiliumEgressGatewayPolicyEgressGateway(obj, ciliumv2.EgressGateway{Interface: "eth0"})
	AddCiliumEgressGatewayPolicyEgressGateway(obj, ciliumv2.EgressGateway{Interface: "eth1"})
	if len(obj.Spec.EgressGateways) != 2 {
		t.Fatalf("expected 2 EgressGateways, got %d", len(obj.Spec.EgressGateways))
	}
}

func TestSetCiliumLocalRedirectPolicySpec(t *testing.T) {
	obj := CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{Name: "p", Namespace: "ns"})
	spec := ciliumv2.CiliumLocalRedirectPolicySpec{Description: "redirect spec"}
	SetCiliumLocalRedirectPolicySpec(obj, spec)
	if obj.Spec.Description != "redirect spec" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumLocalRedirectPolicyFrontend(t *testing.T) {
	obj := CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{Name: "p", Namespace: "ns"})
	frontend := ciliumv2.RedirectFrontend{AddressMatcher: &ciliumv2.Frontend{IP: "1.2.3.4"}}
	SetCiliumLocalRedirectPolicyFrontend(obj, frontend)
	if obj.Spec.RedirectFrontend.AddressMatcher == nil || obj.Spec.RedirectFrontend.AddressMatcher.IP != "1.2.3.4" {
		t.Errorf("unexpected RedirectFrontend: %v", obj.Spec.RedirectFrontend)
	}
}

func TestSetCiliumLocalRedirectPolicyBackend(t *testing.T) {
	obj := CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{Name: "p", Namespace: "ns"})
	backend := ciliumv2.RedirectBackend{LocalEndpointSelector: slimv1.LabelSelector{}}
	SetCiliumLocalRedirectPolicyBackend(obj, backend)
}

func TestSetCiliumLocalRedirectPolicyDescription(t *testing.T) {
	obj := CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{Name: "p", Namespace: "ns"})
	SetCiliumLocalRedirectPolicyDescription(obj, "test redirect")
	if obj.Spec.Description != "test redirect" {
		t.Errorf("unexpected Description: %s", obj.Spec.Description)
	}
}

func TestSetCiliumLocalRedirectPolicySkipRedirectFromBackend(t *testing.T) {
	obj := CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{Name: "p", Namespace: "ns"})
	SetCiliumLocalRedirectPolicySkipRedirectFromBackend(obj, true)
	if !obj.Spec.SkipRedirectFromBackend {
		t.Error("expected SkipRedirectFromBackend to be true")
	}
}

func TestSetCiliumLoadBalancerIPPoolSpec(t *testing.T) {
	obj := CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{Name: "p"})
	spec := ciliumv2.CiliumLoadBalancerIPPoolSpec{
		Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "10.0.0.0/8"}},
	}
	SetCiliumLoadBalancerIPPoolSpec(obj, spec)
	if len(obj.Spec.Blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(obj.Spec.Blocks))
	}
}

func TestSetCiliumLoadBalancerIPPoolServiceSelector(t *testing.T) {
	obj := CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{Name: "p"})
	sel := &slimv1.LabelSelector{MatchLabels: map[string]string{"svc": "lb"}}
	SetCiliumLoadBalancerIPPoolServiceSelector(obj, sel)
	if obj.Spec.ServiceSelector == nil {
		t.Fatal("expected non-nil ServiceSelector")
	}
}

func TestAddCiliumLoadBalancerIPPoolBlock(t *testing.T) {
	obj := CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{Name: "p"})
	AddCiliumLoadBalancerIPPoolBlock(obj, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "10.0.0.0/8"})
	AddCiliumLoadBalancerIPPoolBlock(obj, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "172.16.0.0/12"})
	if len(obj.Spec.Blocks) != 2 {
		t.Fatalf("expected 2 blocks, got %d", len(obj.Spec.Blocks))
	}
}

func TestSetCiliumLoadBalancerIPPoolDisabled(t *testing.T) {
	obj := CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{Name: "p"})
	SetCiliumLoadBalancerIPPoolDisabled(obj, true)
	if !obj.Spec.Disabled {
		t.Error("expected Disabled to be true")
	}
}

func TestSetCiliumLoadBalancerIPPoolAllowFirstLastIPs(t *testing.T) {
	obj := CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{Name: "p"})
	SetCiliumLoadBalancerIPPoolAllowFirstLastIPs(obj, ciliumv2.AllowFirstLastIPYes)
	if obj.Spec.AllowFirstLastIPs != ciliumv2.AllowFirstLastIPYes {
		t.Errorf("unexpected AllowFirstLastIPs: %s", obj.Spec.AllowFirstLastIPs)
	}
}

func TestSetCiliumEnvoyConfigSpec(t *testing.T) {
	obj := CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{Name: "p", Namespace: "ns"})
	spec := ciliumv2.CiliumEnvoyConfigSpec{Resources: []ciliumv2.XDSResource{{}, {}}}
	SetCiliumEnvoyConfigSpec(obj, spec)
	if len(obj.Spec.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(obj.Spec.Resources))
	}
}

func TestAddCiliumEnvoyConfigService(t *testing.T) {
	obj := CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{Name: "p", Namespace: "ns"})
	svc := &ciliumv2.ServiceListener{Name: "svc", Namespace: "ns"}
	AddCiliumEnvoyConfigService(obj, svc)
	AddCiliumEnvoyConfigService(obj, svc)
	if len(obj.Spec.Services) != 2 {
		t.Fatalf("expected 2 services, got %d", len(obj.Spec.Services))
	}
}

func TestAddCiliumEnvoyConfigBackendService(t *testing.T) {
	obj := CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{Name: "p", Namespace: "ns"})
	svc := &ciliumv2.Service{Name: "backend", Namespace: "ns"}
	AddCiliumEnvoyConfigBackendService(obj, svc)
	if len(obj.Spec.BackendServices) != 1 {
		t.Fatalf("expected 1 backend service, got %d", len(obj.Spec.BackendServices))
	}
}

func TestAddCiliumEnvoyConfigResource(t *testing.T) {
	obj := CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{Name: "p", Namespace: "ns"})
	AddCiliumEnvoyConfigResource(obj, ciliumv2.XDSResource{})
	if len(obj.Spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(obj.Spec.Resources))
	}
}

func TestSetCiliumEnvoyConfigNodeSelector(t *testing.T) {
	obj := CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{Name: "p", Namespace: "ns"})
	sel := &slimv1.LabelSelector{MatchLabels: map[string]string{"node": "worker"}}
	SetCiliumEnvoyConfigNodeSelector(obj, sel)
	if obj.Spec.NodeSelector == nil {
		t.Fatal("expected non-nil NodeSelector")
	}
}

func TestSetCiliumClusterwideEnvoyConfigSpec(t *testing.T) {
	obj := CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{Name: "p"})
	spec := ciliumv2.CiliumEnvoyConfigSpec{Resources: []ciliumv2.XDSResource{{}}}
	SetCiliumClusterwideEnvoyConfigSpec(obj, spec)
	if len(obj.Spec.Resources) != 1 {
		t.Fatalf("expected 1 resource, got %d", len(obj.Spec.Resources))
	}
}

func TestAddCiliumClusterwideEnvoyConfigService(t *testing.T) {
	obj := CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{Name: "p"})
	svc := &ciliumv2.ServiceListener{Name: "svc", Namespace: "ns"}
	AddCiliumClusterwideEnvoyConfigService(obj, svc)
	if len(obj.Spec.Services) != 1 {
		t.Fatalf("expected 1 service, got %d", len(obj.Spec.Services))
	}
}

func TestAddCiliumClusterwideEnvoyConfigBackendService(t *testing.T) {
	obj := CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{Name: "p"})
	svc := &ciliumv2.Service{Name: "backend", Namespace: "ns"}
	AddCiliumClusterwideEnvoyConfigBackendService(obj, svc)
	if len(obj.Spec.BackendServices) != 1 {
		t.Fatalf("expected 1 backend service, got %d", len(obj.Spec.BackendServices))
	}
}

func TestAddCiliumClusterwideEnvoyConfigResource(t *testing.T) {
	obj := CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{Name: "p"})
	AddCiliumClusterwideEnvoyConfigResource(obj, ciliumv2.XDSResource{})
	AddCiliumClusterwideEnvoyConfigResource(obj, ciliumv2.XDSResource{})
	if len(obj.Spec.Resources) != 2 {
		t.Fatalf("expected 2 resources, got %d", len(obj.Spec.Resources))
	}
}

func TestSetCiliumClusterwideEnvoyConfigNodeSelector(t *testing.T) {
	obj := CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{Name: "p"})
	sel := &slimv1.LabelSelector{MatchLabels: map[string]string{"node": "worker"}}
	SetCiliumClusterwideEnvoyConfigNodeSelector(obj, sel)
	if obj.Spec.NodeSelector == nil {
		t.Fatal("expected non-nil NodeSelector")
	}
}

func TestSetCiliumBGPClusterConfigSpec(t *testing.T) {
	obj := CiliumBGPClusterConfig(&CiliumBGPClusterConfigConfig{Name: "p"})
	spec := ciliumv2.CiliumBGPClusterConfigSpec{
		BGPInstances: []ciliumv2.CiliumBGPInstance{{Name: "inst"}},
	}
	SetCiliumBGPClusterConfigSpec(obj, spec)
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP instance, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestSetCiliumBGPClusterConfigNodeSelector(t *testing.T) {
	obj := CiliumBGPClusterConfig(&CiliumBGPClusterConfigConfig{Name: "p"})
	sel := &slimv1.LabelSelector{MatchLabels: map[string]string{"bgp": "enabled"}}
	SetCiliumBGPClusterConfigNodeSelector(obj, sel)
	if obj.Spec.NodeSelector == nil {
		t.Fatal("expected non-nil NodeSelector")
	}
}

func TestAddCiliumBGPClusterConfigBGPInstance(t *testing.T) {
	obj := CiliumBGPClusterConfig(&CiliumBGPClusterConfigConfig{Name: "p"})
	AddCiliumBGPClusterConfigBGPInstance(obj, ciliumv2.CiliumBGPInstance{Name: "inst-1"})
	AddCiliumBGPClusterConfigBGPInstance(obj, ciliumv2.CiliumBGPInstance{Name: "inst-2"})
	if len(obj.Spec.BGPInstances) != 2 {
		t.Fatalf("expected 2 BGP instances, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestSetCiliumBGPPeerConfigSpec(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	ref := "bgp-secret"
	spec := ciliumv2.CiliumBGPPeerConfigSpec{AuthSecretRef: &ref}
	SetCiliumBGPPeerConfigSpec(obj, spec)
	if obj.Spec.AuthSecretRef == nil || *obj.Spec.AuthSecretRef != "bgp-secret" {
		t.Errorf("unexpected AuthSecretRef: %v", obj.Spec.AuthSecretRef)
	}
}

func TestSetCiliumBGPPeerConfigTransport(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	port := int32(179)
	transport := &ciliumv2.CiliumBGPTransport{PeerPort: &port}
	SetCiliumBGPPeerConfigTransport(obj, transport)
	if obj.Spec.Transport == nil || obj.Spec.Transport.PeerPort == nil || *obj.Spec.Transport.PeerPort != 179 {
		t.Errorf("unexpected Transport: %v", obj.Spec.Transport)
	}
}

func TestSetCiliumBGPPeerConfigTimers(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	hold := int32(90)
	timers := &ciliumv2.CiliumBGPTimers{HoldTimeSeconds: &hold}
	SetCiliumBGPPeerConfigTimers(obj, timers)
	if obj.Spec.Timers == nil || obj.Spec.Timers.HoldTimeSeconds == nil || *obj.Spec.Timers.HoldTimeSeconds != 90 {
		t.Errorf("unexpected Timers: %v", obj.Spec.Timers)
	}
}

func TestSetCiliumBGPPeerConfigAuthSecretRef(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	SetCiliumBGPPeerConfigAuthSecretRef(obj, "my-secret")
	if obj.Spec.AuthSecretRef == nil || *obj.Spec.AuthSecretRef != "my-secret" {
		t.Errorf("unexpected AuthSecretRef: %v", obj.Spec.AuthSecretRef)
	}
}

func TestSetCiliumBGPPeerConfigEBGPMultihop(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	SetCiliumBGPPeerConfigEBGPMultihop(obj, 5)
	if obj.Spec.EBGPMultihop == nil || *obj.Spec.EBGPMultihop != 5 {
		t.Errorf("unexpected EBGPMultihop: %v", obj.Spec.EBGPMultihop)
	}
}

func TestSetCiliumBGPPeerConfigGracefulRestart(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	enabled := true
	gr := &ciliumv2.CiliumBGPNeighborGracefulRestart{Enabled: enabled}
	SetCiliumBGPPeerConfigGracefulRestart(obj, gr)
	if obj.Spec.GracefulRestart == nil || !obj.Spec.GracefulRestart.Enabled {
		t.Errorf("unexpected GracefulRestart: %v", obj.Spec.GracefulRestart)
	}
}

func TestAddCiliumBGPPeerConfigFamily(t *testing.T) {
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{Name: "p"})
	fam := ciliumv2.CiliumBGPFamilyWithAdverts{
		CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
	}
	AddCiliumBGPPeerConfigFamily(obj, fam)
	AddCiliumBGPPeerConfigFamily(obj, fam)
	if len(obj.Spec.Families) != 2 {
		t.Fatalf("expected 2 families, got %d", len(obj.Spec.Families))
	}
}

func TestSetCiliumBGPAdvertisementSpec(t *testing.T) {
	obj := CiliumBGPAdvertisement(&CiliumBGPAdvertisementConfig{Name: "p"})
	spec := ciliumv2.CiliumBGPAdvertisementSpec{
		Advertisements: []ciliumv2.BGPAdvertisement{
			{AdvertisementType: ciliumv2.BGPServiceAdvert},
		},
	}
	SetCiliumBGPAdvertisementSpec(obj, spec)
	if len(obj.Spec.Advertisements) != 1 {
		t.Fatalf("expected 1 advertisement, got %d", len(obj.Spec.Advertisements))
	}
}

func TestAddCiliumBGPAdvertisementEntry(t *testing.T) {
	obj := CiliumBGPAdvertisement(&CiliumBGPAdvertisementConfig{Name: "p"})
	AddCiliumBGPAdvertisementEntry(obj, ciliumv2.BGPAdvertisement{AdvertisementType: ciliumv2.BGPServiceAdvert})
	AddCiliumBGPAdvertisementEntry(obj, ciliumv2.BGPAdvertisement{AdvertisementType: ciliumv2.BGPPodCIDRAdvert})
	if len(obj.Spec.Advertisements) != 2 {
		t.Fatalf("expected 2 advertisements, got %d", len(obj.Spec.Advertisements))
	}
}

func TestSetCiliumBGPNodeConfigSpec(t *testing.T) {
	obj := CiliumBGPNodeConfig(&CiliumBGPNodeConfigConfig{Name: "p"})
	spec := ciliumv2.CiliumBGPNodeSpec{
		BGPInstances: []ciliumv2.CiliumBGPNodeInstance{{Name: "inst"}},
	}
	SetCiliumBGPNodeConfigSpec(obj, spec)
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP node instance, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestAddCiliumBGPNodeConfigBGPInstance(t *testing.T) {
	obj := CiliumBGPNodeConfig(&CiliumBGPNodeConfigConfig{Name: "p"})
	AddCiliumBGPNodeConfigBGPInstance(obj, ciliumv2.CiliumBGPNodeInstance{Name: "inst-1"})
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP node instance, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestSetCiliumBGPNodeConfigOverrideSpec(t *testing.T) {
	obj := CiliumBGPNodeConfigOverride(&CiliumBGPNodeConfigOverrideConfig{Name: "p"})
	routerID := "10.0.0.1"
	spec := ciliumv2.CiliumBGPNodeConfigOverrideSpec{
		BGPInstances: []ciliumv2.CiliumBGPNodeConfigInstanceOverride{
			{Name: "inst", RouterID: &routerID},
		},
	}
	SetCiliumBGPNodeConfigOverrideSpec(obj, spec)
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP instance override, got %d", len(obj.Spec.BGPInstances))
	}
}

func TestAddCiliumBGPNodeConfigOverrideBGPInstance(t *testing.T) {
	obj := CiliumBGPNodeConfigOverride(&CiliumBGPNodeConfigOverrideConfig{Name: "p"})
	routerID := "10.0.0.2"
	AddCiliumBGPNodeConfigOverrideBGPInstance(obj, ciliumv2.CiliumBGPNodeConfigInstanceOverride{
		Name:     "inst-1",
		RouterID: &routerID,
	})
	if len(obj.Spec.BGPInstances) != 1 {
		t.Fatalf("expected 1 BGP instance override, got %d", len(obj.Spec.BGPInstances))
	}
	if *obj.Spec.BGPInstances[0].RouterID != "10.0.0.2" {
		t.Errorf("unexpected RouterID: %s", *obj.Spec.BGPInstances[0].RouterID)
	}
}
