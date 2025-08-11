package stack

import (
	"fmt"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/internal/gvk"
)

// applicationConfigRegistry is the stack package's registry for ApplicationConfig types
var applicationConfigRegistry = gvk.NewRegistry[ApplicationConfig]()

// RegisterApplicationConfig registers an ApplicationConfig type with the stack registry
func RegisterApplicationConfig(gvk gvk.GVK, factory func() ApplicationConfig) {
	applicationConfigRegistry.Register(gvk, factory)
}

// CreateApplicationConfig creates a new ApplicationConfig instance for the given apiVersion and kind
func CreateApplicationConfig(apiVersion, kind string) (ApplicationConfig, error) {
	parsed := gvk.ParseAPIVersion(apiVersion, kind)
	return applicationConfigRegistry.Create(parsed)
}

// ApplicationWrapper provides type detection and unmarshaling for ApplicationConfig
type ApplicationWrapper struct {
	APIVersion string              `yaml:"apiVersion" json:"apiVersion"`
	Kind       string              `yaml:"kind" json:"kind"`
	Metadata   ApplicationMetadata `yaml:"metadata" json:"metadata"`
	Spec       ApplicationConfig   `yaml:"-" json:"-"`
}

// ApplicationMetadata contains common metadata fields
type ApplicationMetadata struct {
	Name      string            `yaml:"name" json:"name"`
	Namespace string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling with type detection
func (w *ApplicationWrapper) UnmarshalYAML(node *yaml.Node) error {
	// First pass: extract GVK and metadata
	var gvkDetect struct {
		APIVersion string              `yaml:"apiVersion"`
		Kind       string              `yaml:"kind"`
		Metadata   ApplicationMetadata `yaml:"metadata"`
	}
	if err := node.Decode(&gvkDetect); err != nil {
		return fmt.Errorf("failed to detect GVK: %w", err)
	}

	if gvkDetect.APIVersion == "" || gvkDetect.Kind == "" {
		return fmt.Errorf("apiVersion and kind are required fields")
	}

	// Create appropriate config instance
	config, err := CreateApplicationConfig(gvkDetect.APIVersion, gvkDetect.Kind)
	if err != nil {
		return fmt.Errorf("failed to create config for %s/%s: %w",
			gvkDetect.APIVersion, gvkDetect.Kind, err)
	}

	// Decode full content with spec field
	var raw struct {
		APIVersion string              `yaml:"apiVersion"`
		Kind       string              `yaml:"kind"`
		Metadata   ApplicationMetadata `yaml:"metadata"`
		Spec       yaml.Node           `yaml:"spec"`
	}

	if err := node.Decode(&raw); err != nil {
		return fmt.Errorf("failed to decode wrapper: %w", err)
	}

	// Decode spec into the specific config type
	if err := raw.Spec.Decode(config); err != nil {
		return fmt.Errorf("failed to decode spec for %s/%s: %w",
			raw.APIVersion, raw.Kind, err)
	}

	w.APIVersion = raw.APIVersion
	w.Kind = raw.Kind
	w.Metadata = raw.Metadata
	w.Spec = config

	// Apply metadata to the config if it supports it
	if named, ok := config.(gvk.NamedType); ok {
		named.SetName(w.Metadata.Name)
	}
	if namespaced, ok := config.(gvk.NamespacedType); ok {
		namespaced.SetNamespace(w.Metadata.Namespace)
	}

	return nil
}

// MarshalYAML implements custom YAML marshaling
func (w *ApplicationWrapper) MarshalYAML() (interface{}, error) {
	// Create a map representation for clean YAML output
	result := map[string]interface{}{
		"apiVersion": w.APIVersion,
		"kind":       w.Kind,
		"metadata":   w.Metadata,
	}

	if w.Spec != nil {
		result["spec"] = w.Spec
	}

	return result, nil
}

// ToApplication converts the wrapper to a stack.Application
func (w *ApplicationWrapper) ToApplication() *Application {
	return NewApplication(w.Metadata.Name, w.Metadata.Namespace, w.Spec)
}

// ApplicationWrappers is a slice of ApplicationWrapper for unmarshaling multiple configs
type ApplicationWrappers []ApplicationWrapper

// ToApplications converts all wrappers to Applications
func (ws ApplicationWrappers) ToApplications() []*Application {
	apps := make([]*Application, 0, len(ws))
	for _, w := range ws {
		apps = append(apps, w.ToApplication())
	}
	return apps
}
