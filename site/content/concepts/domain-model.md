+++
title = "Domain Model"
weight = 20
+++

# Domain Model

Kure models Kubernetes infrastructure as a four-level hierarchy. Each level maps to a concept in GitOps deployment workflows.

## The Hierarchy

```
Cluster
  └── Node (tree structure)
        └── Bundle (deployment unit)
              └── Application (workload)
```

### Cluster

The root of the hierarchy, representing a target Kubernetes cluster. A cluster has a name, a tree of nodes, and GitOps configuration specifying which workflow engine to use (Flux or ArgoCD). That configuration can also carry a bootstrap: how Flux itself is installed (in `gotk` mode, from the flux2 release Kure vendors unless `FluxVersion` names another) and which source it syncs from, including — for flux-operator — the name of the sync source and Kustomization the operator creates (`BootstrapConfig.SyncName`).

### Node

An organizational grouping within a cluster. Nodes form a tree structure — for example, a cluster might have top-level nodes for `infrastructure` and `applications`, each with child nodes for specific concerns.

Nodes map to **directory structures** in the GitOps repository. Each node can also reference a source (Git repository, OCI registry, S3 bucket) for multi-source deployments.

`stack.ValidateCluster` rejects a cycle in the node graph (a `Node` reached again from one of its own descendants), naming the node where the cycle closes. It does not reject a node shared by two parents, but such a node is walked once under each parent, and Flux and ArgoCD generation refuse the result (`layout.IndexOrigins`: a node or bundle rendered by two layouts has no single path) — keep the graph a tree.

### Bundle

A deployment unit corresponding to a single GitOps reconciliation resource (e.g., a Flux Kustomization or ArgoCD Application). Bundles contain applications and support:

- **Dependency ordering** via `DependsOn` (pointer-based, in-scope bundles) or `NamedDependsOn` (name-based, for cross-scope or hook-phase chains)
- **Umbrella composition** via `Children` (see below)
- **Reconciliation settings**: interval, pruning, timeouts. `Interval`, `Timeout` and `RetryInterval` are Go duration strings (`"10m"`); empty means the default, and a value that does not parse is a validation error rather than a silent fallback
- **Labels and annotations** for metadata

#### Umbrella Bundles

`DependsOn` and `Children` answer different questions. `DependsOn` is about
**ordering** between siblings: "reconcile X only after Y is Ready".
`Children` is about **containment**: the parent Bundle becomes an umbrella
whose Flux Kustomization renders its child Kustomization CRs and aggregates
each child's Ready condition via `spec.healthChecks`. The parent is Ready iff
all children are Ready. `spec.wait` is left to `Bundle.Wait` — upstream ignores
`healthChecks` when `wait` is enabled, so forcing it would defeat the
aggregation.

This gives downstream consumers a single stable anchor — for example, a
`platform` umbrella with tier children `infra`, `services`, `apps` lets an
external bundle `DependsOn: [platform]` without having to know or care about
the internal tiers.

A bundle referenced by `Children` must be **standalone**: it cannot
simultaneously be the `Bundle` of any `Node`. The `stack.ValidateCluster`
check enforces disjointness, cycle detection, and rejects multi-package
umbrellas in the current release.

### Application

An individual Kubernetes workload or resource set. Applications implement the `ApplicationConfig` interface, which defines how to generate Kubernetes resource objects.

## Fluent Builder API

For ergonomic construction of a single path through the tree:

<!-- doc-example: pkg/stack ExampleNewClusterBuilder -->
```go
appConfig := &myConfig{Port: 9090}

cluster, err := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
    WithBundle("monitoring").
    WithApplication("prometheus", appConfig).
    End().
    End().
    Build()
if err != nil {
    panic(err)
}
fmt.Println(cluster.Name, cluster.Node.Name, cluster.Node.Bundle.Name)
```
<!-- doc-example:end -->

`Build` returns the cluster and an error collecting whatever the chain rejected. The builder
follows one path: `WithNode` sets the cluster's root node (a second call replaces it), a node holds
one bundle (a second `WithBundle` on the same node replaces the first), and `WithChild` descends
into a new child node with no way back to its parent. A tree with sibling nodes is written as a
struct literal instead.

Both Go blocks on this page are the bodies of `Example` functions in `pkg/stack` that `go test`
runs: the builder above is `ExampleNewClusterBuilder` (`example_test.go`, where `myConfig` is a
small `ApplicationConfig`), and the tree below is `Example_domainModel`
(`domain_model_example_test.go`, which declares the four `*Config` values and `printLayout`, a
helper printing each directory's repository path). They import `github.com/go-kure/kure/pkg/stack`
and `github.com/go-kure/kure/pkg/stack/layout`.

## How It Maps to GitOps

The domain model maps directly to a GitOps repository structure. This cluster puts each
infrastructure bundle on a node of its own, because a node carries at most one bundle, and walks
it with every level grouped by name:

<!-- doc-example: pkg/stack Example_domainModel -->
```go
cluster := stack.NewCluster("production", &stack.Node{
    Name: "production",
    Children: []*stack.Node{
        {Name: "infrastructure", Children: []*stack.Node{
            {Name: "cert-manager", Bundle: &stack.Bundle{Name: "cert-manager", Applications: []*stack.Application{
                stack.NewApplication("cert-manager", "cert-manager", certManagerConfig),
            }}},
            {Name: "monitoring", Bundle: &stack.Bundle{Name: "monitoring", Applications: []*stack.Application{
                stack.NewApplication("prometheus", "monitoring", prometheusConfig),
            }}},
        }},
        {Name: "applications", Bundle: &stack.Bundle{Name: "web-apps", Applications: []*stack.Application{
            stack.NewApplication("frontend", "web", frontendConfig),
            stack.NewApplication("api", "web", apiConfig),
        }}},
    },
})

rules := layout.DefaultLayoutRules()
rules.BundleGrouping = layout.GroupByName
rules.ApplicationGrouping = layout.GroupByName
ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    panic(err)
}
printLayout(ml)
```
<!-- doc-example:end -->

Written under the default `clusters/` manifests directory, that is:

```
clusters/
  production/                    # Cluster's root Node
    infrastructure/              # Node
      cert-manager/              # Node
        cert-manager/            # Bundle → Flux Kustomization
          cert-manager/          # Application → K8s manifests
      monitoring/                # Node
        monitoring/              # Bundle → Flux Kustomization
          prometheus/            # Application → K8s manifests
    applications/                # Node
      web-apps/                  # Bundle → Flux Kustomization
        frontend/                # Application → K8s manifests
        api/                     # Application → K8s manifests
```

The [Layout Engine](/api-reference/layout) handles this mapping, and the [Flux Engine](/api-reference/flux-engine) generates the corresponding Flux Kustomization resources — by default in a separate `flux-system` directory, not beside each bundle.

## Further Reading

- [Stack package reference](/api-reference/stack) for API details
- [Flux workflow guide](/guides/flux-workflow) for end-to-end usage
- [Architecture](/concepts/architecture/) for system-level design
