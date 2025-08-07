package launcher

import (
	"context"
	"io"
	
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// DefinitionLoader loads package definitions from disk
type DefinitionLoader interface {
	LoadDefinition(ctx context.Context, path string, opts *LauncherOptions) (*PackageDefinition, error)
}

// ResourceLoader loads Kubernetes resources
type ResourceLoader interface {
	LoadResources(ctx context.Context, path string, opts *LauncherOptions) ([]Resource, error)
}

// PatchLoader loads patch files
type PatchLoader interface {
	LoadPatches(ctx context.Context, path string, opts *LauncherOptions) ([]Patch, error)
}

// PackageLoader combines all loading capabilities
type PackageLoader interface {
	DefinitionLoader
	ResourceLoader
	PatchLoader
}

// Resolver resolves variable references in parameters
type Resolver interface {
	// Resolve substitutes variable references in parameters
	Resolve(ctx context.Context, base, overrides ParameterMap, opts *LauncherOptions) (ParameterMapWithSource, error)
	
	// DebugVariableGraph generates a dependency graph for debugging
	DebugVariableGraph(params ParameterMap) string
}

// PatchProcessor handles patch discovery, dependencies, and application
type PatchProcessor interface {
	// ResolveDependencies determines which patches to enable based on conditions and dependencies
	ResolveDependencies(ctx context.Context, patches []Patch, params ParameterMap) ([]Patch, error)
	
	// ApplyPatches applies patches to a package definition (returns deep copy)
	ApplyPatches(ctx context.Context, def *PackageDefinition, patches []Patch, params ParameterMap) (*PackageDefinition, error)
	
	// DebugPatchGraph generates a patch dependency graph for debugging
	DebugPatchGraph(patches []Patch) string
}

// SchemaGenerator generates JSON schemas for validation
type SchemaGenerator interface {
	// GeneratePackageSchema generates a schema for package validation
	GeneratePackageSchema(ctx context.Context) (*JSONSchema, error)
	
	// GenerateResourceSchema generates a schema for a specific resource type
	GenerateResourceSchema(ctx context.Context, gvk schema.GroupVersionKind) (*JSONSchema, error)
	
	// GenerateParameterSchema generates a schema for parameters
	GenerateParameterSchema(ctx context.Context, params ParameterMap) (*JSONSchema, error)
	
	// TraceFieldUsage traces how fields are used across resources
	TraceFieldUsage(resources []Resource) map[string][]string
	
	// ExportSchema exports a schema to JSON
	ExportSchema(schema *JSONSchema) ([]byte, error)
	
	// DebugSchema generates a debug representation of a schema
	DebugSchema(schema *JSONSchema) string
	
	// SetVerbose enables verbose mode
	SetVerbose(verbose bool)
}

// Validator validates package definitions
type Validator interface {
	// ValidatePackage validates an entire package definition
	ValidatePackage(ctx context.Context, def *PackageDefinition) (*ValidationResult, error)
	
	// ValidateResource validates a single resource
	ValidateResource(ctx context.Context, resource Resource) (*ValidationResult, error)
	
	// ValidatePatch validates a patch definition
	ValidatePatch(ctx context.Context, patch Patch) (*ValidationResult, error)
	
	// SetStrictMode enables or disables strict validation mode
	SetStrictMode(strict bool)
	
	// SetMaxErrors sets the maximum number of errors before stopping
	SetMaxErrors(max int)
	
	// SetVerbose enables verbose mode
	SetVerbose(verbose bool)
}

// Builder builds final manifests from package instances
type Builder interface {
	// Build generates final manifests and writes them according to options
	Build(ctx context.Context, inst *PackageInstance, buildOpts BuildOptions, opts *LauncherOptions) error
	
	// SetOutputWriter sets the output writer for stdout output
	SetOutputWriter(w io.Writer)
}

// ExtensionLoader handles .local.kurel extensions
type ExtensionLoader interface {
	// LoadWithExtensions loads a package with local extensions
	LoadWithExtensions(ctx context.Context, def *PackageDefinition, localPath string, opts *LauncherOptions) (*PackageDefinition, error)
}

// ProgressReporter reports progress for long operations
type ProgressReporter interface {
	Update(message string)
	Finish()
}

// FileWriter abstracts file system operations for testing
type FileWriter interface {
	WriteFile(path string, data []byte) error
	MkdirAll(path string) error
}

// OutputWriter abstracts output destinations
type OutputWriter interface {
	io.Writer
	Close() error
}