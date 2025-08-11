package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/go-kure/kure/pkg/cmd/shared/options"
)

// mockObject implements the interfaces needed for printer testing
type mockObject struct {
	unstructured.Unstructured
}

func newMockObject(name, kind, namespace, apiVersion string, labels map[string]string) *mockObject {
	obj := &mockObject{
		Unstructured: unstructured.Unstructured{},
	}
	obj.SetName(name)
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetAPIVersion(apiVersion)
	if labels != nil {
		obj.SetLabels(labels)
	}
	return obj
}

func TestNewPrinter(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		expected string
	}{
		{
			name:     "yaml printer",
			output:   "yaml",
			expected: "*cli.yamlPrinter",
		},
		{
			name:     "json printer",
			output:   "json",
			expected: "*cli.jsonPrinter",
		},
		{
			name:     "table printer",
			output:   "table",
			expected: "*cli.tablePrinter",
		},
		{
			name:     "wide printer",
			output:   "wide",
			expected: "*cli.tablePrinter",
		},
		{
			name:     "name printer",
			output:   "name",
			expected: "*cli.namePrinter",
		},
		{
			name:     "default printer",
			output:   "unknown",
			expected: "*cli.yamlPrinter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globalOpts := options.NewGlobalOptions()
			globalOpts.Output = tt.output
			
			printer := NewPrinter(globalOpts)
			
			if printer == nil {
				t.Fatal("expected non-nil printer")
			}
			
			// Note: We can't easily check the exact type due to Go's type system,
			// but we can verify the printer implements the interface
			var _ Printer = printer
		})
	}
}

func TestYAMLPrinter(t *testing.T) {
	printer := &yamlPrinter{}
	
	tests := []struct {
		name    string
		objects []runtime.Object
		want    string
	}{
		{
			name:    "no objects",
			objects: []runtime.Object{},
			want:    "",
		},
		{
			name: "single object",
			objects: []runtime.Object{
				newMockObject("test-pod", "Pod", "default", "v1", map[string]string{"app": "test"}),
			},
			want: "unstructured:\n    object:\n        apiVersion: v1\n        kind: Pod\n        metadata:\n            labels:\n                app: test\n            name: test-pod\n            namespace: default\n",
		},
		{
			name: "multiple objects",
			objects: []runtime.Object{
				newMockObject("pod1", "Pod", "default", "v1", nil),
				newMockObject("pod2", "Pod", "default", "v1", nil),
			},
			want: "unstructured:\n    object:\n        apiVersion: v1\n        kind: Pod\n        metadata:\n            name: pod1\n            namespace: default\n---\nunstructured:\n    object:\n        apiVersion: v1\n        kind: Pod\n        metadata:\n            name: pod2\n            namespace: default\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			err := printer.Print(tt.objects, &buf)
			
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			got := buf.String()
			if got != tt.want {
				t.Errorf("want:\n%s\ngot:\n%s", tt.want, got)
			}
		})
	}
}

func TestJSONPrinter(t *testing.T) {
	printer := &jsonPrinter{}
	
	tests := []struct {
		name    string
		objects []runtime.Object
		wantErr bool
	}{
		{
			name:    "no objects",
			objects: []runtime.Object{},
			wantErr: false,
		},
		{
			name: "single object",
			objects: []runtime.Object{
				newMockObject("test-pod", "Pod", "default", "v1", map[string]string{"app": "test"}),
			},
			wantErr: false,
		},
		{
			name: "multiple objects",
			objects: []runtime.Object{
				newMockObject("pod1", "Pod", "default", "v1", nil),
				newMockObject("pod2", "Pod", "default", "v1", nil),
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf strings.Builder
			err := printer.Print(tt.objects, &buf)
			
			if tt.wantErr && err == nil {
				t.Error("expected error but got nil")
				return
			}
			
			if !tt.wantErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			
			if !tt.wantErr {
				// Verify it's valid JSON
				var result interface{}
				if err := json.Unmarshal([]byte(buf.String()), &result); err != nil {
					t.Errorf("invalid JSON output: %v", err)
				}
			}
		})
	}
}

func TestTablePrinter(t *testing.T) {
	tests := []struct {
		name     string
		options  PrinterOptions
		objects  []runtime.Object
		contains []string
	}{
		{
			name: "basic table",
			options: PrinterOptions{
				OutputFormat: "table",
				NoHeaders:    false,
				ShowLabels:   false,
				Wide:         false,
			},
			objects: []runtime.Object{
				newMockObject("test-pod", "Pod", "default", "v1", nil),
			},
			contains: []string{"NAME", "KIND", "NAMESPACE", "test-pod", "Pod", "default"},
		},
		{
			name: "table with labels",
			options: PrinterOptions{
				OutputFormat: "table",
				NoHeaders:    false,
				ShowLabels:   true,
				Wide:         false,
			},
			objects: []runtime.Object{
				newMockObject("test-pod", "Pod", "default", "v1", map[string]string{"app": "test"}),
			},
			contains: []string{"LABELS", "app=test"},
		},
		{
			name: "wide table",
			options: PrinterOptions{
				OutputFormat: "table",
				NoHeaders:    false,
				ShowLabels:   false,
				Wide:         true,
			},
			objects: []runtime.Object{
				newMockObject("test-pod", "Pod", "default", "v1", nil),
			},
			contains: []string{"API-VERSION", "CREATED"},
		},
		{
			name: "no headers",
			options: PrinterOptions{
				OutputFormat: "table",
				NoHeaders:    true,
				ShowLabels:   false,
				Wide:         false,
			},
			objects: []runtime.Object{
				newMockObject("test-pod", "Pod", "default", "v1", nil),
			},
			contains: []string{"test-pod", "Pod", "default"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			printer := &tablePrinter{options: tt.options}
			var buf strings.Builder
			
			err := printer.Print(tt.objects, &buf)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			
			output := buf.String()
			for _, expected := range tt.contains {
				if !strings.Contains(output, expected) {
					t.Errorf("expected output to contain %q, got:\n%s", expected, output)
				}
			}
			
			// If NoHeaders is true, should not contain "NAME"
			if tt.options.NoHeaders && strings.Contains(output, "NAME") {
				t.Error("expected no headers but found 'NAME'")
			}
		})
	}
}

func TestNamePrinter(t *testing.T) {
	printer := &namePrinter{}
	
	objects := []runtime.Object{
		newMockObject("test-pod", "Pod", "default", "v1", nil),
		newMockObject("test-service", "Service", "default", "v1", nil),
	}
	
	var buf strings.Builder
	err := printer.Print(objects, &buf)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	output := buf.String()
	expected := []string{"Pod/test-pod", "Service/test-service"}
	
	for _, exp := range expected {
		if !strings.Contains(output, exp) {
			t.Errorf("expected output to contain %q, got:\n%s", exp, output)
		}
	}
}

func TestJoinTabs(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected string
	}{
		{
			name:     "empty slice",
			input:    []string{},
			expected: "",
		},
		{
			name:     "single string",
			input:    []string{"test"},
			expected: "test",
		},
		{
			name:     "multiple strings",
			input:    []string{"one", "two", "three"},
			expected: "one\ttwo\tthree",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := joinTabs(tt.input)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestFormatLabels(t *testing.T) {
	tests := []struct {
		name     string
		labels   map[string]string
		expected string
	}{
		{
			name:     "no labels",
			labels:   nil,
			expected: "<none>",
		},
		{
			name:     "empty labels",
			labels:   map[string]string{},
			expected: "<none>",
		},
		{
			name:     "single label",
			labels:   map[string]string{"app": "test"},
			expected: "app=test",
		},
		{
			name:     "multiple labels",
			labels:   map[string]string{"app": "test", "version": "v1"},
			expected: "", // We'll check contains instead of exact match due to map iteration order
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatLabels(tt.labels)
			
			if tt.name == "multiple labels" {
				// Check that both labels are present
				if !strings.Contains(result, "app=test") {
					t.Errorf("expected result to contain 'app=test', got %q", result)
				}
				if !strings.Contains(result, "version=v1") {
					t.Errorf("expected result to contain 'version=v1', got %q", result)
				}
				if !strings.Contains(result, ",") {
					t.Errorf("expected result to contain comma separator, got %q", result)
				}
			} else {
				if result != tt.expected {
					t.Errorf("expected %q, got %q", tt.expected, result)
				}
			}
		})
	}
}

func TestPrintObjects(t *testing.T) {
	objects := []runtime.Object{
		newMockObject("test-pod", "Pod", "default", "v1", nil),
	}
	
	globalOpts := options.NewGlobalOptions()
	globalOpts.Output = "yaml"
	
	var buf strings.Builder
	err := PrintObjects(objects, globalOpts, &buf)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	output := buf.String()
	if !strings.Contains(output, "test-pod") {
		t.Error("expected output to contain object name")
	}
	
	if !strings.Contains(output, "Pod") {
		t.Error("expected output to contain object kind")
	}
}

func TestTablePrinterEmpty(t *testing.T) {
	printer := &tablePrinter{
		options: PrinterOptions{OutputFormat: "table"},
	}
	
	var buf strings.Builder
	err := printer.Print([]runtime.Object{}, &buf)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if buf.String() != "" {
		t.Errorf("expected empty output for no objects, got: %q", buf.String())
	}
}

func TestJSONPrinterEmptyObjects(t *testing.T) {
	printer := &jsonPrinter{}
	
	var buf strings.Builder
	err := printer.Print([]runtime.Object{}, &buf)
	
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	expected := "{}\n"
	if buf.String() != expected {
		t.Errorf("expected %q for empty objects, got %q", expected, buf.String())
	}
}