# pkg/stack/helm

Client-side Helm chart rendering and hook-phase splitting for GitOps deployment.

## Overview

This package provides two utilities:

- **`RenderChart`** — pulls a Helm chart from an OCI registry or HTTP Helm repository and renders its
  templates locally (equivalent to `helm template`), returning multi-document YAML.
- **`SplitByHookWeight`** — groups rendered Helm manifests by hook phase and weight
  for ordered FluxCD Kustomization generation.

No Kubernetes cluster connection is required.

Each Go block on this page, except the two marked as excerpts, is the body of an `Example`
function in `example_test.go`: it imports this package as `helm`,
`github.com/go-kure/kure/pkg/io`, and `fmt`. `go test` runs the `SplitByHookWeight` example and
checks its output; it compiles the `RenderChart` examples without running them, because they
pull a chart from a registry.

## RenderChart

**OCI registry:**

<!-- doc-example: pkg/stack/helm ExampleRenderChart -->
```go
manifests, err := helm.RenderChart(
    "oci://registry.example.com/charts/cilium", // OCI chart URL
    "1.16.5", // chart version
    map[string]any{ // value overrides (merged on top of chart defaults)
        "kubeProxyReplacement": true,
        "ipam": map[string]any{
            "mode": "kubernetes",
        },
    },
)
if err != nil {
    panic(err)
}
// manifests is multi-doc YAML suitable for kubectl apply -f -
fmt.Print(string(manifests))
```
<!-- doc-example:end -->

**HTTP/HTTPS repository (public only):**

<!-- doc-example: pkg/stack/helm ExampleRenderChart_http -->
```go
manifests, err := helm.RenderChart(
    "https://charts.bitnami.com/bitnami/redis", // repo base URL + chart name
    "19.0.0",
    map[string]any{"replicaCount": 3},
)
if err != nil {
    panic(err)
}
fmt.Print(string(manifests))
```
<!-- doc-example:end -->

The chart name is the last path segment; the rest is the repository base URL.
HTTP repositories must be publicly accessible — basic auth, client TLS, and
other credential mechanisms are not supported.

**Release identity (optional):**

By default rendering uses release name `"release"` in namespace `"default"`.
Pass `RenderOption`s to override either:

<!-- doc-example: pkg/stack/helm ExampleWithReleaseName -->
```go
values := map[string]any{"kubeProxyReplacement": true}

manifests, err := helm.RenderChart(
    "oci://registry.example.com/charts/cilium",
    "1.16.5",
    values,
    helm.WithReleaseName("my-cilium"),
    helm.WithNamespace("kube-system"),
)
if err != nil {
    panic(err)
}
fmt.Print(string(manifests))
```
<!-- doc-example:end -->

**Digest pinning (OCI only, optional):**

`WithDigest` pins an OCI chart to a manifest digest: `RenderChart` renders the manifest with that
digest and nothing else. It still pulls `chartURL:version`, then compares the digest of the
manifest it received with the pinned one. When they differ and the received manifest is an OCI
image index listing an image manifest with the pinned digest, `RenderChart` pulls that child on its
own, by digest, and renders it: this is how one chart is rendered out of a multi-chart index. The
tag then only finds the index; the content is fixed by the digest. Any other difference is an error
naming both digests, before rendering anything, so a tag re-pushed with other content is refused
instead of rendered. The digest must be `sha256:` followed by 64 lowercase hex characters; any
other value, the empty string included, is refused before the pull. HTTP repositories have no
manifest digest, so `RenderChart` refuses `WithDigest` for them.

<!-- doc-example: pkg/stack/helm ExampleWithDigest -->
```go
manifests, err := helm.RenderChart(
    "oci://registry.example.com/charts/cilium",
    "1.16.5",
    nil,
    // the manifest digest 1.16.5 is pinned to; any other is an error
    helm.WithDigest("sha256:3f1c5a9e0b7d2468ace13579bdf02468ace13579bdf02468ace13579bdf02468"),
)
if err != nil {
    panic(err)
}
fmt.Print(string(manifests))
```
<!-- doc-example:end -->

**Digest version (OCI only, optional):**

Flux's helm-controller installs an OCI chart from an `OCIRepository` source at a version that
carries the source's digest: the chart's semver version with its build metadata replaced by the
first 12 hex characters of the manifest digest (`1.16.5` becomes `1.16.5+3f1c5a9e0b7d`).
`WithDigestVersion` renders at that same version, computed from the digest of the manifest the
pull received, so every template that prints `.Chart.Version`, such as the conventional
`helm.sh/chart` label, matches the release helm-controller makes. Combined with `WithDigest`, the
digest is checked first. As helm-controller refuses to install them, a chart version that is not
semver, a semver tag (with `_` read as `+`, the OCI tag form) naming another version than the
chart's, build metadata aside, and a chart that is not a Helm v2 API chart (`apiVersion: v1` or
`v2`) are refused; a tag that is not semver is not compared. HTTP repositories have no manifest
digest, so `RenderChart` refuses `WithDigestVersion` for them. A root that reaches more than one
manifest is refused before any chart is loaded (see Notes), so the version is not computed for a
chart picked among several. A child `WithDigest` selects from an image index is versioned from its
own digest, and the tag check applies to that child's chart version. The single child of an index
naming one image manifest, rendered without `WithDigest` selecting it, is versioned from the index
digest.

<!-- doc-example: pkg/stack/helm ExampleWithDigestVersion -->
```go
manifests, err := helm.RenderChart(
    "oci://registry.example.com/charts/cilium",
    "1.16.5",
    nil,
    // .Chart.Version renders as 1.16.5+<first 12 hex of the pulled digest>,
    // the version Flux's helm-controller installs the chart at
    helm.WithDigestVersion(),
)
if err != nil {
    panic(err)
}
fmt.Print(string(manifests))
```
<!-- doc-example:end -->

## API

<!-- doc-example:excerpt the declarations from render.go, not calls -->
```go
// RenderChart pulls a Helm chart and renders it client-side, returning multi-doc YAML.
//
// OCI registries: chartURL must start with "oci://". Authentication uses the
// local Docker credential store (~/.docker/config.json).
//
// HTTP repositories: chartURL must start with "http://" or "https://", with the
// chart name as the last path segment (e.g. "https://charts.example.com/myapp").
// Only public unauthenticated repositories are supported.
//
// version is the chart version tag (e.g. "1.16.5").
// values are merged on top of the chart's default values.
// opts customizes the release identity; see WithReleaseName and WithNamespace.
// WithDigest pins an OCI chart to a manifest digest; WithDigestVersion renders
// it at its digest-qualified version.
// An OCI image index is rendered only when it names exactly one image
// manifest, or when WithDigest names one of the image manifests it lists;
// either child is pulled and rendered on its own. Any other index, a root
// naming a subject or naming a manifest as its config or a layer, and a child
// doing either or holding an index, are refused (see Notes).
func RenderChart(chartURL, version string, values map[string]any, opts ...RenderOption) ([]byte, error)

// RenderOption customizes how a chart is rendered: the release identity, the
// digest an OCI chart must match, or the version an OCI chart renders at.
type RenderOption func(*renderOptions)

// WithReleaseName sets the release name used during rendering.
func WithReleaseName(name string) RenderOption

// WithNamespace sets the release namespace used during rendering.
func WithNamespace(namespace string) RenderOption

// WithDigest pins an OCI chart to a manifest digest ("sha256:" + 64 lowercase
// hex). A pulled image index listing an image manifest with that digest has
// that child pulled on its own and rendered; any other manifest with another
// digest is an error; HTTP repositories refuse the option.
func WithDigest(digest string) RenderOption

// WithDigestVersion renders an OCI chart at the version Flux's helm-controller
// installs it as: the chart's semver version with its build metadata replaced
// by the first 12 hex characters of the pulled manifest digest. Non-semver
// versions, a semver tag naming another version, non-v2 charts and HTTP
// repositories are refused. A root that reaches more than one manifest is
// refused before any chart is loaded; a child WithDigest selects is versioned
// from its own digest, the single child of a one-manifest index from the
// index digest.
func WithDigestVersion() RenderOption
```

## SplitByHookWeight

Groups a slice of rendered Helm objects by `helm.sh/hook` phase and
`helm.sh/hook-weight`, in deployment order. That is all it does: nothing in kure turns a
`HookGroup` into a Flux Kustomization, a bundle or a layout. A consumer that wants one
Kustomization per group, applied in that order, builds them from the groups itself.

`io.ParseYAML` turns the multi-doc YAML `RenderChart` returns into the `[]client.Object` this
takes. Here the `test` hook is excluded, as the table below says:

<!-- doc-example: pkg/stack/helm ExampleSplitByHookWeight -->
```go
// Multi-doc YAML as RenderChart returns it
rendered := []byte(`apiVersion: batch/v1
kind: Job
metadata:
  name: db-migrate
  annotations:
    helm.sh/hook: pre-install
    helm.sh/hook-weight: "-5"
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: app-config
---
apiVersion: v1
kind: Pod
metadata:
  name: app-test
  annotations:
    helm.sh/hook: test
`)
parsed, err := io.ParseYAML(rendered) // []client.Object
if err != nil {
    panic(err)
}

groups := helm.SplitByHookWeight(parsed)
for _, g := range groups {
    // groups come in deployment order; turning each into its own FluxCD
    // Kustomization is the caller's job: kure does not convert a HookGroup
    fmt.Printf("phase=%q weight=%d resources=%d\n", g.Phase, g.Weight, len(g.Resources))
}
```
<!-- doc-example:end -->

**Phase ordering policy:**

| Phase | Order | Notes |
|---|---|---|
| `pre-install` | 0 | before main resources, on install |
| `pre-upgrade` | 1 | before main resources, on upgrade |
| `""` (non-hook) | 2 | main resources |
| `post-install` | 3 | after main resources, on install |
| `post-upgrade` | 4 | after main resources, on upgrade |
| unknown | 5+ alphabetical | unrecognised phases, included conservatively |
| `pre-delete`, `post-delete`, `pre-rollback`, `post-rollback`, `test` | — | **excluded**: no FluxCD lifecycle equivalent |

Comma-separated annotations (e.g. `"pre-install,post-install"`) are treated as
a single opaque phase string and placed in the unknown group to avoid SSA
ownership conflicts between multiple Kustomizations.

<!-- doc-example:excerpt the declarations from hooks.go, not calls -->
```go
// HookGroup is a set of Helm manifests sharing the same hook phase and weight.
type HookGroup struct {
    Phase     string
    Weight    int
    Resources []client.Object
}

func SplitByHookWeight(objects []client.Object) []HookGroup
```

## Notes

- OCI authentication uses the local Docker credential store (`~/.docker/config.json`).
- The rendered output excludes Helm partial templates (files starting with `_`)
  and any templates that produce empty output.
- The `.Release.Name` defaults to `"release"` and `.Release.Namespace` to
  `"default"`; override either with `WithReleaseName`/`WithNamespace`.
- Without `WithDigest`, an OCI chart is rendered from whatever manifest the tag points to when it
  is pulled.
- Without `WithDigestVersion`, `.Chart.Version` is the version in the chart's `Chart.yaml`, which
  differs from the version helm-controller installs an OCI chart at.
- An OCI reference that resolves to an image index is rendered only when the index names exactly
  one image manifest, whether or not the index carries its optional `mediaType` field, or when
  `WithDigest` selects one of its children (below). Otherwise a root that can reach more than one
  manifest is refused with an error naming `chartURL:version`, before any chart is loaded, with or
  without `WithDigestVersion`, and with `WithDigest` pinned to the root's own digest:
  - an index naming more than one manifest;
  - an index holding a nested index, whatever that one holds;
  - an index holding one chart manifest beside a manifest that is no chart;
  - a root manifest naming a `subject`, whether it is an index or a chart manifest;
  - a root manifest naming a manifest as its config or a layer, whatever else its JSON holds;
  - an index listing its single image manifest without a digest.

  The rule is conservative on purpose. Helm's registry client pulls every manifest under the root,
  a subject included, and every manifest a config or layer descriptor names, and keeps whichever
  chart it meets last, in no fixed order, so such a root could render a different chart on each
  pull. The index lists its children by media type only, without their content, so kure cannot
  tell a chart child from any other. Reference a chart pushed as a single manifest, not as an
  index, or select one child with `WithDigest`.
- An image index naming a single image manifest renders that child pulled on its own, by the
  digest the index lists, so the chart rendered is the one checked: the child is refused, naming
  `chartURL:version`, when its content is itself an index, names a `subject` or names a manifest as
  its config or a layer. This costs one more pull: the tag's pull has already fetched the child and
  its chart archive, and the child's own pull fetches them again. That pull fails for a child
  listed with a digest other than `sha256` when the registry sends the child with a known length and no
  `Docker-Content-Digest`: Helm's registry client then computes a `sha256` digest of the child and
  refuses it against the listed one, so such an index fails with that error instead of rendering.
  `WithDigestVersion` versions the
  chart from the index digest the tag resolved to, as the reference pins the index.
- `WithDigest` with the digest of an image manifest an image index lists renders that child of a
  multi-chart index: the child is chosen from the index's own manifest list and pulled on its own,
  by digest, so the chart Helm kept from the tag's pull is never the one rendered. The selection is
  refused, naming `chartURL:version`, for a digest the index does not list, one it lists as
  anything but an image manifest (a nested index, for one), a selected child whose content is
  itself an index, names a `subject` or names a manifest as its config or a layer, and an index
  root naming a `subject`. The tag's pull still copies every child under the index, so an index
  whose other children cannot be pulled fails before the selection.
- With `WithDigestVersion`, a semver tag must equal the selected child's chart version, build
  metadata aside, so a child of another version is selected through a tag that is not semver.
  This follows from the tag check above; it is not a separate rule.
- Comma-separated hook annotations (e.g. `"pre-install,post-install"`) are
  treated as a single opaque phase and placed in the unknown group.
