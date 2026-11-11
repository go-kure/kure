+++
title = "Deliver with Flux"
weight = 2
+++

Turn a cluster definition into a GitOps repository that Flux reconciles. Read the tutorial first,
then the page for the task in front of you.

## Guides

- [Generating Flux Manifests](/guides/flux-workflow/) - The tutorial: define, select the engine, generate, write
- [Flux Layout Rules](/guides/flux-layout-rules/) - Directory and Kustomization names, per-layout settings
- [Umbrella Bundles](/guides/flux-umbrella-bundles/) - Readiness aggregation over child Kustomizations
- [Augmenters and Delivery Intent](/guides/flux-augmenters-and-delivery/) - Child layouts and per-application prune and force
- [Bootstrapping Flux](/guides/flux-bootstrap/) - Flux system manifests, sync name and source access

## Concepts

- [Layout and Flux Naming](/concepts/layout-and-flux-naming/) - How directories, files and Flux objects are named
- [OCI Artifact Layout](/concepts/oci-layout/) - The directory shape of an OCI artifact and what Kure does to produce it

## Reference

- [Flux Engine](/api-reference/flux-engine/) - The engine overview, with the pages below it
- [Flux Names and Paths](/api-reference/flux-names-and-paths/)
- [Flux Layout Integration](/api-reference/flux-layout-integration/)
- [Flux Bootstrap Generation](/api-reference/flux-bootstrap-generation/)
- [Flux Umbrella Bundles](/api-reference/flux-umbrella-bundles/)
- [Flux Non-Bundle Child Layout CRs](/api-reference/flux-child-layout-crs/)
- [Flux Defaults](/api-reference/flux-defaults/)
- [Layout Engine](/api-reference/layout/) - Manifest directory organization
