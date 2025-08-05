// Package patch provides declarative patching of Kubernetes resources using
// a simple, structured syntax without templates or overlays.
//
// This package enables tools to modify Kubernetes manifests through patches
// that target specific fields and list items using dot-notation paths with
// smart selectors.
//
// # Quick Start
//
// Load resources and apply patches:
//
//	// Load base Kubernetes resources
//	resources, err := patch.LoadResourcesFromMultiYAML(resourceFile)
//	if err != nil {
//		return err
//	}
//
//	// Load patch specifications
//	patches, err := patch.LoadPatchFile(patchFile)
//	if err != nil {
//		return err
//	}
//
//	// Create patchable set and apply
//	set, err := patch.NewPatchableAppSet(resources, patches)
//	if err != nil {
//		return err
//	}
//
//	resolved, err := set.Resolve()
//	if err != nil {
//		return err
//	}
//
//	for _, r := range resolved {
//		if err := r.Apply(); err != nil {
//			return err
//		}
//	}
//
// # Patch Syntax
//
// Both YAML and TOML patch formats are supported with automatic detection:
//
//	# YAML format
//	spec.replicas: 3
//	spec.containers[name=main].image: nginx:latest
//	spec.ports[+name=https]: {name: https, port: 443}
//
//	# TOML format  
//	[deployment.app]
//	spec.replicas: 3
//
//	[deployment.app.containers.name=main]
//	image: nginx:latest
//	resources.requests.cpu: 100m
//
// # Core Types
//
//	PatchableAppSet - Manages resources and their patches
//	PatchOp         - Individual patch operation
//	PathPart        - Structured path component
//	TOMLHeader      - Parsed TOML section header
//
// # Detailed Documentation
//
// For comprehensive information, see the markdown documentation:
//
//   - DESIGN.md - Complete syntax reference and examples
//   - PATCH_ENGINE_DESIGN.md - Architecture and implementation details
//   - PATH_RESOLUTION.md - Advanced path resolution and type inference
//   - ERROR_HANDLING.md - Error handling patterns and debugging
//
// # Debugging
//
// Enable detailed logging:
//
//	export KURE_DEBUG=1
package patch
