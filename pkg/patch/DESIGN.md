# Kure Patch File Format — Specification

This document describes the complete structure and semantics of `.patch` files used in Kure to define Kubernetes resource overrides.

Kure patches are:

- Flat, line-based
- TOML-inspired, but **not** valid TOML
- Declarative (no conditionals or logic)
- Scoped to specific resource kinds and instances

---

## 1. File Extension

All patch files must use the `.patch` extension. These are plain text files.

Examples:

- `deployment.app.patch`
- `service.backend.patch`
- `config.patch` (merged aggregate)

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

### Current Limitations
- No logic, conditionals, or templating expressions
- No map merging — field values are completely replaced
- Only scalar values supported (arrays/objects not allowed in patch values)
- ✅ Pure index-based insertion (`[-3]`, `[+2]`) now implemented
- Variable context must be provided programmatically
- No OpenAPI schema validation (planned for future implementation)

### Future Enhancements
- OpenAPI schema validation for patch target verification

---

## 7. Purpose

Kure patches are designed to:

- Override Kubernetes manifests without templates
- Enable reusable, modular package definitions
- Support clean schema validation via OpenAPI
- Allow editing via CLI and JSONSchema-aware UIs

---

## 8. Example

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

