# Layout Module

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/stack/layout.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/stack/layout)

The layout module is a sophisticated system for organizing and writing Kubernetes manifests to disk in directory structures that work with GitOps tools like Flux and ArgoCD.

## Core Purpose

The layout module transforms Kure's in-memory stack representation (Clusters → Nodes → Bundles → Applications) into organized directory structures with proper kustomization.yaml files that GitOps tools can consume.

## Key Components

### 1. ManifestLayout Structure
- Central data structure representing a directory with its resources and children
- Contains: Name, Namespace, Resources (K8s objects), Children (subdirectories)
- Supports package-aware layouts for multi-OCI/Git scenarios

#### Layout paths

A layout's directory is `FullRepoPath()`, which is `Namespace` joined with `Name`. `Namespace` is
always the **parent's** directory: set a child's `Namespace` to its parent's `FullRepoPath()`, and
use `"."` for the root of the tree. An empty `Namespace` means `cluster`. An `AppFileSingle` layout
writes one file, `<Namespace>/<Name>.yaml`, into its `Namespace`, normally its parent's directory,
and the parent's `kustomization.yaml` lists it. An `AppFileSingle` root writes that file and its `kustomization.yaml`
into its `Namespace`, and lists its children there: build them with the root's `Namespace`, not its
`FullRepoPath()`.

The parent lists an `AppFileSingle` child's file by its path relative to the parent's directory,
slash-separated (go-kure/kure#879): `svc.yaml` when the child's `Namespace` is the parent's
directory, `sub/svc.yaml` when it is `<parent directory>/sub`. Before, the entry was always
`<Name>.yaml`, which named no file for a child below the parent's directory, and named one file
twice for two such children with one name. A listed child whose file lands outside the parent's
directory (a sibling or an ancestor directory) is refused before anything is written: the entry
would start with `../`, and kustomize's default load restrictor rejects it. The parent's directory
must match exactly: a child `Namespace` of `P` or `P/sub` under a parent directory `p` is refused
too, since the two spellings name one directory only on a case-insensitive volume. Below the
parent's directory the child's own spelling is kept, so `p/Sub` under `p` is listed as
`Sub/svc.yaml`. A child the parent does not list (an umbrella child, one that renders bundles, one
with no resources), or any child of a parent that writes no `kustomization.yaml`, may land
anywhere. A `FluxIntegratedPerLayout` parent, or a parent of another package, lists an
`AppFileSingle` child's file all the same. An `AppFileSingle` child's `Name` must be one file name:
one that is rooted or holds a path separator (`/svc`, `sub/svc`, `../q/svc`) is refused, listed or
not. A rooted `Namespace` is joined onto the writer's base path, and `WriteToTar` resolves it
under the archive root, so a child `Namespace` of `/p/sub` under a parent directory `p` is listed
as `sub/svc.yaml` by all three writers.

When a parent's `kustomization.yaml` references a child by directory (`- <Name>`), the child's
directory must be `<parent directory>/<Name>` for the reference to resolve; building every child
with its parent's `FullRepoPath()` as `Namespace` gives exactly that. Layouts with the same name nest:
a bundle `web` with an application `web` gives `web/web`. A caller may still place a child
elsewhere on purpose, for example an OCI artifact whose root lists sibling layers that live under a
group directory; nothing refuses that, but such a root's own `kustomization.yaml` is not a valid
kustomize entry point.

Before go-kure/kure#771, `FullRepoPath()` dropped `Name` whenever `Namespace` ended with it as a
string, and callers commonly set `Namespace` to the full path including the child's own name. That
rule also collapsed layouts that merely shared a name or a name suffix onto one directory, losing
resources from the kustomize graph. A caller still joining the child's `Name` into its `Namespace`
would get that name twice (`.../<name>/<name>`), below the directory its parent's
`kustomization.yaml` lists, which then holds no `kustomization.yaml`. Every writer refuses such a
child before anything is written (go-kure/kure#979): a directory child that its parent lists and
whose `Namespace` is `<parent directory>/<Name>`, the path the child itself should have. The error
names both layouts, the directory written and the directory the parent lists; pass the parent's
path instead. The two paths are compared without regard to case, as the writers compare
directories: a `Namespace` that is the child's own path in another case (`apps/web` for a child
named `Web` under `apps`) is the same mistake, and the error says the difference is one of case.
Only that shape is refused. Layouts of one name built with the parent's path nest as
above (`web/web`), a child placed elsewhere on purpose is written as described above, and so is a
child no entry names: one its parent does not list (an umbrella child, one that renders bundles, a
directory child of a `FluxIntegratedPerLayout` parent, and for `WriteToDisk` and `WriteToTar` a
child of another package), or any child of a parent that writes no `kustomization.yaml`. The
directories compared are the writer's own, so under an `AppFileSingle` root the parent directory
is the root's `Namespace`.

**Breaking change (go-kure/kure#979).** A hand-built tree with such a child was written, with a
parent entry that named a directory without a `kustomization.yaml`. It is now refused.

Every writer (`WriteToDisk`, `WriteToTar`, `WriteManifest`) checks the whole tree before writing
anything, and refuses two layouts that resolve to the same directory (when one of them is a
bundle's own directory, the error names the bundle by its path, so two bundles that take one
directory name are told apart), or two `AppFileSingle`
layouts that resolve to the same file, or an `AppFileSingle` child with children of its own or
with `ConfigMapGenerators`, or any other layout it writes no `kustomization.yaml` for that has
`ConfigMapGenerators`.
It also refuses an `AppFileSingle` layout whose file, `<Name>.yaml`, or one of whose extra files
would replace another file written into the same directory: the `kustomization.yaml` there, one of
the resource or extra files of the layout that owns the directory, or another `AppFileSingle`
layout's file (go-kure/kure#871). A child named `kustomization`, or
named like one of its parent's generated files (`default-configmap-a`, or `configmap` under
`FileNamingKindName` + `FilePerKind`), is refused, as is an `AppFileSingle` root named
`kustomization`. A child with no resources writes no `<Name>.yaml`, so its name is never refused as
replacing another file; its directory, which the writers still create, and its extra files are
checked.
Nor may such a file need a directory where another layout writes a file, or be a file where
another layout needs a directory (go-kure/kure#878): an extra file `default-configmap-a.yaml/v.yaml`
next to the parent's `default-configmap-a.yaml`, an extra file `sub` where a child layout's
directory `sub` is (or a directory below it), or an extra file `x` of one `AppFileSingle` layout
and `x/y.yaml` of another. Nor may a layout's directory, including the one the writers create for
an `AppFileSingle` layout without resources, be or lie beneath a file another layout writes.
Before this, `WriteToDisk` and `WriteManifest` failed partway with `not a directory`, and
`WriteToTar` wrote an archive `tar` could not extract. Of the kustomize control files only
`kustomization.yaml` is written, so a directory named `kustomization`, `Kustomization` or
`kustomization.yml` clashes with nothing. An `AppFileSingle` layout's extra file inside a sibling
layout's directory (`sub/v.yaml` next to its parent's directory child `sub`) is written; that
directory's `kustomization.yaml` does not list it. A layout's own extra file inside its own
child's directory is refused (see [Extra Files and ConfigMap Generators](#extra-files-and-configmap-generators)).
Directories and file names are compared case-insensitively, as on default macOS volumes: an
`AppFileSingle` child named `Kustomization`, or `Default-ConfigMap-A`, is refused too, and so is a
directory child `Default-ConfigMap-A.yaml` next to its parent's `default-configmap-a.yaml`.

The writers also refuse two layouts that hold an object with one kustomize identity when one
kustomize build takes in both (go-kure/kure#880): kustomize refuses the second copy (`may not add
resource with an already registered id`), so that build would fail. The identity is kustomize's
own: group, version, kind, name and namespace, where an omitted namespace counts as `default` and
a kind kustomize knows is cluster-scoped (such as `Namespace`) has none, even when the object sets
one; so one layout holding such an object with and without a namespace field is refused too. Two versions of one object (`autoscaling/v1` and `autoscaling/v2`) are two identities, and
kustomize builds both. A build is every `kustomization.yaml` a writer writes, with the files and the
`AppFileSingle` child files it lists and, recursively, the build of each child directory it lists;
and the Flux build of a `KustomizationRecursive` directory marked with `SetFluxBuild` (see
"Kustomization Generation"). The error names both layouts and the innermost build. A child its
parent does not list (an umbrella child, one that renders bundles, a directory child of a
`FluxIntegratedPerLayout` parent, or under `WriteToDisk` and `WriteToTar` a directory child of
another package) is a build of its own, so it may hold an object its parent holds. An
`AppFileSingle` child of such a parent is still listed, so its objects count in the parent's build.

The ConfigMap a `configMapGenerator` generates counts as an object of the layout whose
`kustomization.yaml` holds the generator (go-kure/kure#894): `v1` `ConfigMap`, the generator's
name and no namespace, so `default`. kustomize compares the name before the content-hash suffix, so
two generators of one name are one identity whatever their files hold. It refuses a generator
whose ConfigMap its own kustomization already holds (`id ... exists; ... behavior must be merge or
replace`; kure writes no `behavior`), and one object reaching a kustomization twice from its
resources, the generator's ConfigMap included (`may not add resource with an already registered
id`). So a generator named `x` is refused next to a ConfigMap `default/x`, or one without a
namespace, anywhere in the same kustomize build (its own layout, a listed child, the parent that
lists it, or a sibling that parent lists), and so are
two generators named `x` in one layout or in two layouts of one build. A ConfigMap `x` in another
namespace, or a generator `x` in a child its parent does not list, is written.

In a tree Flux delivers, the writers refuse a directory that is written and that nothing applies
(go-kure/kure#977). A tree is one Flux delivers when its root is marked with `SetFluxBuild`: the
fluxcd integrator marks the root of every tree it generated a Kustomization for, and a caller that
places Flux Kustomizations itself marks its tree the same way. In such a tree every child its
parent's `kustomization.yaml` does not list (the children named above: an umbrella child, one that
renders bundles, a directory child of a `FluxIntegratedPerLayout` parent, and under `WriteToDisk`
and `WriteToTar` a directory child of another package) is applied only by a Flux Kustomization
whose `spec.path` names its directory, so it must be marked as well. An unmarked one is refused,
naming the layout and its parent. A child of another package is not exempt: those two writers
write its directory into the tree and nothing in the tree lists it. An `AppFileSingle` child
without resources writes no file and is not checked. A tree whose root is not marked, an Argo CD
tree included, is not checked.

The writers also refuse two Flux Kustomizations with one namespace and name anywhere in a tree,
marked or not (go-kure/kure#977). The checks above refuse one object held twice in what one
kustomize build takes in; two Kustomizations in directories that are applied separately pass
those, yet they are one object in the cluster, where each apply replaces what the other wrote. A
Kustomization is matched by the group `kustomize.toolkit.fluxcd.io` and its kind, at any version,
and an omitted namespace counts as `default`. The error names both layouts. A Kustomization inside
a List counts, where a List is what kustomize opens as one: an object written with a kind ending
in `List` and that has an `items` field, opened again when an item is itself such a List. A kind that does
not end in `List` is one object, whatever fields it has, and so is a List kind without `items`; a
List whose `items` is null holds nothing. An item a typed List holds as raw JSON
(`runtime.RawExtension`) is read as the object it encodes, also when the item carries an object
beside the raw JSON: the raw JSON is what is written.

Every check above that compares objects reads a resource that way (go-kure/kure#1006): the check
of one layout, the check of one build and the check of Kustomization names take a resource for the
objects kustomize builds from it, so two objects of one identity are refused however many Lists
deep one of them sits. For a typed object the kind and the `items` field are the ones in the
written file, not what its Go value reports: a typed object written without a kind, as one whose
`TypeMeta` is left out of what is written is unless another field writes a kind, is one object
without a kind, whatever its Go type, and one that writes a kind ending in `List` is opened as a
List. A typed List kind that leaves an empty `items` out is one object, and one that writes other items
than its Go value holds is read for the ones it writes. An item of a typed List need not
carry object metadata to be read: a List without any, such as the core `v1` List as a Go value
(`metav1.List`), is opened like any other, and an item held as raw JSON is the object it encodes.
An item that is none of these and has no object metadata is refused: its namespace and name cannot
be read. An item that is a nil pointer is written as `null` and holds nothing. A List that holds
itself among its items is refused in two forms: an unstructured one, and a typed list as
apimachinery takes one (a pointer to a struct with an `Items` field, `metav1.List` for one) that
holds itself as the object of an item, directly or through such lists. No other value that reaches
itself is looked for: building one is the calling program's error. The Flux integration reads a
resource by the same rule, so it and the writers agree on which objects a tree holds.

Read the same way, every object a build takes in must be written with a `kind` and an
`apiVersion` (go-kure/kure#1020): each resource, and each item of a List however many Lists deep,
typed or unstructured, judged on what is written for it, not on its Go type. A typed object is
written as it is, so one whose `TypeMeta` is unset becomes a document with neither; the writers
refuse a layout that holds one before anything is written, naming the layout, the object and what
it lacks. Set the `TypeMeta`, as this library's `Create*` constructors do, or the two fields of an
unstructured object. The type is not filled in from a scheme, which would hide the mistake, and
only an empty field is refused: whether the kind exists or the apiVersion is well formed is not
looked at. A kustomize build fails on a document without a kind, and one without an apiVersion
builds into an object of no version that nothing applies. A List's own `apiVersion` is exempt:
kustomize drops the envelope without reading it, so a List without one whose items are complete
builds and applies as its items do, and is written as before. So is the `apiVersion` of an object
with kustomize's `config.kubernetes.io/local-config` annotation at any string value but `"false"`:
kustomize drops such an object from the build without requiring one, so it is written as before. Its
`kind` is still required, since kustomize reads the kind before it drops anything.

### 2. LayoutRules Configuration
- **NodeGrouping**: whether each child node gets a directory (`GroupByName`, default) or merges into its parent's (`GroupFlat`; the root keeps its directory)
- **BundleGrouping**: whether each bundle gets a directory inside its node's (`GroupByName`) or renders in the node's directory (`GroupFlat`, default; the root node's bundles get one directory inside the root's, see [The root node's bundles](#the-root-nodes-bundles))
- **ApplicationGrouping**: whether each application gets a directory inside its bundle's (`GroupByName`) or writes into the bundle's directory (`GroupFlat`, default)

The three axes are independent and apply the same way to the root node (also under a
`ClusterName`), to umbrella children and in `WalkClusterByPackage`, with the one exception below.
A level set to `GroupFlat` is rendered into the layout above it: its resources, its child layouts
and its origins. Umbrella child bundles and augmenter applications always keep their own directory,
because each carries its own Flux Kustomization or writer-owned files. `FilePer` is honoured on
every layout. The writers refuse
a merge that makes two layouts claim one directory, and a directory that would hold two objects
with the same group, kind, namespace and name (kustomize cannot build it); objects that merely
share a file name are written into one multi-document file, as `FilePerKind` intends.

A name that becomes a directory must be one path segment (`stack.ValidateDirectoryName`): not
empty, not `.` or `..`, and without `/`, `\` or a NUL byte. Bundle and node names are checked by
`stack.ValidateCluster`, which every walk runs first. That check does not look at the rules: a
node that `NodeGrouping: GroupFlat` absorbs into its parent's directory is checked too, because
its name is a segment of the node's path in the model. An application name is checked by the walk
itself, and only where the rules give the application a directory: under `GroupByName`, or for an
augmenter application that takes its own layout under either grouping. An application written
into its bundle's directory keeps any name. The check runs on the tree the walk returns, which
holds every application directory: `FlattenSingleTier` collapses none. The
error names the application and the directory it would have been created in.

Wherever a bundle's name becomes a directory, the directory is named by `Bundle.DirName`, or by
`Bundle.Name` when that is empty: an umbrella child's directory, a node's bundle's under
`BundleGrouping: GroupByName`, and the one the root node's bundles get inside the root node's
under `GroupFlat` (see [The root node's bundles](#the-root-nodes-bundles)). The name of the
bundle's Flux Kustomization or ArgoCD Application is not affected, and its path follows the
directory: a child with `Name: "shop-infra"` and `DirName: "00-infra"` is written to
`<umbrella>/00-infra` and applied by the Kustomization `shop-infra`. Under `BundleGrouping:
GroupFlat` below the root a node's bundle is rendered in its node's directory and its `DirName`
has no effect. A `DirName` is checked like a name that becomes a directory, by
`stack.ValidateCluster` before the walk, whether or not the rules give the bundle a directory;
it is no object name, so nothing else is asked of it. The directory is named on the bundle:
`IndexOrigins` refuses a bundle's own layout that was renamed after the walk.

- **FilePer**: How resources are written (FilePerResource vs FilePerKind)
- **FluxPlacement**: Where/at what granularity Flux Kustomizations go — `FluxSeparate`, `FluxIntegratedPerLayout` (a CR per layout node), or `FluxIntegratedPerBundle` (CRs at bundle boundaries; application children included as directories)
- **FileNaming**: Resource file naming pattern (see [File Naming Modes](#file-naming-modes))
- **ClusterName**: Optional cluster name prefix for cluster-aware directory paths

The rules carry no application file mode: no walk sets one on the layouts it builds. Whether an
application is written as one file is the layout's own `ApplicationFileMode` (every writer), or
`Config.ApplicationFileMode` as the default for application layouts in `WriteManifest`.

`WalkCluster` and `WalkClusterByPackage` validate the rules they are given before they build
anything (`LayoutRules.Validate`), so no caller has to and no entry point that walks checks on its
own. The rules are validated as given, then unset values take their defaults: an unset value is
valid. Two things are refused, each with an error naming the field and the value:

- an unknown value of `NodeGrouping`, `BundleGrouping`, `ApplicationGrouping`, `FilePer`,
  `FluxPlacement` or `FileNaming`. It is not walked as if it were another value;
- a `ClusterName` with a `..` path segment (`../prod`, `clusters/../prod`). The cluster directory
  is the top layout's `Namespace`, which the writers refuse with such a segment, so the walk
  refuses it first. This is stricter than the writers in one case, by design: `x/../platform`
  over a root node `platform` would be cleaned to `platform` and written, and is refused all the
  same. Every other spelling is accepted and written: `.`, a nested or rooted path, a trailing
  slash, dots inside a segment (`a..b`).

The rules are checked whatever the cluster is: a nil cluster with invalid rules returns the rules
error, not a nil layout.

#### The root node's bundles

The root node's directory renders no bundle (go-kure/kure#979). A directory that renders bundles
is applied by its own Flux Kustomization, which its parent directory hosts; the root node's
directory can be the top of the tree, which has no parent, so its Kustomization would be part of
the build it applies.

With `BundleGrouping: GroupFlat` the bundles that would render in the root node's directory (the
root node's own, and under `NodeGrouping: GroupFlat` those of every node it absorbs) are therefore
rendered into one directory inside it, named after the first of them:

| Root node | Bundle | `BundleGrouping` | Bundle directory |
|---|---|---|---|
| `platform` | `platform` | `GroupFlat` | `platform/platform` |
| `platform` | `core` | `GroupFlat` | `platform/core` |
| `platform`, child node `web` absorbed (`NodeGrouping: GroupFlat`) | `core`, `web` | `GroupFlat` | `platform/core` for both |
| unnamed, `ClusterName: "."` | `core` | `GroupFlat` | `core` |
| `platform` | `core` | `GroupByName` | `platform/core`, as for every node |

They stay one unit: one directory and one Flux Kustomization or ArgoCD Application, named after
the first bundle. The directory takes that bundle's `DirName`, or its `Name` without one (a
`DirName` on a bundle merged into it later has no effect); its `KustomizationName`, when it sets
one, names the Kustomization or Application alone. `ManifestLayout.OriginUnit()` on the root node's layout returns that directory's
layout. Every other node is unchanged: its bundle renders in the node's directory. Both walkers do
this, `WalkClusterByPackage` in the tree of the package the root node is in. The unnamed wrapper `WalkClusterByPackage` builds for a package the root node is not in is
not the root node's directory and is unchanged: under `NodeGrouping: GroupFlat` it renders the
bundles merged into it, and the Flux integration refuses such a tree (its top renders a bundle).

A child node of the root that carries the name of that directory is refused by the walk, since
both would be written to one place; the error names the node, the bundle and the directory. The
two directories are compared as the writers compare them (`ManifestLayout.SameDirectory`: the
path resolved under the output directory and cleaned, without regard to case; it compares
directories, says nothing about an `AppFileSingle` layout, which is a file, and does not
describe `WriteToDisk("")`, which writes a rooted path as an absolute one), so a node name that
differs only in case (`WEB` beside the bundle `web`) is refused too. Rename one of them, or give
the bundle's directory another name with `DirName`: the check compares the directory the bundle
gets, so it follows a `DirName`, and names the bundle by its `Name`. A name
that is not one path segment does not reach the walk: `stack.ValidateCluster` refuses a node
named `/web` or `./web`, and a bundle named `.`, `/` or `web/api`, before it. The bundle's
directory is therefore always one inside the root node's. A layout below a child node can still
be rendered to it, when an application's `LayoutAugmenter` adds one with that name and place;
the walk refuses the tree, and the error names the bundle, the directory and what takes it.
`BundleGrouping: GroupByName` is no way around either: the bundle's directory is the same
there, the walk does not check it, and the writers refuse the tree.

The workflow engines add a directory of their own to the top of the tree: `flux-system` (Flux,
`FluxSeparate`) and `argocd` (ArgoCD). When the top is the root node's directory, a root node's
bundle whose directory has that name (its `DirName`, or its `Name` without one) or a child node
with that name would share it, and the engine refuses the tree, naming the bundle or node and the
directory. A bundle named `flux-system` whose `DirName` is another name is accepted. In the path
patterns below, `<bundle>` is that directory name too: the bundle's `DirName`, or its `Name`
without one.

An unnamed root node under a `ClusterName` is rendered into the `ClusterName` directory, and its
bundle into `<ClusterName>/<bundle>`. Every writer writes the `ClusterName` directory its
`kustomization.yaml`, also when the directory holds no resource of its own. Without it, a Flux
Kustomization that builds the directory would take in every file below it, each bundle's included.

**Breaking change (go-kure/kure#979).** `WriteManifest` used to make one exception: it wrote no
`kustomization.yaml` for a layout with no `Name` and a one-segment `Namespace` that held no
resource file and no `AppFileSingle` child's file and rendered no bundle, neither itself nor in
a directory inside it (`OriginUnit()`). A walk gives that shape as a one-segment `ClusterName`
directory (`prod`, not `clusters/prod`); a caller or an application's `AugmentLayout` can set it
anywhere in a tree. `WriteToDisk` and `WriteToTar` never made that exception. It is gone:
`WriteManifest` writes such a layout's `kustomization.yaml` under
the same rules as any other layout's (none for a `KustomizationRecursive` layout; for an
`AppFileSingle` one only at the root of the written tree, when it has resources or children). A
tree written with `WriteManifest` can so have one more file, at its top when the layout is the
`ClusterName` directory. That file is created like every other one the writer writes: a file of
that name already in the directory is replaced, and a caller that writes its own there afterwards
replaces kure's. The file lists the directory's children, so `WriteManifest` now holds the build
it starts to every check a directory with a `kustomization.yaml` gets, as the other two writers
do, and refuses a tree it used to write when that directory fails one of them. Two examples, not
a complete list: a child directory named `kustomization.yaml`, the path of the file now written
(a walk gives it for a root node of that name below a `ClusterName`), and listed children that
hold one object twice, which kustomize refuses in a build.

**Breaking change (go-kure/kure#979).** Before, the root node's bundles rendered in the root
node's directory. Their files, the Flux Kustomization's `spec.path` and the ArgoCD Application's
`source.path` move one directory down, from `<root>` to `<root>/<first bundle name>` (that
bundle's `DirName` when it sets one, go-kure/kure#972). On a
deployed tree with `prune` on, the objects can be deleted by the outer owner and re-created by the
inner Kustomization.

### 3. Two Main Walker Functions
- **WalkCluster()**: Standard hierarchical layout (Node → Bundle → App structure)
- **WalkClusterByPackage()**: Groups by PackageRef for multi-source scenarios

### 4. Writing System
- **WriteManifest()**: Config-driven writing — uses `Config` to resolve file naming, kustomization mode, and directory structure
- **WriteToDisk()**: Self-contained method on ManifestLayout — uses the layout's own `FileNaming` and `FluxPlacement` fields
- **WriteToTar()**: Same as WriteToDisk but writes to a tar archive for OCI artifact consumers
- **WritePackagesToDisk()**: Package-based writing with sanitized directory names
- All writers auto-generate kustomization.yaml files with proper resource references

## Directory Structure Patterns

### Standard Layout (WalkCluster)
```
clusters/
  cluster-name/
    node1/
      bundle1/
        app1/
          manifest-files.yaml
          kustomization.yaml
        app2/...
      bundle2/...
    node2/...
```

### Package-Based Layout (WalkClusterByPackage)  
```
oci-packages/
  web/
    app-manifests.yaml
git-packages/
  monitoring/
    app-manifests.yaml
```

### Flat Layout (GroupFlat rules)
```
clusters/
  cluster-name/
    root-node/
      kustomization.yaml
      first-bundle/
        all-manifests-together.yaml
        kustomization.yaml
```

The root node's directory renders no bundle, so the merged bundles share one directory inside it
(see [The root node's bundles](#the-root-nodes-bundles)).

## GitOps Tool Compatibility

### Flux Integration
- Every Kustomization `spec.path` is the directory of the layout that renders the bundle,
  e.g. `cluster-name/node` (see [Layout origins](#layout-origins))
- Auto-generates kustomization.yaml files
- Handles FluxSeparate, FluxIntegratedPerLayout and FluxIntegratedPerBundle placement modes

### ArgoCD Integration  
- `spec.source.path` is the same layout directory, e.g. `cluster-name/node`
- Requires explicit kustomization.yaml files (no auto-discovery)
- Each target directory needs its own Application

## Advanced Features

### Package Reference Support
- Tracks different source types (OCIRepository, GitRepository, Bucket)
- Enables multi-source deployments with proper isolation
- Sanitizes package keys into valid directory names

### Flexible File Organization
- **FilePerResource**: Each K8s object gets its own file
- **FilePerKind**: Group objects by Kind (all Services together, etc.)
- **AppFileSingle**: All app resources in one file

### File Naming Modes

Controls how resource YAML files are named:

| Mode | Format | Example |
|------|--------|---------|
| `FileNamingDefault` | `{namespace}-{kind}-{name}.yaml` | `default-service-web.yaml` |
| `FileNamingKindName` | `{kind}-{name}.yaml` | `service-web.yaml` |

`FileNamingKindName` drops the namespace prefix, which is useful when each application already has its own directory (e.g., Pattern A / CentralizedControlPlane). The naming mode is propagated through all writers: `WriteManifest`, `WriteToDisk`, and `WriteToTar`.

`WriteManifest` names every layout's files from its `Config`. `WriteToDisk` and `WriteToTar` name
each layout's files from that layout's own `FileNaming`, which `LayoutRules.FileNaming` sets across
the tree:

- every layout a walker creates carries the rules' `FileNaming`;
- a layout an augmenter adds and leaves unset takes its parent's, at any depth;
- the `flux-system/` directory of `FluxSeparate` takes the rules' (the root layout's when the rules
  passed to `IntegrateWithLayout` leave it unset);
- a layout that sets its own `FileNaming` keeps it, and the layouts below it inherit that one.

A layout added to a tree by hand, outside a walker, is not touched: set its `FileNaming` yourself.
A layout `FlattenSingleTier` absorbs into the root has no directory of its own any more: its files
take the root's `FileNaming`, and its own only when the root's is unset.

**Breaking change (go-kure/kure#976).** Before, the `flux-system/` directory and the layouts an
augmenter added always used the default pattern unless the augmenter set `FileNaming` itself. With
`FileNamingKindName` (the `CentralizedControlPlane` preset included) those files are renamed:
`flux-system/flux-system-kustomization-<name>.yaml` becomes `flux-system/kustomization-<name>.yaml`,
and an augmenter layout's `<namespace>-<kind>-<name>.yaml` becomes `<kind>-<name>.yaml`. Each
directory's `kustomization.yaml` lists the new names. Update anything outside kure that reads those
files by name. An augmenter that needs the old names sets `FileNamingDefault` on the layouts it adds.

### Kustomization Generation
- **KustomizationExplicit**: Lists all manifest files explicitly
- **KustomizationRecursive**: Writes no `kustomization.yaml` into the layout's directory; every
  other file is written as in Explicit mode. The directory is meant for a Flux Kustomization,
  which generates the `kustomization.yaml` from every `.yaml` and `.yml` file below it, adding a
  subdirectory that has its own kustomization file as a whole (go-kure/kure#868). Every writer
  refuses, before writing anything:
  - `ConfigMapGenerators` on a Recursive layout: they exist only inside a `kustomization.yaml`;
  - a parent's `kustomization.yaml` that lists a Recursive layout's directory, which kustomize
    cannot build without one.

  A Flux Kustomization kure generated builds a directory when `SetFluxBuild` marks it: the fluxcd
  integrator marks each generated `spec.path` layout, and the root, which the Flux bootstrap
  applies. Nothing else marks: a caller placing Flux Kustomizations from `GenerateFromLayout` or its
  own code calls `SetFluxBuild` on the root and on their `spec.path` layouts to get these checks.
  Once the root is marked, every directory its parent does not list must be marked too (see
  "Layout paths" above): an unmarked one is a directory nothing applies.

  A marked Recursive directory that holds no file is refused (go-kure/kure#904): no resource file,
  extra file, `AppFileSingle` child file, file of a layout below it or `kustomization.yaml` a
  directory below it gets lands at or below it. A marked `AppFileSingle` root with no resource is
  refused the same way, since it writes no file either. The writers write it no `kustomization.yaml`, so
  every writer would leave an empty directory, which a Git tree drops: the Kustomization would
  name a path the committed output does not contain. This happens to a bundle with no applications yet, one whose applications were all
  removed, or one whose applications render nothing, when the whole tree is Recursive. Give the
  layout a resource, or write it `KustomizationExplicit`, which writes it `resources: []`.

  For a marked Recursive directory, the writers also refuse what would make its build differ from the Explicit
  mode's (its build is its own directory and every directory below it that no `kustomization.yaml`
  shields):
  - another marked layout below it with no `kustomization.yaml` in any directory in between (its
    own does not count): its objects would be applied twice;
  - an extra file named `*.yaml` or `*.yml` in its build: Flux would apply it, or fail the whole
    build on one it cannot decode, and Explicit mode never lists extra files;
  - an extra or resource file named like a kustomization file (`kustomization.yaml`,
    `kustomization.yml`, `Kustomization`) in its build: Flux would build the directory holding it
    as a kustomization;
  - a resource file whose name does not end in `.yaml` or `.yml` (only a `Config.ManifestFileName`
    can produce one): Flux would skip it, where Explicit mode lists it;
  - the file of an `AppFileSingle` umbrella child, or of one that renders bundles, in its build:
    Flux would apply it, where Explicit mode leaves it to the child's own Kustomization;
  - the file of an `AppFileSingle` child in its build that a `kustomization.yaml` at or above the
    Recursive directory lists (the child's parent lists it by its path below the parent's
    directory, see "Layout paths"): both builds would apply it;
  - a child directory its parent's `kustomization.yaml` would not list, in its build: under
    `WriteToDisk` and `WriteToTar`, a child of another package (its `PackageRef` and its parent's
    are both set, with different values). Flux would apply
    it, where Explicit mode leaves it out. `WriteManifest` lists such a child, so it is accepted
    there.

  Two layouts in a marked Recursive directory's build that hold one object are refused as well,
  as in any other build (see "Layout paths"): that fails the Flux build and the Explicit one alike.

  So Recursive fits a generated target with no other target below it: a leaf bundle directory,
  or a GroupByName bundle directory over the application directories it lists, plus any unmarked
  Recursive layout inside one. A Recursive layout with children under `FluxIntegratedPerLayout`
  is refused (each child is a target), and so is a Recursive root over any target no
  `kustomization.yaml` shields. How Argo CD or a caller's own Kustomization treats a Recursive
  directory is not checked: Argo CD Applications do not set `directory.recurse`
  (go-kure/kure#144), and an unmarked Recursive directory holding no file is accepted, although
  Git does not keep an empty directory.
- A `FluxIntegratedPerLayout` layout references no child directory: each child is applied by the
  Flux Kustomization the integrator placed in the parent's `Resources`, listed as one of its own
  files. No reference is derived from a child's name.
- No layout references a child that renders bundles, in any placement: that child is a
  reconciliation unit, applied by its own Flux Kustomization or ArgoCD Application and by nothing
  else, so every object has one owner. Building a parent directory therefore does not include its
  child units; a tree written without Flux or ArgoCD integration has no CR applying them.
- GroupByName bundle layouts are written `KustomizationExplicit`, so the CRs hosted there
  (umbrella children, PerLayout applications) are listed
- A layout that renders a bundle, and every child of a `FluxIntegratedPerLayout` layout, always
  gets a `kustomization.yaml` unless it is `KustomizationRecursive`, even when it holds nothing else (a bundle with no applications, an
  empty node, an empty augmenter layout): a Flux Kustomization or ArgoCD Application names that
  directory, and an empty directory does not survive a Git tree. A kustomization that lists
  nothing is written `resources: []` (kustomize rejects a bare `resources:` as empty). A
  Recursive one that holds nothing and that a generated Flux Kustomization names is refused, as
  described above.
- A `resources` entry, a `configMapGenerator` name and a generator's `files` entry are written
  plain when kustomize reads them back as the same string, and double-quoted otherwise
  (go-kure/kure#896). kustomize reads a `kustomization.yaml` as YAML 1.1, so a child directory or
  generator named `y`, `on`, `null` or `1e3` would otherwise be read as a bool, null or number and
  fail the build.
- An `AppFileSingle` child writes one file, `<Name>.yaml`, into its `Namespace`, normally its
  parent's directory, and never a
  `kustomization.yaml` of its own (before go-kure/kure#860 it replaced the parent's, dropping the
  parent's files), and its file may not take the name of a file the parent writes there (see
  "Layout paths"). The parent's `kustomization.yaml` lists that file next to its own, by its path
  relative to the parent's directory (`sub/<Name>.yaml` for a child below it), and the writers
  refuse a listed file outside that directory or in a spelling of it that differs only in case; in
  `WriteManifest` the child's mode is its effective one, so a child with no mode of its own under
  `ArgoProfile`'s `AppFileSingle` is listed as `<Name>.yaml`, not as a directory, also by the
  unnamed cluster root. A child with no resources writes no file, so nothing lists it; a cluster
  root that holds only such a child still gets its `kustomization.yaml`, which lists nothing.
  An `AppFileSingle` root, with no parent to list its file, still writes a `kustomization.yaml`
  next to it, which lists its children in that same directory (see "Layout paths").
- Every writer refuses an `AppFileSingle` child that has children of its own before writing
  anything: it writes no `kustomization.yaml`, so nothing would list them and they would drop out
  of the build. The walkers never build one themselves; the root is not checked, since the synthetic cluster
  wrapper a walker builds takes `ArgoProfile`'s `AppFileSingle` in `WriteManifest`.
- Every writer likewise refuses an `AppFileSingle` child, with or without resources, that carries
  `ConfigMapGenerators` (go-kure/kure#891): a `configMapGenerator` exists only inside a
  `kustomization.yaml`, the child writes none, and before this its generators were silently
  dropped. They are not moved into the parent's `kustomization.yaml`. In `WriteManifest` this
  includes any application layout written as one file through `Config` (`ArgoProfile`, or
  `ApplicationFileMode: AppFileSingle` set explicitly) that an augmenter gives generators: under
  `WalkCluster` with every Flux placement but `FluxIntegratedPerLayout`, and under
  `WalkClusterByPackage` with every placement, since it clears the placement. An augmenter that also sets
  its layout's `ApplicationFileMode` to `AppFilePerResource` keeps its generators: the layout gets
  its own directory and `kustomization.yaml`.
- The same holds for any other layout the writer writes no `kustomization.yaml` for
  (go-kure/kure#899). That is an `AppFileSingle` root with no resources and no children, in every
  writer, including one made `AppFileSingle` through `Config` in `WriteManifest`. A layout shaped
  like the synthetic cluster root (`Name ""`, a single-segment `Namespace`) with nothing to list
  was one too in `WriteManifest`; since go-kure/kure#979 it gets its `kustomization.yaml` there
  as in the other writers, and its generators are written into it. A `KustomizationRecursive`
  root, `AppFileSingle` or not, is refused as
  Recursive. Each is refused before anything is written when it carries `ConfigMapGenerators`,
  which were silently dropped before. `FlattenSingleTier` no longer builds such a root: it
  moved a lone augmenter application and its generators onto the root, and since
  go-kure/kure#979 it collapses no directory that renders a bundle. Give such a root a resource,
  or put the generators on a layout that writes a `kustomization.yaml`.

### Layout origins

Every layout the walkers build records what it renders: `OriginNodes()`, `OriginBundles()` and
`OriginApplication()`. A node layout renders its node (plus its bundle when `BundleGrouping` is
`GroupFlat`); a GroupByName bundle layout and an umbrella-child layout render their bundle; a
per-app layout its application. The root node's layout renders its node and no bundle: with
`BundleGrouping: GroupFlat` its bundles are rendered by the directory `OriginUnit()` returns (see
[The root node's bundles](#the-root-nodes-bundles)); on every other layout `OriginUnit()` is nil.
A level merged by a `GroupFlat` axis moves the absorbed nodes and bundles into the absorbing
layout's origins, and a `FlattenSingleTier` collapse the absorbed node; a merged application has
no origin of its own. Hand-built layouts have none.
`OriginBundleObjects(b)` returns the objects bundle `b`'s applications render in that directory
or its per-app directories (plus a stand-in for each ConfigMap a `configMapGenerator` makes in an
augmenter application's layout or a layout below it: its kind and name, and the annotations the
entry sets at the time of the call): the Flux generator uses it to refuse a patch that would
reach another bundle sharing the directory.
`OriginApplicationObjects()` returns, per application rendered in a layout, the application, its
objects and its own layout: the objects are what the application emitted plus, for an application
with its own directory, what a `LayoutAugmenter` added there or below; the layout is nil for an
application written into its bundle's directory. The records are on the layout that renders the
application's bundle, under every grouping, and a `FlattenSingleTier` collapse leaves them there:
it collapses no directory that renders a bundle (go-kure/kure#979). A workflow uses them to apply
an application's `Delivery` intent to exactly its objects.

`IndexOrigins(root, cluster)` resolves a cluster's bundles and nodes to those layouts
(`BundleLayout`, `NodeLayout`, `Parent`, `Bundles` in layout pre-order) and
`KustomizationPath(b)` is the directory of the layout that renders `b` — the one path every Flux
Kustomization and ArgoCD Application kure emits for a bundle uses. It refuses a tree it cannot
resolve: an object rendered twice, a rendered set that differs from what the cluster reaches
(hand-built, partial or other-cluster trees), two bundles with one name, two bundles whose
Kustomization or Application would get one name (`Bundle.UnitName`), a node or bundle
layout set to `AppFileSingle` mode, a layout that is a bundle's own directory (an umbrella child's,
a node's bundle's under `GroupByName`, or the root node's bundle's under `GroupFlat` when no
other bundle is merged with it) and is not named by the bundle's `DirName` or else its
`Name`, two bundles whose own directories are one (compared as the writers compare them, so two
`DirName`s that differ in case only are one; named by their paths, as the writers name
them), and a dependency cycle between units. `WriteManifest` refuses a
node or bundle layout whose own `ApplicationFileMode` is `AppFileSingle` too: its files would go
into its `Namespace`, so no directory would exist at its path. `Config.ApplicationFileMode` is only
the default for application (and hand-built) layouts, so `ArgoProfile`'s `AppFileSingle` writes one
file per application while nodes and bundles keep their directories.

`Units()` returns the layouts that render bundles: each is one reconciliation unit, the directory a
Flux Kustomization or ArgoCD Application applies. `UnitName(b)` is the unit that applies `b`: the
name in effect (`Bundle.UnitName`: `KustomizationName`, or `Name` without it) of the first bundle
its directory renders. `UnitOfName(name)` maps a Kustomization name to its unit, and returns a name
no rendered bundle has in effect unchanged. `UnitDependencies(l)` / `UnitNamedDependencies(l)` map
the bundles' `DependsOn` / `NamedDependsOn` to units, dropping dependencies inside `l`'s own unit.
`IndexOrigins` refuses a cycle in those dependencies; waits a workflow adds on top (Flux health
checks, creation order) are checked by that workflow.
Bundles are resolved by `Name`, which is unique, so a copy of a bundle resolves like the original.
A `DependsOn` copy that leaves `KustomizationName` empty or sets the rendered bundle's name in
effect (`UnitName()`) is that bundle. `IndexOrigins` refuses one that sets another name, naming
both, and one that stands for a bundle whose Kustomization name is also in the dependant's
`NamedDependsOn` (one dependency in both lists), in the words `stack.ValidateCluster` uses.

`NodeGrouping: GroupFlat` (as in the `CentralizedControlPlane` preset) moves a merged node's
umbrella children and augmenter layouts under the absorbing node, where they keep their own
directories, files and Flux CRs.

### Extra Files and ConfigMap Generators

`ManifestLayout.ExtraFiles` lets callers attach arbitrary files (e.g. a `values.yaml`) into a layout's directory alongside the resource YAMLs. `ManifestLayout.ConfigMapGenerators` adds entries to a `configMapGenerator:` section in the generated `kustomization.yaml`. kustomize appends a content-hash suffix to the generated ConfigMap name and rewrites references (e.g. `HelmRelease.spec.valuesFrom`) on build, so any change to the source file forces re-reconciliation — the canonical FluxCD pattern for tracking Helm values changes.

A generator needs a `kustomization.yaml` the writer writes, so the writers refuse one on a
`KustomizationRecursive` layout, an `AppFileSingle` child, or a root that writes no
`kustomization.yaml`, and they refuse a generated ConfigMap
whose identity its build already holds (see "Layout paths").

`ConfigMapGeneratorSpec.Annotations` is written as the entry's `options.annotations`, in key order,
so the ConfigMap kustomize builds carries them. An entry without annotations is written as before,
with no `options` block.

An `ExtraFile.Name` is a relative path of `/`-separated segments made of letters, digits, `.`, `_`
and `-`, with no `.` or `..` segment; a name in a subdirectory (`assets/dashboard.json`) creates
that directory. Every writer (`WriteToDisk`, `WriteManifest`, `WriteToTar`) refuses, before writing
any file of the tree, an extra file that would take a path it owns in that directory: a generated
resource file, a kustomize control file (`kustomization.yaml`, `kustomization.yml`,
`Kustomization`), a child layout's output directory where it lies inside this layout's (umbrella
children included; anything under it, or a file on the way to it) or the `<name>.yaml` of an
`AppFileSingle` child with resources, or another extra file (listed twice); nor may it use any of those files as
a directory (`kustomization.yaml/x`), or be a file where one of them needs a directory. A child's
output directory is the one its writer uses (for `WriteManifest`, after applying the `Config`
defaults). All names are compared case-insensitively, as on default macOS volumes. A `..` segment
in the `Namespace` or `Name` of any layout of the tree refuses it before any file is written,
whether or not the layout has extra files (go-kure/kure#977): an augmenter can rename its layout
after the walk, and the final value is what the writers join into a path. When a layout has extra
files, a `..` segment in a generated file name refuses the tree as well. A
rooted name (`/x`) is not refused, since every writer joins it under its base. Previously such an
extra file silently replaced the generated one on disk, or shadowed it as a later tar entry, while
`kustomization.yaml` still listed the path.

`LayoutAugmenter` is an optional interface on `stack.ApplicationConfig`:

<!-- doc-example:excerpt the interface declaration, not a call -->
```go
type LayoutAugmenter interface {
    AugmentLayout(layout *ManifestLayout) error
}
```

When `app.Config` implements it, the walker invokes `AugmentLayout` on the per-app `ManifestLayout` after resource generation, giving the config a chance to attach `ExtraFiles`, `ConfigMapGenerators`, and sub-`ManifestLayout` children. It runs on every per-app layout a walker creates, in both walkers and inside umbrella children — with `ApplicationGrouping: GroupFlat` an augmenter app still gets its own per-app sub-layout instead of merging into its bundle's directory.

`LayoutIntentAugmenter` is an optional companion to `LayoutAugmenter`, for a config whose desire for its own layout varies per instance rather than being fixed for the whole type:

<!-- doc-example:excerpt the interface declaration, not a call -->
```go
type LayoutIntentAugmenter interface {
    LayoutAugmenter
    WantsOwnLayout() bool
}
```

`WantsOwnLayout()` gates placement only, and only where `ApplicationGrouping` is `GroupFlat` (in
`WalkCluster` and `WalkClusterByPackage` alike, umbrella children included):

| `ApplicationGrouping` | `WantsOwnLayout()` absent or `true` | `WantsOwnLayout() == false` |
|---|---|---|
| `GroupByName` | own layout + `AugmentLayout` | own layout + `AugmentLayout` |
| `GroupFlat` | own child layout + `AugmentLayout` | resources merged into the bundle's directory, `AugmentLayout` **not** called (no layout exists to pass it) |

A config that implements only `LayoutAugmenter` keeps today's presence-only behaviour unchanged.

After `AugmentLayout` returns, every layout below the per-app layout that leaves `FileNaming` unset
takes its parent's (see [File Naming Modes](#file-naming-modes)).

#### Sub-Layout Children and Flux Integration

Augmenters may attach sub-layouts as `Children` of a per-app `ManifestLayout`. In `FluxIntegratedPerLayout` mode each such child that is eligible (see below) receives a Flux `Kustomization` CR automatically placed in the parent layout's `Resources`.

**Eligibility for CR generation.** A child layout receives a Flux `Kustomization` CR when ALL of the following hold:

- Integration runs with `FluxIntegratedPerLayout`.
- `!child.UmbrellaChild`
- `child.ApplicationFileMode != AppFileSingle`
- The child renders no bundle (a child that does already has that bundle's CR in the parent).
- A source can be resolved: the `SourceRef` of the nearest layout at or above the parent that renders bundles (with `BundleGrouping: GroupFlat` the root node's layout renders none and counts with the `SourceRef` of its own bundle, see [The root node's bundles](#the-root-nodes-bundles); with `GroupByName` no node's layout counts), else the one `SourceRef` the URL-less bundles below the child share, else (no `SourceRef` without a URL below the child, or two that differ) the `SourceRef` of the bundle of the nearest node, at or above the child's own, that has one (the fluxcd README, "Per-layout name rule"). The Kustomization of the root node's layout, which exists below a `ClusterName` wrapper, takes no Source the integration generates, since it applies the directory that hosts them: such a `SourceRef` is passed over, and with no other left the integration is refused (go-kure/kure#979; the fluxcd README, "Layout Integration"). A missing or incomplete `SourceRef` (nil, empty, or without `Kind` or `Name`) causes `IntegrateWithLayout` to return a hard error — a `Kustomization` without `spec.sourceRef` is rejected by Flux.

The parent's `kustomization.yaml` lists that CR file as one of its own resources, so every child the writers do not reference as a directory has a backing CR. The integrator applies this rule at any depth.

#### Naming Constraint

In `FluxIntegratedPerLayout` mode a child layout's Flux `Kustomization` CR is named `<unit name>-<child Name>`, the unit name being the name of the Kustomization that applies the bundle the application belongs to (`web-nginx-00-pre-install` for the child `nginx-00-pre-install` below the bundle `web`). Set `ManifestLayout.KustomizationName` on the child to give the CR another name. The name in effect must be a Flux Kustomization name, a DNS-1123 subdomain of at most 63 characters. The default is longer than the child's own name by the unit name, so the integrator shortens a default over the limit to `<unit prefix>-<hash>-<child Name>`, keeping the child's name (fluxcd package README, "Per-layout name rule"); a default that fits is unchanged. A child name over 54 characters leaves no room for the hash, and a set name is never shortened: either one over the limit is refused, and is fixed by setting the field (the error names the layout and the field). Flux `Kustomization` CRs live in the `flux-system` namespace, so names must be **globally unique across all apps in the cluster** — two CRs with the same `metadata.name` collide, and the integrator refuses such a tree rather than skipping one of them, naming both owners.

Augmenters are responsible for ensuring uniqueness within their bundle. The recommended convention is to prefix each child name with the app name: `{appName}-{hookGroupDir}` (e.g. `nginx-00-pre-install`).

#### DependsOn

Set `ManifestLayout.DependsOn` to a list of sibling layout names. In `FluxIntegratedPerLayout` mode the layout integrator translates these into `spec.dependsOn` entries on the child's `Kustomization` CR, enabling ordered reconciliation between hook groups (e.g. pre-install → hooks → post-install). Each entry is written as the CR name of the layout it names, which is not the layout's own name (see above). An entry may also name another layout below the same bundle's directory, the parent application layout for instance; when two such layouts share the name and neither is a sibling, the entry is refused. An entry that names no such layout is written as given, as the name of a Kustomization. The CR of a node layout that renders no bundle is one such name: `<path with "/" replaced by "-">-node`, or the node's `KustomizationName`. The field applies only to a child layout that gets a CR of its own (not an umbrella child, not `AppFileSingle`, rendering no bundle). In every other case, and under the other placements, the layout integrator and `ResourceGenerator.GenerateFromLayout` refuse it, naming the layout, every such field it sets, and `FluxIntegratedPerLayout` as the placement that carries them. An augmenter cannot see the placement, so an augmenter that orders its layouts is refused under `FluxSeparate` (the default) and `FluxIntegratedPerBundle`: integrate its application with `FluxIntegratedPerLayout`. A value the walker copied from a node (below) is refused on the node instead, by the node's path, and not again on its layout.

#### Wait, Timeout, RetryInterval, Labels, Annotations

Five fields of `ManifestLayout` are settings of the layout's Flux `Kustomization` CR in `FluxIntegratedPerLayout` mode: `Wait` (`*bool`), `Timeout`, `RetryInterval` (Go durations), `Labels` and `Annotations`. No object in the layout's directory gets them. Unset, the CR of an application's layout and of every layout below it takes the value of the bundle that holds the application; a set `Wait`, `Timeout` or `RetryInterval` replaces the bundle's (a `Wait` pointing at `false` turns an inherited wait off), and labels and annotations are merged with the bundle's key by key, the layout's value winning, so an inherited key cannot be dropped. Any other layout inherits nothing. The layout integrator refuses a duration that does not parse or that the Flux API does not take (a negative one, or a positive one under a millisecond) and a label or annotation the Kubernetes API would not accept, naming the layout. Like `DependsOn`, the fields apply only to a layout that gets a CR of its own, and are refused in every other case and under the other placements. See the Flux engine's "Per-layout settings" for where the values come from and what readiness through a chain of such CRs means.

#### Interval, Prune, Force, Suspend

Four more fields are settings of the same CR, under the same conditions: `Interval` (a Go duration), `Prune`, `Force` and `Suspend` (`*bool`). A set field replaces the value of the bundle that holds the layout's application, and a pointer to `false` turns an inherited `Prune`, `Force` or `Suspend` off. Unset, the CR takes the bundle's. Where neither sets one, `Interval` and `Prune` are the Flux generator's (`ResourceGenerator.DefaultInterval` and `Prune`), and `Force` and `Suspend` are left out of the CR. The layout integrator refuses an interval that does not parse or that the Flux API does not take, naming the layout. A layout has no field for patches or for a post-build substitution: those are the bundle's, and the Flux engine's "Per-layout settings" says which CR takes them.

#### Node fields

On a node's own layout the walker fills `KustomizationName` from `stack.Node.KustomizationName` and `DependsOn` from `stack.Node.NamedDependsOn`; those entries are Kustomization names and are written as given, whatever their value. A node that a grouping axis renders into another node's directory has no layout of its own, so nothing is copied for it. The copy is taken at the walk: a name or an entry the node gets afterwards is not on the layout, and the layout integrator refuses that node until the cluster is walked again or the layout carries the value. The layout integrator decides which nodes may set the fields: see the Flux engine's "Node Kustomizations". The walker fills none of the nine settings above on a node's layout, since `stack.Node` has no field for them: set them on the walked layout.

### ClusterName-Aware Layouts

Setting `LayoutRules.ClusterName` prepends the cluster name as a root directory, producing paths like `{clusterName}/{nodeName}/...` instead of `{nodeName}/...`. This is useful when a single repository manages multiple clusters. When the last segment of `ClusterName` is the root node's name (for example `ClusterName` `platform` with a root node `platform`), the root node is the cluster directory itself: the output is `platform/...`, not `platform/platform/...`.

Without a `ClusterName` the root node sits at `{rootName}` and every child node nests under it (`{rootName}/{childName}/...`). An unnamed root node has no directory of its own and stays at `cluster`, with its children at `cluster/{childName}` as before; deeper descendants now nest under them (`cluster/{childName}/{grandchildName}`), where they used to be written outside `cluster/`, unreferenced. `WalkClusterByPackage` places each package's root the same way, below the package directory; a package whose root node belongs to another package has no root directory of its own either, so its unnamed wrapper stays at `cluster`. A node outside the package adds no directory of its own: its in-package descendants attach to the nearest in-package ancestor (or the package root), including where the tree leaves a package and re-enters it lower down.

`TopDirectory(rootNode, rules)` returns the directory at the top of the tree `WalkCluster` builds: the one that holds the tree's first `kustomization.yaml`, and so the one to point Flux at. With a `ClusterName` it is the cluster directory; without one it is the root node's name, `cluster` for an unnamed root node, and `.` when the root node is nil (no tree is walked, and the caller applies the directory it writes to). It is relative to the directory the tree is written into and has no leading slash (`.` is that directory itself). The walk takes its top from the same place, so the two name the same directory, and the Flux bootstrap asks it for the directory it applies. Only the spelling differs, and only for a rooted `ClusterName` (`/prod`): the walk's own `FullRepoPath` keeps the slash and the writers resolve it under the directory they write to, which is the directory returned (`prod`). The rules are validated first, as `WalkCluster` validates them. Where the root node's name is the directory, a name that is no directory name (`../prod`, `a/b`, `.`) is an error, as it is to `WalkCluster`; under a `ClusterName` the name is no part of the directory and is not checked. Two limits: it does not describe a `WalkClusterByPackage` tree, which is placed without the `ClusterName`, one tree per package; and it does not know the directory a writer is given, so a tree written below a sub-path of the repository is at that sub-path joined with this directory.

### Flatten Single Tier (opt-in)

`LayoutRules.FlattenSingleTier` collapses one vestigial intermediate directory layer at the top of the tree when it adds no semantic value.

Conservative collapse preconditions — ALL must hold:

- `LayoutRules.FlattenSingleTier` is `true`.
- The parent layout is top-level (`Namespace` has no path separator).
- Parent has exactly one `Children` entry.
- Parent has no own `Resources`.
- The single child is not an `UmbrellaChild`.
- The single child has no `Children` of its own (terminal layer).
- The single child renders no bundle (go-kure/kure#979).

A directory that renders a bundle is applied by its own Flux Kustomization, which its parent hosts. Collapsing it into the top of the tree, which has no parent, would make that Kustomization part of the build it applies, so it is left where it is.

What it still collapses in a walked tree is a single child directory that renders no bundle and has no directory below it: a node that has neither a bundle nor child nodes, or, under `NodeGrouping: GroupFlat`, a node with the bundle-less nodes below it merged into its directory. That is a `ClusterName` directory over such a root node, or a root node over one such child. The absorbing layout takes over the collapsed layout's origin nodes (see [Layout origins](#layout-origins)). A hand-built tree, which carries no origins, collapses as before. Nothing is rewritten after generation: a Flux CR a caller adds to the walked tree keeps the `spec.path` it was given.

**Breaking change (go-kure/kure#979).** Before, a single child that rendered a bundle was collapsed too: a root node `apps` with one bundle under `ClusterName: cluster-name` was written to `cluster-name/`. It is now written to `cluster-name/apps/<bundle>/` (the bundle's `DirName`, or its `Name` without one) with or without the flag (see [The root node's bundles](#the-root-nodes-bundles)).

Scoped to `WalkCluster`. `WalkClusterByPackage` is unaffected — its synthetic unnamed wrappers express package boundaries that the flatten helper would otherwise erroneously collapse.

Default: `false` — no behaviour change for existing callers.

## Layout Presets

Three named presets provide pre-configured LayoutRules for common deployment patterns. Use `LayoutRulesForPreset()` to get rules, or `ConfigForPreset()` to get a matching Config.

| Preset | Pattern | FluxPlacement | NodeGrouping | FileNaming |
|--------|---------|---------------|--------------|------------|
| `CentralizedControlPlane` | A | FluxSeparate | GroupFlat | FileNamingKindName |
| `SiblingControlPlane` | B | FluxSeparate | GroupByName | FileNamingDefault |
| `ParentDeployedControl` | C | FluxIntegratedPerLayout | GroupByName | FileNamingDefault |

This block and the one under [Example Usage](#example-usage) are the bodies of `Example` functions
in `example_test.go`, which `go test` runs; the two interface blocks above are declarations, shown
as excerpts. The examples import this package as `layout`, `os`, `path/filepath`, and `fmt` for the
lines that print what they built. Two helpers are declared in the same file: `exampleCluster()`
builds cluster `prod`, whose root node `apps` holds one bundle `web` with applications `api` and
`ui`, each emitting a ConfigMap; `printFiles(dir)` prints every file below `dir`.

<!-- doc-example: pkg/stack/layout ExampleLayoutRulesForPreset -->
```go
rules, err := layout.LayoutRulesForPreset(layout.PresetCentralizedControlPlane)
if err != nil {
    panic(err)
}
cfg, err := layout.ConfigForPreset(layout.PresetCentralizedControlPlane)
if err != nil {
    panic(err)
}
fmt.Println(rules.FluxPlacement, rules.NodeGrouping, rules.FileNaming, cfg.KustomizationFileName("web"))
```
<!-- doc-example:end -->

## Real-World Use Cases

1. **Simple Cluster**: Single source, hierarchical structure
2. **Multi-OCI Deployment**: Different services from different OCI registries  
3. **Monorepo**: Everything flattened into minimal directory structure
4. **Bootstrap Scenarios**: Special handling for Flux/ArgoCD system components

## Example Usage

<!-- doc-example: pkg/stack/layout ExampleWalkCluster -->
```go
cluster := exampleCluster()
out, err := os.MkdirTemp("", "kure-layout-example")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(out) }()

// Create layout rules
rules := layout.DefaultLayoutRules()
rules.BundleGrouping = layout.GroupFlat
rules.ApplicationGrouping = layout.GroupFlat

// Walk cluster to create layout
ml, err := layout.WalkCluster(cluster, rules)
if err != nil {
    panic(err)
}

// Write to disk
cfg := layout.DefaultLayoutConfig()
err = layout.WriteManifest(filepath.Join(out, "out/manifests"), cfg, ml)
if err != nil {
    panic(err)
}
printFiles(out)
```
<!-- doc-example:end -->

## Key Files

- **types.go**: Core types and configuration options
- **walker.go**: Tree traversal algorithms (WalkCluster, WalkClusterByPackage)
- **manifest.go**: ManifestLayout structure and package-based writing
- **write.go**: Standard manifest writing with kustomization generation  
- **config.go**: Configuration and file naming conventions

The layout module essentially bridges the gap between Kure's programmatic resource construction and the file-based expectations of GitOps workflows, with extensive configurability for different organizational preferences and tool requirements.
