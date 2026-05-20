# pkg/stack/helm

Client-side Helm chart rendering and hook-phase splitting for GitOps deployment.

## Overview

This package provides two utilities:

- **`RenderChart`** — pulls a Helm chart from an OCI registry or HTTP Helm repository and renders its
  templates locally (equivalent to `helm template`), returning multi-document YAML.
- **`SplitByHookWeight`** — groups rendered Helm manifests by hook phase and weight
  for ordered FluxCD Kustomization generation.

No Kubernetes cluster connection is required.

## RenderChart

**OCI registry:**

```go
import "github.com/go-kure/kure/pkg/stack/helm"

manifests, err := helm.RenderChart(
    "oci://registry.wharf.zone/charts/cilium", // OCI chart URL
    "1.16.5",                                   // chart version
    map[string]any{                             // value overrides (merged on top of chart defaults)
        "kubeProxyReplacement": true,
        "ipam": map[string]any{
            "mode": "kubernetes",
        },
    },
)
if err != nil {
    return fmt.Errorf("render cilium: %w", err)
}
// manifests is multi-doc YAML suitable for kubectl apply -f -
```

**HTTP/HTTPS repository:**

```go
manifests, err := helm.RenderChart(
    "https://charts.bitnami.com/bitnami/redis", // repo base URL + chart name
    "19.0.0",
    map[string]any{"replicaCount": 3},
)
```

The chart name is the last path segment; the rest is the repository base URL.

## API

```go
// RenderChart pulls a Helm chart and renders it client-side, returning multi-doc YAML.
//
// OCI registries: chartURL must start with "oci://".
// HTTP repositories: chartURL must start with "http://" or "https://", with the
// chart name as the last path segment (e.g. "https://charts.example.com/myapp").
// version is the chart version tag (e.g. "1.16.5").
// values are merged on top of the chart's default values.
func RenderChart(chartURL, version string, values map[string]any) ([]byte, error)
```

## SplitByHookWeight

Groups a slice of rendered Helm objects by `helm.sh/hook` phase and
`helm.sh/hook-weight` for ordered FluxCD Kustomization generation.

```go
objects, _ := helm.RenderChart("oci://example.com/chart", "1.0.0", nil)
parsed := parseYAML(objects) // []client.Object

groups := helm.SplitByHookWeight(parsed)
for _, g := range groups {
    // each group becomes one FluxCD Kustomization, deployed in order
    fmt.Printf("phase=%q weight=%d resources=%d\n", g.Phase, g.Weight, len(g.Resources))
}
```

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
- The `.Release.Name` is set to `"release"` and `.Release.Namespace` to
  `"default"`. Charts that rely on these values will receive those defaults.
- Comma-separated hook annotations (e.g. `"pre-install,post-install"`) are
  treated as a single opaque phase and placed in the unknown group.
