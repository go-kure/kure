package launcher

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/go-kure/kure/pkg/logger"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// SchemaGenerator generates JSON schemas for package validation
type schemaGenerator struct {
	logger    logger.Logger
	cache     map[string]*JSONSchema      // Cache for generated schemas
	traceMap  map[string][]string     // Map of type to field paths that reference it
	maxDepth  int                     // Maximum recursion depth
	verbose   bool                    // Verbose mode for debugging
}

// JSONSchema represents a JSON schema for validation
type JSONSchema struct {
	Type        string                    `json:"type,omitempty"`
	Description string                    `json:"description,omitempty"`
	Properties  map[string]*JSONSchema        `json:"properties,omitempty"`
	Items       *JSONSchema                   `json:"items,omitempty"`
	Required    []string                  `json:"required,omitempty"`
	Enum        []interface{}             `json:"enum,omitempty"`
	Pattern     string                    `json:"pattern,omitempty"`
	MinLength   *int                      `json:"minLength,omitempty"`
	MaxLength   *int                      `json:"maxLength,omitempty"`
	Minimum     *float64                  `json:"minimum,omitempty"`
	Maximum     *float64                  `json:"maximum,omitempty"`
	Default     interface{}               `json:"default,omitempty"`
	Examples    []interface{}             `json:"examples,omitempty"`
	Ref         string                    `json:"$ref,omitempty"`
	OneOf       []*JSONSchema                 `json:"oneOf,omitempty"`
	AnyOf       []*JSONSchema                 `json:"anyOf,omitempty"`
	AllOf       []*JSONSchema                 `json:"allOf,omitempty"`
	Not         *JSONSchema                   `json:"not,omitempty"`
	
	// Custom fields for Kurel
	KurelType   string                    `json:"x-kurel-type,omitempty"`   // Original K8s type
	KurelPath   string                    `json:"x-kurel-path,omitempty"`   // Field path in resource
	KurelSource string                    `json:"x-kurel-source,omitempty"` // Source of the field (k8s, kurel, custom)
}

// NewSchemaGenerator creates a new schema generator
func NewSchemaGenerator(log logger.Logger) SchemaGenerator {
	if log == nil {
		log = logger.Default()
	}
	return &schemaGenerator{
		logger:   log,
		cache:    make(map[string]*JSONSchema),
		traceMap: make(map[string][]string),
		maxDepth: 10,
	}
}

// GeneratePackageSchema generates a schema for a package definition
func (g *schemaGenerator) GeneratePackageSchema(ctx context.Context) (*JSONSchema, error) {
	g.logger.Debug("Generating package schema")
	
	// Root schema for a Kurel package
	schema := &JSONSchema{
		Type:        "object",
		Description: "Kurel package definition",
		Properties: map[string]*JSONSchema{
			"path": {
				Type:        "string",
				Description: "Path to the package",
			},
			"metadata": g.generateMetadataSchema(),
			"parameters": g.generateParametersSchema(),
			"resources":  g.generateResourcesSchema(),
			"patches":    g.generatePatchesSchema(),
		},
		Required: []string{"metadata"},
	}
	
	return schema, nil
}

// GenerateResourceSchema generates a schema for a specific resource type
func (g *schemaGenerator) GenerateResourceSchema(ctx context.Context, gvk schema.GroupVersionKind) (*JSONSchema, error) {
	g.logger.Debug("Generating schema for resource %s", gvk.String())
	
	// Check cache
	cacheKey := gvk.String()
	if cached, ok := g.cache[cacheKey]; ok {
		return cached, nil
	}
	
	// Generate schema based on GVK
	schema := g.generateKubernetesResourceSchema(gvk)
	
	// Cache the result
	g.cache[cacheKey] = schema
	
	return schema, nil
}

// GenerateParameterSchema generates a schema for package parameters
func (g *schemaGenerator) GenerateParameterSchema(ctx context.Context, params ParameterMap) (*JSONSchema, error) {
	g.logger.Debug("Generating parameter schema")
	
	properties := make(map[string]*JSONSchema)
	required := []string{}
	
	// Analyze parameter structure
	for key, value := range params {
		paramSchema := g.inferSchema(value, fmt.Sprintf("$.parameters.%s", key))
		properties[key] = paramSchema
		
		// Mark as required if it doesn't have a default
		if paramSchema.Default == nil {
			required = append(required, key)
		}
	}
	
	return &JSONSchema{
		Type:        "object",
		Description: "Package parameters",
		Properties:  properties,
		Required:    required,
	}, nil
}

// TraceFieldUsage traces how fields are used across resources
func (g *schemaGenerator) TraceFieldUsage(resources []Resource) map[string][]string {
	g.logger.Debug("Tracing field usage across %d resources", len(resources))
	
	usage := make(map[string][]string)
	
	for _, resource := range resources {
		if resource.Raw != nil {
			g.traceObject(resource.Raw.Object, resource.Kind, "", usage)
		}
	}
	
	return usage
}

// generateMetadataSchema generates schema for package metadata
func (g *schemaGenerator) generateMetadataSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Package metadata",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Package name",
				Pattern:     "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
				MinLength:   intPtr(1),
				MaxLength:   intPtr(63),
			},
			"version": {
				Type:        "string",
				Description: "Package version (semantic versioning)",
				Pattern:     "^v?\\d+\\.\\d+\\.\\d+(-[a-z0-9]+)?(\\+[a-z0-9]+)?$",
				Examples:    []interface{}{"1.0.0", "v2.1.0-beta", "3.0.0+build123"},
			},
			"appVersion": {
				Type:        "string",
				Description: "Application version",
			},
			"description": {
				Type:        "string",
				Description: "Package description",
			},
			"home": {
				Type:        "string",
				Description: "Package home URL",
				Pattern:     "^https?://",
			},
			"keywords": {
				Type:        "array",
				Description: "Package keywords",
				Items: &JSONSchema{
					Type: "string",
				},
			},
			"schemas": {
				Type:        "array",
				Description: "CRD schema URLs",
				Items: &JSONSchema{
					Type: "string",
					Pattern: "^https?://",
				},
			},
			"maintainers": {
				Type:        "array",
				Description: "Package maintainers",
				Items: &JSONSchema{
					Type: "object",
					Properties: map[string]*JSONSchema{
						"name": {
							Type:        "string",
							Description: "Maintainer name",
						},
						"email": {
							Type:        "string",
							Description: "Maintainer email",
							Pattern:     "^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\\.[a-zA-Z]{2,}$",
						},
					},
					Required: []string{"name"},
				},
			},
		},
		Required: []string{"name", "version"},
	}
}

// generateParametersSchema generates schema for parameters section
func (g *schemaGenerator) generateParametersSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Package parameters that can be overridden",
		Properties:  map[string]*JSONSchema{},
		// Allow additional properties for flexibility
	}
}

// generateResourcesSchema generates schema for resources section
func (g *schemaGenerator) generateResourcesSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "array",
		Description: "Kubernetes resources defined by this package",
		Items: &JSONSchema{
			Type:        "object",
			Description: "Kubernetes resource",
			Properties: map[string]*JSONSchema{
				"apiVersion": {
					Type:        "string",
					Description: "API version of the resource",
				},
				"kind": {
					Type:        "string",
					Description: "Kind of the resource",
				},
				"metadata": {
					Type:        "object",
					Description: "Resource metadata",
					Properties: map[string]*JSONSchema{
						"name": {
							Type:        "string",
							Description: "Resource name",
						},
						"namespace": {
							Type:        "string",
							Description: "Resource namespace",
						},
						"labels": {
							Type:        "object",
							Description: "Resource labels",
						},
						"annotations": {
							Type:        "object",
							Description: "Resource annotations",
						},
					},
				},
				"spec": {
					Type:        "object",
					Description: "Resource specification",
				},
				"data": {
					Type:        "object",
					Description: "Resource data (for ConfigMap/Secret)",
				},
			},
			Required: []string{"apiVersion", "kind"},
		},
	}
}

// generatePatchesSchema generates schema for patches section
func (g *schemaGenerator) generatePatchesSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "array",
		Description: "Patches to apply to resources",
		Items: &JSONSchema{
			Type:        "object",
			Description: "Patch definition",
			Properties: map[string]*JSONSchema{
				"name": {
					Type:        "string",
					Description: "Unique patch name",
					Pattern:     "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
				},
				"content": {
					Type:        "string",
					Description: "Patch content in TOML or YAML format",
				},
				"metadata": {
					Type:        "object",
					Description: "Patch metadata",
					Properties: map[string]*JSONSchema{
						"description": {
							Type:        "string",
							Description: "Human-readable description",
						},
						"enabled": {
							Type:        "string",
							Description: "Condition for enabling the patch",
							Examples:    []interface{}{"true", "${feature.enabled}", "${env == 'prod'}"},
						},
						"requires": {
							Type:        "array",
							Description: "Patches this patch depends on",
							Items: &JSONSchema{
								Type: "string",
							},
						},
						"conflicts": {
							Type:        "array",
							Description: "Patches that conflict with this patch",
							Items: &JSONSchema{
								Type: "string",
							},
						},
					},
				},
			},
			Required: []string{"name", "content"},
		},
	}
}

// generateKubernetesResourceSchema generates schema for a Kubernetes resource type
func (g *schemaGenerator) generateKubernetesResourceSchema(gvk schema.GroupVersionKind) *JSONSchema {
	// Map common Kubernetes resource types to their schemas
	// This is a simplified version - in production, you'd want to use OpenAPI specs
	
	baseSchema := &JSONSchema{
		Type:        "object",
		Description: fmt.Sprintf("Kubernetes %s resource", gvk.Kind),
		Properties: map[string]*JSONSchema{
			"apiVersion": {
				Type:    "string",
				Default: gvk.GroupVersion().String(),
			},
			"kind": {
				Type:    "string",
				Default: gvk.Kind,
			},
			"metadata": g.generateK8sMetadataSchema(),
		},
		Required: []string{"apiVersion", "kind", "metadata"},
	}
	
	// Add spec based on kind
	switch strings.ToLower(gvk.Kind) {
	case "deployment", "statefulset", "daemonset":
		baseSchema.Properties["spec"] = g.generateWorkloadSpecSchema()
	case "service":
		baseSchema.Properties["spec"] = g.generateServiceSpecSchema()
	case "configmap":
		baseSchema.Properties["data"] = &JSONSchema{
			Type:        "object",
			Description: "ConfigMap data",
		}
	case "secret":
		baseSchema.Properties["data"] = &JSONSchema{
			Type:        "object",
			Description: "Secret data (base64 encoded)",
		}
		baseSchema.Properties["stringData"] = &JSONSchema{
			Type:        "object",
			Description: "Secret string data (not base64 encoded)",
		}
	case "ingress":
		baseSchema.Properties["spec"] = g.generateIngressSpecSchema()
	default:
		baseSchema.Properties["spec"] = &JSONSchema{
			Type:        "object",
			Description: "Resource specification",
		}
	}
	
	return baseSchema
}

// generateK8sMetadataSchema generates schema for Kubernetes metadata
func (g *schemaGenerator) generateK8sMetadataSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Kubernetes metadata",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Resource name",
				Pattern:     "^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$",
				MaxLength:   intPtr(253),
			},
			"namespace": {
				Type:        "string",
				Description: "Resource namespace",
				Pattern:     "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
				MaxLength:   intPtr(63),
			},
			"labels": {
				Type:        "object",
				Description: "Resource labels",
			},
			"annotations": {
				Type:        "object",
				Description: "Resource annotations",
			},
		},
		Required: []string{"name"},
	}
}

// generateWorkloadSpecSchema generates schema for workload specs
func (g *schemaGenerator) generateWorkloadSpecSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Workload specification",
		Properties: map[string]*JSONSchema{
			"replicas": {
				Type:        "integer",
				Description: "Number of replicas",
				Minimum:     float64Ptr(0),
				Default:     1,
			},
			"selector": {
				Type:        "object",
				Description: "Label selector",
				Properties: map[string]*JSONSchema{
					"matchLabels": {
						Type:        "object",
						Description: "Label key-value pairs to match",
					},
				},
			},
			"template": {
				Type:        "object",
				Description: "Pod template",
				Properties: map[string]*JSONSchema{
					"metadata": {
						Type:        "object",
						Description: "Pod metadata",
					},
					"spec": g.generatePodSpecSchema(),
				},
			},
		},
		Required: []string{"selector", "template"},
	}
}

// generatePodSpecSchema generates schema for pod specs
func (g *schemaGenerator) generatePodSpecSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Pod specification",
		Properties: map[string]*JSONSchema{
			"containers": {
				Type:        "array",
				Description: "List of containers",
				Items:       g.generateContainerSchema(),
				MinLength:   intPtr(1),
			},
			"initContainers": {
				Type:        "array",
				Description: "List of init containers",
				Items:       g.generateContainerSchema(),
			},
			"volumes": {
				Type:        "array",
				Description: "List of volumes",
				Items: &JSONSchema{
					Type:        "object",
					Description: "Volume definition",
				},
			},
		},
		Required: []string{"containers"},
	}
}

// generateContainerSchema generates schema for container specs
func (g *schemaGenerator) generateContainerSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Container specification",
		Properties: map[string]*JSONSchema{
			"name": {
				Type:        "string",
				Description: "Container name",
				Pattern:     "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$",
			},
			"image": {
				Type:        "string",
				Description: "Container image",
			},
			"command": {
				Type:        "array",
				Description: "Container command",
				Items: &JSONSchema{
					Type: "string",
				},
			},
			"args": {
				Type:        "array",
				Description: "Container arguments",
				Items: &JSONSchema{
					Type: "string",
				},
			},
			"env": {
				Type:        "array",
				Description: "Environment variables",
				Items: &JSONSchema{
					Type:        "object",
					Description: "Environment variable",
					Properties: map[string]*JSONSchema{
						"name": {
							Type:        "string",
							Description: "Variable name",
						},
						"value": {
							Type:        "string",
							Description: "Variable value",
						},
					},
				},
			},
			"ports": {
				Type:        "array",
				Description: "Container ports",
				Items: &JSONSchema{
					Type:        "object",
					Description: "Port definition",
					Properties: map[string]*JSONSchema{
						"containerPort": {
							Type:        "integer",
							Description: "Port number",
							Minimum:     float64Ptr(1),
							Maximum:     float64Ptr(65535),
						},
						"protocol": {
							Type:        "string",
							Description: "Protocol",
							Enum:        []interface{}{"TCP", "UDP", "SCTP"},
							Default:     "TCP",
						},
					},
				},
			},
		},
		Required: []string{"name", "image"},
	}
}

// generateServiceSpecSchema generates schema for service specs
func (g *schemaGenerator) generateServiceSpecSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Service specification",
		Properties: map[string]*JSONSchema{
			"type": {
				Type:        "string",
				Description: "Service type",
				Enum:        []interface{}{"ClusterIP", "NodePort", "LoadBalancer", "ExternalName"},
				Default:     "ClusterIP",
			},
			"selector": {
				Type:        "object",
				Description: "Label selector for pods",
			},
			"ports": {
				Type:        "array",
				Description: "Service ports",
				Items: &JSONSchema{
					Type:        "object",
					Description: "Service port",
					Properties: map[string]*JSONSchema{
						"port": {
							Type:        "integer",
							Description: "Service port",
							Minimum:     float64Ptr(1),
							Maximum:     float64Ptr(65535),
						},
						"targetPort": {
							Type:        "integer",
							Description: "Target port on pods",
							Minimum:     float64Ptr(1),
							Maximum:     float64Ptr(65535),
						},
						"protocol": {
							Type:        "string",
							Description: "Protocol",
							Enum:        []interface{}{"TCP", "UDP", "SCTP"},
							Default:     "TCP",
						},
					},
				},
			},
		},
	}
}

// generateIngressSpecSchema generates schema for ingress specs
func (g *schemaGenerator) generateIngressSpecSchema() *JSONSchema {
	return &JSONSchema{
		Type:        "object",
		Description: "Ingress specification",
		Properties: map[string]*JSONSchema{
			"rules": {
				Type:        "array",
				Description: "Ingress rules",
				Items: &JSONSchema{
					Type:        "object",
					Description: "Ingress rule",
					Properties: map[string]*JSONSchema{
						"host": {
							Type:        "string",
							Description: "Hostname",
						},
						"http": {
							Type:        "object",
							Description: "HTTP routes",
						},
					},
				},
			},
			"tls": {
				Type:        "array",
				Description: "TLS configuration",
				Items: &JSONSchema{
					Type:        "object",
					Description: "TLS configuration",
					Properties: map[string]*JSONSchema{
						"hosts": {
							Type:        "array",
							Description: "TLS hosts",
							Items: &JSONSchema{
								Type: "string",
							},
						},
						"secretName": {
							Type:        "string",
							Description: "TLS secret name",
						},
					},
				},
			},
		},
	}
}

// inferSchema infers a schema from a value
func (g *schemaGenerator) inferSchema(value interface{}, path string) *JSONSchema {
	if value == nil {
		return &JSONSchema{
			Type:        "null",
			KurelPath:   path,
			KurelSource: "inferred",
		}
	}
	
	switch v := value.(type) {
	case bool:
		return &JSONSchema{
			Type:        "boolean",
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
	case int, int32, int64:
		return &JSONSchema{
			Type:        "integer",
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
	case float32, float64:
		return &JSONSchema{
			Type:        "number",
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
	case string:
		schema := &JSONSchema{
			Type:        "string",
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
		// Check for common patterns
		if strings.Contains(v, "${") {
			schema.Description = "Variable substitution supported"
			schema.Pattern = ".*\\$\\{[^}]+\\}.*"
		}
		return schema
	case []interface{}:
		var itemSchema *JSONSchema
		if len(v) > 0 {
			itemSchema = g.inferSchema(v[0], fmt.Sprintf("%s[0]", path))
		} else {
			itemSchema = &JSONSchema{Type: "any"}
		}
		return &JSONSchema{
			Type:        "array",
			Items:       itemSchema,
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
	case map[string]interface{}:
		properties := make(map[string]*JSONSchema)
		for key, val := range v {
			properties[key] = g.inferSchema(val, fmt.Sprintf("%s.%s", path, key))
		}
		return &JSONSchema{
			Type:        "object",
			Properties:  properties,
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
	default:
		return &JSONSchema{
			Type:        "any",
			Default:     v,
			KurelPath:   path,
			KurelSource: "inferred",
		}
	}
}

// traceObject traces field usage in an object
func (g *schemaGenerator) traceObject(obj map[string]interface{}, kind, path string, usage map[string][]string) {
	for key, value := range obj {
		fieldPath := key
		if path != "" {
			fieldPath = fmt.Sprintf("%s.%s", path, key)
		}
		
		// Record usage
		fullPath := fmt.Sprintf("%s:%s", kind, fieldPath)
		
		// Check if this field references a type
		if str, ok := value.(string); ok && strings.Contains(str, "${") {
			// Extract variable references
			vars := extractVariables(str)
			for _, v := range vars {
				usage[v] = append(usage[v], fullPath)
			}
		}
		
		// Recurse into nested objects
		switch v := value.(type) {
		case map[string]interface{}:
			g.traceObject(v, kind, fieldPath, usage)
		case []interface{}:
			for i, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					g.traceObject(m, kind, fmt.Sprintf("%s[%d]", fieldPath, i), usage)
				}
			}
		}
	}
}

// extractVariables extracts variable references from a string
func extractVariables(str string) []string {
	var vars []string
	start := 0
	for {
		idx := strings.Index(str[start:], "${")
		if idx == -1 {
			break
		}
		start += idx
		end := strings.Index(str[start:], "}")
		if end == -1 {
			break
		}
		vars = append(vars, str[start+2:start+end])
		start += end + 1
	}
	return vars
}

// ExportSchema exports a schema to JSON
func (g *schemaGenerator) ExportSchema(schema *JSONSchema) ([]byte, error) {
	return json.MarshalIndent(schema, "", "  ")
}

// DebugSchema generates a debug representation of a schema
func (g *schemaGenerator) DebugSchema(schema *JSONSchema) string {
	var b strings.Builder
	g.debugSchemaRecursive(schema, "", &b)
	return b.String()
}

// debugSchemaRecursive recursively builds debug output
func (g *schemaGenerator) debugSchemaRecursive(schema *JSONSchema, indent string, b *strings.Builder) {
	if schema == nil {
		return
	}
	
	// Type and description
	b.WriteString(fmt.Sprintf("%sType: %s\n", indent, schema.Type))
	if schema.Description != "" {
		b.WriteString(fmt.Sprintf("%sDescription: %s\n", indent, schema.Description))
	}
	
	// Properties
	if len(schema.Properties) > 0 {
		b.WriteString(fmt.Sprintf("%sProperties:\n", indent))
		keys := make([]string, 0, len(schema.Properties))
		for k := range schema.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, key := range keys {
			b.WriteString(fmt.Sprintf("%s  %s:\n", indent, key))
			g.debugSchemaRecursive(schema.Properties[key], indent+"    ", b)
		}
	}
	
	// Array items
	if schema.Items != nil {
		b.WriteString(fmt.Sprintf("%sItems:\n", indent))
		g.debugSchemaRecursive(schema.Items, indent+"  ", b)
	}
	
	// Required fields
	if len(schema.Required) > 0 {
		b.WriteString(fmt.Sprintf("%sRequired: %v\n", indent, schema.Required))
	}
	
	// Constraints
	if schema.Pattern != "" {
		b.WriteString(fmt.Sprintf("%sPattern: %s\n", indent, schema.Pattern))
	}
	if schema.MinLength != nil {
		b.WriteString(fmt.Sprintf("%sMinLength: %d\n", indent, *schema.MinLength))
	}
	if schema.MaxLength != nil {
		b.WriteString(fmt.Sprintf("%sMaxLength: %d\n", indent, *schema.MaxLength))
	}
}

// SetVerbose enables verbose mode
func (g *schemaGenerator) SetVerbose(verbose bool) {
	g.verbose = verbose
}

// Helper functions
func intPtr(i int) *int {
	return &i
}

func float64Ptr(f float64) *float64 {
	return &f
}

// ValidateWithSchema validates data against a schema
func ValidateWithSchema(data interface{}, schema *JSONSchema) []ValidationError {
	var errors []ValidationError
	validateRecursive(data, schema, "$", &errors)
	return errors
}

// validateRecursive recursively validates data against schema
func validateRecursive(data interface{}, schema *JSONSchema, path string, errors *[]ValidationError) {
	if schema == nil {
		return
	}
	
	// Check type
	actualType := getJSONType(data)
	if schema.Type != "" && schema.Type != "any" {
		// Allow integer values for number type (JSON doesn't distinguish)
		if !(schema.Type == "number" && actualType == "integer") && 
		   !(schema.Type == "integer" && actualType == "number") &&
		   actualType != schema.Type {
			*errors = append(*errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("expected type %s but got %s", schema.Type, actualType),
			})
			return
		}
	}
	
	// Validate based on type
	switch schema.Type {
	case "object":
		if obj, ok := data.(map[string]interface{}); ok {
			// Check required fields
			for _, req := range schema.Required {
				val, exists := obj[req]
				if !exists {
					*errors = append(*errors, ValidationError{
						Field:   fmt.Sprintf("%s.%s", path, req),
						Message: "required field missing",
					})
				} else if str, isString := val.(string); isString && str == "" {
					// For required string fields, empty strings are also invalid
					*errors = append(*errors, ValidationError{
						Field:   fmt.Sprintf("%s.%s", path, req),
						Message: "required field cannot be empty",
					})
				}
			}
			
			// Validate properties
			for key, value := range obj {
				if propSchema, exists := schema.Properties[key]; exists {
					validateRecursive(value, propSchema, fmt.Sprintf("%s.%s", path, key), errors)
				}
			}
		}
		
	case "array":
		if arr, ok := data.([]interface{}); ok {
			// Check length constraints
			if schema.MinLength != nil && len(arr) < *schema.MinLength {
				*errors = append(*errors, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("array length %d is less than minimum %d", len(arr), *schema.MinLength),
				})
			}
			
			// Validate items
			if schema.Items != nil {
				for i, item := range arr {
					validateRecursive(item, schema.Items, fmt.Sprintf("%s[%d]", path, i), errors)
				}
			}
		}
		
	case "string":
		if str, ok := data.(string); ok {
			// Check pattern
			if schema.Pattern != "" {
				// Pattern validation would go here
			}
			
			// Check length
			if schema.MinLength != nil && len(str) < *schema.MinLength {
				*errors = append(*errors, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("string length %d is less than minimum %d", len(str), *schema.MinLength),
				})
			}
			if schema.MaxLength != nil && len(str) > *schema.MaxLength {
				*errors = append(*errors, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("string length %d exceeds maximum %d", len(str), *schema.MaxLength),
				})
			}
		}
		
	case "integer", "number":
		if num, ok := getNumber(data); ok {
			// Check range
			if schema.Minimum != nil && num < *schema.Minimum {
				*errors = append(*errors, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("value %v is less than minimum %v", num, *schema.Minimum),
				})
			}
			if schema.Maximum != nil && num > *schema.Maximum {
				*errors = append(*errors, ValidationError{
					Field:   path,
					Message: fmt.Sprintf("value %v exceeds maximum %v", num, *schema.Maximum),
				})
			}
		}
	}
	
	// Check enum values
	if len(schema.Enum) > 0 {
		found := false
		for _, allowed := range schema.Enum {
			if reflect.DeepEqual(data, allowed) {
				found = true
				break
			}
		}
		if !found {
			*errors = append(*errors, ValidationError{
				Field:   path,
				Message: fmt.Sprintf("value %v is not in allowed values %v", data, schema.Enum),
			})
		}
	}
}

// getJSONType returns the JSON type of a value
func getJSONType(v interface{}) string {
	if v == nil {
		return "null"
	}
	switch val := v.(type) {
	case bool:
		return "boolean"
	case int, int32, int64:
		return "integer"
	case float32, float64:
		return "number"
	case string:
		return "string"
	case []interface{}:
		return "array"
	case []string:
		return "array"
	case map[string]interface{}:
		return "object"
	case ParameterMap:
		return "object"
	default:
		// Use reflection for other types
		rv := reflect.ValueOf(val)
		switch rv.Kind() {
		case reflect.Slice, reflect.Array:
			return "array"
		case reflect.Map, reflect.Struct:
			return "object"
		default:
			return "unknown"
		}
	}
}

// getNumber converts various numeric types to float64
func getNumber(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	default:
		return 0, false
	}
}

// MergeSchemas merges multiple schemas into one
func MergeSchemas(schemas ...*JSONSchema) *JSONSchema {
	if len(schemas) == 0 {
		return nil
	}
	if len(schemas) == 1 {
		return schemas[0]
	}
	
	// Start with first schema as base
	result := &JSONSchema{
		Type:       schemas[0].Type,
		Properties: make(map[string]*JSONSchema),
	}
	
	// Merge all schemas
	for _, schema := range schemas {
		if schema == nil {
			continue
		}
		
		// Merge properties
		for key, prop := range schema.Properties {
			if existing, exists := result.Properties[key]; exists {
				// Recursively merge if both are objects
				if existing.Type == "object" && prop.Type == "object" {
					result.Properties[key] = MergeSchemas(existing, prop)
				} else {
					// Otherwise use the latest
					result.Properties[key] = prop
				}
			} else {
				result.Properties[key] = prop
			}
		}
		
		// Merge required fields (union)
		for _, req := range schema.Required {
			found := false
			for _, existing := range result.Required {
				if existing == req {
					found = true
					break
				}
			}
			if !found {
				result.Required = append(result.Required, req)
			}
		}
	}
	
	return result
}