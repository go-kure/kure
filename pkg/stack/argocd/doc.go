// Package argocd implements the stack.Workflow interface for Argo CD. It
// generates one Argo CD Application (argoproj.io/v1alpha1) for each directory
// of the written layout that renders bundles, as unstructured objects, and
// places them in an argocd/ directory inside the root layout.
//
// Importing the package for its side effect registers the "argocd" workflow
// with the stack package:
//
//	import _ "github.com/go-kure/kure/pkg/stack/argocd"
//
// Engine returns an engine whose RepoURL is the placeholder
// https://github.com/example/manifests.git; set the real repository with
// SetRepoURL before generating.
//
// Bootstrap is not implemented: GenerateBootstrap returns an error when the
// configuration is enabled. The Flux engine in package fluxcd generates
// bootstrap manifests.
package argocd
