package patch

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestApplyStrategicMergePatch_DeploymentContainersMergedByName(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "my-app",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
				"template": map[string]interface{}{
					"spec": map[string]interface{}{
						"containers": []interface{}{
							map[string]interface{}{
								"name":  "main",
								"image": "nginx:1.24",
							},
							map[string]interface{}{
								"name":  "logger",
								"image": "fluentd:latest",
							},
						},
					},
				},
			},
		},
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "main",
							"image": "nginx:1.25",
						},
						map[string]interface{}{
							"name":  "sidecar",
							"image": "envoy:v1.28",
						},
					},
				},
			},
		},
	}

	if err := ApplyStrategicMergePatch(resource, patch, lookup); err != nil {
		t.Fatalf("ApplyStrategicMergePatch: %v", err)
	}

	containers, found, err := unstructured.NestedSlice(resource.Object,
		"spec", "template", "spec", "containers")
	if err != nil || !found {
		t.Fatal("containers not found after patch")
	}

	// SMP should merge by name: main updated, logger preserved, sidecar added
	if len(containers) != 3 {
		t.Fatalf("expected 3 containers, got %d: %v", len(containers), containers)
	}

	containerNames := make(map[string]string)
	for _, c := range containers {
		cm := c.(map[string]interface{})
		containerNames[cm["name"].(string)] = cm["image"].(string)
	}

	if containerNames["main"] != "nginx:1.25" {
		t.Errorf("expected main image nginx:1.25, got %s", containerNames["main"])
	}
	if containerNames["logger"] != "fluentd:latest" {
		t.Errorf("expected logger image fluentd:latest, got %s", containerNames["logger"])
	}
	if containerNames["sidecar"] != "envoy:v1.28" {
		t.Errorf("expected sidecar image envoy:v1.28, got %s", containerNames["sidecar"])
	}
}

func TestApplyStrategicMergePatch_ConfigMapDataMerged(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]interface{}{
				"name":      "my-config",
				"namespace": "default",
			},
			"data": map[string]interface{}{
				"key1": "value1",
				"key2": "value2",
			},
		},
	}

	patch := map[string]interface{}{
		"data": map[string]interface{}{
			"key2": "updated",
			"key3": "new-value",
		},
	}

	if err := ApplyStrategicMergePatch(resource, patch, lookup); err != nil {
		t.Fatalf("ApplyStrategicMergePatch: %v", err)
	}

	data, found, err := unstructured.NestedStringMap(resource.Object, "data")
	if err != nil || !found {
		t.Fatal("data not found after patch")
	}

	if data["key1"] != "value1" {
		t.Errorf("key1 should be preserved, got %s", data["key1"])
	}
	if data["key2"] != "updated" {
		t.Errorf("key2 should be updated, got %s", data["key2"])
	}
	if data["key3"] != "new-value" {
		t.Errorf("key3 should be added, got %s", data["key3"])
	}
}

func TestApplyStrategicMergePatch_UnknownCRDFallback(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "example.com/v1",
			"kind":       "MyCRD",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"items": []interface{}{
					map[string]interface{}{"name": "a"},
					map[string]interface{}{"name": "b"},
				},
				"replicas": int64(1),
			},
		},
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"items": []interface{}{
				map[string]interface{}{"name": "c"},
			},
			"replicas": int64(3),
		},
	}

	if err := ApplyStrategicMergePatch(resource, patch, lookup); err != nil {
		t.Fatalf("ApplyStrategicMergePatch: %v", err)
	}

	// JSON merge patch replaces lists entirely (no merge-by-key for unknown kinds)
	items, found, err := unstructured.NestedSlice(resource.Object, "spec", "items")
	if err != nil || !found {
		t.Fatal("items not found after patch")
	}
	if len(items) != 1 {
		t.Fatalf("expected 1 item (JSON merge replaces list), got %d", len(items))
	}

	replicas, found, err := unstructured.NestedFieldNoCopy(resource.Object, "spec", "replicas")
	if err != nil || !found {
		t.Fatal("replicas not found")
	}
	// JSON round-trip may convert int64 to float64
	switch v := replicas.(type) {
	case float64:
		if v != 3 {
			t.Errorf("expected replicas 3, got %v", v)
		}
	case int64:
		if v != 3 {
			t.Errorf("expected replicas 3, got %v", v)
		}
	default:
		t.Errorf("unexpected replicas type %T", replicas)
	}
}

func TestApplyStrategicMergePatch_NilLookup(t *testing.T) {
	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name": "test",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
	}

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": int64(5),
		},
	}

	// nil lookup should fall back to JSON merge patch
	if err := ApplyStrategicMergePatch(resource, patch, nil); err != nil {
		t.Fatalf("ApplyStrategicMergePatch with nil lookup: %v", err)
	}

	replicas, found, err := unstructured.NestedFieldNoCopy(resource.Object, "spec", "replicas")
	if err != nil || !found {
		t.Fatal("replicas not found")
	}
	// JSON round-trip converts to float64
	switch v := replicas.(type) {
	case float64:
		if v != 5 {
			t.Errorf("expected replicas 5, got %v", v)
		}
	case int64:
		if v != 5 {
			t.Errorf("expected replicas 5, got %v", v)
		}
	}
}

func TestStrategicPatch_VariableSubstitution(t *testing.T) {
	patchYAML := `- target: my-app
  type: strategic
  patch:
    spec:
      replicas: "${values.replicas}"
      template:
        spec:
          containers:
          - name: main
            image: "${values.image}"
`
	varCtx := &VariableContext{
		Values: map[string]interface{}{
			"replicas": "5",
			"image":    "nginx:1.25",
		},
	}

	patches, err := LoadYAMLPatchFile(strings.NewReader(patchYAML), varCtx)
	if err != nil {
		t.Fatalf("LoadYAMLPatchFile: %v", err)
	}

	if len(patches) != 1 {
		t.Fatalf("expected 1 patch, got %d", len(patches))
	}
	if patches[0].Strategic == nil {
		t.Fatal("expected a strategic patch")
	}

	smpPatch := patches[0].Strategic.Patch

	// Verify nested substitution worked
	spec, ok := smpPatch["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec not a map: %T", smpPatch["spec"])
	}
	// After type inference, replicas should be converted to int
	if fmt.Sprintf("%v", spec["replicas"]) != "5" {
		t.Errorf("expected replicas value 5, got %v", spec["replicas"])
	}

	tmpl, ok := spec["template"].(map[string]interface{})
	if !ok {
		t.Fatalf("template not a map: %T", spec["template"])
	}
	tmplSpec, ok := tmpl["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("template.spec not a map: %T", tmpl["spec"])
	}
	containers, ok := tmplSpec["containers"].([]interface{})
	if !ok || len(containers) == 0 {
		t.Fatal("containers not found or empty")
	}
	container, ok := containers[0].(map[string]interface{})
	if !ok {
		t.Fatal("container is not a map")
	}
	if container["image"] != "nginx:1.25" {
		t.Errorf("expected image 'nginx:1.25', got %v", container["image"])
	}
}

// TestStrategicPatch_VariableSubstitutionInfersTypes verifies that after
// variable substitution, numeric and boolean strings are converted to their
// typed equivalents (matching field-level patch behavior).
func TestStrategicPatch_VariableSubstitutionInfersTypes(t *testing.T) {
	patchYAML := `- target: my-app
  type: strategic
  patch:
    spec:
      replicas: "${values.replicas}"
`
	varCtx := &VariableContext{
		Values: map[string]interface{}{
			"replicas": "3",
		},
	}

	patches, err := LoadYAMLPatchFile(strings.NewReader(patchYAML), varCtx)
	if err != nil {
		t.Fatalf("LoadYAMLPatchFile: %v", err)
	}

	if len(patches) != 1 || patches[0].Strategic == nil {
		t.Fatal("expected 1 strategic patch")
	}

	spec, ok := patches[0].Strategic.Patch["spec"].(map[string]interface{})
	if !ok {
		t.Fatalf("spec not a map: %T", patches[0].Strategic.Patch["spec"])
	}

	replicas := spec["replicas"]
	// After type inference, replicas should be an int, not a string
	switch replicas.(type) {
	case int, int64:
		// OK
	default:
		t.Errorf("expected replicas to be inferred as integer, got %T(%v)", replicas, replicas)
	}
}

// TestLoadYAMLPatchFile_StrategicNilPatchPayload verifies that a strategic
// entry with no patch block returns a clear error instead of silently
// producing a nil patch map.
func TestLoadYAMLPatchFile_StrategicNilPatchPayload(t *testing.T) {
	tests := []struct {
		name  string
		input string
	}{
		{
			name: "no patch key",
			input: `- target: my-app
  type: strategic
`,
		},
		{
			name: "explicit empty patch",
			input: `- target: my-app
  type: strategic
  patch: {}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadYAMLPatchFile(strings.NewReader(tt.input), nil)
			if err == nil {
				t.Fatal("expected error for strategic patch with no payload, got nil")
			}
			if !strings.Contains(err.Error(), "has no patch payload") {
				t.Errorf("expected 'has no patch payload' in error, got: %s", err.Error())
			}
		})
	}
}

// TestApplyTypedSMP_OriginalMapNotMutated verifies that applyTypedSMP does not
// mutate the original resource.Object map during a successful merge. This
// ensures that if a caller holds a reference to the original map, it remains
// intact after the patch is applied.
func TestApplyTypedSMP_OriginalMapNotMutated(t *testing.T) {
	lookup, err := DefaultKindLookup()
	if err != nil {
		t.Fatalf("DefaultKindLookup: %v", err)
	}

	resource := &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": "apps/v1",
			"kind":       "Deployment",
			"metadata": map[string]interface{}{
				"name":      "test",
				"namespace": "default",
			},
			"spec": map[string]interface{}{
				"replicas": int64(1),
			},
		},
	}

	// Capture a reference to the original map
	origMap := resource.Object

	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"replicas": int64(5),
		},
	}

	if err := ApplyStrategicMergePatch(resource, patch, lookup); err != nil {
		t.Fatalf("ApplyStrategicMergePatch: %v", err)
	}

	// resource.Object should be a new map (the merge result)
	if resource.Object == nil {
		t.Fatal("resource.Object is nil after patch")
	}

	// The original map should still have replicas=1
	origSpec, ok := origMap["spec"].(map[string]interface{})
	if !ok {
		t.Fatal("original spec not a map")
	}
	if origSpec["replicas"] != int64(1) {
		t.Errorf("original map was mutated: replicas = %v, want 1", origSpec["replicas"])
	}

	// The patched resource should have replicas=5
	patchedReplicas, found, err := unstructured.NestedFieldNoCopy(resource.Object, "spec", "replicas")
	if err != nil || !found {
		t.Fatal("replicas not found in patched resource")
	}
	if fmt.Sprintf("%v", patchedReplicas) != "5" {
		t.Errorf("patched replicas = %v, want 5", patchedReplicas)
	}
}

func TestDeepCopyMap(t *testing.T) {
	original := map[string]interface{}{
		"a": "hello",
		"b": map[string]interface{}{
			"c": []interface{}{int64(1), int64(2)},
		},
	}

	copied := deepCopyMap(original)

	// Mutate the copy
	copied["a"] = "world"
	innerCopy := copied["b"].(map[string]interface{})
	innerCopy["c"] = []interface{}{int64(3)}

	// Original should be unchanged
	if original["a"] != "hello" {
		t.Error("original was mutated")
	}
	innerOrig := original["b"].(map[string]interface{})
	origSlice := innerOrig["c"].([]interface{})
	if len(origSlice) != 2 {
		t.Error("original nested slice was mutated")
	}
}
