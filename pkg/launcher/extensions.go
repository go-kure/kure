package launcher

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/logger"
)

// extensionLoader implements the ExtensionLoader interface
type extensionLoader struct {
	logger logger.Logger
	loader PackageLoader
}

// NewExtensionLoader creates a new extension loader
func NewExtensionLoader(log logger.Logger) ExtensionLoader {
	if log == nil {
		log = logger.Default()
	}
	return &extensionLoader{
		logger: log,
		loader: NewPackageLoader(log),
	}
}

// LoadWithExtensions loads a package with local extensions
func (e *extensionLoader) LoadWithExtensions(ctx context.Context, def *PackageDefinition, localPath string, opts *LauncherOptions) (*PackageDefinition, error) {
	if def == nil {
		return nil, errors.New("package definition is nil")
	}

	if opts == nil {
		opts = DefaultOptions()
	}

	// Look for local extension files
	extensions, err := e.findExtensions(def.Path, localPath, opts)
	if err != nil {
		return nil, errors.Wrap(err, "failed to find extensions")
	}

	if len(extensions) == 0 {
		e.logger.Debug("No local extensions found")
		return def, nil
	}

	e.logger.Info("Found %d local extensions", len(extensions))

	// Apply each extension
	result := def.DeepCopy()
	for _, ext := range extensions {
		if err := e.applyExtension(ctx, result, ext, opts); err != nil {
			if opts.StrictMode {
				return nil, errors.Wrap(err, "failed to apply extension")
			}
			e.logger.Warn("Failed to apply extension %s: %v", ext.Path, err)
		}
	}

	return result, nil
}

// LocalExtension represents a local customization file
type LocalExtension struct {
	Path       string                  `yaml:"-"`
	Type       ExtensionType           `yaml:"type,omitempty"`
	Metadata   KurelMetadata           `yaml:"metadata,omitempty"`
	Parameters ParameterMap            `yaml:"parameters,omitempty"`
	Patches    []LocalPatch            `yaml:"patches,omitempty"`
	Resources  []LocalResourceOverride `yaml:"resources,omitempty"`
	Remove     []ResourceSelector      `yaml:"remove,omitempty"`
}

// ExtensionType defines the type of extension
type ExtensionType string

const (
	ExtensionTypeOverride ExtensionType = "override" // Override existing values
	ExtensionTypeMerge    ExtensionType = "merge"    // Merge with existing values
	ExtensionTypeReplace  ExtensionType = "replace"  // Replace entirely
)

// LocalPatch represents a patch definition in local extension
type LocalPatch struct {
	Name        string         `yaml:"name"`
	Description string         `yaml:"description,omitempty"`
	Enabled     string         `yaml:"enabled,omitempty"`
	Content     string         `yaml:"content"`
	Metadata    *PatchMetadata `yaml:"metadata,omitempty"`
}

// LocalResourceOverride represents resource modifications
type LocalResourceOverride struct {
	Selector ResourceSelector       `yaml:"selector"`
	Override map[string]interface{} `yaml:"override,omitempty"`
	Merge    map[string]interface{} `yaml:"merge,omitempty"`
	Remove   []string               `yaml:"remove,omitempty"`
}

// ResourceSelector identifies resources to modify
type ResourceSelector struct {
	Kind      string            `yaml:"kind,omitempty"`
	Name      string            `yaml:"name,omitempty"`
	Namespace string            `yaml:"namespace,omitempty"`
	Labels    map[string]string `yaml:"labels,omitempty"`
}

// findExtensions locates all local extension files
func (e *extensionLoader) findExtensions(packagePath, localPath string, opts *LauncherOptions) ([]LocalExtension, error) {
	var extensions []LocalExtension

	// Determine search paths
	searchPaths := e.getSearchPaths(packagePath, localPath, opts)

	for _, searchPath := range searchPaths {
		// Look for .local.kurel files
		pattern := filepath.Join(searchPath, "*.local.kurel")
		matches, err := filepath.Glob(pattern)
		if err != nil {
			e.logger.Warn("Failed to search for extensions in %s: %v", searchPath, err)
			continue
		}

		// Also check for .local.yaml files
		yamlPattern := filepath.Join(searchPath, "*.local.yaml")
		yamlMatches, err := filepath.Glob(yamlPattern)
		if err != nil {
			e.logger.Warn("Failed to search for yaml extensions in %s: %v", searchPath, err)
		} else {
			matches = append(matches, yamlMatches...)
		}

		// Load each extension file
		for _, match := range matches {
			ext, err := e.loadExtension(match)
			if err != nil {
				e.logger.Warn("Failed to load extension %s: %v", match, err)
				continue
			}
			extensions = append(extensions, *ext)
		}
	}

	// Sort by priority (alphabetical by filename)
	e.sortExtensions(extensions)

	return extensions, nil
}

// getSearchPaths returns paths to search for extensions
func (e *extensionLoader) getSearchPaths(packagePath, localPath string, opts *LauncherOptions) []string {
	var paths []string

	// Priority 1: Explicit local path
	if localPath != "" {
		paths = append(paths, localPath)
	}

	// Priority 2: Package directory
	if packagePath != "" {
		paths = append(paths, packagePath)

		// Also check parent directory for workspace-level extensions
		parent := filepath.Dir(packagePath)
		if parent != "." && parent != "/" {
			paths = append(paths, parent)
		}
	}

	// Priority 3: Current working directory
	if cwd, err := os.Getwd(); err == nil {
		paths = append(paths, cwd)
	}

	// Priority 4: Home directory .kurel folder
	if home, err := os.UserHomeDir(); err == nil {
		kurelDir := filepath.Join(home, ".kurel", "extensions")
		paths = append(paths, kurelDir)
	}

	// Remove duplicates
	seen := make(map[string]bool)
	var unique []string
	for _, p := range paths {
		abs, err := filepath.Abs(p)
		if err != nil {
			abs = p
		}
		if !seen[abs] {
			seen[abs] = true
			unique = append(unique, abs)
		}
	}

	return unique
}

// loadExtension loads a single extension file
func (e *extensionLoader) loadExtension(path string) (*LocalExtension, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, errors.NewFileError("read", path, "failed to read extension file", err)
	}

	var ext LocalExtension
	if err := yaml.Unmarshal(data, &ext); err != nil {
		return nil, errors.NewParseError(path, "invalid YAML syntax", 0, 0, err)
	}

	ext.Path = path

	// Set default type if not specified
	if ext.Type == "" {
		ext.Type = ExtensionTypeMerge
	}

	return &ext, nil
}

// applyExtension applies a single extension to the package
func (e *extensionLoader) applyExtension(ctx context.Context, def *PackageDefinition, ext LocalExtension, opts *LauncherOptions) error {
	e.logger.Debug("Applying extension from %s (type: %s)", ext.Path, ext.Type)

	// Apply parameter extensions
	if len(ext.Parameters) > 0 {
		e.applyParameterExtension(def, ext.Parameters, ext.Type)
	}

	// Apply patch extensions
	if len(ext.Patches) > 0 {
		e.applyPatchExtension(def, ext.Patches, ext.Type)
	}

	// Apply resource modifications
	if len(ext.Resources) > 0 {
		if err := e.applyResourceExtension(ctx, def, ext.Resources); err != nil {
			return errors.Wrap(err, "failed to apply resource extensions")
		}
	}

	// Remove resources if specified
	if len(ext.Remove) > 0 {
		e.removeResources(def, ext.Remove)
	}

	return nil
}

// applyParameterExtension applies parameter modifications
func (e *extensionLoader) applyParameterExtension(def *PackageDefinition, params ParameterMap, extType ExtensionType) {
	switch extType {
	case ExtensionTypeReplace:
		def.Parameters = params
	case ExtensionTypeOverride:
		// Override only specified keys
		for k, v := range params {
			def.Parameters[k] = v
		}
	case ExtensionTypeMerge:
		// Deep merge parameters
		def.Parameters = e.mergeParameters(def.Parameters, params)
	}
}

// applyPatchExtension applies patch modifications
func (e *extensionLoader) applyPatchExtension(def *PackageDefinition, patches []LocalPatch, extType ExtensionType) {
	switch extType {
	case ExtensionTypeReplace:
		// Replace all patches
		def.Patches = []Patch{}
		for _, p := range patches {
			def.Patches = append(def.Patches, Patch{
				Name:     p.Name,
				Content:  p.Content,
				Metadata: p.Metadata,
			})
		}
	case ExtensionTypeOverride, ExtensionTypeMerge:
		// Add or update patches
		patchMap := make(map[string]int) // Map name to index
		for i := range def.Patches {
			patchMap[def.Patches[i].Name] = i
		}

		for _, p := range patches {
			if idx, ok := patchMap[p.Name]; ok {
				// Update existing patch in place
				def.Patches[idx].Content = p.Content
				if p.Metadata != nil {
					def.Patches[idx].Metadata = p.Metadata
				}
			} else {
				// Add new patch
				def.Patches = append(def.Patches, Patch{
					Name:     p.Name,
					Content:  p.Content,
					Metadata: p.Metadata,
				})
			}
		}
	}
}

// applyResourceExtension applies resource modifications
func (e *extensionLoader) applyResourceExtension(ctx context.Context, def *PackageDefinition, overrides []LocalResourceOverride) error {
	for _, override := range overrides {
		// Find matching resources
		for i := range def.Resources {
			if e.matchesSelector(&def.Resources[i], override.Selector) {
				// Apply override
				if err := e.applyResourceOverride(&def.Resources[i], override); err != nil {
					return errors.Wrap(err, "failed to apply resource override")
				}
			}
		}
	}
	return nil
}

// matchesSelector checks if a resource matches a selector
func (e *extensionLoader) matchesSelector(resource *Resource, selector ResourceSelector) bool {
	// Check kind
	if selector.Kind != "" && selector.Kind != resource.Kind {
		return false
	}

	// Check name (supports wildcards)
	if selector.Name != "" {
		if !e.matchesPattern(resource.Metadata.Name, selector.Name) {
			return false
		}
	}

	// Check namespace
	if selector.Namespace != "" && selector.Namespace != resource.Metadata.Namespace {
		return false
	}

	// Check labels
	if len(selector.Labels) > 0 {
		resourceLabels := resource.Metadata.Labels
		for k, v := range selector.Labels {
			if resourceLabels[k] != v {
				return false
			}
		}
	}

	return true
}

// matchesPattern checks if a string matches a pattern (supports * wildcard)
func (e *extensionLoader) matchesPattern(s, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.Contains(pattern, "*") {
		// Simple wildcard matching
		parts := strings.Split(pattern, "*")
		if len(parts) == 2 {
			return strings.HasPrefix(s, parts[0]) && strings.HasSuffix(s, parts[1])
		}
	}
	return s == pattern
}

// applyResourceOverride applies override to a resource
func (e *extensionLoader) applyResourceOverride(resource *Resource, override LocalResourceOverride) error {
	if resource.Raw == nil {
		return nil
	}

	obj := resource.Raw.Object

	// Apply overrides (replace values)
	if override.Override != nil {
		for path, value := range override.Override {
			if err := setNestedField(obj, value, strings.Split(path, ".")...); err != nil {
				return errors.Wrap(err, "failed to override field")
			}
		}
	}

	// Apply merges (deep merge)
	if override.Merge != nil {
		for path, value := range override.Merge {
			if err := mergeNestedField(obj, value, strings.Split(path, ".")...); err != nil {
				return errors.Wrap(err, "failed to merge field")
			}
		}
	}

	// Remove fields
	for _, path := range override.Remove {
		removeNestedField(obj, strings.Split(path, ".")...)
	}

	return nil
}

// removeResources removes resources matching selectors
func (e *extensionLoader) removeResources(def *PackageDefinition, selectors []ResourceSelector) {
	var filtered []Resource

	for _, resource := range def.Resources {
		remove := false
		for _, selector := range selectors {
			if e.matchesSelector(&resource, selector) {
				remove = true
				e.logger.Debug("Removing resource %s/%s", resource.Kind, resource.Metadata.Name)
				break
			}
		}
		if !remove {
			filtered = append(filtered, resource)
		}
	}

	def.Resources = filtered
}

// mergeParameters performs deep merge of parameter maps
func (e *extensionLoader) mergeParameters(base, overlay ParameterMap) ParameterMap {
	result := make(ParameterMap)

	// Copy base
	for k, v := range base {
		result[k] = e.deepCopyValue(v)
	}

	// Merge overlay
	for k, v := range overlay {
		if existing, ok := result[k]; ok {
			// Merge if both are maps
			if baseMap, ok1 := existing.(map[string]interface{}); ok1 {
				if overlayMap, ok2 := v.(map[string]interface{}); ok2 {
					result[k] = e.mergeMaps(baseMap, overlayMap)
					continue
				}
			}
		}
		// Otherwise replace
		result[k] = e.deepCopyValue(v)
	}

	return result
}

// mergeMaps performs deep merge of two maps
func (e *extensionLoader) mergeMaps(base, overlay map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})

	// Copy base
	for k, v := range base {
		result[k] = e.deepCopyValue(v)
	}

	// Merge overlay
	for k, v := range overlay {
		if existing, ok := result[k]; ok {
			// Recursively merge if both are maps
			if baseMap, ok1 := existing.(map[string]interface{}); ok1 {
				if overlayMap, ok2 := v.(map[string]interface{}); ok2 {
					result[k] = e.mergeMaps(baseMap, overlayMap)
					continue
				}
			}
		}
		result[k] = e.deepCopyValue(v)
	}

	return result
}

// deepCopyValue creates a deep copy of a value
func (e *extensionLoader) deepCopyValue(value interface{}) interface{} {
	switch v := value.(type) {
	case map[string]interface{}:
		result := make(map[string]interface{})
		for k, val := range v {
			result[k] = e.deepCopyValue(val)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, val := range v {
			result[i] = e.deepCopyValue(val)
		}
		return result
	default:
		return v
	}
}

// sortExtensions sorts extensions by priority (alphabetical)
func (e *extensionLoader) sortExtensions(extensions []LocalExtension) {
	// Sort by filename for consistent ordering
	for i := 0; i < len(extensions)-1; i++ {
		for j := i + 1; j < len(extensions); j++ {
			if filepath.Base(extensions[i].Path) > filepath.Base(extensions[j].Path) {
				extensions[i], extensions[j] = extensions[j], extensions[i]
			}
		}
	}
}

// Helper functions for nested field operations

func setNestedField(obj map[string]interface{}, value interface{}, path ...string) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}

	current := obj
	for i := 0; i < len(path)-1; i++ {
		if next, ok := current[path[i]].(map[string]interface{}); ok {
			current = next
		} else {
			// Create intermediate maps if needed
			next := make(map[string]interface{})
			current[path[i]] = next
			current = next
		}
	}

	current[path[len(path)-1]] = value
	return nil
}

func mergeNestedField(obj map[string]interface{}, value interface{}, path ...string) error {
	if len(path) == 0 {
		return errors.New("empty path")
	}

	current := obj
	for i := 0; i < len(path)-1; i++ {
		if next, ok := current[path[i]].(map[string]interface{}); ok {
			current = next
		} else {
			// Create intermediate maps if needed
			next := make(map[string]interface{})
			current[path[i]] = next
			current = next
		}
	}

	key := path[len(path)-1]
	if existing, ok := current[key]; ok {
		// Merge if both are maps
		if existingMap, ok1 := existing.(map[string]interface{}); ok1 {
			if valueMap, ok2 := value.(map[string]interface{}); ok2 {
				for k, v := range valueMap {
					existingMap[k] = v
				}
				return nil
			}
		}
	}

	// Otherwise set value
	current[key] = value
	return nil
}

func removeNestedField(obj map[string]interface{}, path ...string) {
	if len(path) == 0 {
		return
	}

	current := obj
	for i := 0; i < len(path)-1; i++ {
		if next, ok := current[path[i]].(map[string]interface{}); ok {
			current = next
		} else {
			return // Path doesn't exist
		}
	}

	delete(current, path[len(path)-1])
}
