+++
title = "Patching Resources"
weight = 40
+++

# Patching Resources

Kure's patch system lets you declaratively modify Kubernetes resources using JSONPath expressions. Patches are applied after resource generation, making them useful for environment-specific customization.

## When to Patch vs Configure

- **Configure at generation time** when you control the resource builder (set replicas, image, etc. in code)
- **Patch after generation** when you need to modify resources from external sources, or when the same base resources need different modifications per environment

## Patch File Formats

### TOML Format (`.kpatch`)

The TOML format uses section headers to target resources:

```toml
# [kind.name.path.to.field]
[deployment.myapp.spec]
replicas = 3

[deployment.myapp.spec.template.spec.containers.0]
image = "nginx:1.25"
resources.requests.cpu = "200m"
resources.requests.memory = "256Mi"
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
    value: "nginx:1.25"
```

## Applying Patches

```go
import "github.com/go-kure/kure/pkg/patch"

// Load patches from file
file, _ := os.Open("patches/production.kpatch")
specs, err := patch.LoadPatchFile(file)

// Create patchable set
patchSet, err := patch.NewPatchableAppSet(resources, specs)

// Resolve and apply
resolved, err := patchSet.Resolve()
for _, r := range resolved {
    err := r.Apply()
}

// Write output
err = patchSet.WriteToFile("patched-output.yaml")
```

## Variable Substitution

Patches can reference variables:

```toml
[deployment.myapp.spec.template.spec.containers.0]
image = "${registry}/${image}:${tag}"
```

```go
varCtx := &patch.VariableContext{
    Variables: map[string]interface{}{
        "registry": "docker.io",
        "image":    "myapp",
        "tag":      "v2.0.0",
    },
}
specs, err := patch.LoadPatchFileWithVariables(file, varCtx)
```

## List Operations

### Target by index

```toml
[deployment.myapp.spec.template.spec.containers.0]
image = "updated:latest"
```

### Append to list

```toml
[deployment.myapp.spec.template.spec.containers.-]
name = "sidecar"
image = "envoy:latest"
```

### Target by field selector

```toml
[deployment.myapp.spec.template.spec.containers.{name=myapp}]
image = "updated:latest"
```

## Further Reading

- [Patch reference](/api-reference/patch) for API details
- [Patch examples](/examples/patches) for working samples
- [Kurel packages](kurel-packages) for patch-based package customization
