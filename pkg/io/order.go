package io

import (
	"bytes"
	"fmt"
	"math"
	"sort"
	"strconv"

	"gopkg.in/yaml.v3"
)

// EncodeOptions controls how Kubernetes objects are serialized to YAML.
type EncodeOptions struct {
	// KubernetesFieldOrder emits top-level resource keys in the
	// conventional order used by kubectl, Helm, and Kustomize:
	// apiVersion, kind, metadata, spec, data, stringData, type,
	// then remaining keys alphabetically, with status last.
	// Nested maps remain alphabetically sorted.
	KubernetesFieldOrder bool
}

// kubernetesKeyPriority maps well-known top-level Kubernetes resource
// fields to their conventional emission order.
var kubernetesKeyPriority = map[string]int{
	"apiVersion": 0,
	"kind":       1,
	"metadata":   2,
	"spec":       3,
	"data":       4,
	"stringData": 5,
	"type":       6,
	// "status" handled separately as 999
}

const (
	priorityDefault = 100
	priorityStatus  = 999
)

// marshalOrderedYAML converts a cleaned resource map to YAML bytes with
// top-level keys in Kubernetes-conventional order.
func marshalOrderedYAML(m map[string]interface{}) ([]byte, error) {
	node := mapToNode(m, true)
	doc := &yaml.Node{
		Kind:    yaml.DocumentNode,
		Content: []*yaml.Node{node},
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("failed to encode ordered YAML: %w", err)
	}
	if err := enc.Close(); err != nil {
		return nil, fmt.Errorf("failed to close YAML encoder: %w", err)
	}
	return buf.Bytes(), nil
}

// mapToNode builds a yaml.v3 MappingNode from a map. When topLevel is
// true, keys are ordered per Kubernetes conventions; otherwise keys are
// sorted alphabetically.
func mapToNode(m map[string]interface{}, topLevel bool) *yaml.Node {
	node := &yaml.Node{
		Kind: yaml.MappingNode,
	}
	keys := sortedKeys(m, topLevel)
	for _, k := range keys {
		keyNode := &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: k,
			Tag:   "!!str",
		}
		valNode := valueToNode(m[k])
		node.Content = append(node.Content, keyNode, valNode)
	}
	return node
}

// valueToNode converts a value produced by json.Unmarshal into interface{}
// to a yaml.v3 Node.
func valueToNode(v interface{}) *yaml.Node {
	switch val := v.(type) {
	case nil:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: "null",
			Tag:   "!!null",
		}
	case bool:
		s := "false"
		if val {
			s = "true"
		}
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: s,
			Tag:   "!!bool",
		}
	case float64:
		return floatToNode(val)
	case string:
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: val,
			Tag:   "!!str",
		}
	case []interface{}:
		node := &yaml.Node{
			Kind: yaml.SequenceNode,
		}
		for _, item := range val {
			node.Content = append(node.Content, valueToNode(item))
		}
		return node
	case map[string]interface{}:
		return mapToNode(val, false)
	default:
		// Fallback: render as string
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: fmt.Sprintf("%v", val),
			Tag:   "!!str",
		}
	}
}

// floatToNode converts a float64 to a yaml.v3 ScalarNode, rendering
// integer-valued floats without a decimal point (e.g. 8080 not 8080.0).
func floatToNode(f float64) *yaml.Node {
	if f == math.Trunc(f) && !math.IsInf(f, 0) && !math.IsNaN(f) {
		return &yaml.Node{
			Kind:  yaml.ScalarNode,
			Value: fmt.Sprintf("%.0f", f),
			Tag:   "!!int",
		}
	}
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Value: strconv.FormatFloat(f, 'g', -1, 64),
		Tag:   "!!float",
	}
}

// sortedKeys returns the keys of m in the appropriate order.
// When topLevel is true, well-known Kubernetes fields are sorted by
// their conventional priority; remaining keys are sorted alphabetically
// and status is always last. When topLevel is false, all keys are
// sorted alphabetically.
func sortedKeys(m map[string]interface{}, topLevel bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	if !topLevel {
		sort.Strings(keys)
		return keys
	}
	sort.Slice(keys, func(i, j int) bool {
		pi := keyPriority(keys[i])
		pj := keyPriority(keys[j])
		if pi != pj {
			return pi < pj
		}
		return keys[i] < keys[j]
	})
	return keys
}

// keyPriority returns the sort weight for a top-level Kubernetes field.
func keyPriority(key string) int {
	if key == "status" {
		return priorityStatus
	}
	if p, ok := kubernetesKeyPriority[key]; ok {
		return p
	}
	return priorityDefault
}
