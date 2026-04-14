# pkg/stack/helm

Client-side Helm chart rendering from OCI registries.

## Overview

This package provides `RenderChart`, which pulls a Helm chart from an OCI
registry and renders its templates locally — equivalent to `helm template` —
returning multi-document YAML. No Kubernetes cluster connection is required.

## Usage

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

## API

```go
// RenderChart pulls a Helm chart from an OCI registry and renders it
// client-side, returning multi-doc YAML.
//
// chartURL is an OCI URL of the form oci://registry/repo/chart.
// version is the chart version tag (e.g. "1.16.5").
// values are merged on top of the chart's default values.
func RenderChart(chartURL, version string, values map[string]any) ([]byte, error)
```

## Notes

- Authentication uses the local Docker credential store (`~/.docker/config.json`)
  if credentials are needed for the registry.
- The rendered output excludes Helm partial templates (files starting with `_`)
  and any templates that produce empty output.
- The `.Release.Name` is set to `"release"` and `.Release.Namespace` to
  `"default"`. Charts that rely on these values will receive those defaults.
