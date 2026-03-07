// Package metallb exposes helper functions for constructing MetalLB
// resources.  Each function returns a fully initialized controller-runtime
// object that can be serialized to YAML or modified further by the calling
// application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/metallb` so applications can build MetalLB manifests
// programmatically without depending on the internal packages directly.
// All builders operate on configuration structures defined in this package
// and convert them to the appropriate MetalLB custom resources.
//
// Resources covered include [IPAddressPool], [BGPPeer], [BGPAdvertisement],
// [L2Advertisement], and [BFDProfile].
//
// ## Constructors
//
// Constructors follow the form `<Type>` and accept a configuration struct.  A
// minimal example creating an IPAddressPool looks like:
//
//	pool := metallb.IPAddressPool(&metallb.IPAddressPoolConfig{
//	        Name:      "my-pool",
//	        Namespace: "metallb-system",
//	        Addresses: []string{"192.168.1.0/24"},
//	})
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control
// over the generated objects.  They delegate to the internal package to
// perform the actual mutations while keeping the public API stable.
package metallb
