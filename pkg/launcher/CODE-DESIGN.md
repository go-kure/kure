# Launcher Module - Code Design

## Overview

The launcher module is the core engine for the Kurel package system, implementing a declarative approach to generating Kubernetes manifests with validation and customization capabilities. This document captures all design decisions made during the architecture planning phase.

## Design Philosophy

**Core Principle**: "Kurel just generates YAML" - The launcher is a declarative system for generating Kubernetes manifests, not a runtime system or orchestrator.

## Architecture Decisions

### 1. Core Package Structure

**Decision**: Immutable `PackageDefinition` with deep copy support

```go
// Immutable package definition with thread-safe access
type PackageDefinition struct {
    Path        string
    Metadata    KurelMetadata     // From kurel: key in parameters.yaml
    Parameters  ParameterMap      // Default parameters including global:
    Resources   []Resource        // Base K8s manifests
    Patches     []Patch          // Available patches with metadata
    mu          sync.RWMutex      // Protect concurrent reads
}

// DeepCopy creates an independent copy for safe mutation
func (pd *PackageDefinition) DeepCopy() *PackageDefinition {
    pd.mu.RLock()
    defer pd.mu.RUnlock()
    
    return &PackageDefinition{
        Path:       pd.Path,
        Metadata:   pd.Metadata, // struct copy
        Parameters: deepCopyMap(pd.Parameters),
        Resources:  deepCopyResources(pd.Resources),
        Patches:    deepCopyPatches(pd.Patches),
    }
}

// Instance with user customization
type PackageInstance struct {
    Definition  *PackageDefinition  // Immutable package reference
    UserValues  ParameterMap        // User-provided overrides
    Resolved    ParameterMapWithSource // Final values with source tracking
    LocalPath   string              // Path to .local.kurel if exists
}

// Track parameter sources for debugging
type ParameterSource struct {
    Value    interface{}
    Location string // "package", "local", "default"
    File     string // Which file it came from
    Line     int    // Line number if applicable
}

type ParameterMapWithSource map[string]ParameterSource
```

**Rationale**: 
- Deep copy prevents mutation bugs in concurrent processing
- Source tracking aids debugging
- Thread-safe access for concurrent reads
- Package definitions remain cacheable and reusable

### 2. Interface Organization & Common Options

**Decision**: Small interfaces with centralized LauncherOptions

```go
// LauncherOptions centralizes common configuration
type LauncherOptions struct {
    Logger       Logger
    MaxDepth     int           // Variable resolution depth
    Timeout      time.Duration // Operation timeout
    MaxWorkers   int           // Concurrent processing
    CacheDir     string        // Schema cache directory
    Debug        bool          // Enable debug output
    Verbose      bool          // Verbose logging
    ProgressFunc func(string)  // Progress callback
}

// DefaultOptions provides sensible defaults
func DefaultOptions() *LauncherOptions {
    return &LauncherOptions{
        Logger:     NewDefaultLogger(),
        MaxDepth:   10,
        Timeout:    30 * time.Second,
        MaxWorkers: runtime.NumCPU(),
        CacheDir:   "/tmp/kurel-cache",
        Debug:      false,
        Verbose:    false,
    }
}
```

**Decision**: Small, focused interfaces following Go idioms

```go
// pkg/launcher/interfaces.go - Small, composable interfaces
type DefinitionLoader interface {
    LoadDefinition(ctx context.Context, path string, opts *LauncherOptions) (*PackageDefinition, error)
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
    Resolve(ctx context.Context, base, overrides ParameterMap, opts *LauncherOptions) (ParameterMapWithSource, error)
    DebugVariableGraph(params ParameterMap) string // Generate dependency graph
}

type Builder interface {
    Build(ctx context.Context, inst *PackageInstance, buildOpts BuildOptions, opts *LauncherOptions) error
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

**Decision**: Strict validation with uniqueness enforcement and debug visualization

- **Name uniqueness**: Enforced at load time to prevent confusion
- **Conflicts**: Hard error, refuse to continue
- **Auto-enable**: Verbose logging to stderr
- **Missing targets**: Error, patches must match something
- **Application order**: Package patches first (by numeric prefix), then local patches (can override)
- **Failure handling**: Patches MUST apply successfully or error - no silent failures
- **Debug support**: Patch dependency graph visualization

```go
// Enforce patch name uniqueness at load time
func (l *defaultLoader) loadPatches(ctx context.Context, path string) ([]Patch, error) {
    patches := []Patch{}
    seen := make(map[string]string) // name -> file path
    
    err := filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
        if !strings.HasSuffix(p, ".kpatch") {
            return nil
        }
        
        patch, err := l.loadPatch(p)
        if err != nil {
            return err
        }
        
        // Check uniqueness
        if existing, exists := seen[patch.Name]; exists {
            return fmt.Errorf("duplicate patch name '%s' in %s and %s", 
                patch.Name, existing, p)
        }
        seen[patch.Name] = p
        
        patches = append(patches, patch)
        return nil
    })
    
    return patches, err
}

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

// Patch works on deep copy to preserve immutability
func (p *patchProcessor) ApplyPatches(ctx context.Context, def *PackageDefinition, patches []Patch, params ParameterMap) (*PackageDefinition, error) {
    // Work on deep copy, never mutate original
    working := def.DeepCopy()
    
    for _, patch := range patches {
        result, err := p.applyPatch(working, patch, params)
        if err != nil {
            return nil, fmt.Errorf("patch %s failed: %w", patch.Name, err)
        }
        working = result
    }
    
    return working, nil
}

// Debug visualization for patch dependencies
func (p *patchProcessor) DebugPatchGraph(patches []Patch) string {
    var buf strings.Builder
    buf.WriteString("digraph patches {\n")
    
    for _, patch := range patches {
        if patch.Metadata != nil {
            for _, req := range patch.Metadata.Requires {
                buf.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\";\n", patch.Name, req))
            }
            for _, conflict := range patch.Metadata.Conflicts {
                buf.WriteString(fmt.Sprintf("  \"%s\" -> \"%s\" [color=red,style=dashed];\n", 
                    patch.Name, conflict))
            }
        }
    }
    
    buf.WriteString("}\n")
    return buf.String()
}
```

**Rationale**:
- Immutability prevents subtle bugs
- Name uniqueness prevents configuration errors
- Debug visualization aids troubleshooting
- Clear visibility into auto-enabled dependencies
- No silent failures

### 6. Schema Generation

**Decision**: Hybrid approach with memoization and type validation

- Bundle schemas for resources known in internal/ packages
- Package maintainers can specify CRD schema URLs in parameters.yaml
- Auto-generate if missing, allow explicit regeneration
- Memoize path tracing for performance
- Validate type consistency across patches

```yaml
# In parameters.yaml
kurel:
  name: my-app
  schemas:
    - https://raw.githubusercontent.com/cert-manager/cert-manager/v1.13.0/deploy/crds/crd-certificates.yaml
    - https://raw.githubusercontent.com/prometheus-operator/prometheus-operator/v0.68.0/deploy/crds/crd-prometheuses.yaml
```

```go
// Memoized path tracer for performance
type memoizedPathTracer struct {
    cache map[string]*TraceResult // path:variable → schema mapping
    mu    sync.RWMutex
}

func (t *memoizedPathTracer) TracePath(patchPath, variable string) (*TraceResult, error) {
    key := fmt.Sprintf("%s:%s", patchPath, variable)
    
    t.mu.RLock()
    if result, ok := t.cache[key]; ok {
        t.mu.RUnlock()
        return result, nil
    }
    t.mu.RUnlock()
    
    // Compute if not cached
    result, err := t.doTrace(patchPath, variable)
    if err != nil {
        return nil, err
    }
    
    t.mu.Lock()
    t.cache[key] = result
    t.mu.Unlock()
    
    return result, nil
}

// Validate type consistency across patches
func (g *SchemaGenerator) ValidateSchemaMerge(patches []Patch) error {
    typeMap := make(map[string]string) // variable → expected type
    
    for _, patch := range patches {
        traces := g.tracer.TraceAll(patch.Content)
        
        for variable, trace := range traces {
            expectedType := trace.Schema.Type
            
            if existing, ok := typeMap[variable]; ok {
                if existing != expectedType {
                    return fmt.Errorf("type conflict for %s: patch %s expects %s, but %s expected",
                        variable, patch.Name, expectedType, existing)
                }
            } else {
                typeMap[variable] = expectedType
            }
        }
    }
    
    return nil
}
```

**Rationale**:
- Memoization improves performance for repeated operations
- Type validation prevents schema conflicts
- Leverages existing Kure knowledge
- Extensible for custom CRDs

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
├── types.go           # Core data types with deep copy
├── options.go         # LauncherOptions and configuration
├── loader.go          # Package loading with uniqueness checks
├── variables.go       # Variable resolution with source tracking
├── patches.go         # Patch processing with immutability
├── schema.go          # Schema generation with memoization
├── validator.go       # Validation logic
├── builder.go         # Manifest building and output
├── extensions.go      # Local extension handling
├── errors.go          # Custom error types
├── debug.go           # Debug visualization (graphs, etc.)
├── deepcopy.go        # Deep copy utilities
└── testdata/         # Test fixtures
    ├── packages/     # Sample packages for testing
    └── mocks/        # Mock implementations for testing
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
- **Individual component tests**: Each component independently testable
- **Benchmarks**: Ensure performance matches Helm
- **Mock filesystem**: Using afero for loader testing
- **Integration tests**: Full package processing flows
- **Fixture packages**: Real-world package examples in testdata/
- **Concurrent testing**: Validate thread safety

### Individual Component Testability

```go
// Test resolver independently
func TestResolverIndependent(t *testing.T) {
    opts := &LauncherOptions{MaxDepth: 5}
    resolver := NewResolver(opts)
    
    params := ParameterMap{
        "app": "myapp",
        "tag": "v1.0",
        "image": "${app}:${tag}",
    }
    
    resolved, err := resolver.Resolve(context.Background(), params, nil, opts)
    require.NoError(t, err)
    assert.Equal(t, "myapp:v1.0", resolved["image"].Value)
}

// Test patch processor independently
func TestPatchProcessorIndependent(t *testing.T) {
    opts := DefaultOptions()
    processor := NewPatchProcessor(opts)
    
    // Mock resources
    def := &PackageDefinition{
        Resources: []Resource{
            {Kind: "Deployment", Metadata: metav1.ObjectMeta{Name: "app"}},
        },
    }
    
    patch := Patch{
        Name:    "scale",
        Content: "[deployment.app]\nspec.replicas: 3",
    }
    
    result, err := processor.ApplyPatches(context.Background(), def, []Patch{patch}, ParameterMap{})
    require.NoError(t, err)
    assert.NotEqual(t, def, result) // Ensure deep copy
}

// Test validator independently with mock schema
func TestValidatorIndependent(t *testing.T) {
    mockSchema := &MockSchemaGenerator{
        schema: &Schema{
            Properties: map[string]SchemaProperty{
                "replicas": {Type: "integer", Minimum: 1},
            },
        },
    }
    
    opts := DefaultOptions()
    validator := NewValidator(opts, WithSchemaGenerator(mockSchema))
    
    params := ParameterMap{"replicas": 0}
    result := validator.ValidateParameters(context.Background(), params)
    
    assert.Len(t, result.Errors, 1)
    assert.Contains(t, result.Errors[0].Error(), "below minimum")
}
```

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
7. **Schema merging validation** - Detect type mismatches when patches imply incompatible schemas
8. **Interactive debug mode** - Step through patch application
9. **Package composition** - Combine multiple packages safely

## Design Constraints

- No templating engines (use patches)
- No runtime operations (just generate YAML)
- No cluster connectivity required
- No package registry dependency
- Deterministic output (same input = same output)
- No direct secret creation (use external references)
- Patches must succeed or error (no silent failures)
- Local patches can override package patches