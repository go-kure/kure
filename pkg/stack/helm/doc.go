// Package helm provides client-side Helm chart rendering from OCI registries
// and HTTP Helm repositories.
//
// RenderChart pulls a chart and renders its templates using the Helm template
// engine — equivalent to `helm template` — returning multi-document YAML.
// No Kubernetes cluster connection is required.
package helm
