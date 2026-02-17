package gvk

import (
	"io"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/errors"
)

// ParseOptions configures how GVK parsing behaves
type ParseOptions struct {
	// StrictGVK requires apiVersion and kind to be present
	StrictGVK bool
	// AllowUnknownTypes allows parsing of unregistered types
	AllowUnknownTypes bool
}

// DefaultParseOptions provides sensible defaults for parsing
var DefaultParseOptions = ParseOptions{
	StrictGVK:         true,
	AllowUnknownTypes: false,
}

// ParseSingle parses a single GVK-enabled type from YAML data
func ParseSingle[T any](data []byte, registry *Registry[T], options *ParseOptions) (*TypedWrapper[T], error) {
	if options == nil {
		options = &DefaultParseOptions
	}

	wrapper := NewTypedWrapper(registry)
	if err := yaml.Unmarshal(data, wrapper); err != nil {
		return nil, errors.Errorf("failed to unmarshal: %w", err)
	}

	return wrapper, nil
}

// ParseMultiple parses multiple GVK-enabled types from YAML data (separated by ---)
func ParseMultiple[T any](data []byte, registry *Registry[T], options *ParseOptions) ([]*TypedWrapper[T], error) {
	if options == nil {
		options = &DefaultParseOptions
	}

	var wrappers []*TypedWrapper[T]

	// Split by YAML documents
	documents := splitYAMLDocuments(data)

	for i, doc := range documents {
		if len(doc) == 0 {
			continue // Skip empty documents
		}

		wrapper := NewTypedWrapper(registry)
		if err := yaml.Unmarshal(doc, wrapper); err != nil {
			return nil, errors.Errorf("failed to unmarshal document %d: %w", i, err)
		}

		wrappers = append(wrappers, wrapper)
	}

	return wrappers, nil
}

// ParseStream parses a stream of YAML documents
func ParseStream[T any](reader io.Reader, registry *Registry[T], options *ParseOptions) ([]*TypedWrapper[T], error) {
	if options == nil {
		options = &DefaultParseOptions
	}

	var wrappers []*TypedWrapper[T]
	decoder := yaml.NewDecoder(reader)

	for {
		var node yaml.Node
		if err := decoder.Decode(&node); err != nil {
			if err == io.EOF {
				break
			}
			return nil, errors.Errorf("failed to decode YAML document: %w", err)
		}

		wrapper := NewTypedWrapper(registry)
		if err := node.Decode(wrapper); err != nil {
			return nil, errors.Errorf("failed to unmarshal wrapper: %w", err)
		}

		wrappers = append(wrappers, wrapper)
	}

	return wrappers, nil
}

// ValidateGVK validates that a GVK is properly formed
func ValidateGVK(gvk GVK) error {
	if gvk.Kind == "" {
		return errors.Errorf("kind is required")
	}
	if gvk.Version == "" {
		return errors.Errorf("version is required")
	}
	// Group can be empty for core types
	return nil
}

// splitYAMLDocuments splits YAML data by document separators
func splitYAMLDocuments(data []byte) [][]byte {
	// This is a simple implementation - a more robust version would
	// properly handle YAML document separators considering indentation
	// and quoted strings

	documents := [][]byte{}
	current := []byte{}

	lines := splitLines(data)
	for _, line := range lines {
		if string(line) == "---" || string(line) == "---\n" {
			if len(current) > 0 {
				documents = append(documents, current)
				current = []byte{}
			}
			continue
		}
		current = append(current, line...)
		current = append(current, '\n')
	}

	if len(current) > 0 {
		documents = append(documents, current)
	}

	return documents
}

// splitLines splits data into lines
func splitLines(data []byte) [][]byte {
	var lines [][]byte
	start := 0

	for i, b := range data {
		if b == '\n' {
			lines = append(lines, data[start:i])
			start = i + 1
		}
	}

	if start < len(data) {
		lines = append(lines, data[start:])
	}

	return lines
}
