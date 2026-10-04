# IO - YAML Serialization and Resource Printing

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/io.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/io)

The `io` package provides utilities for parsing, serializing, and printing Kubernetes resources. It supports multiple output formats including YAML, JSON, and kubectl-compatible table views.

## Overview

This package handles the I/O boundary of Kure: reading Kubernetes manifests from files, serializing resources to YAML/JSON, and printing resources in human-readable formats. It integrates with Kure's registered scheme for type-aware parsing.

Each Go block on this page is the body of an `Example` function in `example_test.go`, which
`go test` runs: it imports this package as `io`, `github.com/go-kure/kure/pkg/kubernetes` for the
objects it encodes, `appsv1 "k8s.io/api/apps/v1"`, `corev1 "k8s.io/api/core/v1"`,
`sigs.k8s.io/controller-runtime/pkg/client`, `bytes`, `os`, `path/filepath`, `strings`, and `fmt`
for the lines that print what the example read or wrote. `testdata/manifests.yaml` is a
Deployment and a Service.

## Parsing

### Parse YAML Files

<!-- doc-example: pkg/io ExampleParseFile -->
```go
// Parse a multi-document YAML file into typed Kubernetes objects
objects, err := io.ParseFile("testdata/manifests.yaml")
if err != nil {
    panic(err)
}

// Parse YAML bytes directly
yamlData := []byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: app-config\n")
more, err := io.ParseYAML(yamlData)
if err != nil {
    panic(err)
}

for _, obj := range append(objects, more...) {
    fmt.Printf("%T %s\n", obj, obj.GetName())
}
```
<!-- doc-example:end -->

### Unstructured Fallback

By default, only GVKs registered in the kure scheme are accepted. To parse
arbitrary Kubernetes YAML (CRDs, custom operators, etc.) use
`ParseYAMLWithOptions` or `ParseFileWithOptions` with `AllowUnstructured`.

Which kinds are typed follows the scheme, so it widens when kure registers more.
`PriorityClass`, `EndpointSlice`, `Lease`, `RuntimeClass`, the two webhook
configurations and the four admission policy kinds are registered and come back as
their `k8s.io/api` types in both modes; a caller that matched them as
`*unstructured.Unstructured` must match the typed object instead. `APIService` and
`VerticalPodAutoscaler` are not registered and still need `AllowUnstructured`. The
full list is the [generated table](/api-reference/api-tables/).

A list document is flattened into its items; the list itself is never returned.
The items come back in the list's order and take the list's place in a
multi-document stream, and an empty list yields nothing.

- A `<Kind>List` of a registered kind (`DeploymentList`, `PriorityClassList`) yields
  typed items in both modes. An item may leave `apiVersion` and `kind` out, as the
  API server's own list responses do; an item that states them must state the kind
  the list holds.
- A generic `v1` `List` is read item by item, each as a document of its own: a
  registered kind comes back typed, an unregistered one follows `AllowUnstructured`,
  and a list inside the list is flattened where it stands. A list may sit inside
  eight Lists; one nested deeper is refused.
- A `<Kind>List` of a kind that is not registered is refused by a strict parse and
  flattened into unstructured items with `AllowUnstructured`.

Each item is decoded by itself, in both list shapes. An item that cannot be decoded,
a `null` among them, is reported with its position (`item 1 of List`,
`item 1 of DeploymentList`), and the items beside it are still returned, as the
documents of a stream are. Only the items are read: the list's own `metadata` is not.

The fallback in use:

<!-- doc-example: pkg/io ExampleParseYAMLWithOptions -->
```go
yamlData := []byte(`apiVersion: v1
kind: Pod
metadata:
  name: web
---
apiVersion: example.com/v1
kind: Widget
metadata:
  name: my-widget
`)

opts := io.ParseOptions{AllowUnstructured: true}
objects, err := io.ParseYAMLWithOptions(yamlData, opts)
if err != nil {
    panic(err)
}
// Known types are returned as typed objects (e.g. *corev1.Pod).
// Unknown types are returned as *unstructured.Unstructured.
for _, obj := range objects {
    fmt.Printf("%T\n", obj)
}
```
<!-- doc-example:end -->

### Load and Save

`SaveFile` writes one object as YAML; `LoadFile` reads YAML into an object you pass:

<!-- doc-example: pkg/io ExampleSaveFile -->
```go
dir, err := os.MkdirTemp("", "kure-io-example")
if err != nil {
    panic(err)
}
defer func() { _ = os.RemoveAll(dir) }()
path := filepath.Join(dir, "service.yaml")

// Save an object to file
service := kubernetes.CreateService("web", "default")
if err := io.SaveFile(path, service); err != nil {
    panic(err)
}

// Load a single object from file
var loaded corev1.Service
if err := io.LoadFile(path, &loaded); err != nil {
    panic(err)
}
fmt.Println(loaded.Kind, loaded.Namespace+"/"+loaded.Name)
```
<!-- doc-example:end -->

## Serialization

### Marshal and Unmarshal

`Marshal` writes YAML to an `io.Writer`, and `Unmarshal` reads it from an `io.Reader`:

<!-- doc-example: pkg/io ExampleMarshal -->
```go
deployment := kubernetes.CreateDeployment("web", "default")

// Serialize to YAML
var buf bytes.Buffer
if err := io.Marshal(&buf, deployment); err != nil {
    panic(err)
}

// Deserialize from YAML
var obj appsv1.Deployment
if err := io.Unmarshal(&buf, &obj); err != nil {
    panic(err)
}
fmt.Println(obj.Kind, obj.Namespace+"/"+obj.Name)
```
<!-- doc-example:end -->

### Encode Multiple Objects

The encoders take `[]*client.Object`, where the parsers return `[]client.Object`. The YAML
encoder separates documents with `---`. The JSON encoder writes one object per line and follows
each, the last included, with a `---` line, so its output is not a JSON array.

<!-- doc-example: pkg/io ExampleEncodeObjectsToYAML -->
```go
var configMap client.Object = kubernetes.CreateConfigMap("app-config", "default")
var service client.Object = kubernetes.CreateService("web", "default")
objects := []*client.Object{&configMap, &service}

// Encode as multi-document YAML
yamlData, err := io.EncodeObjectsToYAML(objects)
if err != nil {
    panic(err)
}

// Encode as JSON: one object per line, each followed by a --- separator
jsonData, err := io.EncodeObjectsToJSON(objects)
if err != nil {
    panic(err)
}

fmt.Print(string(yamlData))
fmt.Print(string(jsonData))
```
<!-- doc-example:end -->

### Deterministic Field Ordering

<!-- doc-example: pkg/io ExampleEncodeObjectsToYAMLWithOptions -->
```go
configMap := kubernetes.CreateConfigMap("app-config", "default")
configMap.Data = map[string]string{"LOG_LEVEL": "info"}
var obj client.Object = configMap
objects := []*client.Object{&obj}

// Encode with Kubernetes-conventional field ordering:
// apiVersion, kind, metadata, spec, ... status (last)
opts := io.EncodeOptions{KubernetesFieldOrder: true}
yamlData, err := io.EncodeObjectsToYAMLWithOptions(objects, opts)
if err != nil {
    panic(err)
}
fmt.Print(string(yamlData))
```
<!-- doc-example:end -->

### Server-Set Field Stripping

By default, YAML encoding (`EncodeObjectsToYAML`, `EncodeObjectsToYAMLWithOptions`) strips server-managed metadata fields that should not appear in client-generated manifests: `managedFields`, `resourceVersion`, `uid`, `generation`, `selfLink`, the `kubectl.kubernetes.io/last-applied-configuration` annotation, null `creationTimestamp`, and empty `status`.

The example encodes one Service at each stripping level and reports whether its
`resourceVersion` and its empty `status` survived:

<!-- doc-example: pkg/io ExampleServerFieldStripping -->
```go
// A Service as read back from a cluster: a resourceVersion, and an empty status
service := kubernetes.CreateService("web", "default")
service.ResourceVersion = "12345"
var obj client.Object = service
objects := []*client.Object{&obj}

for _, opts := range []io.EncodeOptions{
    // Default behavior — full stripping (the zero value, which EncodeObjectsToYAML uses)
    {},
    // Explicit full stripping with field ordering
    {KubernetesFieldOrder: true, ServerFieldStripping: io.StripServerFieldsFull},
    // Basic stripping (only null creationTimestamp and empty status)
    {ServerFieldStripping: io.StripServerFieldsBasic},
    // No stripping — preserve all fields as-is
    {ServerFieldStripping: io.StripServerFieldsNone},
} {
    yamlData, err := io.EncodeObjectsToYAMLWithOptions(objects, opts)
    if err != nil {
        panic(err)
    }
    out := string(yamlData)
    fmt.Println(strings.Contains(out, "resourceVersion:"), strings.Contains(out, "status:"))
}
```
<!-- doc-example:end -->

## Printing

### Output Formats

The package supports these output formats, named as kubectl names them:

| Format | Constant | Description |
|--------|----------|-------------|
| YAML | `OutputFormatYAML` | Full YAML output |
| JSON | `OutputFormatJSON` | Full JSON output |
| Table | `OutputFormatTable` | Columnar table view |
| Wide | `OutputFormatWide` | Extended table with extra columns |
| Name | `OutputFormatName` | `kind[.group]/name`, with ` (namespace: <ns>)` appended for a namespaced object |

### Usage

The example prints one ConfigMap three ways: as YAML, as a table through a `ResourcePrinter`
(`ShowLabels` adds the `LABELS` column), and in the format a user-supplied string names.
`ValidateOutputFormat` turns that string into an `OutputFormat`, and fails on one it does not
know. `PrintObjectsAsTable` prints a table directly, without a `ResourcePrinter`.

<!-- doc-example: pkg/io ExampleNewResourcePrinter -->
```go
configMap := kubernetes.CreateConfigMap("app-config", "default")
configMap.Labels = map[string]string{"app": "web"}
var obj client.Object = configMap
objects := []*client.Object{&obj}

// Print as YAML to stdout
if err := io.PrintObjectsAsYAML(objects, os.Stdout); err != nil {
    panic(err)
}

// Use ResourcePrinter for configurable output
printer := io.NewResourcePrinter(io.PrintOptions{
    OutputFormat: io.OutputFormatTable,
    ShowLabels:   true,
})
if err := printer.Print(objects, os.Stdout); err != nil {
    panic(err)
}

// Validate a format string before use
format, err := io.ValidateOutputFormat("name")
if err != nil {
    panic(err)
}

// Format-agnostic printing (selects the printer by format)
if err := io.PrintObjects(objects, format, io.PrintOptions{}, os.Stdout); err != nil {
    panic(err)
}
```
<!-- doc-example:end -->

## Related Packages

- [errors](/api-reference/errors/) - Error types for parse failures
- [kubernetes](/api-reference/kubernetes-builders/) - Scheme registration for type-aware parsing
