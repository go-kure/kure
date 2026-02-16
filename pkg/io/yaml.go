package io

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"

	kjson "k8s.io/apimachinery/pkg/runtime/serializer/json"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// Buffer is a simple in-memory buffer that implements io.Reader and io.Writer
// and can marshal and unmarshal YAML representations of objects.
type Buffer struct {
	bytes.Buffer
}

// Marshal writes the YAML representation of obj to the buffer.
func (b *Buffer) Marshal(obj interface{}) error {
	b.Reset()
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = b.Write(data)
	return err
}

// Unmarshal parses the buffer contents as YAML into obj.
func (b *Buffer) Unmarshal(obj interface{}) error {
	return yaml.Unmarshal(b.Bytes(), obj)
}

// Marshal writes the YAML representation of obj to w.
func Marshal(w io.Writer, obj interface{}) error {
	data, err := yaml.Marshal(obj)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// Unmarshal reads YAML from r into obj.
func Unmarshal(r io.Reader, obj interface{}) error {
	data, err := io.ReadAll(r)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, obj)
}

// SaveFile marshals obj as YAML and writes it to the given file path.
func SaveFile(path string, obj interface{}) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
		}
	}(f)
	return Marshal(f, obj)
}

// LoadFile reads YAML from the given path and unmarshals it into obj.
func LoadFile(path string, obj interface{}) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {

		}
	}(f)
	return Unmarshal(f, obj)
}

func EncodeObjectsTo(objects []*client.Object, yamlOutput bool) ([]byte, error) {
	serializer := kjson.NewSerializerWithOptions(
		kjson.DefaultMetaFactory, kubernetes.Scheme, kubernetes.Scheme,
		kjson.SerializerOptions{Yaml: yamlOutput, Pretty: false, Strict: false},
	)

	var buf bytes.Buffer
	for _, obj := range objects {
		if err := serializer.Encode(*obj, &buf); err != nil {
			return nil, err
		}
		// Add YAML document separator
		buf.WriteString("---\n")
	}
	return buf.Bytes(), nil
}

// EncodeObjectsToYAML encodes Kubernetes objects to clean YAML, stripping
// all known server-managed metadata fields by default (see [StripServerFieldsFull]).
func EncodeObjectsToYAML(objects []*client.Object) ([]byte, error) {
	return EncodeObjectsToYAMLWithOptions(objects, EncodeOptions{})
}

// EncodeObjectsToYAMLWithOptions encodes Kubernetes objects to clean YAML with
// configurable output options. When opts.KubernetesFieldOrder is true,
// top-level fields are emitted in the conventional order used by kubectl,
// Helm, and Kustomize (apiVersion, kind, metadata, spec, ..., status last).
func EncodeObjectsToYAMLWithOptions(objects []*client.Object, opts EncodeOptions) ([]byte, error) {
	var buf bytes.Buffer
	for i, obj := range objects {
		cleaned, err := marshalCleanResource(*obj, opts)
		if err != nil {
			return nil, err
		}
		if i > 0 {
			buf.WriteString("---\n")
		}
		buf.Write(cleaned)
	}
	return buf.Bytes(), nil
}

func EncodeObjectsToJSON(objects []*client.Object) ([]byte, error) {
	return EncodeObjectsTo(objects, false)
}

// marshalCleanResource serializes a single Kubernetes resource to clean YAML,
// stripping server-managed fields that are artifacts of K8s API machinery.
// The set of stripped fields is controlled by opts.ServerFieldStripping:
//   - Full (default): managedFields, resourceVersion, uid, generation, selfLink,
//     kubectl.kubernetes.io/last-applied-configuration annotation,
//     null creationTimestamp, and empty status
//   - Basic: null creationTimestamp and empty status only
//   - None: no stripping
func marshalCleanResource(obj client.Object, opts EncodeOptions) ([]byte, error) {
	jsonBytes, err := json.Marshal(obj)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal resource to JSON: %w", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &raw); err != nil {
		return nil, fmt.Errorf("failed to unmarshal JSON for cleanup: %w", err)
	}

	cleanResourceMap(raw, opts.ServerFieldStripping)

	if opts.KubernetesFieldOrder {
		return marshalOrderedYAML(raw)
	}

	out, err := yaml.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal cleaned resource to YAML: %w", err)
	}

	return out, nil
}

// cleanResourceMap removes server-managed fields from a resource map.
// The level parameter controls which fields are removed.
func cleanResourceMap(m map[string]interface{}, level ServerFieldStripping) {
	if level == StripServerFieldsNone {
		return
	}
	removeEmptyStatus(m)
	cleanMetadata(m, "metadata", level)

	// spec.template.metadata (Deployments, Jobs, etc.)
	if spec, ok := m["spec"].(map[string]interface{}); ok {
		cleanMetadata(spec, "template", level)

		// spec.jobTemplate.metadata and spec.jobTemplate.spec.template.metadata (CronJobs)
		if jobTemplate, ok := spec["jobTemplate"].(map[string]interface{}); ok {
			cleanMetadata(jobTemplate, "metadata", level)
			if jtSpec, ok := jobTemplate["spec"].(map[string]interface{}); ok {
				cleanMetadata(jtSpec, "template", level)
			}
		}
	}
}

// cleanMetadata removes server-managed fields from a metadata block.
// When parentKey is "metadata", parent[parentKey] is the metadata map directly.
// Otherwise, parent[parentKey] is a container (e.g. template) whose "metadata"
// child is cleaned.
func cleanMetadata(parent map[string]interface{}, parentKey string, level ServerFieldStripping) {
	var metadata map[string]interface{}
	if parentKey == "metadata" {
		md, ok := parent[parentKey].(map[string]interface{})
		if !ok {
			return
		}
		metadata = md
	} else {
		child, ok := parent[parentKey].(map[string]interface{})
		if !ok {
			return
		}
		md, ok := child["metadata"].(map[string]interface{})
		if !ok {
			return
		}
		metadata = md
	}

	removeNullCreationTimestamp(metadata)

	if level == StripServerFieldsFull {
		stripServerSetFields(metadata)
	}
}

// stripServerSetFields removes well-known server-set metadata fields:
// managedFields, resourceVersion, uid, generation, selfLink, and the
// kubectl.kubernetes.io/last-applied-configuration annotation.
func stripServerSetFields(metadata map[string]interface{}) {
	delete(metadata, "managedFields")
	delete(metadata, "resourceVersion")
	delete(metadata, "uid")
	delete(metadata, "generation")
	delete(metadata, "selfLink")

	if annotations, ok := metadata["annotations"].(map[string]interface{}); ok {
		delete(annotations, "kubectl.kubernetes.io/last-applied-configuration")
		if len(annotations) == 0 {
			delete(metadata, "annotations")
		}
	}
}

// removeNullCreationTimestamp deletes creationTimestamp if its value is nil.
func removeNullCreationTimestamp(metadata map[string]interface{}) {
	if v, exists := metadata["creationTimestamp"]; exists && v == nil {
		delete(metadata, "creationTimestamp")
	}
}

// removeEmptyStatus deletes the top-level "status" key if it is nil or an empty map.
func removeEmptyStatus(m map[string]interface{}) {
	v, exists := m["status"]
	if !exists {
		return
	}
	switch s := v.(type) {
	case nil:
		delete(m, "status")
	case map[string]interface{}:
		if isDeepEmpty(s) {
			delete(m, "status")
		}
	}
}

// isDeepEmpty returns true if a map is empty or contains only empty maps recursively.
func isDeepEmpty(m map[string]interface{}) bool {
	for _, v := range m {
		switch val := v.(type) {
		case map[string]interface{}:
			if !isDeepEmpty(val) {
				return false
			}
		default:
			return false
		}
	}
	return true
}

// PrintObjects prints objects using the specified output format and options
func PrintObjects(objects []*client.Object, format OutputFormat, options PrintOptions, w io.Writer) error {
	printer := NewResourcePrinter(PrintOptions{
		OutputFormat: format,
		NoHeaders:    options.NoHeaders,
		ShowLabels:   options.ShowLabels,
		ColumnLabels: options.ColumnLabels,
		SortBy:       options.SortBy,
	})
	return printer.Print(objects, w)
}

// PrintObjectsAsTable prints objects in table format using the simple table printer
func PrintObjectsAsTable(objects []*client.Object, wide, noHeaders bool, w io.Writer) error {
	printer := NewSimpleTablePrinter(wide, noHeaders)
	return printer.Print(objects, w)
}

// PrintObjectsAsYAML is a convenience function for YAML output
func PrintObjectsAsYAML(objects []*client.Object, w io.Writer) error {
	data, err := EncodeObjectsToYAML(objects)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}

// PrintObjectsAsJSON is a convenience function for JSON output
func PrintObjectsAsJSON(objects []*client.Object, w io.Writer) error {
	data, err := EncodeObjectsToJSON(objects)
	if err != nil {
		return err
	}
	_, err = w.Write(data)
	return err
}
