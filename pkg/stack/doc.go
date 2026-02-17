// Package stack provides the core domain model for defining and generating
// Kubernetes cluster configurations with GitOps tooling (Flux CD or ArgoCD).
//
// # Overview
//
// The stack package models a Kubernetes cluster as a hierarchical tree of
// nodes, where each node can contain bundles of applications. This structure
// maps directly to the directory layouts expected by GitOps tools, enabling
// declarative generation of the complete repository structure needed for
// Flux Kustomizations or ArgoCD Applications.
//
// # Domain Model
//
// The core types form a hierarchical structure:
//
//		Cluster
//		  └── Node (tree structure)
//		        ├── Bundle
//		        │     └── Applications
//		        └── Children (nested Nodes)
//
//	  - [Cluster]: Top-level configuration including GitOps settings
//	  - [Node]: Hierarchical structure for organizing deployment units
//	  - [Bundle]: Collection of applications deployed together
//	  - [Application]: Individual Kubernetes workload or component
//
// # Fluent Builder API
//
// The package provides a fluent builder API for constructing cluster
// configurations in a type-safe, readable manner:
//
//	cluster, err := stack.NewClusterBuilder("production").
//		WithGitOps(&stack.GitOpsConfig{Type: "flux"}).
//		WithNode("infrastructure").
//			WithBundle("monitoring").
//				WithApplication("prometheus", prometheusConfig).
//				End().
//			End().
//		Build()
//
// The fluent API uses a copy-on-write pattern where each method returns a new
// builder instance, allowing safe branching and concurrent construction.
// Build() returns (*Cluster, error) to surface any validation errors.
//
// # Workflow Integration
//
// The [Workflow] interface abstracts the generation of GitOps-specific
// resources. Implementations exist for both Flux CD and ArgoCD:
//
//   - [github.com/go-kure/kure/pkg/stack/fluxcd.FluxWorkflow]: Generates
//     Flux Kustomizations, GitRepositories, and related resources
//   - [github.com/go-kure/kure/pkg/stack/argocd.ArgoCDWorkflow]: Generates
//     ArgoCD Applications and AppProjects
//
// Use the workflow to generate all manifests for a cluster:
//
//	workflow := fluxcd.NewFluxWorkflow()
//	manifests, err := workflow.Generate(cluster)
//
// # Layout Generation
//
// The [github.com/go-kure/kure/pkg/stack/layout] subpackage handles writing
// the generated manifests to disk following the conventions expected by
// GitOps tools.
//
// # Package References
//
// Nodes can specify a [PackageRef] to indicate that a subtree should be
// packaged as a separate OCI artifact or kurel package. When undefined,
// the PackageRef is inherited from the parent node.
//
// # Example
//
// Complete example creating a cluster with infrastructure and applications:
//
//	// Define the cluster structure
//	cluster, err := stack.NewClusterBuilder("prod-cluster").
//		WithGitOps(&stack.GitOpsConfig{
//			Type: "flux",
//			Bootstrap: &stack.BootstrapConfig{
//				Enabled:   true,
//				FluxMode:  "gitops-toolkit",
//			},
//		}).
//		WithNode("infrastructure").
//			WithBundle("cert-manager").
//				WithApplication("cert-manager", certManagerConfig).
//				End().
//			WithChild("applications").
//				WithBundle("web-app").
//					WithApplication("frontend", frontendConfig).
//					WithApplication("backend", backendConfig).
//					End().
//				End().
//			End().
//		Build()
//
//	// Generate Flux manifests
//	workflow := fluxcd.NewFluxWorkflow()
//	manifests, _ := workflow.Generate(cluster)
//
//	// Write to disk using layout package
//	layout.WriteCluster(cluster, "./clusters/prod", manifests)
package stack
