package gvk

import (
	"fmt"
	"strings"
)

// GVK represents a Group, Version, Kind tuple for identifying types
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

// VersionedType is a type that supports GVK identification
type VersionedType interface {
	GetAPIVersion() string // Returns "group/version"
	GetKind() string
}

// NamedType is a type that has a name
type NamedType interface {
	GetName() string
	SetName(string)
}

// NamespacedType is a type that has a namespace
type NamespacedType interface {
	GetNamespace() string
	SetNamespace(string)
}

// MetadataType combines name and namespace interfaces
type MetadataType interface {
	NamedType
	NamespacedType
}

// Convertible allows for version migration between different versions of the same kind
type Convertible interface {
	ConvertTo(version string) (interface{}, error)
	ConvertFrom(from interface{}) error
}

// BaseMetadata provides common metadata fields for GVK types
type BaseMetadata struct {
	Name      string `yaml:"name" json:"name"`
	Namespace string `yaml:"namespace,omitempty" json:"namespace,omitempty"`
}

// GetName returns the name
func (m *BaseMetadata) GetName() string {
	return m.Name
}

// SetName sets the name
func (m *BaseMetadata) SetName(name string) {
	m.Name = name
}

// GetNamespace returns the namespace
func (m *BaseMetadata) GetNamespace() string {
	return m.Namespace
}

// SetNamespace sets the namespace
func (m *BaseMetadata) SetNamespace(namespace string) {
	m.Namespace = namespace
}
