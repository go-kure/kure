package launcher

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/logger"
)

func TestValidator(t *testing.T) {
	log := logger.Noop()
	validator := NewValidator(log)
	ctx := context.Background()

	t.Run("ValidatePackage", func(t *testing.T) {
		t.Run("valid package", func(t *testing.T) {
			def := &PackageDefinition{
				Path: "/test/path",
				Metadata: KurelMetadata{
					Name:    "test-package",
					Version: "1.0.0",
				},
				Parameters: ParameterMap{
					"replicas": int64(3),
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
										"metadata": map[string]interface{}{
											"labels": map[string]interface{}{
												"app": "test",
											},
										},
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
				Patches: []Patch{
					{
						Name:    "scale",
						Content: "spec.replicas: ${replicas}",
					},
				},
			}

			result, err := validator.ValidatePackage(ctx, def)
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.True(t, result.IsValid())
			assert.Empty(t, result.Errors)
		})

		t.Run("missing required fields", func(t *testing.T) {
			def := &PackageDefinition{
				// Missing required metadata fields (version)
				Path: "/test/path",
				Metadata: KurelMetadata{
					Name: "test-package",
					// Missing Version field
				},
			}

			result, err := validator.ValidatePackage(ctx, def)
			require.NoError(t, err)
			require.NotNil(t, result)

			// Debug output
			if len(result.Errors) == 0 {
				t.Logf("No errors found but expected validation errors")
			}
			for _, e := range result.Errors {
				t.Logf("Error: %s: %s", e.Path, e.Message)
			}
			for _, w := range result.Warnings {
				t.Logf("Warning: %s: %s", w.Field, w.Message)
			}

			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)
		})

		t.Run("duplicate resources", func(t *testing.T) {
			def := &PackageDefinition{
				Path: "/test/path",
				Metadata: KurelMetadata{
					Name: "test-package",
				},
				Resources: []Resource{
					{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Metadata: metav1.ObjectMeta{
							Name:      "config",
							Namespace: "default",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name":      "config",
									"namespace": "default",
								},
							},
						},
					},
					{
						APIVersion: "v1",
						Kind:       "ConfigMap",
						Metadata: metav1.ObjectMeta{
							Name:      "config", // Duplicate
							Namespace: "default",
						},
						Raw: &unstructured.Unstructured{
							Object: map[string]interface{}{
								"apiVersion": "v1",
								"kind":       "ConfigMap",
								"metadata": map[string]interface{}{
									"name":      "config",
									"namespace": "default",
								},
							},
						},
					},
				},
			}

			result, err := validator.ValidatePackage(ctx, def)
			require.NoError(t, err)

			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)

			// Check for duplicate error
			found := false
			for _, e := range result.Errors {
				if contains(e.Message, "duplicate") {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected duplicate resource error")
		})

		t.Run("circular patch dependencies", func(t *testing.T) {
			def := &PackageDefinition{
				Path: "/test/path",
				Metadata: KurelMetadata{
					Name: "test-package",
				},
				Patches: []Patch{
					{
						Name:    "patch1",
						Content: "spec.foo: bar",
						Metadata: &PatchMetadata{
							Requires: []string{"patch2"},
						},
					},
					{
						Name:    "patch2",
						Content: "spec.bar: baz",
						Metadata: &PatchMetadata{
							Requires: []string{"patch3"},
						},
					},
					{
						Name:    "patch3",
						Content: "spec.baz: foo",
						Metadata: &PatchMetadata{
							Requires: []string{"patch1"}, // Creates cycle
						},
					},
				},
			}

			result, err := validator.ValidatePackage(ctx, def)
			require.NoError(t, err)

			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)

			// Check for circular dependency error
			found := false
			for _, e := range result.Errors {
				if contains(e.Message, "circular") {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected circular dependency error")
		})

		t.Run("invalid patch dependency", func(t *testing.T) {
			def := &PackageDefinition{
				Path: "/test/path",
				Metadata: KurelMetadata{
					Name: "test-package",
				},
				Patches: []Patch{
					{
						Name:    "patch1",
						Content: "spec.foo: bar",
						Metadata: &PatchMetadata{
							Requires: []string{"non-existent-patch"},
						},
					},
				},
			}

			result, err := validator.ValidatePackage(ctx, def)
			require.NoError(t, err)

			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)

			// Check for missing dependency error
			found := false
			for _, e := range result.Errors {
				if contains(e.Message, "does not exist") {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected missing dependency error")
		})

		t.Run("reserved parameter names", func(t *testing.T) {
			def := &PackageDefinition{
				Path: "/test/path",
				Metadata: KurelMetadata{
					Name:    "test-package",
					Version: "1.0.0",
				},
				Parameters: ParameterMap{
					"kurel.internal": "value", // Reserved prefix
					"system.config":  "value", // Reserved prefix
					"valid":          "value", // OK
				},
			}

			result, err := validator.ValidatePackage(ctx, def)
			require.NoError(t, err)

			// Should have warnings, not errors
			assert.True(t, result.IsValid())
			assert.NotEmpty(t, result.Warnings)

			// Check for reserved name warnings
			found := 0
			for _, w := range result.Warnings {
				if contains(w.Message, "reserved") {
					found++
				}
			}
			assert.Equal(t, 2, found, "Expected 2 reserved name warnings")
		})
	})

	t.Run("ValidateResource", func(t *testing.T) {
		t.Run("valid deployment", func(t *testing.T) {
			resource := Resource{
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
			}

			result, err := validator.ValidateResource(ctx, resource)
			require.NoError(t, err)
			assert.True(t, result.IsValid())
			assert.Empty(t, result.Errors)
		})

		t.Run("missing required fields", func(t *testing.T) {
			resource := Resource{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Metadata: metav1.ObjectMeta{
					Name: "test-app",
				},
				Raw: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"name": "test-app",
						},
						// Missing spec
					},
				},
			}

			result, err := validator.ValidateResource(ctx, resource)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)
		})

		t.Run("invalid service port", func(t *testing.T) {
			resource := Resource{
				APIVersion: "v1",
				Kind:       "Service",
				Metadata: metav1.ObjectMeta{
					Name: "test-svc",
				},
				Raw: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "v1",
						"kind":       "Service",
						"metadata": map[string]interface{}{
							"name": "test-svc",
						},
						"spec": map[string]interface{}{
							"ports": []interface{}{
								map[string]interface{}{
									"port": int64(70000), // Invalid port > 65535
								},
							},
						},
					},
				},
			}

			result, err := validator.ValidateResource(ctx, resource)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)

			// Check for port range error
			found := false
			for _, e := range result.Errors {
				if contains(e.Message, "65535") {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected port range error")
		})

		t.Run("invalid ingress hostname", func(t *testing.T) {
			resource := Resource{
				APIVersion: "networking.k8s.io/v1",
				Kind:       "Ingress",
				Metadata: metav1.ObjectMeta{
					Name: "test-ingress",
				},
				Raw: &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "networking.k8s.io/v1",
						"kind":       "Ingress",
						"metadata": map[string]interface{}{
							"name": "test-ingress",
						},
						"spec": map[string]interface{}{
							"rules": []interface{}{
								map[string]interface{}{
									"host": "invalid_hostname!", // Invalid chars
								},
							},
						},
					},
				},
			}

			result, err := validator.ValidateResource(ctx, resource)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)

			// Check for hostname error
			found := false
			for _, e := range result.Errors {
				if contains(e.Message, "hostname") {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected invalid hostname error")
		})
	})

	t.Run("ValidatePatch", func(t *testing.T) {
		t.Run("valid patch", func(t *testing.T) {
			patch := Patch{
				Name:    "scale-up",
				Content: "spec.replicas: 5",
			}

			result, err := validator.ValidatePatch(ctx, patch)
			require.NoError(t, err)
			assert.True(t, result.IsValid())
			assert.Empty(t, result.Errors)
		})

		t.Run("missing name", func(t *testing.T) {
			patch := Patch{
				Content: "spec.replicas: 5",
			}

			result, err := validator.ValidatePatch(ctx, patch)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)
		})

		t.Run("invalid name format", func(t *testing.T) {
			patch := Patch{
				Name:    "Scale_Up!", // Invalid characters
				Content: "spec.replicas: 5",
			}

			result, err := validator.ValidatePatch(ctx, patch)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)
		})

		t.Run("empty content", func(t *testing.T) {
			patch := Patch{
				Name:    "empty-patch",
				Content: "",
			}

			result, err := validator.ValidatePatch(ctx, patch)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)
		})

		t.Run("self dependency", func(t *testing.T) {
			patch := Patch{
				Name:    "self-dep",
				Content: "spec.foo: bar",
				Metadata: &PatchMetadata{
					Requires: []string{"self-dep"}, // Self dependency
				},
			}

			result, err := validator.ValidatePatch(ctx, patch)
			require.NoError(t, err)
			assert.False(t, result.IsValid())
			assert.NotEmpty(t, result.Errors)
		})

		t.Run("self conflict warning", func(t *testing.T) {
			patch := Patch{
				Name:    "self-conflict",
				Content: "spec.foo: bar",
				Metadata: &PatchMetadata{
					Conflicts: []string{"self-conflict"}, // Self conflict
				},
			}

			result, err := validator.ValidatePatch(ctx, patch)
			require.NoError(t, err)
			assert.True(t, result.IsValid()) // Should be valid but with warning
			assert.NotEmpty(t, result.Warnings)
		})
	})

	t.Run("StrictMode", func(t *testing.T) {
		// Enable strict mode
		validator.SetStrictMode(true)

		def := &PackageDefinition{
			Path: "/test/path",
			Metadata: KurelMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Parameters: ParameterMap{
				"kurel.internal": "value", // Reserved prefix - normally a warning
			},
		}

		result, err := validator.ValidatePackage(ctx, def)
		require.NoError(t, err)

		// In strict mode, warnings become errors
		assert.False(t, result.IsValid())
		assert.NotEmpty(t, result.Errors)
		assert.Empty(t, result.Warnings)

		// Disable strict mode
		validator.SetStrictMode(false)

		result, err = validator.ValidatePackage(ctx, def)
		require.NoError(t, err)

		// Without strict mode, should be valid with warnings
		assert.True(t, result.IsValid())
		assert.Empty(t, result.Errors)
		assert.NotEmpty(t, result.Warnings)
	})

	t.Run("MaxErrors", func(t *testing.T) {
		// Set max errors to 2
		validator.SetMaxErrors(2)

		def := &PackageDefinition{
			Path: "/test/path",
			Metadata: KurelMetadata{
				Name:    "test-package",
				Version: "1.0.0",
			},
			Resources: []Resource{
				// Create multiple invalid resources
				{Kind: "Deployment"}, // Missing APIVersion
				{APIVersion: "v1"},   // Missing Kind
				{},                   // Missing both
				{},                   // Another invalid
			},
		}

		result, err := validator.ValidatePackage(ctx, def)
		require.NoError(t, err)

		// Should stop after max errors
		assert.False(t, result.IsValid())
		// Should have at least max errors + 1 (for the stopping message)
		assert.GreaterOrEqual(t, len(result.Errors), 3)

		// Check for stopping message
		found := false
		for _, e := range result.Errors {
			if contains(e.Message, "stopped after") {
				found = true
				break
			}
		}
		assert.True(t, found, "Expected max errors message")
	})
}

func TestFormatResult(t *testing.T) {
	t.Run("valid result", func(t *testing.T) {
		result := &ValidationResult{
			Errors:   []ValidationError{},
			Warnings: []ValidationWarning{},
		}

		formatted := FormatResult(result)
		assert.Contains(t, formatted, "✓ Package is valid")
	})

	t.Run("errors and warnings", func(t *testing.T) {
		result := &ValidationResult{
			Errors: []ValidationError{
				{
					Path:    "spec.replicas",
					Message: "must be positive",
				},
				{
					Message:  "missing required field",
					Resource: "Deployment/app",
					Field:    "spec.selector",
				},
			},
			Warnings: []ValidationWarning{
				{
					Field:   "metadata.labels",
					Message: "recommended labels missing",
				},
			},
		}

		formatted := FormatResult(result)

		// Check structure
		assert.Contains(t, formatted, "✗ Package validation failed")
		assert.Contains(t, formatted, "Errors (2):")
		assert.Contains(t, formatted, "Warnings (1):")

		// Check error details
		assert.Contains(t, formatted, "spec.replicas: must be positive")
		assert.Contains(t, formatted, "missing required field")
		assert.Contains(t, formatted, "Resource: Deployment/app")
		assert.Contains(t, formatted, "spec.selector")

		// Check warning details
		assert.Contains(t, formatted, "metadata.labels: recommended labels missing")
	})
}

func TestValidatorHelpers(t *testing.T) {
	t.Run("isValidName", func(t *testing.T) {
		tests := []struct {
			name  string
			valid bool
		}{
			{"valid-name", true},
			{"valid.name", true},
			{"valid-name-123", true},
			{"", false},
			{"Invalid_Name", false},
			{"invalid name", false},
			{"-invalid", false},
			{"invalid-", false},
			{strings.Repeat("a", 254), false}, // Too long
		}

		for _, tt := range tests {
			result := isValidName(tt.name)
			assert.Equal(t, tt.valid, result, "Name: %s", tt.name)
		}
	})

	t.Run("isValidHostname", func(t *testing.T) {
		tests := []struct {
			hostname string
			valid    bool
		}{
			{"example.com", true},
			{"sub.example.com", true},
			{"example-123.com", true},
			{"", false},
			{"example_com", false},
			{"example..com", false},
			{"example.com.", false}, // Trailing dot not allowed
			{"-example.com", false},
			{strings.Repeat("a", 254), false}, // Too long
		}

		for _, tt := range tests {
			result := isValidHostname(tt.hostname)
			assert.Equal(t, tt.valid, result, "Hostname: %s", tt.hostname)
		}
	})

	t.Run("isValidVariableName", func(t *testing.T) {
		tests := []struct {
			varName string
			valid   bool
		}{
			{"variable", true},
			{"_private", true},
			{"var123", true},
			{"nested.field", true},
			{"array[0]", true},
			{"nested.array[2]", true},
			{"123invalid", false},
			{"invalid-name", false},
			{"invalid name", false},
			{"", false},
		}

		for _, tt := range tests {
			result := isValidVariableName(tt.varName)
			assert.Equal(t, tt.valid, result, "Variable: %s", tt.varName)
		}
	})
}

// Helper function for string contains check
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
