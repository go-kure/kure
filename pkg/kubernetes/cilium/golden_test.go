package cilium

import (
	"flag"
	"os"
	"path/filepath"
	"testing"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/policy/api"
	"sigs.k8s.io/controller-runtime/pkg/client"

	kureio "github.com/go-kure/kure/pkg/io"
)

var update = flag.Bool("update", false, "update golden files")

func goldenTest(t *testing.T, filename string, obj client.Object) {
	t.Helper()
	objects := []*client.Object{&obj}
	got, err := kureio.EncodeObjectsToYAMLWithOptions(objects, kureio.EncodeOptions{
		KubernetesFieldOrder: true,
	})
	if err != nil {
		t.Fatalf("encoding to YAML: %v", err)
	}
	golden := filepath.Join("testdata", filename)
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatalf("creating testdata dir: %v", err)
		}
		if err := os.WriteFile(golden, got, 0o644); err != nil {
			t.Fatalf("updating golden file: %v", err)
		}
		return
	}
	want, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("reading golden file (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("output does not match golden file %s\n\ngot:\n%s\nwant:\n%s", golden, got, want)
	}
}

func TestGolden_CiliumNetworkPolicySpec(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{
		Name:      "allow-internal",
		Namespace: "default",
		Spec: &api.Rule{
			Description:      "allow traffic from the same namespace",
			EndpointSelector: api.NewESFromLabels(),
			Ingress: []api.IngressRule{{
				IngressCommonRule: api.IngressCommonRule{FromEndpoints: []api.EndpointSelector{api.NewESFromLabels()}},
			}},
		},
	})
	goldenTest(t, "ciliumnetworkpolicy-spec.yaml", obj)
}

func TestGolden_CiliumNetworkPolicySpecs(t *testing.T) {
	obj := CiliumNetworkPolicy(&CiliumNetworkPolicyConfig{
		Name:      "multi-rule",
		Namespace: "default",
		Specs:     api.Rules{&api.Rule{Description: "r1"}, &api.Rule{Description: "r2"}},
	})
	goldenTest(t, "ciliumnetworkpolicy-specs.yaml", obj)
}

func TestGolden_CiliumClusterwideNetworkPolicySpec(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{
		Name: "allow-health-checks",
		Spec: &api.Rule{
			NodeSelector: api.NewESFromLabels(),
			Ingress: []api.IngressRule{{
				IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityHost}},
			}},
		},
	})
	goldenTest(t, "ciliumclusterwidenetworkpolicy-spec.yaml", obj)
}

func TestGolden_CiliumClusterwideNetworkPolicySpecs(t *testing.T) {
	obj := CiliumClusterwideNetworkPolicy(&CiliumClusterwideNetworkPolicyConfig{
		Name:  "multi-rule",
		Specs: api.Rules{&api.Rule{Description: "r1"}},
	})
	goldenTest(t, "ciliumclusterwidenetworkpolicy-specs.yaml", obj)
}

func TestGolden_CiliumCIDRGroup(t *testing.T) {
	obj := CiliumCIDRGroup(&CiliumCIDRGroupConfig{
		Name:          "internal-ranges",
		ExternalCIDRs: []api.CIDR{"10.0.0.0/8", "192.168.0.0/16"},
	})
	goldenTest(t, "ciliumcidrgroup.yaml", obj)
}

func TestGolden_CiliumEgressGatewayPolicy(t *testing.T) {
	obj := CiliumEgressGatewayPolicy(&CiliumEgressGatewayPolicyConfig{
		Name: "prod-egress",
		Spec: ciliumv2.CiliumEgressGatewayPolicySpec{
			DestinationCIDRs: []ciliumv2.CIDR{"0.0.0.0/0"},
			ExcludedCIDRs:    []ciliumv2.CIDR{"10.0.0.0/8"},
			EgressGateway:    &ciliumv2.EgressGateway{Interface: "eth0"},
		},
	})
	goldenTest(t, "ciliumegressgatewaypolicy.yaml", obj)
}

func TestGolden_CiliumLocalRedirectPolicy(t *testing.T) {
	obj := CiliumLocalRedirectPolicy(&CiliumLocalRedirectPolicyConfig{
		Name:      "dns-redirect",
		Namespace: "kube-system",
		Spec: ciliumv2.CiliumLocalRedirectPolicySpec{
			RedirectFrontend: ciliumv2.RedirectFrontend{
				AddressMatcher: &ciliumv2.Frontend{IP: "169.254.20.10", ToPorts: []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}}},
			},
			RedirectBackend: ciliumv2.RedirectBackend{
				LocalEndpointSelector: slimv1.LabelSelector{MatchLabels: map[string]string{"k8s-app": "coredns"}},
				ToPorts:               []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}},
			},
		},
	})
	goldenTest(t, "ciliumlocalredirectpolicy.yaml", obj)
}

func TestGolden_CiliumLoadBalancerIPPool(t *testing.T) {
	obj := CiliumLoadBalancerIPPool(&CiliumLoadBalancerIPPoolConfig{
		Name: "public-pool",
		Spec: ciliumv2.CiliumLoadBalancerIPPoolSpec{
			Blocks:          []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
			ServiceSelector: &slimv1.LabelSelector{MatchLabels: map[string]string{"svc": "lb"}},
		},
	})
	goldenTest(t, "ciliumloadbalancerippool.yaml", obj)
}

func TestGolden_CiliumEnvoyConfig(t *testing.T) {
	obj := CiliumEnvoyConfig(&CiliumEnvoyConfigConfig{
		Name:      "my-proxy",
		Namespace: "default",
		Spec: ciliumv2.CiliumEnvoyConfigSpec{
			Services:        []*ciliumv2.ServiceListener{{Name: "my-svc", Namespace: "default"}},
			BackendServices: []*ciliumv2.Service{{Name: "backend", Namespace: "default"}},
		},
	})
	goldenTest(t, "ciliumenvoyconfig.yaml", obj)
}

func TestGolden_CiliumClusterwideEnvoyConfig(t *testing.T) {
	obj := CiliumClusterwideEnvoyConfig(&CiliumClusterwideEnvoyConfigConfig{
		Name: "cluster-proxy",
		Spec: ciliumv2.CiliumEnvoyConfigSpec{
			Services: []*ciliumv2.ServiceListener{{Name: "my-svc", Namespace: "default"}},
		},
	})
	goldenTest(t, "ciliumclusterwideenvoyconfig.yaml", obj)
}

func TestGolden_CiliumBGPClusterConfig(t *testing.T) {
	obj := CiliumBGPClusterConfig(&CiliumBGPClusterConfigConfig{
		Name: "default-bgp",
		Spec: ciliumv2.CiliumBGPClusterConfigSpec{
			NodeSelector: &slimv1.LabelSelector{MatchLabels: map[string]string{"bgp": "enabled"}},
			BGPInstances: []ciliumv2.CiliumBGPInstance{{Name: "instance-65000"}},
		},
	})
	goldenTest(t, "ciliumbgpclusterconfig.yaml", obj)
}

func TestGolden_CiliumBGPPeerConfig(t *testing.T) {
	ttl := int32(2)
	obj := CiliumBGPPeerConfig(&CiliumBGPPeerConfigConfig{
		Name: "peer-65001",
		Spec: ciliumv2.CiliumBGPPeerConfigSpec{
			EBGPMultihop: &ttl,
			Families: []ciliumv2.CiliumBGPFamilyWithAdverts{{
				CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
			}},
		},
	})
	goldenTest(t, "ciliumbgppeerconfig.yaml", obj)
}

func TestGolden_CiliumBGPAdvertisement(t *testing.T) {
	obj := CiliumBGPAdvertisement(&CiliumBGPAdvertisementConfig{
		Name: "pod-cidr-advert",
		Spec: ciliumv2.CiliumBGPAdvertisementSpec{
			Advertisements: []ciliumv2.BGPAdvertisement{{AdvertisementType: ciliumv2.BGPPodCIDRAdvert}},
		},
	})
	goldenTest(t, "ciliumbgpadvertisement.yaml", obj)
}

func TestGolden_CiliumBGPNodeConfig(t *testing.T) {
	obj := CiliumBGPNodeConfig(&CiliumBGPNodeConfigConfig{
		Name: "node-worker-1",
		Spec: ciliumv2.CiliumBGPNodeSpec{
			BGPInstances: []ciliumv2.CiliumBGPNodeInstance{{Name: "instance-65000"}},
		},
	})
	goldenTest(t, "ciliumbgpnodeconfig.yaml", obj)
}

func TestGolden_CiliumBGPNodeConfigOverride(t *testing.T) {
	routerID := "10.0.0.1"
	obj := CiliumBGPNodeConfigOverride(&CiliumBGPNodeConfigOverrideConfig{
		Name: "node-worker-1",
		Spec: ciliumv2.CiliumBGPNodeConfigOverrideSpec{
			BGPInstances: []ciliumv2.CiliumBGPNodeConfigInstanceOverride{{Name: "instance-65000", RouterID: &routerID}},
		},
	})
	goldenTest(t, "ciliumbgpnodeconfigoverride.yaml", obj)
}
