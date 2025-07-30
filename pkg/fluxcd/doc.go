// Package fluxcd exposes helper functions for constructing resources used by the
// Flux family of controllers.  Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified further by
// the calling application.
//
// ## Overview
//
// The package mirrors the constructors and setters found under
// `internal/fluxcd` so applications can build Flux manifests programmatically
// without depending on the internal packages directly.  All builders operate on
// configuration structures defined in this package and convert them to the
// appropriate Flux custom resources.
//
// Applications covered include sources (`GitRepository`, `OCIRepository`,
// `HelmRepository`, `Bucket`), workloads (`Kustomization`, `HelmRelease`), the
// notification stack (`Provider`, `Alert`, `Receiver`), and objects from the
// Flux operator (`FluxInstance`, `FluxReport`, `ResourceSet`,
// `ResourceSetInputProvider`).
//
// ## Constructors
//
// Constructors follow the form `New<Type>` and accept a configuration struct. A
// minimal example creating a `Kustomization` and a `GitRepository` looks like:
//
//	ks := fluxcd.NewKustomization(&fluxcd.KustomizationConfig{
//	        Name:      "app",
//	        Namespace: "flux-system",
//	        Interval:  "1m",
//	        SourceRef: kustv1.CrossNamespaceSourceReference{
//	                Kind: "GitRepository",
//	                Name: "app-repo",
//	        },
//	})
//
//	repo := fluxcd.NewGitRepository(&fluxcd.GitRepositoryConfig{
//	        Name:      "app-repo",
//	        Namespace: "flux-system",
//	        URL:       "https://github.com/example/app",
//	        Interval:  "1m",
//	        Ref:       "main",
//	})
//
// These helpers allocate and populate the returned objects so callers only need
// to adjust fields that are not part of the config struct.  All objects can be
// passed directly to a YAML printer or added to other manifests.
//
// ## Update helpers
//
// Additional functions prefixed with `Set` or `Add` expose granular control over
// the generated objects.  They delegate to the internal package to perform the
// actual mutations while keeping the public API stable.  For example:
//
//	hr := fluxcd.NewHelmRelease(&fluxcd.HelmReleaseConfig{...})
//	fluxcd.SetHelmReleaseSpec(hr, helmv2.HelmReleaseSpec{Chart: chart})
//
// Using these setters avoids dealing with the nested fields of the underlying
// structs and provides basic error checking where appropriate.
//
// This package aims to provide a convenient typed interface for applications
// that need to generate Flux CD manifests at runtime or as part of a build
// pipeline.
package fluxcd
