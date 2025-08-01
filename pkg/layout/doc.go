/*
Package layout provides utilities for generating cluster directory layouts
and for writing Kubernetes and Flux manifests to disk.

Flux Kustomizations and ArgoCD Applications reference directories in a Git
repository using different fields. Flux uses `spec.path`, which must start
with `./` and is always interpreted relative to the repository root. ArgoCD
uses `spec.source.path` without the `./` prefix but with the same relative
semantics.

When nodes or bundles live in nested subfolders, the path must point directly
to the folder containing the manifests unless the directory tree only contains
files for a single node or bundle. Flux will recursively auto-generate a
`kustomization.yaml` when one is missing and include every manifest under the
specified path. ArgoCD does not auto-generate a `kustomization.yaml` and
therefore ignores nested directories unless they are referenced from a
`kustomization.yaml` at the target path.

For example, consider the layout:

	repo/
	  clusters/
	    prod/
	      nodes/
	        cp/
	          kustomization.yaml
	      bundles/
	        monitoring/
	          kustomization.yaml

The Flux Kustomization for the control-plane node uses:

	spec.path: ./clusters/prod/nodes/cp

The equivalent ArgoCD Application uses:

	spec.source.path: clusters/prod/nodes/cp

With this layout, each node or bundle is targeted individually. Pointing a Flux
Kustomization at `./clusters/prod` would combine the `cp` and `monitoring`
manifests into a single deployment because it would auto-generate a
`kustomization.yaml` for the entire tree. ArgoCD will only process the
manifests under `clusters/prod` itself unless a `kustomization.yaml` aggregates
the subdirectories, so each subfolder must be referenced separately.
*/
package layout
