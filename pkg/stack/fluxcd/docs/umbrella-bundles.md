# Umbrella Bundles

A `Bundle` with a non-empty `Children` slice becomes an **umbrella**: a parent
Flux Kustomization that aggregates the readiness of its children, via
auto-generated `spec.healthChecks` or, where its build applies them, via
`spec.wait`. This gives downstream
consumers a single stable anchor regardless of how many internal tiers the
umbrella contains. A generator that sets `OmitUmbrellaChildHealthChecks` gives
up the first of the two (see
[With `OmitUmbrellaChildHealthChecks`](#with-omitumbrellachildhealthchecks)).

## Resource generation

`ResourceGenerator.GenerateForBundle` detects umbrella bundles and, when `Bundle.Wait` is unset
or false:
- prepends one `HealthChecks` entry per direct child (referencing the child's
  own Kustomization by name/namespace; the name is the child's `KustomizationName` when it sets
  one), unless the generator sets `OmitUmbrellaChildHealthChecks` (see
  [With `OmitUmbrellaChildHealthChecks`](#with-omitumbrellachildhealthchecks))
- leaves user-supplied `HealthChecks` appended after the auto entries

With `Wait` true it writes no entry for the children: upstream documents that when `wait` is
enabled "the HealthChecks are ignored", so an entry would make nothing wait. Each child's
Kustomization name is checked all the same. The caller's own `HealthChecks` are handled as
they were before the child entries were dropped under `wait`, including the exception `Wait`
already made for a check on `b`'s own Kustomization (below); [One Kustomization per directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)
says how the other entry points map them.

`spec.wait` is **not** forced to `true`; it is the caller's `Bundle.Wait` input like any other
field. With `wait`, a Kustomization waits for every object its own build applied (see
"Readiness through the chain" under [Per-layout settings](/api-reference/flux-child-layout-crs/#per-layout-settings)), so whether that
covers the children depends on where their Kustomization objects are written:

- **`FluxIntegratedPerBundle` and `FluxIntegratedPerLayout`:** each child's Kustomization object
  is written in the umbrella's directory (see [Placement in layouts](#placement-in-layouts)),
  so the umbrella applies it and its wait covers the child's `Ready`, with the two limits of
  "Readiness through the chain".
- **`FluxSeparate`:** the children's Kustomization objects are not in the umbrella's build
  (they are in `flux-system`, which the root applies), so `wait: true` does not wait for the
  children. A caller who needs the umbrella to wait for them there leaves `Wait` unset or false,
  so the generated entries take effect. This gap was there before the entries were dropped
  under `wait`, when Flux ignored them, and dropping them does not change it. Those entries hold
  a kustomize-controller worker while the umbrella waits; a caller who would rather not hold one
  sets `OmitUmbrellaChildHealthChecks` and gives up the umbrella's wait on its children (see
  [With `OmitUmbrellaChildHealthChecks`](#with-omitumbrellachildhealthchecks)).
- **`GenerateForBundle` alone** neither generates nor places the children's Kustomization
  objects, and takes `path` as given, so whether `wait: true` covers them depends on whether the
  caller's build at that path applies them. Where it does not, leave `Wait` unset or false.

`GenerateForBundle(b, path)` is strictly self-only — it never recurses into
`b.Children`. `GenerateFromLayout` and `GenerateFromCluster` cover the whole
umbrella closure, because the walker renders every umbrella child as a layout
of its own. It builds no origin index either, so it cannot see every bundle
that shares a Kustomization name; what it does see it checks: a `DependsOn`
bundle or a child that would get `b`'s own Kustomization name
(`KustomizationName`, or `Name` without it), `b` itself in `DependsOn` or in
`Children`, that name in `NamedDependsOn`, or a health check on `b`'s own
Kustomization, is an error, since the Kustomization would otherwise depend on
itself or wait for itself. A longer cycle of `Children` is `Bundle.Validate`'s
to refuse. Two children that would get one Kustomization name are an error as
well, whatever `Wait` says: two Kustomizations would share the name, and
without `wait` the umbrella would health-check it twice. The health
check on `b`'s own Kustomization stays accepted when `b.Wait` is true, and is
written as given: Flux ignores `spec.healthChecks` under `spec.wait`, so
nothing waits for itself. `GenerateFromLayout` and `GenerateFromCluster` drop
a bundle's dependency or health check on itself instead.

The name a Kustomization gets is the bundle's name in effect (`UnitName()`:
its `KustomizationName`, or its `Name` without one), so the generator refuses
one that `stack.ValidateKustomizationName` refuses: not a DNS-1123 subdomain,
or longer than 63 characters. The check runs before anything is written: in
`GenerateFromCluster`, `GenerateFromLayout`, `GenerateForBundle` and the
layout integrator under every `FluxPlacement`. The 63-character limit is
checked only there. It is Flux's, so `stack.ValidateCluster` and
`Bundle.Validate` accept a name of up to 253 characters, which the
ArgoCD workflow renders; they do refuse a `Name` or `KustomizationName` that
is not a DNS-1123 subdomain, so that refusal reaches `GenerateFromCluster` and
`CreateLayoutWithResources` from cluster validation first. A `Name` over 63
characters beside a `KustomizationName` within the limit generates: that
`Name` is no Kustomization's. The error names the bundle by `Bundle.GetPath`,
an umbrella child of a walked cluster with its umbrella's path before its own
name (`platform/platform-infra`), and the field that holds the name.

Which names are checked follows from what the entry point returns:

- `GenerateForBundle` takes a bundle no validation has seen and returns one
  Kustomization. It checks the bundle's own name and the names that
  Kustomization refers to: each umbrella child, named in a health check
  (checked too where no health check is written: with `Wait` true,
  `UmbrellasDoNotWait` or `OmitUmbrellaChildHealthChecks`), and each
  `DependsOn` bundle, named in `spec.dependsOn`. It builds no
  Kustomization for them, so it refuses an umbrella whose child's name Flux
  cannot reconcile instead of returning a reference to it.
- `GenerateFromCluster`, `GenerateFromLayout` and the layout integrator build
  one Kustomization per directory that renders bundles, and check that
  Kustomization's name, which is its first bundle's (see "One Kustomization
  per directory"). The name of a bundle merged into it is written nowhere and
  is not checked. A reference to one of the cluster's bundles, an umbrella
  child or a `DependsOn` bundle, is written as the name of the Kustomization
  that applies that bundle, which the same call builds and checks. A
  `DependsOn` bundle that is not one of the cluster's bundles gets no
  Kustomization from the call and is written under its own name, so that name
  is checked like the bundle's own.
- A `NamedDependsOn` entry and a health check the caller lists are not
  checked. `GenerateForBundle` writes them as given; the other entry points
  write one that names a rendered bundle's Kustomization as the name of the
  Kustomization that applies that bundle, and any other as given.

## Placement in layouts

`LayoutIntegrator` places umbrella child Flux CRs at the **parent** layout
node, with `spec.path` = the child's own directory:

- **Integrated, `BundleGrouping: GroupByName`**: the walker creates a bundle sub-layout
  under the node layout. Umbrella child Kustomization CRs are appended to the
  bundle sub-layout's `Resources`, which that layout lists (it is written in
  `KustomizationExplicit` mode); their Source CRs, if the child has a
  `SourceRef.URL`, go to the root node's layout like every generated Source. Nested umbrella children are placed at their
  enclosing umbrella child's layout node.
- **Integrated, `BundleGrouping: GroupFlat`**: there is no intermediate bundle
  layer, so umbrella children become direct sub-layouts of the node layout,
  and their Flux CRs sit at the node layout. The root node is the exception
  (go-kure/kure#979): its bundle has a directory of its own inside the root
  node's, and the umbrella children and their Flux CRs sit there.
- **FluxSeparate**: the `flux-system` layout directory receives every bundle's
  Kustomization CR, umbrella descendants included, as a flat list.

Under both integrated placements a node bundle's own CR is hosted by the
**parent** of the layout that renders the bundle: that directory is applied by
its own Kustomization only, so the CR that creates it cannot live inside it.
The root node's bundle has a parent too, the root node's layout
(go-kure/kure#979).

## On-disk shape

When a parent layout has an umbrella child, the parent's `kustomization.yaml`
lists the child's Kustomization CR file (it is one of the parent's own
resources) instead of the child subdirectory. The child subdirectory still exists and still contains its own
`kustomization.yaml` plus workload YAML files — but **no** Flux CR files, so
Flux does not double-apply the child's resources.

## With `UmbrellasDoNotWait`

`ResourceGenerator.UmbrellasDoNotWait`, off by default, keeps every umbrella from holding a
kustomize-controller worker. A Kustomization that waits (with `wait`, or on health checks) holds
one of the controller's workers (`--concurrent`, 4 by default) until what it waits for is Ready.
A `dependsOn` that is not met requeues instead and holds none. A tree with as many waiting
umbrellas as workers, each waiting for a child that needs a worker to reconcile, stalls: every
worker waits and no child is applied. With the setting, readiness flows through `dependsOn`
instead of nested waits:

- **R1: an umbrella does not wait.** An umbrella here is any Kustomization the call generates
  whose build applies a Flux Kustomization: a bundle with `Children`, a directory that hosts the
  CRs of child nodes or, under `FluxIntegratedPerLayout`, of the layouts inside it, and one whose
  objects include a Flux Kustomization, an `AppFileSingle` application's file written into its
  directory among them. It gets no `wait` and no `healthChecks`, the automatic
  entries for its children included. An explicit `Wait` true that reaches it (the bundle's, the
  layout's, or the one a layout inherits from the bundle that holds its application) and
  `HealthChecks` on it are refused with `ErrUmbrellaWait`, naming the Kustomization and where the
  input comes from: the setting exists to remove that wait, so it is not dropped without a word.
- **R2: a leaf waits.** An unset `Wait` on any other Kustomization resolves to `true`, so it is
  Ready only once its objects are. An explicit `false` is kept. Bundles merged into one directory
  are compared on the resolved value once the pass knows what their shared Kustomization is: an
  unset `Wait` and an explicit `false` conflict on a leaf, and not on an umbrella, which R1 writes
  without a wait either way.
- **R3: a dependency on an umbrella is one on its terminal leaves.** A `dependsOn` entry that
  names an umbrella is replaced by entries naming its terminal leaves: the Kustomizations it
  applies that no other one it applies depends on, an umbrella among them replaced by its own
  terminal leaves in turn, each listed once. R3 writes a plain entry (no `readyExpr`) on each
  terminal leaf. The rewrite's result check demands a plain entry on every leaf wherever the
  caller's entry on the umbrella waited for `Ready` or an expression; an existence-only entry is
  satisfied by the umbrella entry kept as it is, or by any entry on each leaf. Dependencies are
  compared as R3 writes them: a
  generated Kustomization's `dependsOn` entry naming an umbrella of the subtree stands for its
  terminal leaves, and a Kustomization whose entries together await every terminal leaf of an
  umbrella depends on it, so a tree the setting wrote has the same terminal leaves. A Flux
  Kustomization an application emits is a leaf by its own name, also when it is an item of a
  `List`; its `dependsOn` is not rewritten, so an entry there naming an umbrella awaits none of
  its leaves. Its spec is read in the same way whatever form the application hands it over in
  (the typed object, its own type embedding one, another API version, or unstructured); one
  that cannot be read in typed form is refused with the read error under the setting. A
  Kustomization inside the umbrella's
  subtree that depends on it is refused with `ErrUmbrellaDependency`, since it would depend on
  itself, and so is an umbrella without a terminal leaf. So is a rewrite onto a terminal leaf
  that cannot become Ready before the dependent applies: one that depends on the dependent, or
  that health-checks it or waits for it (an application's Kustomization read as its author wrote
  it, in its context: see below), or that the dependent's apply creates, directly or through Kustomizations applications
  emit (an `AppFileSingle` application's file included), also when the dependent already names
  the leaf itself. The edge onto the umbrella closed no cycle the reconcile-order check sees;
  the edge onto the leaf closes one, and the error names the chain. A health check is not
  rewritten, nor is a Flux Kustomization the integration did not place (an application's, or the
  caller's): a Kustomization health check naming an umbrella, on any Kustomization but an
  umbrella, and such a Kustomization's `dependsOn` naming one are refused with
  `ErrUmbrellaDependency` too, since the umbrella does not wait and so is Ready before what it
  applies. The error names the terminal leaves to name instead. Under
  `AdditiveCELDependencyCheckOff` an entry whose `readyExpr` is exactly `"true"` waits only for
  the dependency to exist, so it is none of these: it is kept as written and judged by no rule
  here. When terminal leaves are picked, an entry with any `readyExpr`, with the setting or
  without, makes no Kustomization it names depended on: Flux skips its own readiness check for
  it, so a dependent of the umbrella also waits for that Kustomization. Where a Flux
  Kustomization of the tree sets `spec.patches` or `spec.components`, which may rewrite such an
  expression, these rules read each entry as the build of the tree shows it, patches applied. Only
  when that build cannot be made do they read the tree as it holds it, and then every entry counts
  as waiting for `Ready`, as without the setting, wherever that decides a refusal; the terminal
  leaves follow the same reading.

These checks read a Flux Kustomization in its context. One the integration places or keeps builds
the directory of the tree its `spec.path` names. For any other (an application's, or the
caller's), `spec.path` is a directory of the tree, and its `wait` waits for what that directory's
build applies, only when it reads the Source of the Kustomization whose apply writes it (its `spec.sourceRef`, an omitted
namespace its own; the Source's identity, so another Source of the same repository, at another
ref, is another Source) and sets no `spec.kubeConfig`. Such a Kustomization the Flux bootstrap
writes, one on another Source, and one applying to another cluster build no directory of this
tree: they add no wait, and are never refused for one. Nor does one whose `spec.path` names an
`AppFileSingle` layout other than the top of the tree: that layout is one file written into its
`Namespace`, with no directory of its own, so Flux finds no path there and the Kustomization
creates, keeps and waits for none of the tree's objects (go-kure/kure#1197). Such a file never
stands for a directory of the same path beside it, which is read as any other. Their `dependsOn` and health checks are
still judged: such a Kustomization that depends on, or health-checks, one that cannot be Ready
before it is refused as any other. One that transforms its build (`spec.patches`,
`spec.components`, `spec.namePrefix`, `spec.nameSuffix` or `spec.targetNamespace`) waits for
what that build applies, read as kustomize-controller builds it: a renamed or moved Kustomization
under its built name, one a patch deletes not at all, and one marked as local config only when
a patch keeps it (kustomize patches before it drops local config). A health check on another cluster
(`spec.kubeConfig`) names none of the tree's Kustomizations. Health-check expressions on Flux
Kustomizations (`spec.healthCheckExprs`) are read conservatively, as readiness: an expression is
arbitrary CEL and may itself wait for the `Ready` condition, so the check, and what a `wait`
polls, waits for the checked Kustomization to be Ready, whatever the expression says.

These checks read what Flux's apply and wait do with a Kustomization's spec, as follows
(go-kure/kure#1094):

- **Objects an apply skips.** A Flux Kustomization that a Kustomization's apply skips is left out
  of what its `wait` waits for. Skipped means `kustomize.toolkit.fluxcd.io/ssa: Ignore` or
  `kustomize.toolkit.fluxcd.io/reconcile: disabled`, as a label or annotation in any case, on the
  object with the Kustomization's `spec.commonMetadata` applied over it. It is read on the object
  as that Kustomization's build makes it, so a patch that sets or clears the mark counts. `ssa: IfNotPresent` is
  not skipped: the apply creates the object and waits for it. A health check on a skipped object
  still waits for it. A mark in `spec.commonMetadata` is that Kustomization's alone, so another
  apply, the root's included, still creates the object. A mark on the object itself skips it in
  every apply whose `spec.commonMetadata` does not override it. If every apply skips it, the
  object never exists: a stall, which the checks do not judge.
- **A health check on the checker itself.** As Flux does, a check that carries the checker's own
  namespace and name is dropped. One with no namespace never passes: it is refused, saying so.
- **`spec.ignore`.** A Kustomization's `spec.ignore` holds drift rules, which a fresh install does
  not use. A Source's `spec.ignore` and `.sourceignore` files are not read.
- **`spec.suspend`.** A cycle through a suspended Kustomization stays refused: a suspended
  Kustomization never applies on a fresh install, so the tree would not converge either.
- **`spec.healthCheckExprs` on Flux Kustomizations.** An expression is read as readiness,
  whatever it says. Flux evaluates it after its own generation guard, and it is arbitrary CEL, so
  a check written as an expression that waits for nothing more is still refused. Drop the check,
  or order the two Kustomizations with a `dependsOn` that closes no cycle.
- **An alternate creator.** A check on what a generated Kustomization's build writes waits for
  that apply, even where another Kustomization of the tree writes it as well and would apply
  first. Let one Kustomization write the target.

They read each build as kustomize-controller builds it (see [One Kustomization per
directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)): a generated Kustomization's patches rewriting a
child Kustomization's spec, `spec.postBuild` substitution and a build's own transforms are applied
before anything is read, and a Kustomization a patched build keeps as local config is read as that
build makes it, its own `wait` included. A built object is read as its JSON, so a number in it,
a port say, is read like any other field, before and after substitution (go-kure/kure#1199). An
unstructured object a caller builds in Go should hold JSON-typed values, as apimachinery's
`Unstructured` requires: an `int64`, not an `int`, because its `Nested*` reads and `DeepCopy`
panic on any other. kure reads such an object as its JSON, so it does not panic on one
(go-kure/kure#1201, go-kure/kure#1252).
`IntegrateWithLayout`, `CreateLayoutWithResources` (through it),
`GenerateFromLayout`, `GenerateFromCluster` (through it) and `UmbrellaTerminals` first replace the
`Object` map of each unstructured object in the layouts they are given, a `List`'s items included
(also the unstructured items of a typed `List`, and of the typed Lists it holds, an unstructured `List`
among them, whose own `Object` map is replaced like its items', and an unstructured object, or `List`,
that a typed object that is no `List` holds in a field, the `Object` of a `RawExtension` included, such a
typed object as an item of a typed `List` too), that holds another value with a copy that holds
JSON-typed values. This changes the object the caller passed: its `Object` field holds a new map, and
the map the caller built is not changed. A typed `List` is read from the items it writes, so an object
it holds in a field and does not write is left as it is. An unstructured object or `List` held by value in a
map or in an interface of a typed object that is copied, which cannot be replaced, is refused when it
holds a value that is not JSON typed. The accepted shapes are a closed list. A `string`, `int64`,
`float64`, `bool`, `nil`,
`json.Number`, `map[string]interface{}` and `[]interface{}` are kept as they are. A Go integer kind
(`int`, `int8` to `int32`, `uint`, `uint8` to `uint32`, `uintptr`, and a `uint64` up to the largest
`int64`) becomes an `int64`, so an integer above 2^53 stays exact. A `float32` becomes the
`float64` of its shortest 32-bit decimal form, as `encoding/json` writes it (`float32(0.1)` is
`0.1`), and a byte slice becomes its base64 string, as `encoding/json` writes it. A named `string`,
`bool`, integer or float kind becomes its base type. A map with string keys and a slice of any
element type (`[]string`, `map[string]string`) are converted element by element. Anything else is refused instead of converted: a struct, a pointer, an array, a channel, a
function, a complex number, a map with other keys, a `uint64` above the largest `int64`, and every type
with a `MarshalJSON` or `MarshalText` method (`json.RawMessage` and `net.IP` among them). So is a value
that holds itself through maps and slices, one nested deeper than 10,000 levels (the depth `encoding/json` takes), a call whose objects
need more than 4 million values converted or checked in all, and a byte slice of more than 4 million bytes. This is a contract of every entry point, stricter than the code before it (go-kure/kure#1201): everything a caller passes is normalized at entry, including a value that no path of that entry point reads, so an unstructured object holding a byte slice of more than 4 million bytes is refused by an entry point that never reads the slice. The objects of one call that need conversion or a check share the one budget of 4 million values (the bytes of a byte slice count as values each time the slice is converted or checked, and an object of a typed `List` is checked before the `List` is written); an unstructured object that holds JSON-typed values only is kept as it is and is not charged to it. An unstructured object held in a field of a typed object, which needs conversion, is checked and then converted, so each of its values counts twice, as the values of a `List`'s items do. A value past a bound of this section is refused even where origin/main would accept it: the bounds are part of the contract of the package, not a measure of what main takes. A refusal names the layout, the object
and the field, and replaces no object. The walk never calls a method of a caller's value, a marshaler
included, also not to read a typed `List` that holds such an object; the metadata methods are called by the entry probe described below, a pass of its own. A typed object is walked, field by
field, before it is written, and refused when it holds itself, when it is nested deeper than 40,000
steps (a pointer, a field, an element, a map value, a map key of interface type and an embedded struct are each one step, and an interface is none, so a value past this bound is refused even where `encoding/json` and origin/main would take it), when the walk of that one typed object meets more than 4 million values (an interface is one of them) or when an unstructured object in it holds a
value that would call a method when written. A typed object that holds a type with a `MarshalJSON`,
`MarshalText` or `MarshalYAML` method, as a field, an element or a map key, is refused, naming the
type, unless the type is from `k8s.io`, `sigs.k8s.io`, `github.com/fluxcd`, `github.com/go-kure` or
one of the standard library packages that define such a type (`time`, `net`, `net/netip`, `net/url`,
`math/big`, `regexp`, `log/slog` and `encoding/json`), or is the Cilium `XDSResource` of a Cilium Envoy
config, or the Cilium policy `EndpointSelector` (its `IsZero` tests one field for nil, and kure's own CiliumNetworkPolicy builders emit it). Each is an audited type, trusted by its exact name. The `XDSResource` marshaler writes a protobuf `Any` with `protojson`, which resolves the type URL in the global protobuf registry, so a message type a caller registered there still runs. A field tagged `omitzero` whose type has an `IsZero` method is judged the same way, as `encoding/json` calls that method when it writes the field: the typed object is refused, naming the type, unless the type is from one of those packages or is one of the exact types listed above. For an interface field the type judged is the one the interface holds, and a nil pointer, or an interface that holds one, is not judged, since `encoding/json` calls no `IsZero` on it. A type that only promotes `IsZero` from an embedded type is judged by its own package. An embedded struct (or pointer to one) with no name in its tag is flattened by `encoding/json`, which calls no `IsZero` for it, so it is not judged. A struct embedded under a name and an embedded interface are fields, and are judged. A field tagged `json:"-"`, which `encoding/json` never writes, is walked, charged and judged like any other field, so an untrusted `IsZero` inside it is refused: `encoding/json` does not write it, but a marshaler or kure's own code can (a `RawExtension` marshals its `Object`, which is tagged `-`), and each exemption tried here left a hole. The check does not copy the rest of how `encoding/json` picks fields, and refuses where that choice is subtle: a field with the same JSON name as a shallower field, which `encoding/json` drops, is still judged, and so is an embedded struct under a tag name that is not valid for JSON. A refusal is the safe side, and a copy of those rules could miss a method that is called. A `RawExtension` whose `Object` holds its own List is refused whatever `Raw` holds, since a copy of the object follows `Object`. A type of `package main` is not trusted. A named struct that only promotes such a method from an embedded type of a trusted package (a type that embeds `*unstructured.Unstructured`) is accepted; one that declares the method itself is not. A wrapper that embeds an `unstructured.Unstructured` (by pointer or by value, directly or through another embedded struct, as a resource or as an item of a typed List) is not normalized: it is refused, naming its Go type, when the map it embeds holds a value that is not JSON typed, and kept as it is when it holds none, since a copy put in its place would no longer carry what is done to the wrapper. Hold the object as a plain `*unstructured.Unstructured` to have it converted. The object that counts is the one Go promotes the methods of, the shallowest; a wrapper that embeds two at the same depth is refused, naming both paths. A wrapper with a `DeepCopyObject` of its own, one embedded on the way to the object that has one, or one that embeds a nil pointer, is refused. So is a wrapper that embeds an interface, or a named type that is not a struct, with a `DeepCopyObject` above the object, where Go promotes that method and not the object's, or at its depth, where Go promotes neither: it is refused whatever the map holds, naming the type and the path to the embedded interface or type. An embedded interface or non-struct type without that method, or one deeper than the object, changes nothing. This holds for a wrapper around an `unstructured.UnstructuredList` as well. A typed resource whose own method panics when kure reads
its kind (a struct that embeds a nil pointer) is refused, naming the layout, the resource's place in it
and its Go type; so is a typed nil resource, an unstructured one included. The paths below guard a
call to a method of a caller's object, and a panic in any is a refusal that names only the Go types
of the object and of the panic value, never the panic's text:

- the read of a built tree, and the labels and annotations that `Bundle.Generate` gives the objects of
  its applications;
- the entry probe (go-kure/kure#1245): `IntegrateWithLayout`, `CreateLayoutWithResources`,
  `UmbrellaTerminals`, `GenerateFromLayout`, `GenerateFromCluster` and the layout writers
  (`WriteToDisk`, `WriteToTar`, `WritePackagesToDisk`, `WriteManifest`) call `GetObjectKind`,
  `GetName`, `GetNamespace`, `GetLabels` and `GetAnnotations` once on each object they are given, and
  on each object a typed `List` holds, before they read any. `GenerateFromLayout` and
  `GenerateFromCluster` call these methods only when bundles merged into one directory share a
  Kustomization and a bundle's patch has a target, which is matched against the other bundles'
  objects; they probe at entry all the same, so the contract does not depend on that shape.
  An item that is no `client.Object` is probed through the metadata it exposes with
  `ObjectMetaAccessor`, as the writers read it. The generator, the integrator and `UmbrellaTerminals`
  also probe the objects the origin records of the layouts retain (`OriginApplicationObjects`), which a
  delivery intent is applied to, and the objects each bundle of a layout emitted
  (`OriginBundleObjects`), which that patch check reads: the layouts hold the same values, but a
  caller can drop one from a layout and leave it in a record. The probe is a separate pass that
  follows the normalization above. It changes no caller's object; the one write it makes is the one
  every read of `OriginBundleObjects` makes, copying the annotations of a `configMapGenerator` entry
  to its stand-in, which is kure's own object. A nil left in the record of a bundle's emitted objects,
  a typed nil pointer or a nil interface, is refused at entry with an error naming the layout and the
  bundle, in the words used for a typed nil in a layout's `Resources`: the patch check calls a method
  on every element of that record. An object whose metadata methods panic is refused at
  entry, whether or not a later phase would have called the method that panics, and a refused
  `GenerateFromLayout` or `GenerateFromCluster` puts back what the normalization replaced.
  `WalkCluster` reads no metadata method beyond the guarded read of the kind, and has no probe;
- the annotations an application's delivery intent sets on its objects, and takes back when a later
  check refuses the call. A setter cannot be probed, so the call that sets and the call that takes
  back are each guarded. A refusal while setting leaves the object annotated by no call. A panic
  while taking back is reported in the same error as the refusal that started it, names the Go type
  and says the object could not be restored.

The paths leave out a method that answers or panics differently on a later call than on the probe's,
and a method that never returns. A setter that panics part-way may leave the object changed, and then
the undo cannot promise to restore it.

kure reads
the items of a `List` by reflection and never formats a caller's object, so an item that is no
object cannot make it format without end. The cases the issue names (a
plain `int` in an object, nested in a `sourceRef`, in a `List`'s items, in a `Source` `spec`) all stay accepted. A
call that refuses later, for another reason, puts every `Object` map it replaced back, as long as the
caller does not change the tree during the call. What they still leave out:

- **An object a build moves.** A health check on an object's name as its file writes it, where a
  `spec.targetNamespace` or a name prefix or suffix applies it under another, names an object that
  never exists: a stall, which the checks do not judge. Name the object as the build applies it.
- **Strict substitution.** kustomize-controller substitutes strictly by default since v1.9
  (`StrictPostBuildSubstitutions`, kustomize-controller v1.10.1 `internal/features/features.go`),
  so a variable `spec.postBuild` does not set and that has no default fails the build. kure
  substitutes non-strictly and reads it as empty. That Kustomization then never applies and the
  tree stalls whatever kure reads, so the read may refuse such a tree but never admits a deadlock.
- **`readyExpr`.** Read as a wait for `Ready`, or for existence when the expression is exactly
  `"true"` and the generator sets `AdditiveCELDependencyCheckOff` (see [With
  `AdditiveCELDependencyCheckOff`](/api-reference/flux-names-and-paths/#with-additiveceldependencycheckoff)). Terminal selection and
  a node's kept `dependsOn` count only an entry with no `readyExpr` as a wait for `Ready`.
- **`openapi`.** A build that reads a kustomization file setting `openapi` is not read:
  kustomize-controller isolates its schema by resetting kustomize's global one around each build,
  which kure does not do (see [One Kustomization per
  directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)). Drop the field. kure tells a kustomization file
  by its name, so a generator's data file named `kustomization.yaml`, `kustomization.yml` or
  `Kustomization` whose content sets `openapi` is refused too, though kustomize reads it only as
  data: rename it.
- **Remote references.** A build that reads a remote reference, a Git repository or an `http(s)`
  URL kustomize-controller would fetch, is not read, naming the file and the reference: an
  integration never fetches (see [One Kustomization per
  directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)). Vendor the base into the tree, or reference it
  through a Flux Source.

A tree refused for a cycle through one of these integrates once the dependency or health check
that closes the reported chain is dropped, or names another Kustomization. The "umbrellas do not
wait" rewrite and `UmbrellaTerminals` read the Kustomizations of the tree as it holds them, and
on a tree that patches they read each `dependsOn` entry as the build shows it (go-kure/kure#1226).
So a local-config Kustomization a patched build may keep, and that cannot be read in typed form,
is still refused there.

The rules apply to the Kustomizations the call generates. One kept from an earlier integration
is not rewritten; it is refused when it breaks R1 (with `ErrUmbrellaWait`, for `wait` or any
`healthChecks`) or R3 (with `ErrUmbrellaDependency`), since it was placed without the setting.
A tree the setting wrote integrates again unchanged: a node's kept Kustomization whose
`dependsOn` names the terminal leaves of an umbrella the node depends on satisfies that
dependency, and one that names neither is refused with `ErrUmbrellaDependency`. An entry that
names a leaf only with a `readyExpr` names it for existence, so it counts as naming neither. The
reconcile-order check (see [One Kustomization per directory](/api-reference/flux-names-and-paths/#one-kustomization-per-directory)) runs on the
rewritten specs.

| Entry point | With the setting |
|---|---|
| `IntegrateWithLayout`, `CreateLayoutWithResources` under `FluxIntegratedPerBundle` or `FluxIntegratedPerLayout` | R1, R2 and R3 |
| `GenerateForBundle` | R1 and R2 on the Kustomization it returns, a bundle with `Children` being the umbrella. It sees no other Kustomization, so R3 is the caller's |
| `GenerateFromLayout`, `GenerateFromCluster`, and `FluxSeparate` placement | refused with `ErrUmbrellaWait` |

`UmbrellaTerminals(roots ...)` returns the terminal leaves of each umbrella in one or more
integrated trees, keyed `namespace/name`, so that a caller can point a dependency from outside a
tree (a `NamedDependsOn` entry) at the leaves instead of the umbrella. Only a Kustomization the
integration placed or kept can be an umbrella there: each integration records, per layout
(`layout.ManifestLayout.FluxKustomizations`), the Kustomizations it placed or kept, and one counts while
its `spec.path`, `spec.sourceRef` and presence of `spec.kubeConfig` are still the recorded ones, in
any form (typed, unstructured, a `List` item). Every other Kustomization is a leaf: one an
application emits and the integration did not keep, one a caller added, one whose Source, cluster or
path changed after the integration, and every one of a tree built by hand or read back from disk. A
caller that places its own records them with `SetFluxKustomizations`. A record counts when it equals the Kustomization's
current one, both read as kure reads identities: the path cleaned, an omitted namespace `default`,
an omitted Source namespace the Kustomization's own. A Kustomization that two trees
both hold, and a tree whose objects cannot be read, are refused with `ErrUmbrellaTerminals`.
It counts only a `dependsOn` entry with no `readyExpr` as a wait for `Ready`: a sibling that
another one names with a `readyExpr` is a terminal leaf, with `AdditiveCELDependencyCheckOff` or
without, and in a tree with a patching Kustomization or without. It runs no cycle check, so a tree
whose entries wait for each other only through a `readyExpr` returns its leaves instead of
`ErrUmbrellaTerminals`. The generator's method `ResourceGenerator.UmbrellaTerminals` returns the
same leaves; it stays for callers that already use it.

## With `OmitUmbrellaChildHealthChecks`

`ResourceGenerator.OmitUmbrellaChildHealthChecks`, off by default, writes no automatic health
check on an umbrella's children. It is for an umbrella whose children are applied by their own
Kustomizations, as under `FluxSeparate`, where the children's Kustomization objects are not in the
umbrella's build. There the automatic entries make the umbrella's reconcile wait until every
child is Ready, holding one of kustomize-controller's workers the whole time, while each child
needs a worker of its own to become Ready. `UmbrellasDoNotWait` is refused under `FluxSeparate`,
and `Wait` true makes Flux ignore the caller's own `HealthChecks` too, so this setting is the
one that drops the automatic entries alone.

- **What it changes:** an umbrella's Kustomization gets no entry per child, whatever `Wait`
  says, in every placement and from every entry point (`GenerateForBundle`,
  `GenerateFromLayout`, `GenerateFromCluster`, the layout integrator). No other field changes:
  the caller's own `HealthChecks` are written and mapped as without it, and each child's
  Kustomization name is still checked. The reconcile-order check reads health checks as edges,
  so it no longer sees one from the umbrella to each child.
- **The trade-off:** with `Wait` unset or false the umbrella no longer waits for its children:
  it is Ready once applied and its own `HealthChecks`, if any, pass, so a bundle that depends on
  it no longer waits for its children. To keep that, list the children's Kustomizations in the
  umbrella's `HealthChecks` (which brings the worker wait back for that umbrella), or depend on
  the children directly; `UmbrellaTerminals` names the last ones to become Ready in an
  integrated tree. With `Wait` true the field changes nothing in any placement, since no entry
  was written before either; under an integrated placement the umbrella's wait still covers its
  children, under `FluxSeparate` it never did.
- **A kept Kustomization** from an earlier integration under an integrated placement stays as
  written, child entries included, as every kept CR does. Under `FluxSeparate` an earlier
  `flux-system` child written with the entries no longer matches, and the integration is
  refused; integrate a freshly walked layout.
- **Off:** every tree renders as without the field.

<!-- doc-example:excerpt setting the field on a generator; the behaviour is pinned by pkg/stack/fluxcd/umbrella_child_checks_test.go -->
```go
gen := fluxcd.NewResourceGenerator()
gen.OmitUmbrellaChildHealthChecks = true
integrator := fluxcd.NewLayoutIntegrator(gen)
```
