+++
title = "Augmenters and Delivery Intent"
weight = 23
+++

Two ways to shape what Flux does with one application: child layouts that an augmenter attaches, and the per-application delivery intent.

The Go blocks on this page are the bodies of `Example` functions in `pkg/stack/fluxcd`, which `go test` runs. They use the imports and the helpers named in the [tutorial](/guides/flux-workflow/).

## Augmenter-Added Child Layouts

A `LayoutAugmenter` can attach sub-`ManifestLayout` children to a per-app layout after resource generation. In `FluxIntegratedPerLayout` mode kure automatically emits a Flux `Kustomization` CR for each eligible child.

## Eligibility

A child layout receives a CR when:
- `!child.UmbrellaChild`
- `child.ApplicationFileMode != AppFileSingle`
- it renders no bundle (a child that does already has that bundle's CR in the parent)
- a source resolves: the `SourceRef` of the nearest bundle-rendering layout at or above the parent, with both `Kind` and `Name` set (with `BundleGrouping: GroupFlat` the root node's directory renders no bundle and counts with the `SourceRef` of the root node's own bundle; with `GroupByName` no node's directory counts), else the one `SourceRef` the URL-less bundles below the child share, else (no `SourceRef` without a URL below the child, or two that differ) the `SourceRef` of the bundle of the nearest node, at or above the child's own, that has one: what a node's directory takes with `GroupByName` when every `SourceRef` at and below it has a URL or the ones without a URL below it differ (nil, empty struct, missing either field, or still ambiguous with no such bundle is a hard error — a `Kustomization` without `spec.sourceRef` is invalid); the Kustomization of the root node's directory, below a `ClusterName` wrapper, takes no Source the integration generates and is refused when no other is left (go-kure/kure#979)

`CreateLayoutWithResources` validates SourceRef completeness for all bundles before layout walking. Both the node bundle and every umbrella child bundle must have `SourceRef.Kind` and `SourceRef.Name` set when either inline mode (`FluxIntegratedPerLayout` or `FluxIntegratedPerBundle`) is active — both emit bundle/node CRs carrying a `spec.sourceRef`. `FluxSeparate` and non-Flux callers are unaffected.

## Ordered reconciliation with DependsOn

Set `ManifestLayout.DependsOn` to the names of the sibling layouts to express reconciliation order between hook groups. A hook-group layout's CR is named `<unit name>-<layout name>` (or by its `KustomizationName`), and the integrator writes that name into `spec.dependsOn` for an entry that names a layout of the same unit: a sibling first, else the one layout of that name below the unit's directory. Any other entry is a Kustomization name and is written as given. This happens only under `FluxIntegratedPerLayout`; under the other placements the child gets no Kustomization and the field is refused (see [A layout's Kustomization settings are refused where it gets no Kustomization](/guides/flux-layout-rules/#a-layouts-kustomization-settings-are-refused-where-it-gets-no-kustomization-breaking-change)):

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowDependsOn -->
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
fmt.Println(hooks.Name, "after", hooks.DependsOn[0] == preInstall.Name)
```
<!-- doc-example:end -->

Each layout's CR is named `<unit name>-<layout name>`, and the entry is written as the CR name of
the layout it names. Below the bundle `web` this produces a `web-nginx-01-hooks` Kustomization CR
with:

```yaml
spec:
  dependsOn:
    - name: web-nginx-00-pre-install
```

Flux reconciles `web-nginx-01-hooks` only after `web-nginx-00-pre-install` is healthy. An entry
may also name another layout of the same bundle, the parent application layout for instance
(not while that layout's Kustomization has `wait`, its own or the bundle's: see [Per-layout Kustomizations take the bundle's readiness settings and labels](/guides/flux-layout-rules/#per-layout-kustomizations-take-the-bundles-readiness-settings-and-labels-breaking-change)). An entry that names no such layout is written as given, as the name of a Kustomization.

## Naming uniqueness

The Flux `Kustomization` CR's `metadata.name` is `<unit name>-<child layout Name>`, or the child's `KustomizationName` when the augmenter sets one. Since all `Kustomization` CRs live in the `flux-system` namespace, names must be **globally unique across the cluster**. The recommended convention for the layout name is `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`, `nginx-01-hooks`). Augmenters are responsible for enforcing this uniqueness within their bundle.

## Extra files an augmenter attaches

`ExtraFiles` an augmenter attaches must use portable relative names (letters, digits, `.`, `_`,
`-`, and `/` between segments) and may not take a path the writer owns in the layout's directory:
a generated resource file, a kustomize control file, a child layout's directory or file, or
another extra file. The layout writers refuse such a layout with an error instead of letting the
extra file replace a generated manifest. `ConfigMapGenerators` need the layout's own
`kustomization.yaml`, so the writers refuse them on an application written `AppFileSingle` (as
`ArgoProfile` writes applications), which gets none. See the
[Layout Engine reference](/api-reference/layout/) for the full rule.

## Disk layout

With the application `nginx` in the bundle `web`, each CR and its file carry the unit name:

```
clusters/production/prod/
  flux-system-kustomization-web-nginx.yaml  # app CR (placed at node level)
  kustomization.yaml                        # references nginx CR
  nginx/
    flux-system-kustomization-web-nginx-00-pre-install.yaml
    flux-system-kustomization-web-nginx-01-hooks.yaml
    kustomization.yaml                      # references hook CRs
    nginx-00-pre-install/
      workload-*.yaml
      kustomization.yaml
    nginx-01-hooks/
      workload-*.yaml
      kustomization.yaml
```

## Delivery Intent per Application

`Bundle.Prune` and `Bundle.Force` apply to a whole Flux Kustomization. To protect one application
from pruning, or to allow delete-and-recreate for one application, set its `Delivery`:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowDeliveryIntent -->
```go
database := stack.NewApplication("database", "data", databaseConfig)
database.Delivery = stack.DeliveryIntent{
    PruneProtection: true, // keep its objects when they leave the source
    ForceReplace:    true, // delete and recreate on an immutable-field change
}
cache := stack.NewApplication("cache", "data", cacheConfig)

dataTier, err := stack.NewBundle("data-tier", []*stack.Application{database, cache}, nil)
if err != nil {
    panic(err)
}
dataTier.SourceRef = &stack.SourceRef{Kind: "GitRepository", Name: "flux-system"}
cluster := stack.NewCluster("production", &stack.Node{Name: "production", Bundle: dataTier})

// The Flux engine annotates the database's objects, and only those.
ml, err := fluxcd.Engine().CreateLayoutWithResources(cluster, layout.DefaultLayoutRules())
if err != nil {
    panic(err)
}
var show func(l *layout.ManifestLayout)
show = func(l *layout.ManifestLayout) {
    for _, rec := range l.OriginApplicationObjects() {
        for _, obj := range rec.Objects {
            kind := obj.GetObjectKind().GroupVersionKind().Kind
            fmt.Println(rec.Application.Name, kind, len(obj.GetAnnotations()))
            for _, key := range slices.Sorted(maps.Keys(obj.GetAnnotations())) {
                fmt.Printf("  %s: %s\n", key, obj.GetAnnotations()[key])
            }
        }
    }
    for _, child := range l.Children {
        show(child)
    }
}
show(ml.(*layout.ManifestLayout))
```
<!-- doc-example:end -->

The engine writes `kustomize.toolkit.fluxcd.io/prune: disabled` and
`kustomize.toolkit.fluxcd.io/force: enabled` on that application's objects under every placement
and grouping, including the objects an augmenter adds to the application's own layout and the
ConfigMaps its `configMapGenerator` entries build. For a `List` the application emits, the objects
it holds are annotated, those of a `List` inside it included, and never the List itself. An
object that already carries one of the two
annotations with another value is refused with an error naming it. The annotations are applied by
`IntegrateWithLayout` and `CreateLayoutWithResources`; see the
[Flux Layout Integration reference](/api-reference/flux-layout-integration/) for the full rule, including what prune
protection means for generated ConfigMaps.
