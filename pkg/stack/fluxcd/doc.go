// Package fluxcd implements the stack.Workflow interface for Flux. It turns
// the domain model (Cluster, Node, Bundle, Application) into Flux resources:
// Kustomizations and their source references, placed into the directories a
// layout writes, and the manifests that bootstrap Flux.
//
// Importing the package for its side effect registers the "flux" workflow
// with the stack package:
//
//	import _ "github.com/go-kure/kure/pkg/stack/fluxcd"
//
// Engine returns a WorkflowEngine with the defaults docs/defaults.md lists.
// The engine has three parts: the resource generator, the layout integrator and
// the bootstrap generator. The package README names the page that documents
// each of them, the names and paths the generated Kustomizations get, and the
// checks that refuse a cluster the engine cannot render.
package fluxcd
