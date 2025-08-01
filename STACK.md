We analyzed the code in modules stack, stack/fluxcd and layout.
We here explain how we get from a Cluster to a fully rendered FluxCD set of yaml deployments.

Overview

Stack model

    A Cluster holds a root Node that represents a hierarchical tree of configuration bundles

Each Node may carry a Bundle, which groups Applications and optional metadata such as source references, dependencies, and reconciliation interval

An Application uses a pluggable ApplicationConfig to generate Kubernetes objects when requested via Generate

FluxCD workflow

    Package stack/fluxcd implements the Workflow interface to convert stack objects into Flux resources.

    Cluster → Node → Bundle are traversed recursively; for each bundle a Flux Kustomization is produced. The repository path for a bundle is derived from its ancestry, intervals and source references are resolved, and DependsOn relationships become Flux dependencies

Layout generation

    WalkCluster in package layout traverses the Cluster tree and builds a ManifestLayout hierarchy, collecting the Kubernetes objects produced by each application. Layout rules determine whether bundles or applications are flattened or nested in the directory tree

WriteManifest renders each ManifestLayout to disk: resources are grouped into files, a kustomization.yaml is generated when needed, and the function recurses into child layouts to build the full repository structure

From Cluster to FluxCD YAML deployments

    Model the cluster – Define a Cluster with a tree of Node and Bundle objects; each bundle aggregates Application instances that know how to generate their Kubernetes resources.

    Render manifests – Run layout.WalkCluster to collect the resources from all applications into a ManifestLayout, then invoke layout.WriteManifest to write those manifests (and local kustomization.yaml files) into the repository structure.

    Create Flux definitions – Execute the Flux workflow (stack/fluxcd.Workflow), which converts each bundle in the tree into a Flux Kustomization pointing at the appropriate path in the generated layout, including any dependency or interval information.

    Write Flux YAML – Treat the Flux Kustomization objects as another ManifestLayout and write them to disk with layout.WriteManifest, producing the FluxCD YAML that references the earlier manifest directories.

    Result – The repository now holds both the Kubernetes resource manifests and the FluxCD Kustomization CRDs, enabling Flux to reconcile the cluster by applying the rendered YAML deployments.
