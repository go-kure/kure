// Package launcher provides a declarative Kubernetes manifest generation system
// that supports variable substitution, patch application, and schema validation.
//
// The launcher module is designed to load, validate, and process Kurel packages
// which are declarative definitions of Kubernetes resources with support for
// parameterization and composition.
//
// # Architecture
//
// The launcher follows a pipeline architecture with clear separation of concerns:
//
//  1. Loading: Reads package definitions from disk
//  2. Resolution: Resolves variables and parameter references
//  3. Patching: Applies patches with dependency resolution
//  4. Validation: Validates against JSON schemas
//  5. Building: Outputs final manifests
//
// # Core Components
//
//   - DefinitionLoader: Loads package definitions from YAML files
//   - VariableResolver: Resolves variable substitutions with cycle detection
//   - PatchProcessor: Applies patches with dependency and conflict resolution
//   - SchemaGenerator: Generates JSON schemas for validation
//   - Validator: Validates packages with error/warning distinction
//   - OutputBuilder: Generates final YAML/JSON output
//
// # Usage Example
//
//	// Create a loader
//	loader := launcher.NewLoader(logger)
//
//	// Load a package
//	def, err := loader.LoadPackage(ctx, "/path/to/package")
//	if err != nil {
//	    return errors.Wrap(err, "failed to load package")
//	}
//
//	// Create resolver and resolve variables
//	resolver := launcher.NewResolver(logger)
//	instance := &launcher.PackageInstance{
//	    Definition: def,
//	    UserValues: userParams,
//	}
//	resolved, err := resolver.ResolveVariables(ctx, instance)
//	if err != nil {
//	    return errors.Wrap(err, "failed to resolve variables")
//	}
//
//	// Validate the package
//	validator := launcher.NewValidator(logger)
//	result, err := validator.ValidatePackage(ctx, def)
//	if err != nil {
//	    return errors.Wrap(err, "validation failed")
//	}
//	if !result.IsValid() {
//	    return errors.Errorf("package has %d errors", len(result.Errors))
//	}
//
//	// Apply patches
//	processor := launcher.NewPatchProcessor(logger)
//	patched, err := processor.ApplyPatches(ctx, resolved.Resources, resolved.Patches)
//	if err != nil {
//	    return errors.Wrap(err, "failed to apply patches")
//	}
//
// # Error Handling
//
// The launcher uses a hybrid error handling approach:
//
//   - Errors: Block processing and must be fixed
//   - Warnings: Non-blocking issues that should be addressed
//
// All errors are created using the github.com/go-kure/kure/pkg/errors package
// which provides stack traces and error wrapping.
//
// # Thread Safety
//
// All launcher components are thread-safe and can be used concurrently.
// Resource and PackageDefinition types use read-write mutexes for protection.
//
// # Performance
//
// The launcher is optimized for performance with:
//
//   - Schema caching to avoid regeneration
//   - Variable resolution memoization
//   - Efficient cycle detection algorithms
//   - Parallel processing where applicable
//
// Target performance: < 1 second for full package build.
package launcher
