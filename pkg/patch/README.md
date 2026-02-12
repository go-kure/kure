# Patch - Declarative Resource Patching

The `patch` package provides a JSONPath-based system for declaratively modifying Kubernetes resources. It supports both TOML and YAML patch file formats with structure-preserving modifications and variable substitution.

## Overview

Patches allow you to modify Kubernetes manifests without rewriting them. The system uses JSONPath expressions to target specific fields and applies changes while preserving the original YAML structure (comments, ordering, formatting).

## Patch File Formats

### TOML Format (`.kpatch`)

```toml
# Target a specific resource by kind and name
[deployment.myapp.spec]
replicas = 3

[deployment.myapp.spec.template.spec.containers.0]
image = "${app.image}:${app.tag}"
resources.requests.cpu = "200m"
resources.requests.memory = "256Mi"

# Append to a list with .-
[deployment.myapp.spec.template.spec.containers.-]
name = "sidecar"
image = "envoy:latest"
```

### YAML Format

```yaml
target:
  kind: Deployment
  name: myapp
patches:
  - path: spec.replicas
    value: 3
  - path: spec.template.spec.containers[0].image
    value: "nginx:latest"
```

## Quick Start

```go
import "github.com/go-kure/kure/pkg/patch"

// Load patches from file
file, _ := os.Open("patches/customize.kpatch")
specs, err := patch.LoadPatchFile(file)

// Create patchable set with resources and patches
patchSet, err := patch.NewPatchableAppSet(resources, specs)

// Resolve targets and apply
resolved, err := patchSet.Resolve()
for _, r := range resolved {
    err := r.Apply()
}

// Write patched output
err = patchSet.WriteToFile("output.yaml")
```

## Key Features

### Variable Substitution

Patches support variable references that resolve against a parameter context:

```toml
[deployment.myapp.spec.template.spec.containers.0]
image = "${registry}/${image}:${tag}"
replicas = "${replicas}"
```

```go
varCtx := &patch.VariableContext{
    Variables: map[string]interface{}{
        "registry": "docker.io",
        "image":    "myapp",
        "tag":      "v1.0.0",
        "replicas": 3,
    },
}
specs, err := patch.LoadPatchFileWithVariables(file, varCtx)
```

### List Selectors

Target specific items in lists using selectors:

```toml
# By index
[deployment.myapp.spec.template.spec.containers.0]
image = "updated:latest"

# Append to list
[deployment.myapp.spec.template.spec.containers.-]
name = "new-container"

# By field value (name selector)
[deployment.myapp.spec.template.spec.containers.{name=myapp}]
image = "updated:latest"
```

### Structure Preservation

When using `NewPatchableAppSetWithStructure`, the original YAML document structure is preserved through patching, maintaining comments, key ordering, and formatting.

## API Reference

### Loading Patches

| Function | Description |
|----------|-------------|
| `LoadPatchFile(r)` | Load with automatic format detection |
| `LoadPatchFileWithVariables(r, ctx)` | Load with variable substitution |
| `LoadTOMLPatchFile(r, ctx)` | Load TOML-format patches |
| `LoadYAMLPatchFile(r, ctx)` | Load YAML-format patches |

### Applying Patches

| Function | Description |
|----------|-------------|
| `NewPatchableAppSet(resources, patches)` | Create patchable set |
| `NewPatchableAppSetWithStructure(docSet, patches)` | Create with structure preservation |
| `ParsePatchLine(path, value)` | Parse a single patch operation |

## Related Packages

- [launcher](../launcher/) - Uses patches in kurel package system
- [io](../io/) - YAML parsing for patch targets
