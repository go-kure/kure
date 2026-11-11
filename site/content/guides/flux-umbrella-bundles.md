+++
title = "Umbrella Bundles"
weight = 22
+++

A bundle with children aggregates their readiness. This page covers how, and how to opt out.

The Go blocks on this page are the bodies of `Example` functions in `pkg/stack/fluxcd`, which `go test` runs. They use the imports and the helpers named in the [tutorial](/guides/flux-workflow/).

A bundle with non-empty `Children` becomes an **umbrella**: Flux will only mark
its Kustomization `Ready` once every child Kustomization is `Ready`. The Flux
engine enforces this by prepending an auto `spec.healthChecks` entry for each
direct child, unless the generator sets `OmitUmbrellaChildHealthChecks`
([below](#umbrellas-without-child-health-checks)).

`spec.wait` is not forced. It is `Bundle.Wait` like any other input, and leaving
it unset is what makes the `healthChecks` entries take effect — upstream ignores
`healthChecks` when `wait` is enabled.

The resulting umbrella Kustomization aggregates child readiness regardless of
how many children there are, giving external consumers a single stable anchor:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowUmbrella -->
```go
umbrella := &stack.Bundle{
    Name: "platform",
    Children: []*stack.Bundle{
        {Name: "platform-infra"},
        {Name: "platform-services"},
        {Name: "platform-apps"},
    },
}

objects, err := fluxcd.Engine().ResourceGen.GenerateForBundle(umbrella, "production/apps/platform")
if err != nil {
    panic(err)
}
ks := objects[0].(*kustv1.Kustomization)
for _, hc := range ks.Spec.HealthChecks {
    fmt.Println(hc.Kind, hc.Namespace, hc.Name)
}
```
<!-- doc-example:end -->

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
becomes `Ready` only when every child is, through those health checks (unless
the generator sets `OmitUmbrellaChildHealthChecks`). With `Wait: true` no
entry is written for the children, since Flux ignores `healthChecks` under
`wait`; the user-supplied entries are handled as before. The integrated
placements then still wait for the children, whose Kustomization objects the
umbrella applies; `FluxSeparate` does not, so leave `Wait` unset there for the
umbrella to wait for them. With `GenerateForBundle` alone it depends on whether
the caller's build at the given path applies the children's Kustomization
objects.

Umbrella children must be **standalone** — a bundle cannot simultaneously be
the `Bundle` of a `stack.Node` and appear in another bundle's `Children`.
`stack.ValidateCluster` rejects any such overlap before resource generation.

## Umbrellas that do not wait

A Kustomization that waits, on `wait` or on health checks, holds one of
kustomize-controller's workers (4 by default) until what it waits for is
Ready, while an unmet `dependsOn` only requeues. A tree with as many waiting
umbrellas as workers can stall with every worker waiting on a child that
never gets one. Setting `ResourceGenerator.UmbrellasDoNotWait` moves readiness
onto `dependsOn` instead:

<!-- doc-example:excerpt setting the field on a generator; the rules are pinned by pkg/stack/fluxcd/umbrellas_do_not_wait_test.go -->
```go
gen := fluxcd.NewResourceGenerator()
gen.UmbrellasDoNotWait = true
integrator := fluxcd.NewLayoutIntegrator(gen)
```

- every Kustomization that applies a Flux Kustomization (an umbrella) gets no
  `wait` and no health checks, and an explicit `Wait: true` or `HealthChecks`
  on one is refused;
- every other Kustomization with `Wait` unset waits;
- a `dependsOn` on an umbrella names its terminal leaves instead: the
  Kustomizations it applies that none of its other ones depends on, expanded
  through nested umbrellas. A rewrite onto a leaf that cannot become Ready
  before the dependent applies (one that depends on, waits for or
  health-checks it, or that it creates) is refused, and so is a health check
  on an umbrella, and a `dependsOn` on one in a Flux Kustomization an
  application emits: the integration rewrites neither.

The pass reads each `dependsOn` entry as the build of the tree shows it, with
patches applied, before and after the rewrite. A patch that works on the
rewritten list (`remove /spec/dependsOn/1`, say), that adds a `Ready` entry on
an umbrella, or that copies a `dependsOn` name into another field, is refused
when the build after the rewrite waits for less than the build before it. A
patch that never touches `dependsOn` no longer refuses a tree that holds a
`readyExpr: "true"` entry. A tree the build before the rewrite cannot read keeps
the earlier, blunter refusal of every patched tree that holds such an entry
(go-kure/kure#1226).

A build can also make a Kustomization the tree does not hold, for example a
renamed copy of an umbrella from a hand-written Kustomization with a
`namePrefix`. Such a copy may lose the `wait` and `healthChecks` the rewrite
clears on the umbrella only when its whole spec before the rewrite equals the
spec of an umbrella of the tree, either as the build shows that umbrella or as
the tree holds it (a patch on an ancestor changes the first, not a copy made
from the umbrella's directory). If a patch of the copy leaves a spec equal to
neither of those, it is no copy but a Kustomization with a spec of its own, and
the loss is refused as a different `spec.wait` or `spec.healthChecks`; a patch
that leaves it equal to one of them leaves it a copy. When the tree cannot be
built before the rewrite, a Kustomization the tree holds that the integration
neither generates nor keeps (one an application emits, say) is trusted as the object
the controller applies only if every writer of its applied copy is the Flux
bootstrap or a Kustomization with no patches, components, name prefix or
suffix, target namespace or `postBuild`. Any other writer could have changed
what the directory holds, so the tree is refused with a message that it does
not hold the Kustomization the build makes. A Kustomization the integration
generates or keeps is not held to this rule (go-kure/kure#1247).

The integrated placements apply all three rules. `GenerateForBundle` applies
the first two to the one Kustomization it returns, and `GenerateFromLayout`,
`GenerateFromCluster` and `FluxSeparate` refuse the setting.
These checks read the `spec.path` of a Kustomization the integration did not
place as a directory of the tree only when it reads the Source of the
Kustomization that applies it, on this cluster. A `spec.path` naming an
`AppFileSingle` layout below the top of the tree is no directory at all: that
layout is one file in its `Namespace`, so such a Kustomization builds nothing
of the tree. A health check on another cluster (`spec.kubeConfig`) waits for no
readiness here, and a `spec.healthCheckExprs` predicate on Flux Kustomizations
is read as a readiness wait, whatever it says.
A `wait` does not wait for a Flux Kustomization its apply skips
(`kustomize.toolkit.fluxcd.io/ssa: Ignore` or
`kustomize.toolkit.fluxcd.io/reconcile: disabled`, on the object or in the
waiting Kustomization's `spec.commonMetadata`, as its build makes the object),
and a health check on the checker itself is dropped, as Flux drops it,
unless it is written with no namespace, which never passes and is refused (see
the [Flux Engine reference](/api-reference/flux-engine/)).
With or without the setting, every integration also judges the order of the
Kustomizations of the tree, on each later integration too, each read as
kustomize-controller builds its writer: patches, components, name prefix and
suffix, target namespace and postBuild substitution applied. A Kustomization
whose postBuild substitutes is built when its directory generates a ConfigMap, so
the generated name is read as built and substituted. A tree that was admitted
with such a Kustomization whose generator kure cannot build (a remote file
source, a kustomize failure) is now refused. A built object is
read as its JSON, so a number in it, a port say, is read like any other field,
before and after substitution. An unstructured object you build in Go should hold
JSON-typed values, as apimachinery's `Unstructured` requires: an `int64`, not an
`int`. kure reads such an object as its JSON and does not panic on one
(go-kure/kure#1252). The integration entry points replace a plain Go integer, `[]string`, `map[string]string`
or other accepted shape in an `Object` map with a JSON-typed copy (the caller's
map is not changed), also in an unstructured object or List that a typed object that is no List holds in a
field, the `Object` of a `RawExtension` included (such a typed object as an item of a typed List too),
and refuse a value they cannot convert, such as a struct,
a pointer, a channel or a type that marshals itself, naming the layout, the
object and the field. A typed object whose field has the `omitzero` tag and a
type of your own with an `IsZero` method is refused as well, since
`encoding/json` would call that method. An embedded struct with no name in its
tag is not refused for this, since `encoding/json` flattens it and calls no
`IsZero` there. A nil pointer, and an interface that holds one, are not judged,
since `encoding/json` calls no `IsZero` on them. A field tagged `json:"-"` is
judged like any other, with what it holds: `encoding/json` does not write it,
but a marshaler or kure's own code can. The check does not
copy every rule of `encoding/json` for picking fields. A field that shares its
JSON name with a shallower field, and an embedded struct under an invalid tag
name, are still refused, which is the safe side. A type of your own that embeds an unstructured object
(by pointer or by value, also as an item of a typed List) is not replaced by a
copy: it is refused, naming its Go type, when its map holds such a value, so
hold the object as a plain `*unstructured.Unstructured` to have it converted.
This holds for every integration entry point: all a caller passes is normalized
at entry, also a value that entry point never reads, so an object holding a byte
slice of more than 4 million bytes is refused even where no path reads the slice
(go-kure/kure#1201). The objects of one call that need conversion or a check
share one budget of 4 million values (the bytes of a byte slice count as
values each time the slice is converted or checked, and an object of a typed
List is checked before the List is written). An unstructured object that holds
JSON-typed values only is kept as it is and is not charged to it. A typed
object is walked in steps (a pointer, a field, an element, a map value, a map
key of interface type and an embedded struct are one step each, and an
interface is none), up to 40,000, and
its walk meets at most 4 million values (an interface is one of them). A field
tagged `json:"-"` is walked like any other. A value past a bound is refused even where
origin/main would accept it: the bounds are part of the contract, and
the package README lists them.
A type that embeds two such objects at the same depth, or that has a
`DeepCopyObject` of its own (or one in a type it embeds, an embedded interface
or a named type that is not a struct included, at the object's depth or
shallower), is refused too.
A typed nil resource is refused naming the Go type. So is a typed object whose
`GetObjectKind`, `GetName`, `GetNamespace`, `GetLabels` or `GetAnnotations`
panics, or one that a typed `List` holds: every entry that reads your objects
(`IntegrateWithLayout`, `CreateLayoutWithResources`, `UmbrellaTerminals`,
`GenerateFromLayout`, `GenerateFromCluster` and the layout writers) calls these
five once on each object before it reads any, and refuses a panic in an error
that names the Go type and never the panic's text. The call refuses before it
changes anything of yours, whether or not a later step would have called the
method that panics. It also reaches an object that only a bundle's emitted
record still holds, which a caller took out of the layout: the integrator and
`UmbrellaTerminals` refuse that object too. A method that answers or panics
differently on a later call, or never returns, is not covered. A setter that
panics part-way, when an application's delivery intent sets its annotations, may
leave the object changed, and then the undo cannot promise to restore it. The
package README lists the accepted shapes. A Flux Kustomization you hand-write
as an unstructured object is read the way the API server reads it: a key
matches a field only in its exact spelling, and a key the type does not have is
dropped. A `Wait:` or `KubeConfig:` key therefore sets nothing, as it sets
nothing for Flux. If the API server rejects such a key instead of pruning it,
the apply of that object fails there, and the integration does not report it.
A build that
fails, a Kustomization two builds write differently (`wait` or
`ignoreMissingComponents` set to `false` or `null`, `decryption` set to `null`,
or a list field such as
`dependsOn` or `patches` set to an empty list or `null`, is the field omitted,
as Flux applies it; any other difference counts, and a key in another case,
`Wait:` say, is no field and is ignored), the eighth build of one `spec.path`
along a chain of builds that each write a copy of the next, since such a chain
may write Kustomizations without end (a chain that comes back to a copy already
built stops there), a placed one no apply
creates (a Kustomization the Flux bootstrap applies counts as an apply only on
the Source of a member whose build reads its directory), and a placed one a build applies on another cluster or from another
Source, are refused. So is a build kure cannot read as Flux does, naming the
file: one that reads a remote reference (a Git base or an `http(s)` URL; an
integration never fetches, so vendor the base into the tree or reference it
through a Flux Source), a kustomization file kure cannot parse, one that sets
`openapi`, or one that may merge into a Secret while the Kustomization sets
`spec.decryption` or the binary links another kustomize than Flux's (a merge
otherwise is read as built, under the name kustomize gives the merged Secret).
So is a build, with `spec.decryption` set, that reads a file holding a SOPS
marker (`"mac": "ENC[`, `sops_mac=ENC[`, `[sops]` or `mac: ENC[`) or a file
kure cannot read: Flux decrypts such a file, and the names and namespaces the
build gives are over what it decrypts to, so kure, which sees only the
ciphertext, cannot read them. The refusal names the file and covers the whole
tree, not only that Kustomization, since no object of an unread build is read.
It is wider than Flux: a marker in a file Flux would not decrypt (another
format, a `configMapGenerator` source, a comment, a fixture) refuses too. It
applies only to a Kustomization kure builds, one with `patches`, `components`,
`namePrefix`, `nameSuffix` or `targetNamespace`, or one with post-build
substitution over a directory whose layout generates a ConfigMap; to keep such
a tree, keep the encrypted files out of the Kustomizations kure builds.
So is a tree, whole, when kustomize's schema in this process is not its
built-in one: another use of kustomize set a custom schema, or added or
changed definitions of it, such as a CRD schema, which can change how a field
merges and so a verdict either way. kure checks the schema before the first
build of a tree and again after its last (about 17 ms each), and refuses every
build of the tree if either differs, naming the definitions. A process that
registers CRD schemas with kustomize beside an integration therefore gets no
reconcile-order verdict from it; run such a use apart from the integration.
The definitions are not the whole of what kustomize remembers from a schema
added: it also records, from the schema's paths, which kinds are cluster-scoped,
and decides on that when it applies a target namespace or rewrites name
references. A schema added with a path alone for a kind therefore leaves the
definitions as they were and still changes a build, so kure also asks, in each
build, for the scope of every kind the build made and of every kind in a file or
inline patch it read, an object an overlay deletes later included, and does not
read a build in which one differs from the built-in schema's. It does not read a
build either when it cannot follow an object to its kind: an object that
reaches a merge chain that leads back to itself; any mapping that reaches a
merge cycle whose members offer different values of an `apiVersion`, `kind` or
`items` that one of them lacks; a List nested more than eight
deep that holds an object (a mapping with a `metadata` key, of its own or
through merge keys); or an object with an `apiVersion` or `kind` of more than
1024 bytes. The package README states the full contract, including what is not
refused.
One
without `wait` that health-checks any other object waits for the Kustomization
that applies it, whatever its expression says, when that writer is certain,
and a cycle through them is refused. With several certain writers it waits for
whichever creates the object first: the tree is refused when every one of them
waits for the checker, but not for a deadlock that needs two such checks at
once ([health checks on objects](/api-reference/flux-engine/#one-kustomization-per-directory)).
A copy another apply excludes
(`kustomize.toolkit.fluxcd.io/ssa: Ignore` or `reconcile: disabled`) counts for
no writer, unless it, or that apply's `spec.commonMetadata`, is templated or
encrypted. So does a Kustomization no build holds when every directory holding
a copy of it is loaded by some writer's build and by no other Kustomization of
the tree: each such build deleted it, or wrote it under another name, read as
that one. An object a controller, or a
Kustomization outside the tree, also writes is outside what the check models,
and a tree Flux would converge can then be refused. A Kustomization an
application emits is followed as a placed one is, and a `dependsOn` with a
`readyExpr` waits for `Ready`, which under kustomize-controller's default
feature gates refuses a tree whose expression asks for less and whose
`dependsOn` closes a cycle ([reconcile order](/api-reference/flux-engine/#one-kustomization-per-directory)).
Flux's default since v1.7 is the `AdditiveCELDependencyCheck` gate off, so an
operator running the default controller sets
`ResourceGenerator.AdditiveCELDependencyCheckOff` to true; false keeps that
conservative read. With it, a `readyExpr` of exactly `"true"` waits only for
the dependency to exist; set on a cluster whose gate is on, the checks can
admit a tree that never becomes Ready
([details](/api-reference/flux-engine/#with-additiveceldependencycheckoff)).
Picking terminal leaves counts only a `dependsOn` entry with no `readyExpr` as
a wait for `Ready`, with the setting or without: Flux skips its own readiness
check for an entry with an expression. Where the tree holds a patch
(`spec.patches` or `spec.components` on any Kustomization), the pass reads each
`dependsOn` as the build of the tree shows it, patches applied, so an entry with
no `readyExpr` still proves a wait for `Ready` unless a patch adds an expression
to it, and a `"true"` entry on a patched tree stays an existence wait unless a
patch changes it. When that build cannot be made, the pass reads the tree as it
holds it and no entry with no `readyExpr` proves anything: the dependency stays
a terminal leaf, so a dependent is written onto more leaves, a kept node
Kustomization is refused for a dependency it names only that way, and a `"true"`
entry is read as a wait for `Ready`, which can refuse a tree Flux would
converge. Trees without a patch read as before.
`fluxcd.UmbrellaTerminals` returns each umbrella's terminal leaves, for a
dependency written from outside the tree, for trees as an integration returned
them: only a Kustomization the integration recorded, still on the recorded
path, Source and cluster, counts as an umbrella. For trees integrated with
`AdditiveCELDependencyCheckOff`, that generator's `UmbrellaTerminals` method
returns the same leaves. The full rules and refusals are in
the [Flux Engine reference](/api-reference/flux-engine/#with-umbrellasdonotwait).

## Umbrellas without child health checks

Under `FluxSeparate` the children's Kustomization objects are not in the
umbrella's build: each child is applied by its own Kustomization. The automatic
health checks then hold a kustomize-controller worker for the umbrella until
every child is Ready, each child needing a worker of its own, and
`UmbrellasDoNotWait` is refused there. Setting
`ResourceGenerator.OmitUmbrellaChildHealthChecks` writes no automatic entry for
the children, in every placement; the umbrella's own `HealthChecks` are written
as before:

<!-- doc-example:excerpt setting the field on a generator; the behaviour is pinned by pkg/stack/fluxcd/umbrella_child_checks_test.go -->
```go
gen := fluxcd.NewResourceGenerator()
gen.OmitUmbrellaChildHealthChecks = true
integrator := fluxcd.NewLayoutIntegrator(gen)
```

With `Wait` unset or false the umbrella then no longer waits for its children:
it is Ready once applied and its own `HealthChecks`, if any, pass, so a bundle
that depends on it no longer waits for its children. To keep that, list the
children's Kustomizations in the umbrella's `HealthChecks`, or depend on the
children directly. With `Wait` true the field changes nothing in any
placement, since no entry was written before either; under an integrated
placement the umbrella's wait still covers its children, under `FluxSeparate`
it never did. Off, every tree renders as before. Details are in the
[Flux Engine reference](/api-reference/flux-engine/#with-omitumbrellachildhealthchecks).

## Disk layout

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
