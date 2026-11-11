# Non-Bundle Child Layout CRs

In `FluxIntegratedPerLayout` mode every child layout that is not an umbrella child, not `AppFileSingle` and renders no bundle gets a `Kustomization` CR in its parent's `Resources`, with `spec.path` set to `child.FullRepoPath()`. (A child that renders a bundle already has that bundle's CR there.) The CR's name is checked with `stack.ValidateKustomizationName` where the CR is created: one that is not a DNS-1123 subdomain of at most 63 characters is refused, and the error names what to set to change it (see [Per-layout name rule](#per-layout-name-rule)). This covers:

- **Application layouts** — per-app layouts (`ApplicationGrouping: GroupByName`, or augmenter apps that want their own layout, which keep a directory under `GroupFlat`). The CR is named `<unit name>-<layout name>`.
- **Augmenter sub-layouts** — hook-group child layouts added by a `LayoutAugmenter` are children of an app layout. They are named `<unit name>-<layout name>` as well, and `spec.dependsOn` is populated from `ManifestLayout.DependsOn`, enabling ordered reconciliation between hook groups.
- **Bundle-less node layouts** — a GroupByName node layout above its bundle layout, or a node without a bundle. The CR is named `<path with "/" replaced by "-">-node` (with `ClusterName: "."`, node `web`'s path is `web`, which is also its bundle's CR name when the bundle sets no `KustomizationName`), unless the node names it: see [Node Kustomizations](#node-kustomizations).

The unit name is the name in effect of the Kustomization that applies the bundles the layout belongs to: the nearest layout at or above its parent that renders bundles (`Bundle.KustomizationName`, or the bundle's `Name`; the first bundle's when a grouping axis merged several into one directory). The application `web` of the bundle `web` therefore gets the Kustomization `web-web`, not a second `web`. `ManifestLayout.KustomizationName` replaces the default name: an augmenter sets it on a layout it creates, and a caller may set it on a walked layout before integration.

## Per-layout name rule

The name in effect of a per-layout CR is a Flux Kustomization name: a DNS-1123 subdomain of at most 63 characters (`stack.ValidateKustomizationName`). It is checked where the CR is created, so nothing is written for a tree with a name Flux could not reconcile. kure shortens one name only, the default of an application or augmenter layout (below); any other name is checked as it is, and the error names the object and the field that changes it.

| Where the name comes from | The error names |
|---|---|
| `Node.KustomizationName` | the node by its path (`Node 'prod/apps'`) and the field `kustomizationName` |
| the name derived for a node, `<path>-node` | the node, and `Node.KustomizationName` as the field to set |
| `ManifestLayout.KustomizationName`, set by an augmenter or on a walked layout | the layout by its directory (`ManifestLayout 'prod/web/api'`) and the field `kustomizationName` |
| the default of an application or augmenter layout, `<unit name>-<layout name>` (shortened where it is over the limit, so refused only for its characters) | the layout, and `ManifestLayout.KustomizationName` as the field to set |

The default of an application or augmenter layout is longer than the layout's own name by the unit name, so a directory name within the limit can give a default over it. kure shortens such a default, whatever the length of the layout name, the same way for every caller, and leaves one that fits byte for byte as it is (go-kure/kure#1030, go-kure/kure#1036). The hash is the first 8 hexadecimal characters of the SHA-256 of the whole default.

- A layout name of at most 54 characters keeps its `-<layout name>` tail, and the unit name is replaced by a prefix of it and the hash: `<unit prefix>-<hash>-<layout name>`, at most 63 characters. The prefix is the leading `63 - len(layout name) - 10` bytes of the unit name, without the hyphens and dots that end them, so the name can be shorter than 63. A layout name of 53 or 54 characters leaves no byte for it, and the name is `<hash>-<layout name>`.
- A longer layout name leaves no room for the hash beside it. The name is the layout name's first 54 bytes, without the hyphens and dots that end them, and the hash: `<layout name prefix>-<hash>`, at most 63 characters; where nothing is left of those bytes, the name is the hash alone. The unit name and the rest of the layout name show only in the hash. Before go-kure/kure#1036 such a default was refused.
- The hash is of the whole default, so the name is the same on every run for the same unit and layout names. Two defaults that differ only in the part the shortening drops almost always get different hashes, but 8 hexadecimal characters are 32 bits and two can collide (the units `platform-services-65327` and `platform-services-103659` do, beside the layout `checkout-service-payments-reconciler-worker`; so do the layouts `checkout-service-payments-reconciler-worker-hook-group-46715` and `-61807` below the unit `shop`). A name used twice, by a collision or otherwise, is refused, naming both owners (below); set `ManifestLayout.KustomizationName` on one of them.
- A `DependsOn` entry that names the layout is written as the shortened name.

The application `checkout-service-payments-reconciler-worker` (43 characters) of the bundle `platform-services-payments` (26) has a default of 70 characters, so its Kustomization is `platform-s-7365aca5-checkout-service-payments-reconciler-worker`. Below the same bundle, the 76-character layout `checkout-service-payments-reconciler-worker-pre-install-hooks-schema-migrate` gets `checkout-service-payments-reconciler-worker-pre-instal-5c8f049d`.

`DefaultLayoutKustomizationName(unit, layoutName)` returns this default, shortened or not, so a caller that checks names before writing them (for collisions or length across its own Kustomizations) gets the name kure writes instead of composing `<unit name>-<layout name>` itself. `unit` is the unit name as defined above (the first bundle's `Bundle.UnitName()` where a grouping axis merged several bundles into one directory, not necessarily that of the bundle holding the application), `layoutName` the layout's `Name`; the result is the same on every run for the same inputs (go-kure/kure#1040). It returns the name only: its characters and a duplicate are still checked where the Kustomization is created, and a layout that sets `KustomizationName` uses that name instead.

<!-- doc-example: pkg/stack/fluxcd ExampleDefaultLayoutKustomizationName -->
```go
// The application "api" of the bundle "web": "<unit>-<layout name>".
fmt.Println(fluxcd.DefaultLayoutKustomizationName("web", "api"))

// Over 63 characters: the layout name stays, a prefix of the unit and a
// hash replace the rest.
fmt.Println(fluxcd.DefaultLayoutKustomizationName(
    "platform-services-payments", "checkout-service-payments-reconciler-worker"))
```
<!-- doc-example:end -->

A `ManifestLayout.DependsOn` entry on an application or augmenter layout is a layout name. Where it names a layout of the same unit that has a CR of its own, the entry is written as that layout's CR name: a sibling first, else the one layout with that name elsewhere below the unit's directory (the parent application layout of a hook group, for instance). Two such layouts with no sibling among them are refused, since the entry does not say which: set `KustomizationName` on the one meant and list that name. Any other entry is written as given, as the name of a Kustomization.

The integrator applies this rule at any depth. The CR's `spec.sourceRef` is the `SourceRef` of the nearest layout at or above the host that renders bundles; with `BundleGrouping: GroupFlat` the root node's layout renders none and counts with the `SourceRef` of its own bundle, rendered in the directory inside it (`OriginUnit`, go-kure/kure#979), so a bundle-less child node of the root still takes the root bundle's source; with none, the one `SourceRef` the URL-less bundles below the child share; with none of those either (no `SourceRef` without a URL below the child, or two that differ), the `SourceRef` of the bundle of the nearest node, at or above the child's own, that has one. That last rule is what a node's layout takes with `BundleGrouping: GroupByName`, where no node's layout renders a bundle, the root's included, when every `SourceRef` at and below it has a URL or the ones without a URL below it differ: its own node's bundle's, rendered one directory lower, or for a node without a bundle the one of the nearest node above, the bundle that encloses it with `GroupFlat` (go-kure/kure#979). It comes last, so it gives a source only to a layout that has no other. The CR of the root node's layout (below a `ClusterName` wrapper; on a tree built by hand, that of any layout above it too) takes no Source the integration generates, whichever of these would give it one (see [Layout Integration](/api-reference/flux-layout-integration/#layout-integration): no Kustomization takes its source from inside what it applies). A missing, incomplete or ambiguous source is a hard error — a `Kustomization` without a valid `spec.sourceRef` is rejected by Flux and must not be emitted silently — and so is a layout whose bundles have different `SourceRef`s (a `NodeGrouping: GroupFlat` merge) hosting a layout CR. A node without a bundle, with no bundle on a node above it, above bundles whose `SourceRef`s all have a URL has no source under either `BundleGrouping`: the error names its Kustomization and the three ways out (a `SourceRef` without a URL on a bundle below, a bundle with a `SourceRef` on a node at or above, or `FluxIntegratedPerBundle`). Such a node above bundles whose `SourceRef`s without a URL differ has none either, and its error names the bundle on a node at or above it as the way out: without a URL where the layout is the root node's below a `ClusterName` wrapper. A CR name used twice (a node whose `KustomizationName` is also a bundle's Kustomization name, say) is an error, not a silent skip: Flux Kustomizations share one namespace. When this integration generates both, the error names both owners, each as `bundle "<path>"`, `node "<path>"` or `layout "<directory>"`; a clash with a Kustomization already in the tree names the two layouts that hold them.

## Per-layout settings

Besides its name, path, source and dependencies, a per-layout Kustomization carries twelve settings. Five came with go-kure/kure#1015: `spec.wait`, `spec.timeout`, `spec.retryInterval`, `metadata.labels` and `metadata.annotations`. Six came with go-kure/kure#1021: `spec.interval`, `spec.prune`, `spec.force`, `spec.suspend`, `spec.postBuild` and `spec.patches`. One came with go-kure/kure#1034: `spec.serviceAccountName`. They come from the bundle and, all but `spec.postBuild`, `spec.patches` and `spec.serviceAccountName`, from the layout.

**From the bundle.** The Kustomization of an application's own layout, and of every layout below it at any depth (the ones the application's `LayoutAugmenter` added), takes them from the bundle that holds the application: `Bundle.Wait`, `Timeout`, `RetryInterval`, `Labels`, `Annotations`, `Interval`, `Prune`, `Force`, `Suspend`, `PostBuild`, `Patches` and `ServiceAccountName`. Where a grouping axis merged several bundles into the directory above the application's, that bundle is the one whose `Applications` lists the application, not the first of them and not the merged set. An application of an umbrella child inherits from the child bundle, not from the umbrella above it.

**From the layout.** `layout.ManifestLayout` has nine of the fields, set by an augmenter on a layout it creates or by a caller on a walked layout before integration:

| Layout field | Unset | Set |
|---|---|---|
| `Wait` (`*bool`) | the bundle's | replaces the bundle's; a pointer to `false` turns an inherited wait off |
| `Timeout`, `RetryInterval` | the bundle's | replaces the bundle's. An inherited value can be replaced, not removed |
| `Labels`, `Annotations` | the bundle's | merged with the bundle's key by key, the layout's value winning. An inherited key can be given another value, not dropped |
| `Interval` | the bundle's, or else `ResourceGenerator.DefaultInterval` | replaces the bundle's |
| `Prune` (`*bool`) | the bundle's, or else `ResourceGenerator.Prune` | replaces the bundle's; a pointer to `false` turns an inherited prune off |
| `Force`, `Suspend` (`*bool`) | the bundle's; with neither set the field is left out | replaces the bundle's; a pointer to `false` turns an inherited one off |

A layout's own fields apply to that layout's Kustomization only: a layout below it inherits from the bundle again, not from the layout above. The fields are read on every layout that gets a Kustomization of its own under this placement, a node's layout included (see [Node Kustomizations](#node-kustomizations)); a layout that belongs to no application inherits nothing, so its own fields are all it has. On a layout that gets no Kustomization of its own (the top of the tree, a layout that renders a bundle, an umbrella child, an `AppFileSingle` layout), and on every layout under `FluxSeparate` and `FluxIntegratedPerBundle`, the fields are refused, as `DependsOn` and `KustomizationName` are: the error names the layout, every such field it sets, why it gets no Kustomization, and `FluxIntegratedPerLayout` as the placement that carries them. `ResourceGenerator.GenerateFromLayout` refuses them as `FluxSeparate` does. The check runs after the one on [node fields](#node-kustomizations), before any Kustomization is placed or generated, so a value the walker copied from a node is refused once, on the node.

**Interval and prune fall back to the generator's.** Where neither the layout nor the holding bundle sets one, `spec.interval` is `ResourceGenerator.DefaultInterval` and `spec.prune` is `ResourceGenerator.Prune`, the values every per-layout Kustomization had before the two were read from the bundle: a tree that sets none of the six renders byte for byte as it did. A bundle that does set one now passes it on. In particular a bundle with `Prune` on turns garbage collection on for the Kustomizations of its applications' layouts, which rendered with the generator's `Prune` before, and a bundle's `Interval` replaces the generator's on them.

**The substitution is the bundle's, whole.** A per-layout Kustomization takes the holding bundle's `PostBuild` as it is, the inline variables and the `SubstituteFrom` references, each Kustomization with a copy of its own. There is no layout field for it. Flux reads a referenced ConfigMap or Secret from the Kustomization's namespace, and every Kustomization of the integration is in the generator's, so a reference the bundle's own Kustomization resolves is one the per-layout ones resolve.

**The service account is the bundle's.** A per-layout Kustomization takes the holding bundle's `ServiceAccountName`, and there is no layout field for it: the layout's objects are the application's, and the bundle names the account they are applied under. Unset, the field is left out. See [Service account](/api-reference/flux-engine/#service-account).

**The parent's substitution leaves them as written.** A per-layout Kustomization is a resource of its parent's build, whose postBuild substitution runs over it before it applies its own postBuild and patches. Where that would change its postBuild or its patches at all (a `${VAR}`, plain or not, or an escape `$${VAR}`), the integration writes Flux's opt-out annotation `kustomize.toolkit.fluxcd.io/substitute: disabled` on it, and the Kustomization applies them as if no parent had run. What a patch the parent substituted first comes to depends on the objects it patches (one may carry the opt-out itself, and a value's YAML type is read from the object), so any change counts. The opt-out also stops the parent substituting the Kustomization's labels, annotations and path, so where the parent substitutes one of those as well the integration is refused, naming the Kustomization, its layout and the fields. The annotations are checked once more with the opt-out in them, and the integration is refused, naming the layout, where they are then more than the total size the Kubernetes API accepts.

**No health checks.** A per-layout Kustomization takes no `spec.healthChecks`: `wait` is its readiness setting, and with `wait` Flux ignores health checks. A bundle's `HealthChecks` stay on the bundle's Kustomization.

**An application rendered in the bundle's directory** (flat `ApplicationGrouping`, no `LayoutAugmenter`) has no Kustomization of its own: the bundle's Kustomization applies it and its settings are the bundle's, as before.

### Patches of a bundle with per-layout Kustomizations

Under this placement the bundle's Kustomization builds the bundle's directory, and the Kustomizations of its applications' directories build those. A patch reaches an object only from the Kustomization whose build holds the object, so a bundle's `Patches` are placed by object once every Kustomization and Source of the tree is placed. There is no layout field for patches.

A build holds the objects of its directory and of the directories its `kustomization.yaml` reaches, the Flux Kustomizations and Sources hosted there included, and the ConfigMap each `configMapGenerator` entry there generates, which a patch names by the entry's name. A generated ConfigMap has no namespace: an entry has no field for one and the `kustomization.yaml` kure writes sets none for the directory. Kustomize reads that as `default`, so a patch document names it without a namespace or in `default`; a document that names it in another namespace names an object no build holds.

| Patch | Written on |
|---|---|
| with a `Target` | the bundle's own Kustomization and every per-layout Kustomization of the bundle. Kustomize applies it to what the target selects in that build, and to nothing where it selects nothing |
| without a `Target`, strategic-merge | the Kustomizations of the bundle, its own among them, whose build holds every object the patch names, and no other. Objects are compared as kustomize compares them: group, version, kind, name and namespace, a document without a namespace naming the object in `default` |
| without a `Target`, strategic-merge, after an entry of `Bundle.Patches` that is not a plain strategic-merge patch | the bundle's own Kustomization, and besides the per-layout Kustomizations whose build holds every object the patch names. No placement check refuses it, but a build it fails is refused, naming the patch; see "After an entry that is not a plain strategic-merge patch" below |
| without a `Target`, not a plain strategic-merge patch (a JSON6902 patch, a patch with one of kustomize's build annotations, or one that can remove its object: a top-level `$patch: delete`, or `config.kubernetes.io/local-config` with any value but `"false"`) | the bundle's own Kustomization only, and no placement check refuses it. This is a limit: it is not read, so there is no object to place it by. Kustomize refuses a JSON6902 patch without a target wherever it is, and the integration refuses the build that fails |

Each Kustomization keeps the order of `Bundle.Patches` among the patches it takes.

**The bundle's own Kustomization loses a patch.** An untargeted strategic-merge patch whose object the bundle's own Kustomization does not build is no longer written on it: it is on the Kustomization that builds the object, and there alone. Before go-kure/kure#1021 it was on the bundle's own, whose build fails on the cluster with `no resource matches strategic merge patch` when the object is in an application's directory.

**A failure moves from the cluster to the integration.** Kustomize fails the build of a Kustomization that holds an untargeted strategic-merge patch for an object it does not build. Two cases are therefore refused before anything is written, the error naming the bundle, the patch by its index in `Bundle.Patches`, the object and the Kustomizations looked at:

- no Kustomization of the bundle builds an object the patch names. Give the patch a `Target` (for a patch of several objects: one patch per object first, each with its `Target`), or place the object in a build of the bundle.
- every object a multi-document patch names is built, but no one Kustomization builds them all. The documents of one patch are applied in one build: write one patch per object.

**A `Target` is for a patch of one object.** Kustomize refuses a target on an entry that holds several strategic-merge patches, before it selects anything: ``Multiple Strategic-Merge Patches in one `patches` entry is not allowed to set `patches.target` field``. It counts the objects of the entry: its documents, and for a document that is a `List` its items, each of which it loads as a patch of its own. A patch with a target is written on the bundle's own Kustomization and on every per-layout one, so each of their builds then fails, the ones whose objects the target does not select included. Wherever this section gives a `Target` as the remedy, a patch of several objects is therefore first written as one entry of `Bundle.Patches` per object, each with its own `Target`.

A tree that built on the cluster before is not refused, and its bundle's own Kustomization builds what it built. Where every entry of the list is a plain strategic-merge patch (next paragraph), that build held every object its untargeted patches name, so each of those patches stays there, and is written on a per-layout Kustomization as well only where that one builds an object of the same identity. For a bundle without per-layout Kustomizations nothing changes at all: its own Kustomization keeps every patch, byte for byte, and no patch is examined.

**After an entry that is not a plain strategic-merge patch.** The objects of a build are compared as generated, which is what a patch names only while no earlier entry of the list has changed an object's identity. Placement by object therefore holds for a patch only while every entry of `Bundle.Patches` before it, and the patch itself, is a *plain* strategic-merge patch: its text parses as resources, no document of it carries an annotation whose key begins with `internal.config.kubernetes.io/`, the domain kustomize keeps its own build state in, and no document can remove its object from the build, with a top-level `$patch: delete` or with the annotation `config.kubernetes.io/local-config` at any value but `"false"`. Kustomize keeps the `apiVersion`, kind, name and namespace of the object a plain patch is merged into, with a `Target` or without, and a Flux patch has no option that allows the change. Any other entry is not known to keep them:

- a JSON6902 patch with a `Target` may replace `metadata.name`, `metadata.namespace`, `kind` or `apiVersion`, and kustomize then matches a later strategic-merge patch against the new identity as well as the one before;
- kustomize reads its build annotations from the text of a patch as it reads its own: `internal.config.kubernetes.io/allowNameChange` and `allowKindChange` let a strategic-merge patch change the name or the kind of the object it is merged into, and the `previousNames`, `previousNamespaces` and `previousKinds` annotations give a document the identity it names its object by;
- a document with a top-level `$patch: delete` removes its object from the build, and so does one that gives it `config.kubernetes.io/local-config` with any value but `"false"`: kustomize drops every object with that annotation once its patches have run. A document counts by what it carries, since what the merge leaves is not read: one that sets the annotation to null counts as well, and so does one whose `annotations` also carry `$patch: delete`, which removes them and leaves the object. Where the object removed is a per-layout Kustomization, the objects of its build are no longer applied, so a later untargeted patch placed by object on that Kustomization alone would never run.

kure does not read what such an entry does, so each one counts, one that changes no identity included. The entry itself is written where it was before go-kure/kure#1021 and no placement check refuses it: with a `Target` on the bundle's own Kustomization and on every per-layout one, without one on the bundle's own only. An untargeted strategic-merge patch that comes after it in `Bundle.Patches` is not placed by object and no placement check refuses it either: it stays on the bundle's own Kustomization, where every patch was before, and is written besides on each per-layout Kustomization whose build holds every object it names as generated. A bundle that renames an object of its own directory and then patches it, by the new name or the old, renders its own Kustomization byte for byte as before.

This is a limit. In three cases the build of the bundle's own Kustomization fails with `no resource matches strategic merge patch`. Before go-kure/kure#1094 it failed on the cluster; the integration now builds each Kustomization as kustomize-controller does and refuses that build, naming the Kustomization (see [One Kustomization per directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)). In the first two cases the refusal also names the later patch, by its index in `Bundle.Patches`, the entry it comes after and the object the build does not hold, with the remedy (go-kure/kure#1160). It does so only where the integration's own builds show that patch is the entry kustomize stopped at, and kustomize's error names an object of that patch the build does not hold; a patch kustomize refuses for another reason, a malformed one, keeps kustomize's message. For a patch of several documents the remedy splits it first into one patch per object, each with its target, since kustomize refuses a target on a patch of several documents. It builds the Kustomization again, then with its patches cut after each later patch in turn, and only after its build has failed. Every other failure, the third case among them, keeps kustomize's message alone. The trees stay refused: kustomize fixes back-references only after all patches, and the placement cannot see the copies a parent's patches apply, so it cannot follow what the entry does.

- A later untargeted patch names an object only a layout builds. It is on that layout's Kustomization, and on the bundle's own, which does not build the object. Put the untargeted patch before the entry in `Bundle.Patches`, or give it a `Target`.
- A later untargeted patch names the name the entry gives an object of a layout. The entry, when it has a target, is on that layout's Kustomization and renames the object there; the untargeted patch is not, since that build does not hold the new name as generated. Give the patch a `Target` that selects the new name: it is then written on every Kustomization and applied where the object is.
- An untargeted patch with a build annotation, or one that can remove its object, names an object only a layout builds. It is on the bundle's own Kustomization alone. Give it a `Target`.

Each `Target` here is for a patch of one object; a patch of several is first written as one entry per object, as above.

**A patch on a Kustomization of the bundle** lands on the Kustomization whose build holds that object, which is the one that builds the directory hosting it. The Kustomization of an application's own layout is hosted in the bundle's directory, so a patch that names it is written on the bundle's own Kustomization; the Kustomization of a layout below is hosted in the layout above it, so a patch that names it is written on that layout's Kustomization. A patch that names the bundle's own Kustomization is refused, since no build of the bundle holds it. The check that the generated Kustomizations can all become Ready reads them as generated, not as patched: what a patch changes on one of them (`dependsOn`, `wait`, `healthChecks`) is not part of it, as before.

**Bundles merged into one directory.** The shared Kustomization keeps the merged patches as [Patches in a shared directory](/api-reference/flux-names-and-paths/#patches-in-a-shared-directory) describes, untargeted ones being refused there already; a per-layout Kustomization below takes the targeted patches of the bundle that holds its application.

**Bundle labels and annotations.** In a tree walked from a cluster, `Bundle.Labels` and `Bundle.Annotations` are on the bundle's Kustomization and on the per-layout Kustomizations above, and on no object the applications generate: the walker generates each application on its own, and only `Bundle.Generate` adds them to objects.

**Refusals.** The settings in effect are checked where the Kustomization is created, before anything is written. An `Interval`, `Timeout` or `RetryInterval` must parse as a Go duration and be one Flux takes, neither negative nor positive and under a millisecond (see [Durations](/api-reference/flux-engine/#durations)). Every label and annotation in effect is checked with the validators the Kubernetes API uses for `metadata.labels` and `metadata.annotations`, key by key and then as a whole (the total size of the annotations). The error names the layout by its directory (`ManifestLayout 'prod/shop/db'`) and the field, and for an inherited entry the bundle (`inherited from bundle "shop"`). A bundle's own Kustomization carries the bundle's labels and annotations unchecked, as before: the check is made only where a per-layout Kustomization takes them. The two refusals of an untargeted patch are in [Patches of a bundle with per-layout Kustomizations](#patches-of-a-bundle-with-per-layout-kustomizations).

**A second integration** keeps the per-layout Kustomization the first one placed, as it is: its settings and patches are not written again, so a field changed on the layout or the bundle in between is not on it. A kept Kustomization still counts as a build that can hold a patch's object, so a second integration of an unchanged tree refuses nothing the first took and changes no byte: the patch for an object of a layout is on that layout's kept Kustomization already. A patch the bundle got, lost or changed between two integrations of one tree is not written on a kept Kustomization and not taken off it: the kept Kustomization then carries the patches of the first integration, stale like every other setting on it, and a new untargeted patch whose object only a kept Kustomization builds is written on none. Remove the kept Kustomization or walk the cluster again.

**Readiness through the chain.** With `wait`, a Kustomization health-checks every object it applied, the Kustomization objects among them, and becomes Ready only after that check. A Kustomization object is healthy once the controller has observed its current generation and its `Ready` condition is true. With `Bundle.Wait` set, the bundle's Kustomization is therefore Ready only when the Kustomization of each application directory is, which is Ready only when the Kustomization of each layout below it is, each of them Ready only when the objects in its directory are: a `dependsOn` on the bundle's Kustomization waits for the workloads, where it used to wait only until each directory had been applied. This is how kustomize-controller v1.10.1 reads, with the status rules of fluxcd/cli-utils v1.4.0, and it holds for a first apply and for every change of a Kustomization's own spec. Two limits:

- **A change of content only.** When a new revision of the source changes the files in a layout's directory and leaves the layout's Kustomization object as it was, the Kustomization above finds that object unchanged, with its generation observed and `Ready` true from the revision before. The health check does not look at the revision an object last applied, so the Kustomization above can be Ready for the new revision before the one below has applied it.
- **Nested timeouts.** A health check runs under its own Kustomization's timeout (`spec.timeout`, or the interval less 30 seconds without one) and has to outlast everything below it. One timeout inherited on every level gives the innermost Kustomization as long as the outermost, which then fails at the moment the innermost would. Set a shorter `Timeout` on the layouts below: the layout fields are how the layouts get a shorter timeout than their bundle. Flux raises any timeout under 30 seconds to 30 seconds, so a shorter value gives no margin below that: the timeout above has to exceed the timeout in effect below, which is never less than 30 seconds.

Under `ResourceGenerator.UmbrellasDoNotWait` the chain is not nested waits: no Kustomization that applies another one waits, each leaf does, and a `dependsOn` on an umbrella waits on its terminal leaves instead (see [With `UmbrellasDoNotWait`](/api-reference/flux-umbrella-bundles/#with-umbrellasdonotwait)).

**Wait and a dependency on the layout above.** A layout that lists the application layout above it in `DependsOn` (a hook group that applies after the application's own objects) cannot be combined with `wait` on that application layout's Kustomization: it would wait for the hook group's Kustomization, which waits for it. With `Bundle.Wait` set both now have `wait`, and the reconcile-order check refuses the tree, naming the chain, where it rendered before. Set `Wait` to a pointer to `false` on the application's layout; the layouts below keep the bundle's. Under `UmbrellasDoNotWait` the application layout's Kustomization is an umbrella and the hook group is inside its subtree, so that dependency is refused with `ErrUmbrellaDependency`.

## Node Kustomizations

A node whose directory renders no bundle (a group of nodes, or a node above its bundle's own directory under `BundleGrouping: GroupByName`) is applied by a Kustomization of its own. Three fields of `stack.Node` name and order it:

<!-- doc-example: pkg/stack/fluxcd ExampleLayoutIntegrator_nodeKustomization -->
```go
// Two groups of nodes, neither with a bundle of its own. Each group gets
// a Kustomization that applies its directory; the one for apps waits for
// the one for platform.
source := &stack.SourceRef{Kind: "GitRepository", Name: "flux-system", Namespace: "flux-system"}
platform := &stack.Node{Name: "platform", KustomizationName: "platform", Children: []*stack.Node{
    {Name: "cert-manager", Bundle: &stack.Bundle{Name: "cert-manager", SourceRef: source}},
}}
apps := &stack.Node{Name: "apps", KustomizationName: "apps", DependsOn: []*stack.Node{platform}, Children: []*stack.Node{
    {Name: "shop", Bundle: &stack.Bundle{Name: "shop", SourceRef: source}},
}}
cluster := &stack.Cluster{Name: "prod", Node: &stack.Node{Name: "prod", Children: []*stack.Node{platform, apps}}}

rules := layout.DefaultLayoutRules()
rules.FluxPlacement = layout.FluxIntegratedPerLayout
ml, err := fluxcd.NewLayoutIntegrator(fluxcd.NewResourceGenerator()).CreateLayoutWithResources(cluster, rules)
if err != nil {
    panic(err)
}
for _, obj := range ml.Resources {
    kust, ok := obj.(*kustv1.Kustomization)
    if !ok {
        continue
    }
    var waitsFor []string
    for _, dep := range kust.Spec.DependsOn {
        waitsFor = append(waitsFor, dep.Name)
    }
    fmt.Println(kust.Name, kust.Spec.Path, waitsFor)
}
```
<!-- doc-example:end -->

It prints `platform prod/platform []` and `apps prod/apps [platform]`: each group's Kustomization has the name set on the node and applies the node's directory, which the field does not move.

| Field | Effect |
|---|---|
| `KustomizationName` | names the node's Kustomization instead of the derived `-node` name. The name in effect, set or derived, is held to the [per-layout name rule](#per-layout-name-rule): a derived name over 63 characters is refused until the node sets this field |
| `DependsOn` | nodes whose Kustomizations this one waits for. Each is written into `spec.dependsOn` under the target's name in effect: its `KustomizationName`, or its derived `-node` name. A target that has no Kustomization of its own is refused, and the error names both nodes |
| `NamedDependsOn` | Kustomization names, written as given after the `DependsOn` entries. An entry is a caller-supplied reference: it need not belong to this cluster and its value is not checked, as for a bundle's. An empty entry, a repeated one, and one that names the Kustomization of a node in `DependsOn` are refused, the node named by its path |

The three fields are refused, not ignored, wherever the node has no Kustomization of its own. The error names the node by its path from the root (`node "prod/apps"`) and the fields it sets:

- under `FluxSeparate` or `FluxIntegratedPerBundle`, and in `GenerateFromCluster` and `GenerateFromLayout`: only bundles get a Kustomization there;
- on a node that `NodeGrouping: GroupFlat`, or a `FlattenSingleTier` collapse, renders into another node's directory;
- on a node whose directory renders a bundle: that bundle's Kustomization applies the directory, so name and order the bundle (`Bundle.KustomizationName`, `Bundle.DependsOn`);
- on the node whose directory is the top of the written tree, which the Flux bootstrap applies;
- on a node whose layout a caller marked `UmbrellaChild` after the walk.

The ArgoCD workflow generates nothing for a node and refuses the fields as well. A cycle through node dependencies is refused by the reconcile-order check, like any other.

Set `KustomizationName` and `NamedDependsOn` before the cluster is walked. The walker copies both onto the node's layout (`ManifestLayout.KustomizationName`, `ManifestLayout.DependsOn`) and the Kustomization is built from the layout, so a name or an entry the node gets between `WalkCluster` and `IntegrateWithLayout`, or before a second integration, is not there. That is refused, naming the node and its layout: walk the cluster again, or put the value on the layout. A name a caller sets on the walked layout wins over the node's. `DependsOn` is read from the node at integration.

A second integration of the same tree keeps the node Kustomization the first one placed. If the node sets `DependsOn` or `NamedDependsOn` and that Kustomization's `spec.dependsOn` lacks one of the Kustomizations they ask for (in the Kustomization's own namespace), the integration is refused rather than dropping the dependency: remove the kept Kustomization, add the entry to it, or walk the cluster again.

A node Kustomization inherits no settings and `stack.Node` has no field for any. `spec.interval` and `spec.prune` are those the node's layout sets (`ManifestLayout.Interval`, `Prune`) or else the generator's (`ResourceGenerator.DefaultInterval` and `Prune`). `spec.wait`, `spec.timeout`, `spec.retryInterval`, `spec.force`, `spec.suspend`, labels and annotations are those the node's layout sets (`ManifestLayout.Wait`, `Timeout`, `RetryInterval`, `Force`, `Suspend`, `Labels`, `Annotations`, on the walked layout before integration: see [Per-layout settings](#per-layout-settings)) and are left out otherwise. It takes no `spec.postBuild`, no `spec.patches` and no `spec.serviceAccountName`: those are a bundle's, and a node's layout has no holding bundle. What a group node applies is the Kustomizations of the nodes below it; the settings of the workloads stay on their bundles. Without `Wait` on its layout, a node Kustomization is therefore Ready once those Kustomization objects are applied, and a dependency on it (`Node.DependsOn`) waits for that, not for the workloads below.
