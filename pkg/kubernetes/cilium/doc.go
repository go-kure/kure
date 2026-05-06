// Package cilium exposes helper functions for constructing Cilium network
// policy resources.  Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified further
// by the calling application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/cilium` so applications can build Cilium manifests
// programmatically without depending on the internal packages directly.
// All builders operate on configuration structures defined in this package
// and convert them to the appropriate Cilium custom resources.
//
// Resources covered include [CiliumNetworkPolicy],
// [CiliumClusterwideNetworkPolicy], and [CiliumCIDRGroup].
//
// ## Constructors
//
// Constructors follow the form `<Type>` and accept a configuration struct.  A
// minimal example creating a CiliumNetworkPolicy looks like:
//
//	policy := cilium.CiliumNetworkPolicy(&cilium.CiliumNetworkPolicyConfig{
//	        Name:      "allow-internal",
//	        Namespace: "default",
//	})
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control
// over the generated objects.  They delegate to the internal package to
// perform the actual mutations while keeping the public API stable.
package cilium
