package launcher

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/logger"
)

func TestSchemaGenerator(t *testing.T) {
	log := logger.Noop()
	generator := NewSchemaGenerator(log)
	ctx := context.Background()

	t.Run("GeneratePackageSchema", func(t *testing.T) {
		schema, err := generator.GeneratePackageSchema(ctx)
		require.NoError(t, err)
		require.NotNil(t, schema)

		// Check required fields
		assert.Equal(t, "object", schema.Type)
		assert.Contains(t, schema.Properties, "path")
		assert.Contains(t, schema.Properties, "metadata")
		assert.Contains(t, schema.Properties, "parameters")
		assert.Contains(t, schema.Properties, "resources")
		assert.Contains(t, schema.Properties, "patches")
		assert.Contains(t, schema.Required, "metadata")

		// Check metadata schema
		metadata := schema.Properties["metadata"]
		assert.NotNil(t, metadata)
		assert.Equal(t, "object", metadata.Type)
		assert.Contains(t, metadata.Properties, "name")
		assert.Contains(t, metadata.Properties, "version")

		// Check top-level properties
		params := schema.Properties["parameters"]
		assert.NotNil(t, params)
		assert.Equal(t, "object", params.Type)

		resources := schema.Properties["resources"]
		assert.NotNil(t, resources)
		assert.Equal(t, "array", resources.Type)

		patches := schema.Properties["patches"]
		assert.NotNil(t, patches)
		assert.Equal(t, "array", patches.Type)
	})

	t.Run("GenerateResourceSchema", func(t *testing.T) {
		tests := []struct {
			name string
			gvk  schema.GroupVersionKind
		}{
			{
				name: "deployment",
				gvk: schema.GroupVersionKind{
					Group:   "apps",
					Version: "v1",
					Kind:    "Deployment",
				},
			},
			{
				name: "service",
				gvk: schema.GroupVersionKind{
					Version: "v1",
					Kind:    "Service",
				},
			},
			{
				name: "configmap",
				gvk: schema.GroupVersionKind{
					Version: "v1",
					Kind:    "ConfigMap",
				},
			},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				schema, err := generator.GenerateResourceSchema(ctx, tt.gvk)
				require.NoError(t, err)
				require.NotNil(t, schema)

				// Check basic structure
				assert.Equal(t, "object", schema.Type)
				assert.Contains(t, schema.Properties, "apiVersion")
				assert.Contains(t, schema.Properties, "kind")
				assert.Contains(t, schema.Properties, "metadata")

				// Check for spec or data depending on kind
				switch tt.gvk.Kind {
				case "Deployment":
					assert.Contains(t, schema.Properties, "spec")
					spec := schema.Properties["spec"]
					assert.Contains(t, spec.Properties, "replicas")
					assert.Contains(t, spec.Properties, "selector")
					assert.Contains(t, spec.Properties, "template")
				case "Service":
					assert.Contains(t, schema.Properties, "spec")
					spec := schema.Properties["spec"]
					assert.Contains(t, spec.Properties, "type")
					assert.Contains(t, spec.Properties, "selector")
					assert.Contains(t, spec.Properties, "ports")
				case "ConfigMap":
					assert.Contains(t, schema.Properties, "data")
				}
			})
		}
	})

	t.Run("GenerateParameterSchema", func(t *testing.T) {
		params := ParameterMap{
			"app": map[string]any{
				"name":    "test-app",
				"version": "1.0.0",
				"port":    8080,
			},
			"enabled":  true,
			"replicas": 3,
			"features": []any{"logging", "metrics"},
		}

		schema, err := generator.GenerateParameterSchema(ctx, params)
		require.NoError(t, err)
		require.NotNil(t, schema)

		// Check structure
		assert.Equal(t, "object", schema.Type)
		assert.Contains(t, schema.Properties, "app")
		assert.Contains(t, schema.Properties, "enabled")
		assert.Contains(t, schema.Properties, "replicas")
		assert.Contains(t, schema.Properties, "features")

		// Check inferred types
		appSchema := schema.Properties["app"]
		assert.Equal(t, "object", appSchema.Type)
		assert.Contains(t, appSchema.Properties, "name")
		assert.Contains(t, appSchema.Properties, "version")
		assert.Contains(t, appSchema.Properties, "port")

		enabledSchema := schema.Properties["enabled"]
		assert.Equal(t, "boolean", enabledSchema.Type)
		assert.Equal(t, true, enabledSchema.Default)

		replicasSchema := schema.Properties["replicas"]
		assert.Equal(t, "integer", replicasSchema.Type)

		featuresSchema := schema.Properties["features"]
		assert.Equal(t, "array", featuresSchema.Type)
		assert.NotNil(t, featuresSchema.Items)
	})

	t.Run("TraceFieldUsage", func(t *testing.T) {
		resources := []Resource{
			{
				APIVersion: "apps/v1",
				Kind:       "Deployment",
				Metadata: metav1.ObjectMeta{
					Name: "app",
				},
				Raw: &unstructured.Unstructured{
					Object: map[string]any{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]any{
							"name": "app",
						},
						"spec": map[string]any{
							"replicas": "${replicas}",
							"template": map[string]any{
								"spec": map[string]any{
									"containers": []any{
										map[string]any{
											"name":  "app",
											"image": "${app.image}",
											"env": []any{
												map[string]any{
													"name":  "PORT",
													"value": "${app.port}",
												},
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

		usage := generator.TraceFieldUsage(resources)

		// Check that variables are traced
		assert.Contains(t, usage, "replicas")
		assert.Contains(t, usage, "app.image")
		assert.Contains(t, usage, "app.port")

		// Check paths
		assert.Contains(t, usage["replicas"][0], "Deployment:spec.replicas")
		assert.Contains(t, usage["app.image"][0], "Deployment:spec.template.spec.containers[0].image")
	})

	t.Run("ExportSchema", func(t *testing.T) {
		schema := &JSONSchema{
			Type:        "object",
			Description: "Test schema",
			Properties: map[string]*JSONSchema{
				"name": {
					Type:        "string",
					Description: "Name field",
				},
			},
			Required: []string{"name"},
		}

		data, err := generator.ExportSchema(schema)
		require.NoError(t, err)

		// Check JSON is valid
		var parsed map[string]any
		err = json.Unmarshal(data, &parsed)
		require.NoError(t, err)

		// Check content
		assert.Equal(t, "object", parsed["type"])
		assert.Equal(t, "Test schema", parsed["description"])
		assert.NotNil(t, parsed["properties"])
		assert.Contains(t, parsed["required"].([]any), "name")
	})

	t.Run("DebugSchema", func(t *testing.T) {
		schema := &JSONSchema{
			Type:        "object",
			Description: "Test schema",
			Properties: map[string]*JSONSchema{
				"field1": {
					Type:        "string",
					Description: "String field",
					Pattern:     "^[a-z]+$",
				},
				"field2": {
					Type:        "integer",
					Description: "Number field",
					Minimum:     float64Ptr(0),
					Maximum:     float64Ptr(100),
				},
			},
			Required: []string{"field1"},
		}

		debug := generator.DebugSchema(schema)

		// Check debug output contains expected information
		assert.Contains(t, debug, "Type: object")
		assert.Contains(t, debug, "Description: Test schema")
		assert.Contains(t, debug, "field1:")
		assert.Contains(t, debug, "Type: string")
		assert.Contains(t, debug, "Pattern: ^[a-z]+$")
		assert.Contains(t, debug, "field2:")
		assert.Contains(t, debug, "Type: integer")
		assert.Contains(t, debug, "Required: [field1]")
	})
}

func TestValidateWithSchema(t *testing.T) {
	t.Run("valid data", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name": {
					Type:      "string",
					MinLength: new(1),
				},
				"age": {
					Type:    "integer",
					Minimum: float64Ptr(0),
					Maximum: float64Ptr(150),
				},
			},
			Required: []string{"name"},
		}

		data := map[string]any{
			"name": "John",
			"age":  30,
		}

		errors := ValidateWithSchema(data, schema)
		assert.Empty(t, errors)
	})

	t.Run("missing required field", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name": {Type: "string"},
			},
			Required: []string{"name"},
		}

		data := map[string]any{}

		errors := ValidateWithSchema(data, schema)
		assert.Len(t, errors, 1)
		assert.Contains(t, errors[0].Message, "required field missing")
		assert.Equal(t, "$.name", errors[0].Field)
	})

	t.Run("type mismatch", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"count": {Type: "integer"},
			},
		}

		data := map[string]any{
			"count": "not a number",
		}

		errors := ValidateWithSchema(data, schema)
		assert.Len(t, errors, 1)
		assert.Contains(t, errors[0].Message, "expected type")
	})

	t.Run("string constraints", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"username": {
					Type:      "string",
					MinLength: new(3),
					MaxLength: new(10),
				},
			},
		}

		tests := []struct {
			value     string
			shouldErr bool
		}{
			{"ab", true},          // too short
			{"abc", false},        // min length
			{"abcdefghij", false}, // max length
			{"abcdefghijk", true}, // too long
		}

		for _, tt := range tests {
			data := map[string]any{
				"username": tt.value,
			}
			errors := ValidateWithSchema(data, schema)
			if tt.shouldErr {
				assert.NotEmpty(t, errors, "Expected error for value: %s", tt.value)
			} else {
				assert.Empty(t, errors, "Unexpected error for value: %s", tt.value)
			}
		}
	})

	t.Run("number constraints", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"port": {
					Type:    "integer",
					Minimum: float64Ptr(1),
					Maximum: float64Ptr(65535),
				},
			},
		}

		tests := []struct {
			value     int
			shouldErr bool
		}{
			{0, true},      // below min
			{1, false},     // min value
			{8080, false},  // valid
			{65535, false}, // max value
			{65536, true},  // above max
		}

		for _, tt := range tests {
			data := map[string]any{
				"port": tt.value,
			}
			errors := ValidateWithSchema(data, schema)
			if tt.shouldErr {
				assert.NotEmpty(t, errors, "Expected error for value: %d", tt.value)
			} else {
				assert.Empty(t, errors, "Unexpected error for value: %d", tt.value)
			}
		}
	})

	t.Run("enum validation", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"environment": {
					Type: "string",
					Enum: []any{"dev", "staging", "prod"},
				},
			},
		}

		tests := []struct {
			value     string
			shouldErr bool
		}{
			{"dev", false},
			{"staging", false},
			{"prod", false},
			{"test", true},
			{"local", true},
		}

		for _, tt := range tests {
			data := map[string]any{
				"environment": tt.value,
			}
			errors := ValidateWithSchema(data, schema)
			if tt.shouldErr {
				assert.NotEmpty(t, errors, "Expected error for value: %s", tt.value)
			} else {
				assert.Empty(t, errors, "Unexpected error for value: %s", tt.value)
			}
		}
	})

	t.Run("pattern validation", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name": {
					Type:    "string",
					Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
				},
			},
		}

		tests := []struct {
			value     string
			shouldErr bool
		}{
			{"valid-name", false},
			{"a", false},
			{"abc123", false},
			{"my-app-v2", false},
			{"Invalid", true},        // uppercase
			{"-invalid", true},       // starts with dash
			{"invalid-", true},       // ends with dash
			{"no spaces", true},      // spaces
			{"no_underscores", true}, // underscores
		}

		for _, tt := range tests {
			data := map[string]any{
				"name": tt.value,
			}
			errors := ValidateWithSchema(data, schema)
			if tt.shouldErr {
				assert.NotEmpty(t, errors, "Expected error for value: %s", tt.value)
				assert.Contains(t, errors[0].Message, "does not match pattern")
			} else {
				assert.Empty(t, errors, "Unexpected error for value: %s", tt.value)
			}
		}
	})

	t.Run("pattern validation with email", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"email": {
					Type:    "string",
					Pattern: `^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`,
				},
			},
		}

		tests := []struct {
			value     string
			shouldErr bool
		}{
			{"user@example.com", false},
			{"test.user@domain.org", false},
			{"notanemail", true},
			{"@missing-local.com", true},
			{"missing-domain@", true},
		}

		for _, tt := range tests {
			data := map[string]any{
				"email": tt.value,
			}
			errors := ValidateWithSchema(data, schema)
			if tt.shouldErr {
				assert.NotEmpty(t, errors, "Expected error for value: %s", tt.value)
			} else {
				assert.Empty(t, errors, "Unexpected error for value: %s", tt.value)
			}
		}
	})

	t.Run("pattern validation with invalid regex", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"field": {
					Type:    "string",
					Pattern: `[invalid`,
				},
			},
		}

		data := map[string]any{
			"field": "any value",
		}
		errors := ValidateWithSchema(data, schema)
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].Message, "invalid schema pattern")
	})

	t.Run("pattern validation skips empty string", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"version": {
					Type:    "string",
					Pattern: `^v?\d+\.\d+\.\d+(-[a-z0-9]+)?(\+[a-z0-9]+)?$`,
				},
			},
		}

		// Empty string is not validated against patterns (use minLength to enforce presence)
		data := map[string]any{
			"version": "",
		}
		errors := ValidateWithSchema(data, schema)
		assert.Empty(t, errors)
	})

	t.Run("pattern validation non-empty mismatch", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"version": {
					Type:    "string",
					Pattern: `^v?\d+\.\d+\.\d+(-[a-z0-9]+)?(\+[a-z0-9]+)?$`,
				},
			},
		}

		// Non-empty string that doesn't match should fail
		data := map[string]any{
			"version": "not-a-version",
		}
		errors := ValidateWithSchema(data, schema)
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].Message, "does not match pattern")
	})

	t.Run("pattern validation skips template variables", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name": {
					Type:    "string",
					Pattern: `^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`,
				},
			},
		}

		// Template variables should not be validated against patterns
		data := map[string]any{
			"name": "${app_name}",
		}
		errors := ValidateWithSchema(data, schema)
		assert.Empty(t, errors)
	})

	t.Run("pattern with no pattern set", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"name": {
					Type: "string",
				},
			},
		}

		data := map[string]any{
			"name": "anything goes",
		}
		errors := ValidateWithSchema(data, schema)
		assert.Empty(t, errors)
	})

	t.Run("nested object validation", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"app": {
					Type: "object",
					Properties: map[string]*JSONSchema{
						"name": {
							Type:      "string",
							MinLength: new(1),
						},
						"port": {
							Type:    "integer",
							Minimum: float64Ptr(1),
							Maximum: float64Ptr(65535),
						},
					},
					Required: []string{"name"},
				},
			},
		}

		// Valid nested object
		data := map[string]any{
			"app": map[string]any{
				"name": "test-app",
				"port": 8080,
			},
		}
		errors := ValidateWithSchema(data, schema)
		assert.Empty(t, errors)

		// Missing required nested field
		data = map[string]any{
			"app": map[string]any{
				"port": 8080,
			},
		}
		errors = ValidateWithSchema(data, schema)
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].Field, "app.name")
	})

	t.Run("array validation", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"ports": {
					Type:      "array",
					MinLength: new(1),
					Items: &JSONSchema{
						Type:    "integer",
						Minimum: float64Ptr(1),
						Maximum: float64Ptr(65535),
					},
				},
			},
		}

		// Valid array
		data := map[string]any{
			"ports": []any{80, 443, 8080},
		}
		errors := ValidateWithSchema(data, schema)
		assert.Empty(t, errors)

		// Empty array (violates min length)
		data = map[string]any{
			"ports": []any{},
		}
		errors = ValidateWithSchema(data, schema)
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].Message, "less than minimum")

		// Invalid item in array
		data = map[string]any{
			"ports": []any{80, 70000}, // 70000 > 65535
		}
		errors = ValidateWithSchema(data, schema)
		assert.NotEmpty(t, errors)
		assert.Contains(t, errors[0].Field, "ports[1]")
	})
}

func TestMergeSchemas(t *testing.T) {
	t.Run("merge simple schemas", func(t *testing.T) {
		schema1 := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"field1": {Type: "string"},
			},
			Required: []string{"field1"},
		}

		schema2 := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"field2": {Type: "integer"},
			},
			Required: []string{"field2"},
		}

		merged := MergeSchemas(schema1, schema2)

		assert.Equal(t, "object", merged.Type)
		assert.Contains(t, merged.Properties, "field1")
		assert.Contains(t, merged.Properties, "field2")
		assert.Contains(t, merged.Required, "field1")
		assert.Contains(t, merged.Required, "field2")
	})

	t.Run("merge nested schemas", func(t *testing.T) {
		schema1 := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"app": {
					Type: "object",
					Properties: map[string]*JSONSchema{
						"name": {Type: "string"},
					},
				},
			},
		}

		schema2 := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"app": {
					Type: "object",
					Properties: map[string]*JSONSchema{
						"version": {Type: "string"},
					},
				},
			},
		}

		merged := MergeSchemas(schema1, schema2)

		appSchema := merged.Properties["app"]
		assert.NotNil(t, appSchema)
		assert.Contains(t, appSchema.Properties, "name")
		assert.Contains(t, appSchema.Properties, "version")
	})

	t.Run("handle nil schemas", func(t *testing.T) {
		schema := &JSONSchema{
			Type: "object",
			Properties: map[string]*JSONSchema{
				"field": {Type: "string"},
			},
		}

		// Merge with nil
		merged := MergeSchemas(schema, nil)
		assert.NotNil(t, merged)
		assert.Contains(t, merged.Properties, "field")

		// All nil
		merged = MergeSchemas(nil, nil)
		assert.Nil(t, merged)
	})
}

func TestInferSchema(t *testing.T) {
	generator := &schemaGenerator{
		logger: logger.Noop(),
	}

	tests := []struct {
		name     string
		value    any
		expected string // expected type
	}{
		{"nil", nil, "null"},
		{"bool", true, "boolean"},
		{"int", 42, "integer"},
		{"float", 3.14, "number"},
		{"string", "hello", "string"},
		{"array", []any{1, 2, 3}, "array"},
		{"object", map[string]any{"key": "value"}, "object"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := generator.inferSchema(tt.value, "$")
			assert.Equal(t, tt.expected, schema.Type)
			assert.Equal(t, "$", schema.KurelPath)
			assert.Equal(t, "inferred", schema.KurelSource)

			// Check default value is set
			if tt.value != nil {
				assert.Equal(t, tt.value, schema.Default)
			}
		})
	}

	t.Run("string with variable", func(t *testing.T) {
		schema := generator.inferSchema("Hello ${name}", "$")
		assert.Equal(t, "string", schema.Type)
		assert.Contains(t, schema.Description, "Variable substitution")
		assert.NotEmpty(t, schema.Pattern)
	})

	t.Run("nested object", func(t *testing.T) {
		value := map[string]any{
			"app": map[string]any{
				"name": "test",
				"port": 8080,
			},
		}

		schema := generator.inferSchema(value, "$")
		assert.Equal(t, "object", schema.Type)
		assert.Contains(t, schema.Properties, "app")

		appSchema := schema.Properties["app"]
		assert.Equal(t, "object", appSchema.Type)
		assert.Contains(t, appSchema.Properties, "name")
		assert.Contains(t, appSchema.Properties, "port")

		// Check paths
		assert.Equal(t, "$.app", appSchema.KurelPath)
		assert.Equal(t, "$.app.name", appSchema.Properties["name"].KurelPath)
	})
}

func TestGetJSONType(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected string
	}{
		{name: "nil", input: nil, expected: "null"},
		{name: "bool", input: true, expected: "boolean"},
		{name: "int", input: 42, expected: "integer"},
		{name: "int32", input: int32(42), expected: "integer"},
		{name: "int64", input: int64(42), expected: "integer"},
		{name: "float32", input: float32(3.14), expected: "number"},
		{name: "float64", input: float64(3.14), expected: "number"},
		{name: "string", input: "hello", expected: "string"},
		{name: "slice interface", input: []any{1, 2}, expected: "array"},
		{name: "slice string", input: []string{"a", "b"}, expected: "array"},
		{name: "map", input: map[string]any{"a": 1}, expected: "object"},
		{name: "ParameterMap", input: ParameterMap{"a": 1}, expected: "object"},
		{name: "slice int via reflect", input: []int{1, 2}, expected: "array"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getJSONType(tt.input)
			if got != tt.expected {
				t.Errorf("getJSONType(%v) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestGetNumber(t *testing.T) {
	tests := []struct {
		name     string
		input    any
		expected float64
		ok       bool
	}{
		{name: "int", input: 42, expected: 42.0, ok: true},
		{name: "int32", input: int32(100), expected: 100.0, ok: true},
		{name: "int64", input: int64(200), expected: 200.0, ok: true},
		{name: "float32", input: float32(3.14), expected: float64(float32(3.14)), ok: true},
		{name: "float64", input: float64(2.718), expected: 2.718, ok: true},
		{name: "string", input: "not a number", expected: 0, ok: false},
		{name: "bool", input: true, expected: 0, ok: false},
		{name: "nil", input: nil, expected: 0, ok: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := getNumber(tt.input)
			if ok != tt.ok {
				t.Errorf("getNumber(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if ok && got != tt.expected {
				t.Errorf("getNumber(%v) = %v, want %v", tt.input, got, tt.expected)
			}
		})
	}
}

func TestSchemaGeneratorSetVerbose(t *testing.T) {
	log := logger.Noop()
	gen := NewSchemaGenerator(log).(*schemaGenerator)

	gen.SetVerbose(true)
	if !gen.verbose {
		t.Error("expected verbose to be true")
	}

	gen.SetVerbose(false)
	if gen.verbose {
		t.Error("expected verbose to be false")
	}
}

func TestGeneratePackageSchemaWithOptions(t *testing.T) {
	log := logger.Noop()
	gen := NewSchemaGenerator(log)
	ctx := context.Background()

	t.Run("with k8s schemas", func(t *testing.T) {
		schema, err := gen.GeneratePackageSchemaWithOptions(ctx, &SchemaOptions{IncludeK8s: true})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
		if schema.Type != "object" {
			t.Errorf("expected type 'object', got %q", schema.Type)
		}
	})

	t.Run("without k8s schemas", func(t *testing.T) {
		schema, err := gen.GeneratePackageSchemaWithOptions(ctx, &SchemaOptions{IncludeK8s: false})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
	})

	t.Run("nil options", func(t *testing.T) {
		schema, err := gen.GeneratePackageSchemaWithOptions(ctx, nil)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if schema == nil {
			t.Fatal("expected non-nil schema")
		}
	})
}

func TestNewSchemaGeneratorNilLogger(t *testing.T) {
	gen := NewSchemaGenerator(nil)
	if gen == nil {
		t.Fatal("expected non-nil schema generator with nil logger")
	}
}
