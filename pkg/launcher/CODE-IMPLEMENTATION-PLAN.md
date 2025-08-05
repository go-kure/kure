# Launcher Module - Implementation Plan

## Overview

This document provides a detailed implementation plan for the Kurel launcher module, breaking down the work into concrete tasks with specific implementation details.

## Phase 1: Core Foundation (Week 1)

### Task 1.1: Create Base Types and Interfaces
**Files to create:**
- `pkg/launcher/types.go` - Core data structures
- `pkg/launcher/interfaces.go` - Public interfaces
- `pkg/launcher/errors.go` - Custom error types

**Implementation:**
```go
// types.go
package launcher

type KurelMetadata struct {
    Name        string   `yaml:"name"`
    Version     string   `yaml:"version"`
    AppVersion  string   `yaml:"appVersion"`
    Description string   `yaml:"description"`
    Home        string   `yaml:"home"`
    Keywords    []string `yaml:"keywords"`
    Schemas     []string `yaml:"schemas"`  // CRD schema URLs
}

type ParameterMap map[string]interface{}

type Resource struct {
    APIVersion string            `yaml:"apiVersion"`
    Kind       string            `yaml:"kind"`
    Metadata   ResourceMetadata  `yaml:"metadata"`
    Content    []byte            // Raw YAML content
}

type Patch struct {
    Name      string
    Path      string
    Content   string         // TOML content
    Metadata  *PatchMetadata
}

type PatchMetadata struct {
    Enabled     string   `yaml:"enabled"`
    Description string   `yaml:"description"`
    Requires    []string `yaml:"requires"`
    Conflicts   []string `yaml:"conflicts"`
}

type PackageDefinition struct {
    Path       string
    Metadata   KurelMetadata
    Parameters ParameterMap
    Resources  []Resource
    Patches    []Patch
}

type PackageInstance struct {
    Definition *PackageDefinition
    UserValues ParameterMap
    Resolved   ParameterMap
    LocalPath  string
}
```

**Tests to write:**
- Type marshaling/unmarshaling tests
- Interface compliance tests

### Task 1.2: Implement Package Loader
**Files to create:**
- `pkg/launcher/loader.go` - Package loading logic
- `pkg/launcher/loader_test.go` - Loader tests

**Implementation details:**
```go
type defaultLoader struct {
    fs afero.Fs  // For testing with mock filesystem
}

func (l *defaultLoader) LoadDefinition(path string) (*PackageDefinition, error) {
    // 1. Validate package directory structure
    if err := validatePackageStructure(path); err != nil {
        return nil, err
    }
    
    // 2. Load parameters.yaml (critical)
    params, err := l.loadParameters(filepath.Join(path, "parameters.yaml"))
    if err != nil {
        return nil, fmt.Errorf("critical: %w", err)
    }
    
    // 3. Extract metadata from kurel: key
    metadata, err := extractMetadata(params)
    
    // 4. Load resources (best effort)
    resources, resourceErrs := l.loadResources(filepath.Join(path, "resources"))
    
    // 5. Discover and load patches (best effort)
    patches, patchErrs := l.loadPatches(filepath.Join(path, "patches"))
    
    // 6. Collect non-critical errors
    var errs []error
    errs = append(errs, resourceErrs...)
    errs = append(errs, patchErrs...)
    
    def := &PackageDefinition{
        Path:       path,
        Metadata:   metadata,
        Parameters: params,
        Resources:  resources,
        Patches:    patches,
    }
    
    if len(errs) > 0 {
        return def, &LoadErrors{Errors: errs}
    }
    
    return def, nil
}
```

**Key functions to implement:**
- `validatePackageStructure()` - Check required directories exist
- `loadParameters()` - Parse parameters.yaml with validation
- `extractMetadata()` - Extract kurel: key from parameters
- `loadResources()` - Discover and parse all resource YAML files
- `loadPatches()` - Discover .kpatch files and metadata
- `sortPatches()` - Sort by numeric prefix and path

**Tests:**
- Valid package loading
- Missing parameters.yaml handling
- Malformed YAML handling
- Patch discovery and ordering
- Error collection for non-critical files

## Phase 2: Variable Resolution (Week 1-2)

### Task 2.1: Implement Variable Resolver
**Files to create:**
- `pkg/launcher/variables.go` - Variable resolution engine
- `pkg/launcher/variables_test.go` - Resolution tests

**Implementation details:**
```go
type defaultResolver struct {
    maxDepth int
}

type ResolverOption func(*defaultResolver)

func WithMaxDepth(depth int) ResolverOption {
    return func(r *defaultResolver) {
        r.maxDepth = depth
    }
}

func NewResolver(opts ...ResolverOption) Resolver {
    r := &defaultResolver{maxDepth: 10}
    for _, opt := range opts {
        opt(r)
    }
    return r
}

func (r *defaultResolver) Resolve(base, overrides ParameterMap) (ParameterMap, error) {
    // 1. Deep merge parameters
    merged := deepMerge(base, overrides)
    
    // 2. Extract all variable references
    refs := extractVariableRefs(merged)
    
    // 3. Build dependency graph
    graph := buildDependencyGraph(refs)
    
    // 4. Detect circular dependencies
    if cycles := detectCycles(graph); len(cycles) > 0 {
        return nil, fmt.Errorf("circular dependencies: %v", cycles)
    }
    
    // 5. Topological sort for resolution order
    order := topologicalSort(graph)
    
    // 6. Resolve in order
    resolved := make(ParameterMap)
    for _, key := range order {
        value, err := resolveValue(merged, key, resolved, 0, r.maxDepth)
        if err != nil {
            return nil, fmt.Errorf("failed to resolve %s: %w", key, err)
        }
        resolved[key] = value
    }
    
    return resolved, nil
}
```

**Key functions:**
- `deepMerge()` - Merge parameter maps preserving structure
- `extractVariableRefs()` - Find all ${...} patterns
- `buildDependencyGraph()` - Create variable dependency DAG
- `detectCycles()` - Find circular dependencies
- `resolveValue()` - Recursively resolve variable value
- `substituteVariables()` - Replace ${...} with resolved values

**Tests:**
- Simple variable substitution
- Nested variable references
- Circular dependency detection
- Maximum depth enforcement
- Missing variable errors
- Complex parameter merging

## Phase 3: Patch System (Week 2)

### Task 3.1: Implement Patch Discovery and Dependencies
**Files to create:**
- `pkg/launcher/patches.go` - Patch processing logic
- `pkg/launcher/patches_test.go` - Patch tests

**Implementation details:**
```go
type patchProcessor struct {
    logger Logger
}

func (p *patchProcessor) DiscoverPatches(patchDir string) ([]Patch, error) {
    var patches []Patch
    
    // 1. Glob for *.kpatch files
    files, err := filepath.Glob(filepath.Join(patchDir, "**/*.kpatch"))
    if err != nil {
        return nil, err
    }
    
    // 2. Sort by numeric prefix, then alphabetically
    sort.Slice(files, func(i, j int) bool {
        return numericPrefixSort(files[i], files[j])
    })
    
    // 3. Load each patch and metadata
    for _, file := range files {
        patch, err := p.loadPatch(file)
        if err != nil {
            continue // Collect error, continue loading
        }
        patches = append(patches, patch)
    }
    
    return patches, nil
}

func (p *patchProcessor) ResolveDependencies(patches []Patch, params ParameterMap) ([]Patch, error) {
    // 1. Evaluate enabled conditions
    enabled := make(map[string]bool)
    for _, patch := range patches {
        if patch.Metadata != nil && patch.Metadata.Enabled != "" {
            enabled[patch.Name] = evaluateCondition(patch.Metadata.Enabled, params)
        } else {
            enabled[patch.Name] = true // Default enabled
        }
    }
    
    // 2. Build dependency graph
    graph := buildPatchDependencyGraph(patches)
    
    // 3. Auto-enable required patches
    for _, patch := range patches {
        if enabled[patch.Name] && patch.Metadata != nil {
            for _, req := range patch.Metadata.Requires {
                p.logger.Info("Auto-enabling %s (required by %s)", req, patch.Name)
                enabled[req] = true
            }
        }
    }
    
    // 4. Check for conflicts
    for _, patch := range patches {
        if !enabled[patch.Name] {
            continue
        }
        if patch.Metadata != nil {
            for _, conflict := range patch.Metadata.Conflicts {
                if enabled[conflict] {
                    return nil, fmt.Errorf("conflict: %s and %s cannot both be enabled", 
                        patch.Name, conflict)
                }
            }
        }
    }
    
    // 5. Filter and sort enabled patches
    var result []Patch
    for _, patch := range patches {
        if enabled[patch.Name] {
            result = append(result, patch)
        }
    }
    
    return result, nil
}
```

**Tests:**
- Patch discovery with numeric ordering
- Dependency resolution
- Auto-enable behavior
- Conflict detection
- Conditional enabling

### Task 3.2: Integrate with Patch Engine
**Files to modify:**
- `pkg/launcher/patches.go` - Add apply functionality

**Implementation:**
```go
func (p *patchProcessor) ApplyPatches(resources []Resource, patches []Patch, resolved ParameterMap) ([]Resource, error) {
    engine := patch.NewEngine()
    
    for _, patchDef := range patches {
        // 1. Substitute variables in patch content
        substituted := substituteVariables(patchDef.Content, resolved)
        
        // 2. Parse patch
        patchOps, err := engine.Parse(substituted)
        if err != nil {
            return nil, fmt.Errorf("failed to parse patch %s: %w", patchDef.Name, err)
        }
        
        // 3. Apply to matching resources
        matched := false
        for i, resource := range resources {
            if matches, err := engine.Matches(resource, patchOps); err != nil {
                return nil, err
            } else if matches {
                matched = true
                resources[i], err = engine.Apply(resource, patchOps)
                if err != nil {
                    return nil, fmt.Errorf("failed to apply patch %s: %w", patchDef.Name, err)
                }
            }
        }
        
        if !matched {
            return nil, fmt.Errorf("patch %s matched no resources", patchDef.Name)
        }
    }
    
    return resources, nil
}
```

## Phase 4: Validation and Schema (Week 2-3)

### Task 4.1: Implement Schema Generator
**Files to create:**
- `pkg/launcher/schema.go` - Schema generation logic
- `pkg/launcher/schema_test.go` - Schema tests
- `pkg/launcher/schemas/` - Bundled K8s schemas

**Implementation:**
```go
type defaultSchemaGenerator struct {
    bundledSchemas map[string]Schema
    tracer         *pathTracer
}

func (g *defaultSchemaGenerator) GenerateSchema(def *PackageDefinition) (*Schema, error) {
    schema := &Schema{
        Type: "object",
        Properties: make(map[string]SchemaProperty),
    }
    
    // Phase 1: Type inference from parameter values
    for key, value := range def.Parameters {
        schema.Properties[key] = inferType(value)
    }
    
    // Phase 2: Trace patch paths to K8s fields
    for _, patch := range def.Patches {
        traces := g.tracer.TracePaths(patch.Content)
        for _, trace := range traces {
            if k8sSchema := g.getK8sSchema(trace.ResourceType, trace.Field); k8sSchema != nil {
                // Enhance parameter schema with K8s constraints
                enhanceSchema(schema, trace.Variable, k8sSchema)
            }
        }
    }
    
    // Phase 3: Load custom CRD schemas from URLs
    if len(def.Metadata.Schemas) > 0 {
        for _, url := range def.Metadata.Schemas {
            crdSchema, err := fetchSchema(url)
            if err != nil {
                continue // Log warning, continue
            }
            g.bundledSchemas[crdSchema.Kind] = crdSchema
        }
    }
    
    return schema, nil
}
```

**Bundled schemas to include:**
- Core K8s resources (Deployment, Service, ConfigMap, etc.)
- Resources from internal/ packages (FluxCD, cert-manager, MetalLB, etc.)

### Task 4.2: Implement Validator
**Files to create:**
- `pkg/launcher/validator.go` - Validation logic
- `pkg/launcher/validator_test.go` - Validation tests

**Implementation:**
```go
type defaultValidator struct {
    schemaGen SchemaGenerator
    logger    Logger
}

func (v *defaultValidator) ValidateDefinition(def *PackageDefinition) ValidationResult {
    result := ValidationResult{}
    
    // 1. Validate package structure
    if err := validateStructure(def); err != nil {
        result.Errors = append(result.Errors, err)
    }
    
    // 2. Check patch variable references
    for _, patch := range def.Patches {
        refs := extractVariableRefs(patch.Content)
        for _, ref := range refs {
            if !parameterExists(ref, def.Parameters) {
                result.Errors = append(result.Errors, 
                    fmt.Errorf("patch %s: variable %s not defined", patch.Name, ref))
            }
        }
    }
    
    // 3. Validate patch dependencies exist
    patchNames := make(map[string]bool)
    for _, patch := range def.Patches {
        patchNames[patch.Name] = true
    }
    
    for _, patch := range def.Patches {
        if patch.Metadata != nil {
            for _, req := range patch.Metadata.Requires {
                if !patchNames[req] {
                    result.Errors = append(result.Errors,
                        fmt.Errorf("patch %s requires non-existent patch %s", patch.Name, req))
                }
            }
        }
    }
    
    return result
}

func (v *defaultValidator) ValidateInstance(inst *PackageInstance) ValidationResult {
    result := ValidationResult{}
    
    // 1. Validate parameters against schema
    schema, err := v.schemaGen.GenerateSchema(inst.Definition)
    if err != nil {
        result.Warnings = append(result.Warnings, 
            fmt.Sprintf("Could not generate schema: %v", err))
    } else {
        if errs := validateAgainstSchema(inst.Resolved, schema); len(errs) > 0 {
            result.Errors = append(result.Errors, errs...)
        }
    }
    
    // 2. Validate all variables resolve
    for key, value := range inst.Resolved {
        if strings.Contains(fmt.Sprint(value), "${") {
            result.Errors = append(result.Errors,
                fmt.Errorf("unresolved variable in %s: %v", key, value))
        }
    }
    
    // 3. Validate K8s resources
    for _, resource := range inst.Definition.Resources {
        if err := v.validateResource(resource); err != nil {
            result.Errors = append(result.Errors, err)
        }
    }
    
    return result
}

func (v *defaultValidator) validateResource(resource Resource) error {
    // Try full validation with schema
    if schema := getK8sSchema(resource.APIVersion, resource.Kind); schema != nil {
        return validateWithSchema(resource, schema)
    }
    
    // Fallback to medium validation
    return validateMedium(resource)
}
```

## Phase 5: Output and Extensions (Week 3)

### Task 5.1: Implement Output Builder
**Files to create:**
- `pkg/launcher/builder.go` - Output generation logic
- `pkg/launcher/builder_test.go` - Builder tests

**Implementation:**
```go
type OutputFormat string

const (
    OutputFormatSingle     OutputFormat = "single"
    OutputFormatByKind     OutputFormat = "by-kind"
    OutputFormatByResource OutputFormat = "by-resource"
)

type OutputType string

const (
    OutputTypeYAML OutputType = "yaml"
    OutputTypeJSON OutputType = "json"
)

type defaultBuilder struct {
    writer FileWriter
}

func (b *defaultBuilder) Build(inst *PackageInstance, opts BuildOptions) error {
    // 1. Apply patches to resources
    processor := newPatchProcessor()
    resources, err := processor.ApplyPatches(
        inst.Definition.Resources,
        inst.Definition.Patches,
        inst.Resolved,
    )
    if err != nil {
        return err
    }
    
    // 2. Sort resources by phase annotations
    phased := organizeByPhase(resources)
    
    // 3. Output based on options
    switch opts.OutputPath {
    case "", "-":
        // Write to stdout
        return b.writeToStdout(resources, opts.OutputType)
    default:
        // Write to files
        return b.writeToFiles(resources, opts.OutputPath, opts.OutputFormat, opts.OutputType)
    }
}

func (b *defaultBuilder) writeToFiles(resources []Resource, path string, format OutputFormat, outputType OutputType) error {
    switch format {
    case OutputFormatSingle:
        return b.writeSingleFile(resources, path, outputType)
    case OutputFormatByKind:
        return b.writeByKind(resources, path, outputType)
    case OutputFormatByResource:
        return b.writeByResource(resources, path, outputType)
    default:
        return fmt.Errorf("unknown output format: %s", format)
    }
}
```

### Task 5.2: Implement Local Extensions
**Files to create:**
- `pkg/launcher/extensions.go` - Extension handling
- `pkg/launcher/extensions_test.go` - Extension tests

**Implementation:**
```go
type extensionLoader struct {
    loader Loader
}

func (e *extensionLoader) LoadWithExtensions(def *PackageDefinition, localPath string) (*PackageDefinition, error) {
    // 1. Check if local extension exists
    if localPath == "" {
        localPath = def.Path + ".local.kurel"
    }
    
    if !exists(localPath) {
        return def, nil
    }
    
    // 2. Load local parameters
    localParams, err := e.loadLocalParameters(localPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load local parameters: %w", err)
    }
    
    // 3. Validate parameter compatibility
    if err := validateParameterCompatibility(def.Parameters, localParams); err != nil {
        return nil, fmt.Errorf("incompatible local parameters: %w", err)
    }
    
    // 4. Load local patches
    localPatches, err := e.loadLocalPatches(localPath)
    if err != nil {
        return nil, fmt.Errorf("failed to load local patches: %w", err)
    }
    
    // 5. Merge with package
    extended := &PackageDefinition{
        Path:       def.Path,
        Metadata:   def.Metadata,
        Parameters: deepMerge(def.Parameters, localParams),
        Resources:  def.Resources, // Cannot modify resources
        Patches:    append(def.Patches, localPatches...),
    }
    
    return extended, nil
}

func validateParameterCompatibility(base, override ParameterMap) error {
    for key, overrideValue := range override {
        if baseValue, exists := base[key]; exists {
            if !compatibleTypes(baseValue, overrideValue) {
                return fmt.Errorf("parameter %s: incompatible type change", key)
            }
        }
    }
    return nil
}
```

## Phase 6: CLI Integration (Week 3-4)

### Task 6.1: Implement CLI Commands
**Files to modify:**
- `pkg/cmd/kurel/build.go` - Build command
- `pkg/cmd/kurel/validate.go` - Validate command
- `pkg/cmd/kurel/info.go` - Info command
- `pkg/cmd/kurel/schema.go` - Schema command

**Build command implementation:**
```go
func newBuildCommand(globalOpts *options.GlobalOptions) *cobra.Command {
    var (
        valuesFile     string
        outputPath     string
        outputFormat   string
        outputType     string
        localPath      string
        verbose        bool
        showPatches    bool
    )
    
    cmd := &cobra.Command{
        Use:   "build <package>",
        Short: "Build Kubernetes manifests from kurel package",
        Args:  cobra.ExactArgs(1),
        RunE: func(cmd *cobra.Command, args []string) error {
            // Setup logging
            logger := setupLogger(verbose)
            
            // 1. Load package definition
            logger.Info("Loading package from %s", args[0])
            loader := launcher.NewLoader()
            def, err := loader.LoadDefinition(args[0])
            if err != nil {
                if loadErrs, ok := err.(*launcher.LoadErrors); ok {
                    for _, e := range loadErrs.Errors {
                        logger.Warn("Load warning: %v", e)
                    }
                } else {
                    return fmt.Errorf("failed to load package: %w", err)
                }
            }
            
            // 2. Load local extensions
            if localPath != "" || exists(args[0]+".local.kurel") {
                logger.Info("Loading local extensions")
                def, err = launcher.LoadWithExtensions(def, localPath)
                if err != nil {
                    return fmt.Errorf("failed to load extensions: %w", err)
                }
            }
            
            // 3. Load user values
            userValues := make(launcher.ParameterMap)
            if valuesFile != "" {
                logger.Info("Loading values from %s", valuesFile)
                userValues, err = loadValuesFile(valuesFile)
                if err != nil {
                    return fmt.Errorf("failed to load values: %w", err)
                }
            }
            
            // 4. Create instance
            instance := &launcher.PackageInstance{
                Definition: def,
                UserValues: userValues,
                LocalPath:  localPath,
            }
            
            // 5. Resolve variables
            logger.Info("Resolving variables")
            resolver := launcher.NewResolver()
            instance.Resolved, err = resolver.Resolve(def.Parameters, userValues)
            if err != nil {
                return fmt.Errorf("failed to resolve variables: %w", err)
            }
            
            // 6. Process patches
            logger.Info("Processing patches")
            processor := launcher.NewPatchProcessor(launcher.WithLogger(logger))
            enabledPatches, err := processor.ResolveDependencies(def.Patches, instance.Resolved)
            if err != nil {
                return fmt.Errorf("failed to resolve patch dependencies: %w", err)
            }
            
            if showPatches {
                fmt.Fprintf(os.Stderr, "Enabled patches:\n")
                for _, p := range enabledPatches {
                    fmt.Fprintf(os.Stderr, "  - %s\n", p.Name)
                }
            }
            
            // 7. Validate
            logger.Info("Validating configuration")
            validator := launcher.NewValidator()
            result := validator.ValidateInstance(instance)
            
            if result.HasErrors() {
                for _, err := range result.Errors {
                    logger.Error("Validation error: %v", err)
                }
                return fmt.Errorf("validation failed with %d errors", len(result.Errors))
            }
            
            for _, warn := range result.Warnings {
                logger.Warn("Validation warning: %v", warn)
            }
            
            // 8. Build output
            logger.Info("Building manifests")
            builder := launcher.NewBuilder()
            opts := launcher.BuildOptions{
                OutputPath:   outputPath,
                OutputFormat: launcher.OutputFormat(outputFormat),
                OutputType:   launcher.OutputType(outputType),
            }
            
            if outputPath == "" {
                opts.OutputPath = "-" // stdout
            }
            
            if err := builder.Build(instance, opts); err != nil {
                return fmt.Errorf("failed to build manifests: %w", err)
            }
            
            logger.Info("Build complete")
            return nil
        },
    }
    
    cmd.Flags().StringVarP(&valuesFile, "values", "f", "", "Values file for overrides")
    cmd.Flags().StringVarP(&outputPath, "output", "o", "", "Output path (default: stdout)")
    cmd.Flags().StringVar(&outputFormat, "format", "single", "Output format: single|by-kind|by-resource")
    cmd.Flags().StringVar(&outputType, "output-format", "yaml", "Output type: yaml|json")
    cmd.Flags().StringVar(&localPath, "local", "", "Path to .local.kurel directory")
    cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "Verbose output")
    cmd.Flags().BoolVar(&showPatches, "show-patches", false, "Show enabled patches")
    
    return cmd
}
```

## Phase 7: Testing (Week 4)

### Task 7.1: Create Test Fixtures
**Files to create:**
- `pkg/launcher/testdata/packages/simple/` - Basic package
- `pkg/launcher/testdata/packages/complex/` - Complex package with patches
- `pkg/launcher/testdata/packages/invalid/` - Invalid packages for error testing

### Task 7.2: Write Comprehensive Tests
**Test coverage targets:**
- Package loading: 90%
- Variable resolution: 95%
- Patch processing: 90%
- Validation: 85%
- Schema generation: 80%
- Output generation: 90%

**Key test scenarios:**
1. **Loader tests:**
   - Valid package loading
   - Missing critical files
   - Malformed YAML/TOML
   - Patch discovery ordering

2. **Variable tests:**
   - Simple substitution
   - Nested references
   - Circular dependencies
   - Missing variables
   - Deep merging

3. **Patch tests:**
   - Dependency resolution
   - Auto-enabling
   - Conflict detection
   - Variable substitution in patches
   - Patch application failures

4. **Validation tests:**
   - Schema validation
   - Resource validation
   - Parameter compatibility
   - K8s resource constraints

5. **Integration tests:**
   - Full build pipeline
   - Local extensions
   - Multiple output formats
   - Error handling

## Implementation Timeline

### Week 1
- [ ] Core types and interfaces
- [ ] Package loader
- [ ] Variable resolver (start)

### Week 2
- [ ] Variable resolver (complete)
- [ ] Patch discovery and dependencies
- [ ] Patch application

### Week 3
- [ ] Schema generation
- [ ] Validation system
- [ ] Output builder
- [ ] Local extensions

### Week 4
- [ ] CLI integration
- [ ] Comprehensive testing
- [ ] Documentation
- [ ] Performance optimization

## Testing Strategy

### Unit Tests
- Each module tested in isolation
- Mock dependencies using interfaces
- Table-driven tests for complex logic
- Use `testify/assert` for assertions

### Integration Tests
- Full package processing flows
- Real filesystem operations
- End-to-end CLI testing
- Performance benchmarks

### Test Data
- Create realistic test packages
- Include edge cases and error conditions
- Test with actual Kubernetes resources
- Validate against real K8s schemas

## Performance Targets

- Package loading: < 100ms for typical package
- Variable resolution: < 50ms for 100 variables
- Patch application: < 200ms for 50 patches
- Schema generation: < 500ms (with caching)
- Full build: < 1s for typical package

## Documentation Requirements

### Code Documentation
- GoDoc comments for all public types and functions
- Example usage in documentation
- Clear error messages with context

### User Documentation
- Update README.md with launcher usage
- Create examples/ directory with sample packages
- Document all CLI commands and flags
- Provide troubleshooting guide

## Success Criteria

1. **Functionality**: All features from design implemented
2. **Quality**: >85% test coverage, no critical bugs
3. **Performance**: Meets performance targets
4. **Usability**: Clear CLI interface with helpful output
5. **Documentation**: Complete API and user documentation
6. **Integration**: Seamless integration with existing Kure codebase