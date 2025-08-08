package generators

import (
	"fmt"
	"strings"
	"sync"
)

// GVK represents a Group, Version, Kind tuple for identifying generator types
type GVK struct {
	Group   string
	Version string
	Kind    string
}

// String returns the string representation of GVK
func (g GVK) String() string {
	return fmt.Sprintf("%s/%s, Kind=%s", g.Group, g.Version, g.Kind)
}

// APIVersion returns the apiVersion string (group/version)
func (g GVK) APIVersion() string {
	if g.Group == "" {
		return g.Version
	}
	return fmt.Sprintf("%s/%s", g.Group, g.Version)
}

// ParseAPIVersion parses an apiVersion and kind into a GVK
func ParseAPIVersion(apiVersion, kind string) GVK {
	parts := strings.Split(apiVersion, "/")
	if len(parts) == 2 {
		return GVK{
			Group:   parts[0],
			Version: parts[1],
			Kind:    kind,
		}
	}
	// Handle core/v1 style or bare version
	return GVK{
		Group:   "",
		Version: parts[0],
		Kind:    kind,
	}
}

// ApplicationConfigFactory is a function that creates a new ApplicationConfig instance
type ApplicationConfigFactory func() interface{}

// Registry manages ApplicationConfig implementations
type Registry struct {
	factories map[GVK]ApplicationConfigFactory
	mu        sync.RWMutex
}

// globalRegistry is the singleton registry instance
var globalRegistry = &Registry{
	factories: make(map[GVK]ApplicationConfigFactory),
}

// Register adds a new ApplicationConfig type to the registry
func (r *Registry) Register(gvk GVK, factory ApplicationConfigFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[gvk] = factory
}

// Create creates a new ApplicationConfig instance for the given GVK
func (r *Registry) Create(gvk GVK) (interface{}, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[gvk]
	if !exists {
		return nil, fmt.Errorf("unknown application config type: %s", gvk)
	}
	return factory(), nil
}

// ListKinds returns all registered GVKs
func (r *Registry) ListKinds() []GVK {
	r.mu.RLock()
	defer r.mu.RUnlock()

	kinds := make([]GVK, 0, len(r.factories))
	for gvk := range r.factories {
		kinds = append(kinds, gvk)
	}
	return kinds
}

// HasKind checks if a GVK is registered
func (r *Registry) HasKind(gvk GVK) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[gvk]
	return exists
}

// Global registry functions

// Register adds a new ApplicationConfig type to the global registry
func Register(gvk GVK, factory ApplicationConfigFactory) {
	globalRegistry.Register(gvk, factory)
}

// Create creates a new ApplicationConfig instance for the given apiVersion and kind
func Create(apiVersion, kind string) (interface{}, error) {
	gvk := ParseAPIVersion(apiVersion, kind)
	return globalRegistry.Create(gvk)
}

// CreateFromGVK creates a new ApplicationConfig instance for the given GVK
func CreateFromGVK(gvk GVK) (interface{}, error) {
	return globalRegistry.Create(gvk)
}

// ListKinds returns all registered GVKs from the global registry
func ListKinds() []GVK {
	return globalRegistry.ListKinds()
}

// HasKind checks if a GVK is registered in the global registry
func HasKind(apiVersion, kind string) bool {
	gvk := ParseAPIVersion(apiVersion, kind)
	return globalRegistry.HasKind(gvk)
}