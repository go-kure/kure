package gvk

import (
	"reflect"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/errors"
)

// TypedWrapper provides type detection and unmarshaling for GVK-based types
type TypedWrapper[T any] struct {
	APIVersion string         `yaml:"apiVersion" json:"apiVersion"`
	Kind       string         `yaml:"kind" json:"kind"`
	Metadata   map[string]any `yaml:"metadata" json:"metadata"`
	Spec       T              `yaml:"-" json:"-"`
	registry   *Registry[T]
}

// NewTypedWrapper creates a new typed wrapper with the given registry
func NewTypedWrapper[T any](registry *Registry[T]) *TypedWrapper[T] {
	return &TypedWrapper[T]{
		registry: registry,
		Metadata: make(map[string]any),
	}
}

// UnmarshalYAML implements custom YAML unmarshaling with type detection
func (w *TypedWrapper[T]) UnmarshalYAML(node *yaml.Node) error {
	if w.registry == nil {
		return errors.Errorf("registry not set - use NewTypedWrapper to create instances")
	}

	// First pass: extract GVK
	var gvkDetect struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}
	if err := node.Decode(&gvkDetect); err != nil {
		return errors.Errorf("failed to detect GVK: %w", err)
	}

	if gvkDetect.APIVersion == "" || gvkDetect.Kind == "" {
		return errors.Errorf("apiVersion and kind are required fields")
	}

	// Create appropriate instance
	instance, err := w.registry.CreateFromAPIVersion(gvkDetect.APIVersion, gvkDetect.Kind)
	if err != nil {
		return errors.Errorf("failed to create instance for %s/%s: %w",
			gvkDetect.APIVersion, gvkDetect.Kind, err)
	}

	// Decode full content
	var raw struct {
		APIVersion string         `yaml:"apiVersion"`
		Kind       string         `yaml:"kind"`
		Metadata   map[string]any `yaml:"metadata"`
		Spec       yaml.Node      `yaml:"spec"`
	}

	if err := node.Decode(&raw); err != nil {
		return errors.Errorf("failed to decode wrapper: %w", err)
	}

	// Decode spec into the specific type
	if err := raw.Spec.Decode(&instance); err != nil {
		return errors.Errorf("failed to decode spec for %s/%s: %w",
			raw.APIVersion, raw.Kind, err)
	}

	w.APIVersion = raw.APIVersion
	w.Kind = raw.Kind
	w.Metadata = raw.Metadata
	w.Spec = instance

	// Apply metadata to the instance if it supports it
	if w.Metadata != nil {
		if name, ok := w.Metadata["name"].(string); ok {
			if named, ok := any(&instance).(NamedType); ok {
				named.SetName(name)
			}
		}
		if namespace, ok := w.Metadata["namespace"].(string); ok {
			if namespaced, ok := any(&instance).(NamespacedType); ok {
				namespaced.SetNamespace(namespace)
			}
		}
	}

	return nil
}

// MarshalYAML implements custom YAML marshaling
func (w *TypedWrapper[T]) MarshalYAML() (interface{}, error) {
	// Create a map representation for clean YAML output
	result := map[string]interface{}{
		"apiVersion": w.APIVersion,
		"kind":       w.Kind,
	}

	if len(w.Metadata) > 0 {
		result["metadata"] = w.Metadata
	}

	if !isZero(w.Spec) {
		result["spec"] = w.Spec
	}

	return result, nil
}

// GetGVK returns the GVK for this wrapper
func (w *TypedWrapper[T]) GetGVK() GVK {
	return ParseAPIVersion(w.APIVersion, w.Kind)
}

// GetName returns the name from metadata if available
func (w *TypedWrapper[T]) GetName() string {
	if name, ok := w.Metadata["name"].(string); ok {
		return name
	}
	return ""
}

// GetNamespace returns the namespace from metadata if available
func (w *TypedWrapper[T]) GetNamespace() string {
	if namespace, ok := w.Metadata["namespace"].(string); ok {
		return namespace
	}
	return ""
}

// SetName sets the name in metadata
func (w *TypedWrapper[T]) SetName(name string) {
	if w.Metadata == nil {
		w.Metadata = make(map[string]any)
	}
	w.Metadata["name"] = name
}

// SetNamespace sets the namespace in metadata
func (w *TypedWrapper[T]) SetNamespace(namespace string) {
	if w.Metadata == nil {
		w.Metadata = make(map[string]any)
	}
	w.Metadata["namespace"] = namespace
}

// isZero checks if a value is the zero value for its type.
func isZero[T any](v T) bool {
	var zero T
	return reflect.DeepEqual(v, zero)
}

// TypedWrappers is a slice of TypedWrapper for unmarshaling multiple configs
type TypedWrappers[T any] []TypedWrapper[T]

// UnmarshalYAML implements custom YAML unmarshaling for slices
func (ws *TypedWrappers[T]) UnmarshalYAML(node *yaml.Node) error {
	// This requires the registry to be set somehow -
	// typically this would be handled by a containing type
	// that knows about the registry
	return errors.Errorf("TypedWrappers requires registry context - use a container type")
}
