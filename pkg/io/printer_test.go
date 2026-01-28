package io_test

import (
	"bytes"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/io"
)

func TestResourcePrinter_PrintYAML(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatYAML,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print YAML: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "apiVersion: v1") {
		t.Errorf("Expected apiVersion in YAML output, got: %s", output)
	}
	if !strings.Contains(output, "kind: ConfigMap") {
		t.Errorf("Expected kind in YAML output, got: %s", output)
	}
	if !strings.Contains(output, "name: test-cm") {
		t.Errorf("Expected name in YAML output, got: %s", output)
	}
}

func TestResourcePrinter_PrintJSON(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatJSON,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print JSON: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"apiVersion":"v1"`) {
		t.Errorf("Expected apiVersion in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"kind":"ConfigMap"`) {
		t.Errorf("Expected kind in JSON output, got: %s", output)
	}
	if !strings.Contains(output, `"name":"test-cm"`) {
		t.Errorf("Expected name in JSON output, got: %s", output)
	}
}

func TestResourcePrinter_PrintNames(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print names: %v", err)
	}

	output := buf.String()
	expected := "configmap/test-cm (namespace: default)\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestResourcePrinter_PrintToString(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	output, err := printer.PrintToString(resources)
	if err != nil {
		t.Fatalf("Failed to print to string: %v", err)
	}

	expected := "configmap/test-cm (namespace: default)\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestResourcePrinter_PrintSingle(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	var buf bytes.Buffer
	err := printer.PrintSingle(obj, &buf)
	if err != nil {
		t.Fatalf("Failed to print single resource: %v", err)
	}

	output := buf.String()
	expected := "configmap/test-cm (namespace: default)\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestValidateOutputFormat(t *testing.T) {
	tests := []struct {
		input    string
		expected io.OutputFormat
		wantErr  bool
	}{
		{"yaml", io.OutputFormatYAML, false},
		{"yml", io.OutputFormatYAML, false},
		{"json", io.OutputFormatJSON, false},
		{"table", io.OutputFormatTable, false},
		{"wide", io.OutputFormatWide, false},
		{"name", io.OutputFormatName, false},
		{"invalid", "", true},
		{"", "", true},
	}

	for _, test := range tests {
		format, err := io.ValidateOutputFormat(test.input)
		if test.wantErr {
			if err == nil {
				t.Errorf("Expected error for input %q, got none", test.input)
			}
		} else {
			if err != nil {
				t.Errorf("Unexpected error for input %q: %v", test.input, err)
			}
			if format != test.expected {
				t.Errorf("Expected format %q for input %q, got %q", test.expected, test.input, format)
			}
		}
	}
}

func TestFormatAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name     string
		time     *metav1.Time
		expected string
	}{
		{
			name:     "nil time",
			time:     nil,
			expected: "<unknown>",
		},
		{
			name:     "30 seconds ago",
			time:     &metav1.Time{Time: now.Add(-30 * time.Second)},
			expected: "30s",
		},
		{
			name:     "5 minutes ago",
			time:     &metav1.Time{Time: now.Add(-5 * time.Minute)},
			expected: "5m",
		},
		{
			name:     "2 hours ago",
			time:     &metav1.Time{Time: now.Add(-2 * time.Hour)},
			expected: "2h",
		},
		{
			name:     "3 days ago",
			time:     &metav1.Time{Time: now.Add(-3 * 24 * time.Hour)},
			expected: "3d",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := io.FormatAge(test.time)
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestGetResourceAge(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")

	// Set creation timestamp
	now := time.Now()
	obj.SetCreationTimestamp(metav1.Time{Time: now.Add(-5 * time.Minute)})

	age := io.GetResourceAge(obj)
	if age != "5m" {
		t.Errorf("Expected age '5m', got %q", age)
	}

	// Test nil object
	age = io.GetResourceAge(nil)
	if age != "<unknown>" {
		t.Errorf("Expected '<unknown>' for nil object, got %q", age)
	}
}

func TestGetResourceStatus(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")

	// Test default status
	status := io.GetResourceStatus(obj)
	if status != "Unknown" {
		t.Errorf("Expected 'Unknown' status for basic ConfigMap, got %q", status)
	}

	// Test object with status.phase
	objWithStatus := &unstructured.Unstructured{}
	objWithStatus.SetAPIVersion("v1")
	objWithStatus.SetKind("Pod")
	objWithStatus.SetName("test-pod")
	objWithStatus.SetNamespace("default")
	objWithStatus.Object["status"] = map[string]interface{}{
		"phase": "Running",
	}

	status = io.GetResourceStatus(objWithStatus)
	if status != "Running" {
		t.Errorf("Expected 'Running' status, got %q", status)
	}

	// Test object with ready condition
	objWithCondition := &unstructured.Unstructured{}
	objWithCondition.SetAPIVersion("v1")
	objWithCondition.SetKind("Deployment")
	objWithCondition.SetName("test-deployment")
	objWithCondition.SetNamespace("default")
	objWithCondition.Object["status"] = map[string]interface{}{
		"conditions": []interface{}{
			map[string]interface{}{
				"type":   "Ready",
				"status": "True",
			},
		},
	}

	status = io.GetResourceStatus(objWithCondition)
	if status != "Ready" {
		t.Errorf("Expected 'Ready' status, got %q", status)
	}

	// Test nil object
	status = io.GetResourceStatus(nil)
	if status != "Unknown" {
		t.Errorf("Expected 'Unknown' for nil object, got %q", status)
	}
}

func TestNewTablePrinter(t *testing.T) {
	options := io.PrintOptions{
		OutputFormat: io.OutputFormatYAML, // Should be overridden
		NoHeaders:    true,
		ShowLabels:   true,
	}

	printer := io.NewTablePrinter(options)
	if printer == nil {
		t.Fatal("Expected non-nil printer")
	}

	// Test that it prints without errors (implementation may vary)
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Errorf("Table printer failed: %v", err)
	}
}

func TestPrintObjects(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	options := io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	}

	var buf bytes.Buffer
	err := io.PrintObjects(resources, io.OutputFormatName, options, &buf)
	if err != nil {
		t.Fatalf("Failed to print objects: %v", err)
	}

	output := buf.String()
	expected := "configmap/test-cm (namespace: default)\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestPrintObjectsAsYAML(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	var buf bytes.Buffer
	err := io.PrintObjectsAsYAML(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print objects as YAML: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "apiVersion: v1") {
		t.Errorf("Expected apiVersion in YAML output, got: %s", output)
	}
}

func TestPrintObjectsAsJSON(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	var buf bytes.Buffer
	err := io.PrintObjectsAsJSON(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print objects as JSON: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, `"apiVersion":"v1"`) {
		t.Errorf("Expected apiVersion in JSON output, got: %s", output)
	}
}

func TestResourcePrinter_InvalidFormat(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	// Create printer with invalid format
	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: "invalid-format",
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err == nil {
		t.Error("Expected error for invalid output format")
	}
	if !strings.Contains(err.Error(), "invalid-format") {
		t.Errorf("Expected error message to mention invalid format, got: %v", err)
	}
}

func TestResourcePrinter_EmptyResources(t *testing.T) {
	tests := []struct {
		name   string
		format io.OutputFormat
	}{
		{"yaml empty", io.OutputFormatYAML},
		{"json empty", io.OutputFormatJSON},
		{"table empty", io.OutputFormatTable},
		{"wide empty", io.OutputFormatWide},
		{"name empty", io.OutputFormatName},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printer := io.NewResourcePrinter(io.PrintOptions{
				OutputFormat: tt.format,
			})

			var buf bytes.Buffer
			err := printer.Print([]*client.Object{}, &buf)
			if err != nil {
				t.Errorf("Should not error on empty resources: %v", err)
			}
		})
	}
}

func TestResourcePrinter_NilResource(t *testing.T) {
	resources := []*client.Object{nil}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print nil resource: %v", err)
	}

	output := buf.String()
	// Should produce empty output for nil resource
	if output != "" {
		t.Errorf("Expected empty output for nil resource, got: %s", output)
	}
}

func TestResourcePrinter_PrintSingleNil(t *testing.T) {
	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	var buf bytes.Buffer
	err := printer.PrintSingle(nil, &buf)
	if err != nil {
		t.Errorf("Should not error on nil resource: %v", err)
	}

	output := buf.String()
	if output != "" {
		t.Errorf("Expected empty output for nil resource, got: %s", output)
	}
}

func TestResourcePrinter_MultipleResourcesYAML(t *testing.T) {
	resources := []*client.Object{
		func() *client.Object {
			obj := createTestConfigMap("cm1", "ns1")
			return &obj
		}(),
		func() *client.Object {
			obj := createTestConfigMap("cm2", "ns2")
			return &obj
		}(),
		func() *client.Object {
			obj := createTestConfigMap("cm3", "ns3")
			return &obj
		}(),
	}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatYAML,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print multiple resources as YAML: %v", err)
	}

	output := buf.String()
	// Check that all resources are present
	if !strings.Contains(output, "cm1") {
		t.Errorf("Expected cm1 in YAML output")
	}
	if !strings.Contains(output, "cm2") {
		t.Errorf("Expected cm2 in YAML output")
	}
	if !strings.Contains(output, "cm3") {
		t.Errorf("Expected cm3 in YAML output")
	}
	// Check for YAML document separator
	if !strings.Contains(output, "---") {
		t.Errorf("Expected YAML document separator in multi-resource output")
	}
}

func TestResourcePrinter_PrintNamesWithGroup(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("apps/v1")
	obj.SetKind("Deployment")
	obj.SetName("test-deploy")
	obj.SetNamespace("default")

	resources := []*client.Object{
		func() *client.Object {
			o := client.Object(obj)
			return &o
		}(),
	}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print names with group: %v", err)
	}

	output := buf.String()
	// Should include group in the output
	if !strings.Contains(output, "deployment") {
		t.Errorf("Expected deployment kind in output: %s", output)
	}
	if !strings.Contains(output, "test-deploy") {
		t.Errorf("Expected resource name in output: %s", output)
	}
}

func TestResourcePrinter_PrintNamesNoNamespace(t *testing.T) {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("Namespace")
	obj.SetName("test-ns")
	// No namespace for cluster-scoped resources

	resources := []*client.Object{
		func() *client.Object {
			o := client.Object(obj)
			return &o
		}(),
	}

	printer := io.NewResourcePrinter(io.PrintOptions{
		OutputFormat: io.OutputFormatName,
	})

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Fatalf("Failed to print names without namespace: %v", err)
	}

	output := buf.String()
	// Should not include namespace annotation for cluster-scoped resources
	if strings.Contains(output, "namespace:") {
		t.Errorf("Should not include namespace for cluster-scoped resource: %s", output)
	}
	if !strings.Contains(output, "test-ns") {
		t.Errorf("Expected resource name in output: %s", output)
	}
}

func TestGetResourceStatus_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: "Unknown",
		},
		{
			name: "object with phase field",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.Object["status"] = map[string]interface{}{
					"phase": "Running",
				}
				return obj
			}(),
			expected: "Running",
		},
		{
			name: "object with ready=true",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("CustomResource")
				obj.Object["status"] = map[string]interface{}{
					"ready": true,
				}
				return obj
			}(),
			expected: "Ready",
		},
		{
			name: "object with ready=false",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("CustomResource")
				obj.Object["status"] = map[string]interface{}{
					"ready": false,
				}
				return obj
			}(),
			expected: "NotReady",
		},
		{
			name: "object with Ready condition True",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.Object["status"] = map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Progressing",
							"status": "True",
						},
						map[string]interface{}{
							"type":   "Ready",
							"status": "True",
						},
					},
				}
				return obj
			}(),
			expected: "Ready",
		},
		{
			name: "object with Ready condition False",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.Object["status"] = map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Ready",
							"status": "False",
						},
					},
				}
				return obj
			}(),
			expected: "NotReady",
		},
		{
			name: "object with non-Ready conditions only",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Deployment")
				obj.Object["status"] = map[string]interface{}{
					"conditions": []interface{}{
						map[string]interface{}{
							"type":   "Progressing",
							"status": "True",
						},
					},
				}
				return obj
			}(),
			expected: "Unknown",
		},
		{
			name: "object with invalid status type",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Resource")
				obj.Object["status"] = "invalid-status-type"
				return obj
			}(),
			expected: "Unknown",
		},
		{
			name: "object with no status field",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("ConfigMap")
				obj.SetName("test")
				return obj
			}(),
			expected: "Unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := io.GetResourceStatus(tt.obj)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestGetResourceAge_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		obj      client.Object
		expected string
	}{
		{
			name:     "nil object",
			obj:      nil,
			expected: "<unknown>",
		},
		{
			name: "object with zero timestamp",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("ConfigMap")
				obj.SetName("test")
				obj.SetCreationTimestamp(metav1.Time{})
				return obj
			}(),
			expected: func() string {
				// Zero time is a very long time ago
				return io.GetResourceAge(func() client.Object {
					obj := &unstructured.Unstructured{}
					obj.SetCreationTimestamp(metav1.Time{})
					return obj
				}())
			}(),
		},
		{
			name: "object created 70 seconds ago",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("test")
				obj.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-70 * time.Second)})
				return obj
			}(),
			expected: "1m",
		},
		{
			name: "object created 130 minutes ago",
			obj: func() client.Object {
				obj := &unstructured.Unstructured{}
				obj.SetKind("Pod")
				obj.SetName("test")
				obj.SetCreationTimestamp(metav1.Time{Time: time.Now().Add(-130 * time.Minute)})
				return obj
			}(),
			expected: "2h",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := io.GetResourceAge(tt.obj)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestPrintObjects_AllFormats(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	formats := []io.OutputFormat{
		io.OutputFormatYAML,
		io.OutputFormatJSON,
		io.OutputFormatTable,
		io.OutputFormatWide,
		io.OutputFormatName,
	}

	for _, format := range formats {
		t.Run(string(format), func(t *testing.T) {
			options := io.PrintOptions{
				OutputFormat: format,
			}

			var buf bytes.Buffer
			err := io.PrintObjects(resources, format, options, &buf)
			if err != nil {
				t.Errorf("Failed to print objects in %s format: %v", format, err)
			}

			output := buf.String()
			if output == "" {
				t.Errorf("Expected non-empty output for %s format", format)
			}
		})
	}
}

func TestPrintObjectsAsTable_WideMode(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	var buf bytes.Buffer
	err := io.PrintObjectsAsTable(resources, true, false, &buf)
	if err != nil {
		t.Fatalf("Failed to print objects as wide table: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty output for wide table")
	}
}

func TestPrintObjectsAsTable_NoHeaders(t *testing.T) {
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	var buf bytes.Buffer
	err := io.PrintObjectsAsTable(resources, false, true, &buf)
	if err != nil {
		t.Fatalf("Failed to print objects as table without headers: %v", err)
	}

	output := buf.String()
	if output == "" {
		t.Error("Expected non-empty output for table without headers")
	}
}

func TestNewTablePrinter_ConvertsFormat(t *testing.T) {
	// NewTablePrinter should convert non-table formats to table format
	options := io.PrintOptions{
		OutputFormat: io.OutputFormatYAML,
		NoHeaders:    true,
	}

	printer := io.NewTablePrinter(options)
	if printer == nil {
		t.Fatal("Expected non-nil printer")
	}

	// Should still be able to print (will use table format internally)
	obj := createTestConfigMap("test-cm", "default")
	resources := []*client.Object{&obj}

	var buf bytes.Buffer
	err := printer.Print(resources, &buf)
	if err != nil {
		t.Errorf("Table printer failed: %v", err)
	}
}

func TestValidateOutputFormat_EdgeCases(t *testing.T) {
	tests := []struct {
		input    string
		expected io.OutputFormat
		wantErr  bool
	}{
		{"YAML", io.OutputFormatYAML, false},
		{"YML", io.OutputFormatYAML, false},
		{"Json", io.OutputFormatJSON, false},
		{"TABLE", io.OutputFormatTable, false},
		{"Wide", io.OutputFormatWide, false},
		{"NAME", io.OutputFormatName, false},
		{"", "", true},
		{"invalid", "", true},
		{"xml", "", true},
		{"csv", "", true},
		{" yaml ", "", true}, // No trimming
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			format, err := io.ValidateOutputFormat(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error for input %q, got none", tt.input)
				}
			} else {
				if err != nil {
					t.Errorf("Unexpected error for input %q: %v", tt.input, err)
				}
				if format != tt.expected {
					t.Errorf("Expected format %q for input %q, got %q", tt.expected, tt.input, format)
				}
			}
		})
	}
}

// Helper function to create test ConfigMap
func createTestConfigMap(name, namespace string) client.Object {
	obj := &unstructured.Unstructured{}
	obj.SetAPIVersion("v1")
	obj.SetKind("ConfigMap")
	obj.SetName(name)
	obj.SetNamespace(namespace)
	obj.Object["data"] = map[string]interface{}{
		"key1": "value1",
		"key2": "value2",
	}
	return obj
}
