+++
title = "Bootstrapping Flux"
weight = 24
+++

Generate the Flux system manifests that point a cluster at the tree the [tutorial](/guides/flux-workflow/) writes.

The Go blocks on this page are the bodies of `Example` functions in `pkg/stack/fluxcd`, which `go test` runs. They use the imports and the helpers named in the [tutorial](/guides/flux-workflow/).

Generate Flux system bootstrap manifests. Two modes are available:

- **`"flux-operator"`** (default) — emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters.
- **`"gotk"`** — emits the legacy GitOps Toolkit component manifests directly.

When `FluxMode` is empty, it defaults to `"flux-operator"`.

`GenerateBootstrap` takes the layout rules you write the tree with, and both modes point Flux at
the top directory of that tree, relative to the root of the source: the cluster directory when
the rules have a `ClusterName`; without one the root node's name, `cluster` for an unnamed root
node, and the root of the source when you pass no root node. `"gotk"` mode writes it as the
bootstrap Kustomization's `spec.path` (`prod`, `clusters/prod`, or `.` for the root of the
source); `"flux-operator"` mode writes it as the `FluxInstance`'s `sync.path` (`./prod`,
`./clusters/prod`, or `./`). Pass the same rules you pass to `GenerateFromCluster` or
`WalkCluster`: the engine refuses rules that are not a `layout.LayoutRules` value, nil included,
and invalid ones.

**Breaking change (go-kure/kure#979).** `GenerateBootstrap` and `GenerateFluxInstance` take the
rules as a third argument. The bootstrap used to name the root node whatever the rules were, so
with a `ClusterName` both paths move to the cluster directory, and an unnamed root node without
one moves from the root of the source to `cluster`. A named root node without a `ClusterName`
keeps its path. Apply the regenerated bootstrap objects; until then a cluster keeps applying the
old directory.

Two limits. The directory is relative to the directory the tree is written into: if you write the
tree below a sub-path of the repository, or under a prefix such as the `manifests/<root>` that
`"gotk"` mode once wrote, set `spec.path` or `sync.path` on the returned object yourself. And it
describes a `WalkCluster` tree: `WalkClusterByPackage` writes one tree per package, placed
without the `ClusterName`, and the bootstrap does not point at those.

`"flux-operator"` mode needs both `FluxVersion` and `Registry`: they become the `FluxInstance`'s
distribution version and registry, and Kure has no default for either. If one is empty,
`GenerateBootstrap` returns an error naming the missing field and emits nothing, rather than a
`FluxInstance` that cannot work. `"gotk"` mode accepts both empty.

To pull the Flux controller images from a private registry, set `ImagePullSecret` to the name of
the pull Secret. Both modes use it: `"gotk"` mode adds it to every controller `Deployment`, and
`"flux-operator"` mode sets it as the `FluxInstance`'s distribution `imagePullSecret`. It is for
the controller images only, not a credential for the sync source. In `"flux-operator"` mode
neither `Registry` nor `ImagePullSecret` reaches the flux-operator `Deployment` in the vendored
install bundle: its image is pulled as upstream ships it, from its upstream registry with no pull
secret.

`Prune` sets garbage collection on the bootstrap Kustomization in both modes (go-kure/kure#1088).
In `"flux-operator"` mode it becomes a `spec.kustomize` patch on the sync Kustomization the
operator creates, since the sync has no field for it. Unset keeps each mode's default: `prune:
false` in `"gotk"` mode, and the operator's `prune: true` in `"flux-operator"` mode. So in
`"flux-operator"` mode, set `Prune: false` to keep objects that leave the source.

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

<!-- doc-example: pkg/stack/fluxcd ExampleWorkflowEngine_GenerateBootstrap -->
```go
engine := fluxcd.Engine()
rootNode := &stack.Node{Name: "prod"}

bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxMode:    "flux-operator",  // or "gotk"; empty defaults to "flux-operator"
    FluxVersion: "v2.8.2",         // required in flux-operator mode
    Registry:    "ghcr.io/fluxcd", // required in flux-operator mode
    SourceURL:   "oci://registry.example.com/fleet",
    SourceRef:   "latest",
}

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode, layout.LayoutRules{})
if err != nil {
    panic(err)
}
for _, obj := range objects {
    if obj.GetObjectKind().GroupVersionKind().Kind == "FluxInstance" {
        fmt.Println(obj.GetNamespace(), obj.GetName())
    }
}
```
<!-- doc-example:end -->

`SourceRef` names the revision to sync, and the same value works in both modes: an OCI tag
(empty means `latest`) or a Git branch name. In `"flux-operator"` mode Kure turns a branch name
into the full reference the operator's `GitRepository` needs (`main` becomes `refs/heads/main`).
A value that already starts with `refs/` — a tag such as `refs/tags/v1.0.0` — is passed through
unchanged in `"flux-operator"` mode only; `"gotk"` mode always treats a Git `SourceRef` as a branch.

Without a `ClusterName` the root node's name is the path the bootstrap applies, so it is checked
as a directory name wherever a path is built from it: always in `"gotk"` mode, and in
`"flux-operator"` mode when `SourceURL` is set (without one the `FluxInstance` has no sync and the
name is not used). A name holding `/`, `\` or a NUL byte, or `.` or `..`, is refused; no root node
and an unnamed root are valid. Under a `ClusterName` the path is the cluster directory and the
bootstrap does not check the name as a directory name; the walk still does, as a named root node
is the directory `<ClusterName>/<root>` of the written tree. In `"gotk"` mode a named root also names the generated source and
the bootstrap Kustomization's `sourceRef`, so there it must be a DNS-1123 subdomain as well:
`Prod` and `prod_root` are refused. With `SyncName` and `SourceURL` both set, `SyncName` names
those objects instead: `SyncName` must then be a valid Flux Kustomization name (a DNS-1123
subdomain of at most 63 characters), and the root name is checked only as a directory name.

## Bootstrap namespace

The bootstrap namespace is not part of `BootstrapConfig` — it lives on the generator. The engine
holds its own generator and `engine.GenerateBootstrap` delegates to that one, so configure it
through the engine rather than building a second generator the call never reads:

<!-- doc-example: pkg/stack/fluxcd Example_fluxWorkflowBootstrapNamespace -->
```go
engine := fluxcd.Engine()
rootNode := &stack.Node{Name: "production"}
bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxVersion: "v2.8.2",
    Registry:    "ghcr.io/fluxcd",
}

engine.GetBootstrapGenerator().DefaultNamespace = "custom-flux" // default: "flux-system"

objects, err := engine.GenerateBootstrap(bootstrapConfig, rootNode, layout.LayoutRules{})
if err != nil {
    panic(err)
}
for _, obj := range objects {
    switch obj.GetObjectKind().GroupVersionKind().Kind {
    case "FluxInstance":
        fmt.Println("FluxInstance in", obj.GetNamespace())
    case "Namespace":
        fmt.Println("Namespace", obj.GetName())
    }
}
```
<!-- doc-example:end -->

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

## Sync name

`BootstrapConfig.SyncName` names the bootstrap source and Kustomization in both modes. In
`"flux-operator"` mode the `FluxInstance` sync makes the operator create a source and a
Kustomization of its own, and `SyncName` becomes `spec.sync.name`. In `"gotk"` mode it names the
source and Kustomization Kure emits, and the Kustomization's `sourceRef` follows. It needs a
`SourceURL`, without which neither mode emits a source. Left empty, each mode keeps its own
names: the operator names both after the `FluxInstance`'s namespace — `flux-system` unless you
moved it as above — and `"gotk"` names the source after the root node and the Kustomization
`flux-system` (`BootstrapGenerator.BootstrapName`). Set `SyncName` when the Kustomizations you
generate reference the sync source under a name of your choosing:

<!-- doc-example: pkg/stack/fluxcd ExampleBootstrapGenerator_GenerateFluxInstance -->
```go
engine := fluxcd.Engine()
rootNode := &stack.Node{Name: "prod"}

bootstrapConfig := &stack.BootstrapConfig{
    Enabled:     true,
    FluxVersion: "v2.8.2",
    Registry:    "ghcr.io/fluxcd",
    SourceURL:   "oci://registry.example.com/fleet",
    SourceRef:   "latest",
    SyncName:    "fleet",
}

fi, err := engine.GetBootstrapGenerator().GenerateFluxInstance(bootstrapConfig, rootNode, layout.LayoutRules{})
if err != nil {
    panic(err)
}
fmt.Println(fi.Name, fi.Spec.Sync.Name, fi.Spec.Sync.Ref)
```
<!-- doc-example:end -->

It only takes effect with `SourceURL` set, since neither mode emits a source without one. In
`"gotk"` mode kure checks it as a Flux Kustomization name and refuses one that is not valid. In
`"flux-operator"` mode kure does not validate the value. The CRD caps it at 63 characters and makes
it immutable once set, so renaming the sync of a running cluster means recreating the
`FluxInstance`. It is distinct from the
`FluxInstance`'s own `metadata.name`, which is always `flux` (`fluxcd.FluxInstanceName`): the
flux-operator CRD rejects any other name at admission. The generator's
`BootstrapName` names only the `"gotk"` bootstrap Kustomization, and only while `SyncName` is
unset; it never reaches the `FluxInstance`.

## Source access

A sync source in a private or plain-HTTP OCI registry needs three more `BootstrapConfig`
options. `SourceSecretRef` names a `kubernetes.io/dockerconfigjson` Secret with the registry
credentials. `SourceProvider` picks the OIDC provider: `generic`, `aws`, `azure` or `gcp`.
`SourceInsecure` allows plain HTTP. They work in both modes:

- In `"gotk"` mode they land on the generated `OCIRepository`.
- In `"flux-operator"` mode the secret and provider land on `spec.sync`. Plain HTTP becomes a
  `spec.kustomize` patch on the `OCIRepository` the operator creates for the sync, since the sync
  has no field for it.

They apply to an `OCIRepository` source with `SourceURL` set. Anything else is refused: a
`GitRepository` source, no `SourceURL`, or a provider an `OCIRepository` does not take.
`ImagePullSecret` is a different secret: it is for pulling the Flux controller images.

To make the source verify the artifact's signature before Flux applies it, set `SourceVerify`:
a provider (`cosign`, the default, or `notation`), the Secret with the trusted public keys, and,
for keyless cosign, the issuer and subject patterns the signing certificate must match. In
`"gotk"` mode it is the generated `OCIRepository`'s `spec.verify`. In `"flux-operator"` mode it is
a `spec.kustomize` patch adding `spec.verify` to the operator's sync source, as for plain HTTP. It
takes the same `OCIRepository` source with `SourceURL`. An unknown provider, keyless cosign (no
public-key Secret) without an identity match, which would accept every signer, or an identity
pattern that is empty, is not a Go regular expression, or holds DEL, a C1 control such as NEL or
the noncharacter U+FFFE or U+FFFF literally, is refused. kure writes such a pattern as given, but
Flux's build, which writes every object with kustomize's `AsYaml`, folds NEL into a space and
fails on the others; write `\x{85}` instead.

To follow a release line instead of a fixed tag, set `SourceSemVer` to a semver range and leave
`SourceRef` empty:

<!-- doc-example:excerpt setting the field on a config; the behaviour is pinned by pkg/stack/fluxcd/bootstrap_source_semver_test.go -->
```go
config.SourceSemVer = ">=1.0.0 <2.0.0"
```

In `"gotk"` mode the range is the generated `OCIRepository`'s `spec.ref.semver`. In
`"flux-operator"` mode it is a `spec.kustomize` patch replacing `spec.ref` on the operator's sync
source, because `spec.sync.ref` takes only a tag. Both pull the highest tag inside the range. A
range with a `SourceRef`, with a `GitRepository` source, without `SourceURL`, or one that does not
parse is refused.

`BootstrapConfig.Labels` puts your own labels on the bootstrap source and Kustomization in both
modes: `metadata.labels` in `"gotk"` mode, a `spec.kustomize` patch on the operator's sync source
and Kustomization in `"flux-operator"` mode. Like `SyncName` it has no effect without `SourceURL`.
The keys a Flux controller stamps on these objects are refused in both modes, because the
controller would overwrite them: `app.kubernetes.io/managed-by`, `instance`, `version` and
`part-of`, `fluxcd.controlplane.io/name` and `namespace`, and `kustomize.toolkit.fluxcd.io/name`
and `namespace`. The [Flux Engine reference](/api-reference/flux-engine) lists which controller
stamps each.
