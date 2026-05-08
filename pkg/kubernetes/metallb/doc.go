// Package metallb exposes helper functions for constructing MetalLB
// resources. Each function returns a fully initialized controller-runtime
// object that can be serialized to YAML or modified further by the calling
// application.
//
// # Overview
//
// The package provides two layers of API:
//
//   - CreateX constructors: allocate an object with TypeMeta and ObjectMeta set,
//     leaving the spec empty. Use the Set/Add helpers to populate it.
//   - SetX/AddX helpers: granular setters that mutate a single field on an existing
//     object.
//
// Resources covered include IPAddressPool, BGPPeer, BGPAdvertisement,
// L2Advertisement, and BFDProfile.
//
// # Constructors
//
// Constructors follow the form CreateX(name, namespace string). A minimal example
// creating an IPAddressPool looks like:
//
//	pool := metallb.CreateIPAddressPool("my-pool", "metallb-system")
//	metallb.AddIPAddressPoolAddress(pool, "192.168.1.0/24")
//	metallb.SetIPAddressPoolAutoAssign(pool, true)
//
// # Update helpers
//
// Additional functions prefixed with Set or Add expose granular control over
// the generated objects. SetXSpec functions replace the entire spec at once.
package metallb
