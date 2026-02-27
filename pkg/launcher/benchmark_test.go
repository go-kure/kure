package launcher

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/logger"
)

// BenchmarkVariableResolution benchmarks variable resolution performance
func BenchmarkVariableResolution(b *testing.B) {
	log := logger.Noop()
	resolver := NewResolver(log)
	ctx := context.Background()

	// Create test data with nested variables
	base := ParameterMap{
		"app": map[string]interface{}{
			"name":    "${base.name}-app",
			"version": "${base.version}",
			"image":   "${registry}/${app.name}:${app.version}",
		},
		"base": map[string]interface{}{
			"name":    "test",
			"version": "1.0.0",
		},
		"registry": "docker.io",
		"replicas": 3,
	}

	overrides := ParameterMap{}
	opts := DefaultOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.Resolve(ctx, base, overrides, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkPatchApplication benchmarks patch application performance
func BenchmarkPatchApplication(b *testing.B) {
	log := logger.Noop()
	resolver := NewResolver(log)
	processor := NewPatchProcessor(log, resolver)
	ctx := context.Background()

	// Create test package with resources
	def := &PackageDefinition{
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
							"replicas": int64(1),
						},
					},
				},
			},
		},
	}

	patches := []Patch{
		{
			Name: "scale",
			Content: `[deployment.test-app]
spec.replicas: 3`,
		},
		{
			Name: "labels",
			Content: `[deployment.test-app]
metadata.labels.app: test
metadata.labels.env: prod`,
		},
	}

	params := ParameterMap{}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := processor.ApplyPatches(ctx, def, patches, params)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkSchemaGeneration benchmarks schema generation performance
func BenchmarkSchemaGeneration(b *testing.B) {
	log := logger.Noop()
	generator := NewSchemaGenerator(log)
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := generator.GeneratePackageSchema(ctx)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkValidation benchmarks package validation performance
func BenchmarkValidation(b *testing.B) {
	log := logger.Noop()
	validator := NewValidator(log)
	ctx := context.Background()

	def := &PackageDefinition{
		Path: "/test/path",
		Metadata: KurelMetadata{
			Name:    "benchmark-package",
			Version: "1.0.0",
		},
		Parameters: ParameterMap{
			"replicas": 3,
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
							"selector": map[string]interface{}{
								"matchLabels": map[string]interface{}{
									"app": "test",
								},
							},
							"template": map[string]interface{}{
								"spec": map[string]interface{}{
									"containers": []interface{}{
										map[string]interface{}{
											"name":  "app",
											"image": "nginx:latest",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := validator.ValidatePackage(ctx, def)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkDeepCopy benchmarks deep copy performance for thread safety
func BenchmarkDeepCopy(b *testing.B) {
	// Create a complex package definition
	def := &PackageDefinition{
		Path: "/test/path",
		Metadata: KurelMetadata{
			Name:    "test-package",
			Version: "1.0.0",
		},
		Parameters: ParameterMap{
			"nested": map[string]interface{}{
				"level1": map[string]interface{}{
					"level2": map[string]interface{}{
						"value": "deep",
					},
				},
			},
		},
		Resources: make([]Resource, 10),
		Patches:   make([]Patch, 5),
	}

	// Initialize resources
	for i := range def.Resources {
		def.Resources[i] = Resource{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Metadata: metav1.ObjectMeta{
				Name: "config",
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = def.DeepCopy()
	}
}

// BenchmarkCycleDetection benchmarks cycle detection in variable resolution
func BenchmarkCycleDetection(b *testing.B) {
	log := logger.Noop()
	resolver := NewResolver(log)
	ctx := context.Background()

	// Create parameters with potential cycles
	base := ParameterMap{
		"a": "${b}",
		"b": "${c}",
		"c": "${d}",
		"d": "${e}",
		"e": "value", // No cycle
		"f": "${g}",
		"g": "${h}",
		"h": "${i}",
		"i": "${j}",
		"j": "another",
	}

	overrides := ParameterMap{}
	opts := DefaultOptions()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := resolver.Resolve(ctx, base, overrides, opts)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFieldUsageTracing benchmarks field usage tracing in schema generation
func BenchmarkFieldUsageTracing(b *testing.B) {
	log := logger.Noop()

	// Create schema generator that implements field usage tracing
	// Since TraceFieldUsage is part of the interface, we need to ensure
	// the implementation exists
	generator, ok := NewSchemaGenerator(log).(interface {
		TraceFieldUsage(resources []Resource) map[string][]string
	})
	if !ok {
		b.Skip("TraceFieldUsage not implemented yet")
	}

	// Create resources with variable references
	resources := make([]Resource, 20)
	for i := range resources {
		resources[i] = Resource{
			APIVersion: "v1",
			Kind:       "ConfigMap",
			Raw: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "v1",
					"kind":       "ConfigMap",
					"data": map[string]interface{}{
						"config": "${app.config}",
						"name":   "${app.name}",
						"env":    "${environment}",
					},
				},
			},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generator.TraceFieldUsage(resources)
	}
}
