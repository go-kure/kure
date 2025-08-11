// Package gvk provides shared infrastructure for Group, Version, Kind (GVK)
// based type systems within Kure.
//
// This package implements the foundational components needed for version-aware
// type registration, unmarshaling, and conversion - similar to Kubernetes' own
// GVK system but tailored for Kure's specific needs.
//
// # Core Components
//
// GVK represents the Group, Version, Kind tuple that uniquely identifies a type:
//
//	gvk := GVK{
//	    Group:   "generators.gokure.dev",
//	    Version: "v1alpha1",
//	    Kind:    "AppWorkload",
//	}
//
// Registry provides type-safe registration and factory pattern for GVK types:
//
//	registry := NewRegistry[MyConfigType]()
//	registry.Register(gvk, func() MyConfigType {
//	    return MyConfigType{}
//	})
//
//	instance, err := registry.Create(gvk)
//
// TypedWrapper enables automatic type detection during YAML unmarshaling:
//
//	wrapper := NewTypedWrapper(registry)
//	err := yaml.Unmarshal(data, wrapper)
//	// wrapper.Spec contains the correctly typed instance
//
// # Key Features
//
// - Type-safe generics for compile-time checking
// - Automatic YAML unmarshaling with type detection
// - Version comparison and migration support
// - Extensible conversion system for version upgrades
// - Thread-safe registry operations
//
// # Usage Patterns
//
// The package is designed to be used by higher-level packages that need
// GVK-based type systems:
//
//	// In a generator package
//	type MyGenerator struct { ... }
//
//	var registry = gvk.NewRegistry[MyGenerator]()
//
//	func init() {
//	    registry.Register(gvk.GVK{
//	        Group: "example.gokure.dev",
//	        Version: "v1alpha1",
//	        Kind: "MyGenerator",
//	    }, func() MyGenerator { return MyGenerator{} })
//	}
//
//	// For YAML parsing
//	func ParseConfig(data []byte) (*MyGenerator, error) {
//	    wrapper := gvk.NewTypedWrapper(registry)
//	    err := yaml.Unmarshal(data, wrapper)
//	    if err != nil {
//	        return nil, err
//	    }
//	    return &wrapper.Spec, nil
//	}
//
// # Internal Package
//
// This package is internal to Kure and provides shared infrastructure
// for both the generators system and the future stack GVK system.
// It is not intended for external use and its API may change without notice.
package gvk
