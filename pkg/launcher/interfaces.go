package launcher

import (
	"context"
	"io"
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

// SchemaGenerator generates and validates schemas
type SchemaGenerator interface {
	// GenerateSchema creates a JSON schema from package definition
	GenerateSchema(ctx context.Context, def *PackageDefinition) (*Schema, error)
	
	// ValidateSchemaMerge checks for type conflicts across patches
	ValidateSchemaMerge(patches []Patch) error
}

// Validator validates package definitions and instances
type Validator interface {
	// ValidateDefinition validates a package definition structure
	ValidateDefinition(ctx context.Context, def *PackageDefinition) ValidationResult
	
	// ValidateInstance validates a package instance with resolved parameters
	ValidateInstance(ctx context.Context, inst *PackageInstance) ValidationResult
	
	// ValidateParameters validates parameters against a schema
	ValidateParameters(ctx context.Context, params ParameterMap, schema *Schema) ValidationResult
}

// Builder builds final manifests from package instances
type Builder interface {
	// Build generates final manifests and writes them according to options
	Build(ctx context.Context, inst *PackageInstance, buildOpts BuildOptions, opts *LauncherOptions) error
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