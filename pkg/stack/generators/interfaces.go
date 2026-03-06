package generators

import "github.com/go-kure/kure/pkg/gvk"

// Deprecated: Use the types from pkg/gvk directly instead.
// These aliases are kept for backward compatibility.
type VersionedType = gvk.VersionedType
type NamedConfig = gvk.NamedType
type NamespacedConfig = gvk.NamespacedType
type MetadataConfig = gvk.MetadataType
type Convertible = gvk.Convertible

// BaseMetadata provides common metadata fields for generators.
type BaseMetadata = gvk.BaseMetadata
