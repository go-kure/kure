package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/logger"
)

func TestBuilder(t *testing.T) {
	log := logger.Noop()
	builder := NewBuilder(log)
	ctx := context.Background()

	// Create test package instance
	instance := &PackageInstance{
		Definition: &PackageDefinition{
			Path: "/test/path",
			Metadata: KurelMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Parameters: ParameterMap{
				"replicas": 3,
				"image":    "nginx:latest",
			},
			Resources: []Resource{
				{
					APIVersion: "apps/v1",
					Kind:       "Deployment",
					Metadata: metav1.ObjectMeta{
						Name:      "test-app",
						Namespace: "default",
					},
					Raw: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]any{
								"name":      "test-app",
								"namespace": "default",
							},
							"spec": map[string]any{
								"replicas": int64(3),
							},
						},
					},
				},
				{
					APIVersion: "v1",
					Kind:       "Service",
					Metadata: metav1.ObjectMeta{
						Name:      "test-svc",
						Namespace: "default",
					},
					Raw: &unstructured.Unstructured{
						Object: map[string]any{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata": map[string]any{
								"name":      "test-svc",
								"namespace": "default",
							},
							"spec": map[string]any{
								"type": "ClusterIP",
								"ports": []any{
									map[string]any{
										"port":       int64(80),
										"targetPort": int64(8080),
									},
								},
							},
						},
					},
				},
			},
		},
		UserValues: ParameterMap{},
	}

	t.Run("build to stdout YAML", func(t *testing.T) {
		var buf bytes.Buffer

		// Set the builder's output writer to our buffer
		builder.SetOutputWriter(&buf)

		buildOpts := BuildOptions{
			Output: OutputStdout,
			Format: FormatYAML,
		}

		err := builder.Build(ctx, instance, buildOpts, nil)
		assert.NoError(t, err)

		// Check output contains both resources
		output := buf.String()
		assert.Contains(t, output, "kind: Deployment")
		assert.Contains(t, output, "kind: Service")
	})

	t.Run("build to memory YAML", func(t *testing.T) {
		// Create a mock builder that writes to buffer
		mockBuilder := &outputBuilder{
			logger:    log,
			writer:    &mockFileWriter{},
			resolver:  NewResolver(log),
			processor: NewPatchProcessor(log, NewResolver(log)),
		}

		var buf bytes.Buffer
		buildOpts := BuildOptions{
			Output: OutputStdout,
			Format: FormatYAML,
		}

		// Test YAML output
		err := mockBuilder.writeYAML(&buf, convertResources(instance.Definition.Resources), buildOpts)
		require.NoError(t, err)

		// Parse YAML to verify structure
		var docs []map[string]any
		decoder := yaml.NewDecoder(&buf)
		for {
			var doc map[string]any
			if err := decoder.Decode(&doc); err != nil {
				if errors.Is(err, io.EOF) {
					break
				}
				t.Fatalf("Failed to decode YAML: %v", err)
			}
			if len(doc) > 0 { // Skip empty documents
				docs = append(docs, doc)
			}
		}

		assert.Len(t, docs, 2, "Expected 2 documents in YAML output")
		if len(docs) >= 2 {
			assert.Equal(t, "Deployment", docs[0]["kind"])
			assert.Equal(t, "Service", docs[1]["kind"])
		}
	})

	t.Run("build to memory JSON", func(t *testing.T) {
		mockBuilder := &outputBuilder{
			logger:    log,
			writer:    &mockFileWriter{},
			resolver:  NewResolver(log),
			processor: NewPatchProcessor(log, NewResolver(log)),
		}

		var buf bytes.Buffer
		buildOpts := BuildOptions{
			Output:      OutputStdout,
			Format:      FormatJSON,
			PrettyPrint: true,
		}

		// Test JSON output
		err := mockBuilder.writeJSON(&buf, convertResources(instance.Definition.Resources), buildOpts)
		require.NoError(t, err)

		// Parse JSON to verify structure
		var items []map[string]any
		err = json.Unmarshal(buf.Bytes(), &items)
		require.NoError(t, err)

		assert.Len(t, items, 2)
		assert.Equal(t, "Deployment", items[0]["kind"])
		assert.Equal(t, "Service", items[1]["kind"])
	})

	t.Run("build with filters", func(t *testing.T) {
		buildOpts := BuildOptions{
			Output:     OutputStdout,
			Format:     FormatYAML,
			FilterKind: "Deployment",
		}

		mockBuilder := &outputBuilder{
			logger:    log,
			writer:    &mockFileWriter{},
			resolver:  NewResolver(log),
			processor: NewPatchProcessor(log, NewResolver(log)),
		}

		resources, err := mockBuilder.buildResources(ctx, instance.Definition, ParameterMap{}, buildOpts)
		require.NoError(t, err)

		assert.Len(t, resources, 1)
		assert.Equal(t, "Deployment", resources[0].GetKind())
	})

	t.Run("build with labels and annotations", func(t *testing.T) {
		buildOpts := BuildOptions{
			Output: OutputStdout,
			Format: FormatYAML,
			AddLabels: map[string]string{
				"env":     "test",
				"version": "v1",
			},
			AddAnnotations: map[string]string{
				"managed-by": "kurel",
			},
		}

		mockBuilder := &outputBuilder{
			logger:    log,
			writer:    &mockFileWriter{},
			resolver:  NewResolver(log),
			processor: NewPatchProcessor(log, NewResolver(log)),
		}

		resources, err := mockBuilder.buildResources(ctx, instance.Definition, ParameterMap{}, buildOpts)
		require.NoError(t, err)

		for _, res := range resources {
			labels := res.GetLabels()
			assert.Equal(t, "test", labels["env"])
			assert.Equal(t, "v1", labels["version"])

			annotations := res.GetAnnotations()
			assert.Equal(t, "kurel", annotations["managed-by"])
		}
	})

	t.Run("generate filename", func(t *testing.T) {
		mockBuilder := &outputBuilder{
			logger: log,
		}

		resource := &unstructured.Unstructured{
			Object: map[string]any{
				"kind": "Deployment",
				"metadata": map[string]any{
					"name":      "test-app",
					"namespace": "production",
				},
			},
		}

		testCases := []struct {
			name     string
			opts     BuildOptions
			expected string
		}{
			{
				name: "basic",
				opts: BuildOptions{
					Format: FormatYAML,
				},
				expected: "deployment-test-app.yaml",
			},
			{
				name: "with index",
				opts: BuildOptions{
					Format:       FormatYAML,
					IncludeIndex: true,
				},
				expected: "005-deployment-test-app.yaml",
			},
			{
				name: "with namespace",
				opts: BuildOptions{
					Format:           FormatYAML,
					IncludeNamespace: true,
				},
				expected: "deployment-test-app-production.yaml",
			},
			{
				name: "JSON format",
				opts: BuildOptions{
					Format: FormatJSON,
				},
				expected: "deployment-test-app.json",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				filename := mockBuilder.generateFilename(resource, 5, tc.opts)
				assert.Equal(t, tc.expected, filename)
			})
		}
	})
}

func TestBuilderDirectory(t *testing.T) {
	log := logger.Noop()

	t.Run("write to directory", func(t *testing.T) {
		// Create temp directory
		tmpDir := t.TempDir()

		instance := &PackageInstance{
			Definition: &PackageDefinition{
				Resources: []Resource{
					{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Metadata: metav1.ObjectMeta{
							Name: "config",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]any{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]any{
									"name": "config",
								},
								"data": map[string]any{
									"key": "value",
								},
							},
						},
					},
				},
			},
		}

		buildOpts := BuildOptions{
			Output:       OutputDirectory,
			OutputPath:   tmpDir,
			Format:       FormatYAML,
			IncludeIndex: true,
		}

		builder := NewBuilder(log)
		err := builder.Build(context.Background(), instance, buildOpts, nil)
		require.NoError(t, err)

		// Check file was created
		files, err := os.ReadDir(tmpDir)
		require.NoError(t, err)
		assert.Len(t, files, 1)
		assert.True(t, strings.HasPrefix(files[0].Name(), "000-configmap"))
		assert.True(t, strings.HasSuffix(files[0].Name(), ".yaml"))

		// Read and verify content
		content, err := os.ReadFile(filepath.Join(tmpDir, files[0].Name()))
		require.NoError(t, err)
		assert.Contains(t, string(content), "kind: ConfigMap")
		assert.Contains(t, string(content), "name: config")
	})
}

// Helper functions

func convertResources(resources []Resource) []*unstructured.Unstructured {
	var result []*unstructured.Unstructured
	for _, r := range resources {
		if r.Raw != nil {
			result = append(result, r.Raw)
		}
	}
	return result
}

type mockFileWriter struct {
	files map[string][]byte
	dirs  map[string]bool
}

func (w *mockFileWriter) WriteFile(path string, data []byte) error {
	if w.files == nil {
		w.files = make(map[string][]byte)
	}
	w.files[path] = data
	return nil
}

func (w *mockFileWriter) MkdirAll(path string) error {
	if w.dirs == nil {
		w.dirs = make(map[string]bool)
	}
	w.dirs[path] = true
	return nil
}

func TestNewBuilderNilLogger(t *testing.T) {
	builder := NewBuilder(nil)
	if builder == nil {
		t.Fatal("expected non-nil builder when passing nil logger")
	}
}

func TestDefaultFileWriterWriteFile(t *testing.T) {
	tmpDir := t.TempDir()
	w := &defaultFileWriter{}

	testPath := filepath.Join(tmpDir, "test.txt")
	data := []byte("hello world")

	err := w.WriteFile(testPath, data)
	if err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	got, err := os.ReadFile(testPath)
	if err != nil {
		t.Fatalf("failed to read written file: %v", err)
	}
	if string(got) != "hello world" {
		t.Errorf("expected 'hello world', got %q", string(got))
	}
}

func TestConvertToString(t *testing.T) {
	log := logger.Noop()
	b := &outputBuilder{logger: log}

	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{name: "string", input: "hello", expected: "hello"},
		{name: "int", input: 42, expected: "42"},
		{name: "int64", input: int64(100), expected: "100"},
		{name: "uint", input: uint(7), expected: "7"},
		{name: "uint64", input: uint64(99), expected: "99"},
		{name: "float64", input: 3.14, expected: "3.14"},
		{name: "float32", input: float32(2.5), expected: "2.5"},
		{name: "bool true", input: true, expected: "true"},
		{name: "bool false", input: false, expected: "false"},
		{name: "nil", input: nil, expected: "null"},
		{name: "slice of strings", input: []string{"a", "b"}, expected: "[a b]"},
		{name: "map[string]any", input: map[string]any{"key": "val"}, expected: "key: val"},
		{name: "[]any", input: []any{"a", "b"}, expected: "- a\n- b"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := b.convertToString(tt.input)
			if got != tt.expected {
				t.Errorf("convertToString(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestResolveVariablePath(t *testing.T) {
	log := logger.Noop()
	b := &outputBuilder{logger: log}

	t.Run("simple key in ParameterMap", func(t *testing.T) {
		params := ParameterMap{"name": "test-app"}
		val, err := b.resolveVariablePath("name", params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "test-app" {
			t.Errorf("expected 'test-app', got %v", val)
		}
	})

	t.Run("nested key in ParameterMap", func(t *testing.T) {
		params := ParameterMap{
			"app": map[string]any{
				"name": "nested-app",
			},
		}
		val, err := b.resolveVariablePath("app.name", params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "nested-app" {
			t.Errorf("expected 'nested-app', got %v", val)
		}
	})

	t.Run("nested key in map[string]interface{}", func(t *testing.T) {
		params := ParameterMap{
			"config": map[string]any{
				"db": map[string]any{
					"host": "localhost",
				},
			},
		}
		val, err := b.resolveVariablePath("config.db.host", params)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if val != "localhost" {
			t.Errorf("expected 'localhost', got %v", val)
		}
	})

	t.Run("missing key", func(t *testing.T) {
		params := ParameterMap{"name": "test"}
		_, err := b.resolveVariablePath("missing", params)
		if err == nil {
			t.Fatal("expected error for missing key")
		}
	})

	t.Run("traverse non-map value", func(t *testing.T) {
		params := ParameterMap{"name": "test"}
		_, err := b.resolveVariablePath("name.sub", params)
		if err == nil {
			t.Fatal("expected error when traversing non-map value")
		}
	})
}

func TestWriteJSONSingleResource(t *testing.T) {
	log := logger.Noop()
	b := &outputBuilder{
		logger: log,
		writer: &mockFileWriter{},
	}

	resource := &unstructured.Unstructured{
		Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "ConfigMap",
			"metadata": map[string]any{
				"name": "test",
			},
		},
	}

	var buf bytes.Buffer
	err := b.writeJSON(&buf, []*unstructured.Unstructured{resource}, BuildOptions{
		Format:      FormatJSON,
		PrettyPrint: false,
	})
	if err != nil {
		t.Fatalf("writeJSON failed: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("expected valid JSON, got: %v", err)
	}
	if result["kind"] != "ConfigMap" {
		t.Errorf("expected kind ConfigMap, got %v", result["kind"])
	}
}

func TestWriteDirectoryJSON(t *testing.T) {
	log := logger.Noop()
	tmpDir := t.TempDir()

	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata": map[string]any{
					"name": "test-cm",
				},
			},
		},
	}

	err := b.writeDirectory(context.Background(), resources, BuildOptions{
		OutputPath: tmpDir,
		Format:     FormatJSON,
	})
	if err != nil {
		t.Fatalf("writeDirectory failed: %v", err)
	}

	files, err := os.ReadDir(tmpDir)
	if err != nil {
		t.Fatalf("failed to read dir: %v", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}
	if !strings.HasSuffix(files[0].Name(), ".json") {
		t.Errorf("expected .json suffix, got %s", files[0].Name())
	}
}

func TestWriteDirectoryEmptyPath(t *testing.T) {
	log := logger.Noop()
	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	err := b.writeDirectory(context.Background(), nil, BuildOptions{
		OutputPath: "",
		Format:     FormatYAML,
	})
	if err == nil {
		t.Fatal("expected error for empty output path")
	}
}

func TestWriteDirectoryContextCancelled(t *testing.T) {
	log := logger.Noop()
	tmpDir := t.TempDir()

	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "test"},
			},
		},
	}

	err := b.writeDirectory(ctx, resources, BuildOptions{
		OutputPath: tmpDir,
		Format:     FormatYAML,
	})
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWriteOutputUnsupportedType(t *testing.T) {
	log := logger.Noop()
	b := &outputBuilder{
		logger:       log,
		writer:       &defaultFileWriter{},
		outputWriter: &bytes.Buffer{},
	}

	err := b.writeOutput(context.Background(), nil, BuildOptions{
		Output: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for unsupported output type")
	}
}

func TestWriteOutputUnsupportedFormat(t *testing.T) {
	log := logger.Noop()
	var buf bytes.Buffer
	b := &outputBuilder{
		logger:       log,
		writer:       &defaultFileWriter{},
		outputWriter: &buf,
	}

	err := b.writeOutput(context.Background(), nil, BuildOptions{
		Output: OutputStdout,
		Format: "invalid",
	})
	if err == nil {
		t.Fatal("expected error for unsupported format")
	}
}

func TestWriteOutputToFile(t *testing.T) {
	log := logger.Noop()
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output.yaml")

	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "test"},
			},
		},
	}

	err := b.writeOutput(context.Background(), resources, BuildOptions{
		Output:     OutputFile,
		OutputPath: outPath,
		Format:     FormatYAML,
	})
	if err != nil {
		t.Fatalf("writeOutput to file failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}
	if !strings.Contains(string(data), "kind: ConfigMap") {
		t.Error("output file should contain ConfigMap")
	}
}

func TestWriteOutputToFileJSON(t *testing.T) {
	log := logger.Noop()
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "output.json")

	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "test"},
			},
		},
	}

	err := b.writeOutput(context.Background(), resources, BuildOptions{
		Output:     OutputFile,
		OutputPath: outPath,
		Format:     FormatJSON,
	})
	if err != nil {
		t.Fatalf("writeOutput to file JSON failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output file: %v", err)
	}

	var result map[string]any
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatalf("output should be valid JSON: %v", err)
	}
}

func TestWriteOutputFileNoPath(t *testing.T) {
	log := logger.Noop()
	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	err := b.writeOutput(context.Background(), nil, BuildOptions{
		Output:     OutputFile,
		OutputPath: "",
		Format:     FormatYAML,
	})
	if err == nil {
		t.Fatal("expected error for file output without path")
	}
}

func TestBuildNilInstance(t *testing.T) {
	log := logger.Noop()
	builder := NewBuilder(log)
	ctx := context.Background()

	err := builder.Build(ctx, nil, BuildOptions{}, nil)
	if err == nil {
		t.Fatal("expected error for nil instance")
	}

	err = builder.Build(ctx, &PackageInstance{}, BuildOptions{}, nil)
	if err == nil {
		t.Fatal("expected error for nil definition")
	}
}

func TestBuildContextCancelled(t *testing.T) {
	log := logger.Noop()
	builder := NewBuilder(log)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	instance := &PackageInstance{
		Definition: &PackageDefinition{
			Metadata: KurelMetadata{Name: "test"},
		},
	}

	err := builder.Build(ctx, instance, BuildOptions{}, nil)
	if err == nil {
		t.Fatal("expected error for cancelled context")
	}
}

func TestWriteDirectoryUnsupportedFormat(t *testing.T) {
	log := logger.Noop()
	tmpDir := t.TempDir()

	b := &outputBuilder{
		logger: log,
		writer: &defaultFileWriter{},
	}

	resources := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
				"metadata":   map[string]any{"name": "test"},
			},
		},
	}

	err := b.writeDirectory(context.Background(), resources, BuildOptions{
		OutputPath: tmpDir,
		Format:     "invalid",
	})
	if err == nil {
		t.Fatal("expected error for unsupported format in writeDirectory")
	}
}
