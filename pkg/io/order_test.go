package io

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
	sigsyaml "sigs.k8s.io/yaml"
)

func TestSortedKeysKubernetesOrder(t *testing.T) {
	m := map[string]interface{}{
		"status":     map[string]interface{}{},
		"spec":       map[string]interface{}{},
		"metadata":   map[string]interface{}{},
		"kind":       "Deployment",
		"apiVersion": "apps/v1",
		"extra":      "value",
		"another":    "value",
	}

	keys := sortedKeys(m, true)
	expected := []string{"apiVersion", "kind", "metadata", "spec", "another", "extra", "status"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %v", len(expected), len(keys), keys)
	}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("position %d: expected %q, got %q (full order: %v)", i, k, keys[i], keys)
		}
	}
}

func TestSortedKeysAlphabetical(t *testing.T) {
	m := map[string]interface{}{
		"zebra":  1,
		"alpha":  2,
		"middle": 3,
	}

	keys := sortedKeys(m, false)
	expected := []string{"alpha", "middle", "zebra"}

	if len(keys) != len(expected) {
		t.Fatalf("expected %d keys, got %d: %v", len(expected), len(keys), keys)
	}
	for i, k := range expected {
		if keys[i] != k {
			t.Errorf("position %d: expected %q, got %q", i, k, keys[i])
		}
	}
}

func TestValueToNode(t *testing.T) {
	tests := []struct {
		name     string
		input    interface{}
		wantTag  string
		wantVal  string
		wantKind int
	}{
		{"nil", nil, "!!null", "null", 8}, // yaml.ScalarNode == 8
		{"bool true", true, "!!bool", "true", 8},
		{"bool false", false, "!!bool", "false", 8},
		{"integer float", float64(8080), "!!int", "8080", 8},
		{"fractional float", float64(3.14), "!!float", "3.14", 8},
		{"high precision float", float64(0.123456789), "!!float", "0.123456789", 8},
		{"string", "hello", "!!str", "hello", 8},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			node := valueToNode(tt.input)
			if node.Tag != tt.wantTag {
				t.Errorf("tag: got %q, want %q", node.Tag, tt.wantTag)
			}
			if node.Value != tt.wantVal {
				t.Errorf("value: got %q, want %q", node.Value, tt.wantVal)
			}
		})
	}

	// Test slice
	t.Run("slice", func(t *testing.T) {
		node := valueToNode([]interface{}{"a", "b"})
		if node.Kind != yaml.SequenceNode {
			t.Errorf("expected SequenceNode (%d), got kind %d", yaml.SequenceNode, node.Kind)
		}
		if len(node.Content) != 2 {
			t.Errorf("expected 2 items, got %d", len(node.Content))
		}
	})

	// Test nested map
	t.Run("nested map", func(t *testing.T) {
		node := valueToNode(map[string]interface{}{"key": "val"})
		if node.Kind != yaml.MappingNode {
			t.Errorf("expected MappingNode (%d), got kind %d", yaml.MappingNode, node.Kind)
		}
	})
}

func TestMarshalOrderedYAML_Deployment(t *testing.T) {
	m := map[string]interface{}{
		"status":     map[string]interface{}{},
		"spec":       map[string]interface{}{"replicas": float64(3)},
		"metadata":   map[string]interface{}{"name": "test", "namespace": "default"},
		"kind":       "Deployment",
		"apiVersion": "apps/v1",
	}

	out, err := marshalOrderedYAML(m)
	if err != nil {
		t.Fatalf("marshalOrderedYAML: %v", err)
	}

	s := string(out)
	lines := strings.Split(s, "\n")

	// Verify first four keys appear in order
	expectedOrder := []string{"apiVersion:", "kind:", "metadata:", "spec:", "status:"}
	idx := 0
	for _, line := range lines {
		if idx < len(expectedOrder) && strings.HasPrefix(strings.TrimSpace(line), expectedOrder[idx]) {
			idx++
		}
	}
	if idx != len(expectedOrder) {
		t.Errorf("expected keys in order %v, only found %d in output:\n%s", expectedOrder, idx, s)
	}
}

func TestMarshalOrderedYAML_ConfigMap(t *testing.T) {
	m := map[string]interface{}{
		"data":       map[string]interface{}{"key": "value"},
		"metadata":   map[string]interface{}{"name": "cm"},
		"kind":       "ConfigMap",
		"apiVersion": "v1",
		"status":     map[string]interface{}{"phase": "Active"},
	}

	out, err := marshalOrderedYAML(m)
	if err != nil {
		t.Fatalf("marshalOrderedYAML: %v", err)
	}

	s := string(out)

	// data should come after metadata and before status
	dataIdx := strings.Index(s, "data:")
	metadataIdx := strings.Index(s, "metadata:")
	statusIdx := strings.Index(s, "status:")

	if metadataIdx >= dataIdx {
		t.Errorf("metadata should come before data:\n%s", s)
	}
	if dataIdx >= statusIdx {
		t.Errorf("data should come before status:\n%s", s)
	}
}

func TestMarshalOrderedYAML_NestedAlphabetical(t *testing.T) {
	m := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "ConfigMap",
		"metadata": map[string]interface{}{
			"namespace": "default",
			"name":      "test",
			"labels": map[string]interface{}{
				"zebra": "z",
				"alpha": "a",
			},
		},
	}

	out, err := marshalOrderedYAML(m)
	if err != nil {
		t.Fatalf("marshalOrderedYAML: %v", err)
	}

	s := string(out)

	// Inside metadata, labels should come before name, name before namespace (alphabetical)
	labelsIdx := strings.Index(s, "labels:")
	nameIdx := strings.Index(s, "name:")
	namespaceIdx := strings.Index(s, "namespace:")

	if labelsIdx >= nameIdx {
		t.Errorf("labels should come before name (alphabetical):\n%s", s)
	}
	if nameIdx >= namespaceIdx {
		t.Errorf("name should come before namespace (alphabetical):\n%s", s)
	}

	// Inside labels, alpha should come before zebra
	alphaIdx := strings.Index(s, "alpha:")
	zebraIdx := strings.Index(s, "zebra:")
	if alphaIdx >= zebraIdx {
		t.Errorf("alpha should come before zebra (alphabetical):\n%s", s)
	}
}

func TestMarshalOrderedYAML_Deterministic(t *testing.T) {
	m := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
			"labels": map[string]interface{}{
				"app":     "test",
				"version": "v1",
			},
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{
					"app": "test",
				},
			},
		},
		"status": map[string]interface{}{},
	}

	first, err := marshalOrderedYAML(m)
	if err != nil {
		t.Fatalf("first marshal: %v", err)
	}

	for i := 0; i < 100; i++ {
		out, err := marshalOrderedYAML(m)
		if err != nil {
			t.Fatalf("marshal iteration %d: %v", i, err)
		}
		if string(out) != string(first) {
			t.Fatalf("non-deterministic output at iteration %d:\nfirst:\n%s\ngot:\n%s", i, first, out)
		}
	}
}

func TestMarshalOrderedYAML_RoundTrip(t *testing.T) {
	original := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      "test",
			"namespace": "default",
		},
		"spec": map[string]interface{}{
			"replicas": float64(3),
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "app",
							"image": "nginx:latest",
							"ports": []interface{}{
								map[string]interface{}{
									"containerPort": float64(8080),
								},
							},
						},
					},
				},
			},
		},
	}

	out, err := marshalOrderedYAML(original)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var roundTripped map[string]interface{}
	if err := sigsyaml.Unmarshal(out, &roundTripped); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	// Verify key data is preserved
	if roundTripped["apiVersion"] != "apps/v1" {
		t.Errorf("apiVersion lost: got %v", roundTripped["apiVersion"])
	}
	if roundTripped["kind"] != "Deployment" {
		t.Errorf("kind lost: got %v", roundTripped["kind"])
	}

	meta, ok := roundTripped["metadata"].(map[string]interface{})
	if !ok {
		t.Fatal("metadata not a map")
	}
	if meta["name"] != "test" {
		t.Errorf("metadata.name lost: got %v", meta["name"])
	}

	spec, ok := roundTripped["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec not a map")
	}
	// sigs.k8s.io/yaml unmarshals into interface{} as float64
	replicas, ok := spec["replicas"].(float64)
	if !ok {
		// May also be int depending on yaml library version
		if r, ok2 := spec["replicas"].(int); ok2 {
			replicas = float64(r)
		} else {
			t.Fatalf("spec.replicas unexpected type: %T", spec["replicas"])
		}
	}
	if replicas != 3 {
		t.Errorf("spec.replicas lost: got %v", replicas)
	}

	// Verify containers survived
	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.template not a map")
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("spec.template.spec not a map")
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok || len(containers) != 1 {
		t.Fatal("containers not preserved")
	}
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		t.Fatal("container not a map")
	}
	if container["image"] != "nginx:latest" {
		t.Errorf("container image lost: got %v", container["image"])
	}
}
