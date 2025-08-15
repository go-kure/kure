package kurelpackage

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/go-kure/kure/internal/gvk"
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/generators"
)

func init() {
	// Register the KurelPackage v1alpha1 generator
	gvkObj := gvk.GVK{
		Group:   "generators.gokure.dev",
		Version: "v1alpha1",
		Kind:    "KurelPackage",
	}

	factory := func() stack.ApplicationConfig {
		return &ConfigV1Alpha1{}
	}

	// Register with generators package for backward compatibility
	generators.Register(generators.GVK(gvkObj), factory)

	// Register with stack package for direct usage
	stack.RegisterApplicationConfig(gvkObj, factory)
}

// ConfigV1Alpha1 generates a kurel package structure
type ConfigV1Alpha1 struct {
	generators.BaseMetadata `yaml:",inline" json:",inline"`

	// Package metadata
	Package PackageMetadata `yaml:"package" json:"package"`

	// Resources to include in the package
	Resources []ResourceSource `yaml:"resources,omitempty" json:"resources,omitempty"`

	// Patches to apply to resources
	Patches []PatchDefinition `yaml:"patches,omitempty" json:"patches,omitempty"`

	// Values configuration
	Values *ValuesConfig `yaml:"values,omitempty" json:"values,omitempty"`

	// Extensions for conditional features
	Extensions []Extension `yaml:"extensions,omitempty" json:"extensions,omitempty"`

	// Package dependencies
	Dependencies []Dependency `yaml:"dependencies,omitempty" json:"dependencies,omitempty"`

	// Build configuration
	Build *BuildConfig `yaml:"build,omitempty" json:"build,omitempty"`
}

// PackageMetadata contains kurel package metadata
type PackageMetadata struct {
	Name        string            `yaml:"name" json:"name"`
	Version     string            `yaml:"version" json:"version"`
	Description string            `yaml:"description,omitempty" json:"description,omitempty"`
	Authors     []string          `yaml:"authors,omitempty" json:"authors,omitempty"`
	License     string            `yaml:"license,omitempty" json:"license,omitempty"`
	Homepage    string            `yaml:"homepage,omitempty" json:"homepage,omitempty"`
	Repository  string            `yaml:"repository,omitempty" json:"repository,omitempty"`
	Keywords    []string          `yaml:"keywords,omitempty" json:"keywords,omitempty"`
	Labels      map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ResourceSource defines where to find resources
type ResourceSource struct {
	Source   string   `yaml:"source" json:"source"`                         // Directory or file path
	Includes []string `yaml:"includes,omitempty" json:"includes,omitempty"` // Include patterns
	Excludes []string `yaml:"excludes,omitempty" json:"excludes,omitempty"` // Exclude patterns
	Recurse  bool     `yaml:"recurse,omitempty" json:"recurse,omitempty"`   // Recurse into subdirectories
}

// PatchDefinition defines a patch to apply
type PatchDefinition struct {
	Target PatchTarget `yaml:"target" json:"target"`
	Patch  string      `yaml:"patch" json:"patch"`                   // JSONPatch or strategic merge patch
	Type   string      `yaml:"type,omitempty" json:"type,omitempty"` // "json" or "strategic", default "json"
}

// PatchTarget identifies what to patch
type PatchTarget struct {
	APIVersion string            `yaml:"apiVersion,omitempty" json:"apiVersion,omitempty"`
	Kind       string            `yaml:"kind" json:"kind"`
	Name       string            `yaml:"name" json:"name"`
	Namespace  string            `yaml:"namespace,omitempty" json:"namespace,omitempty"`
	Labels     map[string]string `yaml:"labels,omitempty" json:"labels,omitempty"`
}

// ValuesConfig defines values schema and defaults
type ValuesConfig struct {
	Schema   string      `yaml:"schema,omitempty" json:"schema,omitempty"`     // Path to JSON schema
	Defaults string      `yaml:"defaults,omitempty" json:"defaults,omitempty"` // Path to default values
	Values   interface{} `yaml:"values,omitempty" json:"values,omitempty"`     // Inline default values
}

// Extension defines a conditional extension
type Extension struct {
	Name      string            `yaml:"name" json:"name"`
	When      string            `yaml:"when,omitempty" json:"when,omitempty"`           // CEL expression
	Resources []ResourceSource  `yaml:"resources,omitempty" json:"resources,omitempty"` // Additional resources
	Patches   []PatchDefinition `yaml:"patches,omitempty" json:"patches,omitempty"`     // Additional patches
}

// Dependency defines a package dependency
type Dependency struct {
	Name       string `yaml:"name" json:"name"`
	Version    string `yaml:"version" json:"version"`                           // Semantic version constraint
	Repository string `yaml:"repository,omitempty" json:"repository,omitempty"` // OCI repository
	Optional   bool   `yaml:"optional,omitempty" json:"optional,omitempty"`
}

// BuildConfig defines build-time configuration
type BuildConfig struct {
	OutputDir   string            `yaml:"outputDir,omitempty" json:"outputDir,omitempty"`     // Output directory for built package
	Format      string            `yaml:"format,omitempty" json:"format,omitempty"`           // "directory" or "oci"
	Registry    string            `yaml:"registry,omitempty" json:"registry,omitempty"`       // OCI registry for push
	Repository  string            `yaml:"repository,omitempty" json:"repository,omitempty"`   // OCI repository name
	Tags        []string          `yaml:"tags,omitempty" json:"tags,omitempty"`               // Additional tags
	Annotations map[string]string `yaml:"annotations,omitempty" json:"annotations,omitempty"` // OCI annotations
}

// GetAPIVersion returns the API version for this config
func (c *ConfigV1Alpha1) GetAPIVersion() string {
	return "generators.gokure.dev/v1alpha1"
}

// GetKind returns the kind for this config
func (c *ConfigV1Alpha1) GetKind() string {
	return "KurelPackage"
}

// Generate creates the kurel package structure
func (c *ConfigV1Alpha1) Generate(app *stack.Application) ([]*client.Object, error) {
	// For now, this is a placeholder implementation
	// The actual implementation would:
	// 1. Generate kurel.yaml with package metadata
	// 2. Copy/process resources according to ResourceSource definitions
	// 3. Generate patches in the appropriate format
	// 4. Create values schema and defaults
	// 5. Process extensions
	// 6. Validate dependencies
	// 7. Build the package according to BuildConfig

	// Since kurel packages aren't Kubernetes resources, we might need to
	// rethink this interface or create a separate generation path for
	// file-based outputs rather than client.Object outputs

	return nil, fmt.Errorf("KurelPackage generator not yet implemented")
}

// GeneratePackageFiles generates the kurel package file structure
// This is a more appropriate interface for this generator type
func (c *ConfigV1Alpha1) GeneratePackageFiles(app *stack.Application) (map[string][]byte, error) {
	files := make(map[string][]byte)

	// Process resources first
	resourceFiles, err := c.gatherResources()
	if err != nil {
		return nil, errors.Wrap(err, "failed to gather resources")
	}

	// Add resources to package under resources/ directory
	for path, content := range resourceFiles {
		files[filepath.Join("resources", path)] = content
	}

	// Generate patches
	patchFiles, err := c.generatePatches()
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate patches")
	}

	// Add patches to package under patches/ directory
	for path, content := range patchFiles {
		files[filepath.Join("patches", path)] = content
	}

	// Generate values files
	if c.Values != nil {
		valuesFiles, err := c.generateValues()
		if err != nil {
			return nil, errors.Wrap(err, "failed to generate values")
		}

		// Add values to package under values/ directory
		for path, content := range valuesFiles {
			files[filepath.Join("values", path)] = content
		}
	}

	// Process extensions
	for i, ext := range c.Extensions {
		extFiles, err := c.processExtension(ext, i)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to process extension %s", ext.Name)
		}

		// Add extension files under extensions/<name>/ directory
		for path, content := range extFiles {
			files[filepath.Join("extensions", ext.Name, path)] = content
		}
	}

	// Generate kurel.yaml (after processing all components)
	kurelYAML, err := c.generateKurelYAML(files)
	if err != nil {
		return nil, errors.Wrap(err, "failed to generate kurel.yaml")
	}
	files["kurel.yaml"] = kurelYAML

	return files, nil
}

// gatherResources collects resources from filesystem according to ResourceSource definitions
func (c *ConfigV1Alpha1) gatherResources() (map[string][]byte, error) {
	files := make(map[string][]byte)

	for _, resource := range c.Resources {
		resourceFiles, err := c.gatherResourcesFromSource(resource)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to gather resources from source %s", resource.Source)
		}

		// Merge collected files, handling potential conflicts
		for path, content := range resourceFiles {
			if _, exists := files[path]; exists {
				return nil, errors.Errorf("duplicate resource file: %s (conflicts between sources)", path)
			}
			files[path] = content
		}
	}

	return files, nil
}

// gatherResourcesFromSource collects resources from a single ResourceSource
func (c *ConfigV1Alpha1) gatherResourcesFromSource(resource ResourceSource) (map[string][]byte, error) {
	files := make(map[string][]byte)

	// Check if source exists
	info, err := os.Stat(resource.Source)
	if err != nil {
		return nil, errors.Wrapf(err, "resource source not found: %s", resource.Source)
	}

	if info.IsDir() {
		// Collect from directory
		err = c.walkDirectory(resource.Source, resource, files)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to walk directory %s", resource.Source)
		}
	} else {
		// Single file
		if c.shouldIncludeFile(resource.Source, resource) {
			content, err := os.ReadFile(resource.Source)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to read file %s", resource.Source)
			}

			// Validate it's a Kubernetes resource
			if c.isKubernetesResource(content) {
				// Use relative path from source directory
				relPath := filepath.Base(resource.Source)
				files[relPath] = content
			}
		}
	}

	return files, nil
}

// walkDirectory recursively walks a directory collecting matching files
func (c *ConfigV1Alpha1) walkDirectory(root string, resource ResourceSource, files map[string][]byte) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			// Check if we should recurse
			if !resource.Recurse && path != root {
				return filepath.SkipDir
			}
			return nil
		}

		// Check if file should be included
		if !c.shouldIncludeFile(path, resource) {
			return nil
		}

		// Read and validate file
		content, err := os.ReadFile(path)
		if err != nil {
			return errors.Wrapf(err, "failed to read file %s", path)
		}

		// Validate it's a Kubernetes resource
		if !c.isKubernetesResource(content) {
			return nil // Skip non-Kubernetes files
		}

		// Get relative path from root
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return errors.Wrapf(err, "failed to get relative path for %s", path)
		}

		files[relPath] = content
		return nil
	})
}

// shouldIncludeFile checks if a file matches include/exclude patterns
func (c *ConfigV1Alpha1) shouldIncludeFile(path string, resource ResourceSource) bool {
	fileName := filepath.Base(path)

	// Check excludes first
	for _, exclude := range resource.Excludes {
		if matched, _ := filepath.Match(exclude, fileName); matched {
			return false
		}
	}

	// Check includes (if any specified)
	if len(resource.Includes) == 0 {
		// No includes specified, include by default (unless excluded)
		return true
	}

	for _, include := range resource.Includes {
		if matched, _ := filepath.Match(include, fileName); matched {
			return true
		}
	}

	return false
}

// isKubernetesResource validates that content contains a Kubernetes resource
func (c *ConfigV1Alpha1) isKubernetesResource(content []byte) bool {
	var resource struct {
		APIVersion string `yaml:"apiVersion"`
		Kind       string `yaml:"kind"`
	}

	err := yaml.Unmarshal(content, &resource)
	if err != nil {
		return false
	}

	return resource.APIVersion != "" && resource.Kind != ""
}

// generatePatches creates patch files from PatchDefinitions
func (c *ConfigV1Alpha1) generatePatches() (map[string][]byte, error) {
	files := make(map[string][]byte)

	for i, patch := range c.Patches {
		patchContent, err := c.generatePatchFile(patch, i)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to generate patch %d", i)
		}

		patchFile := fmt.Sprintf("patch-%03d.yaml", i)
		files[patchFile] = patchContent
	}

	return files, nil
}

// generatePatchFile creates a single patch file from PatchDefinition
func (c *ConfigV1Alpha1) generatePatchFile(patch PatchDefinition, index int) ([]byte, error) {
	patchDoc := map[string]interface{}{
		"apiVersion": "kurel.gokure.dev/v1alpha1",
		"kind":       "Patch",
		"metadata": map[string]interface{}{
			"name": fmt.Sprintf("patch-%03d", index),
		},
		"spec": map[string]interface{}{
			"target": patch.Target,
			"patch":  patch.Patch,
			"type":   patch.Type,
		},
	}

	return yaml.Marshal(patchDoc)
}

// generateValues creates values files from ValuesConfig
func (c *ConfigV1Alpha1) generateValues() (map[string][]byte, error) {
	files := make(map[string][]byte)

	// Generate default values
	if c.Values.Defaults != "" {
		// Read defaults file
		defaultsContent, err := os.ReadFile(c.Values.Defaults)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read defaults file %s", c.Values.Defaults)
		}
		files["values.yaml"] = defaultsContent
	} else if c.Values.Values != nil {
		// Generate from inline values
		valuesContent, err := yaml.Marshal(c.Values.Values)
		if err != nil {
			return nil, errors.Wrap(err, "failed to marshal inline values")
		}
		files["values.yaml"] = valuesContent
	}

	// Generate schema
	if c.Values.Schema != "" {
		schemaContent, err := os.ReadFile(c.Values.Schema)
		if err != nil {
			return nil, errors.Wrapf(err, "failed to read schema file %s", c.Values.Schema)
		}
		files["values.schema.json"] = schemaContent
	}

	return files, nil
}

// processExtension processes a single extension
func (c *ConfigV1Alpha1) processExtension(ext Extension, index int) (map[string][]byte, error) {
	files := make(map[string][]byte)

	// Process extension resources
	if len(ext.Resources) > 0 {
		for i, resource := range ext.Resources {
			resourceFiles, err := c.gatherResourcesFromSource(resource)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to gather extension resources from source %s", resource.Source)
			}

			// Add with prefix to avoid conflicts
			for path, content := range resourceFiles {
				prefixedPath := fmt.Sprintf("resources-%d/%s", i, path)
				files[prefixedPath] = content
			}
		}
	}

	// Process extension patches
	if len(ext.Patches) > 0 {
		for i, patch := range ext.Patches {
			patchContent, err := c.generatePatchFile(patch, i)
			if err != nil {
				return nil, errors.Wrapf(err, "failed to generate extension patch %d", i)
			}

			patchFile := fmt.Sprintf("patches/patch-%03d.yaml", i)
			files[patchFile] = patchContent
		}
	}

	// Generate extension manifest
	extManifest := map[string]interface{}{
		"apiVersion": "kurel.gokure.dev/v1alpha1",
		"kind":       "Extension",
		"metadata": map[string]interface{}{
			"name": ext.Name,
		},
		"spec": map[string]interface{}{
			"when": ext.When,
		},
	}

	manifestContent, err := yaml.Marshal(extManifest)
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal extension manifest")
	}
	files["extension.yaml"] = manifestContent

	return files, nil
}

// generateKurelYAML creates the main package manifest
func (c *ConfigV1Alpha1) generateKurelYAML(packageFiles map[string][]byte) ([]byte, error) {
	// Build file inventory
	var resourceFiles, patchFiles, valueFiles, extensionDirs []string

	for path := range packageFiles {
		switch {
		case strings.HasPrefix(path, "resources/"):
			resourceFiles = append(resourceFiles, strings.TrimPrefix(path, "resources/"))
		case strings.HasPrefix(path, "patches/"):
			patchFiles = append(patchFiles, strings.TrimPrefix(path, "patches/"))
		case strings.HasPrefix(path, "values/"):
			valueFiles = append(valueFiles, strings.TrimPrefix(path, "values/"))
		case strings.HasPrefix(path, "extensions/"):
			parts := strings.Split(strings.TrimPrefix(path, "extensions/"), "/")
			if len(parts) > 0 {
				extDir := parts[0]
				found := false
				for _, existing := range extensionDirs {
					if existing == extDir {
						found = true
						break
					}
				}
				if !found {
					extensionDirs = append(extensionDirs, extDir)
				}
			}
		}
	}

	kurelDoc := map[string]interface{}{
		"apiVersion": "kurel.gokure.dev/v1alpha1",
		"kind":       "Package",
		"metadata": map[string]interface{}{
			"name":        c.Package.Name,
			"version":     c.Package.Version,
			"description": c.Package.Description,
			"authors":     c.Package.Authors,
			"license":     c.Package.License,
			"homepage":    c.Package.Homepage,
			"repository":  c.Package.Repository,
			"keywords":    c.Package.Keywords,
			"labels":      c.Package.Labels,
		},
		"spec": map[string]interface{}{
			"resources":  resourceFiles,
			"patches":    patchFiles,
			"values":     valueFiles,
			"extensions": extensionDirs,
		},
	}

	// Add dependencies if any
	if len(c.Dependencies) > 0 {
		deps := make([]map[string]interface{}, len(c.Dependencies))
		for i, dep := range c.Dependencies {
			deps[i] = map[string]interface{}{
				"name":       dep.Name,
				"version":    dep.Version,
				"repository": dep.Repository,
				"optional":   dep.Optional,
			}
		}
		kurelDoc["spec"].(map[string]interface{})["dependencies"] = deps
	}

	// Add build config if any
	if c.Build != nil {
		kurelDoc["spec"].(map[string]interface{})["build"] = map[string]interface{}{
			"outputDir":   c.Build.OutputDir,
			"format":      c.Build.Format,
			"registry":    c.Build.Registry,
			"repository":  c.Build.Repository,
			"tags":        c.Build.Tags,
			"annotations": c.Build.Annotations,
		}
	}

	return yaml.Marshal(kurelDoc)
}

// Validate checks if the configuration is valid
func (c *ConfigV1Alpha1) Validate() error {
	// Validate required fields
	if c.Package.Name == "" {
		return errors.New("package name is required")
	}
	if c.Package.Version == "" {
		return errors.New("package version is required")
	}

	// Validate package name format (kubernetes resource name rules)
	if err := c.validatePackageName(c.Package.Name); err != nil {
		return errors.Wrap(err, "invalid package name")
	}

	// Validate version format (semantic versioning)
	if err := c.validateVersionFormat(c.Package.Version); err != nil {
		return errors.Wrap(err, "invalid package version")
	}

	// Validate resource sources exist
	for i, resource := range c.Resources {
		if err := c.validateResourceSource(resource); err != nil {
			return errors.Wrapf(err, "invalid resource source %d", i)
		}
	}

	// Validate patch syntax
	for i, patch := range c.Patches {
		if err := c.validatePatchDefinition(patch); err != nil {
			return errors.Wrapf(err, "invalid patch definition %d", i)
		}
	}

	// Validate values configuration
	if c.Values != nil {
		if err := c.validateValuesConfig(*c.Values); err != nil {
			return errors.Wrap(err, "invalid values configuration")
		}
	}

	// Validate extensions
	for i, ext := range c.Extensions {
		if err := c.validateExtension(ext); err != nil {
			return errors.Wrapf(err, "invalid extension %d (%s)", i, ext.Name)
		}
	}

	// Validate dependencies
	for i, dep := range c.Dependencies {
		if err := c.validateDependency(dep); err != nil {
			return errors.Wrapf(err, "invalid dependency %d (%s)", i, dep.Name)
		}
	}

	// Validate build configuration
	if c.Build != nil {
		if err := c.validateBuildConfig(*c.Build); err != nil {
			return errors.Wrap(err, "invalid build configuration")
		}
	}

	return nil
}

// validatePackageName checks if package name follows Kubernetes naming rules
func (c *ConfigV1Alpha1) validatePackageName(name string) error {
	// Kubernetes resource names must be DNS compatible
	if len(name) == 0 || len(name) > 253 {
		return errors.New("name must be 1-253 characters")
	}

	// Must start and end with alphanumeric
	nameRegex := regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	if !nameRegex.MatchString(name) {
		return errors.New("name must start and end with alphanumeric characters and contain only lowercase letters, numbers, and hyphens")
	}

	return nil
}

// validateVersionFormat checks if version follows semantic versioning
func (c *ConfigV1Alpha1) validateVersionFormat(version string) error {
	// Basic semantic versioning pattern
	semverRegex := regexp.MustCompile(`^(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)(-[a-zA-Z0-9]+([-.]?[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+([-.]?[a-zA-Z0-9]+)*)?$`)
	if !semverRegex.MatchString(version) {
		return errors.New("version must follow semantic versioning format (e.g., 1.0.0, 1.0.0-alpha.1)")
	}

	return nil
}

// validateResourceSource checks if resource source is valid
func (c *ConfigV1Alpha1) validateResourceSource(resource ResourceSource) error {
	if resource.Source == "" {
		return errors.New("resource source path is required")
	}

	// Check if source path exists
	if _, err := os.Stat(resource.Source); err != nil {
		return errors.Wrapf(err, "resource source path does not exist: %s", resource.Source)
	}

	// Validate include/exclude patterns
	for _, pattern := range resource.Includes {
		if _, err := filepath.Match(pattern, "test"); err != nil {
			return errors.Wrapf(err, "invalid include pattern: %s", pattern)
		}
	}

	for _, pattern := range resource.Excludes {
		if _, err := filepath.Match(pattern, "test"); err != nil {
			return errors.Wrapf(err, "invalid exclude pattern: %s", pattern)
		}
	}

	return nil
}

// validatePatchDefinition checks if patch definition is valid
func (c *ConfigV1Alpha1) validatePatchDefinition(patch PatchDefinition) error {
	// Validate target
	if patch.Target.Kind == "" {
		return errors.New("patch target kind is required")
	}
	if patch.Target.Name == "" {
		return errors.New("patch target name is required")
	}

	// Validate patch content
	if patch.Patch == "" {
		return errors.New("patch content is required")
	}

	// Validate patch type
	patchType := patch.Type
	if patchType == "" {
		patchType = "json" // default
	}

	switch patchType {
	case "json":
		// Validate JSON patch syntax
		if err := c.validateJSONPatch(patch.Patch); err != nil {
			return errors.Wrap(err, "invalid JSON patch")
		}
	case "strategic":
		// Validate strategic merge patch syntax (basic YAML validation)
		if err := c.validateStrategicMergePatch(patch.Patch); err != nil {
			return errors.Wrap(err, "invalid strategic merge patch")
		}
	default:
		return errors.New("patch type must be 'json' or 'strategic'")
	}

	return nil
}

// validateJSONPatch validates JSON patch syntax
func (c *ConfigV1Alpha1) validateJSONPatch(patchContent string) error {
	// Try to parse as YAML first (since patches are often written in YAML)
	var jsonPatch []map[string]interface{}
	if err := yaml.Unmarshal([]byte(patchContent), &jsonPatch); err != nil {
		return errors.Wrap(err, "patch content is not valid YAML/JSON")
	}

	// Validate each operation
	for i, op := range jsonPatch {
		operation, ok := op["op"].(string)
		if !ok {
			return errors.Errorf("operation %d missing 'op' field", i)
		}

		path, ok := op["path"].(string)
		if !ok {
			return errors.Errorf("operation %d missing 'path' field", i)
		}

		// Validate operation type
		switch operation {
		case "add", "replace", "test":
			if _, ok := op["value"]; !ok {
				return errors.Errorf("operation %d (%s) missing 'value' field", i, operation)
			}
		case "remove":
			// Remove doesn't need value
		case "move", "copy":
			if _, ok := op["from"].(string); !ok {
				return errors.Errorf("operation %d (%s) missing 'from' field", i, operation)
			}
		default:
			return errors.Errorf("operation %d has invalid op '%s'", i, operation)
		}

		// Basic path validation (must start with /)
		if !strings.HasPrefix(path, "/") {
			return errors.Errorf("operation %d path must start with '/'", i)
		}
	}

	return nil
}

// validateStrategicMergePatch validates strategic merge patch syntax
func (c *ConfigV1Alpha1) validateStrategicMergePatch(patchContent string) error {
	// Strategic merge patches are just YAML documents
	var patch map[string]interface{}
	if err := yaml.Unmarshal([]byte(patchContent), &patch); err != nil {
		return errors.Wrap(err, "patch content is not valid YAML")
	}

	// Basic validation - should be a non-empty map
	if len(patch) == 0 {
		return errors.New("strategic merge patch cannot be empty")
	}

	return nil
}

// validateValuesConfig checks if values configuration is valid
func (c *ConfigV1Alpha1) validateValuesConfig(values ValuesConfig) error {
	// At least one of defaults file, schema file, or inline values should be provided
	if values.Defaults == "" && values.Schema == "" && values.Values == nil {
		return errors.New("values configuration must specify at least one of: defaults file, schema file, or inline values")
	}

	// Check if defaults file exists
	if values.Defaults != "" {
		if _, err := os.Stat(values.Defaults); err != nil {
			return errors.Wrapf(err, "defaults file does not exist: %s", values.Defaults)
		}
	}

	// Check if schema file exists
	if values.Schema != "" {
		if _, err := os.Stat(values.Schema); err != nil {
			return errors.Wrapf(err, "schema file does not exist: %s", values.Schema)
		}
	}

	return nil
}

// validateExtension checks if extension configuration is valid
func (c *ConfigV1Alpha1) validateExtension(ext Extension) error {
	if ext.Name == "" {
		return errors.New("extension name is required")
	}

	// Validate extension name format
	if err := c.validatePackageName(ext.Name); err != nil {
		return errors.Wrap(err, "invalid extension name")
	}

	// Validate CEL expression (basic syntax check)
	if ext.When != "" {
		if err := c.validateCELExpression(ext.When); err != nil {
			return errors.Wrap(err, "invalid CEL expression in when clause")
		}
	}

	// Validate extension resources
	for i, resource := range ext.Resources {
		if err := c.validateResourceSource(resource); err != nil {
			return errors.Wrapf(err, "invalid extension resource %d", i)
		}
	}

	// Validate extension patches
	for i, patch := range ext.Patches {
		if err := c.validatePatchDefinition(patch); err != nil {
			return errors.Wrapf(err, "invalid extension patch %d", i)
		}
	}

	return nil
}

// validateCELExpression performs basic CEL expression validation
func (c *ConfigV1Alpha1) validateCELExpression(expr string) error {
	// Basic validation - should not be empty and should contain valid identifiers
	if strings.TrimSpace(expr) == "" {
		return errors.New("CEL expression cannot be empty")
	}

	// Check for common CEL patterns (.Values, operators, etc.)
	if !strings.Contains(expr, ".Values") {
		return errors.New("CEL expression should reference .Values")
	}

	// TODO: Add proper CEL validation using cel-go library
	// For now, just do basic syntax checks
	invalidChars := regexp.MustCompile(`[^a-zA-Z0-9_.()[\]<>=!&|+\-*/%\s"']`)
	if invalidChars.MatchString(expr) {
		return errors.New("CEL expression contains invalid characters")
	}

	return nil
}

// validateDependency checks if dependency specification is valid
func (c *ConfigV1Alpha1) validateDependency(dep Dependency) error {
	if dep.Name == "" {
		return errors.New("dependency name is required")
	}

	if dep.Version == "" {
		return errors.New("dependency version is required")
	}

	// Validate dependency name format
	if err := c.validatePackageName(dep.Name); err != nil {
		return errors.Wrap(err, "invalid dependency name")
	}

	// Validate version constraint format (basic semantic versioning or constraint)
	if err := c.validateVersionConstraint(dep.Version); err != nil {
		return errors.Wrap(err, "invalid dependency version constraint")
	}

	return nil
}

// validateVersionConstraint validates semantic version constraints
func (c *ConfigV1Alpha1) validateVersionConstraint(constraint string) error {
	// Allow version constraints like ">=1.0.0", "~1.2.0", "^1.0.0", etc.
	constraintRegex := regexp.MustCompile(`^([\^~>=<]+)?(\d+\.\d+\.\d+)(-[a-zA-Z0-9]+([-.]?[a-zA-Z0-9]+)*)?(\+[a-zA-Z0-9]+([-.]?[a-zA-Z0-9]+)*)?$`)
	if !constraintRegex.MatchString(constraint) {
		return errors.New("version constraint must be a valid semantic version or constraint (e.g., '1.0.0', '>=1.0.0', '^1.2.0')")
	}

	return nil
}

// validateBuildConfig checks if build configuration is valid
func (c *ConfigV1Alpha1) validateBuildConfig(build BuildConfig) error {
	// Validate format
	if build.Format != "" && build.Format != "directory" && build.Format != "oci" {
		return errors.New("build format must be 'directory' or 'oci'")
	}

	// If OCI format, validate registry/repository
	if build.Format == "oci" {
		if build.Registry == "" {
			return errors.New("registry is required for OCI format")
		}
		if build.Repository == "" {
			return errors.New("repository is required for OCI format")
		}
	}

	// Validate output directory if specified
	if build.OutputDir != "" {
		// Check if parent directory exists
		parentDir := filepath.Dir(build.OutputDir)
		if parentDir != "." {
			if _, err := os.Stat(parentDir); err != nil {
				return errors.Wrapf(err, "output directory parent does not exist: %s", parentDir)
			}
		}
	}

	return nil
}
