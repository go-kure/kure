// Package fluxcd exposes helper functions for constructing resources used by the
// Flux family of controllers. Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified further by
// the calling application.
//
// # Overview
//
// The package provides two layers of API:
//
//   - CreateX constructors: allocate an object with TypeMeta and ObjectMeta set,
//     leaving the spec empty. Use the Set/Add helpers to populate it.
//   - SetX/AddX helpers: granular setters that mutate a single field on an existing
//     object, keeping callers in control of which fields are set.
//
// Applications covered include sources (GitRepository, OCIRepository,
// HelmRepository, Bucket, ExternalArtifact), workloads (Kustomization,
// HelmRelease), the notification stack (Provider, Alert, Receiver), image
// automation (ImageUpdateAutomation), and objects from the Flux operator
// (FluxInstance, FluxReport, ResourceSet, ResourceSetInputProvider).
//
// # Constructors
//
// Constructors follow the form CreateX(name, namespace string). A minimal example
// creating a Kustomization and a GitRepository looks like:
//
//	repo := fluxcd.CreateGitRepository("app-repo", "flux-system")
//	fluxcd.SetGitRepositoryURL(repo, "https://github.com/example/app")
//	fluxcd.SetGitRepositoryInterval(repo, metav1.Duration{Duration: time.Minute})
//	fluxcd.SetGitRepositoryReference(repo, &sourcev1.GitRepositoryRef{Branch: "main"})
//
//	ks := fluxcd.CreateKustomization("app", "flux-system")
//	fluxcd.SetKustomizationSourceRef(ks, kustv1.CrossNamespaceSourceReference{
//	        Kind: "GitRepository",
//	        Name: "app-repo",
//	})
//	fluxcd.SetKustomizationPath(ks, "./deploy")
//	fluxcd.SetKustomizationPrune(ks, true)
//
// # Update helpers
//
// Additional functions prefixed with Set or Add expose granular control over
// the generated objects. For coarse-grained replacement, SetXSpec functions
// replace the entire spec at once:
//
//	hr := fluxcd.CreateHelmRelease("my-app", "default")
//	fluxcd.SetHelmReleaseSpec(hr, helmv2.HelmReleaseSpec{Chart: chart})
package fluxcd
