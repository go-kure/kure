# Launcher Module - Code Design

## Overview

The launcher module is the core engine for the Kurel package system, implementing a declarative approach to generating Kubernetes manifests with validation and customization capabilities. This document captures all design decisions made during the architecture planning phase.

## Design Philosophy

**Core Principle**: "Kurel just generates YAML" - The launcher is a declarative system for generating Kubernetes manifests, not a runtime system or orchestrator.

## Architecture Decisions

### 1. Core Package Structure

**Decision**: Separate `PackageDefinition` and `PackageInstance` pattern

```go
// Immutable package definition
type PackageDefinition struct {
    Path        string
    Metadata    KurelMetadata     // From kurel: key in parameters.yaml
    Parameters  ParameterMap      // Default parameters including global:
    Resources   []Resource        // Base K8s manifests
    Patches     []Patch          // Available patches with metadata
}

// Instance with user customization
type PackageInstance struct {
    Definition  *PackageDefinition  // Immutable package reference
    UserValues  ParameterMap        // User-provided overrides
    Resolved    ParameterMap        // Final resolved values
    LocalPath   string              // Path to .local.kurel if exists
}
```

**Rationale**: 
- Clear separation between immutable package and user state
- Enables processing same package with different configs in parallel
- Package definitions are cacheable and reusable
- More functional style with immutable data + transformations

### 2. Interface Organization

**Decision**: Small, focused interfaces following Go idioms

```go
// pkg/launcher/interfaces.go - Small, composable interfaces
type DefinitionLoader interface {
    LoadDefinition(ctx context.Context, path string) (*PackageDefinition, error)
}

type ResourceLoader interface {
    LoadResources(ctx context.Context, path string) ([]Resource, error)
}

type PatchLoader interface {
    LoadPatches(ctx context.Context, path string) ([]Patch, error)
}

// Compose when needed
type PackageLoader interface {
    DefinitionLoader
    ResourceLoader
    PatchLoader
}

type Resolver interface {
    Resolve(ctx context.Context, base, overrides ParameterMap) (ParameterMap, error)
}

type Builder interface {
    Build(ctx context.Context, inst *PackageInstance, opts BuildOptions) error
}
```

**Rationale**:
- Follows Go's preference for small interfaces (like `io.Reader`, `io.Writer`)
- Enables better testing through focused mocks
- Clean separation of contracts from data types
- Supports interface composition

### 3. Package Loading Strategy

**Decision**: Hybrid error handling with context support and size limits

- **Critical files** (parameters.yaml): Must load successfully or fail immediately
- **Other files** (resources, patches): Collect all errors, load what's possible
- **Size limits**: Enforce maximum package size (50MB) and resource count (1000)
- **Context cancellation**: Support timeout and cancellation

```go
const (
    MaxPackageSize   = 50 * 1024 * 1024  // 50MB hard limit
    WarnPackageSize  = 10 * 1024 * 1024  // 10MB warning
    MaxResourceCount = 1000               // Max resources
)

func (l *defaultLoader) LoadDefinition(ctx context.Context, path string) (*PackageDefinition, error) {
    // Check package size first
    if err := l.validatePackageSize(path); err != nil {
        return nil, err
    }
    
    // Critical: parameters.yaml MUST load
    params, err := l.loadParameters(ctx, filepath.Join(path, "parameters.yaml"))
    if err != nil {
        return nil, fmt.Errorf("critical: parameters.yaml: %w", err)
    }
    
    // Best effort for others with context
    var errs []error
    resources, resourceErrs := l.loadAllResources(ctx, path)
    patches, patchErrs := l.loadAllPatches(ctx, path)
    
    errs = append(errs, resourceErrs...)
    errs = append(errs, patchErrs...)
    
    def := &PackageDefinition{
        Path:       path,
        Metadata:   extractMetadata(params),
        Parameters: params,
        Resources:  resources,
        Patches:    patches,
    }
    
    if len(errs) > 0 {
        return def, &LoadErrors{
            PartialDefinition: def,
            Issues:           errs,
        }
    }
    
    return def, nil
}
```

**Rationale**:
- Can't proceed without valid parameters.yaml
- Size limits prevent memory issues (based on typical Helm chart sizes)
- Context support enables cancellation
- See all syntax errors at once for debugging

### 4. Variable Resolution

**Decision**: No inline defaults, configurable depth

- Variables must exist in parameters.yaml (no `${var|default}` syntax)
- Configurable maximum nesting depth to prevent infinite recursion
- Parameters.yaml is where all defaults are defined

```go
type variableResolver struct {
    maxDepth int  // Configurable, default 10
}

// Resolution without inline defaults
// ${monitoring.namespace} - ERROR if not defined
// No fallback syntax supported
```

**Rationale**:
- Keeps variable syntax simple
- All defaults in one place (parameters.yaml)
- Prevents infinite recursion while allowing deep nesting

### 5. Patch Processing

**Decision**: Strict validation with verbose logging

- **Conflicts**: Hard error, refuse to continue
- **Auto-enable**: Verbose logging to stderr
- **Missing targets**: Error, patches must match something
- **Application order**: Package patches first (by numeric prefix), then local patches (can override)
- **Failure handling**: Patches MUST apply successfully or error - no silent failures

```go
// Hard error on conflicts
if hasConflicts(enabledPatches) {
    return nil, fmt.Errorf("conflict: %s and %s cannot both be enabled", p1, p2)
}

// Verbose logging during build (--verbose flag)
INFO: Enabling patch 10-monitoring.kpatch (monitoring.enabled=true)
INFO: Auto-enabling 05-metrics.kpatch (required by 10-monitoring)
DEBUG: Applying patch 10-monitoring.kpatch to deployment/prometheus
DEBUG: Successfully patched field spec.template.spec.containers[0].resources

// Error if patch doesn't match
if matchCount == 0 {
    return fmt.Errorf("patch %s targets non-existent resource: deployment.frontend", patchName)
}

// Detailed error on patch failure
if err := applyPatch(resource, patch); err != nil {
    return fmt.Errorf("failed to apply patch %s to %s/%s: %w", 
        patch.Name, resource.Kind, resource.Name, err)
}
```

**Rationale**:
- Ensures patches work as expected
- Clear visibility into auto-enabled dependencies
- No silent failures
- Verbose mode provides debugging information
- Local patches can customize package behavior

### 6. Schema Generation

**Decision**: Hybrid approach with package-defined CRD URLs

- Bundle schemas for resources known in internal/ packages
- Package maintainers can specify CRD schema URLs in parameters.yaml
- Auto-generate if missing, allow explicit regeneration

```yaml
# In parameters.yaml
kurel:
  name: my-app
  schemas:
    - https://raw.githubusercontent.com/cert-manager/cert-manager/v1.13.0/deploy/crds/crd-certificates.yaml
    - https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.68.0/deploy/crds/crd-prometheuses.yaml
```

**Rationale**:
- Leverages existing Kure knowledge
- Extensible for custom CRDs
- Balance between convenience and flexibility

### 7. Validation System

**Decision**: Errors block, warnings don't; concurrent validation for performance

```go
type ValidationResult struct {
    Errors   []ValidationError
    Warnings []ValidationWarning
}

type ConcurrentValidator struct {
    maxWorkers int
    schemaGen  SchemaGenerator
}

func (v *ConcurrentValidator) ValidateInstance(ctx context.Context, inst *PackageInstance) ValidationResult {
    // Validate resources concurrently for performance
    numWorkers := runtime.NumCPU()
    if len(inst.Definition.Resources) < numWorkers {
        numWorkers = len(inst.Definition.Resources)
    }
    
    work := make(chan Resource, len(inst.Definition.Resources))
    results := make(chan ValidationError, len(inst.Definition.Resources))
    
    // Start workers
    var wg sync.WaitGroup
    for i := 0; i < numWorkers; i++ {
        wg.Add(1)
        go func() {
            defer wg.Done()
            for resource := range work {
                if err := v.validateResource(ctx, resource); err != nil {
                    results <- ValidationError{Resource: resource.GetName(), Error: err}
                }
            }
        }()
    }
    
    // Queue work
    for _, r := range inst.Definition.Resources {
        work <- r
    }
    close(work)
    
    // Collect results
    go func() {
        wg.Wait()
        close(results)
    }()
    
    var result ValidationResult
    for err := range results {
        result.Errors = append(result.Errors, err)
    }
    
    return result
}
```

**Rationale**:
- Concurrent validation for Helm-like performance
- Clear distinction between blocking and non-blocking issues
- Best-effort validation based on available information
- Worker pool pattern prevents resource exhaustion

### 8. Output Generation

**Decision**: No GitOps-specific support, configurable output format

- Kurel only manages `kurel.gokure.dev/` annotations for phase organization
- Actual deployment handled by GitOps tools (Flux/ArgoCD)
- Configurable output: stdout (default/dry-run), single file, by-kind, by-resource
- Multi-document YAML files are properly handled

```bash
# Default: multi-doc YAML to stdout (dry-run mode)
kurel build my-app.kurel/

# Output to directory with by-kind grouping
kurel build my-app.kurel/ -o out/ --format=by-kind

# Single file output
kurel build my-app.kurel/ -o manifests.yaml --format=single

# JSON output
kurel build my-app.kurel/ --output-format=json

# Verbose mode for debugging
kurel build my-app.kurel/ --verbose
```

**Rationale**:
- Keeps kurel focused on YAML generation only
- Default stdout output serves as dry-run mode
- Flexible output for different workflows
- Clean separation of concerns
- Verbose mode aids in debugging patch application

### 9. Local Extensions

**Decision**: Full integration with validation

- Local patches CAN reference package patches in dependencies
- Parameter conflicts are validated for compatibility
- Local extensions can only add, not replace

```yaml
# my-app.local.kurel/patches/50-custom.yaml
requires:
  - "features/10-monitoring.kpatch"  # Can reference package patches
conflicts:
  - "features/20-basic-monitoring.kpatch"  # Can conflict with package

# Parameter override validation
# Error if local changes parameter type/structure incompatibly
```

**Rationale**:
- Allows sophisticated customization
- Prevents breaking changes
- Maintains package integrity

### 10. CLI Integration

**Decision**: YAML to stdout, logs to stderr, with timeout and progress indication

```go
const (
    DefaultBuildTimeout = 30 * time.Second  // Similar to Helm
    MaxBuildTimeout     = 5 * time.Minute
)

type buildOptions struct {
    valuesFile   string
    outputPath   string
    outputFormat string
    outputType   string
    localPath    string
    timeout      time.Duration
    verbose      bool
    dryRun       bool
    quiet        bool
    showProgress bool
}

// Build command with proper flag handling
func newBuildCommand() *cobra.Command {
    var opts buildOptions
    
    cmd := &cobra.Command{
        Use:   "build <package>",
        Short: "Build Kubernetes manifests from kurel package",
        RunE: func(cmd *cobra.Command, args []string) error {
            // Apply timeout
            ctx, cancel := context.WithTimeout(context.Background(), opts.timeout)
            defer cancel()
            
            // Progress indication for long operations
            var progress ProgressReporter
            if opts.showProgress && !opts.quiet {
                progress = NewProgressBar("Building package...")
                defer progress.Finish()
            }
            
            return runBuild(ctx, args[0], opts, progress)
        },
    }
    
    flags := cmd.Flags()
    flags.StringVarP(&opts.valuesFile, "values", "f", "", "Values file")
    flags.StringVarP(&opts.outputPath, "output", "o", "", "Output path (default: stdout)")
    flags.DurationVar(&opts.timeout, "timeout", DefaultBuildTimeout, "Build timeout")
    flags.BoolVar(&opts.dryRun, "dry-run", false, "Print to stdout without writing")
    flags.BoolVarP(&opts.verbose, "verbose", "v", false, "Verbose output")
    flags.BoolVar(&opts.showProgress, "progress", true, "Show progress bar")
    
    return cmd
}
```

**Rationale**:
- Timeout prevents hanging builds
- Progress indication for better UX
- Unix philosophy: stdout for data, stderr for logs
- Matches Helm's performance characteristics

## Module Organization

```
pkg/launcher/
├── interfaces.go       # All public interfaces
├── types.go           # Core data types
├── loader.go          # Package loading implementation
├── variables.go       # Variable resolution engine
├── patches.go         # Patch discovery and processing
├── schema.go          # Schema generation
├── validator.go       # Validation logic
├── builder.go         # Manifest building and output
├── extensions.go      # Local extension handling
├── errors.go          # Custom error types
└── testdata/         # Test fixtures
    └── packages/     # Sample packages for testing
```

## Error Handling Philosophy

1. **Fail fast** for critical errors (missing parameters.yaml)
2. **Abort all** on patch failures (transactional behavior)
3. **Use existing error patterns** from pkg/errors
4. **Clear error messages** with context and suggestions
5. **Distinguish** between errors (blocking) and warnings (advisory)
6. **Context cancellation** respected throughout

```go
// Leverage existing Kure error patterns
type LoadErrors struct {
    errors.BaseError
    PartialDefinition *PackageDefinition
    Issues            []error
}

func (e *LoadErrors) Unwrap() []error {
    return e.Issues
}

// Transactional patch application
func (p *patchProcessor) ApplyPatches(ctx context.Context, resources []Resource, patches []Patch) ([]Resource, error) {
    // Work on copy for rollback capability
    working := make([]Resource, len(resources))
    copy(working, resources)
    
    for i, patch := range patches {
        select {
        case <-ctx.Done():
            return nil, fmt.Errorf("cancelled at patch %d/%d: %w", i+1, len(patches), ctx.Err())
        default:
        }
        
        result, err := p.applyPatch(working, patch)
        if err != nil {
            // Fail fast - abort all
            return nil, fmt.Errorf("patch %s failed (aborting all): %w", patch.Name, err)
        }
        working = result
    }
    
    return working, nil
}
```

## Testing Strategy

- **Table-driven tests**: Following Go best practices with subtests
- **Benchmarks**: Ensure performance matches Helm
- **Mock filesystem**: Using afero for loader testing
- **Integration tests**: Full package processing flows
- **Fixture packages**: Real-world package examples in testdata/
- **Concurrent testing**: Validate thread safety

```go
// Table-driven test example
func TestVariableResolver(t *testing.T) {
    tests := []struct {
        name      string
        base      ParameterMap
        overrides ParameterMap
        want      ParameterMap
        wantErr   string
    }{
        {
            name: "simple_substitution",
            base: ParameterMap{"app": "myapp", "image": "${app}:latest"},
            want: ParameterMap{"app": "myapp", "image": "myapp:latest"},
        },
        {
            name:    "circular_dependency",
            base:    ParameterMap{"a": "${b}", "b": "${a}"},
            wantErr: "circular dependency",
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            resolver := NewResolver()
            ctx := context.Background()
            got, err := resolver.Resolve(ctx, tt.base, tt.overrides)
            
            if tt.wantErr != "" {
                require.Error(t, err)
                assert.Contains(t, err.Error(), tt.wantErr)
                return
            }
            
            require.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}

// Benchmark example
func BenchmarkBuildPackage(b *testing.B) {
    packages := []struct {
        name      string
        resources int
        target    time.Duration
    }{
        {"small", 10, 100 * time.Millisecond},
        {"medium", 50, 500 * time.Millisecond},
        {"large", 200, 2 * time.Second},
    }
    
    for _, pkg := range packages {
        b.Run(pkg.name, func(b *testing.B) {
            p := generateTestPackage(pkg.resources)
            b.ResetTimer()
            
            for i := 0; i < b.N; i++ {
                ctx := context.Background()
                _, err := Build(ctx, p, BuildOptions{})
                require.NoError(b, err)
            }
        })
    }
}
```

## Performance Considerations

1. **Concurrent processing**: Use worker pools for CPU-intensive operations
2. **Memory limits**: Enforce package size limits (50MB max, 10MB warning)
3. **Streaming**: Stream large resource sets to avoid memory spikes
4. **Caching**: Cache schemas with TTL for repeated operations
5. **Context cancellation**: Support timeouts and early termination
6. **Performance targets** (matching Helm):
   - Small packages (1-20 resources): < 100ms
   - Medium packages (21-100 resources): < 500ms
   - Large packages (101-500 resources): < 2s
   - X-Large packages (500+ resources): < 5s

```go
// Concurrent validation with worker pool
func (v *Validator) ValidateConcurrent(ctx context.Context, resources []Resource) []error {
    workers := runtime.NumCPU()
    if len(resources) < workers {
        workers = len(resources)
    }
    
    sem := make(chan struct{}, workers)
    errChan := make(chan error, len(resources))
    
    var wg sync.WaitGroup
    for _, r := range resources {
        wg.Add(1)
        go func(resource Resource) {
            defer wg.Done()
            select {
            case sem <- struct{}{}:
                defer func() { <-sem }()
                if err := v.validate(resource); err != nil {
                    errChan <- err
                }
            case <-ctx.Done():
                errChan <- ctx.Err()
            }
        }(r)
    }
    
    go func() {
        wg.Wait()
        close(errChan)
    }()
    
    var errors []error
    for err := range errChan {
        errors = append(errors, err)
    }
    return errors
}
```

## Security Considerations

1. **Path traversal protection** in package loading
2. **URL validation** for schema URLs
3. **Variable injection prevention** in resolution
4. **Resource validation** against schemas
5. **No direct secret creation** - only references via external-secrets or similar patterns
6. **Sensitive data handling** - parameters may contain references but not actual secrets

## Future Extensibility Points

1. **Plugin system** for custom validators (future consideration)
2. **Remote package loading** (git, https)
3. **Package signing** and verification
4. **Advanced patch operations** (JSONPatch, strategic merge)
5. **Dependency resolution** between packages
6. **Observability and metrics** (future consideration)
7. **Enhanced debugging tools** for patch application

## Design Constraints

- No templating engines (use patches)
- No runtime operations (just generate YAML)
- No cluster connectivity required
- No package registry dependency
- Deterministic output (same input = same output)
- No direct secret creation (use external references)
- Patches must succeed or error (no silent failures)
- Local patches can override package patches