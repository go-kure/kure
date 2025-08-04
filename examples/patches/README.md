# Kure Patch Module Examples (TOML Format)

This directory demonstrates the new TOML-style patch system with cert-manager as an example.

## Files

- **`cert-manager-simple.yaml`** - Base cert-manager resources (simplified)
- **`resources.patch`** - Resource limits using TOML container selectors
- **`ingress.patch`** - Service configuration with port selectors
- **`security.patch`** - Security contexts with deployment targeting
- **`advanced.patch`** - Complex selectors and variable substitution

## Running the Demo

```bash
go run ./cmd/demo -patches
```

## TOML Patch Format

The patch files now use TOML-style headers for precise resource targeting:

```toml
# Basic resource targeting
[deployment.app]
spec.replicas: 3
metadata.labels.env: production

# Container-specific patches
[deployment.app.containers.name=main]
resources.requests.cpu: 100m
resources.limits.memory: 512Mi

# Service port configuration
[service.app.ports.name=https]
port: 443
nodePort: 30443

# Array index targeting
[ingress.web.rules.0.paths.0]
path: /api
pathType: Prefix
```

## Header Grammar

The TOML header format follows this grammar:
```
[kind.name[.section[.subsection[.selector]]]]
```

### Selector Types

1. **Key-value selectors**: `containers.name=main`
2. **Index selectors**: `ports.0`, `rules.1`
3. **Bracketed selectors**: `containers[image=nginx]`

### Kubernetes Path Mapping

The system intelligently maps TOML sections to Kubernetes paths:

| TOML Section | Kubernetes Path (Deployment) | Kubernetes Path (Service) |
|--------------|------------------------------|---------------------------|
| `containers` | `spec.template.spec.containers` | `spec.containers` |
| `ports` | `spec.template.spec.containers.ports` | `spec.ports` |
| `volumes` | `spec.template.spec.volumes` | `spec.volumes` |
| `env` | `spec.template.spec.containers.env` | N/A |

## Variable Substitution

Support for dynamic values using variable substitution:

```toml
[deployment.app.containers.name=main]
image.tag: "${values.version}"
resources.requests.cpu: "${values.cpu_request}"
debug.enabled: "${features.enable_debug}"
```

Variable context:
```go
&patch.VariableContext{
    Values: map[string]interface{}{
        "version": "1.20",
        "cpu_request": "100m",
    },
    Features: map[string]bool{
        "enable_debug": true,
    },
}
```

## Examples by Complexity

### Basic Resource Targeting
```toml
[deployment.cert-manager]
spec.replicas: 3
metadata.labels.environment: production
```

### Container-Specific Configuration
```toml
[deployment.cert-manager.containers.name=cert-manager-controller]
resources.requests.cpu: 100m
resources.limits.memory: 512Mi
securityContext.readOnlyRootFilesystem: true
```

### Service Configuration
```toml
[service.cert-manager-webhook.ports.name=https]
port: 9443
nodePort: 30443
```

### Complex Array Manipulation
```toml
# Add new environment variable
[deployment.app.containers.name=main.env[+]]
name: DEBUG_MODE
value: "true"

# Add new volume mount
[deployment.app.containers.name=main.volumeMounts[+]]
name: config
mountPath: /etc/config
readOnly: true
```

## Key Features

1. **Intelligent Path Resolution** - Automatic mapping based on resource kind
2. **Precise Targeting** - Container-specific, port-specific, rule-specific patches
3. **Variable Substitution** - Dynamic values with `${values.key}` syntax
4. **Complex Selectors** - Multiple ways to target list items
5. **Backward Compatibility** - Still supports legacy YAML format
6. **Context Awareness** - Different behavior for different resource types

## Migration from YAML

Old YAML format:
```yaml
- target: cert-manager
  patch:
    spec.template.spec.containers[0].resources.requests.cpu: "100m"
```

New TOML format:
```toml
[deployment.cert-manager.containers.0]
resources.requests.cpu: 100m
```

Or with semantic selector:
```toml
[deployment.cert-manager.containers.name=cert-manager-controller]
resources.requests.cpu: 100m
```

The TOML format provides better readability, more precise targeting, and eliminates the need for long JSONPath expressions.