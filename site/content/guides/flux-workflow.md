+++
title = "Generating Flux Manifests"
weight = 20
+++

# Generating Flux Manifests

This guide walks through the complete workflow for generating a GitOps repository structure with Flux resources using Kure.

## Overview

The workflow has four stages:

1. **Define** the cluster topology using the domain model
2. **Select** the Flux workflow engine
3. **Generate** Flux resources and directory layout
4. **Write** manifests to disk

## Step 1: Define the Cluster

Use the fluent builder to define your cluster's structure:

```go
import "github.com/go-kure/kure/pkg/stack"

cluster := stack.NewClusterBuilder("production").
    WithNode("infrastructure").
        WithBundle("cert-manager").
            WithApplication("cert-manager", certManagerConfig).
        End().
    End().
    WithNode("applications").
        WithBundle("web-tier").
            WithApplication("frontend", frontendConfig).
            WithApplication("api-gateway", apiConfig).
        End().
    End().
    Build()
```

Each bundle becomes a Flux Kustomization, and each application generates its Kubernetes manifests.

## Step 2: Create the Flux Engine

```go
import (
    "github.com/go-kure/kure/pkg/stack/fluxcd"
    "github.com/go-kure/kure/pkg/stack/layout"
)

engine := fluxcd.Engine()
```

Placement is configured per call on `layout.LayoutRules.FluxPlacement` (see Step 3
below). `FluxUnset` normalizes to `FluxSeparate`. See the
[Flux Engine reference](/api-reference/flux-engine) for configuration options.

## Step 3: Generate Resources with Layout

```go
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
    return errors.Wrap(err, "failed to create layout")
}
```

## Step 4: Write to Disk

```go
err := layout.WriteManifest("./out", layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
```

`WriteManifest` writes under `<basePath>/<ManifestsDir>` (`./out/clusters` here). Every Flux
Kustomization `spec.path` is a layout directory relative to that root, so root the Flux source
there. This produces a directory structure like:

```
clusters/
  production/
    infrastructure/
      cert-manager/
        cert-manager/
          deployment.yaml
          service.yaml
          kustomization.yaml
        kustomization.yaml        # Flux Kustomization
    applications/
      web-tier/
        frontend/
          deployment.yaml
          service.yaml
        api-gateway/
          deployment.yaml
          service.yaml
        kustomization.yaml        # Flux Kustomization
```

## Layout Configuration

The [Layout Engine](/api-reference/layout) supports multiple grouping and file organization strategies:

| Option | Values | Effect |
|--------|--------|--------|
| NodeGrouping | `GroupByName`, `GroupFlat` | A directory per child node, or merge child nodes into their parent |
| BundleGrouping | `GroupByName`, `GroupFlat` | A directory per bundle, or render bundles in their node's directory |
| ApplicationGrouping | `GroupByName`, `GroupFlat` | A directory per app, or write apps into their bundle's directory |
| FilePer | `FilePerResource`, `FilePerKind` | One file per resource or group by kind |
| FluxPlacement | `FluxSeparate`, `FluxIntegratedPerLayout`, `FluxIntegratedPerBundle` | Separate dir; a Flux CR per layout node; or Flux CRs at bundle boundaries with children as directories |

The three grouping axes are independent; umbrella child bundles and augmenter applications always get
a directory of their own.

### Layout paths (breaking change in go-kure/kure#771)

A layout's directory is `FullRepoPath()`, which is `Namespace` joined with `Name`. `Namespace` is
always the **parent's** directory, and `"."` is the root of the tree. The walkers build every layout
this way. If you build `ManifestLayout` trees by hand, or add child layouts to a walked tree, set each
child's `Namespace` to its parent's `FullRepoPath()`.

What changed, and what to do:

- **Hand-built children.** `FullRepoPath()` used to drop `Name` when `Namespace` already ended with
  it. A caller that set a child's `Namespace` to the full path, including the child's own `Name`
  (`Namespace: "apps/web", Name: "web"`), now gets that name twice (`apps/web/web`). Pass the
  parent's path instead (`Namespace: "apps"`).
- **Same-name and suffix-sharing layouts nest.** A bundle and an application with the same name
  now give `web/web`. Before, they collapsed onto `web/`, and one of the two `kustomization.yaml`
  files replaced the other. The same applies to a name that merely ends another as a string: `oo`
  under `foo` now gives `foo/oo`.
- **No `ClusterName`, named root.** The root node now sits at `<root>` instead of `cluster/<root>`,
  and its child nodes stay at `<root>/<child>`. With `FluxSeparate`, the `flux-system/` layout now
  sits inside the root's directory, at `<root>/flux-system`, where the root's `kustomization.yaml`
  references it. Before, it was written beside the root, and that reference dangled. The ArgoCD
  `argocd/` layout moved the same way. Point anything outside kure that reads these directories
  (CI scripts, a hand-written sync path) at the new paths.
- **No `ClusterName`, unnamed root.** The root and its child nodes stay at `cluster/` and
  `cluster/<child>`. Deeper descendants move: a grandchild used to be written at
  `<child>/<grandchild>`, outside `cluster/` and referenced by nothing. It now sits at
  `cluster/<child>/<grandchild>`.
- **With a `ClusterName`.** Most output is unchanged, and `flux-system/` stays at
  `<ClusterName>/flux-system`. When the last segment of `ClusterName` is the root node's name
  (`ClusterName: "platform"`, root `platform`), the root is still the cluster directory. When
  `ClusterName` only ends with the root's name as a string (`ClusterName: "myprod"`, root `prod`),
  the old rule collapsed the root onto `myprod`. It now nests at `myprod/prod`, so move any
  external sync path that pointed at `myprod`.
- **Collisions are refused.** `WriteToDisk`, `WriteToTar` and `WriteManifest` check the whole tree
  before writing anything. They refuse two layouts that resolve to the same directory (compared
  case-insensitively), and two `AppFileSingle` layouts that resolve to the same file.

See the [Layout Engine reference](/api-reference/layout/) for the full rule.

### Flux paths come from the layout (breaking change)

Every Flux Kustomization `spec.path` (and ArgoCD Application `source.path`) is now the directory of
the layout that renders the bundle, taken from the walked layout tree. Before, the generator
derived it from bundle names alone, so a node-level bundle's path was just its name and pointed at
a directory the layout never wrote whenever a node and its bundle were named differently, or a
`ClusterName` prefixed the tree. See
[Kustomization paths](/api-reference/flux-engine/#kustomization-paths) for the rule.

What changed, and what to do:

- **Removed APIs.** `GenerateFromBundle`, `GenerateFromNode`, `EngineWithMode`, `EngineWithConfig`,
  `NewWorkflowEngineWithConfig`, `SetKustomizationMode` and `ResourceGenerator.Mode` are gone. <!-- doc-api-refs:ignore removed in this release -->
  Generate from a walked layout (`ResourceGenerator.GenerateFromLayout`) or for one bundle at a path
  you supply (`GenerateForBundle`). `GenerateFromCluster` stays and uses the directories of a
  default-rules walk.
- **Integrate walked layouts only.** `IntegrateWithLayout` refuses a layout `layout.WalkCluster`
  did not build from the same cluster — build the tree with `WalkCluster` instead of by hand.
- **PerLayout hosts.** Under `FluxIntegratedPerLayout` a node bundle's CR now sits in the parent of
  the layout that renders it, like every other PerLayout CR, and the parent's `kustomization.yaml`
  lists it as a resource file. The writers no longer emit a `flux-system-kustomization-<child>.yaml`
  reference guessed from a child's name. A bundle-less node layout gets its own CR named
  `<path, "/" → "-">-node`.
- **Refusals.** Two bundles with one name, a CR name used twice, a node or bundle layout written as
  `AppFileSingle`, and a directory holding two objects with one identity are errors.
- **Grouping axes.** `NodeGrouping`, `BundleGrouping` and `ApplicationGrouping` are independent;
  they used to take effect only when bundles and applications were both flat, and a `ClusterName`
  always flattened the root bundle. Combinations that were silently rendered fully nested now
  render as configured, and a merged node's umbrella children and augmenter layouts keep their own
  directories and CRs.
- **FlattenSingleTier** no longer rewrites Flux CRs a caller added to the tree; generated CRs
  already name the surviving directory.

## Umbrella Bundles — Readiness Aggregation

A bundle with non-empty `Children` becomes an **umbrella**: Flux will only mark
its Kustomization `Ready` once every child Kustomization is `Ready`. The Flux
engine enforces this by prepending an auto `spec.healthChecks` entry for each
direct child.

`spec.wait` is not forced. It is `Bundle.Wait` like any other input, and leaving
it unset is what makes the `healthChecks` entries take effect — upstream ignores
`healthChecks` when `wait` is enabled.

The resulting umbrella Kustomization aggregates child readiness regardless of
how many children there are, giving external consumers a single stable anchor:

```go
umbrella := &stack.Bundle{
    Name: "platform",
    Children: []*stack.Bundle{
        {Name: "platform-infra"},
        {Name: "platform-services"},
        {Name: "platform-apps"},
    },
}
```

```yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: platform
  namespace: flux-system
spec:
  prune: false
  healthChecks:
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-infra
    namespace: flux-system
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-services
    namespace: flux-system
  - apiVersion: kustomize.toolkit.fluxcd.io/v1
    kind: Kustomization
    name: platform-apps
    namespace: flux-system
  # ...rest of spec
```

User-supplied `HealthChecks` on the umbrella bundle are appended AFTER the
auto entries. `Wait` on an umbrella is the caller's choice like on any other
bundle: unset and `Wait: false` emit the same `spec`, and the umbrella still
becomes `Ready` only when every child is, through those health checks.

Umbrella children must be **standalone** — a bundle cannot simultaneously be
the `Bundle` of a `stack.Node` and appear in another bundle's `Children`.
`stack.ValidateCluster` rejects any such overlap before resource generation.

### Disk layout

In `FluxIntegratedPerLayout` placement, each Flux Kustomization CR sits in the
parent of the directory it applies, and that parent's `kustomization.yaml`
lists the CR file (not the child subdirectory). With `GroupByName` bundles:

```
clusters/production/apps/                         # node directory
  flux-system-kustomization-platform.yaml         # umbrella self CR (healthChecks), spec.path: .../apps/platform
  kustomization.yaml                              # lists the platform CR file
  platform/                                       # umbrella bundle directory
    flux-system-kustomization-platform-infra.yaml # child CR (placed at parent), spec.path: .../platform/platform-infra
    flux-system-kustomization-platform-apps.yaml  # child CR (placed at parent)
    kustomization.yaml                            # lists the child CR files
    platform-infra/                               # umbrella child subdirectory
      workload-*.yaml
      kustomization.yaml                          # workloads only, no Flux CRs
    platform-apps/
      workload-*.yaml
      kustomization.yaml
```

The child subdirectories contain **only** their workload manifests and a
per-directory `kustomization.yaml` listing those workloads. They do **not**
contain any `flux-system-kustomization-*.yaml` files — those live in the
parent directory, so Flux applies them once via the parent's Kustomization.

In `FluxSeparate` placement, all Kustomization CRs (the umbrella's own plus
every descendant) are written to the shared `flux-system/` directory as a
flat list.

## Augmenter-Added Child Layouts

A `LayoutAugmenter` can attach sub-`ManifestLayout` children to a per-app layout after resource generation. In `FluxIntegratedPerLayout` mode kure automatically emits a Flux `Kustomization` CR for each eligible child.

### Eligibility

A child layout receives a CR when:
- `!child.UmbrellaChild`
- `child.ApplicationFileMode != AppFileSingle`
- it renders no bundle (a child that does already has that bundle's CR in the parent)
- a source resolves: the `SourceRef` of the nearest bundle-rendering layout at or above the parent, with both `Kind` and `Name` set, else the one `SourceRef` the URL-less bundles below the child share (nil, empty struct, missing either field, or ambiguous is a hard error — a `Kustomization` without `spec.sourceRef` is invalid)

`CreateLayoutWithResources` validates SourceRef completeness for all bundles before layout walking. Both the node bundle and every umbrella child bundle must have `SourceRef.Kind` and `SourceRef.Name` set when either inline mode (`FluxIntegratedPerLayout` or `FluxIntegratedPerBundle`) is active — both emit bundle/node CRs carrying a `spec.sourceRef`. `FluxSeparate` and non-Flux callers are unaffected.

### Ordered reconciliation with DependsOn

Set `ManifestLayout.DependsOn` to a list of sibling layout names to express reconciliation order between hook groups. The integrator translates these into `spec.dependsOn` entries on the emitted CR:

```go
preInstall := &layout.ManifestLayout{
    Name: "nginx-00-pre-install",
    // ...
}
hooks := &layout.ManifestLayout{
    Name:      "nginx-01-hooks",
    DependsOn: []string{"nginx-00-pre-install"},
    // ...
}
```

This produces a `nginx-01-hooks` Kustomization CR with:

```yaml
spec:
  dependsOn:
    - name: nginx-00-pre-install
```

Flux reconciles `nginx-01-hooks` only after `nginx-00-pre-install` is healthy.

### Naming uniqueness

The child layout `Name` becomes the Flux `Kustomization` CR's `metadata.name`. Since all `Kustomization` CRs live in the `flux-system` namespace, names must be **globally unique across the cluster**. The recommended convention is `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`, `nginx-01-hooks`). Augmenters are responsible for enforcing this uniqueness.

### Extra files an augmenter attaches

`ExtraFiles` an augmenter attaches must use portable relative names (letters, digits, `.`, `_`,
`-`, and `/` between segments) and may not take a path the writer owns in the layout's directory:
a generated resource file, a kustomize control file, a child layout's directory or file, or
another extra file. The layout writers refuse such a layout with an error instead of letting the
extra file replace a generated manifest. See the [Layout Engine reference](/api-reference/layout/)
for the full rule.

### Disk layout

```
clusters/production/prod/
  flux-system-kustomization-nginx.yaml      # app CR (placed at node level)
  kustomization.yaml                        # references nginx CR
  nginx/
    flux-system-kustomization-nginx-00-pre-install.yaml
    flux-system-kustomization-nginx-01-hooks.yaml
    kustomization.yaml                      # references hook CRs
    nginx-00-pre-install/
      workload-*.yaml
      kustomization.yaml
    nginx-01-hooks/
      workload-*.yaml
      kustomization.yaml
```

## Bootstrap

Generate Flux system bootstrap manifests. Two modes are available:

- **`"flux-operator"`** (default) — emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters.
- **`"gotk"`** — emits the legacy GitOps Toolkit component manifests directly.

When `FluxMode` is empty, it defaults to `"flux-operator"`.

The `"flux-operator"` bundle is vendored from one specific upstream flux-operator release
(`FluxOperatorVersion`), so upgrading Kure can also change the CRDs it installs. A CRD that
tightens validation rejects objects the previous release accepted — for example a
`ResourceSetInputProvider` whose `filter.limit` exceeds 10000, or whose OCI `url` names only a
registry host. Kure has no setter for those fields, but you can assign them directly on the
upstream struct, so check hand-built objects against the release notes before upgrading. The
[compatibility matrix](/api-reference/compatibility/) records the supported flux-operator range.

The vendored bundle follows Kure's own `flux-operator` module version: when Renovate bumps the
module it also re-vendors the bundle and updates `FluxOperatorVersion`, so the bundle you get
always matches the release Kure was built against. Widening the supported range to cover a new
release is a separate, manual step that records a compatibility assessment.

`"gotk"` mode is vendored the same way: its components are built from the install manifests of
the flux2 release Kure depends on (`GotkVersion`), with no network access, as long as
`FluxVersion` is empty or names that release. Setting `FluxVersion` to any other release, or to
`"latest"`, opts in to downloading manifests from GitHub each time the bundle is generated: the
named release when written `vX.Y.Z`, otherwise the latest release.

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "flux-operator", // or "gotk"; empty defaults to "flux-operator"
    FluxVersion: "v2.8.2",
    SourceRef:   sourceRef,
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
```

`SourceRef` names the revision to sync, and the same value works in both modes: an OCI tag
(empty means `latest`) or a Git branch name. In `"flux-operator"` mode Kure turns a branch name
into the full reference the operator's `GitRepository` needs (`main` becomes `refs/heads/main`).
A value that already starts with `refs/` — a tag such as `refs/tags/v1.0.0` — is passed through
unchanged in `"flux-operator"` mode only; `"gotk"` mode always treats a Git `SourceRef` as a branch.

### Bootstrap namespace

The bootstrap namespace is not part of `BootstrapConfig` — it lives on the generator. The engine
holds its own generator and `engine.GenerateBootstrap` delegates to that one, so configure it
through the engine rather than building a second generator the call never reads:

```go
engine.GetBootstrapGenerator().DefaultNamespace = "custom-flux" // default: "flux-system"

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode)
```

**How much it moves depends on the mode, and the two differ sharply.**

With `"gotk"` it relocates the whole bundle: the toolkit components themselves — the controllers'
Deployments, ServiceAccounts, Services, NetworkPolicies and ResourceQuota — alongside the root
Kustomization and the root source. The cluster-scoped objects follow where they name a namespace,
since the emitted `Namespace` and the `ClusterRoleBinding` names and subjects are derived from the
same value; CRDs and ClusterRoles carry no namespace.

With `"flux-operator"` — the default mode — it reaches **only the `FluxInstance`**. The operator's
own install bundle is appended unmodified from an embedded manifest, so its `Namespace`,
ServiceAccount, Service and Deployment stay hardcoded to `flux-system` whatever this field says. A
non-default value therefore yields an operator running in `flux-system` reconciling a `FluxInstance`
in your namespace. That works, but it is not "the bundle moved", and the emitted `Namespace` object
is still named `flux-system`.

One further consequence, for `"gotk"`, worth knowing before you narrow anything: the toolkit
components are generated with `WatchAllNamespaces` at its upstream default of `true`, and that is
not currently derived from configuration. Controllers therefore reconcile across namespaces
regardless of where they run.

### Sync name

In `"flux-operator"` mode the `FluxInstance` sync makes the operator create a source and a
Kustomization of its own. `BootstrapConfig.SyncName` names both; it becomes `spec.sync.name`.
Left empty, the operator names them after the `FluxInstance`'s namespace — `flux-system` unless
you moved it as above. Set `SyncName` when the Kustomizations you generate reference the sync
source under another name, or their `sourceRef` points at a source nothing creates:

```go
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:   true,
    SourceURL: "oci://registry.example.com/fleet",
    SourceRef: "latest",
    SyncName:  "fleet",
}
```

It only takes effect with `SourceURL` set, since no sync block is emitted without one, and
`"gotk"` mode ignores it: there kure creates and names the root source itself. kure does not
validate the value. The CRD caps it at 63 characters and makes it immutable once set, so renaming
the sync of a running cluster means recreating the `FluxInstance`. It is distinct from the
`FluxInstance`'s own `metadata.name`, which is always `flux` (`fluxcd.FluxInstanceName`): the
flux-operator CRD rejects any other name at admission. The generator's
`BootstrapName` names the bootstrap Kustomization only and never reaches the `FluxInstance`.

## Further Reading

- [Stack](/api-reference/stack) - Domain model reference
- [Flux Engine](/api-reference/flux-engine) - Workflow engine reference
- [Layout Engine](/api-reference/layout) - Directory organization reference
