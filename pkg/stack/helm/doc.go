// Package helm provides client-side Helm chart rendering from OCI registries
// and HTTP Helm repositories.
//
// RenderChart pulls a chart and renders its templates using the Helm template
// engine — equivalent to `helm template` — returning multi-document YAML.
// No Kubernetes cluster connection is required. RenderOption (WithReleaseName,
// WithNamespace) overrides the default release identity ("release" in
// "default"). WithDigest pins an OCI chart to a manifest digest: the render
// is refused when the pulled manifest has another one, unless it is an image
// index listing an image manifest with that digest, which is then pulled on
// its own and rendered. WithDigestVersion
// renders an OCI chart at the version Flux's helm-controller installs it as,
// the chart version with the pulled manifest digest as build metadata.
//
// SplitByHookWeight groups rendered objects into HookGroup values by their
// helm.sh/hook phase and helm.sh/hook-weight, in deployment order, and drops
// the phases that have no Flux lifecycle equivalent. A missing or non-integer
// weight counts as 0. Kure does not turn a HookGroup into a Flux Kustomization;
// the caller does.
package helm
