# Layout Integration

Combine resource generation with directory structure:

<!-- doc-example: pkg/stack/fluxcd ExampleWorkflowEngine_CreateLayoutWithResources -->
```go
cluster := exampleCluster()
engine := fluxcd.Engine()
rules := layout.DefaultLayoutRules()
out, err := os.MkdirTemp("", "kure-fluxcd-example")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(out) }()

// Create layout with Flux resources integrated
ml, err := engine.CreateLayoutWithResources(cluster, rules)
if err != nil {
    panic(err)
}

// Write to disk: every spec.path is relative to <out>/clusters
err = layout.WriteManifest(out, layout.DefaultLayoutConfig(), ml.(*layout.ManifestLayout))
if err != nil {
    panic(err)
}
printFiles(out)
```
<!-- doc-example:end -->

`IntegrateWithLayout(ml, cluster, rules)` does the same for a layout you walked yourself. It
refuses a tree `layout.WalkCluster` did not build from that cluster (see
[Kustomization paths](/api-reference/flux-names-and-paths/#kustomization-paths)). Integrating the same layout twice adds nothing: a
CR already present with the same name and `spec.path`, in the layout that would host it, is kept
when it reads the Source the generated one would (its `spec.sourceRef`, an omitted namespace its
own) and sets no `spec.kubeConfig`, since only then is its `spec.path` this tree's directory;
the same name in another layout, with another path, on another Source or with `spec.kubeConfig`
is an error, and under `FluxSeparate` an identical `flux-system` child — same directory, same
resources, nothing beneath it — is kept rather than a second one appended; any other is refused. Every Flux Kustomization already in the tree counts — typed or unstructured, placed
by an earlier integration, by the caller or emitted by an application, top-level or inside a
`List` (kustomize builds a List's items; a List is an object written with a kind that ends in
`List`, whatever kind a typed object's Go value reports, and that has an `items` field, and a
List among the items is opened too, while a kind that does not end in `List` is one object whatever fields it has; an item a typed List holds as raw JSON is read as
the object it encodes, also when it carries an object beside the raw JSON, and one without object
metadata as the object it serializes to): an
identity (namespace/name) present twice, or taken by
a generated CR elsewhere, is refused in every placement, since the kustomize build would register
the id twice. Where a generated CR meets one in the tree, the error names both and the way out
(see [Kustomization names](/api-reference/flux-names-and-paths/#kustomization-names)). A generated Source has one definition across the whole pass: every Source of its
identity — kind, namespace and name, whatever the API version (a `v1beta2` `GitRepository` is the
same Source as the generated `v1` one) — anywhere in the tree, top-level or inside a `List`, and
every Source the pass places in another layout, must have the same API version and content, or the
integration is refused. Content is compared as the objects' unstructured form, so a typed Source
and an unstructured copy of it are the same. A Kustomization or Source written with kustomize's
`config.kubernetes.io/local-config` annotation at any value but `false` (the string `"false"` or
the boolean `false`; `"False"`, a number and `null` are other values, as kustomize reads them)
does not count anywhere in the integration: kustomize drops it from the build, so it never
reaches the cluster.
It is no existing object to collide with or keep, no copy of a Source to host in place of the
generated one, no member or child of an umbrella, and it marks no directory as a Flux build
(go-kure/kure#1059). With no annotation, or `false`, it is built and counts as above. It
still may not share a kustomize build with an object of its identity: kustomize registers both
before it drops it, and refuses the second. The writers refuse such a tree, as they refuse one
object held twice in a build.

Under the integrated placements every Source the integration generates is hosted **once, in the
root node's layout**, whichever layouts hold the Kustomizations that use it (go-kure/kure#876),
and not in a `ClusterName` wrapper above it. For a named root node under the default rules that is
the directory the bootstrap applies (see
[The directory the bootstrap applies](/api-reference/flux-bootstrap-generation/#the-directory-the-bootstrap-applies), and
[Kustomization paths](/api-reference/flux-names-and-paths/#kustomization-paths) for other rules), so the Source is in one build, the
root's, and exists before any Kustomization that uses it. Below a `ClusterName` wrapper the
bootstrap applies the wrapper, and the root node's directory with it or through that directory's
own Kustomization (below).

**No Kustomization takes its source from inside what it applies** (go-kure/kure#979). A generated
Source is in a build that is applied before every Kustomization that names it, never in a
directory that Kustomization delivers: the bootstrap applies the top of the written tree (the
`ClusterName` directory, see
[The directory the bootstrap applies](/api-reference/flux-bootstrap-generation/#the-directory-the-bootstrap-applies)), and where nothing
there delivers that Source, the Kustomization waits for a Source only its own apply creates and
never reconciles. On a walked tree every bundle's Kustomization meets the rule
by construction, because the root node's directory renders no bundle. The one Kustomization that
applies the root node's directory is that directory's layout Kustomization below a `ClusterName`
wrapper under `FluxIntegratedPerLayout`, hosted in the wrapper. It takes no
generated Source: the `SourceRef` of the root node's bundle is passed over when it has a URL or
names a Source another `SourceRef` generates, and its source is the one `SourceRef` the other
URL-less bundles below share, or, where those differ, the root node's own bundle's when it was
not passed over. When nothing is left (every `SourceRef` in the tree has a URL, say)
the integration is refused, naming the Kustomization, its `spec.path`, the Source and the
directory that hosts it. Give a bundle a `SourceRef` without a URL, naming a Source that exists
before the tree is applied (the one the bootstrap creates, for one), or use
`FluxIntegratedPerBundle`, under which those directories have no Kustomization of their own.
Without a `ClusterName` wrapper the root node's directory has no Kustomization and nothing
changes. A `SourceRef` with a URL on a kind the integration generates no Source for (anything but
`GitRepository` and `OCIRepository`) is not counted as a generated Source; where generation is
reached, it is the generator's own error. The rule is held for the Kustomizations
the integration generates or keeps and the directories they apply: a Kustomization of your own
placed inside one of them and pointing back up the tree is not followed (see below, the builds
kure answers for).

Among the bootstrap and the Kustomizations the integration generates or keeps, each hosted Source
has one applier. Without a `ClusterName` wrapper, and below a wrapper whose `kustomization.yaml`
lists the root node's directory, that is the bootstrap, and none of those Kustomizations builds
that directory. Below a wrapper under
`FluxIntegratedPerLayout` it is the layout Kustomization of the root node's directory: the
bootstrap applies the wrapper, which lists that Kustomization and not the directory. A
Kustomization of your own kept in its place (below) may therefore carry patches or a postBuild
that change a hosted Source; they are the only ones that Source gets (go-kure/kure#979: the
bootstrap used to be pointed at the root node's directory, beside that Kustomization, and such a
patch or postBuild was refused). A `sourceRef` names the object, not the layout holding it.

The root build also covers the child directories the root's `kustomization.yaml` lists and the
`AppFileSingle` files written into them. When that build already holds a copy the integration did
not add (an earlier integration's, the caller's or an application's), that copy is kept and the
integration adds none, since kustomize refuses one object twice. Two copies the integration did
not add, in any one build (the root's, a `ClusterName` wrapper's or a generated Kustomization's
`spec.path`), are refused. So is one such copy in a wrapper whose `kustomization.yaml` lists the
root node's directory: the wrapper's build would hold it and the root's copy, and the integration
removes neither; move the copy into the root node's build or remove it. A copy the caller or an application puts in any other
build is theirs to keep, and the integration still hosts its own at the root: that Source then has
two owners, one of them the caller's.
Kustomizations the caller or an application places are not builds kure answers for. Under
`FluxSeparate` every generated Source goes into
`flux-system`, which is built beside the rest of the tree, so a Source with a generated Source's
identity anywhere else in the tree is refused even when it is identical.

A Kustomization the integration keeps in place of its own (same name and `spec.path`, in the
layout that would host it) is checked as the generated one would be, with what the kept object
itself sets and in whatever form it has: typed, unstructured or inside a `List`
(go-kure/kure#979). Its `spec.path` is a build in which a generated Source may appear once; when
that build holds the root node's layout, its `sourceRef` may not name a Source hosted there (an
omitted `sourceRef` namespace is the Kustomization's own): the integration is refused, naming it,
since it would take its source from inside what it applies; and its `dependsOn`, `wait` and health
checks enter the reconcile-order check (see
[One Kustomization per directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)). An unstructured one is read
through the typed Flux `Kustomization`, the way the API server reads a custom resource: a key
matches a field only in its exact spelling, and a key the type does not have is dropped. A key
written `Wait:` or `KubeConfig:` therefore sets nothing, as it sets nothing for Flux: it adds no
`wait` edge, and the Kustomization still applies to this cluster (go-kure/kure#1173). If the
API server rejects such a key instead of pruning it, the apply of that object fails there; the
integration does not report it. One that cannot be read that way (a field of the wrong
type, for one) is refused, with an error naming the layout that holds it and its namespace and
name.

## Delivery intent

An application's `Delivery` (see [stack](/api-reference/stack/)) is written as kustomize-controller
annotations on that application's objects, and on no other application's:

| `stack.DeliveryIntent` field | Annotation |
|---|---|
| `PruneProtection` | `kustomize.toolkit.fluxcd.io/prune: disabled` |
| `ForceReplace` | `kustomize.toolkit.fluxcd.io/force: enabled` |

`IntegrateWithLayout` (and so `CreateLayoutWithResources`) sets them, under all three placements
and every grouping, on:

- every object the application emits. For a `List` that is what it holds, a `List` inside it
  included, and never the envelope. As in kustomize, an object is a `List` when its kind ends in
  `List` and it has an `items` field in the written file; a kind ending in `List` without `items`
  (a typed object that leaves an empty one out included), and any other kind with an `items`
  field, is one object and carries the annotation itself. The written file decides: a typed
  object whose written form still holds an object without the annotation, because its Go value
  gives no access to it, is refused. An item held as raw JSON (`runtime.RawExtension.Raw`) is
  such an object: it is written as it is, so it has to carry the annotation already. An object
  the Go value holds that is not among the written items is not the application's and is left
  as it is. A `List`
  among a typed List's items is opened whether or not it carries object metadata of its own
  (`metav1.List`, the core `v1` List as a Go value, has none), and the objects it holds as Go
  values are annotated like any other (go-kure/kure#1006);
- every object a `LayoutAugmenter` adds to the application's own layout or a child layout below it;
- every `configMapGenerator` of those layouts, as the entry's `options.annotations`, because
  kustomize builds those ConfigMaps after kure has written the tree.

An object's other annotations are kept. An application that sets no intent gets no annotation and
its output does not change.

An application may emit the Flux Source that a bundle's `SourceRef` also derives; under the
integrated placements the two are one object, hosted once per build. With an intent on that
application, the derived Source carries the same annotations, so every copy that is applied is
the same. A Source the application emits with `config.kubernetes.io/local-config` (any value but
`false`, the string or the boolean) is dropped by kustomize and so is no copy that is applied:
the derived Source does not take its application's annotations from it. The dropped copy is annotated all the same, as
kustomize patches it before it drops it.

An object or generator that already carries one of these annotations with **another** value is
refused, with an error naming the application, the object (kind, namespace/name) or generator, the
annotation and both values; the same value is left as it is, so integrating a layout again changes
nothing. When the integration is refused, for this or any later reason, the annotations it added
are taken back with the rest of its changes.

Things to know:

- The annotations are set on the objects the layout holds, not on copies. An `ApplicationConfig`
  that returns the same object pointers on every `Generate` call therefore keeps them after a
  successful integration, also when the intent is cleared afterwards; return fresh objects per call
  to avoid that.
- The integrator applies the intent. `GenerateFromCluster` returns Kustomizations and Sources and
  none of the application's objects, so it refuses a cluster in which an application sets an
  intent, with an error naming the application and pointing to `CreateLayoutWithResources`.
  `GenerateFromLayout` does not refuse an intent: its caller holds the layout, and the list it
  returns carries no intent by itself. The intent is applied by `IntegrateWithLayout` or
  `CreateLayoutWithResources` on the layout, and a layout that is walked and written without
  either carries none.
- A bundle patch runs after the build: `Bundle.Patches` become the Kustomization's `spec.patches`,
  which Flux applies to what the directory builds. A patch can therefore still change or remove a
  delivery annotation on the applied object; kure does not read patch bodies and does not override
  an explicit patch.
- kustomize names a generated ConfigMap after its content. With `PruneProtection`, each content
  change therefore leaves the previous ConfigMap in the cluster instead of pruning it; remove old
  ones by hand, or leave the intent off an application whose generated values change often.
- `Bundle.Prune` and `Bundle.Force` set `spec.prune` and `spec.force` on the whole generated
  Kustomization and are independent of the per-application intent.
