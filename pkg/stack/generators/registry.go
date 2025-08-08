package generators

import (
	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/stack"
)

// Re-export GVK type for backward compatibility
type GVK = gvk.GVK

// Re-export common functions for backward compatibility
var (
	ParseAPIVersion = gvk.ParseAPIVersion
)

// ApplicationConfigFactory is a function that creates a new ApplicationConfig instance
type ApplicationConfigFactory = gvk.Factory[stack.ApplicationConfig]

// globalRegistry is the singleton registry instance for ApplicationConfig
var globalRegistry = gvk.NewRegistry[stack.ApplicationConfig]()

// Register adds a new ApplicationConfig type to the global registry
func Register(gvk GVK, factory ApplicationConfigFactory) {
	globalRegistry.Register(gvk, factory)
}

// Create creates a new ApplicationConfig instance for the given apiVersion and kind
func Create(apiVersion, kind string) (stack.ApplicationConfig, error) {
	parsed := gvk.ParseAPIVersion(apiVersion, kind)
	return globalRegistry.Create(parsed)
}

// CreateFromGVK creates a new ApplicationConfig instance for the given GVK
func CreateFromGVK(gvkObj GVK) (stack.ApplicationConfig, error) {
	return globalRegistry.Create(gvkObj)
}

// ListKinds returns all registered GVKs from the global registry
func ListKinds() []GVK {
	return globalRegistry.ListGVKs()
}

// HasKind checks if a GVK is registered in the global registry
func HasKind(apiVersion, kind string) bool {
	return globalRegistry.HasAPIVersion(apiVersion, kind)
}