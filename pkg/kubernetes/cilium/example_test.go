package cilium_test

import (
	"fmt"

	ciliumv2 "github.com/cilium/cilium/pkg/k8s/apis/cilium.io/v2"
	slimv1 "github.com/cilium/cilium/pkg/k8s/slim/k8s/apis/meta/v1"
	"github.com/cilium/cilium/pkg/policy/api"

	"github.com/go-kure/kure/pkg/kubernetes/cilium"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it built so go test checks
// the example as well as compiling it.

func ExampleCreateCiliumNetworkPolicy() {
	obj := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
	cl := cilium.CreateCiliumCIDRGroup("internal-ranges")
	fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name, cl.Kind, cl.Name)
	// Output: CiliumNetworkPolicy default/allow-internal CiliumCIDRGroup internal-ranges
}

func ExampleSetCiliumNetworkPolicySpec() {
	policy := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
	cilium.SetCiliumNetworkPolicySpec(policy, &api.Rule{
		EndpointSelector: api.NewESFromLabels(),
		Ingress: []api.IngressRule{
			{
				IngressCommonRule: api.IngressCommonRule{
					FromEndpoints: []api.EndpointSelector{
						api.NewESFromLabels(),
					},
				},
			},
		},
	})
	fmt.Println(len(policy.Spec.Ingress))
	// Output: 1
}

func ExampleSetCiliumClusterwideNetworkPolicySpec() {
	ccnp := cilium.CreateCiliumClusterwideNetworkPolicy("allow-health-checks")
	cilium.SetCiliumClusterwideNetworkPolicySpec(ccnp, &api.Rule{
		NodeSelector: api.NewESFromLabels(),
		Ingress: []api.IngressRule{
			{
				IngressCommonRule: api.IngressCommonRule{
					FromEntities: []api.Entity{api.EntityHost},
				},
			},
		},
	})
	fmt.Println(ccnp.Spec.Ingress[0].FromEntities)
	// Output: [host]
}

func ExampleCreateCiliumCIDRGroup() {
	group := cilium.CreateCiliumCIDRGroup("internal-ranges")
	group.Spec.ExternalCIDRs = []api.CIDR{
		"10.0.0.0/8",
		"192.168.0.0/16",
		"172.16.0.0/12",
	}
	fmt.Println(group.Spec.ExternalCIDRs)
	// Output: [10.0.0.0/8 192.168.0.0/16 172.16.0.0/12]
}

func ExampleCreateCiliumEgressGatewayPolicy() {
	cegp := cilium.CreateCiliumEgressGatewayPolicy("prod-egress")
	cegp.Spec = ciliumv2.CiliumEgressGatewayPolicySpec{
		DestinationCIDRs: []ciliumv2.CIDR{"0.0.0.0/0"},
		EgressGateway:    &ciliumv2.EgressGateway{Interface: "eth0"},
	}
	fmt.Println(cegp.Spec.DestinationCIDRs, cegp.Spec.EgressGateway.Interface)
	// Output: [0.0.0.0/0] eth0
}

func ExampleCreateCiliumLocalRedirectPolicy() {
	lrp := cilium.CreateCiliumLocalRedirectPolicy("dns-redirect", "kube-system")
	lrp.Spec = ciliumv2.CiliumLocalRedirectPolicySpec{
		RedirectFrontend: ciliumv2.RedirectFrontend{
			AddressMatcher: &ciliumv2.Frontend{IP: "169.254.20.10", ToPorts: []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}}},
		},
		RedirectBackend: ciliumv2.RedirectBackend{
			LocalEndpointSelector: slimv1.LabelSelector{MatchLabels: map[string]string{"k8s-app": "coredns"}},
			ToPorts:               []ciliumv2.PortInfo{{Port: "53", Protocol: "ANY"}},
		},
	}
	fmt.Println(lrp.Spec.RedirectFrontend.AddressMatcher.IP, lrp.Spec.RedirectBackend.LocalEndpointSelector.MatchLabels["k8s-app"])
	// Output: 169.254.20.10 coredns
}

func ExampleAddCiliumLoadBalancerIPPoolBlock() {
	pool := cilium.CreateCiliumLoadBalancerIPPool("public-pool")
	pool.Spec = ciliumv2.CiliumLoadBalancerIPPoolSpec{
		Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
	}
	// Add blocks incrementally:
	cilium.AddCiliumLoadBalancerIPPoolBlock(pool, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "198.51.100.0/24"})
	fmt.Println(pool.Spec.Blocks[0].Cidr, pool.Spec.Blocks[1].Cidr)
	// Output: 203.0.113.0/24 198.51.100.0/24
}

func ExampleAddCiliumEnvoyConfigService() {
	// An Envoy listener, cluster or route, wrapped in its protobuf Any.
	var xdsResource ciliumv2.XDSResource

	cec := cilium.CreateCiliumEnvoyConfig("my-proxy", "default")
	cilium.AddCiliumEnvoyConfigService(cec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
	cilium.AddCiliumEnvoyConfigResource(cec, xdsResource)
	fmt.Println(cec.Spec.Services[0].Name, len(cec.Spec.Resources))
	// Output: my-svc 1
}

func ExampleAddCiliumClusterwideEnvoyConfigService() {
	ccec := cilium.CreateCiliumClusterwideEnvoyConfig("cluster-proxy")
	cilium.AddCiliumClusterwideEnvoyConfigService(ccec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
	fmt.Println(ccec.Spec.Services[0].Namespace + "/" + ccec.Spec.Services[0].Name)
	// Output: default/my-svc
}

func ExampleSetCiliumBGPClusterConfigNodeSelector() {
	bgpcc := cilium.CreateCiliumBGPClusterConfig("default-bgp")
	cilium.SetCiliumBGPClusterConfigNodeSelector(bgpcc, &slimv1.LabelSelector{
		MatchLabels: map[string]string{"bgp": "enabled"},
	})
	cilium.AddCiliumBGPClusterConfigBGPInstance(bgpcc, ciliumv2.CiliumBGPInstance{
		Name: "instance-65000",
	})
	fmt.Println(bgpcc.Spec.NodeSelector.MatchLabels["bgp"], bgpcc.Spec.BGPInstances[0].Name)
	// Output: enabled instance-65000
}

func ExampleSetCiliumBGPPeerConfigEBGPMultihop() {
	peer := cilium.CreateCiliumBGPPeerConfig("peer-65001")
	cilium.SetCiliumBGPPeerConfigEBGPMultihop(peer, 2)
	cilium.AddCiliumBGPPeerConfigFamily(peer, ciliumv2.CiliumBGPFamilyWithAdverts{
		CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv4", Safi: "unicast"},
	})
	fmt.Println(*peer.Spec.EBGPMultihop, peer.Spec.Families[0].Afi)
	// Output: 2 ipv4
}

func ExampleAddCiliumBGPAdvertisementEntry() {
	advert := cilium.CreateCiliumBGPAdvertisement("pod-cidr-advert")
	cilium.AddCiliumBGPAdvertisementEntry(advert, ciliumv2.BGPAdvertisement{
		AdvertisementType: ciliumv2.BGPPodCIDRAdvert,
	})
	fmt.Println(advert.Spec.Advertisements[0].AdvertisementType)
	// Output: PodCIDR
}

func ExampleAddCiliumBGPNodeConfigBGPInstance() {
	nc := cilium.CreateCiliumBGPNodeConfig("node-worker-1")
	cilium.AddCiliumBGPNodeConfigBGPInstance(nc, ciliumv2.CiliumBGPNodeInstance{
		Name: "instance-65000",
	})
	fmt.Println(nc.Spec.BGPInstances[0].Name)
	// Output: instance-65000
}

func ExampleAddCiliumBGPNodeConfigOverrideBGPInstance() {
	routerID := "10.0.0.1"
	override := cilium.CreateCiliumBGPNodeConfigOverride("node-worker-1")
	cilium.AddCiliumBGPNodeConfigOverrideBGPInstance(override, ciliumv2.CiliumBGPNodeConfigInstanceOverride{
		Name:     "instance-65000",
		RouterID: &routerID,
	})
	fmt.Println(*override.Spec.BGPInstances[0].RouterID)
	// Output: 10.0.0.1
}

func ExampleAddCiliumNetworkPolicySpec() {
	policy := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
	ccnp := cilium.CreateCiliumClusterwideNetworkPolicy("allow-health-checks")
	group := cilium.CreateCiliumCIDRGroup("internal-ranges")
	cegp := cilium.CreateCiliumEgressGatewayPolicy("prod-egress")
	pool := cilium.CreateCiliumLoadBalancerIPPool("public-pool")
	cec := cilium.CreateCiliumEnvoyConfig("my-proxy", "default")
	bgpcc := cilium.CreateCiliumBGPClusterConfig("default-bgp")
	peer := cilium.CreateCiliumBGPPeerConfig("peer-65001")
	advert := cilium.CreateCiliumBGPAdvertisement("pod-cidr-advert")
	nc := cilium.CreateCiliumBGPNodeConfig("node-worker-1")
	override := cilium.CreateCiliumBGPNodeConfigOverride("node-worker-1")

	selector := &slimv1.LabelSelector{MatchLabels: map[string]string{"bgp": "enabled"}}
	multihop := int32(2)

	// Network policies
	cilium.SetCiliumNetworkPolicySpec(policy, &api.Rule{EndpointSelector: api.NewESFromLabels()})
	cilium.AddCiliumNetworkPolicySpec(policy, &api.Rule{EndpointSelector: api.NewESFromLabels()})
	cilium.SetCiliumClusterwideNetworkPolicyNodeSelector(ccnp, api.NewESFromLabels())
	cilium.AddCiliumNetworkPolicyIngressRule(policy, api.IngressRule{
		IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityCluster}},
	})
	cilium.AddCiliumNetworkPolicyEgressRule(policy, api.EgressRule{
		EgressCommonRule: api.EgressCommonRule{ToEntities: []api.Entity{api.EntityWorld}},
	})
	cilium.AddCiliumNetworkPolicyIngressDenyRule(policy, api.IngressDenyRule{
		IngressCommonRule: api.IngressCommonRule{FromEntities: []api.Entity{api.EntityWorld}},
	})
	cilium.AddCiliumNetworkPolicyEgressDenyRule(policy, api.EgressDenyRule{
		EgressCommonRule: api.EgressCommonRule{ToEntities: []api.Entity{api.EntityHost}},
	})

	// CIDR groups
	cilium.AddCiliumCIDRGroupCIDR(group, "203.0.113.0/24")
	group.Spec.ExternalCIDRs = []api.CIDR{"10.0.0.0/8"}

	// Egress gateway
	cilium.AddCiliumEgressGatewayPolicySelectorRule(cegp, ciliumv2.EgressRule{PodSelector: selector})
	cilium.AddCiliumEgressGatewayPolicyDestinationCIDR(cegp, "10.0.0.0/8")
	cilium.SetCiliumEgressGatewayPolicyEgressGateway(cegp, &ciliumv2.EgressGateway{Interface: "eth0"})

	// LB IP pool
	cilium.AddCiliumLoadBalancerIPPoolBlock(pool, ciliumv2.CiliumLoadBalancerIPPoolIPBlock{Cidr: "198.51.100.0/24"})
	cilium.SetCiliumLoadBalancerIPPoolServiceSelector(pool, selector)
	pool.Spec.Disabled = true

	// Envoy config
	cilium.AddCiliumEnvoyConfigService(cec, &ciliumv2.ServiceListener{Name: "my-svc", Namespace: "default"})
	cilium.AddCiliumEnvoyConfigBackendService(cec, &ciliumv2.Service{Name: "backend", Namespace: "default"})
	cilium.SetCiliumEnvoyConfigNodeSelector(cec, selector)

	// BGP
	cilium.SetCiliumBGPClusterConfigNodeSelector(bgpcc, selector)
	cilium.AddCiliumBGPClusterConfigBGPInstance(bgpcc, ciliumv2.CiliumBGPInstance{Name: "instance-65000"})
	cilium.SetCiliumBGPPeerConfigTransport(peer, &ciliumv2.CiliumBGPTransport{})
	cilium.SetCiliumBGPPeerConfigTimers(peer, &ciliumv2.CiliumBGPTimers{})
	cilium.SetCiliumBGPPeerConfigEBGPMultihop(peer, multihop)
	cilium.AddCiliumBGPPeerConfigFamily(peer, ciliumv2.CiliumBGPFamilyWithAdverts{
		CiliumBGPFamily: ciliumv2.CiliumBGPFamily{Afi: "ipv6", Safi: "unicast"},
	})
	cilium.AddCiliumBGPAdvertisementEntry(advert, ciliumv2.BGPAdvertisement{AdvertisementType: ciliumv2.BGPServiceAdvert})
	cilium.AddCiliumBGPNodeConfigBGPInstance(nc, ciliumv2.CiliumBGPNodeInstance{Name: "instance-65000"})
	cilium.AddCiliumBGPNodeConfigOverrideBGPInstance(override, ciliumv2.CiliumBGPNodeConfigInstanceOverride{Name: "instance-65000"})

	fmt.Println(len(policy.Specs), len(policy.Spec.Ingress), len(policy.Spec.EgressDeny), cegp.Spec.DestinationCIDRs, pool.Spec.Disabled)
	// Output: 1 1 1 [10.0.0.0/8] true
}
