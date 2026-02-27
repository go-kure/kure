package launcher

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
	"github.com/go-kure/kure/pkg/patch"
)

// patchProcessor implements the PatchProcessor interface
type patchProcessor struct {
	logger   logger.Logger
	resolver Resolver
	verbose  bool
}

// NewPatchProcessor creates a new patch processor
func NewPatchProcessor(log logger.Logger, resolver Resolver) PatchProcessor {
	if log == nil {
		log = logger.Default()
	}
	if resolver == nil {
		resolver = NewResolver(log)
	}
	return &patchProcessor{
		logger:   log,
		resolver: resolver,
	}
}

// ResolveDependencies determines which patches to enable based on conditions and dependencies
func (p *patchProcessor) ResolveDependencies(ctx context.Context, patches []Patch, params ParameterMap) ([]Patch, error) {
	p.logger.Debug("Resolving patch dependencies for %d patches", len(patches))

	// Build patch map for quick lookup
	patchMap := make(map[string]*Patch)
	for i := range patches {
		patchMap[patches[i].Name] = &patches[i]
	}

	// Track enabled patches
	enabled := make(map[string]bool)
	resolved := []Patch{}

	// First pass: evaluate conditions
	for _, patch := range patches {
		if p.shouldEnablePatch(patch, params) {
			enabled[patch.Name] = true
			p.logger.Debug("Patch %s enabled by condition", patch.Name)
		} else {
			p.logger.Debug("Patch %s disabled by condition", patch.Name)
		}
	}

	// Second pass: resolve dependencies
	for name := range enabled {
		if err := p.checkDependencies(name, patchMap, enabled); err != nil {
			return nil, err
		}
	}

	// Third pass: check conflicts
	for name := range enabled {
		if err := p.checkConflicts(name, patchMap, enabled); err != nil {
			return nil, err
		}
	}

	// Build final list of enabled patches in dependency order
	ordered := p.orderByDependencies(enabled, patchMap)
	for _, name := range ordered {
		if patch, ok := patchMap[name]; ok {
			resolved = append(resolved, *patch)
		}
	}

	p.logger.Info("Resolved %d patches from %d total", len(resolved), len(patches))
	return resolved, nil
}

// ApplyPatches applies patches to a package definition (returns deep copy)
func (p *patchProcessor) ApplyPatches(ctx context.Context, def *PackageDefinition, patches []Patch, params ParameterMap) (*PackageDefinition, error) {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return nil, errors.Wrap(ctx.Err(), "context cancelled during patch application")
	default:
	}

	if def == nil {
		return nil, errors.Errorf("package definition is nil")
	}

	if len(patches) == 0 {
		p.logger.Debug("No patches to apply")
		return def.DeepCopy(), nil
	}

	p.logger.Info("Applying %d patches to package", len(patches))

	// Create deep copy to maintain immutability
	result := def.DeepCopy()

	// Resolve variables in parameters first
	resolvedParams, err := p.resolver.Resolve(ctx, params, nil, nil)
	if err != nil {
		return nil, errors.Errorf("failed to resolve parameters: %w", err)
	}

	// Convert resolved params to VariableContext for patch engine
	varCtx := p.createVariableContext(resolvedParams)

	// Apply each patch
	for _, patch := range patches {
		p.logger.Debug("Applying patch %s", patch.Name)

		if p.verbose {
			p.logger.Info("Patch %s:\n%s", patch.Name, patch.Content)
		}

		// Parse patch content
		patchSpecs, err := p.parsePatch(patch, varCtx)
		if err != nil {
			return nil, NewPatchError(patch.Name, "", "", "", fmt.Sprintf("failed to parse patch: %v", err))
		}

		// Apply patch to resources
		for i, resource := range result.Resources {
			applied, err := p.applyPatchToResource(&resource, patchSpecs, patch.Name)
			if err != nil {
				return nil, NewPatchError(patch.Name, resource.Kind, resource.GetName(), "", fmt.Sprintf("patch application failed: %v", err))
			}
			if applied {
				result.Resources[i] = resource
				p.logger.Debug("Applied patch %s to resource %s", patch.Name, resource.GetName())
			}
		}
	}

	p.logger.Info("Successfully applied all patches")
	return result, nil
}

// shouldEnablePatch evaluates if a patch should be enabled based on its condition
func (p *patchProcessor) shouldEnablePatch(patch Patch, params ParameterMap) bool {
	if patch.Metadata == nil || patch.Metadata.Enabled == "" {
		// No condition, patch is enabled by default
		return true
	}

	// Evaluate the enabled expression
	enabled := p.evaluateExpression(patch.Metadata.Enabled, params)
	return enabled
}

// evaluateExpression evaluates a simple boolean expression
func (p *patchProcessor) evaluateExpression(expr string, params ParameterMap) bool {
	// Handle variable references like ${feature.enabled}
	if strings.HasPrefix(expr, "${") && strings.HasSuffix(expr, "}") {
		varPath := expr[2 : len(expr)-1]
		value := p.lookupVariable(varPath, params)
		return p.toBool(value)
	}

	// Handle literal values
	return p.toBool(expr)
}

// lookupVariable looks up a variable by path
func (p *patchProcessor) lookupVariable(path string, params ParameterMap) interface{} {
	parts := strings.Split(path, ".")
	current := params

	for i, part := range parts {
		val, ok := current[part]
		if !ok {
			return nil
		}

		if i == len(parts)-1 {
			return val
		}

		if m, ok := val.(map[string]interface{}); ok {
			current = m
		} else if m, ok := val.(ParameterMap); ok {
			current = m
		} else {
			return nil
		}
	}

	return nil
}

// toBool converts a value to boolean
func (p *patchProcessor) toBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		// Empty string is false, any non-empty string (except explicit false values) is true
		if v == "" || v == "false" || v == "no" || v == "0" || v == "disabled" {
			return false
		}
		return true
	case int:
		return v != 0
	case int32:
		return v != 0
	case int64:
		return v != 0
	case float32:
		return v != 0
	case float64:
		return v != 0
	default:
		return value != nil
	}
}

// checkDependencies verifies all required dependencies are enabled
func (p *patchProcessor) checkDependencies(name string, patchMap map[string]*Patch, enabled map[string]bool) error {
	patch, ok := patchMap[name]
	if !ok {
		return errors.Errorf("patch %s not found", name)
	}

	if patch.Metadata == nil {
		return nil
	}

	for _, dep := range patch.Metadata.Requires {
		if !enabled[dep] {
			return NewDependencyError("missing", name, dep, nil)
		}
	}

	return nil
}

// checkConflicts verifies no conflicting patches are enabled
func (p *patchProcessor) checkConflicts(name string, patchMap map[string]*Patch, enabled map[string]bool) error {
	patch, ok := patchMap[name]
	if !ok {
		return errors.Errorf("patch %s not found", name)
	}

	if patch.Metadata == nil {
		return nil
	}

	for _, conflict := range patch.Metadata.Conflicts {
		if enabled[conflict] {
			return NewDependencyError("conflict", name, conflict, nil)
		}
	}

	return nil
}

// orderByDependencies returns patches in dependency order
func (p *patchProcessor) orderByDependencies(enabled map[string]bool, patchMap map[string]*Patch) []string {
	// Build dependency graph
	graph := make(map[string][]string)
	indegree := make(map[string]int)

	for name := range enabled {
		if _, ok := indegree[name]; !ok {
			indegree[name] = 0
		}

		if patch, ok := patchMap[name]; ok && patch.Metadata != nil {
			for _, dep := range patch.Metadata.Requires {
				if enabled[dep] {
					graph[dep] = append(graph[dep], name)
					indegree[name]++
				}
			}
		}
	}

	// Topological sort
	var result []string
	queue := []string{}

	for name, degree := range indegree {
		if degree == 0 {
			queue = append(queue, name)
		}
	}

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		for _, next := range graph[current] {
			indegree[next]--
			if indegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	// If we couldn't order all patches, there's a cycle (shouldn't happen after dependency check)
	if len(result) != len(enabled) {
		// Fall back to alphabetical order
		for name := range enabled {
			found := false
			for _, r := range result {
				if r == name {
					found = true
					break
				}
			}
			if !found {
				result = append(result, name)
			}
		}
		sort.Strings(result)
	}

	return result
}

// createVariableContext converts resolved parameters to patch.VariableContext
func (p *patchProcessor) createVariableContext(params ParameterMapWithSource) *patch.VariableContext {
	// The patch engine expects variables under "values" namespace
	values := make(map[string]interface{})
	features := make(map[string]bool)

	for key, source := range params {
		// Add to values map (flattened)
		p.addToValues(key, source.Value, values)

		// If it's a boolean and starts with "feature", also add to features map
		if b, ok := source.Value.(bool); ok && strings.HasPrefix(key, "feature") {
			// Remove "feature." prefix if present
			featureKey := strings.TrimPrefix(key, "feature.")
			features[featureKey] = b
		}
	}

	return &patch.VariableContext{
		Values:   values,
		Features: features,
	}
}

// addToValues recursively adds parameters to values map
func (p *patchProcessor) addToValues(prefix string, value interface{}, values map[string]interface{}) {
	switch v := value.(type) {
	case map[string]interface{}:
		// Add the map itself
		values[prefix] = v
		// Also add flattened keys
		for k, val := range v {
			key := prefix + "." + k
			p.addToValues(key, val, values)
		}
	case []interface{}:
		// Add the array itself
		values[prefix] = v
		// Also add individual elements
		for i, val := range v {
			key := fmt.Sprintf("%s[%d]", prefix, i)
			values[key] = val
		}
	default:
		// Add simple values directly
		values[prefix] = value
	}
}

// parsePatch parses patch content into PatchSpecs
func (p *patchProcessor) parsePatch(patchDef Patch, varCtx *patch.VariableContext) ([]patch.PatchSpec, error) {
	reader := strings.NewReader(patchDef.Content)
	return patch.LoadPatchFileWithVariables(reader, varCtx)
}

// applyPatchToResource applies patch specs to a resource
func (p *patchProcessor) applyPatchToResource(resource *Resource, specs []patch.PatchSpec, patchName string) (bool, error) {
	if resource.Raw == nil {
		return false, nil
	}

	applied := false

	for _, spec := range specs {
		// Check if this patch targets this resource
		if !p.matchesTarget(resource, spec.Target) {
			continue
		}

		// Apply the patch operation directly
		if err := applyPatchOp(resource.Raw.Object, spec.Patch); err != nil {
			if p.verbose {
				p.logger.Warn("Failed to apply patch %s to %s: %v", patchName, resource.GetName(), err)
			}
			// Continue with other patches
			continue
		}

		applied = true
	}

	return applied, nil
}

// matchesTarget checks if a resource matches a patch target
func (p *patchProcessor) matchesTarget(resource *Resource, target string) bool {
	if target == "" {
		// No target specified, applies to all resources
		return true
	}

	// Parse target format: kind.name or kind/name
	parts := strings.Split(target, ".")
	if len(parts) == 1 {
		parts = strings.Split(target, "/")
	}

	if len(parts) == 1 {
		// Just kind specified
		return strings.EqualFold(resource.Kind, parts[0])
	}

	if len(parts) == 2 {
		// Kind and name specified
		return strings.EqualFold(resource.Kind, parts[0]) && resource.GetName() == parts[1]
	}

	return false
}

// DebugPatchGraph generates a patch dependency graph for debugging
func (p *patchProcessor) DebugPatchGraph(patches []Patch) string {
	graph := &strings.Builder{}
	graph.WriteString("Patch Dependency Graph:\n")
	graph.WriteString("======================\n\n")

	// Build patch map
	patchMap := make(map[string]*Patch)
	for i := range patches {
		patchMap[patches[i].Name] = &patches[i]
	}

	// Sort patch names for consistent output
	var names []string
	for name := range patchMap {
		names = append(names, name)
	}
	sort.Strings(names)

	// Print each patch and its relationships
	for _, name := range names {
		patch := patchMap[name]
		graph.WriteString(fmt.Sprintf("%s:\n", name))

		if patch.Metadata != nil {
			if patch.Metadata.Enabled != "" {
				graph.WriteString(fmt.Sprintf("  Condition: %s\n", patch.Metadata.Enabled))
			}

			if len(patch.Metadata.Requires) > 0 {
				graph.WriteString("  Requires:\n")
				for _, req := range patch.Metadata.Requires {
					graph.WriteString(fmt.Sprintf("    -> %s\n", req))
				}
			}

			if len(patch.Metadata.Conflicts) > 0 {
				graph.WriteString("  Conflicts:\n")
				for _, conf := range patch.Metadata.Conflicts {
					graph.WriteString(fmt.Sprintf("    x %s\n", conf))
				}
			}

			if patch.Metadata.Description != "" {
				graph.WriteString(fmt.Sprintf("  Description: %s\n", patch.Metadata.Description))
			}
		} else {
			graph.WriteString("  (no metadata)\n")
		}

		graph.WriteString("\n")
	}

	// Check for issues
	issues := p.findPatchIssues(patchMap)
	if len(issues) > 0 {
		graph.WriteString("Issues Detected:\n")
		graph.WriteString("===============\n")
		for _, issue := range issues {
			graph.WriteString(fmt.Sprintf("  - %s\n", issue))
		}
	}

	return graph.String()
}

// findPatchIssues detects potential problems in patch configuration
func (p *patchProcessor) findPatchIssues(patchMap map[string]*Patch) []string {
	var issues []string

	for name, patch := range patchMap {
		if patch.Metadata == nil {
			continue
		}

		// Check for missing dependencies
		for _, req := range patch.Metadata.Requires {
			if _, ok := patchMap[req]; !ok {
				issues = append(issues, fmt.Sprintf("Patch %s requires non-existent patch %s", name, req))
			}
		}

		// Check for mutual conflicts
		for _, conf := range patch.Metadata.Conflicts {
			if conflictPatch, ok := patchMap[conf]; ok && conflictPatch.Metadata != nil {
				// Check if the conflict is mutual
				mutual := false
				for _, c := range conflictPatch.Metadata.Conflicts {
					if c == name {
						mutual = true
						break
					}
				}
				if !mutual {
					issues = append(issues, fmt.Sprintf("Patch %s conflicts with %s, but not vice versa", name, conf))
				}
			}
		}

		// Check for circular dependencies
		if p.hasCircularDependency(name, patchMap, make(map[string]bool)) {
			issues = append(issues, fmt.Sprintf("Patch %s has circular dependencies", name))
		}
	}

	return issues
}

// hasCircularDependency checks if a patch has circular dependencies
func (p *patchProcessor) hasCircularDependency(name string, patchMap map[string]*Patch, visited map[string]bool) bool {
	if visited[name] {
		return true
	}

	visited[name] = true
	defer delete(visited, name)

	patch, ok := patchMap[name]
	if !ok || patch.Metadata == nil {
		return false
	}

	for _, req := range patch.Metadata.Requires {
		if p.hasCircularDependency(req, patchMap, visited) {
			return true
		}
	}

	return false
}

// SetVerbose enables verbose mode for debugging
func (p *patchProcessor) SetVerbose(verbose bool) {
	p.verbose = verbose
}

// applyPatchOp applies a patch operation to an object
func applyPatchOp(obj map[string]interface{}, op patch.PatchOp) error {
	// Use the parsed path to navigate and apply the patch
	if len(op.ParsedPath) == 0 && op.Path != "" {
		// Parse the path if not already parsed
		parsed, err := parsePath(op.Path)
		if err != nil {
			return err
		}
		op.ParsedPath = parsed
	}

	return applyOperation(obj, op.ParsedPath, op.Value, op.Op)
}

// parsePath parses a dot-notation path into PathPart components
func parsePath(path string) ([]patch.PathPart, error) {
	if path == "" {
		return nil, nil
	}

	parts := strings.Split(path, ".")
	var result []patch.PathPart

	for _, part := range parts {
		// Check for array selector
		if idx := strings.Index(part, "["); idx > 0 {
			fieldName := part[:idx]
			selectorStr := part[idx+1 : len(part)-1]

			// Try to parse as index
			if _, err := strconv.Atoi(selectorStr); err == nil {
				result = append(result, patch.PathPart{
					Field:      fieldName,
					MatchType:  "index",
					MatchValue: selectorStr,
				})
			} else {
				// It's a selector like [name=value]
				result = append(result, patch.PathPart{
					Field:      fieldName,
					MatchType:  "key",
					MatchValue: selectorStr,
				})
			}
		} else {
			result = append(result, patch.PathPart{
				Field: part,
			})
		}
	}

	return result, nil
}

// applyOperation applies a patch operation at the specified path
func applyOperation(obj map[string]interface{}, path []patch.PathPart, value interface{}, op string) error {
	if len(path) == 0 {
		return errors.Errorf("empty path")
	}

	// Navigate to the target location
	current := obj
	for i := 0; i < len(path)-1; i++ {
		part := path[i]

		if part.MatchType == "index" {
			// Array access by index
			arr, ok := current[part.Field].([]interface{})
			if !ok {
				return errors.Errorf("field %s is not an array", part.Field)
			}
			index, _ := strconv.Atoi(part.MatchValue)
			if index >= len(arr) {
				return errors.Errorf("index %d out of bounds for field %s", index, part.Field)
			}
			if m, ok := arr[index].(map[string]interface{}); ok {
				current = m
			} else {
				return errors.Errorf("array element at %s[%d] is not an object", part.Field, index)
			}
		} else if part.MatchType == "key" {
			// Selector-based array access
			arr, ok := current[part.Field].([]interface{})
			if !ok {
				return errors.Errorf("field %s is not an array", part.Field)
			}

			// Find matching element
			found := false
			for _, elem := range arr {
				if m, ok := elem.(map[string]interface{}); ok {
					if matchesSelector(m, part.MatchValue) {
						current = m
						found = true
						break
					}
				}
			}
			if !found {
				return errors.Errorf("no element matching selector %s in field %s", part.MatchValue, part.Field)
			}
		} else {
			// Regular field access
			if next, ok := current[part.Field].(map[string]interface{}); ok {
				current = next
			} else {
				// Create intermediate objects if needed
				if current[part.Field] == nil {
					current[part.Field] = make(map[string]interface{})
					current = current[part.Field].(map[string]interface{})
				} else {
					return errors.Errorf("field %s is not an object", part.Field)
				}
			}
		}
	}

	// Apply the operation at the final location
	lastPart := path[len(path)-1]

	switch op {
	case "replace", "":
		current[lastPart.Field] = value
	case "delete":
		delete(current, lastPart.Field)
	case "add":
		if arr, ok := current[lastPart.Field].([]interface{}); ok {
			current[lastPart.Field] = append(arr, value)
		} else {
			current[lastPart.Field] = value
		}
	default:
		return errors.Errorf("unsupported operation: %s", op)
	}

	return nil
}

// matchesSelector checks if an object matches a selector string
func matchesSelector(obj map[string]interface{}, selector string) bool {
	// Parse selector like "name=value"
	parts := strings.SplitN(selector, "=", 2)
	if len(parts) != 2 {
		return false
	}

	key := parts[0]
	expectedValue := parts[1]

	actualValue, ok := obj[key]
	if !ok {
		return false
	}

	return fmt.Sprintf("%v", actualValue) == expectedValue
}
