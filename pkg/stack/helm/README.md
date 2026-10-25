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

`WithDigest` pins an OCI chart to a manifest digest. `RenderChart` still pulls `chartURL:version`,
then compares the digest of the manifest it received with the pinned one, and returns an error
naming both when they differ, before rendering anything. A tag re-pushed with other content is
refused instead of rendered. The digest must be `sha256:` followed by 64 lowercase hex characters;
any other value, the empty string included, is refused before the pull. HTTP repositories have no
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
// WithDigest pins an OCI chart to a manifest digest.
func RenderChart(chartURL, version string, values map[string]any, opts ...RenderOption) ([]byte, error)

// RenderOption customizes how a chart is rendered: the release identity, or
// the digest an OCI chart must match.
type RenderOption func(*renderOptions)

// WithReleaseName sets the release name used during rendering.
func WithReleaseName(name string) RenderOption

// WithNamespace sets the release namespace used during rendering.
func WithNamespace(namespace string) RenderOption

// WithDigest pins an OCI chart to a manifest digest ("sha256:" + 64 lowercase
// hex). A pulled manifest with another digest is an error; HTTP repositories
// refuse the option.
func WithDigest(digest string) RenderOption
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
- Comma-separated hook annotations (e.g. `"pre-install,post-install"`) are
  treated as a single opaque phase and placed in the unknown group.
