package launcher

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
)

// variableResolver implements the Resolver interface
type variableResolver struct {
	logger   logger.Logger
	maxDepth int

	// Memoization for resolved values
	cache map[string]interface{}

	// Track variables being resolved to detect cycles
	resolving map[string]bool
}

// NewResolver creates a new variable resolver
func NewResolver(log logger.Logger) Resolver {
	if log == nil {
		log = logger.Default()
	}
	return &variableResolver{
		logger:    log,
		maxDepth:  10,
		cache:     make(map[string]interface{}),
		resolving: make(map[string]bool),
	}
}

// Variable reference pattern: ${var.name} or ${var.nested.path}
var varPattern = regexp.MustCompile(`\$\{([^}]+)\}`)

// Resolve substitutes variable references in parameters
func (r *variableResolver) Resolve(ctx context.Context, base, overrides ParameterMap, opts *LauncherOptions) (ParameterMapWithSource, error) {
	if opts == nil {
		opts = DefaultOptions()
	}

	// Set max depth from options
	if opts.MaxDepth > 0 {
		r.maxDepth = opts.MaxDepth
	}

	// Clear cache for new resolution
	r.cache = make(map[string]interface{})
	r.resolving = make(map[string]bool)

	r.logger.Debug("Starting variable resolution with max depth %d", r.maxDepth)

	// Merge parameters (overrides take precedence)
	merged := r.mergeParameters(base, overrides)

	// Create result with source tracking
	result := make(ParameterMapWithSource)

	// Resolve each parameter
	for key, value := range merged {
		source := r.determineSource(key, base, overrides)

		// Clear resolution tracking for each top-level parameter
		r.resolving = make(map[string]bool)

		resolved, err := r.resolveValue(ctx, key, value, merged, 0)
		if err != nil {
			r.logger.Error("Failed to resolve parameter %s: %v", key, err)
			return nil, NewVariableError(key, fmt.Sprintf("%v", value), err.Error())
		}

		result[key] = ParameterSource{
			Value:    resolved,
			Location: source,
			File:     r.getSourceFile(source),
		}
	}

	r.logger.Info("Resolved %d parameters", len(result))
	return result, nil
}

// resolveValue recursively resolves a single value
func (r *variableResolver) resolveValue(ctx context.Context, path string, value interface{}, params ParameterMap, depth int) (interface{}, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "context cancelled during resolution")
	default:
	}

	// Check depth limit
	if depth > r.maxDepth {
		return nil, errors.Errorf("maximum substitution depth %d exceeded", r.maxDepth)
	}

	// Check if we're already resolving this variable (cycle detection)
	if r.resolving[path] {
		return nil, errors.Errorf("cyclic reference detected for %s", path)
	}

	// Check cache
	if cached, ok := r.cache[path]; ok {
		return cached, nil
	}

	// Mark as resolving
	r.resolving[path] = true
	defer func() { delete(r.resolving, path) }()

	// Handle different value types
	switch v := value.(type) {
	case string:
		resolved, err := r.resolveString(ctx, v, params, depth)
		if err != nil {
			return nil, err
		}
		r.cache[path] = resolved
		return resolved, nil

	case map[string]interface{}:
		// Recursively resolve map values
		resolved := make(map[string]interface{})
		for k, val := range v {
			childPath := fmt.Sprintf("%s.%s", path, k)
			resolvedVal, err := r.resolveValue(ctx, childPath, val, params, depth)
			if err != nil {
				return nil, err
			}
			resolved[k] = resolvedVal
		}
		r.cache[path] = resolved
		return resolved, nil

	case []interface{}:
		// Recursively resolve array values
		resolved := make([]interface{}, len(v))
		for i, val := range v {
			childPath := fmt.Sprintf("%s[%d]", path, i)
			resolvedVal, err := r.resolveValue(ctx, childPath, val, params, depth)
			if err != nil {
				return nil, err
			}
			resolved[i] = resolvedVal
		}
		r.cache[path] = resolved
		return resolved, nil

	default:
		// Primitive values don't need resolution
		r.cache[path] = value
		return value, nil
	}
}

// resolveString resolves variable references in a string
func (r *variableResolver) resolveString(ctx context.Context, s string, params ParameterMap, depth int) (interface{}, error) {
	// Find all variable references
	matches := varPattern.FindAllStringSubmatch(s, -1)
	if len(matches) == 0 {
		return s, nil // No variables to resolve
	}

	// If the string is exactly one variable reference, return the resolved value directly
	if len(matches) == 1 && s == matches[0][0] {
		varPath := matches[0][1]
		value := r.lookupVariable(varPath, params)
		if value == nil {
			return nil, errors.Errorf("undefined variable: %s", varPath)
		}

		// Recursively resolve the value
		return r.resolveValue(ctx, varPath, value, params, depth+1)
	}

	// Multiple variables or mixed content - perform string substitution
	result := s
	for _, match := range matches {
		fullMatch := match[0] // ${var.name}
		varPath := match[1]   // var.name

		value := r.lookupVariable(varPath, params)
		if value == nil {
			return nil, errors.Errorf("undefined variable: %s", varPath)
		}

		// Recursively resolve the value
		resolved, err := r.resolveValue(ctx, varPath, value, params, depth+1)
		if err != nil {
			return nil, err
		}

		// Convert to string for substitution
		strValue := r.valueToString(resolved)
		result = strings.Replace(result, fullMatch, strValue, 1)
	}

	return result, nil
}

// lookupVariable looks up a variable by path (e.g., "app.name" or "feature.enabled")
func (r *variableResolver) lookupVariable(path string, params ParameterMap) interface{} {
	parts := strings.Split(path, ".")
	current := params

	for i, part := range parts {
		// Handle array index notation (e.g., items[0])
		if idx := strings.Index(part, "["); idx > 0 {
			arrayName := part[:idx]
			indexStr := part[idx+1 : len(part)-1]

			// Get the array
			val, ok := current[arrayName]
			if !ok {
				return nil
			}

			// Convert to array
			arr, ok := val.([]interface{})
			if !ok {
				return nil
			}

			// Parse index
			var index int
			if _, err := fmt.Sscanf(indexStr, "%d", &index); err != nil {
				return nil
			}

			// Check bounds
			if index < 0 || index >= len(arr) {
				return nil
			}

			// If this is the last part, return the array element
			if i == len(parts)-1 {
				return arr[index]
			}

			// Otherwise, continue traversing
			if m, ok := arr[index].(map[string]interface{}); ok {
				current = m
			} else {
				return nil
			}
		} else {
			// Regular map lookup
			val, ok := current[part]
			if !ok {
				return nil
			}

			// If this is the last part, return the value
			if i == len(parts)-1 {
				return val
			}

			// Otherwise, continue traversing
			if m, ok := val.(map[string]interface{}); ok {
				current = m
			} else if m, ok := val.(ParameterMap); ok {
				current = m
			} else {
				return nil
			}
		}
	}

	return nil
}

// valueToString converts a value to string for substitution
func (r *variableResolver) valueToString(value interface{}) string {
	switch v := value.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%t", v)
	case int, int32, int64, float32, float64:
		return fmt.Sprintf("%v", v)
	default:
		// For complex types, use JSON-like representation
		return fmt.Sprintf("%v", v)
	}
}

// mergeParameters merges base and override parameters
func (r *variableResolver) mergeParameters(base, overrides ParameterMap) ParameterMap {
	result := make(ParameterMap)

	// Copy base parameters
	for k, v := range base {
		result[k] = r.deepCopyValue(v)
	}

	// Apply overrides
	for k, v := range overrides {
		result[k] = r.deepCopyValue(v)
	}

	return result
}

// deepCopyValue creates a deep copy of a value
func (r *variableResolver) deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = r.deepCopyValue(val)
		}
		return result
	case ParameterMap:
		result := make(ParameterMap)
		for k, val := range v {
			result[k] = r.deepCopyValue(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = r.deepCopyValue(val)
		}
		return result
	default:
		return v
	}
}

// determineSource determines where a parameter came from
func (r *variableResolver) determineSource(key string, base, overrides ParameterMap) string {
	if _, ok := overrides[key]; ok {
		return "local"
	}
	if _, ok := base[key]; ok {
		return "package"
	}
	return "default"
}

// getSourceFile returns the file path for a source location
func (r *variableResolver) getSourceFile(source string) string {
	switch source {
	case "package":
		return "parameters.yaml"
	case "local":
		return "parameters.local.yaml"
	default:
		return ""
	}
}

// DebugVariableGraph generates a dependency graph for debugging
func (r *variableResolver) DebugVariableGraph(params ParameterMap) string {
	graph := &strings.Builder{}
	graph.WriteString("Variable Dependency Graph:\n")
	graph.WriteString("==========================\n\n")

	// Find all variables and their dependencies
	deps := r.findDependencies(params)

	// Sort keys for consistent output
	var keys []string
	for k := range deps {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// Print each variable and its dependencies
	for _, key := range keys {
		graph.WriteString(fmt.Sprintf("%s:\n", key))
		if len(deps[key]) == 0 {
			graph.WriteString("  (no dependencies)\n")
		} else {
			for _, dep := range deps[key] {
				graph.WriteString(fmt.Sprintf("  -> %s\n", dep))
			}
		}
	}

	// Check for cycles
	cycles := r.findCycles(deps)
	if len(cycles) > 0 {
		graph.WriteString("\nCycles Detected:\n")
		graph.WriteString("================\n")
		for _, cycle := range cycles {
			graph.WriteString(fmt.Sprintf("  %s\n", strings.Join(cycle, " -> ")))
		}
	}

	return graph.String()
}

// findDependencies finds variable dependencies in parameters
func (r *variableResolver) findDependencies(params ParameterMap) map[string][]string {
	deps := make(map[string][]string)

	var findDepsInValue func(path string, value interface{})
	findDepsInValue = func(path string, value interface{}) {
		// Initialize the path in deps map even if no dependencies
		if path != "" && deps[path] == nil {
			deps[path] = []string{}
		}

		switch v := value.(type) {
		case string:
			// Find variable references
			matches := varPattern.FindAllStringSubmatch(v, -1)
			for _, match := range matches {
				varPath := match[1]
				if path != "" {
					deps[path] = append(deps[path], varPath)
				}
			}
		case map[string]interface{}:
			for k, val := range v {
				childPath := k
				if path != "" {
					childPath = fmt.Sprintf("%s.%s", path, k)
				}
				findDepsInValue(childPath, val)
			}
		case ParameterMap:
			for k, val := range v {
				childPath := k
				if path != "" {
					childPath = fmt.Sprintf("%s.%s", path, k)
				}
				findDepsInValue(childPath, val)
			}
		case []interface{}:
			for i, val := range v {
				childPath := fmt.Sprintf("%s[%d]", path, i)
				if path == "" {
					childPath = fmt.Sprintf("[%d]", i)
				}
				findDepsInValue(childPath, val)
			}
		}
	}

	// Process all parameters at root level
	for k, v := range params {
		findDepsInValue(k, v)
	}

	return deps
}

// findCycles detects cycles in the dependency graph
func (r *variableResolver) findCycles(deps map[string][]string) [][]string {
	var cycles [][]string
	visited := make(map[string]bool)
	recStack := make(map[string]bool)
	path := []string{}

	var dfs func(node string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true
		path = append(path, node)

		for _, dep := range deps[node] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				// Found a cycle
				cycleStart := 0
				for i, n := range path {
					if n == dep {
						cycleStart = i
						break
					}
				}
				cycle := append([]string{}, path[cycleStart:]...)
				cycle = append(cycle, dep) // Complete the cycle
				cycles = append(cycles, cycle)
				return true
			}
		}

		path = path[:len(path)-1]
		recStack[node] = false
		return false
	}

	// Check each node
	for node := range deps {
		if !visited[node] {
			dfs(node)
		}
	}

	return cycles
}
