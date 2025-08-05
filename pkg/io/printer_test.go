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
