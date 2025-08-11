// Package fluxhelm provides generators for creating Flux HelmRelease resources
// along with their associated source resources (HelmRepository, GitRepository,
// OCIRepository, or Bucket).
//
// The FluxHelm generator follows the GVK (Group, Version, Kind) pattern:
//   - Group: generators.gokure.dev
//   - Version: v1alpha1
//   - Kind: FluxHelm
//
// This generator supports multiple source types:
//   - HelmRepository: Traditional Helm chart repositories
//   - GitRepository: Charts stored in Git repositories
//   - OCIRepository: OCI-compliant container registries
//   - Bucket: S3-compatible object storage
//
// Example usage with HelmRepository:
//
//	apiVersion: generators.gokure.dev/v1alpha1
//	kind: FluxHelm
//	metadata:
//	  name: postgresql
//	  namespace: database
//	spec:
//	  chart:
//	    name: postgresql
//	    version: 12.0.0
//	  source:
//	    type: HelmRepository
//	    url: https://charts.bitnami.com/bitnami
//	  values:
//	    auth:
//	      database: myapp
//	  release:
//	    createNamespace: true
//
// Example usage with OCIRepository:
//
//	apiVersion: generators.gokure.dev/v1alpha1
//	kind: FluxHelm
//	metadata:
//	  name: podinfo
//	  namespace: apps
//	spec:
//	  chart:
//	    name: podinfo
//	    version: "6.*"
//	  source:
//	    type: OCIRepository
//	    ociUrl: oci://ghcr.io/stefanprodan/charts/podinfo
//	  values:
//	    replicaCount: 2
package fluxhelm
