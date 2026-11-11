# Bootstrap Generation

Generate Flux system bootstrap manifests. Two modes are supported:

| Mode | Description |
|------|-------------|
| `"flux-operator"` | **Default.** Emits a full Flux Operator install bundle (CRDs, Deployment, RBAC). Recommended for new clusters. |
| `"gotk"` | Legacy mode. Emits the GitOps Toolkit component manifests directly. |

When `FluxMode` is empty, it defaults to `"flux-operator"`.

In `"flux-operator"` mode `FluxVersion` and `Registry` are both required. They go verbatim into
the `FluxInstance`'s `spec.distribution`, and a `FluxInstance` with an empty version or registry
cannot work, so `GenerateBootstrap` and `GenerateFluxInstance` return a validation error naming
each missing field instead of emitting one, and no part of the bundle is returned with it. There
is no default for either: the caller supplies both, which is why the [defaults table](/api-reference/flux-defaults/) has no
row for them. `"gotk"` mode reads the same two fields and keeps accepting them empty: there an
empty `FluxVersion` means the vendored release and an empty `Registry` the upstream registry.

`ImagePullSecret` names the Secret the Flux controller images are pulled with, and both modes
carry it: `"gotk"` mode passes it to the install options, which add it to every controller
`Deployment`, and `"flux-operator"` mode sets it as the `FluxInstance`'s
`spec.distribution.imagePullSecret`. Empty sets none. It covers the controller images only; it
is not a credential for the sync source.

Neither `Registry` nor `ImagePullSecret` reaches the flux-operator `Deployment` in the vendored
install bundle: the bundle is emitted as upstream ships it, so the operator image is pulled from
its upstream registry with no pull secret.

The `"flux-operator"` bundle is vendored from the upstream flux-operator release and pinned
in lockstep with the `github.com/controlplaneio-fluxcd/flux-operator` Go module
(`FluxOperatorVersion`, currently **v0.62.1**). Renovate re-vendors the bundle and updates
the constant and this version when it bumps the module (`scripts/sync-flux-operator-pin.sh`);
see `flux_operator_install.go` for the manual refresh procedure.

The `"gotk"` components are built from the flux2 release's install manifests base, vendored
at the module root (`internal/gotk/manifests.tar.gz`, shared with the tests that read the
toolkit's CustomResourceDefinitions out of it) in lockstep with the
`github.com/fluxcd/flux2/v2` Go module
(`GotkVersion`, currently **v2.10.0**), so gotk generation makes no network call and the same
input always produces the same manifests. That happens when `FluxVersion` is empty or names
`GotkVersion` (with or without the leading `v`). Any other `FluxVersion`, `"latest"` included,
is an explicit opt-in to the upstream behaviour: a manifests base is downloaded from GitHub at
generation time — the named release for a `vX.Y.Z` value, the latest release for anything else
(upstream selects a release only for a `v`-prefixed version). `TestVendoredPinsMatchGoMod` fails when
`GotkVersion` or `FluxOperatorVersion` differs from its `go.mod` require, or when a controller
image in the gotk bundle differs from the matching `fluxcd/<controller>/api` require (a controller
whose API module kure does not require is not compared); see
`internal/gotk` for the refresh procedure after a flux2 bump.

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

## The directory the bootstrap applies

`GenerateBootstrap` and `GenerateFluxInstance` take the layout rules the tree is written with, and
both modes point Flux at the top directory of that tree (go-kure/kure#979): the directory
`layout.TopDirectory` returns for the root node and the rules, which is where `WalkCluster` takes
its own top from, so the two cannot differ. Each mode spells it the way its field requires:

| Root node | Rules | `"gotk"`: bootstrap Kustomization `spec.path` | `"flux-operator"`: `FluxInstance` `spec.sync.path` |
|---|---|---|---|
| named `prod` | no `ClusterName` | `prod` | `./prod` |
| unnamed | no `ClusterName` | `cluster` | `./cluster` |
| named, unnamed or none passed | `ClusterName` `clusters/prod` | `clusters/prod` | `./clusters/prod` |
| none passed | no `ClusterName` | `.` | `./` |

`spec.path` is spelled like every other Kustomization path this package writes (see
[Kustomization paths](/api-reference/flux-names-and-paths/#kustomization-paths)): no leading `./`, and `.` for the root of the source.
`spec.sync.path` is `DefaultSyncPath` followed by the directory. Flux treats `x` and `./x` alike.
`"gotk"` mode used to write `manifests/<root>`, a prefix none of the layout writers produces; a
consumer that depends on such a prefix sets `spec.path` on the returned Kustomization.

With no root node there is no tree to walk: without a `ClusterName` the bootstrap applies the root
of the source, and the caller writes there. The source's name and the bootstrap Kustomization's
`sourceRef` stay on the root node's name whatever the rules are (see
[Root node name](#root-node-name)).

The rules are validated first, as a walk validates them, so invalid rules are an error whatever
the `BootstrapConfig` is, a nil or disabled one included. `WorkflowEngine.GenerateBootstrap`
takes them as `stack.LayoutRulesProvider` and refuses anything that is not a `layout.LayoutRules`
value, nil included, as `GenerateFromCluster` does; it does not fall back to the defaults.

The [OCI artifact layout design](https://github.com/go-kure/.github/blob/main/docs/design/oci-layout.md)
shows the bootstrap Kustomization's path as the `flux-system` directory. kure's bootstrap applies the top directory of the written tree, whose
`kustomization.yaml` lists `flux-system` under the Separate placement, so the objects that
directory holds are applied either way.

Two limits. The directory is relative to the directory the tree is written into: a tree written
below a sub-path of the repository (the base directory a writer is given is the caller's) is at
that sub-path joined with it, and the caller sets `spec.path` or `spec.sync.path` accordingly.
And it describes a `WalkCluster` tree only: `WalkClusterByPackage` writes one tree per package,
placed without the `ClusterName`, and the bootstrap does not point at those.

**Breaking change (go-kure/kure#979).** `GenerateBootstrap` (on `BootstrapGenerator`,
`WorkflowEngine`, the ArgoCD engine and the `stack.Workflow` interface) and
`GenerateFluxInstance` take the layout rules as a third argument. The directory used to be the
root node's name whatever the rules were, so two outputs move:

- with a `ClusterName`, both paths move from the root node's name (`.` and `./` for an unnamed or
  absent one) to the cluster directory;
- an unnamed root node without a `ClusterName` moves from the root of the source (`.`, `./`) to
  `cluster` (`./cluster`), where the walk writes it.

A named root node without a `ClusterName` keeps its path. Under a `ClusterName` the bootstrap no
longer checks the root node's name as a directory name: its path is the cluster directory and
takes no segment from the name. The name still becomes a directory of the walked tree
(`<ClusterName>/<root>`), where the walk's cluster validation checks it. A cluster bootstrapped
with the old path keeps applying the old directory until the regenerated bootstrap objects are
applied.

One refusal is no longer met (go-kure/kure#979). Below a `ClusterName` wrapper under
`FluxIntegratedPerLayout`, a layout Kustomization of the root node's directory that you keep in
place of the generated one was refused when a patch or postBuild of it changed a Source hosted
there, because the bootstrap applied that directory too. The bootstrap now applies the wrapper,
which lists the Kustomization, so it is accepted (see
[Layout Integration](/api-reference/flux-layout-integration/#layout-integration)).

## Sync name

`BootstrapConfig.SyncName` names the bootstrap source and Kustomization in both modes, so one
config gives the root one identity whichever mode renders it. In `flux-operator` mode it becomes
the `FluxInstance`'s `spec.sync.name`: the name flux-operator gives the source and Kustomization
it creates for the sync. In `gotk` mode it names the `OCIRepository` or `GitRepository` and the
bootstrap Kustomization kure emits, and the Kustomization's `sourceRef` follows; it takes
precedence over the root node's name and `BootstrapGenerator.BootstrapName`. It has no effect
without `SourceURL`, in either mode, because no source is emitted then.

When it is empty each mode keeps its own names. The operator names both after the
`FluxInstance`'s namespace (`DefaultNamespace`); that fallback is the operator's, not kure's,
which is why the [defaults table](/api-reference/flux-defaults/) has no row for it. `gotk` names the source after the root
node (`DefaultSourceName` without one) and the Kustomization `BootstrapName`. Set `SyncName` when
the Kustomizations you generate reference the sync source by a name of your choosing; otherwise
their `sourceRef` has to follow whichever name the mode picks.

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

- It needs `SourceURL`: without one neither mode emits a source, so there is nothing to name.
- In `gotk` mode it must be a valid Flux Kustomization name (`stack.ValidateKustomizationName`:
  a DNS-1123 subdomain of at most 63 characters), else generation is refused. In `flux-operator`
  mode kure passes it through unvalidated. The CRD caps it at 63 characters and makes it immutable
  once set, so renaming the sync of a live cluster means recreating the `FluxInstance`. In `gotk`
  mode a new name means new objects: the source and Kustomization under the old name are not
  renamed.
- It is not the `FluxInstance`'s own `metadata.name`, which is always `FluxInstanceName` (`flux`):
  the CRD accepts no other. `BootstrapGenerator.BootstrapName` names only the `gotk` bootstrap
  Kustomization, and only while `SyncName` is unset.

## Source access

Three `BootstrapConfig` options say how an `OCIRepository` sync source reaches a private or
plain-HTTP registry (go-kure/kure#1060), and a fourth, `SourceVerify`, makes it verify the
artifact's signature (go-kure/kure#1091). Unset, the output is what it was before they existed.

| Option | gotk `OCIRepository` | flux-operator `FluxInstance` |
|---|---|---|
| `SourceSecretRef` | `spec.secretRef.name` | `spec.sync.pullSecret` |
| `SourceProvider` | `spec.provider` | `spec.sync.provider` |
| `SourceInsecure` | `spec.insecure: true` | a `spec.kustomize` patch |
| `SourceVerify` | `spec.verify` | a `spec.kustomize` patch |

`SourceSecretRef` names a `kubernetes.io/dockerconfigjson` Secret with the registry credentials.
It is not `ImagePullSecret`, which is for the Flux controller images. `SourceProvider` is one of
`generic`, `aws`, `azure` and `gcp`, the values both CRDs take for an `OCIRepository`.

A `FluxInstance` sync has no insecure field. So in flux-operator mode `SourceInsecure` adds one
`spec.kustomize` patch that sets `spec.insecure: true` on the `OCIRepository` the operator
generates for the sync, matched by group, kind and name. The name is `SyncName`, or, when that is
empty, the `FluxInstance`'s namespace, which is how the operator names that source.

`SourceVerify` is a `stack.SourceVerification`, source-controller's `OCIRepositoryVerification`
in kure's own type: a `Provider`, `cosign` or `notation`; a `SecretRef` naming the Secret with
the trusted public keys; `MatchOIDCIdentity`, the issuer and subject patterns a keyless cosign
signature's certificate must match; and a `TrustedRootSecretRef` naming the Secret with a
self-hosted Sigstore `trusted_root.json`. An empty `Provider` is written as `cosign`, the CRD's
default. A sync has no verify field either, so in flux-operator mode it is a second patch on the
same source, after the insecure one, adding `spec.verify`; both modes then verify the same way.

Both `GenerateBootstrap` and `GenerateFluxInstance` refuse, with nothing returned:
- any of the four without `SourceURL`, where no source is generated for it to apply to;
- any of the four with a `GitRepository` source, which they are not defined for;
- a `SourceProvider` an `OCIRepository` does not take, `github` included;
- a `SourceVerify` provider other than `cosign` and `notation`;
- keyless cosign (`cosign` or no provider, without `SecretRef`) with no identity match, a
  `TrustedRootSecretRef` or not: cosign then checks no identity and accepts every signer;
- an identity match whose issuer or subject is empty, which would match any signer, or is not a
  Go regular expression, which source-controller would reject only at reconcile;
- an issuer or subject holding DEL, a C1 control (U+0080 to U+009F, NEL among them) or the
  noncharacter U+FFFE or U+FFFF. kure writes such a pattern as given, but Flux's build does not
  keep it: flux-operator's build, which applies the verify patch, and kustomize-controller's,
  which applies the gotk sync source where it is written into the tree the bootstrap applies,
  write every object with kustomize's `AsYaml`, which folds NEL into a space and fails on the
  others. The pattern can match one with a regular-expression escape, such as `\x{85}`.

Apart from keyless cosign needing an identity match, the verification's cross-field rules (which
provider takes which fields) are source-controller's and are not checked here, and the Secrets it
names are the caller's to create.

## Semver source ref

`BootstrapConfig.SourceSemVer` selects the `OCIRepository` sync source's tag by a semver range,
such as `>=1.0.0 <2.0.0`, instead of the fixed tag `SourceRef` names (go-kure/kure#1090). Unset,
the output is what it was before it existed.

| Mode | Where the range goes |
|---|---|
| gotk | the `OCIRepository`'s `spec.ref`, as `semver` and nothing else |
| flux-operator | a `spec.kustomize` patch replacing `spec.ref` on the sync's `OCIRepository` |

A `FluxInstance` sync's `ref` is a tag only, and the operator writes it into the source unquoted,
where a range such as `>=1.0.0` or `*` would be YAML syntax. So `spec.sync.ref` stays
`DefaultSourceRef`, which the field requires, and the patch replaces the whole `spec.ref` with
`semver: <range>`. It is matched as the `SourceInsecure` patch is, by group, kind and name, and the
two sit side by side when both are set. Either way the source resolves to the same revision in
both modes: the highest tag inside the range.

Both `GenerateBootstrap` and `GenerateFluxInstance` refuse, with nothing returned:
- `SourceSemVer` without `SourceURL`, where no source is generated for it to apply to;
- `SourceSemVer` with a `GitRepository` source;
- `SourceSemVer` with a `SourceRef`, `latest` included: the range and an explicit tag would both
  choose the revision;
- a `SourceSemVer` that does not parse as a range, checked with the library source-controller
  parses it with, so it is refused at generation rather than at reconcile.

## Prune

`BootstrapConfig.Prune` sets garbage collection on the bootstrap Kustomization in both modes
(go-kure/kure#1088). Unset keeps each mode's default.

| `Prune` | gotk Kustomization | flux-operator sync Kustomization |
|---|---|---|
| unset | `prune: false` | `prune: true`, the operator's |
| `false` | `prune: false` | `prune: false`, by patch |
| `true` | `prune: true` | `prune: true`, by patch |

The operator writes `prune: true` into the sync Kustomization it generates, and a `FluxInstance`
sync has no prune field. So in flux-operator mode a set `Prune` adds one `spec.kustomize` patch
that sets `spec.prune` on that Kustomization, matched by group, kind and name, the name chosen as
for the `SourceInsecure` patch. Without `SourceURL` no sync is generated and `Prune` adds nothing.

## Labels

`BootstrapConfig.Labels` sets labels on the bootstrap source and Kustomization in both modes
(go-kure/kure#1180). Unset or empty, the output is what it was before it existed.

| Mode | Source (`OCIRepository` or `GitRepository`) | Kustomization |
|---|---|---|
| gotk | `metadata.labels` | `metadata.labels` |
| flux-operator | a `spec.kustomize` patch | a `spec.kustomize` patch |

A `FluxInstance` sync has no labels field, and the operator's sync template writes no labels. So
in flux-operator mode the labels add two `spec.kustomize` patches, after any other, each adding
the whole map as `metadata.labels`: one on the sync source, of either kind, and one on the sync
Kustomization, matched by group, kind and name as the `SourceInsecure` patch is. The map is
copied, never shared with the config, and its values stay strings as written (`"true"`, `""`).

Like `SyncName`, the labels have no effect without `SourceURL`: no source is emitted then, in
either mode, and the `gotk` Kustomization emitted without one carries no labels.

Flux controllers then stamp labels of their own on these objects, and a value `Labels` gave one
of those keys would be overwritten:

| Keys | Stamped by |
|---|---|
| `app.kubernetes.io/managed-by`, `app.kubernetes.io/instance`, `app.kubernetes.io/version`, `app.kubernetes.io/part-of` | flux-operator's `LabelTransformer`, on every object it builds |
| `fluxcd.controlplane.io/name`, `fluxcd.controlplane.io/namespace` | flux-operator's owner labels, on every object it applies |
| `kustomize.toolkit.fluxcd.io/name`, `kustomize.toolkit.fluxcd.io/namespace` | kustomize-controller's owner labels, on every object a Kustomization applies |

flux-operator's builder lists its `LabelTransformer` under `transformers:`, where the
`FluxInstance` patches go under `patches:` (flux-operator v0.62.1 `internal/builder/templates.go`),
and kustomize runs the transformers after the patches, so the sync objects carry both sets. Both
controllers set their owner labels after any build, on what they apply, and use them to find
stale objects to garbage-collect. kustomize-controller's reach the `gotk` source and
Kustomization when you write them into the tree the bootstrap Kustomization applies, the usual
self-managed Flux bootstrap; kure returns the objects and does not decide where they go.

Both `GenerateBootstrap` and `GenerateFluxInstance` refuse, with nothing returned, in both modes
and with or without `SourceURL`:
- any of those eight keys, exactly as written (other keys under the same prefixes are accepted),
  so one config means the same in either mode;
- a key that is not a qualified name, and a value that is not a label value, as the API server
  checks them.

With several, the error is about the first key in sorted order.

## Root node name

Without a `ClusterName` the root node's name is the gotk bootstrap Kustomization's `spec.path`
and the directory in the `FluxInstance`'s `spec.sync.path`. `GenerateBootstrap` and
`GenerateFluxInstance` take a node, not a cluster, so they check that name themselves with
`stack.ValidateDirectoryName`: a name holding `/`, `\` or a NUL byte, or `.` or `..`, is refused.
No root node and an unnamed root are valid; neither is a path segment. Nor is any root node under
rules with a `ClusterName`, where the path is the cluster directory: the bootstrap does not check
the name as a directory name there. The walk does, in every case: under a `ClusterName` a named
root node is the directory `<ClusterName>/<root>` of the written tree.

The check runs only where a path is built. In `gotk` mode the bootstrap Kustomization always has
a `spec.path`, so the name is always checked. A `FluxInstance` gets a `spec.sync` only when
`BootstrapConfig.SourceURL` is set: without one, flux-operator mode and `GenerateFluxInstance` do
not use the root name and do not check it.

In `gotk` mode a named root is also an object name: the generated `GitRepository` or
`OCIRepository` and the bootstrap Kustomization's `sourceRef` carry it. There the name must be a
DNS-1123 subdomain as well, with or without a `SourceURL`; `Prod` and `prod_root` are refused.
The exception is `SyncName` set with a `SourceURL`: `SyncName` then names those objects and is
the name checked, as a Flux Kustomization name (`stack.ValidateKustomizationName`: a DNS-1123
subdomain of at most 63 characters), and the root name is checked only as a directory name, as
in `flux-operator` mode.
