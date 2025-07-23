# Kur8 Package System — Design Specification

This document describes the structure, behavior, and purpose of a **Kur8 package**, which encapsulates a reusable, versionable application for Kubernetes. Kur8 builds on Kure’s patch engine to enable declarative configuration of application instances without templates or overlays.

---

## Goals

- Enable reusable packaging of Kubernetes applications
- Declarative customization via config + patch
- No Helm-style templating or Kustomize overlays
- Strong schema validation and upgrade safety
- Compatible with GitOps/Flux-based deployments

---

## Package Directory Structure

A Kur8 package is a folder containing Kubernetes resources, metadata, and default configuration.

```
my-app.kur8/
├── resources/               # Base Kubernetes manifests (one per file)
│   ├── deployment.yaml
│   ├── service.yaml
│   └── ingress.yaml
├── parameters.patch         # Default patch set for a single resource (optional)
├── config.patch             # Multi-resource patch set (optional)
├── config.schema.json       # Generated JSONSchema for validation
├── instance.schema.json     # Schema for instance-level fields
├── instance.yaml            # Instance-level metadata (external to package)
└── README.md                # Optional documentation
```

---

## Core Concepts

### 1. **resources/**

Contains the base Kubernetes resources, one per file. These are raw YAMLs defining Deployments, Services, etc. Patches will apply on top of these.

### 2. **parameters.patch**

An optional file containing default patches for one resource. This uses Kure's single-line patch syntax, one `path: value` entry per line.

```yaml
spec.replicas: 2
spec.template.spec.containers[0].image: nginx:latest
```

### 3. **config.patch**

A structured patch file containing multiple resource-specific patch entries using `target:` blocks.

```yaml
- target: app-deployment
  patch:
    spec.replicas: 3
- target: app-service
  patch:
    spec.ports[-]: { port: 443, name: https }
```

Patches use Kure’s validated path syntax and support operations including `replace`, `delete`, `insertbefore`, `insertafter`, and `append`, depending on the structure of the key path.

### 4. **config.schema.json**

A JSONSchema file generated from `parameters.patch`, describing the valid structure of configurable fields. Used to validate user-supplied values.

### 5. **instance.schema.json**

Schema for validating high-level metadata, feature flags, or required values for the consuming environment.

### 6. **instance.yaml** (external file)

Defines the instance-specific deployment configuration:

```yaml
package: nginx.kur8
name: my-nginx
values:
  replicas: 2
  domain: mysite.example.com
```

This file is external to the package and never versioned alongside it. Values from this file may be referenced within patch files using simple variable substitution — but only for variables declared in `instance.yaml` under `values:`.

For example:

```yaml
metadata.name: ${values.name}
spec.replicas: ${values.replicas}
```

This substitution is limited, explicit, and always validated.

---

## Patching Model

Kur8 patches are line-based YAML fragments:

```yaml
spec.replicas: 3
spec.template.spec.containers[name=web].resources.limits.cpu: "500m"
spec.ports[-]: { port: 80, name: http }
```

Kure supports both map field and list field operations:

| Operation       | Syntax Example                                 | Description                                       |
| --------------- | ---------------------------------------------- | ------------------------------------------------- |
| Replace         | `spec.replicas: 3`                             | Replace scalar or map at path                     |
| Insert Before   | `spec.containers[-2]: {...}`                   | Insert before item at index 2                     |
| Insert After    | `spec.containers[+name=main]: {...}`           | Insert after matching list item                   |
| Append          | `spec.volumes[-]: {...}`                       | Append item to list                               |
| Delete (future) | Reserved keyword or marker (e.g. `__delete__`) | Remove a field or list item (not yet implemented) |

---

## Customization Flow

1. Author defines `resources/` and optional `parameters.patch`
2. Schema is generated from patch inputs
3. End-user provides `instance.yaml` and optionally overrides patches
4. Kur8 applies patches on base resources
5. Output includes both plain Kubernetes YAML and Flux-compatible manifests

---

## Design Constraints

- ❌ No templating or embedded logic
- ❌ No overlays or merging strategies
- ❌ No conditionals or loops in YAML
- ❌ No composition or shared libraries
- ✅ Variable substitution is allowed, but only for keys in `instance.yaml.values`
- ✅ All patches are deterministic, declarative, and validated

---

## Use Cases

- Application packages with well-defined customization boundaries
- Platform-authored app bundles with schema and constraints
- Declarative GitOps-friendly deployments with no templating engines

---

## Future Extensions

- Schema-driven UI support
- Per-package metadata and capabilities
- Optional deletion support in patches
- Ecosystem of signed, versioned packages for clusters

