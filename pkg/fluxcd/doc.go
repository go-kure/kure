// Package fluxcd exposes helper functions for constructing Flux CD
// resources. The builders return typed controller-runtime objects that
// can be used directly or embedded into layout generation routines.
//
// Example usage:
//
//	ks := fluxcd.NewKustomization("app", "flux-system", kustv1.KustomizationSpec{})
//	repo := fluxcd.NewOCIRepositoryYAML(cfg)
//
// These helpers mirror the constructors found under internal/fluxcd
// and are intended for consumers of the library.
package fluxcd
