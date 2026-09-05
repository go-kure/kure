// Package fluxcd exposes helper functions for constructing resources used by the
// Flux family of controllers. Each function returns a fully initialized
// controller-runtime object that can be serialized to YAML or modified further by
// the calling application.
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
// Applications covered include sources (GitRepository, OCIRepository,
// HelmRepository, Bucket, ExternalArtifact, ArtifactGenerator), workloads
// (Kustomization, HelmRelease), the notification stack (Provider, Alert,
// Receiver), image automation (ImageUpdateAutomation), and objects from the
// Flux operator (FluxInstance, FluxReport, ResourceSet,
// ResourceSetInputProvider).
//
// # Constructors
//
// Constructors follow the form Create<Kind>(name, namespace string). A minimal example
// creating a Kustomization and a GitRepository looks like:
//
//	repo := fluxcd.CreateGitRepository("app-repo", "flux-system")
//	repo.Spec.URL = "https://github.com/example/app"
//	repo.Spec.Interval = metav1.Duration{Duration: time.Minute}
//	fluxcd.SetGitRepositoryReference(repo, &sourcev1.GitRepositoryRef{Branch: "main"})
//
//	ks := fluxcd.CreateKustomization("app", "flux-system")
//	ks.Spec.SourceRef = kustv1.CrossNamespaceSourceReference{
//	        Kind: "GitRepository",
//	        Name: "app-repo",
//	}
//	ks.Spec.Path = "./deploy"
//	ks.Spec.Prune = true
//
// # Update helpers
//
// Additional functions prefixed with Set or Add expose granular control over
// the generated objects. There is no whole-spec setter: replacing a spec is a
// plain assignment, and the builder contract does not wrap one.
//
//	hr := fluxcd.CreateHelmRelease("my-app", "default")
//	hr.Spec = helmv2.HelmReleaseSpec{Chart: chart}
package fluxcd
