// Package helm provides client-side Helm chart rendering from OCI registries.
//
// RenderChart pulls a chart from an OCI registry and renders its templates
// using the Helm template engine — equivalent to `helm template` — returning
// multi-document YAML. No Kubernetes cluster connection is required.
package helm
