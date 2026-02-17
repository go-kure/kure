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

## Strategic Merge Patch

For broad document-level changes, use strategic merge patch (SMP). Instead of targeting individual fields, SMP deep-merges a partial YAML document into the target resource.

### How It Works

Known Kubernetes kinds (Deployment, Service, etc.) are merged using struct tags — lists like `containers` are merged by `name`, not replaced. Unknown kinds (CRDs) fall back to JSON merge patch (RFC 7386), where lists are replaced entirely.

### YAML Syntax

```yaml
# Add a sidecar and update the main container's resources
- target: deployment.my-app
  type: strategic
  patch:
    spec:
      template:
        spec:
          containers:
          - name: main
            resources:
              limits:
                cpu: "500m"
                memory: "256Mi"
          - name: sidecar
            image: envoy:v1.28
```

### Mixing with Field-Level Patches

SMP and field-level patches can coexist in the same file. SMP patches are applied first (setting the document shape), then field-level patches make precise tweaks:

```yaml
# Strategic merge: add a sidecar container
- target: deployment.my-app
  type: strategic
  patch:
    spec:
      template:
        spec:
          containers:
          - name: sidecar
            image: envoy:v1.28

# Field-level: set replica count precisely
- target: deployment.my-app
  patch:
    spec.replicas: 3
```

### Enabling Kind-Aware Merging

```go
import "github.com/go-kure/kure/pkg/patch"

// Create a kind lookup for schema-aware merging
lookup, err := patch.DefaultKindLookup()
patchSet.KindLookup = lookup
```

### Conflict Detection

When multiple SMP patches target the same resource, check for conflicts:

```go
resolved, reports, err := patchSet.ResolveWithConflictCheck()
for _, r := range reports {
    if r.HasConflicts() {
        for _, c := range r.Conflicts {
            log.Printf("conflict on %s: %s", r.ResourceName, c.Description)
        }
    }
}
```

## Further Reading

- [Patch reference](/api-reference/patch) for API details
- [Patch examples](/examples/patches) for working samples
- [Kurel packages](kurel-packages) for patch-based package customization
