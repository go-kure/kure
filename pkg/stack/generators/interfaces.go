package generators

import "github.com/go-kure/kure/internal/gvk"

// Re-export interfaces from internal/gvk for backward compatibility
type VersionedType = gvk.VersionedType
type NamedConfig = gvk.NamedType
type NamespacedConfig = gvk.NamespacedType
type MetadataConfig = gvk.MetadataType
type Convertible = gvk.Convertible

// BaseMetadata provides common metadata fields for generators
type BaseMetadata = gvk.BaseMetadata