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

Each block below is the body of an `Example` function in `example_test.go`, which `go test` runs:
it imports this package as `gvk`, `gopkg.in/yaml.v3` (the YAML library whose `yaml.Node` the
wrapper decodes from), and `fmt` for the line that prints what the example built. The same file
declares the example spec types `MyConfigType` (one `Replicas int` field), and `OldType` and
`NewType`, whose one field is `Name` and `DisplayName` respectively.

### Type Registration

<!-- doc-example: pkg/gvk ExampleRegistry_Create -->
```go
myGVK := gvk.GVK{
    Group:   "generators.gokure.dev",
    Version: "v1alpha1",
    Kind:    "MyGenerator",
}

registry := gvk.NewRegistry[MyConfigType]()
registry.Register(myGVK, func() MyConfigType {
    return MyConfigType{Replicas: 1}
})

instance, err := registry.Create(myGVK)
if err != nil {
    panic(err)
}
fmt.Println(myGVK.APIVersion(), instance.Replicas)
```
<!-- doc-example:end -->

### YAML Unmarshaling with Type Detection

<!-- doc-example: pkg/gvk ExampleNewTypedWrapper -->
```go
registry := gvk.NewRegistry[MyConfigType]()
registry.Register(gvk.GVK{Group: "generators.gokure.dev", Version: "v1alpha1", Kind: "MyGenerator"},
    func() MyConfigType { return MyConfigType{} })

data := []byte(`
apiVersion: generators.gokure.dev/v1alpha1
kind: MyGenerator
metadata:
  name: web
spec:
  replicas: 3
`)

wrapper := gvk.NewTypedWrapper(registry)
if err := yaml.Unmarshal(data, wrapper); err != nil {
    panic(err)
}
// wrapper.Spec contains the correctly typed instance
fmt.Println(wrapper.GetName(), wrapper.Spec.Replicas)
```
<!-- doc-example:end -->

### Version Conversion

A converter receives and returns `any`: `RegisterFunc` takes a `ConversionFunc`, and `Register`
takes any `Converter`.

<!-- doc-example: pkg/gvk ExampleConversionRegistry_RegisterFunc -->
```go
oldGVK := gvk.GVK{Group: "generators.gokure.dev", Version: "v1alpha1", Kind: "MyGenerator"}
newGVK := gvk.GVK{Group: "generators.gokure.dev", Version: "v1beta1", Kind: "MyGenerator"}

conversions := gvk.NewConversionRegistry()
conversions.RegisterFunc(oldGVK, newGVK, func(from any) (any, error) {
    old := from.(OldType)
    return NewType{DisplayName: old.Name}, nil
})

converted, err := conversions.Convert(oldGVK, newGVK, OldType{Name: "web"})
if err != nil {
    panic(err)
}
fmt.Printf("%+v\n", converted)
```
<!-- doc-example:end -->

## Key Interfaces

- `VersionedType` - Types that carry GVK metadata
- `NamedType` - Types with a name field
- `NamespacedType` - Types with a namespace field
- `MetadataType` - Types with full metadata (name + namespace)
- `Convertible` - Types that support version conversion
