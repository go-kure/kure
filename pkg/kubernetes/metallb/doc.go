// Package metallb exposes helper functions for constructing MetalLB
// resources. Each function returns a fully initialized controller-runtime
// object that can be serialized to YAML or modified further by the calling
// application.
//
// # Overview
//
// The package provides two layers of API:
//
//   - Create<Kind> constructors: allocate an object with TypeMeta and ObjectMeta
//     set, leaving the spec empty. Assign the spec fields directly, or use a
//     Set/Add helper where one exists.
//   - Set/Add helpers: admissible sugar over a single field of an existing
//     object. They exist for three shapes of write — appending to a slice or
//     inserting into a map, assigning to a pointer-typed field (a pointer the
//     caller already holds counts, and is forwarded directly), and composing an
//     upstream struct under a name that states the opinion. Everything else is
//     a field on the upstream type, assigned directly.
//
// Resources covered include IPAddressPool, BGPPeer, BGPAdvertisement,
// L2Advertisement, and BFDProfile.
//
// # Constructors
//
// Constructors follow the form Create<Kind>(name, namespace string). A minimal example
// creating an IPAddressPool looks like:
//
//	pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
//	metallb.AddIPAddressPoolAddress(pool, "192.168.1.0/24")
//	metallb.SetIPAddressPoolAutoAssign(pool, true)
//
// # Update helpers
//
// Additional functions prefixed with Set or Add expose granular control over
// the generated objects. There is no whole-spec setter: replacing a spec is a
// plain assignment (pool.Spec = metallbv1beta1.IPAddressPoolSpec{...}), and the
// builder contract does not wrap one.
package metallb
