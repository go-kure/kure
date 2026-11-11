# Names and Paths

## Kustomization names

A bundle's Kustomization is named after the bundle, unless the bundle sets `KustomizationName`:

<!-- doc-example: pkg/stack/fluxcd ExampleResourceGenerator_GenerateForBundle -->
```go
// The bundles keep their names and directories; their Kustomizations
// get the names set here.
db := &stack.Bundle{Name: "db", KustomizationName: "apps-db"}
shop := &stack.Bundle{Name: "shop", KustomizationName: "apps-shop", DependsOn: []*stack.Bundle{db}}

objects, err := fluxcd.NewResourceGenerator().GenerateForBundle(shop, "clusters/prod/shop")
if err != nil {
    panic(err)
}
kust := objects[0].(*kustv1.Kustomization)
fmt.Println(kust.Name, kust.Spec.Path, kust.Spec.DependsOn[0].Name)
```
<!-- doc-example:end -->

`Bundle.Name` stays the bundle's identity, and the field does not move the bundle's directory
(named by `Bundle.DirName`, or by `Name` without one; see [Directory names](#directory-names)),
so `spec.path` is the same with and without the field. `Bundle.UnitName()` returns the name in effect, and everything that refers to
the bundle's Kustomization uses it:

| Reference | Written as |
|---|---|
| `metadata.name` of the bundle's Kustomization | the bundle's `KustomizationName`, or its `Name` |
| `spec.dependsOn` of a bundle that lists it in `DependsOn` | the same name |
| an umbrella's health check on it as a child | the same name |
| a `NamedDependsOn` entry | it names a Kustomization, not a bundle: a bundle that sets `KustomizationName` is reached by that value. Its `Name` then reaches whichever bundle's Kustomization has that name, and is kept as written when none has, like any name no generated Kustomization has |

Two bundles whose Kustomizations would get one name are refused, and the error names both bundles
by their paths. `Bundle.Name` stays unique too. A bundle may hold an object that is itself a Flux
Kustomization named like the bundle (a component that delivers its own content): give the bundle
another `KustomizationName` and the generated Kustomization no longer collides with it.

Without it the tree is refused under every placement: the generated Kustomization and the one in
the tree would be two objects of one name in one namespace. The error names the one in the tree by
its layout and `spec.path`, and by the application and bundle that hold it; it names what the
generated one is for (the bundle, or under `FluxIntegratedPerLayout` the node or layout that gets
a Kustomization of its own); and it names the way out: the field that names the generated one
(`Bundle.KustomizationName`, `Node.KustomizationName` or `ManifestLayout.KustomizationName`), or
another name for the one in the tree. For a node whose walked layout carries a name the node does
not set, that is the layout and its field: the layout's name is the one in effect. A Kustomization
that no application holds, such as one added to a walked layout by hand or put where an
application's was, is named by its layout alone.

`Bundle.Validate` compares the names in effect wherever it compares against a Kustomization
reference. A bundle that lists the bundle `db` in `DependsOn` and the name `db` in
`NamedDependsOn` is valid when `db` sets `KustomizationName: "db-cr"`: the two are different
Kustomizations. The entry `db-cr` is then the duplicate.

A `DependsOn` entry that is a copy of a bundle of the cluster (another `Bundle` value with its
`Name`) is that bundle: `spec.dependsOn` gets the bundle's Kustomization name whether the copy
carries that name as its `KustomizationName` or none. Every entry point that takes a cluster or a
walked layout refuses a copy that sets another `KustomizationName`, naming both, and a copy of a
bundle whose Kustomization name is also in `NamedDependsOn`. `GenerateForBundle` has no cluster to
resolve a copy against and writes the name the entry carries. One valid input is refused: a name-only
copy of a bundle that sets `KustomizationName`, beside a `NamedDependsOn` entry equal to the
bundle's `Name`; set the bundle's `KustomizationName` on the copy (see the
[stack](/api-reference/stack/)).

## Kustomization paths

The layout tree is the only authority on directories. Every walked layout records which stack
nodes, bundles and application it renders (`ManifestLayout.OriginNodes`, `OriginBundles`,
`OriginApplication`), and `layout.IndexOrigins(root, cluster)` resolves each bundle to the one
layout whose directory holds its resources. A bundle's Kustomization `spec.path` is that layout's
`FullRepoPath()` — `OriginIndex.KustomizationPath(b)` — and nothing else:

| Tree | Rules | Bundle | `spec.path` |
|---|---|---|---|
| node `platform`, bundle `web` | `GroupByName` | `web` | `platform/web` |
| node `platform`, bundle `platform` | `GroupByName` | `platform` | `platform/platform` |
| root node `platform` (bundle `platform`) with child node `apps` (bundle `apps-bundle`) | bundles and applications `GroupFlat`, `ClusterName: "prod"` | `platform`, `apps-bundle` | `prod/platform/platform`, `prod/platform/apps` |
| unnamed root node, bundle `web` | bundles and applications `GroupFlat`, `ClusterName: "."` | `web` | `web` |

The path is emitted as `FullRepoPath()` returns it (no `./` prefix; Flux treats `x` and `./x`
alike) and is relative to the root of what the writer wrote: the `WriteToDisk` / `WriteToTar`
base, or `<basePath>/<ManifestsDir>` for `layout.WriteManifest`. Root the Flux source there.

The root node's directory renders no bundle (go-kure/kure#979): with a flat `BundleGrouping` its
bundle has a directory inside it, named after the bundle (by its `DirName` when it sets one, see
[Directory names](#directory-names)), as the last two rows show. Before, the
bundle rendered in the root node's directory (`prod/platform` and `.` in those rows), and the
Kustomization that applied that directory was hosted inside it. See "The root node's bundles" in
the layout package README, also for the path move this is on a deployed tree.

### Directory names

A bundle's directory is named after the bundle. `Bundle.DirName` names it otherwise, and
leaves the Kustomization's name and every reference to it alone; `spec.path` follows the
directory, in every placement:

| Umbrella `shop` at `platform/apps`, child | Directory | Kustomization | `spec.path` |
|---|---|---|---|
| `Name: "shop-infra"` | `platform/apps/shop-infra` | `shop-infra` | `platform/apps/shop-infra` |
| `Name: "shop-infra"`, `DirName: "00-infra"` | `platform/apps/00-infra` | `shop-infra` | `platform/apps/00-infra` |
| `Name: "shop-infra"`, `DirName: "00-infra"`, `KustomizationName: "apps-shop-infra"` | `platform/apps/00-infra` | `apps-shop-infra` | `platform/apps/00-infra` |

The field applies wherever a bundle's name becomes a directory: for an umbrella child, for a
node's bundle under `BundleGrouping: GroupByName`, and for the root node's bundle under
`GroupFlat`, which has a directory inside the root node's (root node `platform` with bundle
`platform-bundle` and `DirName: "00-platform"` gives the Kustomization `platform-bundle` with
`spec.path: platform/00-platform`). Under `GroupFlat` below the root a node's bundle is rendered
in its node's directory, which the node names, and its `DirName` has no effect. Bundles a flat
`NodeGrouping` merges into the root node share one directory and one Kustomization (see below):
the directory takes the first merged bundle's `DirName`, or its `Name`, and a `DirName` on a
later one has no effect.

A `DirName` must be one path segment (`stack.ValidateDirectoryName`); every entry point that
takes a cluster refuses any other value before it walks. It is no object name and may hold upper
case.

Two bundles whose own directories would be one, two umbrella children with one `DirName` for
example (or with two that differ in case only, which the writers treat as one directory), are
refused before any Kustomization is generated, and the error names both bundles by their paths. The checks on the root node's bundle directory compare the directory it gets, so
they follow a `DirName`: a child node of the root with that name is refused, and so is a
`DirName` of `flux-system` under `FluxSeparate`. The directory is set on the bundle, not on the
layout: a bundle's own layout renamed between `WalkCluster` and `IntegrateWithLayout` is refused,
and the error points at `DirName`. Such a rename used to be accepted, and gave the directory
another name without renaming the bundle or its Kustomization; a caller that relied on it sets
`DirName` instead.

### One Kustomization per directory

A directory is what a Flux Kustomization applies, so kure emits exactly one per directory that
renders bundles. When a `GroupFlat` axis merges several bundles into one directory, they share that
Kustomization, named after the first of them (the absorbing node's own bundle when it has one), by
its `KustomizationName` when it sets one. A reference to any of the merged bundles' Kustomization
names resolves to the shared one.

Each such directory has **one owner**: it is applied by its own Kustomization and by nothing else.
No parent `kustomization.yaml` lists a child directory that renders bundles, in any placement, so
two Kustomizations never apply (and prune, and patch) the same objects. The child's CR is what
applies it — in `flux-system` under `FluxSeparate`, in the parent directory under the integrated
placements. Building a parent directory therefore does not include its child units.

A Kustomization is never hosted inside the directory it applies. Every directory that renders
bundles has a parent to host its CR: the top of the tree `layout.WalkCluster` returns renders none
(go-kure/kure#979). `IntegrateWithLayout` refuses a tree whose top does render a bundle, in every
placement: integrate the whole tree `layout.WalkCluster` returns. A subtree of a walked tree,
integrated on its own, can be such a tree, and so can the `layout.WalkClusterByPackage` tree of a
package the root node is not in, under a flat `NodeGrouping`.

The bundles merged into one directory combine as follows:

| Bundle setting | In the shared Kustomization |
|---|---|
| `SourceRef`, `Interval`, `Timeout`, `RetryInterval`, `Prune`, `Wait`, `Force`, `Suspend`, `PostBuild`, `ServiceAccountName` | must be the same for every merged bundle (unset compares as the default; an omitted `SourceRef` namespace is the generator's `DefaultNamespace`; without a `URL` only its kind, name and namespace are compared, the fields a Kustomization carries), else an error naming the setting and the bundles |
| `HealthChecks`, umbrella health checks | combined, each listed once; a check on a Flux Kustomization names the unit that applies that bundle, and one on the unit itself is dropped; under `UmbrellasDoNotWait` a unit that applies a Flux Kustomization has none (see [With `UmbrellasDoNotWait`](/api-reference/flux-umbrella-bundles/#with-umbrellasdonotwait)); under `OmitUmbrellaChildHealthChecks` a unit has no automatic entries for umbrella children, only the checks its bundles list (see [With `OmitUmbrellaChildHealthChecks`](/api-reference/flux-umbrella-bundles/#with-omitumbrellachildhealthchecks)) |
| `Labels`, `Annotations` | combined; one key with two values is an error |
| `Patches` | combined, but only when every patch has a `Target` that selects none of the other merged bundles' objects (see [Patches in a shared directory](#patches-in-a-shared-directory)) |
| `DependsOn` | mapped to the Kustomization that applies each dependency; dependencies between the merged bundles are dropped |
| `NamedDependsOn` | combined and mapped like `DependsOn`: a name that is a rendered bundle's Kustomization name (see [Kustomization names](#kustomization-names)) becomes the name of the Kustomization that applies it (dropped when that is this one); any other name is kept as given |

`GenerateFromLayout` and the integrator refuse a set of Kustomizations kure generates that can
never all become Ready. Applying waits for every `dependsOn` to be Ready; becoming Ready waits for
every Kustomization it health-checks, unless `wait` is set (Flux then ignores health checks and
waits for everything it applied, the CRs it created included); and, under integrated placement, a
CR exists only once the Kustomization whose directory references reach its file has applied — a CR
the root directory reaches is created by the Flux bootstrap. Identities are namespace and name. A
health check on another cluster (`spec.kubeConfig`) is no wait. A `spec.healthCheckExprs` entry
for Flux Kustomizations is read as a wait for readiness, whatever its expression says. A
cycle among these waits is refused, naming the chain. Kustomizations an application emits itself,
and objects in other namespaces, are outside this check; the integrator's check below reads
health checks on objects. A merge can close one (a health
check or dependency on a bundle merged into a unit that waits for it), and so can a parent node's
bundle depending on a child node's bundle whose CR only the parent's directory holds (both
integrated placements). The root node's bundle cannot close that one any more: the CRs of the
root's child nodes are hosted in the root node's directory, which the bundle's Kustomization does
not build, and are created by whatever applies that directory (go-kure/kure#979). The bundle's own
directory still hosts the CRs of its umbrella children and, under `FluxIntegratedPerLayout`, of
the layouts inside it, as every bundle's directory does. A per-layout Kustomization with `wait`
(its layout's, or the one it inherits: see [Per-layout settings](/api-reference/flux-child-layout-crs/#per-layout-settings)) waits for
the CRs its directory hosts like any other, so a layout that depends on the application layout
above it closes a cycle once that one has `wait`. Under `UmbrellasDoNotWait` no Kustomization
that applies another one waits, and a `dependsOn` on it names its terminal leaves, so these
cycles through a wait are gone; the check runs on the rewritten specs. ArgoCD
Applications get the unit rule but not this check: only a `DependsOn` cycle between units is
refused there. Give bundles directories of their own (`NodeGrouping` or
`BundleGrouping` `GroupByName`) when they need different settings.

Every integrator entry point, in every placement, then judges the order of the Kustomizations the
integration placed or kept again (go-kure/kure#1133), each `dependsOn` as built (a kept
Kustomization's `dependsOn` that closes a cycle on a later integration is refused too), each
`wait` and each health check. A health check on an object that is no Flux Kustomization, on a
Kustomization without `wait` or `spec.kubeConfig`, waits for that object to exist: Flux fetches a
checked object before it evaluates any `spec.healthCheckExprs` entry, whatever the expression
says. As Flux does, a check on a Kustomization of any group that carries the checker's own
namespace and name is dropped, a check on a Flux Kustomization included; there the checker's
namespace is the one it has on the cluster, `default` for a checker written without one. Flux does not default a
check's namespace, so a check on itself written with no namespace is not dropped: its poller
rejects it, the checker never becomes Ready, and the integration refuses it, saying so.

Every Kustomization and object is read as its writer's build makes it (go-kure/kure#1094). A
writer is a Kustomization whose build is the tree's: the Flux bootstrap, each one the integration
placed or kept, and each other one written into the tree on this cluster (no `spec.kubeConfig`),
whose `spec.path` is a directory of the tree and whose Source is that of the Kustomization that
writes it, or, for one the Flux bootstrap writes, the Source of a member whose build reads its
directory. kure reads that last kind only where the tree would otherwise hold a member no apply
creates, in a second view: every check runs over the first view and then the second, and a
refusal of either refuses the tree, so the only verdict it changes is that one
(go-kure/kure#1174). The tree as held (`UmbrellasDoNotWait`, `UmbrellaTerminals`) still reads a
Kustomization the bootstrap writes as building nothing. kure builds each writer as kustomize-controller does: its `spec.patches`,
`spec.components`, `spec.namePrefix`, `spec.nameSuffix` and `spec.targetNamespace` applied, its
local config dropped, its generators run, then its `spec.postBuild` substitution, and a writer is
built from its spec as its own writer's result holds it. The build uses Flux's kustomize options
but not Flux's reset of kustomize's global OpenAPI schema around each build: that reset writes a
schema every other use of kustomize in the process reads, so two integrations, or one beside any
other kustomize build, would race. kure builds only with kustomize's built-in schema, which it
has kustomize parse before any of its uses of kustomize reads it, again after a reset. That is a
contract with the rest of the process, since kustomize's schema is process-global: another user of
kustomize must not set, reset or add definitions to it while an integration builds. kure reads the
schema's version, so it does not read a build under a custom schema or another built-in version,
naming the version. Definitions added to the built-in schema (`openapi.AddSchema`, `openapi.AddDefinitions`) <!-- doc-api-refs:ignore kyaml openapi functions, not kure API -->
leave the version unchanged but can change how a field merges, so a verdict
could go either way (a definition that merges a Kustomization's `spec.dependsOn` by name keeps a
dependency a patch replaced, so kure would report a cycle Flux's build does not have). kure
therefore compares a SHA-256 of a walk over every field of the schema's definitions, exported or
not, with a pinned one for the built-in schema, before the first build of a tree and again after
its last, and does not read any build of a tree when either differs, naming the definitions. Any
difference counts, a CRD schema another use of kustomize registered included, and so does a value
moved to a field kyaml does not read, such as an extension moved to `ExtraProps`, which marshals
to the same JSON. A change made and reverted between the two checks is not seen. One check costs
about 17 ms, and about 110 ms under `-race`. A tree that a view reads pays it twice, and a tree built only to
name a patch in a refusal pays it once. The check covers
the definitions kyaml exposes, and the schema kyaml's per-resource-type index serves for every kind
they declare: the index holds a copy of each definition, apart from the one in the definitions, so
a write through the schema kyaml returns for a kind changes what a build reads and leaves the
definitions alone. Every kyaml path that registers a definition leaves an entry there. It does not
see code that registers a definition and then deletes its entry from
`openapi.Schema().Definitions` by hand: the index still serves the registered schema, and no
definition declares its kind any more. That needs deliberate concealment by code in the
same process, which no in-process check can stop, so it is out of scope. The definitions are not the
whole of what kyaml keeps from a schema added: it also records, from the schema's paths, which kinds
are cluster-scoped, and kustomize's namespace filter and name references decide on that. A schema
added with a path alone for a kind leaves the definitions as they were and still changes a build, so
each build also asks kyaml, through its public `IsNamespaceScoped`, for the scope of every kind it
made, and of every kind in a file or inline patch it read, an object an overlay deletes later
included (kustomize reads a kind's scope as it loads the object), and is not read when one differs
from the built-in schema's. A kind only a generator made and an overlay deleted is not asked. Kinds are read from the YAML nodes of each file, with aliases followed one hop and merge keys
followed through their whole chain, each shared node read once, so the work grows with the size of the file and not with the paths through its aliases or the mappings that share a merge key's sequence; an object that reaches a merge chain that leads back to itself, or a List nested more than eight deep that holds an object (a mapping with a `metadata` key, of its own or through merge keys at any depth, at any depth below the eighth List), or an object with an apiVersion or kind of more than 1024 bytes,
makes the build not read (a mapping with no `metadata` key, in no List that has one and holding none below the eighth, is no object and is never refused for it; deep Lists of such data, a merge chain that leads back to itself in such data, and an apiVersion or kind over 1024 bytes on it, are not read or are read without a refusal). In a merge cycle each mapping keeps the `apiVersion`, `kind` and `items` it sets itself and takes of those it lacks the one value the other members offer, whichever member the file reaches it by; when they offer different values, any mapping that reaches the cycle makes the build not read, data included, since what it holds would depend on the order of the file. It orders only its own goroutines:
another user's concurrent set or reset of the schema races with a build (go-kure/kure#1166).

**An integration never fetches; a remote reference is reported as not read.**
kustomize-controller fetches what a build names remotely: a Git repository in `resources`,
`bases`, `components` or a `generators`, `transformers` or `validators` entry, which it clones,
and an `http(s)` URL in any of those or in a field kustomize loads a file through (`patches`,
`replacements`, `crds`, `configurations`, generator `files` and `envs`, and the same fields of a
builtin plugin config), which it requests. kure fetches nothing. A build that reads a file naming
a remote reference is not read, naming the file and the reference, and kure refuses the file
before kustomize reads its fields, so nothing is cloned or requested. The check fails closed: a
build is not read either when it reads a file kure cannot check, a kustomization or plugin config
file that does not parse, a plugin config that is not builtin (Flux builds with builtins only and
refuses it too) or of a builtin kure does not classify, or a `generators`, `transformers` or
`validators` entry that is a directory, whose plugin configs a kustomization builds. A local path
shaped like a repository, such as `github.com/org/repo`, is read as one. A data file named like a
kustomization file is checked as one. The property rests on one dependency: kustomize reads files
through no field kure leaves unclassified, which kure's tests pin to every field of kustomize's
`Kustomization`, every builtin plugin, and the kustomize version kure builds with. Vendor a remote
base into the tree, or reference it through a Flux Source.

**kustomize's schema is the process's, not kure's.** kure guarantees that its own goroutines,
concurrent integrations included, do not race each other on it. It refuses a build under a custom
schema or another built-in version, naming it, and every build of a tree whose schema holds
definitions other than the built-in ones, such as those kyaml's `AddSchema` and `AddDefinitions` <!-- doc-api-refs:ignore kyaml openapi functions, not kure API -->
add, checked before the tree's first build and after its last (about 17 ms each). A process that
registers CRD schemas with kustomize beside an integration therefore gets no reconcile-order
verdict from it: kure cannot tell such a definition from one that changes how a field merges. It
cannot order another use of kustomize in the same process: one that sets or resets the schema
while an integration runs, `fluxcd/pkg/kustomize`'s `Build` among them, races with it, and one that
makes kustomize parse the schema for the first time while an integration starts may too. Run such
a use apart from an integration. The pinned fingerprint is for the kyaml this module builds with and
for the layout of the types the walk reads (kyaml's schema type is `k8s.io/kube-openapi`'s
`spec.Schema`, read field by field): a bump of either that changes the definitions or those types
fails `TestDefinitions_BuiltInMatchesThePin`, which prints the value to set in
`internal/kustomizeschema`.

A copy whose apply skips it,
`kustomize.toolkit.fluxcd.io/ssa: Ignore` or `kustomize.toolkit.fluxcd.io/reconcile: disabled` on
it or in its writer's `spec.commonMetadata`, overwrites nothing and creates nothing: the
Kustomization is read from the first copy an apply does not skip, and is created by a writer that
applies it. A member every apply skips is still read, from its first copy. Eight cases are refused,
each naming the Kustomization: a build that fails, as the controller's apply would; a build kure
does not read, which Flux may build and apply, when it reads a kustomization file setting
`openapi`, naming the file, since kustomize would keep its schema for every later build and kure
does not reset it, when it reads a file naming a remote reference, or one kure cannot check for
one, a kustomization file kustomize rejects among them, naming the file, since an integration
never fetches, when another use of kustomize in the process has set a schema other than the
built-in one, naming it, or when it loads, as a kustomization (a component's included), a file
holding a `secretGenerator` entry with `behavior: merge` or a `generators` entry and the
Kustomization sets `spec.decryption` or the binary links another kustomize than the audited one,
naming the file (see the merging build below), or when the Kustomization sets `spec.decryption` and
it reads a file holding a SOPS marker, or one it cannot read, naming the file (see the encrypted
sources below); a
Kustomization two writers apply with a different `dependsOn`, `wait`, `healthChecks`,
`healthCheckExprs`, `path`, `sourceRef`, `kubeConfig`, `postBuild`, `commonMetadata`, `decryption`, `images`
(which Flux's generator writes into the build) or transform (`ignoreMissingComponents` included,
which decides whether a missing component fails the build), which both keep overwriting (a copy
that sets `wait` or `ignoreMissingComponents` to `false` or `null`, `decryption` to `null`, or `dependsOn`, `healthChecks`,
`healthCheckExprs`, `patches`, `components` or `images` to an empty list or `null`, agrees with one
that omits the field, which Flux applies alike; every other compared field and value compares as
written (a field a later Flux adds is compared only once kure lists it, and as written until its
zero value is listed too), so `namePrefix: ""` (the CRD requires a non-empty value), a value of
another type (`wait: ""`), an empty `postBuild` or a null entry in a map still differs, which can
refuse a tree and never admit one; a key in another case (`Wait:`) is no field, as the API server
and kure's own read treat it, so it compares as nothing); a member
built from what no apply creates, a skipped copy, the first its writers' results showed, or its
spec as the tree holds it, where no result held it yet, when the first copy that is applied
differs in those fields, since what it applies cannot then be read; a member built from its spec as the tree holds it that no apply creates
once every writer is known (a parent's patch deleted or renamed it), since nothing applies what
it builds, naming each Kustomization the bootstrap applies that may create it but whose build kure
does not read; a member a build applies with `spec.kubeConfig`, or with another Source than its
spec in the tree names, since it then applies on another cluster or another Source's directory;
a Kustomization that is the eighth along its own chain of writers (each built from a copy the one
before wrote) to build one `spec.path`, since a Kustomization that builds the directory holding it
can write, after a substitution, a copy of itself under a new name, and so on without end, so the
writers cannot all be found (a chain that comes back to a copy already built stops there and is
admitted; the bound cannot tell a chain that would converge only after eight builds from one that
never does, and refuses both);
and, with `spec.postBuild.substituteFrom`, a
variable that a Flux Kustomization of the build, or any object's `apiVersion` or kind, references
and `spec.postBuild.substitute` does not set, since its value is read from the cluster; with it
too, any variable in a key at an object's top level, which Flux may make its `apiVersion`, kind or
metadata, and an inline value with a line break YAML reads but Flux keeps (a carriage return,
U+0085, U+2028 or U+2029) that an object references, which may move a value the cluster sets
anywhere in it; and with it, an `apiVersion` or kind referencing a variable whose text, as Flux
substitutes it with `spec.postBuild.substitute`'s values (defaults, replacements and the text
around the variables included), is more than a name (letters, digits, `.`, `/`, `_` and `-`, bare
or inside one pair of quotes), on an object that references a variable it does not set, since such
a text (an alias, an open quote) may move a value the cluster sets into it. An object is a Flux
Kustomization by the `apiVersion` and kind the inline values make, each value with every newline
removed, as Flux removes it before substituting. Every object is read as the tree's writers write
it to its file: they drop server fields, the `kubectl.kubernetes.io/last-applied-configuration`
annotation, a null creation timestamp and an empty status from each resource's top-level object
(a List's items are written as they are), so none of these reaches Flux, and none is read. A
value read from the cluster is assumed to be plain text that keeps the structure Flux substitutes it into: one
holding YAML syntax (a line break, a quote, a comment, `: ` in a key) can change any field at
apply, which no offline reading sees, as main's reading of the object as written does not either.
An object's namespace or name is unknown where a cluster value reaches it through an inline YAML
anchor and alias in another field, as where it references the variable itself: kure substitutes
the object again with the variables it references that `spec.postBuild.substitute` does not set
bound, each to a value of its own, twice, and reads the identity as unknown where either run differs from the
offline one (go-kure/kure#1176). A value that reaches it only through a form neither run triggers
(a substring past the length of both) is not seen, and the identity stays certain, which may
refuse a tree Flux converges and never admits a deadlock.
One input is read in a way that may refuse a tree Flux converges, never admit a deadlock: a
generator over SOPS-encrypted sources, hashed from the ciphertext Flux
decrypts first (go-kure/kure#1178). A writer that substitutes (a `spec.postBuild` with `substitute`,
`substituteFrom` or `substituteStrategy: Always`) over a directory that generates a ConfigMap is
built as kustomize-controller builds it, so the ConfigMap is read under its hashed name and
substituted as the rest are (go-kure/kure#1177). That build is refused as any other is: a tree
that was admitted with such a writer whose generator kure cannot build (a remote file source, a
kustomize failure) is now refused, naming the build.
A Kustomization is read as it is
applied: kustomize-controller drops its `status` and `metadata.creationTimestamp` first, so
neither keeps it from being read.

A build that merges into a Secret is read as built. A merge changes a Secret another file writes,
and kustomize hashes the merged Secret's name over its merged content inside the build, so the
build gives the name Flux computes: the unhashed name for a merge into a literal Secret or one
whose hash is disabled, the hash of the merged data for a generated one (the name before the merge
then has no writer). A build may merge when a kustomization file it loads as one (a component's
included) holds a `secretGenerator` entry with `behavior: merge`, or a `generators` entry, whose
generator may merge too. kure decodes each such file as kustomize does: keys in any case, YAML
anchors, aliases and merge keys expanded, and the last of duplicate keys taking effect; a build that
reads a file it cannot decode is not read, as above, so no merge is missed. A file no build reads,
or one a generator reads only as data, merges nothing.

Such a build is not read, naming the file and the reason, in two cases kure cannot build as Flux
does. The Kustomization sets `spec.decryption` (any provider): Flux decrypts sources before the
build and resources after it, so decrypted data may change the hash, and nothing follows which
data reaches a name. A copy of the Kustomization that differs from the one built only in
`decryption` is refused as any other difference between two applied copies is (a null `decryption`
is none). Or the binary links another kustomize api than the one kustomize-controller v1.10.1
builds with (v0.21.1): v0.21.2 normalises `stringData` before a merge, v0.21.1 does not, so the hash
can differ. kure reads the module the binary links as Go reports it, the replacement when there is
one, by path and version; a module that imports kure resolves v0.21.2 unless it repeats kure's
`replace` directives, and its merging builds stay unread. This refuses conservatively: a tree
whose build merges into a Secret in these cases is refused even where Flux converges.

**Encrypted sources.** With `spec.decryption` set (any provider), Flux decrypts, before it builds,
the files a `secretGenerator` entry reads and every `patches[].path` file, in the directories of
`resources` and `components`, whenever a file holds a SOPS marker; it decrypts built Secrets and
whole-resource SOPS documents after the build. The names and namespaces a build gives are over what
the files decrypt to: a generator's hash, a `replacements` entry carrying a decrypted value into a
name or namespace, a `bases` or legacy `env` consumer of a decrypted file, a patch that changes
generated data. kure reads only the ciphertext, so such a build is not read, naming the file and
the marker. kure tests every file the build reads for the four marker strings of Flux's formats
(`"mac": "ENC[`, `sops_mac=ENC[`, `[sops]`, `mac: ENC[`) whatever the file's name, and a file it
cannot read, other than an absent one, counts as marked. This refuses more than Flux decrypts: a
marker in any file the build reads, in another format, a `configMapGenerator` source Flux does not
decrypt, a comment or a fixture, a provider other than `sops`, or a file over 5 MiB, which Flux
refuses to decrypt, makes the build unread. A build that is not read refuses the whole tree, not only its Kustomization, as for any
other build kure cannot read: no object of it, of any kind, is read. It applies only to a
Kustomization kure builds, one with `patches`, `components`, `namePrefix`, `nameSuffix` or
`targetNamespace`, or one with post-build substitution (`postBuild` with a `substitute`, a
`substituteFrom` or `substituteStrategy: Always`) over a directory whose layout generates a
ConfigMap; any other is read as the tree holds it, which holds no `secretGenerator` and no patch
Flux decrypts. Without `spec.decryption` a marked file refuses nothing. The refusal never
admits a wrong name.

kure adds the existence wait only when the object's writers are certain: only writers other than
the Flux bootstrap apply an object matching it by group, kind, namespace and name as built (the
version ignored; an empty `apiVersion` is `apps/v1`, as Flux reads it), none of those applies
skips it, and no other apply may write it under an identity the tree does not show. A writer may
be a Kustomization an application emits, when its build is the tree's. With one writer the check
waits for its apply. With several it waits for whichever creates the object first, so the tree is
refused only when every writer's apply waits for the checking Kustomization's `Ready` through a
chain of these waits, and the checker, or a Kustomization on any such chain from a writer, is one
a cycle through it would be refused for: a member, or one a build writes under a name the tree
holds none by (below; the refusal names one chain per writer). A chain
does not pass through another health check with several writers, so a deadlock that needs two of
them at once is admitted, which errs towards admitting (go-kure/kure#1172). A new refusal through
a health check on such an object comes only from such a wait. A matching copy its apply excludes,
`kustomize.toolkit.fluxcd.io/ssa: Ignore` or `kustomize.toolkit.fluxcd.io/reconcile: disabled`
(any case) on it or in that apply's `spec.commonMetadata`, is never created, so it counts neither
as a match nor as another writer, as for a Flux Kustomization's copy above. That holds only when
neither the copy as written nor that `spec.commonMetadata` holds `$` or `ENC[` anywhere: a
substitution or a decryption may change the copy's marks, so a templated or encrypted copy is
counted as before. These leave the writers uncertain and add no wait, so a cycle through them
passes:

- a check with no namespace, or a matching object written with none that its apply does not
  exclude, a cluster-scoped one included (it may exist before any apply);
- a `configMapGenerator` entry of a directory read as the tree holds it (the bootstrap's, that of a
  writer that is not built, that is, one that transforms nothing and substitutes nothing over a
  directory that generates a ConfigMap, or that of a Kustomization whose build is not the tree's)
  whose name, or that name and a dash, begins the checked name, as kustomize appends a content
  hash; a built writer's generated names are read as built, substituted;
- an object whose identity may change at apply, whatever it names, unless only the results of the
  object's writers hold it: `$` left after substitution or `ENC[` (SOPS) in its `apiVersion`, kind,
  namespace or name, or a namespace or name a `substituteFrom` variable sets. Another apply may
  give it the checked identity. For a ConfigMap, so does such a `configMapGenerator` entry read as
  held;
- a matching object whose labels or annotations as built, with its writer's
  `spec.commonMetadata` set over them, carry `kustomize.toolkit.fluxcd.io/ssa: IfNotPresent` (any
  case), as the apply skips it only when it already exists, or an exclusion on a copy counted as
  above (written with `$` or `ENC[`). With `spec.postBuild.substituteFrom`, these labels and
  annotations are read as written, before substitution, as main reads them: a skip holds only where it is written literally and the
  substitution leaves it as written, never as a default or an inline value makes it
  (`${SKIP:=Ignore}` does not skip, with `SKIP` set inline or not, nor does a key
  `${KEY:=kustomize.toolkit.fluxcd.io/ssa}`). The cluster may set a variable, and an inline value
  may carry YAML that moves a value the cluster sets into a skip, so whether the apply skips the
  object is then not known. A key holding a `$` may become a skip key at apply and meet a written
  one, so no skip is read from labels or annotations holding one, even beside a skip written
  literally; a key of the object's metadata referencing a variable `spec.postBuild.substitute` does
  not set may become its labels or annotations at apply, so no skip is read from either beside one.
  No skip at all is read from an object referencing any variable `spec.postBuild.substitute` does
  not set: an expression the cluster controls may change the text around a skip at apply (a
  substring of an inline quote whose length the cluster sets swallows the skip line), while an
  object every variable of which is set inline applies as substituted offline, inline values
  overriding the cluster's. This errs towards refusing;
- a ServiceAccount `default` or a ConfigMap `kube-root-ca.crt`, which Kubernetes creates in every
  namespace; this list is closed;
- a match the Flux bootstrap applies that its apply does not exclude;
- a Kustomization on this cluster whose `spec.path` is a directory of the tree but whose build is
  not the tree's (another Source), read from its first copy an apply does not skip (one every
  apply skips applies nothing). One no result holds is read as the tree holds it, unless every
  directory holding a copy of it is loaded by some writer's build and by the build of no other
  Kustomization of the tree: each writer's build then deleted it, or wrote it under another name
  read as that one, so no apply creates it (go-kure/kure#1183). Otherwise it may apply a match it
  does not exclude, read as its directory
  holds it, or write
  the object under another identity: with `spec.components`; with `spec.patches`, when its
  directory holds any object or `configMapGenerator` entry (a patch may change a kind, or keep an
  object marked `config.kubernetes.io/local-config`); or with a name prefix or suffix or a target
  namespace, when it holds an object of the checked group and kind (for a ConfigMap, also a
  `configMapGenerator` entry);
- a Kustomization of the tree that cannot be read in typed form, even from its writer's
  substituted result.

The check assumes that an object the tree writes literally is created only by the applies of the
tree's own directories. An object that a controller also creates (a HelmRelease's chart, an
operator), or that a Kustomization outside the modelled tree also writes (another source, or a
path that is no directory of the tree), is outside the model: such a tree has two owners of one
object, and the check can then refuse a tree Flux would converge.

A `spec.healthCheckExprs` entry adds that existence wait only, never readiness. A Kustomization an
application emits, or the caller writes, is followed as a placed one is, with or without
`UmbrellasDoNotWait`, read from its writer's result: its `dependsOn`, `wait` and health checks are
judged, so a tree can be refused for a cycle through one. One that cannot be read in typed form
even there (a variable no `postBuild` sets) adds no wait: its writer's apply fails, a stall,
which the checks do not judge. A cycle made only of Kustomizations the integration did not place,
under the names the tree holds them by, is their author's, and is admitted. A Kustomization a
build writes under a name the tree holds no Kustomization by (a member renamed by a name prefix,
for one) may be a member, and the tree cannot tell whose it is: it is judged as the
integration's, and a cycle through it is refused. Such a one that every apply skips is never
created, and is not judged.

A `dependsOn` with a `readyExpr`, on any Kustomization, waits for the dependency's `Ready`. With
kustomize-controller's `AdditiveCELDependencyCheck` feature gate off, its default
(kustomize-controller v1.10.1 `internal/features/features.go:97-99`), Flux evaluates only the
expression and skips its own readiness check (`internal/controller/kustomization_controller.go:614-626`).
kure does not evaluate CEL and cannot see the gate, and an expression may itself require `Ready`,
so it reads the readiness wait, which never admits a deadlock. Under the default gate a tree whose
expression asks for less than `Ready`, and whose dependsOn closes a cycle, is therefore refused
although it converges. Drop that `dependsOn`, depend on the object some other way, or, for a
`readyExpr` of exactly `"true"`, set `AdditiveCELDependencyCheckOff` (below).

That read is the safe one for the checks that look for a cycle, where counting too many waits can
only refuse. It is the wrong one where a check needs proof that a Kustomization is `Ready` after
its dependency: picking the terminal leaves of an umbrella, and what a node's kept Kustomization
must carry. There only a `dependsOn` entry with no `readyExpr` counts, with
`AdditiveCELDependencyCheckOff` or without. Under the default gate Flux skips its own readiness
check for an entry with an expression, so it proves nothing about `Ready`; counting it would drop
a sibling from the leaves, and a dependent could proceed before that sibling is `Ready`. The
"umbrellas do not wait" integration still refuses an umbrella that has no terminal leaf when every
entry is read as a wait for `Ready`, as before, whether or not anything depends on it; the
standalone `UmbrellaTerminals` queries run no such check.

On a tree that patches, the standalone `UmbrellaTerminals` queries and the "umbrellas do not wait"
integration read each `dependsOn` entry as the build of the tree shows it, with every patch
applied (go-kure/kure#1226). The integration builds the tree twice, once before the rewrite and
once after it. Each entry has a class: 2 for an entry with no `readyExpr`, 1 for any other
`readyExpr`, 0 for `readyExpr: "true"` where `AdditiveCELDependencyCheckOff` is set (it waits for
existence only). The integration admits a patched tree when, for every Kustomization, the build
after the rewrite waits for at least what the build before it waited for, each umbrella replaced
by its terminal leaves. It refuses the tree when:

- a build gives any Kustomization a `dependsOn` entry that waits for `Ready` or an expression and
  whose target is an umbrella of the tree (a patch added it); the rewrite never sees such an
  entry;
- a build gives any Kustomization a `dependsOn` entry, that waits for `Ready` or an expression, on
  a Flux Kustomization it makes but the tree does not hold, and that one applies Flux
  Kustomizations (a renamed copy of an umbrella, say);
- a positional or copying patch lands differently on the rewritten list than on the list as
  written, and the build after the rewrite waits for less than the one before it (`remove
  /spec/dependsOn/1`, say), or for the same target with a weaker class;
- a build makes a Kustomization under a name the build before the rewrite does not;
- one of the fields the reconcile-order checks read (`path`, `sourceRef`, `kubeConfig`, `patches`,
  `healthChecks`, `wait` and the other graph fields, but not `dependsOn`) differs between the two
  builds, other than the `wait` and `healthChecks` the rewrite clears on an umbrella, and on a
  copy of one that a build makes (a Kustomization the tree does not hold, that applies Flux
  Kustomizations and, before the rewrite, has the whole spec of an umbrella of the tree, every
  field and `dependsOn` included, as the build shows the umbrella or as the tree holds it: a patch
  of an ancestor's build changes the first and not a copy made from the umbrella's directory); or
  an umbrella regains a `wait` or health check.

A tree whose build before the rewrite fails, or that holds a Flux Kustomization the build cannot
read (an unreadable copy, or a patching non-member on a member's Source that no build reads as a
writer, whether the tree holds it or a patch keeps it from local config), falls back to the earlier blanket: no entry proves anything and the checks refuse more.
The fallback asks the build after the rewrite to keep every entry each member held, and to keep
the terminal leaves written for an umbrella among them.

These refusals are deliberate. kure cannot tell the author's intent from a lost wait, so it refuses
these trees, though some of them are fine:

- a patch that removes a held entry by its index in the rewritten list, when the build before the
  rewrite cannot be made;
- a patch that removes an entry on a dependency that exists nowhere, since kure cannot prove a
  target absent;
- a held Kustomization that is no member, with a copy a build makes under its name, whose applied
  copy differs from it in a field the checks read, or is written by a Kustomization that patches,
  renames or substitutes (`spec.patches`, `spec.components`, `spec.namePrefix`, `spec.nameSuffix`,
  `spec.targetNamespace` or `spec.postBuild`), on the fallback path (it is not trusted as a
  stand-in for the copy: the copy may be one a build made from a member, with a wait a patch took
  from it, and no comparison of fields can tell the two apart);
- a renamed copy of an umbrella whose own patches change a field of its spec (its `wait`,
  `healthChecks` or `dependsOn`, say) so that the spec equals neither the umbrella's as the build
  shows it nor as the tree holds it: the specs are compared as the typed Kustomization encodes
  them, so a duration written `60s` or `1m`, and an empty or an absent list, compare equal, and
  only a value the typed form keeps can tell two specs apart. Such a copy is no copy of the
  umbrella, and the wait it loses is refused as a different `spec.wait` or `spec.healthChecks`.
  A Kustomization with the whole spec of an umbrella, as the build shows it or as the tree holds
  it, is a duplicate of it and loses what the umbrella loses. A copy that equals neither (its own
  patch changes a field the umbrella's patch did not, or sets one the umbrella's patch set to
  another value) is refused.

Not handled: two CEL expressions are never compared, so an entry of class 1 only has to stay of
class 1.

#### With `AdditiveCELDependencyCheckOff`

`ResourceGenerator.AdditiveCELDependencyCheckOff` states that every kustomize-controller
reconciling the tree runs with its `AdditiveCELDependencyCheck` feature gate off. That is Flux's
default since v1.7 (kustomize-controller v1.10.1 `internal/features/features.go:97-99`), so an
operator running the default controller sets it to true. kure cannot see the gate.

- **false**, the default, keeps the conservative read: a `dependsOn` with a `readyExpr` waits for
  the dependency's `Ready`, whatever the expression says.
- **true** changes one case: a `dependsOn` whose `readyExpr` is exactly `"true"` waits only for
  the dependency to exist. With the gate off Flux evaluates the expression instead of its own
  readiness check, and it always requires the dependency to exist first
  (`internal/controller/kustomization_controller.go:597-628`). The reconcile-order checks then
  wait for the apply of the Kustomization that creates the dependency, or for nothing when no
  Kustomization of the tree does (the Flux bootstrap, say); a chain through that wait is reported
  as `exist namespace/name`. Every other expression still waits for `Ready`.
- **The risk:** set on a cluster whose gate is on, where Flux checks `Ready` after the expression
  (`:630-655`), the checks can admit a tree that never becomes Ready.
- **Where it applies:** Kustomizations kure generates carry no `readyExpr`, so the field changes
  only how Kustomizations an application emits, or a caller keeps, patches or substitutes, are
  judged, in every check: the members' check, the tree's, and the "umbrellas do not wait" rewrite
  (an existence wait on an umbrella is kept as written). That rewrite reads each entry as the
  build of the tree shows it, patches applied (above): an existence-only entry on a patched tree is
  admitted when the built class is existence-only, and refused when a patch makes it anything else.
  Terminal leaves do not depend on the setting: an entry with a `readyExpr` never makes a sibling
  depended on.
  No entry point refuses it. A v1beta2 Kustomization's `dependsOn` has no `readyExpr` field, so
  it still waits for `Ready`.

<!-- doc-example:excerpt setting the field on a generator; the behaviour is pinned by pkg/stack/fluxcd/readyexpr_existence_test.go -->
```go
gen := fluxcd.NewResourceGenerator()
gen.AdditiveCELDependencyCheckOff = true
integrator := fluxcd.NewLayoutIntegrator(gen)
```

#### Patches in a shared directory

Flux applies a Kustomization's patches to **everything that Kustomization builds**, not to the
bundle that declared them. While a bundle has a directory of its own that is the same thing. Once
a `GroupFlat` axis puts several bundles in one directory, the shared
Kustomization builds all their objects, so a patch meant for one bundle would also change the
others' — and a layout setting would silently change what a bundle's patch does. kure refuses
that instead of generating it:

- an untargeted patch in a shared directory is refused;
- a targeted patch is refused when its `Target` selects any object another bundle in that
  directory renders, a ConfigMap an augmenter's `configMapGenerator` makes included, in the
  application's own layout or a child layout below it (matched with the annotations its entry
  sets, those of a [delivery intent](/api-reference/flux-layout-integration/#delivery-intent) among them). Matching is
  kustomize's own: `Group`, `Version`, `Kind`, `Name` and `Namespace` are anchored regular
  expressions (an empty one matches anything; `Namespace` is matched against the object's
  effective namespace, `default` for a namespaced object that names none), `LabelSelector` and
  `AnnotationSelector` are Kubernetes selector expressions. A written object's labels and
  annotations are matched as kustomize reads them from the written file, every value as its
  text: an object decoded from YAML may hold a boolean, a number or null there, which reads as
  `true`, `3`, `1.5` (a whole number as an integer, `2`) or `null`, and its other labels or
  annotations are still matched (go-kure/kure#1100). An annotation the writer leaves out of the
  file (`kubectl.kubernetes.io/last-applied-configuration`) is not matched. A generated
  ConfigMap is not written: its entry's annotations are matched as they are, that one included,
  as kustomize puts them on the ConfigMap. The same matching decides whether a patch with a
  `Target` selects a hosted Source. Only objects the shared Kustomization
  builds count: under `FluxIntegratedPerLayout` a per-app directory is applied by its own CR, so
  its objects are outside the patch's reach. A bundle `b1` with `Target: {Kind: ConfigMap}` merged with a
  bundle that renders a ConfigMap is refused; `Target: {Kind: ConfigMap, Name: one-cm}`, naming
  `b1`'s own ConfigMap, is accepted.

The error names both bundles, the target and the object it reaches. Narrow the target to the
bundle's own objects, or give the bundles directories of their own (`NodeGrouping` or
`BundleGrouping` `GroupByName`). Not covered: objects inside an `ExtraFiles` file (they are not
kustomize resources) and objects without a kind. Under `FluxIntegratedPerLayout` the check counts
no generated ConfigMap, and needs none: no walked tree has a shared directory that builds one. An
augmenter application that wants its own layout keeps a directory, which its own Kustomization applies without
patches, and `FlattenSingleTier` collapses no directory that renders a bundle (go-kure/kure#979).

The generator computes no path, the integrator matches nothing by name, and `FlattenSingleTier`
rewrites nothing afterwards: it collapses no directory that renders a bundle
(go-kure/kure#979), so it changes no Kustomization's path and merges no bundles. An umbrella
child's path is its own directory, in every mode (an earlier `KustomizationRecursive` rule
pointed it at the parent bundle's directory, whose kustomization excludes the child).

`IndexOrigins` refuses a tree it cannot resolve unambiguously: a hand-built, partial or
other-cluster tree (the rendered set is not what the cluster reaches), a bundle or node rendered
twice, two bundles with one name or with one Kustomization name, a bundle's own directory
under another name than the bundle's `DirName` (or its `Name` without one), two bundles with
one directory of their own, and a node or bundle
layout in `AppFileSingle` mode (written into its `Namespace`, not its own directory). Build the
tree with `layout.WalkCluster`. Not covered: a bundle whose `SourceRef.URL` names another
artifact — its path is relative to that artifact.
