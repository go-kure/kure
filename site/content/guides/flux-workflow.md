+++
title = "Generating Flux Manifests"
weight = 20
+++

This guide walks through the complete workflow for generating a GitOps repository structure with Flux resources using Kure.

## Overview

The workflow has four stages:

1. **Define** the cluster topology using the domain model
2. **Select** the Flux workflow engine
3. **Generate** Flux resources and directory layout
4. **Write** manifests to disk

The code on this page imports:

<!-- doc-example:excerpt the import block alone, which the examples below share -->
```go
import (
    "github.com/go-kure/kure/pkg/stack"
    "github.com/go-kure/kure/pkg/stack/fluxcd"
    "github.com/go-kure/kure/pkg/stack/layout"
)
```

Every other Go block on this page is the body of an `Example` function in
`flux_workflow_example_test.go` in `pkg/stack/fluxcd`, which `go test` runs. The task pages listed
under [Where things went](#where-things-went) use the same imports and helpers. Their blocks are
`Example` bodies too, except a few that only set a field and are marked as excerpts, and the
Kustomization-name, bootstrap and sync-name blocks, which the
[Flux Engine reference](/api-reference/flux-engine) shares and which live in its `example_test.go`.
Besides the imports above the examples use `os`, `kustv1`
(`github.com/fluxcd/kustomize-controller/api/v1`), and `fmt` for the lines that print what the
example built. The test file declares what the examples take as given: `certManagerConfig`,
`frontendConfig` and `apiConfig`, which each emit a Deployment and a Service named after their
application; `productionCluster()` and `productionLayout()`, the cluster Step 1 builds and the
layout Step 3 creates; and `printFiles(dir)`, which prints every file below `dir`.

## Step 1: Define the Cluster

Define your cluster's structure. A cluster has one root node, a node holds at most one bundle, and
a bundle holds applications, so the two groups here are two child nodes of the root, each with a
bundle of its own:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowDefine -->
```go
// The Flux source every bundle's Kustomization reads from.
source := &stack.SourceRef{Kind: "GitRepository", Name: "flux-system"}

certManager, err := stack.NewBundle("cert-manager", []*stack.Application{
    stack.NewApplication("cert-manager", "cert-manager", certManagerConfig),
}, nil)
if err != nil {
    panic(err)
}
certManager.SourceRef = source
webTier, err := stack.NewBundle("web-tier", []*stack.Application{
    stack.NewApplication("frontend", "web", frontendConfig),
    stack.NewApplication("api-gateway", "web", apiConfig),
}, nil)
if err != nil {
    panic(err)
}
webTier.SourceRef = source

cluster := stack.NewCluster("production", &stack.Node{
    Name: "production",
    Children: []*stack.Node{
        {Name: "infrastructure", Bundle: certManager},
        {Name: "applications", Bundle: webTier},
    },
})
for _, node := range cluster.Node.Children {
    b := node.Bundle
    fmt.Println(node.Name, b.Name, len(b.Applications), b.SourceRef.Kind, b.SourceRef.Name)
}
```
<!-- doc-example:end -->

Each bundle becomes a Flux Kustomization, and each application generates its Kubernetes manifests.
Each bundle needs a `SourceRef` naming the Flux source its Kustomization reads from: `FluxSeparate`,
the placement Step 3 uses, does not enforce one, and without it the Kustomizations written to
`flux-system/` carry an empty `sourceRef`, which Flux's CRD rejects.

Names are checked when the cluster is validated, which the layout walk of Step 3 does before it
renders anything. A bundle name must be a DNS-1123 subdomain: lower-case letters, digits, `-` and
`.`, starting and ending with a letter or digit (`my.app` is valid, `My-App` is not). A node name
must be one path segment: not empty, not `.` or `..`, and without `/`, `\` or a NUL byte; only the
root node may be unnamed. In the Flux workflow a bundle name also becomes the name of its Flux
Kustomization, so it must be at most 63 characters: Flux writes the name into a label value on
every object it applies. That limit is Flux's, so cluster validation does not apply it; the Flux
generator refuses a longer name when it builds the Kustomization, which is still before anything
is written. A bundle that sets `KustomizationName` gives its Kustomization that name instead: the
value must be a DNS-1123 subdomain as well, and the 63-character limit then applies to it and not
to the bundle name. A refused name is reported with its node or bundle path, and Kure never shortens one.
See the [Stack reference](/api-reference/stack/) for the rules in full.

The fluent builder (`stack.NewClusterBuilder`) builds a single path from the root, not a tree
like this one: `WithNode` sets the root node, so a second call replaces it; a second `WithBundle`
on one node replaces the first; and `WithChild` descends into the child it adds, with no way back
to its parent. Build a tree with sibling nodes from `stack.Node` values, as above.

## Step 2: Create the Flux Engine

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowEngine -->
```go
engine := fluxcd.Engine()
fmt.Println(engine.GetName())
```
<!-- doc-example:end -->

Placement is configured per call on `layout.LayoutRules.FluxPlacement` (see Step 3
below). `FluxUnset` normalizes to `FluxSeparate`. See the
[Flux Engine reference](/api-reference/flux-engine) for configuration options.

## Step 3: Generate Resources with Layout

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowLayout -->
```go
cluster := productionCluster()
engine := fluxcd.Engine()

// Define layout rules
rules := layout.LayoutRules{
    NodeGrouping:        layout.GroupByName,
    BundleGrouping:      layout.GroupByName,
    ApplicationGrouping: layout.GroupByName,
    FilePer:             layout.FilePerResource,
    FluxPlacement:       layout.FluxSeparate, // Flux resources in separate tree
}

// Generate layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)
if err != nil {
    panic(err)
}
for _, child := range ml.(*layout.ManifestLayout).Children {
    fmt.Println(child.FullRepoPath())
}
```
<!-- doc-example:end -->

`CreateLayoutWithResources` walks the cluster and calls each application's `Generate` once. What
`Generate` returns belongs to the caller, and kure writes into it: here, an application's
delivery annotations. (`Bundle.Generate`, which the walk does not call, adds the bundle's labels
and annotations.) A config
therefore returns fresh objects on every call, or deep copies of a cached result. Each entry
point that renders applications (`GenerateFromCluster`, `CreateLayoutWithResources`) walks
again, so validating with one and laying out with another generates twice; to generate once,
walk once with `layout.WalkCluster` and pass the layout to `IntegrateWithLayout`. A wrapper config forwards `Validator`
and the layout augmenter interfaces, or loses them. See [Stack](/api-reference/stack/) §
Generation contract.

The layout is yours to edit before writing it. Every Kustomization in it holds copies of its
bundle's labels and annotations, so an edit there leaves the cluster's bundles, and what the next
generation from them produces, as they were. The applications' objects are the ones their configs
returned, passed through without copying; an entry that is nil or holds no object (a nil
`client.Object`, or a nil pointer such as `(*corev1.ConfigMap)(nil)`) holds nothing to write and
is skipped. That holds for what an application returns. A nil you leave in the record of a bundle's
emitted objects (the slice `OriginBundleObjects` returns) is not skipped: `GenerateFromLayout`
refuses it with an error that names the layout, the bundle and the type. That slice is the
layout's own record, so a write to it changes the layout. `OriginBundleObjects(b)` brings up to
date only the stand-ins for the ConfigMaps that bundle `b` emitted when the layout was walked, and
no other bundle's. See the
[Flux Engine reference](/api-reference/flux-engine/) § Resource Generation.

## Step 4: Write to Disk

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowWrite -->
```go
ml := productionLayout()
out, err := os.MkdirTemp("", "kure-flux-workflow")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(out) }()

err = layout.WriteManifest(out, layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
if err != nil {
    panic(err)
}
printFiles(out)
```
<!-- doc-example:end -->

`WriteManifest` writes under `<basePath>/<ManifestsDir>` (`<out>/clusters` here). Every Flux
Kustomization `spec.path` is a layout directory relative to that root, so root the Flux source
there. The example prints every file it wrote, and `go test` checks that list against the
Example's `// Output:` comment. The tree below draws the same files by hand; it is not generated:

```
clusters/
  production/
    kustomization.yaml
    flux-system/                  # FluxSeparate: every Flux Kustomization, one file each
      flux-system-kustomization-cert-manager.yaml
      flux-system-kustomization-web-tier.yaml
      kustomization.yaml
    infrastructure/               # node
      kustomization.yaml
      cert-manager/               # bundle, applied by its Flux Kustomization
        kustomization.yaml
        cert-manager/             # application
          cert-manager-deployment-cert-manager.yaml
          cert-manager-service-cert-manager.yaml
          kustomization.yaml
    applications/
      kustomization.yaml
      web-tier/
        kustomization.yaml
        frontend/
          web-deployment-frontend.yaml
          web-service-frontend.yaml
          kustomization.yaml
        api-gateway/
          web-deployment-api-gateway.yaml
          web-service-api-gateway.yaml
          kustomization.yaml
```

Every `kustomization.yaml`, the one in `flux-system/` included, is a kustomize file, not a Flux
Kustomization: it lists the manifests and subdirectories kustomize builds from its directory. The
Flux Kustomizations are the `flux-system-kustomization-*.yaml` files, which
`flux-system/kustomization.yaml` lists.

Those names are the default pattern, `{namespace}-{kind}-{name}.yaml`. Under `WriteToDisk` and
`WriteToTar`, `LayoutRules.FileNaming` names the files of every layout of the tree, `flux-system/`
and the layouts an augmenter adds included: with `FileNamingKindName` the directory holds
`kustomization-cert-manager.yaml` and `kustomization-web-tier.yaml`. Before go-kure/kure#976 those
two kinds of layout kept the default pattern whatever the rules said, so their files are renamed
for a tree written with `FileNamingKindName`; see the
[Layout Engine reference](/api-reference/layout/#file-naming-modes).


## Where things went

This guide used to hold every Flux topic on one page. The topics after Step 4 now have pages of their own.

| Topic | Page |
|-------|------|
| Layout options, directory and Kustomization names, per-layout settings | [Flux Layout Rules](/guides/flux-layout-rules/) |
| Umbrella bundles and readiness aggregation | [Umbrella Bundles](/guides/flux-umbrella-bundles/) |
| Augmenter-added child layouts | [Augmenters and Delivery Intent](/guides/flux-augmenters-and-delivery/) |
| Delivery intent per application (prune and force) | [Augmenters and Delivery Intent](/guides/flux-augmenters-and-delivery/#delivery-intent-per-application) |
| Bootstrap, sync name and source access | [Bootstrapping Flux](/guides/flux-bootstrap/) |

A link to a section of the old page opens this tutorial. Use the table to find the new page.

## Further Reading

- [Stack](/api-reference/stack) - Domain model reference
- [Flux Engine](/api-reference/flux-engine) - Workflow engine reference
- [Layout Engine](/api-reference/layout) - Directory organization reference
