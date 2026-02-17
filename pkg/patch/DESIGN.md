# Kure Patch File Format — Specification

This document describes the complete structure and semantics of `.kpatch` files used in Kure to define Kubernetes resource overrides.

Kure patches are:

- Flat, line-based
- TOML-inspired, but **not** valid TOML
- Declarative (no conditionals or logic)
- Scoped to specific resource kinds and instances

---

## 1. File Extension

All patch files must use the `.kpatch` extension. These are plain text files with Kure's custom patch format.

Examples:

- `deployment.app.kpatch`
- `service.backend.kpatch`
- `config.kpatch` (merged aggregate)

---

## 2. Patch Header Syntax

Each patch file is divided into sections. Each section is introduced by a **header**:

```toml
[kind.name]
[kind.name.section]
[kind.name.section.key=value]
[kind.name.section.index]
```

### 2.1 Header Grammar

```
[kind.name[.section[.subsection[.selector or .index]]]]
```

#### Simplified Selector Rule

List selectors using `key=value` pairs can omit brackets unless the key or value contains special characters (e.g., `.`, `=`, `[`, `]`, `+`, `-`).

Preferred syntax:

```toml
[deployment.app.containers.name=main]
```

Bracketed fallback (required when ambiguous):

```toml
[deployment.app.containers[image.name=main]]
```

### 2.2 Examples

```toml
[deployment.app]                           # Top-level fields
[deployment.app.containers]               # Applies to all containers
[deployment.app.containers.name=main]     # Replace item with name == "main"
[deployment.app.ports.0]                  # First port entry
```

---

## 3. Patch Keys

Within each header block, individual settings are expressed as **dotpaths**, referencing fields to override.

Syntax:

```toml
key.subkey.subsubkey: value
```

### 3.1 Values

- Must be scalar (string, int, float, boolean)
- Strings may be quoted or unquoted (if YAML-compliant)
- Variables are allowed (see below)

### 3.2 Examples

```toml
replicas: 3
image.repository: ghcr.io/example/myapp
resources.limits.cpu: 500m
host: "${values.domain}"
```

---

## 4. Variables

Kure patch files support scalar substitution using instance-level variables.

### Syntax

```toml
${features.myflag}
${values.domain}
```

### Scope

- `features.*`: booleans provided programmatically
- `values.*`: strings or numbers provided programmatically

Variables must resolve to scalars. No objects or arrays allowed.

### Example

```toml
[deployment.app]
enabled: ${features.web_enabled}
replicas: 2

[service.app]
hostname: "${values.name}.${values.domain}"
```

---

## 5. Lists and Selectors

Kure supports patching into Kubernetes lists like `containers`, `env`, `ports`, `volumes`, `volumeMounts`, etc.

### 5.1 List Selector Syntax

List selectors allow addressing or inserting elements within Kubernetes lists.

| Selector Type       | Example                                     | Meaning                                    |
| ------------------- | ------------------------------------------- | ------------------------------------------ |
| By index            | `spec.containers[0]` / `spec.containers.0`  | Replace at index 0                         |
| By key-value        | `spec.containers[name=web]` / `...name=web` | Replace item with `name=web`               |
| Insert before index | `spec.containers[-3]`                       | Insert before index 3                      |
| Insert before match | `spec.containers[-name=sidecar]`            | Insert before item matching `name=sidecar` |
| Insert after index  | `spec.containers[+2]`                       | Insert after index 2                       |
| Insert after match  | `spec.containers[+name=main]`               | Insert after item matching `name=main`     |
| Append to list      | `spec.containers[-]`                        | Append item to end of list                 |

Note: You may omit brackets around `key=value` unless the key or value contains special characters (e.g. `.`, `[`, `]`).

---

## 6. Implementation Status

### Supported Features
- Dual format support (YAML and TOML) with automatic detection
- Advanced structure preservation maintaining comments and formatting
- Intelligent path resolution with disambiguation
- Variable substitution with `${values.key}` and `${features.flag}` syntax
- Automatic type inference for Kubernetes compatibility
- Comprehensive debug logging with `KURE_DEBUG=1`
- Graceful error handling with warnings for missing targets
- Strategic merge patch (SMP) for deep-merging partial YAML documents
- JSON merge patch fallback (RFC 7386) for CRDs without Go struct tags
- SMP conflict detection for overlapping patches

### Current Limitations
- No logic, conditionals, or templating expressions
- Field-level patches only support scalar values (arrays/objects not allowed)
- ✅ Pure index-based insertion (`[-3]`, `[+2]`) now implemented
- Variable context must be provided programmatically
- No OpenAPI schema validation (planned for future implementation)
- SMP is YAML-only (not available in TOML format)
- CRD list merging uses replace semantics (no merge-by-key without struct tags)

### Future Enhancements
- OpenAPI schema validation for patch target verification

---

## 7. Strategic Merge Patch

Strategic merge patch (SMP) is a Kubernetes-native patching strategy where a partial YAML document is deep-merged into a target resource. Unlike JSON merge patch, SMP is schema-aware: it knows that `spec.containers` should be merged by `name`, not replaced entirely.

### 7.1 YAML Syntax

SMP patches use `type: strategic` in the targeted patch list format:

```yaml
# Field-level patches (unchanged)
- target: deployment.my-app
  patch:
    spec.replicas: 3

# Strategic merge patch
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
          - name: sidecar
            image: envoy:v1.28
```

### 7.2 Behavior

- **Known Kubernetes kinds** (Deployment, Service, etc.): Uses `StrategicMergeMapPatch` with Go struct tags for list-merge-by-key semantics.
- **Unknown kinds** (CRDs): Falls back to RFC 7386 JSON merge patch. Lists are replaced, not merged.
- **Application order**: SMP patches are applied before field-level patches. SMP sets the broad document shape; field patches make precise tweaks on top.

### 7.3 Conflict Detection

When multiple SMP patches target the same resource, conflicts can be detected before application:

```go
resolved, reports, err := patchSet.ResolveWithConflictCheck()
for _, r := range reports {
    if r.HasConflicts() {
        // handle conflict
    }
}
```

### 7.4 Limitations

- SMP is YAML-only — TOML format does not support strategic merge patches.
- CRD fallback loses list merge semantics. Use field-level patches for precise CRD list manipulation.
- Patch maps are deep-copied before application to prevent mutation.

---

## 8. Purpose

Kure patches are designed to:

- Override Kubernetes manifests without templates
- Enable reusable, modular package definitions
- Support clean schema validation via OpenAPI
- Allow editing via CLI and JSONSchema-aware UIs

---

## 9. Example

```toml
[deployment.app]
replicas: 3

[deployment.app.containers.name=main]
image.repository: ghcr.io/example/app
image.tag: "${values.version}"
resources.requests.cpu: 250m

[service.app.ports.name=http]
port: 80

[ingress.web.tls.0]
hosts.0: "${values.name}.${values.domain}"
```

This file:

- Updates the replica count
- Modifies the main container image and CPU request
- Sets the service port
- Configures the first TLS entry of the ingress

---

This format is the foundation for declarative, schema-validated Kubernetes customization in Kure.

