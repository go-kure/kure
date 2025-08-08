// Package generators provides a pluggable system for generating Kubernetes resources
// from different configuration formats. It implements a registry pattern with
// Group, Version, Kind (GVK) identification following Kubernetes conventions.
//
// Architecture
//
// The package uses a registry pattern where each generator type:
//   - Is identified by a GVK (Group, Version, Kind)
//   - Implements the ApplicationConfig interface
//   - Is organized in its own subpackage
//   - Supports multiple versions (v1alpha1, v1beta1, v1, etc.)
//
// Generator Organization
//
// Each generator type is organized as:
//
//	generators/
//	├── <type>/                 # Generator type package
//	│   ├── v1alpha1.go        # Version implementation
//	│   ├── v1beta1.go         # Future version
//	│   ├── internal/          # Internal implementation
//	│   │   └── <type>.go      # Core logic
//	│   └── doc.go             # Package documentation
//
// Available Generators
//
//   - appworkload: Standard Kubernetes workloads (Deployments, StatefulSets, DaemonSets)
//   - fluxhelm: Flux HelmRelease with various source types
//
// Registration
//
// Generators self-register during package initialization:
//
//	func init() {
//	    generators.Register(generators.GVK{
//	        Group:   "generators.gokure.dev",
//	        Version: "v1alpha1",
//	        Kind:    "MyGenerator",
//	    }, func() interface{} {
//	        return &MyGeneratorV1Alpha1{}
//	    })
//	}
//
// Usage
//
// Generators are typically used through the ApplicationWrapper:
//
//	var wrapper stack.ApplicationWrapper
//	err := yaml.Unmarshal(yamlData, &wrapper)
//	if err != nil {
//	    // handle error
//	}
//	
//	app := wrapper.ToApplication()
//	resources, err := app.Config.Generate(app)
//
// Creating New Generators
//
// To create a new generator:
//
// 1. Create a new package under generators/<type>
// 2. Implement the ApplicationConfig interface
// 3. Add version files (v1alpha1.go, etc.)
// 4. Register in init() function
// 5. Add documentation
//
// Version Evolution
//
// Generators support version evolution:
//   - v1alpha1: Initial implementation, API may change
//   - v1beta1: API stabilizing, backward compatibility within beta
//   - v1: Stable API, backward compatibility guaranteed
//
// When adding new versions, implement conversion interfaces to support
// migration between versions.
package generators