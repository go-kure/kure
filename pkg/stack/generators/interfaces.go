package generators

// VersionedConfig is an ApplicationConfig that supports GVK
type VersionedConfig interface {
	GetAPIVersion() string // Returns "group/version"
	GetKind() string
}

// NamedConfig is an ApplicationConfig that has a name
type NamedConfig interface {
	GetName() string
	SetName(string)
}

// NamespacedConfig is an ApplicationConfig that has a namespace
type NamespacedConfig interface {
	GetNamespace() string
	SetNamespace(string)
}

// MetadataConfig combines name and namespace interfaces
type MetadataConfig interface {
	NamedConfig
	NamespacedConfig
}

// Convertible allows for version migration
type Convertible interface {
	ConvertTo(version string) (interface{}, error)
	ConvertFrom(from interface{}) error
}

// BaseMetadata provides common metadata fields for generators
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