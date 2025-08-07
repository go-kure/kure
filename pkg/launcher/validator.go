package launcher

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// validator implements the Validator interface
type validator struct {
	logger          logger.Logger
	schemaGenerator SchemaGenerator
	strictMode      bool // If true, warnings become errors
	maxErrors       int  // Maximum errors before stopping
	verbose         bool // Verbose mode for debugging
}

// NewValidator creates a new validator
func NewValidator(log logger.Logger) Validator {
	if log == nil {
		log = logger.Default()
	}
	return &validator{
		logger:          log,
		schemaGenerator: NewSchemaGenerator(log),
		maxErrors:       100,
		strictMode:      false,
	}
}

// ValidatePackage validates an entire package definition
func (v *validator) ValidatePackage(ctx context.Context, def *PackageDefinition) (*ValidationResult, error) {
	if def == nil {
		return nil, errors.Errorf("package definition is nil")
	}
	
	v.logger.Debug("Validating package %s", def.Metadata.Name)
	
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}
	
	// Generate package schema
	schema, err := v.schemaGenerator.GeneratePackageSchema(ctx)
	if err != nil {
		return nil, errors.Errorf("failed to generate schema: %w", err)
	}
	
	// Convert package to unstructured for validation
	pkgData := v.packageToMap(def)
	
	// Validate against schema
	schemaErrors := ValidateWithSchema(pkgData, schema)
	for _, err := range schemaErrors {
		// Schema validation errors are always errors (not warnings)
		err.Severity = "error"
		v.addValidationError(result, err)
	}
	
	// Perform semantic validation
	v.validateSemantics(ctx, def, result)
	
	// Validate resources
	v.validateResources(ctx, def.Resources, result)
	
	// Validate patches
	v.validatePatches(ctx, def.Patches, result)
	
	// Validate parameters
	v.validateParameters(ctx, def.Parameters, result)
	
	// Check if we exceeded error limit
	if len(result.Errors) >= v.maxErrors {
		result.Errors = append(result.Errors, ValidationError{
			Path:    "",
			Message: fmt.Sprintf("validation stopped after %d errors", v.maxErrors),
		})
	}
	
	// In strict mode, warnings become errors
	if v.strictMode {
		for _, w := range result.Warnings {
			result.Errors = append(result.Errors, ValidationError{
				Resource: w.Resource,
				Field:    w.Field,
				Message:  w.Message,
			})
		}
		result.Warnings = []ValidationWarning{}
	}
	
	v.logger.Info("Validation complete: %d errors, %d warnings", len(result.Errors), len(result.Warnings))
	
	return result, nil
}

// ValidateResource validates a single resource
func (v *validator) ValidateResource(ctx context.Context, resource Resource) (*ValidationResult, error) {
	v.logger.Debug("Validating resource %s/%s", resource.Kind, resource.GetName())
	
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}
	
	// Basic validation
	if resource.APIVersion == "" {
		v.addError(result, "resource", "apiVersion is required")
	}
	if resource.Kind == "" {
		v.addError(result, "resource", "kind is required")
	}
	if resource.GetName() == "" {
		v.addError(result, "resource.metadata", "name is required")
	}
	
	// Validate against Kubernetes schema if available
	if resource.APIVersion != "" && resource.Kind != "" {
		gv, err := schema.ParseGroupVersion(resource.APIVersion)
		if err != nil {
			v.addWarning(result, "resource.apiVersion", fmt.Sprintf("invalid apiVersion format: %v", err))
		} else {
			gvk := gv.WithKind(resource.Kind)
			resourceSchema, err := v.schemaGenerator.GenerateResourceSchema(ctx, gvk)
			if err != nil {
				v.logger.Warn("Could not generate schema for %s: %v", gvk, err)
			} else {
				// Validate against resource schema
				if resource.Raw != nil {
					schemaErrors := ValidateWithSchema(resource.Raw.Object, resourceSchema)
					for _, err := range schemaErrors {
						// Schema validation errors are always errors (not warnings)
						err.Severity = "error"
						v.addValidationError(result, err)
					}
				}
			}
		}
	}
	
	// Resource-specific validation
	v.validateResourceSpecific(&resource, result)
	return result, nil
}

// ValidatePatch validates a patch definition
func (v *validator) ValidatePatch(ctx context.Context, patch Patch) (*ValidationResult, error) {
	v.logger.Debug("Validating patch %s", patch.Name)
	
	result := &ValidationResult{
		Errors:   []ValidationError{},
		Warnings: []ValidationWarning{},
	}
	
	// Name validation
	if patch.Name == "" {
		v.addError(result, "patch", "name is required")
	} else if !isValidName(patch.Name) {
		v.addError(result, "patch.name", fmt.Sprintf("invalid name format: %s", patch.Name))
	}
	
	// Content validation
	if patch.Content == "" {
		v.addError(result, "patch", "content is required")
	} else {
		// Try to parse patch content
		if err := v.validatePatchContent(patch.Content); err != nil {
			v.addError(result, "patch.content", fmt.Sprintf("invalid patch content: %v", err))
		}
	}
	
	// Metadata validation
	if patch.Metadata != nil {
		// Validate dependencies
		for _, dep := range patch.Metadata.Requires {
			if dep == patch.Name {
				v.addError(result, "patch.metadata.requires", "patch cannot depend on itself")
			}
		}
		
		// Validate conflicts
		for _, conflict := range patch.Metadata.Conflicts {
			if conflict == patch.Name {
				v.addWarning(result, "patch.metadata.conflicts", "patch cannot conflict with itself")
			}
		}
		
		// Validate enabled condition
		if patch.Metadata.Enabled != "" {
			if err := v.validateCondition(patch.Metadata.Enabled); err != nil {
				v.addWarning(result, "patch.metadata.enabled", fmt.Sprintf("potentially invalid condition: %v", err))
			}
		}
	}
	return result, nil
}

// validateSemantics performs semantic validation beyond schema
func (v *validator) validateSemantics(ctx context.Context, def *PackageDefinition, result *ValidationResult) {
	// Check for duplicate resource names within same namespace
	resourceMap := make(map[string]bool)
	for i, resource := range def.Resources {
		key := fmt.Sprintf("%s/%s/%s", resource.Kind, resource.GetNamespace(), resource.GetName())
		if resourceMap[key] {
			v.addError(result, fmt.Sprintf("resources[%d]", i), 
				fmt.Sprintf("duplicate resource: %s", key))
		}
		resourceMap[key] = true
	}
	
	// Check for patch name uniqueness (already enforced at load time, but double-check)
	patchMap := make(map[string]bool)
	for i, patch := range def.Patches {
		if patchMap[patch.Name] {
			v.addError(result, fmt.Sprintf("patches[%d]", i),
				fmt.Sprintf("duplicate patch name: %s", patch.Name))
		}
		patchMap[patch.Name] = true
	}
	
	// Validate patch dependencies exist
	for _, patch := range def.Patches {
		if patch.Metadata != nil {
			for _, req := range patch.Metadata.Requires {
				if !patchMap[req] {
					v.addError(result, fmt.Sprintf("patch[%s].requires", patch.Name),
						fmt.Sprintf("dependency '%s' does not exist", req))
				}
			}
			for _, conflict := range patch.Metadata.Conflicts {
				if !patchMap[conflict] {
					v.addWarning(result, fmt.Sprintf("patch[%s].conflicts", patch.Name),
						fmt.Sprintf("conflict '%s' does not exist", conflict))
				}
			}
		}
	}
	
	// Check for circular dependencies in patches
	if cycles := v.findPatchCycles(def.Patches); len(cycles) > 0 {
		for _, cycle := range cycles {
			v.addError(result, "patches", fmt.Sprintf("circular dependency: %s", strings.Join(cycle, " -> ")))
		}
	}
}

// validateResources validates all resources in the package
func (v *validator) validateResources(ctx context.Context, resources []Resource, result *ValidationResult) {
	for i, resource := range resources {
		resResult, err := v.ValidateResource(ctx, resource)
		if err != nil {
			v.addError(result, fmt.Sprintf("resources[%d]", i), fmt.Sprintf("validation error: %v", err))
			continue
		}
		
		// Merge results
		for _, e := range resResult.Errors {
			e.Path = fmt.Sprintf("resources[%d].%s", i, e.Path)
			result.Errors = append(result.Errors, e)
		}
		for _, w := range resResult.Warnings {
			w.Field = fmt.Sprintf("resources[%d].%s", i, w.Field)
			result.Warnings = append(result.Warnings, w)
		}
		
		if !resResult.IsValid() {
		}
	}
}

// validatePatches validates all patches in the package
func (v *validator) validatePatches(ctx context.Context, patches []Patch, result *ValidationResult) {
	for i, patch := range patches {
		patchResult, err := v.ValidatePatch(ctx, patch)
		if err != nil {
			v.addError(result, fmt.Sprintf("patches[%d]", i), fmt.Sprintf("validation error: %v", err))
			continue
		}
		
		// Merge results
		for _, e := range patchResult.Errors {
			e.Path = fmt.Sprintf("patches[%d].%s", i, e.Path)
			result.Errors = append(result.Errors, e)
		}
		for _, w := range patchResult.Warnings {
			w.Field = fmt.Sprintf("patches[%d].%s", i, w.Field)
			result.Warnings = append(result.Warnings, w)
		}
		
		if !patchResult.IsValid() {
		}
	}
}

// validateParameters validates package parameters
func (v *validator) validateParameters(ctx context.Context, params ParameterMap, result *ValidationResult) {
	// Check for reserved parameter names
	reserved := []string{"kurel", "system", "internal"}
	for key := range params {
		for _, r := range reserved {
			if key == r || strings.HasPrefix(key, r+".") {
				v.addWarning(result, fmt.Sprintf("parameters.%s", key),
					fmt.Sprintf("parameter name '%s' uses reserved prefix", key))
			}
		}
	}
	
	// Check for circular references in parameters
	if cycles := v.findParameterCycles(params); len(cycles) > 0 {
		for _, cycle := range cycles {
			v.addError(result, "parameters", fmt.Sprintf("circular reference: %s", strings.Join(cycle, " -> ")))
		}
	}
}

// validateResourceSpecific performs resource-type specific validation
func (v *validator) validateResourceSpecific(resource *Resource, result *ValidationResult) {
	switch strings.ToLower(resource.Kind) {
	case "deployment", "statefulset", "daemonset":
		v.validateWorkload(resource, result)
	case "service":
		v.validateService(resource, result)
	case "configmap", "secret":
		v.validateConfigMapSecret(resource, result)
	case "ingress":
		v.validateIngress(resource, result)
	}
}

// validateWorkload validates workload resources
func (v *validator) validateWorkload(resource *Resource, result *ValidationResult) {
	if resource.Raw == nil {
		return
	}
	
	// Check for required fields
	spec, found, _ := unstructured.NestedMap(resource.Raw.Object, "spec")
	if !found {
		v.addError(result, "spec", "spec is required for workload resources")
		return
	}
	
	// Check selector
	_, found, _ = unstructured.NestedMap(spec, "selector")
	if !found {
		v.addError(result, "spec.selector", "selector is required")
	}
	
	// Check template
	template, found, _ := unstructured.NestedMap(spec, "template")
	if !found {
		v.addError(result, "spec.template", "template is required")
		return
	}
	
	// Check pod spec
	podSpec, found, _ := unstructured.NestedMap(template, "spec")
	if !found {
		v.addError(result, "spec.template.spec", "pod spec is required")
		return
	}
	
	// Check containers
	containers, found, _ := unstructured.NestedSlice(podSpec, "containers")
	if !found || len(containers) == 0 {
		v.addError(result, "spec.template.spec.containers", "at least one container is required")
	}
	
	// Validate each container
	for i, container := range containers {
		if c, ok := container.(map[string]interface{}); ok {
			// Check name
			name, found, _ := unstructured.NestedString(c, "name")
			if !found || name == "" {
				v.addError(result, fmt.Sprintf("spec.template.spec.containers[%d].name", i), "container name is required")
			}
			
			// Check image
			image, found, _ := unstructured.NestedString(c, "image")
			if !found || image == "" {
				v.addError(result, fmt.Sprintf("spec.template.spec.containers[%d].image", i), "container image is required")
			}
		}
	}
}

// validateService validates service resources
func (v *validator) validateService(resource *Resource, result *ValidationResult) {
	if resource.Raw == nil {
		return
	}
	
	spec, found, _ := unstructured.NestedMap(resource.Raw.Object, "spec")
	if !found {
		v.addError(result, "spec", "spec is required for service resources")
		return
	}
	
	// Check ports
	ports, found, _ := unstructured.NestedSlice(spec, "ports")
	if found {
		for i, port := range ports {
			if p, ok := port.(map[string]interface{}); ok {
				// Check port number
				portNum, found, _ := unstructured.NestedInt64(p, "port")
				if !found {
					v.addError(result, fmt.Sprintf("spec.ports[%d].port", i), "port number is required")
				} else if portNum < 1 || portNum > 65535 {
					v.addError(result, fmt.Sprintf("spec.ports[%d].port", i), "port must be between 1 and 65535")
				}
			}
		}
	}
	
	// Check selector for ClusterIP services
	svcType, _, _ := unstructured.NestedString(spec, "type")
	if svcType == "" || svcType == "ClusterIP" || svcType == "NodePort" || svcType == "LoadBalancer" {
		selector, found, _ := unstructured.NestedMap(spec, "selector")
		if !found || len(selector) == 0 {
			v.addWarning(result, "spec.selector", "selector is recommended for service")
		}
	}
}

// validateConfigMapSecret validates ConfigMap and Secret resources
func (v *validator) validateConfigMapSecret(resource *Resource, result *ValidationResult) {
	if resource.Raw == nil {
		return
	}
	
	// Check for either data or binaryData
	_, dataFound, _ := unstructured.NestedMap(resource.Raw.Object, "data")
	_, binaryFound, _ := unstructured.NestedMap(resource.Raw.Object, "binaryData")
	
	if !dataFound && !binaryFound {
		v.addWarning(result, "", "neither data nor binaryData specified")
	}
	
	// For Secrets, check stringData
	if strings.ToLower(resource.Kind) == "secret" {
		stringData, stringFound, _ := unstructured.NestedMap(resource.Raw.Object, "stringData")
		if !dataFound && !binaryFound && !stringFound {
			v.addWarning(result, "", "no data specified in secret")
		}
		
		// Warn about sensitive data in plain text
		if stringFound && len(stringData) > 0 {
			v.addWarning(result, "stringData", "stringData is not encrypted and will be base64 encoded")
		}
	}
}

// validateIngress validates Ingress resources
func (v *validator) validateIngress(resource *Resource, result *ValidationResult) {
	if resource.Raw == nil {
		return
	}
	
	spec, found, _ := unstructured.NestedMap(resource.Raw.Object, "spec")
	if !found {
		v.addError(result, "spec", "spec is required for ingress resources")
		return
	}
	
	// Check rules
	rules, found, _ := unstructured.NestedSlice(spec, "rules")
	if !found || len(rules) == 0 {
		v.addWarning(result, "spec.rules", "no ingress rules defined")
	}
	
	// Validate each rule
	for i, rule := range rules {
		if r, ok := rule.(map[string]interface{}); ok {
			// Check host
			host, _, _ := unstructured.NestedString(r, "host")
			if host != "" && !isValidHostname(host) {
				v.addError(result, fmt.Sprintf("spec.rules[%d].host", i), fmt.Sprintf("invalid hostname: %s", host))
			}
		}
	}
}

// validatePatchContent validates the syntax of patch content
func (v *validator) validatePatchContent(content string) error {
	// Try to detect format and validate accordingly
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return errors.Errorf("empty patch content")
	}
	
	// Simple validation for now - just check if it looks like valid TOML or YAML
	hasColon := false
	hasBracket := false
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.Contains(line, ":") {
			hasColon = true
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			hasBracket = true
		}
	}
	
	if !hasColon && !hasBracket {
		return errors.Errorf("patch content does not appear to be valid TOML or YAML")
	}
	
	return nil
}

// validateCondition validates a condition expression
func (v *validator) validateCondition(condition string) error {
	// Check for variable references
	if strings.Contains(condition, "${") {
		// Extract variables and check format
		vars := extractVariables(condition)
		for _, varName := range vars {
			if varName == "" {
				return errors.Errorf("empty variable reference")
			}
			// Check for valid variable name format
			if !isValidVariableName(varName) {
				return errors.Errorf("invalid variable name: %s", varName)
			}
		}
	}
	
	// Check for simple boolean literals
	if condition == "true" || condition == "false" {
		return nil
	}
	
	return nil
}

// findPatchCycles finds circular dependencies in patches
func (v *validator) findPatchCycles(patches []Patch) [][]string {
	var cycles [][]string
	
	// Build dependency graph
	deps := make(map[string][]string)
	for _, patch := range patches {
		if patch.Metadata != nil && len(patch.Metadata.Requires) > 0 {
			deps[patch.Name] = patch.Metadata.Requires
		}
	}
	
	// Find cycles using DFS
	visited := make(map[string]int) // 0: unvisited, 1: visiting, 2: visited
	var path []string
	
	var dfs func(node string) bool
	dfs = func(node string) bool {
		if visited[node] == 1 {
			// Found cycle
			cycleStart := -1
			for i, n := range path {
				if n == node {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := append([]string{}, path[cycleStart:]...)
				cycle = append(cycle, node)
				cycles = append(cycles, cycle)
			}
			return true
		}
		if visited[node] == 2 {
			return false
		}
		
		visited[node] = 1
		path = append(path, node)
		
		for _, dep := range deps[node] {
			if dfs(dep) {
				// Don't return immediately to find all cycles
			}
		}
		
		path = path[:len(path)-1]
		visited[node] = 2
		return false
	}
	
	for node := range deps {
		if visited[node] == 0 {
			dfs(node)
		}
	}
	
	return cycles
}

// findParameterCycles finds circular references in parameters
func (v *validator) findParameterCycles(params ParameterMap) [][]string {
	var cycles [][]string
	
	// Build dependency graph from variable references
	deps := make(map[string][]string)
	for key, value := range params {
		if str, ok := value.(string); ok && strings.Contains(str, "${") {
			vars := extractVariables(str)
			for _, v := range vars {
				// Only track dependencies within parameters
				if _, exists := params[v]; exists {
					deps[key] = append(deps[key], v)
				}
			}
		}
	}
	
	// Use same cycle detection as patches
	visited := make(map[string]int)
	var path []string
	
	var dfs func(node string) bool
	dfs = func(node string) bool {
		if visited[node] == 1 {
			// Found cycle
			cycleStart := -1
			for i, n := range path {
				if n == node {
					cycleStart = i
					break
				}
			}
			if cycleStart >= 0 {
				cycle := append([]string{}, path[cycleStart:]...)
				cycle = append(cycle, node)
				cycles = append(cycles, cycle)
			}
			return true
		}
		if visited[node] == 2 {
			return false
		}
		
		visited[node] = 1
		path = append(path, node)
		
		for _, dep := range deps[node] {
			dfs(dep)
		}
		
		path = path[:len(path)-1]
		visited[node] = 2
		return false
	}
	
	for node := range deps {
		if visited[node] == 0 {
			dfs(node)
		}
	}
	
	return cycles
}

// packageToMap converts a PackageDefinition to a map for validation
func (v *validator) packageToMap(def *PackageDefinition) map[string]interface{} {
	metadata := map[string]interface{}{
		"name":        def.Metadata.Name,
		"version":     def.Metadata.Version,
		"appVersion":  def.Metadata.AppVersion,
		"description": def.Metadata.Description,
		"home":        def.Metadata.Home,
	}
	
	// Only add arrays if they're not nil
	if def.Metadata.Keywords != nil {
		metadata["keywords"] = def.Metadata.Keywords
	} else {
		metadata["keywords"] = []string{}
	}
	
	if def.Metadata.Schemas != nil {
		metadata["schemas"] = def.Metadata.Schemas
	} else {
		metadata["schemas"] = []string{}
	}
	
	if def.Metadata.Maintainers != nil {
		metadata["maintainers"] = def.Metadata.Maintainers
	}
	
	result := map[string]interface{}{
		"path":     def.Path,
		"metadata": metadata,
	}
	
	// Only add parameters if not nil
	if def.Parameters != nil {
		result["parameters"] = def.Parameters
	} else {
		result["parameters"] = make(map[string]interface{})
	}
	
	// Convert resources
	var resources []interface{}
	for _, r := range def.Resources {
		if r.Raw != nil {
			resources = append(resources, r.Raw.Object)
		}
	}
	if len(resources) > 0 {
		result["resources"] = resources
	}
	
	// Convert patches
	var patches []interface{}
	for _, p := range def.Patches {
		patchMap := map[string]interface{}{
			"name":    p.Name,
			"content": p.Content,
		}
		if p.Metadata != nil {
			patchMap["metadata"] = map[string]interface{}{
				"description": p.Metadata.Description,
				"enabled":     p.Metadata.Enabled,
				"requires":    p.Metadata.Requires,
				"conflicts":   p.Metadata.Conflicts,
			}
		}
		patches = append(patches, patchMap)
	}
	if len(patches) > 0 {
		result["patches"] = patches
	}
	
	return result
}

// Helper functions

func (v *validator) addError(result *ValidationResult, path, message string) {
	result.Errors = append(result.Errors, ValidationError{
		Path:    path,
		Message: message,
	})
}

func (v *validator) addWarning(result *ValidationResult, path, message string) {
	result.Warnings = append(result.Warnings, ValidationWarning{
		Field:   path,
		Message: message,
	})
}

func (v *validator) addValidationError(result *ValidationResult, err ValidationError) {
	if v.strictMode || err.Severity == "error" {
		result.Errors = append(result.Errors, err)
	} else {
		result.Warnings = append(result.Warnings, ValidationWarning{
			Resource: err.Resource,
			Field:    err.Field,
			Message:  err.Message,
		})
	}
}

// isValidName checks if a name follows Kubernetes naming conventions
func isValidName(name string) bool {
	if len(name) == 0 || len(name) > 253 {
		return false
	}
	pattern := `^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

// isValidHostname checks if a hostname is valid
func isValidHostname(host string) bool {
	if len(host) == 0 || len(host) > 253 {
		return false
	}
	pattern := `^([a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?$`
	matched, _ := regexp.MatchString(pattern, host)
	return matched
}

// isValidVariableName checks if a variable name is valid
func isValidVariableName(name string) bool {
	// Variable names can have dots and brackets for nested access
	pattern := `^[a-zA-Z_][a-zA-Z0-9_]*(\.[a-zA-Z_][a-zA-Z0-9_]*|\[[0-9]+\])*$`
	matched, _ := regexp.MatchString(pattern, name)
	return matched
}

// SetStrictMode enables or disables strict validation mode
func (v *validator) SetStrictMode(strict bool) {
	v.strictMode = strict
}

// SetMaxErrors sets the maximum number of errors before stopping
func (v *validator) SetMaxErrors(max int) {
	v.maxErrors = max
}

// SetVerbose enables verbose mode
func (v *validator) SetVerbose(verbose bool) {
	v.verbose = verbose
}

// FormatResult formats a validation result for display
func FormatResult(result *ValidationResult) string {
	var b strings.Builder
	
	if result.IsValid() {
		b.WriteString("✓ Package is valid\n")
	} else {
		b.WriteString("✗ Package validation failed\n")
	}
	
	if len(result.Errors) > 0 {
		b.WriteString(fmt.Sprintf("\nErrors (%d):\n", len(result.Errors)))
		for _, err := range result.Errors {
			path := err.Path
			if path == "" {
				path = err.Field
			}
			if path != "" {
				b.WriteString(fmt.Sprintf("  - %s: %s\n", path, err.Message))
			} else {
				b.WriteString(fmt.Sprintf("  - %s\n", err.Message))
			}
			if err.Resource != "" {
				b.WriteString(fmt.Sprintf("    Resource: %s\n", err.Resource))
			}
		}
	}
	
	if len(result.Warnings) > 0 {
		b.WriteString(fmt.Sprintf("\nWarnings (%d):\n", len(result.Warnings)))
		for _, warn := range result.Warnings {
			if warn.Field != "" {
				b.WriteString(fmt.Sprintf("  - %s: %s\n", warn.Field, warn.Message))
			} else {
				b.WriteString(fmt.Sprintf("  - %s\n", warn.Message))
			}
		}
	}
	
	return b.String()
}