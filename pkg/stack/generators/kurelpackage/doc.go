// Package kurelpackage provides a generator for creating kurel packages from
// Kubernetes manifests with support for values substitution, patches, and
// conditional extensions.
//
// # Overview
//
// The KurelPackage generator (v1alpha1) transforms a collection of Kubernetes
// manifests into a reusable kurel package. This enables:
//
//   - Parameterization via CEL expressions and .Values references
//   - Conditional resource inclusion via Extensions
//   - Patch application at build time
//   - Package dependencies and version management
//
// # Configuration
//
// A kurel package is defined using [ConfigV1Alpha1]:
//
//	apiVersion: generators.gokure.dev/v1alpha1
//	kind: KurelPackage
//	package:
//	  name: my-app
//	  version: 1.0.0
//	resources:
//	  - source: ./manifests
//	    includes:
//	      - "*.yaml"
//	values:
//	  schema:
//	    replicas:
//	      type: integer
//	      default: 1
//	patches:
//	  - target:
//	      kind: Deployment
//	      name: my-app
//	    patch: |
//	      spec:
//	        replicas: {{ .Values.replicas }}
//
// # Values and CEL Expressions
//
// Values can be referenced in patches and extensions using CEL expressions:
//
//	# Simple value reference
//	image: {{ .Values.image.repository }}:{{ .Values.image.tag }}
//
//	# CEL expression with default
//	replicas: {{ .Values.replicas || 3 }}
//
//	# CEL conditional
//	{{ if .Values.autoscaling.enabled }}
//	enabled: true
//	{{ end }}
//
// # Extensions
//
// Extensions enable conditional resource inclusion:
//
//	extensions:
//	  - name: ingress
//	    condition: .Values.ingress.enabled
//	    resources:
//	      - source: ./ingress.yaml
//
// When the condition evaluates to true, the extension's resources are included.
//
// # Resource Sources
//
// Resources can be loaded from directories or files with glob patterns:
//
//	resources:
//	  - source: ./base           # Directory
//	    includes: ["*.yaml"]     # Glob patterns
//	    excludes: ["*test*"]     # Exclusion patterns
//	    recurse: true            # Recurse into subdirectories
//
// # Generate
//
// The [ConfigV1Alpha1.Generate] method collects Kubernetes resource files from
// the package structure produced by [ConfigV1Alpha1.GeneratePackageFiles] and
// returns them as []*client.Object, making KurelPackage configs usable in the
// stack generation pipeline alongside other ApplicationConfig implementations.
//
// # GVK Registration
//
// The generator is automatically registered during package initialization:
//
//	Group:   generators.gokure.dev
//	Version: v1alpha1
//	Kind:    KurelPackage
//
// # Usage with kurel CLI
//
// Build a package using the kurel command:
//
//	kurel build -f package.yaml -o ./dist
//
// This generates the package directory structure with processed manifests.
package kurelpackage
