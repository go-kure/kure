package patch

import (
	"fmt"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

func TestArraySelectorPatching(t *testing.T) {
	// Test YAML with service containing ports array
	testYAML := `apiVersion: v1
kind: Service
metadata:
  name: test-service
spec:
  ports:
  - name: https
    port: 443
    targetPort: 10250
  - name: http
    port: 80
    targetPort: 8080
  selector:
    app: test-app`

	// Test TOML patch with array selectors
	testPatch := `[service.test-service.ports.name=https]
port: 9443
targetPort: webhook

[service.test-service.ports.name=http]
port: 8888`

	// Load the YAML document (for structure validation)
	_, err := LoadResourcesWithStructure(strings.NewReader(testYAML))
	if err != nil {
		t.Fatalf("Failed to load YAML: %v", err)
	}

	// Load the TOML patch
	patches, err := LoadPatchFileWithVariables(strings.NewReader(testPatch), nil)
	if err != nil {
		t.Fatalf("Failed to load patches: %v", err)
	}

	// Verify patch parsing
	if len(patches) != 3 {
		t.Errorf("Expected 3 patches, got %d", len(patches))
	}

	// Check that all patches have correct structure
	for i, p := range patches {
		if p.Target != "service.test-service" {
			t.Errorf("Patch %d: expected target 'service.test-service', got '%s'", i, p.Target)
		}
		if p.Patch.Path != "spec.ports" {
			t.Errorf("Patch %d: expected path 'spec.ports', got '%s'", i, p.Patch.Path)
		}
		if p.Patch.Op != "replace" {
			t.Errorf("Patch %d: expected op 'replace', got '%s'", i, p.Patch.Op)
		}
		if p.Patch.Selector == "" {
			t.Errorf("Patch %d: expected non-empty selector", i)
		}
	}

	// Create a test resource with proper types (avoiding deep copy issues)
	serviceData := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Service",
		"metadata": map[string]interface{}{
			"name": "test-service",
		},
		"spec": map[string]interface{}{
			"ports": []interface{}{
				map[string]interface{}{
					"name":       "https",
					"port":       float64(443),
					"targetPort": float64(10250),
				},
				map[string]interface{}{
					"name":       "http",
					"port":       float64(80),
					"targetPort": float64(8080),
				},
			},
			"selector": map[string]interface{}{
				"app": "test-app",
			},
		},
	}

	service := &unstructured.Unstructured{Object: serviceData}

	// Apply patches using the new array selector logic
	for _, patchSpec := range patches {
		err := applyPatchOp(service.Object, patchSpec.Patch)
		if err != nil {
			t.Fatalf("Failed to apply patch: %v", err)
		}
	}

	// Verify the results
	ports, found, err := unstructured.NestedSlice(service.Object, "spec", "ports")
	if !found || err != nil {
		t.Fatalf("Failed to get ports: found=%v, err=%v", found, err)
	}

	if len(ports) != 2 {
		t.Fatalf("Expected 2 ports, got %d", len(ports))
	}

	// Check HTTPS port was updated
	httpsPort := ports[0].(map[string]interface{})
	if httpsPort["name"] != "https" {
		t.Errorf("Expected first port to be 'https', got %v", httpsPort["name"])
	}
	if fmt.Sprintf("%v", httpsPort["port"]) != "9443" {  // Updated value
		t.Errorf("Expected HTTPS port to be 9443, got %v", httpsPort["port"])
	}
	if httpsPort["targetPort"] != "webhook" {  // Updated value
		t.Errorf("Expected HTTPS targetPort to be 'webhook', got %v", httpsPort["targetPort"])
	}

	// Check HTTP port was updated
	httpPort := ports[1].(map[string]interface{})
	if httpPort["name"] != "http" {
		t.Errorf("Expected second port to be 'http', got %v", httpPort["name"])
	}
	if fmt.Sprintf("%v", httpPort["port"]) != "8888" {  // Updated value
		t.Errorf("Expected HTTP port to be 8888, got %v", httpPort["port"])
	}
	if httpPort["targetPort"] != float64(8080) {  // Unchanged value
		t.Errorf("Expected HTTP targetPort to be 8080, got %v", httpPort["targetPort"])
	}
}

func TestParsePatchLineWithMidSelectors(t *testing.T) {
	testCases := []struct {
		name           string
		input          string
		expectedPath   string
		expectedSel    string
		expectedValue  interface{}
	}{
		{
			name:          "service port selector",
			input:         "spec.ports[name=https].port",
			expectedPath:  "spec.ports",
			expectedSel:   "name=https",
			expectedValue: map[string]interface{}{"port": "test-value"},
		},
		{
			name:          "container image selector",
			input:         "spec.template.spec.containers[name=main].image",
			expectedPath:  "spec.template.spec.containers",
			expectedSel:   "name=main",
			expectedValue: map[string]interface{}{"image": "test-value"},
		},
		{
			name:          "ingress path selector",
			input:         "spec.rules[0].paths[path=/api].backend.service.name",
			expectedPath:  "spec.rules[0].paths",
			expectedSel:   "path=/api",
			expectedValue: map[string]interface{}{"backend.service.name": "test-value"},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			op, err := ParsePatchLine(tc.input, "test-value")
			if err != nil {
				t.Fatalf("ParsePatchLine failed: %v", err)
			}

			if op.Path != tc.expectedPath {
				t.Errorf("Expected path %s, got %s", tc.expectedPath, op.Path)
			}

			if op.Selector != tc.expectedSel {
				t.Errorf("Expected selector %s, got %s", tc.expectedSel, op.Selector)
			}

			// For complex paths, check that the value is a map with the remaining path
			if valueMap, ok := tc.expectedValue.(map[string]interface{}); ok {
				opValueMap, ok := op.Value.(map[string]interface{})
				if !ok {
					t.Errorf("Expected value to be a map, got %T", op.Value)
				} else {
					for k, v := range valueMap {
						if opValueMap[k] != v {
							t.Errorf("Expected value[%s] = %v, got %v", k, v, opValueMap[k])
						}
					}
				}
			}
		})
	}
}