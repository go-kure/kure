package launcher

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
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
						Object: map[string]interface{}{
							"apiVersion": "apps/v1",
							"kind":       "Deployment",
							"metadata": map[string]interface{}{
								"name":      "test-app",
								"namespace": "default",
							},
							"spec": map[string]interface{}{
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
						Object: map[string]interface{}{
							"apiVersion": "v1",
							"kind":       "Service",
							"metadata": map[string]interface{}{
								"name":      "test-svc",
								"namespace": "default",
							},
							"spec": map[string]interface{}{
								"type": "ClusterIP",
								"ports": []interface{}{
									map[string]interface{}{
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
		var docs []map[string]interface{}
		decoder := yaml.NewDecoder(&buf)
		for {
			var doc map[string]interface{}
			if err := decoder.Decode(&doc); err != nil {
				if err == io.EOF {
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
		var items []map[string]interface{}
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
			Object: map[string]interface{}{
				"kind": "Deployment",
				"metadata": map[string]interface{}{
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
							Object: map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name": "config",
								},
								"data": map[string]interface{}{
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