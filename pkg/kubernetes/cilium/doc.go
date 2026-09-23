// Package cilium exposes the generated constructors and the admissible sugar
// for Cilium resources: the network policies, CIDR groups, egress gateway and
// local redirect policies, load balancer IP pools, Envoy configs and the BGPv2
// kinds. Each constructor returns a controller-runtime object carrying
// identity only; the upstream ciliumv2 struct (and policy/api.Rule for the
// two policy kinds) is the construction API.
//
// ## Constructors
//
// Create<Kind> is generated from the registered scheme and emits apiVersion,
// kind, metadata.name and, for a namespaced kind, metadata.namespace. Spec
// fields are the caller's own assignments:
//
//	policy := cilium.CreateCiliumNetworkPolicy("allow-internal", "default")
//	cilium.SetCiliumNetworkPolicySpec(policy, &api.Rule{
//	        EndpointSelector: api.NewESFromLabels(),
//	})
//
//	pool := cilium.CreateCiliumLoadBalancerIPPool("public-pool")
//	pool.Spec = ciliumv2.CiliumLoadBalancerIPPoolSpec{
//	        Blocks: []ciliumv2.CiliumLoadBalancerIPPoolIPBlock{{Cidr: "203.0.113.0/24"}},
//	}
//
// ## Update helpers
//
// Functions prefixed with Set or Add write exactly the one field they name
// and fall into the builder contract's admitted classes. ObjectMeta labels
// and annotations use the generic kubernetes.AddLabel /
// kubernetes.AddAnnotation; the Set*Labels helpers here write the policy's
// own `spec.labels`, which is a different field.
//
// The config-struct layer this package used to carry — one Kind(&KindConfig)
// function per kind, a verbatim copy of `obj.Spec = spec` — was retired by
// release 2 of the builder contract; see docs/builder-contract-release-2.md
// for the field-by-field mapping.
package cilium
