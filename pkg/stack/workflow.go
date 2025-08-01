package stack

import "sigs.k8s.io/controller-runtime/pkg/client"

// Workflow defines an interface for translating stack objects into
// workflow-specific custom resources such as Flux Kustomizations or
// Argo CD Applications.
type Workflow interface {
	// Cluster converts a Cluster into workflow specific CRDs.
	Cluster(*Cluster) ([]client.Object, error)
	// Node converts a Node into workflow specific CRDs.
	Node(*Node) ([]client.Object, error)
	// Bundle converts a Bundle into workflow specific CRDs.
	Bundle(*Bundle) ([]client.Object, error)
}
