package launcher

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-kure/kure/pkg/logger"
)

func TestFilterPatches(t *testing.T) {
	cli := &CLI{logger: logger.Noop()}

	patches := []Patch{
		{Name: "scale"},
		{Name: "labels"},
		{Name: "security"},
		{Name: "network"},
	}

	tests := []struct {
		name     string
		names    []string
		expected int
	}{
		{
			name:     "filter single patch",
			names:    []string{"scale"},
			expected: 1,
		},
		{
			name:     "filter multiple patches",
			names:    []string{"scale", "labels"},
			expected: 2,
		},
		{
			name:     "filter non-existent patch",
			names:    []string{"non-existent"},
			expected: 0,
		},
		{
			name:     "filter empty names",
			names:    []string{},
			expected: 0,
		},
		{
			name:     "filter all patches",
			names:    []string{"scale", "labels", "security", "network"},
			expected: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cli.filterPatches(patches, tt.names)
			if len(result) != tt.expected {
				t.Errorf("expected %d patches, got %d", tt.expected, len(result))
			}
		})
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{
			name:     "simple string",
			input:    "hello",
			expected: "hello",
		},
		{
			name:     "multiline string",
			input:    "line1\nline2\nline3",
			expected: "(multiline)",
		},
		{
			name:     "map",
			input:    map[string]any{"a": 1, "b": 2, "c": 3},
			expected: "(map with 3 keys)",
		},
		{
			name:     "empty map",
			input:    map[string]any{},
			expected: "(map with 0 keys)",
		},
		{
			name:     "array",
			input:    []any{"a", "b"},
			expected: "(array with 2 items)",
		},
		{
			name:     "empty array",
			input:    []any{},
			expected: "(array with 0 items)",
		},
		{
			name:     "integer",
			input:    42,
			expected: "42",
		},
		{
			name:     "bool",
			input:    true,
			expected: "true",
		},
		{
			name:     "nil",
			input:    nil,
			expected: "<nil>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatValue(tt.input)
			if got != tt.expected {
				t.Errorf("formatValue(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestNewCLINilLogger(t *testing.T) {
	cli := NewCLI(nil)
	if cli == nil {
		t.Fatal("expected non-nil CLI when passing nil logger")
	}
}

func TestRunSchemaToFile(t *testing.T) {
	log := logger.Noop()
	cli := NewCLI(log)

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "schema.json")

	var buf bytes.Buffer
	cli.SetOutputWriter(&buf)

	err := cli.runSchema(context.Background(), ".", outPath, false, true, DefaultOptions())
	if err != nil {
		t.Fatalf("runSchema to file failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if len(data) == 0 {
		t.Error("schema file should not be empty")
	}
}

func TestRunSchemaToFileNoPretty(t *testing.T) {
	log := logger.Noop()
	cli := NewCLI(log)

	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "schema.json")

	var buf bytes.Buffer
	cli.SetOutputWriter(&buf)

	err := cli.runSchema(context.Background(), ".", outPath, false, false, DefaultOptions())
	if err != nil {
		t.Fatalf("runSchema to file (no pretty) failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	if len(data) == 0 {
		t.Error("schema file should not be empty")
	}
}

func TestRunSchemaToStdout(t *testing.T) {
	log := logger.Noop()
	cli := NewCLI(log)

	var buf bytes.Buffer
	cli.SetOutputWriter(&buf)

	err := cli.runSchema(context.Background(), ".", "", false, true, DefaultOptions())
	if err != nil {
		t.Fatalf("runSchema to stdout failed: %v", err)
	}

	if buf.Len() == 0 {
		t.Error("expected schema output on stdout")
	}
}
