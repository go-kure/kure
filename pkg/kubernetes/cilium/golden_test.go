package cilium

import (
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/policy/api"
	listenerv3 "github.com/envoyproxy/go-control-plane/envoy/config/listener/v3"
	"google.golang.org/protobuf/types/known/anypb"
	"k8s.io/utils/ptr"

	"github.com/go-kure/kure/internal/kuretest"
)

// The fixtures under testdata were written by the config-struct layer this
// package used to carry (CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{...})
// and its twelve siblings) before it was retired. Each test below builds the
// same object on the generated constructor plus the upstream struct and must
// reproduce that output byte for byte.

func TestGolden_CiliumNetworkPolicySpec(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("allow-internal", "default")
	SetCiliumNetworkPolicySpec(obj, &api.Rule{
		Description:      "allow traffic from the same namespace",
		EndpointSelector: api.NewESFromLabels(),
		Ingress: []api.IngressRule{{
			IngressCommonRule: api.IngressCommonRule{FromEndpoints: []api.EndpointSelector{api.NewESFromLabels()}},
		}},
	})
	kuretest.Golden(t, "ciliumnetworkpolicy-spec.yaml", obj)
}

func TestGolden_CiliumNetworkPolicySpecs(t *testing.T) {
	obj := CreateCiliumNetworkPolicy("multi-rule", "default")
	// every rule needs a subject selector and at least one traffic direction
	AddCiliumNetworkPolicySpec(obj, &api.Rule{
		Description:      "r1",
		EndpointSelector: api.NewESFromLabels(),
		Ingress: []api.IngressRule{{
			IngressCommonRule: api.IngressCommonRule{FromEndpoints: []api.EndpointSelector{api.NewESFromLabels()}},
		}},
	})
	AddCiliumNetworkPolicySpec(obj, &api.Rule{
		Description:      "r2",
		EndpointSelector: api.NewESFromLabels(),
		Egress: []api.EgressRule{{
			EgressCommonRule: api.EgressCommonRule{ToEntities: []api.Entity{api.EntityWorld}},
		}},
	})
	kuretest.Golden(t, "ciliumnetworkpolicy-specs.yaml", obj)
}

func TestGolden_CiliumClusterwideNetworkPolicySpec(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("allow-health-checks")
	SetCiliumClusterwideNetworkPolicySpec(obj, &api.Rule{
		NodeSelector: api.NewESFromLabels(),
		Ingress: []api.IngressRule{{
			IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityHost}},
		}},
	})
	kuretest.Golden(t, "ciliumclusterwidenetworkpolicy-spec.yaml", obj)
}

func TestGolden_CiliumClusterwideNetworkPolicySpecs(t *testing.T) {
	obj := CreateCiliumClusterwideNetworkPolicy("multi-rule")
	AddCiliumClusterwideNetworkPolicySpec(obj, &api.Rule{
		Description:  "r1",
		NodeSelector: api.NewESFromLabels(),
		Ingress: []api.IngressRule{{
			IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityHost}},
		}},
	})
	kuretest.Golden(t, "ciliumclusterwidenetworkpolicy-specs.yaml", obj)
}

func TestGolden_CiliumCIDRGroup(t *testing.T) {
	obj := CreateCiliumCIDRGroup("internal-ranges")
	obj.Spec.ExternalCIDRs = []api.CIDR{"10.0.0.0/8", "192.168.0.0/16"}
	kuretest.Golden(t, "ciliumcidrgroup.yaml", obj)
}

func TestGolden_CiliumEgressGatewayPolicy(t *testing.T) {
	obj := CreateCiliumEgressGatewayPolicy("prod-egress")
	obj.Spec = ciliumv2.CiliumEgressGatewayPolicySpec{
		Selectors:        []ciliumv2.EgressRule{{PodSelector: &slimv1.LabelSelector{MatchLabels: map[string]string{"app": "crawler"}}}},
		DestinationCIDRs: []ciliumv2.CIDR{"0.0.0.0/0"},
		ExcludedCIDRs:    []ciliumv2.CIDR{"10.0.0.0/8"},
		EgressGateway: &ciliumv2.EgressGateway{
			NodeSelector: &slimv1.LabelSelector{MatchLabels: map[string]string{"node-role.kubernetes.io/egress": ""}},
			Interface:    "eth0",
		},
	}
	kuretest.Golden(t, "ciliumegressgatewaypolicy.yaml", obj)
}

func TestGolden_CiliumLocalRedirectPolicy(t *testing.T) {
	obj := CreateCiliumLocalRedirectPolicy("dns-redirect", "kube-system")
	obj.Spec = ciliumv2.CiliumLocalRedirectPolicySpec{
		RedirectFrontend: ciliumv2.RedirectFrontend{
			AddressMatcher: &ciliumv2.Frontend{IP: "169.254.20.10", ToPorts: []ciliumv2.PortInfo{{Port: "53", Protocol: "UDP"}}},
		},
		RedirectBackend: ciliumv2.RedirectBackend{
			LocalEndpointSelector: slimv1.LabelSelector{MatchLabels: map[string]string{"k8s-app": "coredns"}},
			ToPorts:               []ciliumv2.PortInfo{{Port: "53", Protocol: "UDP"}},
		},
	}
	kuretest.Golden(t, "ciliumlocalredirectpolicy.yaml", obj)
}

func TestGolden_CiliumLoadBalancerIPPool(t *testing.T) {
	obj := CreateCiliumLoadBalancerIPPool("public-pool")
	obj.Spec = ciliumv2.CiliumLoadBalancerIPPoolSpec{
		Blocks:          []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
		ServiceSelector: &slimv1.LabelSelector{MatchLabels: map[string]string{"svc": "lb"}},
	}
	kuretest.Golden(t, "ciliumloadbalancerippool.yaml", obj)
}

// listener is an Envoy listener carried the way cilium carries every xDS
// resource: a protobuf Any, which XDSResource marshals with protojson.
func listener(t *testing.T, name string) ciliumv2.XDSResource {
	t.Helper()
	res, err := anypb.New(&listenerv3.Listener{
		Name: name,
		FilterChains: []*listenerv3.FilterChain{{
			Filters: []*listenerv3.Filter{{Name: "envoy.filters.network.http_connection_manager"}},
		}},
	})
	if err != nil {
		t.Fatalf("wrapping the listener: %v", err)
	}
	return ciliumv2.XDSResource{Any: res}
}

func TestGolden_CiliumEnvoyConfig(t *testing.T) {
	obj := CreateCiliumEnvoyConfig("my-proxy", "default")
	obj.Spec = ciliumv2.CiliumEnvoyConfigSpec{
		Services:        []*ciliumv2.ServiceListener{{Name: "my-svc", Namespace: "default"}},
		BackendServices: []*ciliumv2.Service{{Name: "backend", Namespace: "default"}},
		Resources:       []ciliumv2.XDSResource{listener(t, "my-proxy-listener")},
	}
	kuretest.Golden(t, "ciliumenvoyconfig.yaml", obj)
}

func TestGolden_CiliumClusterwideEnvoyConfig(t *testing.T) {
	obj := CreateCiliumClusterwideEnvoyConfig("cluster-proxy")
	obj.Spec = ciliumv2.CiliumEnvoyConfigSpec{
		Services:  []*ciliumv2.ServiceListener{{Name: "my-svc", Namespace: "default"}},
		Resources: []ciliumv2.XDSResource{listener(t, "cluster-proxy-listener")},
	}
	kuretest.Golden(t, "ciliumclusterwideenvoyconfig.yaml", obj)
}

func TestGolden_CiliumBGPClusterConfig(t *testing.T) {
	obj := CreateCiliumBGPClusterConfig("default-bgp")
	obj.Spec = ciliumv2.CiliumBGPClusterConfigSpec{
		NodeSelector: &slimv1.LabelSelector{MatchLabels: map[string]string{"bgp": "enabled"}},
		BGPInstances: []ciliumv2.CiliumBGPInstance{{Name: "instance-65000"}},
	}
	kuretest.Golden(t, "ciliumbgpclusterconfig.yaml", obj)
}

func TestGolden_CiliumBGPPeerConfig(t *testing.T) {
	obj := CreateCiliumBGPPeerConfig("peer-65001")
	obj.Spec = ciliumv2.CiliumBGPPeerConfigSpec{
		EBGPMultihop: ptr.To[int32](2),
		Families: []ciliumv2.CiliumBGPFamilyWithAdverts{{
			CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
		}},
	}
	kuretest.Golden(t, "ciliumbgppeerconfig.yaml", obj)
}

func TestGolden_CiliumBGPAdvertisement(t *testing.T) {
	obj := CreateCiliumBGPAdvertisement("pod-cidr-advert")
	obj.Spec = ciliumv2.CiliumBGPAdvertisementSpec{
		Advertisements: []ciliumv2.BGPAdvertisement{{AdvertisementType: ciliumv2.BGPPodCIDRAdvert}},
	}
	kuretest.Golden(t, "ciliumbgpadvertisement.yaml", obj)
}

func TestGolden_CiliumBGPNodeConfig(t *testing.T) {
	obj := CreateCiliumBGPNodeConfig("node-worker-1")
	obj.Spec = ciliumv2.CiliumBGPNodeSpec{
		BGPInstances: []ciliumv2.CiliumBGPNodeInstance{{Name: "instance-65000"}},
	}
	kuretest.Golden(t, "ciliumbgpnodeconfig.yaml", obj)
}

func TestGolden_CiliumBGPNodeConfigOverride(t *testing.T) {
	obj := CreateCiliumBGPNodeConfigOverride("node-worker-1")
	obj.Spec = ciliumv2.CiliumBGPNodeConfigOverrideSpec{
		BGPInstances: []ciliumv2.CiliumBGPNodeConfigInstanceOverride{{Name: "instance-65000", RouterID: ptr.To("10.0.0.1")}},
	}
	kuretest.Golden(t, "ciliumbgpnodeconfigoverride.yaml", obj)
}
