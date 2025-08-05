# Kurel Package System — Design Specification

This document describes the structure, behavior, and purpose of a **Kurel package** (Kubernetes Resources Launcher), which encapsulates a reusable, versionable application for Kubernetes. Kurel builds on Kure's patch engine to enable declarative configuration of application instances without templates or overlays.

---

## Goals

- Enable reusable packaging of Kubernetes applications
- Declarative customization via parameters + patches
- No Helm-style templating or Kustomize overlays
- Strong schema validation and deployment safety
- Compatible with GitOps workflows (Flux/ArgoCD)
- Multi-namespace and multi-phase deployment support

---

## Design Philosophy

**"Kurel just generates YAML"** - Kurel is a declarative system for generating Kubernetes manifests with validation and customization capabilities. It is not a runtime system, orchestrator, or complex package manager.

### Core Principles
- **Explicit over Implicit** - Always prefer explicit configuration
- **Flexible but Validated** - Maximum flexibility with comprehensive validation
- **GitOps Compatible** - Generate proper Kubernetes manifests for GitOps workflows
- **No Templating** - Use patches instead of complex template logic
- **Deterministic Output** - Same inputs always produce same outputs

---

## Package Directory Structure

A Kurel package is a directory containing Kubernetes resources, parameters, patches, and metadata.

```
my-app.kurel/
├── parameters.yaml          # All variables and package metadata
├── resources/               # Base Kubernetes manifests (one GVK per file)
│   ├── deployment.yaml
│   ├── service.yaml
│   └── namespaces.yaml
├── patches/                 # Modular patches with conditional enabling
│   ├── 00-base.kpatch      # Global patterns (explicit)
│   ├── features/
│   │   ├── 10-monitoring.kpatch
│   │   ├── 10-monitoring.yaml   # Patch metadata
│   │   └── 20-ingress.kpatch
│   └── profiles/
│       ├── 10-development.kpatch
│       └── 10-production.kpatch
├── schemas/                 # Auto-generated validation schemas
│   └── parameters.schema.json
├── examples/               # Example parameter configurations
│   └── production.yaml
└── README.md              # Package documentation

my-app.local.kurel/         # User extensions (optional)
├── patches/                # Additional user patches
│   └── 50-custom.kpatch
└── parameters.yaml         # Parameter overrides
```

---

## Core Concepts

### 1. **parameters.yaml** - Variables and Metadata

Contains all package metadata and configurable variables with a structured hierarchy:

```yaml
# Package metadata (fixed key)
kurel:
  name: prometheus-operator
  version: 0.68.0
  appVersion: 0.68.0
  description: "Prometheus Operator for monitoring"
  home: https://github.com/prometheus-operator/prometheus-operator

# Global defaults (fixed key) 
global:
  labels:
    app.kubernetes.io/name: "${kurel.name}"
    app.kubernetes.io/managed-by: "kurel"
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 1000m
      memory: 1Gi

# Feature configuration (author-defined)
monitoring:
  enabled: false
  retention: 30d

persistence:
  enabled: true
  size: 10Gi
```

### 2. **resources/** - Base Kubernetes Manifests

Contains the base Kubernetes resources, one GVK per file. These are standard Kubernetes YAML files without templating.

### 3. **patches/** - Modular Patch System

Multiple patch files organized in subdirectories with numeric ordering:

```toml
# patches/features/10-monitoring.kpatch
[deployment.prometheus.spec.template.spec]
securityContext.runAsNonRoot: true

[deployment.prometheus.spec.template.spec.containers.0]
resources: "${global.resources}"
image.tag: "${kurel.appVersion}"
```

Each patch can have corresponding metadata:

```yaml
# patches/features/10-monitoring.yaml
enabled: "${monitoring.enabled}"
description: "Adds Prometheus monitoring capabilities"
requires:
  - "features/05-metrics-base.kpatch"
conflicts:
  - "features/20-lightweight-monitoring.kpatch"
```

### 4. **GitOps Deployment Phases**

Resources can be annotated for ordered deployment:

```yaml
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring
  annotations:
    kurel.gokure.dev/install-phase: "pre-install"
    kurel.gokure.dev/wait-for-ready: "true"
```

**Phases:**
- `pre-install` - CRDs, namespaces, RBAC
- `main` - Primary application resources (default)
- `post-install` - Monitoring, backups, optional components

### 5. **Schema Generation and Validation**

Schemas are auto-generated from parameters and Kubernetes API tracing:

```bash
kurel schema generate my-app.kurel/
# Generates schemas/parameters.schema.json
```

Validation includes:
- Parameter value validation against schema
- Variable reference validation in patches
- Kubernetes resource validation
- Patch dependency and conflict checking

### 6. **User Extensions (.local.kurel)**

Users can extend packages without modifying the original:

```
my-app.local.kurel/
├── patches/50-custom.kpatch    # Additional patches
└── parameters.yaml             # Parameter overrides
```

---

## Variable System

### Variable Reference Syntax
- **Dot notation**: `${section.subsection.value}`
- **Nested references**: `${monitoring.serviceMonitor.enabled}` can reference `${monitoring.enabled}`
- **Metadata access**: `${kurel.appVersion}` for package metadata

### Variable Resolution
1. Load package `parameters.yaml`
2. Override with local `my-app.local.kurel/parameters.yaml`
3. Resolve all variable references
4. Apply type validation and casting

### Fixed Top-Level Keys
- **`kurel:`** - Package metadata (name, version, description, etc.)
- **`global:`** - Default values applied across resources via base patches
- **Everything else** - Author-defined variable hierarchy

---

## Patch System

### Patch Discovery and Ordering
- **Discovery**: `patches/**/*.kpatch`
- **Ordering**: Numeric prefixes (`10-`, `20-`) then alphabetical
- **Organization**: Subdirectories preferred for better organization

### Conditional Patch Enabling
- **Simple boolean expressions**: `enabled: "${monitoring.enabled}"`
- **Auto-enable dependencies**: `requires:` automatically enables referenced patches
- **Conflict detection**: `conflicts:` prevents incompatible patches

### Patch Processing Flow
1. Discover all patch files
2. Load metadata and evaluate conditions
3. Build dependency graph and auto-enable required patches
4. Validate no conflicts exist
5. Apply patches in dependency order

---

## Multi-Namespace Support

### Flexible Namespace Handling
- **Full flexibility** - Resources can target any namespaces
- **Namespace creation control** - `global.namespaces.create` flag
- **Cross-namespace references** - Supported and validated

### Namespace Creation Pattern
```yaml
# parameters.yaml
global:
  namespaces:
    create: true
    exclude: ["kube-system", "default"]
```

```toml
# patches/00-base.kpatch
[namespace.*]
enabled: "${global.namespaces.create}"
metadata.labels: "${global.labels}"
```

---

## Build and Validation

### CLI Commands
```bash
# Validate package and parameters
kurel validate my-app.kurel/ --values custom.yaml

# Generate schemas from package
kurel schema generate my-app.kurel/

# Build final manifests
kurel build my-app.kurel/ --values custom.yaml --output ./manifests/

# Show package information
kurel info my-app.kurel/
```

### Generated Output Structure
```
output/
├── pre-install/          # Phase 1 resources
│   ├── kustomization.yaml
│   └── namespaces.yaml
├── main/                 # Phase 2 resources (depends on pre-install)
│   ├── kustomization.yaml
│   ├── deployments.yaml
│   └── services.yaml
└── post-install/         # Phase 3 resources (depends on main)
    ├── kustomization.yaml
    └── monitoring.yaml
```

---

## Design Constraints

- ❌ No templating or embedded logic in YAML
- ❌ No overlays or merging strategies (use patches)
- ❌ No conditionals or loops in YAML
- ❌ No complex package dependencies (handled at GitOps level)
- ✅ Variable substitution allowed for keys in parameters.yaml
- ✅ All patches are deterministic, declarative, and validated
- ✅ Multi-namespace and multi-phase deployments supported

---

## Use Cases

- **Application Packaging** - Reusable packages for common applications
- **Platform Engineering** - Standardized app bundles with customization boundaries
- **GitOps Deployments** - Generate manifests compatible with Flux/ArgoCD
- **Multi-Environment** - Same package deployed with different configurations
- **Complex Applications** - Multi-namespace apps with ordered deployment phases

---

## Future Extensions

- **Enhanced Schema Generation** - Better Kubernetes API tracing and CRD support
- **Package Registry** - Central repository for sharing kurel packages
- **Advanced Validation** - Integration with policy engines and security scanners
- **IDE Integration** - Language servers and editor support
- **Testing Framework** - Unit and integration testing for packages