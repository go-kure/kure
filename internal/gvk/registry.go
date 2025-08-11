package gvk

import (
	"fmt"
	"sync"
)

// Factory is a function that creates a new instance of type T
type Factory[T any] func() T

// Registry manages type factories for GVK-based types
type Registry[T any] struct {
	factories map[GVK]Factory[T]
	mu        sync.RWMutex
}

// NewRegistry creates a new registry for type T
func NewRegistry[T any]() *Registry[T] {
	return &Registry[T]{
		factories: make(map[GVK]Factory[T]),
	}
}

// Register adds a new type factory to the registry
func (r *Registry[T]) Register(gvk GVK, factory Factory[T]) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.factories[gvk] = factory
}

// Create creates a new instance for the given GVK
func (r *Registry[T]) Create(gvk GVK) (T, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	factory, exists := r.factories[gvk]
	if !exists {
		var zero T
		return zero, fmt.Errorf("unknown type: %s", gvk)
	}
	return factory(), nil
}

// CreateFromAPIVersion creates a new instance for the given apiVersion and kind
func (r *Registry[T]) CreateFromAPIVersion(apiVersion, kind string) (T, error) {
	gvk := ParseAPIVersion(apiVersion, kind)
	return r.Create(gvk)
}

// ListGVKs returns all registered GVKs
func (r *Registry[T]) ListGVKs() []GVK {
	r.mu.RLock()
	defer r.mu.RUnlock()

	gvks := make([]GVK, 0, len(r.factories))
	for gvk := range r.factories {
		gvks = append(gvks, gvk)
	}
	return gvks
}

// HasGVK checks if a GVK is registered
func (r *Registry[T]) HasGVK(gvk GVK) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, exists := r.factories[gvk]
	return exists
}

// HasAPIVersion checks if an apiVersion and kind combination is registered
func (r *Registry[T]) HasAPIVersion(apiVersion, kind string) bool {
	gvk := ParseAPIVersion(apiVersion, kind)
	return r.HasGVK(gvk)
}

// Count returns the number of registered types
func (r *Registry[T]) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.factories)
}
