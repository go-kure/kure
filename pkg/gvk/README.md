# GVK - Group/Version/Kind Type System

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/gvk.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/gvk)

The `gvk` package provides shared infrastructure for Group, Version, Kind (GVK) based type systems within Kure. It implements version-aware type registration, unmarshaling, and conversion similar to Kubernetes' own GVK system but tailored for Kure's needs.

## Core Components

| Component | Description |
|-----------|-------------|
| `GVK` | Group, Version, Kind tuple that uniquely identifies a type |
| `Registry[T]` | Type-safe registration and factory pattern for GVK types |
| `TypedWrapper[T]` | Automatic type detection during YAML unmarshaling |
| `ConversionRegistry` | Version conversion system for type migrations |

## Usage

### Type Registration

```go
import "github.com/go-kure/kure/pkg/gvk"

registry := gvk.NewRegistry[MyConfigType]()
registry.Register(gvk.GVK{
    Group:   "generators.gokure.dev",
    Version: "v1alpha1",
    Kind:    "MyGenerator",
}, func() MyConfigType {
    return MyConfigType{}
})

instance, err := registry.Create(myGVK)
```

### YAML Unmarshaling with Type Detection

```go
wrapper := gvk.NewTypedWrapper(registry)
err := yaml.Unmarshal(data, wrapper)
// wrapper.Spec contains the correctly typed instance
```

### Version Conversion

```go
conversions := gvk.NewConversionRegistry()
conversions.RegisterConversion(oldGVK, newGVK, func(old OldType) (NewType, error) {
    return NewType{Field: old.Field}, nil
})
```

## Key Interfaces

- `VersionedType` - Types that carry GVK metadata
- `NamedType` - Types with a name field
- `NamespacedType` - Types with a namespace field
- `MetadataType` - Types with full metadata (name + namespace)
- `Convertible` - Types that support version conversion
