package kurelpackage

import (
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/internal/gvk"
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

	// Generate kurel.yaml
	kurelYAML := c.generateKurelYAML()
	files["kurel.yaml"] = kurelYAML

	// Process resources
	// TODO: Implement resource gathering

	// Generate patches
	// TODO: Implement patch generation

	// Generate values files
	if c.Values != nil {
		// TODO: Implement values generation
	}

	// Process extensions
	for _, ext := range c.Extensions {
		// TODO: Implement extension processing
		_ = ext
	}

	return files, nil
}

func (c *ConfigV1Alpha1) generateKurelYAML() []byte {
	// TODO: Implement kurel.yaml generation
	// This would create the package manifest that kurel uses
	return []byte(`# Generated kurel.yaml
apiVersion: kurel.gokure.dev/v1alpha1
kind: Package
metadata:
  name: ` + c.Package.Name + `
  version: ` + c.Package.Version + `
spec:
  # TODO: Complete implementation
`)
}

// Validate checks if the configuration is valid
func (c *ConfigV1Alpha1) Validate() error {
	if c.Package.Name == "" {
		return fmt.Errorf("package name is required")
	}
	if c.Package.Version == "" {
		return fmt.Errorf("package version is required")
	}

	// TODO: Add more validation
	// - Validate version format
	// - Validate resource paths exist
	// - Validate patch syntax
	// - Validate CEL expressions in extensions
	// - Validate dependency versions

	return nil
}
