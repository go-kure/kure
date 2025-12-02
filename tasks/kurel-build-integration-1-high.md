# Task: Wire Generator into kurel build

**Priority:** 1 - High (Short Term: 2-4 weeks)
**Category:** kurel
**Status:** ✅ Completed
**Completed:** 2025-12-03 (commit 9453a52)
**Dependencies:** kurel-generator-mvp-1-high.md
**Blocked By:** None

---

## Overview

Integrate the KurelPackage generator into the `kurel build` CLI command so users can author `KurelPackage` YAML files and use them to produce complete kurel packages.

## Current Status

- `kurel build` currently works with existing kurel package structures
- KurelPackage generator is scaffolded but not wired into the build pipeline
- No command-line support for building from KurelPackage YAML

## Objectives

Enable this workflow:

```bash
# User writes a KurelPackage YAML
cat > my-app.yaml <<EOF
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: my-application
version: v1.0.0
description: My application package
parameters:
  replicas:
    type: integer
    default: 3
  image:
    type: string
    default: nginx:latest
resources:
  - apiVersion: kure.dev/v1alpha1
    kind: AppWorkload
    # ... app config
EOF

# Build kurel package from KurelPackage YAML
kurel build --from-generator my-app.yaml --output ./my-app-package

# Resulting structure:
# ./my-app-package/
#   kurel.yaml
#   parameters.yaml
#   resources/
#     deployment.yaml
#     service.yaml
#   patches/
#     replicas.kpatch
#     image.kpatch
```

## Implementation Tasks

### 1. Add CLI Flag to kurel build

**File:** `pkg/cmd/kurel/cmd.go`

```go
// Add flag to build command
func newBuildCommand(opts *LauncherOptions) *cobra.Command {
    var (
        outputDir     string
        fromGenerator string  // NEW FLAG
    )

    cmd := &cobra.Command{
        Use:   "build [package-dir]",
        Short: "Build a kurel package",
        RunE: func(cmd *cobra.Command, args []string) error {
            // NEW: Check if building from generator
            if fromGenerator != "" {
                return buildFromGenerator(fromGenerator, outputDir, opts)
            }

            // Existing: build from package directory
            return buildFromPackage(args[0], outputDir, opts)
        },
    }

    cmd.Flags().StringVar(&outputDir, "output", "", "Output directory")
    cmd.Flags().StringVar(&fromGenerator, "from-generator", "", "Build from KurelPackage YAML")  // NEW

    return cmd
}
```

### 2. Implement buildFromGenerator Function

**File:** `pkg/cmd/kurel/build.go` (new file or add to cmd.go)

```go
func buildFromGenerator(generatorFile, outputDir string, opts *LauncherOptions) error {
    // 1. Read KurelPackage YAML
    data, err := os.ReadFile(generatorFile)
    if err != nil {
        return fmt.Errorf("failed to read generator file: %w", err)
    }

    // 2. Parse YAML and detect GVK
    wrapper := &stack.ApplicationWrapper{}
    if err := yaml.Unmarshal(data, wrapper); err != nil {
        return fmt.Errorf("failed to parse YAML: %w", err)
    }

    // 3. Verify it's a KurelPackage
    if wrapper.Kind != "KurelPackage" {
        return fmt.Errorf("expected KurelPackage, got %s", wrapper.Kind)
    }

    // 4. Load generator config
    config, err := generators.Create(wrapper.APIVersion, wrapper.Kind)
    if err != nil {
        return fmt.Errorf("failed to create generator: %w", err)
    }

    kurelPkgConfig := config.(*kurelpackage.Config)
    if err := yaml.Unmarshal(data, kurelPkgConfig); err != nil {
        return fmt.Errorf("failed to unmarshal config: %w", err)
    }

    // 5. Generate package structure
    pkgDef, err := kurelPkgConfig.GeneratePackageFiles()
    if err != nil {
        return fmt.Errorf("failed to generate package: %w", err)
    }

    // 6. Write to output directory
    if err := writePackageToDir(pkgDef, outputDir); err != nil {
        return fmt.Errorf("failed to write package: %w", err)
    }

    opts.Logger.Info("Package generated successfully", "output", outputDir)
    return nil
}
```

### 3. Implement writePackageToDir Function

```go
func writePackageToDir(pkg *launcher.PackageDefinition, outputDir string) error {
    // Create directory structure
    dirs := []string{
        filepath.Join(outputDir, "resources"),
        filepath.Join(outputDir, "patches"),
    }

    for _, dir := range dirs {
        if err := os.MkdirAll(dir, 0755); err != nil {
            return err
        }
    }

    // Write kurel.yaml
    kurelData, err := yaml.Marshal(pkg.Metadata)
    if err != nil {
        return err
    }
    if err := os.WriteFile(filepath.Join(outputDir, "kurel.yaml"), kurelData, 0644); err != nil {
        return err
    }

    // Write parameters.yaml
    paramsData, err := yaml.Marshal(pkg.Parameters)
    if err != nil {
        return err
    }
    if err := os.WriteFile(filepath.Join(outputDir, "parameters.yaml"), paramsData, 0644); err != nil {
        return err
    }

    // Write resources
    for i, resource := range pkg.Resources {
        resourceData, err := yaml.Marshal(resource)
        if err != nil {
            return err
        }

        filename := fmt.Sprintf("resource-%d.yaml", i)
        if resource.GetObjectMeta().GetName() != "" {
            filename = fmt.Sprintf("%s.yaml", resource.GetObjectMeta().GetName())
        }

        path := filepath.Join(outputDir, "resources", filename)
        if err := os.WriteFile(path, resourceData, 0644); err != nil {
            return err
        }
    }

    // Write patches
    for i, patch := range pkg.Patches {
        patchData := []byte(patch.Content)
        filename := fmt.Sprintf("patch-%d.kpatch", i)

        path := filepath.Join(outputDir, "patches", filename)
        if err := os.WriteFile(path, patchData, 0644); err != nil {
            return err
        }
    }

    return nil
}
```

### 4. Update Command Help

**File:** `pkg/cmd/kurel/cmd.go`

```go
cmd := &cobra.Command{
    Use:   "build [package-dir|--from-generator FILE]",
    Short: "Build a kurel package",
    Long: `Build a kurel package from a package directory or KurelPackage YAML.

Examples:
  # Build from existing package directory
  kurel build ./my-package

  # Generate package from KurelPackage YAML
  kurel build --from-generator my-app.yaml --output ./my-package
`,
    // ...
}
```

## Testing Requirements

### 1. Unit Tests

```go
func TestBuildFromGenerator(t *testing.T) {
    // Create temporary generator YAML
    tmpFile := filepath.Join(t.TempDir(), "test.yaml")
    yaml := `
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: test-package
version: v1.0.0
# ... complete config
`
    os.WriteFile(tmpFile, []byte(yaml), 0644)

    // Build package
    outputDir := t.TempDir()
    err := buildFromGenerator(tmpFile, outputDir, opts)
    if err != nil {
        t.Fatalf("build failed: %v", err)
    }

    // Verify structure
    assertFileExists(t, filepath.Join(outputDir, "kurel.yaml"))
    assertFileExists(t, filepath.Join(outputDir, "parameters.yaml"))
    assertFileExists(t, filepath.Join(outputDir, "resources"))
}
```

### 2. Integration Test

Create example in `examples/generators/kurelpackage.yaml` and test full workflow:

```bash
go run ./cmd/kurel build --from-generator examples/generators/kurelpackage.yaml --output /tmp/test-pkg
```

## Files to Modify

1. `pkg/cmd/kurel/cmd.go` - Add `--from-generator` flag
2. `pkg/cmd/kurel/build.go` - Implement `buildFromGenerator`, `writePackageToDir`
3. `pkg/cmd/kurel/cmd_test.go` - Add tests
4. `examples/generators/kurelpackage.yaml` - Add example

## Success Criteria

- [ ] `kurel build --from-generator` flag implemented
- [ ] Can build package from KurelPackage YAML
- [ ] Output directory structure correct
- [ ] All files (kurel.yaml, parameters.yaml, resources/, patches/) generated
- [ ] Unit tests passing
- [ ] Integration test passing
- [ ] Example YAML in examples/ directory
- [ ] Help text updated

## Example Usage

```bash
# Create KurelPackage YAML
cat > nginx-app.yaml <<EOF
apiVersion: generators.gokure.dev/v1alpha1
kind: KurelPackage
metadata:
  name: nginx-app
version: v1.0.0
description: Simple nginx application
parameters:
  replicas:
    type: integer
    default: 3
    description: Number of replicas
  image:
    type: string
    default: nginx:latest
    description: Container image
resources:
  - apiVersion: kure.dev/v1alpha1
    kind: AppWorkload
    metadata:
      name: nginx
    spec:
      workload: Deployment
      containers:
        - name: nginx
          image: \${values.image}
EOF

# Generate package
kurel build --from-generator nginx-app.yaml --output ./nginx-package

# Use package
kurel build ./nginx-package --output ./manifests
```

## References

- Existing build command: `pkg/cmd/kurel/cmd.go`
- Generator registry: `pkg/stack/generators/registry.go`
- Package structure: `pkg/launcher/types.go`
- Example package: `examples/kurel/frigate/`

## Estimated Effort

**2-3 days** (depends on kurel-generator-mvp-1-high.md completion)
