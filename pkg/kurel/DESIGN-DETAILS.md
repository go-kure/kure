# Kurel Package System - Comprehensive Design Details

This document captures the complete design discussion, decisions, alternatives considered, and rationale for the Kurel (Kubernetes Resources Launcher) package system. It serves as a comprehensive record of all design choices made during the extensive design iteration process.

---

## Design Philosophy & Core Principles

### Fundamental Philosophy
**"Kurel just generates YAML"** - This principle guided every design decision. Kurel is not a runtime system, orchestrator, or complex package manager. It's a declarative system for generating Kubernetes manifests with validation and customization capabilities.

### Core Design Principles
1. **Explicit over Implicit** - Always prefer explicit configuration over hidden defaults
2. **Flexible but Validated** - Don't constrain unnecessarily but validate what we can
3. **GitOps Compatible** - Generate proper Kubernetes manifests for GitOps workflows
4. **No Templating Engines** - Use patches instead of complex template logic
5. **Deterministic Output** - Same inputs always produce same outputs

### Key Design Constraints
- ❌ No templating or embedded logic in YAML
- ❌ No overlays or merging strategies (use patches instead)
- ❌ No conditionals or loops in YAML
- ❌ No composition or shared libraries between packages
- ✅ Variable substitution allowed, but only for keys in parameters.yaml
- ✅ All patches are deterministic, declarative, and validated

---

## Research Insights from Existing Systems

The kurel design was informed by extensive research into existing package management and deployment systems. Here are the key insights that shaped our decisions:

### Helm Charts Analysis
**Structure patterns adopted**:
- `Chart.yaml` → Inspired our metadata in `parameters.yaml` under `kurel:` key
- `values.yaml` → Our `parameters.yaml` serves similar purpose
- `templates/` → Our `resources/` for base manifests

**Patterns rejected**:
- Complex templating with `{{ }}` syntax → Use patches instead
- Dependencies in `Chart.yaml` → Handle at GitOps level
- `.helmignore` → Not needed for kurel's simpler model

### Docker Compose Learnings
**Patterns adopted**:
- `docker-compose.override.yml` → Our `.local.kurel` extension pattern
- Environment variable substitution → Our `${variable}` syntax
- Service profiles → Our conditional patch enabling

**Insights gained**:
- Override files work well for user customization
- Simple variable substitution is sufficient for most cases
- Profiles enable different deployment configurations

### Terraform Modules Study
**Structure patterns adopted**:
- `variables.tf` → Our parameter documentation approach
- `README.md` → Documentation importance
- Clear input/output interface → Our parameters/generated manifests

**Patterns adapted**:
- Version constraints → Decided to handle at GitOps level
- Module composition → Kept packages self-contained

### Kustomize Patterns
**Concepts adopted**:
- `patches/` directory structure
- Declarative customization without templating
- Base + overlay pattern → Our package + .local.kurel pattern

**Improvements made**:
- Better patch organization with subdirectories
- Conditional patch application vs static overlays
- Integrated variable system vs separate files

### ArgoCD Applications Research
**Patterns adopted**:
- Sync waves → Our install phase annotations
- Application dependencies → Our phase-based deployment
- Health checks → Our wait-for-ready annotations

**GitOps integration insights**:
- Need for deployment ordering in complex applications
- Importance of GitOps-native manifest generation
- Value of dependency management at orchestration level

### Key Research Conclusions
1. **No single system does everything well** - Each has strengths for specific use cases
2. **Templating complexity** - Most users struggle with complex template syntax
3. **Override patterns work** - Docker Compose override model is intuitive
4. **Validation is crucial** - All successful systems provide parameter validation
5. **GitOps compatibility** - Modern systems must integrate well with GitOps workflows

---

## Package Structure Evolution

### Final Package Structure
```
my-app.kurel/
├── parameters.yaml          # All variables + metadata
├── resources/               # Base Kubernetes manifests (one GVK per file)
│   ├── deployment.yaml
│   ├── service.yaml
│   └── namespaces.yaml
├── patches/                 # Modular patches with numeric ordering
│   ├── 00-base.kpatch      # Standard global patterns (explicit)
│   ├── features/
│   │   ├── 10-monitoring.kpatch
│   │   ├── 10-monitoring.yaml   # Patch metadata
│   │   └── 20-ingress.kpatch
│   └── profiles/
│       ├── 10-development.kpatch
│       └── 10-production.kpatch
├── schemas/                 # Auto-generated validation
│   └── parameters.schema.json
├── examples/               # Example configurations
│   └── production.yaml
└── README.md              # Documentation

my-app.local.kurel/         # User extensions (optional)
├── patches/                # Additional patches only
│   └── 50-custom.kpatch
└── parameters.yaml         # Override parameter values
```

### Original Structure (Rejected)
The initial design from the existing DESIGN.md included:
```
my-app.kurel/
├── resources/
├── parameters.kpatch        # Single patch file
├── config.kpatch           # Multi-resource patch set
├── config.schema.json      # JSONSchema for validation
├── instance.schema.json    # Schema for instance-level fields
├── instance.yaml           # External instance configuration
└── README.md
```

### Evolution & Rejected Alternatives

#### Directory Name
- **Chosen**: `my-app.kurel/` - Clear package identity
- **Considered**: `.kurel` suffix vs directory structure - decided on directory for better organization

#### Patch Organization
- **Original**: Single `parameters.kpatch` file
- **Intermediate**: `config.kpatch` for multi-resource patches
- **Final**: Multiple `.kpatch` files in `patches/` subdirectories
- **Rationale**: Better organization, modular patches, easier to maintain

#### Configuration Files
- **Original**: Separate `instance.yaml` external to package
- **Final**: `parameters.yaml` within package, `.local.kurel` for overrides
- **Rationale**: Simpler structure, explicit override pattern

#### Metadata Location
- **Considered**: Separate `kurel.yaml` for package metadata
- **Final**: Metadata in `parameters.yaml` under `kurel:` key
- **Rationale**: Single source of truth, metadata available as variables

---

## Parameters System Design

### Final Parameter Structure
```yaml
# parameters.yaml

# Package metadata (fixed key)
kurel:
  name: prometheus-operator
  version: 0.68.0
  appVersion: 0.68.0
  description: "Prometheus Operator creates/manages Prometheus clusters"
  home: https://github.com/prometheus-operator/prometheus-operator
  keywords: ["monitoring", "prometheus", "operator"]
  maintainers:
    - name: "Prometheus Team"
      email: "prometheus-operator@googlegroups.com"

# Global defaults (fixed key)
global:
  labels:
    app.kubernetes.io/name: "${kurel.name}"
    app.kubernetes.io/version: "${kurel.appVersion}"
    app.kubernetes.io/managed-by: "kurel"
  annotations:
    kurel.gokure.dev/package: "${kurel.name}"
    kurel.gokure.dev/version: "${kurel.version}"
  resources:
    requests:
      cpu: 100m
      memory: 128Mi
    limits:
      cpu: 1000m
      memory: 1Gi
  securityContext:
    runAsNonRoot: true
    runAsUser: 65534
    fsGroup: 65534
  imagePullPolicy: IfNotPresent
  nodeSelector: {}
  tolerations: []

# Author-defined variables (any structure)
monitoring:
  enabled: false
  serviceMonitor:
    enabled: "${monitoring.enabled}"   # Nested reference
    interval: 30s

image:
  registry: quay.io
  repository: prometheus-operator/prometheus-operator
  tag: "v${kurel.appVersion}"         # Reference to metadata
  pullPolicy: IfNotPresent

persistence:
  enabled: true
  size: 10Gi
  storageClass: ""

resources:
  controller:
    requests:
      cpu: 200m    # Override global default
      memory: 256Mi
```

### Variable Reference System
- **Syntax**: `${section.subsection.value}` with dot notation
- **Nested references supported**: `${monitoring.serviceMonitor.enabled}` can reference `${monitoring.enabled}`
- **Metadata references**: Variables can reference package metadata like `${kurel.appVersion}`
- **Global patterns**: `${global.*}` for cross-cutting defaults

### Fixed Top-Level Keys

#### `kurel:` Key (Package Metadata)
**Purpose**: Package identification and metadata, also available as variables
**Required fields**:
- `name`: Package name (used in generated resources)
- `version`: Package version
- `appVersion`: Upstream application version

**Optional fields**:
- `description`: Human-readable description
- `home`: URL to project homepage
- `keywords`: Array of keywords for discovery
- `maintainers`: Array of maintainer objects

#### `global:` Key (Default Values)
**Purpose**: Default values applied across all resources via base patches
**Common patterns**:
- `labels`: Applied to all resources
- `annotations`: Applied to all resources  
- `resources`: Default resource requests/limits
- `securityContext`: Default security settings
- `nodeSelector`: Default node selection
- `tolerations`: Default tolerations
- `imagePullPolicy`: Default image pull policy

### ExtendedValue Evolution

#### Original ExtendedValue Design (Rejected)
We initially considered an "ExtendedValue" struct to provide validation metadata:

```yaml
# Original ExtendedValue approach
persistence:
  size:
    _schema: extended        # Explicit marker
    value: 10Gi
    type: string
    pattern: "^[0-9]+[KMGT]i$"
    description: "Storage size in Kubernetes format"
    required: true
    minimum: null
    maximum: null
```

#### Detection Problem
The challenge was distinguishing between ExtendedValue objects and regular nested configuration:

```yaml
# Is this an ExtendedValue or regular config?
database:
  credentials:
    value: secret-ref    # Could be ExtendedValue.value or just a config key
    type: kubernetes     # Could be ExtendedValue.type or just a config key
```

#### Final Decision: Direct Schema Generation
**Chosen approach**: Generate JSON Schema directly from parameters.yaml + K8s API tracing
**Rationale**: 
- Avoids duplication between ExtendedValue and schema
- Leverages existing Kubernetes validation
- Cleaner parameter files
- Standard JSON Schema tooling support

### Parameter Override System

#### Resolution Order
1. **Package `parameters.yaml`** - Base values and metadata
2. **Local `my-app.local.kurel/parameters.yaml`** - User overrides (highest priority)
3. **Error if variable not found**

#### Local Override Pattern
- **Same filename**: `parameters.yaml` in both locations
- **Rejected alternatives**: `values.yaml`, `overrides.yaml`, `local.yaml`
- **Rationale**: Consistency and simplicity

---

## Patch System Architecture

### Patch Discovery & Organization

#### File Organization
```
patches/
├── 00-base.kpatch          # Global patterns (explicit)
├── features/               # Feature-specific patches
│   ├── 10-monitoring.kpatch
│   ├── 10-monitoring.yaml  # Patch metadata
│   ├── 20-ingress.kpatch
│   └── 30-persistence.kpatch
├── profiles/               # Environment profiles
│   ├── 10-development.kpatch
│   ├── 20-staging.kpatch
│   └── 30-production.kpatch
└── resources/              # Resource-specific patches
    ├── 10-limits-small.kpatch
    ├── 20-limits-medium.kpatch
    └── 30-limits-large.kpatch
```

#### Naming Convention Evolution
- **Initial idea**: Required prefixes like `feature-`, `profile-`
- **Final decision**: Numeric prefixes only: `NN-descriptive-name.kpatch`
- **Rationale**: Flexibility without unnecessary constraints

#### Numeric Prefix Guidelines (Rejected)
We considered prescriptive numeric ranges:
- 10-19: Core features/settings
- 20-29: Additional features  
- 30-39: Advanced/optional features
- 90-99: Override/cleanup patches

**Decision**: No prescribed ranges - users decide their own numbering system
**Rationale**: Avoid artificial constraints, let package authors organize as they see fit

#### Directory Structure Preferences
- **Allow**: Direct patches in `patches/` root
- **Prefer**: Subdirectories for organization
- **Rationale**: Flexibility with gentle guidance toward better organization

### Patch Discovery & Ordering

#### Discovery Pattern
- **Glob**: `patches/**/*.kpatch`
- **Processing order**: Alphabetical by full path (directory + filename)
- **Numeric sorting**: `10-` comes before `20-` comes before `9-` (string sort)

#### Example Processing Order
1. `patches/00-base.kpatch`
2. `patches/features/10-monitoring.kpatch`
3. `patches/features/20-ingress.kpatch`
4. `patches/profiles/10-development.kpatch`
5. `patches/resources/10-limits-small.kpatch`

### Conditional Patch Enabling

#### Patch Metadata Files
Each patch can have a corresponding `.yaml` file with the same base name:

```yaml
# features/10-monitoring.yaml
enabled: "${monitoring.enabled}"      # Simple boolean expression
description: "Adds Prometheus monitoring sidecars and annotations"
requires:                            # Auto-enable these patches
  - "features/05-metrics-base.kpatch"
  - "features/15-monitoring-rbac.kpatch"
conflicts:                           # Cannot be enabled together
  - "features/25-lightweight-monitoring.kpatch"
```

#### Enabling Expression Language
- **Chosen**: Simple boolean variables only
- **Syntax**: `"${variable.name}"` evaluates to true/false
- **Rejected**: Complex expressions like `"${environment == 'production'}"`
- **Rationale**: Keep it simple, avoid expression language complexity

#### Dependency Resolution Evolution

**Option A: Requirements as Prerequisites (Initially Chosen)**
```yaml
requires:
  - "features/10-metrics-base.kpatch"
```
- If `metrics-base` NOT enabled → Error: "monitoring requires metrics-base"
- User must explicitly enable both patches
- More explicit control, prevents surprises

**Option B: Requirements with Auto-Enable (Final Choice)**
```yaml
requires:
  - "features/10-metrics-base.kpatch"
```
- If `monitoring` enabled → automatically enables `metrics-base`
- Creates dependency chains
- Transitive dependencies supported
- User changed mind during design process

**Rationale for change**: User convenience outweighs explicitness concern

#### Dependency Resolution Process
1. **Parse all patch metadata** files
2. **Evaluate `enabled` expressions** against parameters
3. **Build dependency graph** from `requires` fields
4. **Auto-enable required patches** transitively
5. **Check for conflicts** between enabled patches
6. **Detect circular dependencies**
7. **Report what was auto-enabled** to user

#### Conflict Resolution
- **Error on conflicts**: Cannot enable conflicting patches simultaneously
- **Example**: Monitoring and lightweight-monitoring are mutually exclusive
- **User feedback**: Clear error messages explaining conflicts

### Base Patch Pattern

#### Evolution from Implicit to Explicit

**Original idea**: Automatic `base.kpatch` applied to all resources
**Problem**: If it's always applied, why not just update the base YAML?
**Final decision**: Explicit `00-base.kpatch` that package author must include

#### Base Patch Content
```toml
# patches/00-base.kpatch - Applied to all resources

# Global labels and annotations
metadata.labels: "${global.labels}"
metadata.annotations: "${global.annotations}"

# Apply to all Deployments
[deployment.*.spec.template.spec]
securityContext: "${global.securityContext}"
nodeSelector: "${global.nodeSelector}"
tolerations: "${global.tolerations}"

# Apply to all containers in all Deployments
[deployment.*.spec.template.spec.containers.*]
resources: "${global.resources}"
imagePullPolicy: "${global.imagePullPolicy}"
```

#### What Goes in Base Patches
- **Cross-cutting concerns**: Labels, annotations applied to all resources
- **Security defaults**: SecurityContext, RBAC patterns
- **Resource defaults**: CPU/memory requests and limits
- **Image patterns**: Registry, pull policies
- **Scheduling**: Node selectors, tolerations, affinity

### TOML Headers Clarification

#### What We Kept
- **Standard TOML headers** for patch targeting remain part of the core patch design
- **Example**: `[deployment.my-app.spec.template.spec.containers.0]`

#### What We Rejected
- **Special TOML headers** for variable definitions in patch files
- **Example** (rejected): 
  ```toml
  [variables]
  cpu_request = "100m"
  memory_request = "128Mi"
  ```

#### Final Decision
- **All variable definitions** go in `parameters.yaml`
- **Patch files** contain only targeting headers and patch operations
- **Metadata files** (`.yaml`) contain patch enabling/dependency info
- **Clean separation** of concerns

---

## Multi-Namespace Support & Validation

### Namespace Handling Philosophy

#### Design Decision: Full Flexibility
- **Allow**: Resources targeting any namespaces
- **Allow**: Creating multiple namespaces
- **Allow**: Cross-namespace references
- **Rejected**: Single-namespace enforcement (like some Helm charts)
- **Rationale**: "kurel just generates YAML" - don't artificially constrain users

#### Example Multi-Namespace Package
```yaml
# resources/namespaces.yaml
apiVersion: v1
kind: Namespace
metadata:
  name: apps
---
apiVersion: v1
kind: Namespace
metadata:
  name: monitoring
---

# resources/deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: frontend
  namespace: apps      # Different namespace

---
# resources/service.yaml
apiVersion: v1
kind: Service
metadata:
  name: metrics
  namespace: monitoring  # Another namespace
```

### Namespace Creation Control

#### Control Mechanism
```yaml
# parameters.yaml
global:
  namespaces:
    create: true         # Default: create namespaces
    exclude:             # Don't create these
      - "kube-system"
      - "default"
```

#### Base Patch Integration
```toml
# patches/00-base.kpatch - Conditional namespace creation
[namespace.${monitoring.namespace}]
enabled: "${global.namespaces.create}"
metadata.labels: "${global.labels}"
metadata.annotations: "${global.annotations}"
```

#### Manual Namespace Control
Users can disable specific namespace creation:
```toml
# patches/99-namespace-overrides.kpatch
[namespace.kube-system]
enabled: false  # Don't try to create kube-system
```

### Validation Scope & Approach

#### Validation Scope Decision
- **Within package only**: Check conflicts within the generated manifests
- **Not against live cluster**: No validation against existing cluster resources
- **Rationale**: Keep kurel simple, cluster validation is GitOps tool responsibility

#### Validation Checks
```bash
kurel validate my-app.kurel/

✓ No naming conflicts within namespaces
✓ Resources reference consistent namespaces  
⚠ Warning: Resources use namespace 'custom-ns' but no Namespace resource found
   → Enable global.namespaces.create=true or create Namespace resource manually
✓ Cross-namespace Service→Deployment references look valid
✓ All patch variable references exist in parameters.yaml
✗ Error: Two Services named 'api' in namespace 'apps'
```

#### Cross-Namespace Reference Validation
- **Basic validation**: Check that referenced resources exist in package
- **Example**: Service targeting Deployment in different namespace
- **Future enhancement**: More sophisticated reference checking

---

## GitOps Integration & Deployment Phases

### Install Phase Annotations

#### Annotation Design
- **Domain**: `kurel.gokure.dev/` (uses our domain as discussed)
- **Install phase**: `kurel.gokure.dev/install-phase`
- **Valid values**: `pre-install`, `main`, `post-install`
- **Default**: `main` if not specified

#### Additional Control Annotations
```yaml
# Resource-level deployment control
metadata:
  annotations:
    kurel.gokure.dev/install-phase: "pre-install"
    kurel.gokure.dev/wait-for-ready: "true"
    kurel.gokure.dev/timeout: "5m"
```

#### Three-Phase Deployment Pattern
1. **Pre-install**: CRDs, namespaces, RBAC, secrets
2. **Main**: Primary application resources (default)
3. **Post-install**: Monitoring, backups, optional components

### Flux Translation Example

#### Generated Kustomization Structure
```
kustomizations/
├── my-app-pre-install/
│   ├── kustomization.yaml    # No dependencies
│   └── ...
├── my-app-main/
│   ├── kustomization.yaml    # Depends on: my-app-pre-install
│   └── ...
└── my-app-post-install/
    ├── kustomization.yaml    # Depends on: my-app-main
    └── ...
```

#### Flux Kustomization Dependencies
```yaml
# my-app-main/kustomization.yaml
apiVersion: kustomize.toolkit.fluxcd.io/v1
kind: Kustomization
metadata:
  name: my-app-main
spec:
  dependsOn:
    - name: my-app-pre-install
  # ... rest of spec
```

### ArgoCD Compatibility
- **Sync waves**: Install phases can map to ArgoCD sync wave annotations
- **Health checks**: Compatible with ArgoCD health assessment
- **Dependency management**: ArgoCD can handle phase dependencies

### Patch Modification of Phases
- **Patches can modify** install phase annotations
- **Use case**: User wants to move a resource to different phase
- **Example**:
  ```toml
  # User patch to move monitoring to pre-install
  [deployment.monitoring]
  metadata.annotations["kurel.gokure.dev/install-phase"]: "pre-install"
  ```

---

## User Extension System

### .local.kurel Design Pattern

#### Design Philosophy
- **Simple overlay model**: Local extends, doesn't modify package
- **Docker Compose inspiration**: Similar to `docker-compose.override.yml`
- **No resource overrides**: Can only add patches, not replace base resources

#### Extension Structure
```
my-app.local.kurel/
├── patches/                # Additional patches only
│   ├── 50-custom-limits.kpatch
│   ├── 60-local-config.kpatch
│   └── team/
│       └── 70-team-policy.kpatch
└── parameters.yaml         # Override parameter values
```

#### Processing Order
1. **Load package parameters.yaml**
2. **Merge local parameters.yaml** (local values override package values)
3. **Resolve all variables** with final parameter values
4. **Apply package patches** (enabled based on final parameters)
5. **Apply local patches** (enabled based on final parameters)

#### Local Patch Capabilities & Restrictions

**Can do**:
- Add new patches that apply to any resources
- Use any variables from merged parameters
- Target any resources generated by package

**Cannot do**:
- Reference package patches in `requires` field
- Override or disable package patches directly
- Replace base resources (only patch them)

**Rationale**: Keep local extensions simple and avoid complex interactions

### Rejected Extension Patterns

#### Multiple Overlay Types
**Considered**: `.env.kurel`, `.team.kurel`, `.local.kurel` for different contexts
**Rejected**: Too complex, single `.local.kurel` is sufficient
**Rationale**: Most users need one level of customization

#### Direct Patch Disabling
**Considered**: Allow local config to disable package patches
```yaml
# Rejected approach
disable_patches:
  - "features/20-monitoring.kpatch"
```
**Rejected**: User can already control this via parameter values
**Rationale**: Don't duplicate control mechanisms

#### Resource Replacement
**Considered**: Allow local extensions to replace base resources
**Rejected**: Too complex, patches are sufficient
**Rationale**: Maintain clear separation between base resources and customizations

---

## Schema Generation & Validation

### Schema Generation Approach

#### Three-Phase Strategy

**Phase 1: Basic Type Inference**
- Scan `parameters.yaml` and infer types from current values
- `3` → `"type": "integer"`
- `true` → `"type": "boolean"`
- `"10Gi"` → `"type": "string"` with K8s quantity pattern detection

**Phase 2: Kubernetes Path Tracing**
- Parse all `.kpatch` files for variable usage
- Trace patch paths to Kubernetes resource fields
- Query Kubernetes OpenAPI schemas for validation rules
- Generate constraints based on K8s field definitions

**Phase 3: Manual Overrides**
- Support manual schema additions for complex cases
- Allow override of auto-generated constraints
- Handle cases where tracing fails or is insufficient

#### Path Tracing Example
```yaml
# parameters.yaml
replicas: 3
resources:
  memory: 1Gi
```

```toml
# patches/10-scale.kpatch
[deployment.my-app]
spec.replicas: "${replicas}"
spec.template.spec.containers[0].resources.limits.memory: "${resources.memory}"
```

**Tracing process**:
1. `${replicas}` → `deployment.spec.replicas` → K8s: integer, minimum: 0
2. `${resources.memory}` → `container.resources.limits.memory` → K8s: string, K8s quantity format

**Generated schema**:
```json
{
  "replicas": {
    "type": "integer", 
    "minimum": 0,
    "description": "Number of replicas (from Deployment.spec.replicas)"
  },
  "resources": {
    "type": "object",
    "properties": {
      "memory": {
        "type": "string",
        "pattern": "^[0-9]+[KMGT]i$",
        "description": "Memory limit (from Container.resources.limits.memory)"
      }
    }
  }
}
```

### Validation Command Design

#### Comprehensive Validation
```bash
kurel validate my-app.kurel/ --values custom-values.yaml

✓ Validating parameters against generated schema
✓ Validating patch variable references  
✓ Checking patch dependencies and conflicts
✓ Validating generated Kubernetes resources
⚠ Warning: Variable 'monitoring.retention' used but not in schema
✗ Error: persistence.size "10GB" doesn't match pattern "^[0-9]+[KMGT]i$"
✗ Error: Circular dependency: monitoring → metrics → monitoring
```

#### Validation Levels
1. **Parameter schema validation**: User values against generated/manual schema
2. **Variable reference validation**: All `${...}` exist in parameters
3. **Patch dependency validation**: No circular dependencies, no conflicts
4. **Kubernetes resource validation**: Against K8s OpenAPI when possible
5. **Naming conflict validation**: Within package scope

### CRD Support Strategy

#### Well-Known CRDs
- **Start**: Support popular CRDs (Cert-Manager, External Secrets, MetalLB)
- **Mechanism**: Bundle known CRD schemas with kurel
- **Discovery**: Detect CRD usage in patches and apply appropriate schemas

#### Custom CRDs
- **Future**: Allow users to provide CRD schemas
- **Mechanism**: `schemas/crds/` directory in package
- **Auto-detection**: Parse CRD YAML to extract schema

### Schema Distribution

#### Generation Strategy
- **On-demand generation**: Generate schemas when needed
- **Caching**: Cache generated schemas for performance
- **Version awareness**: Regenerate when parameters or patches change

#### Pre-generation (Rejected)
**Considered**: Include pre-generated schemas in packages
**Rejected**: Adds complexity, schemas can become stale
**Rationale**: Generate fresh schemas ensure accuracy

---

## Rejected Features & Design Alternatives

### Package Dependencies

#### What Was Considered
Helm-style package dependencies with version constraints:
```yaml
# Rejected approach
kurel:
  name: my-app
  dependencies:
    - name: postgresql
      version: ">=11.0.0"
      repository: "https://charts.bitnami.com"
    - name: redis
      version: "^6.0.0"
      condition: caching.enabled
```

#### Why Rejected
- **Philosophy conflict**: "kurel just generates YAML"
- **Complexity**: Would require package registry, version resolution
- **Better handled elsewhere**: GitOps tools (Flux/ArgoCD) handle dependencies
- **User preference**: Dependencies "explicitly configured at higher level"

### RBAC Auto-Management

#### What Was Considered
Automatic RBAC generation and validation:
```yaml
# Rejected approach
kurel:
  rbac:
    required: true
    permissions:
      - apiGroups: [""]
        resources: ["pods"]
        verbs: ["get", "list", "watch"]
```

#### Why Rejected
- **No clear "automagical" benefit**: RBAC requirements are application-specific
- **Simple alternative**: Include RBAC resources in `resources/` like any other K8s resource
- **Validation complexity**: Hard to automatically determine required permissions
- **Philosophy**: Keep kurel focused on YAML generation, not security analysis

### Multi-Tenancy Enforcement

#### What Was Considered
Built-in tenancy validation and namespace enforcement:
```yaml
# Rejected approach
kurel:
  tenancy:
    mode: strict
    allowedNamespaces: ["app-*"]
    resourceNaming: tenant-prefixed
```

#### Why Rejected
- **Higher-level concern**: Tenancy better handled by GitOps tools and admission controllers
- **Flexibility loss**: Would constrain valid use cases unnecessarily
- **Philosophy**: kurel generates YAML, tenancy tools enforce policies

### Complex Expression Language

#### What Was Considered
Rich expression language for patch enabling:
```yaml
# Rejected approach
enabled: "${environment == 'production' && monitoring.enabled && !minimal_install}"
```

#### Why Rejected
- **Complexity**: Would require expression parser and evaluator
- **Simple alternative**: Boolean variables work for most cases
- **Debugging difficulty**: Complex expressions hard to troubleshoot
- **Philosophy**: Keep patch enabling simple and predictable

### Conditional YAML Structures

#### What Was Considered
Helm-style conditional blocks in YAML:
```yaml
# Rejected approach (Helm-style)
{{- if .Values.persistence.enabled }}
spec:
  volumeClaimTemplates:
  - metadata:
      name: data
{{- end }}
```

#### Why Rejected
- **Design constraint**: No templating or embedded logic
- **Alternative**: Use patches to add/remove structures
- **Cleaner separation**: Base YAML + patches vs templated YAML

### Pre-Generated Schemas in Packages

#### What Was Considered
Include generated schemas in package distribution:
```
my-app.kurel/
├── schemas/
│   ├── parameters.schema.json    # Pre-generated
│   └── resources.schema.json     # Pre-generated
```

#### Why Rejected
- **Staleness risk**: Schemas become outdated when parameters change
- **Build complexity**: Requires build step to generate schemas
- **Size overhead**: Adds to package size unnecessarily
- **Alternative**: Generate on-demand with caching

### Complex Package Composition

#### What Was Considered
Ability to compose packages from multiple sources:
```yaml
# Rejected approach
kurel:
  name: my-app
  includes:
    - package: base-app
      patches: ["security/*"]
    - package: monitoring  
      version: "1.0.0"
```

#### Why Rejected
- **Complexity**: Would require dependency resolution, version management
- **Philosophy**: Keep packages self-contained
- **Alternative**: Copy/fork packages for customization

---

## Implementation Considerations

### CLI Command Design

#### Core Commands
```bash
# Validate package and user configuration
kurel validate my-app.kurel/ --values custom-values.yaml

# Generate schemas from package
kurel schema generate my-app.kurel/

# Build final manifests
kurel build my-app.kurel/ --values custom-values.yaml --output ./manifests/

# Package information
kurel info my-app.kurel/

# List available patches and their status
kurel patches list my-app.kurel/ --values custom-values.yaml
```

#### Validation Output Design
```bash
$ kurel validate my-app.kurel/ --values production.yaml

✓ Package structure valid
✓ Parameters schema validation passed
✓ All patch variables resolved

Enabled patches:
  ✓ patches/00-base.kpatch (always enabled)
  ✓ patches/features/10-monitoring.kpatch (monitoring.enabled=true)
  → patches/features/05-metrics-base.kpatch (required by 10-monitoring)
  → patches/features/15-monitoring-rbac.kpatch (required by 10-monitoring)
  ✗ patches/features/20-ingress.kpatch (conflicts with 25-simple-ingress)

Generated resources:
  ✓ 3 Namespaces
  ✓ 5 Deployments  
  ✓ 8 Services
  ✓ 2 Ingresses
  ⚠ Warning: Resources span 3 namespaces

Validation summary: 1 error, 1 warning
```

### Variable Resolution Engine

#### Resolution Algorithm  
1. **Parse parameter files**: Package + local override
2. **Build variable map**: Flatten nested structure to dot notation
3. **Scan patch files**: Extract all `${...}` references
4. **Validate references**: Ensure all variables exist
5. **Resolve nested references**: Handle `${a.b}` where `a.b: "${c.d}"`
6. **Type casting**: Apply schema-based type conversion
7. **Circular dependency detection**: Prevent infinite resolution

#### Variable Resolution Example
```yaml
# parameters.yaml
kurel:
  appVersion: "1.0.0"
image:
  tag: "v${kurel.appVersion}"    # References metadata
  full: "${image.registry}/${image.repository}:${image.tag}"
```

**Resolution steps**:
1. `kurel.appVersion` = `"1.0.0"`
2. `image.tag` = `"v${kurel.appVersion}"` → `"v1.0.0"`
3. `image.full` = `"${image.registry}/${image.repository}:${image.tag}"` → `"quay.io/myapp:v1.0.0"`

### Patch Processing Engine

#### Processing Pipeline
1. **Discovery**: Glob `patches/**/*.kpatch`
2. **Metadata loading**: Load corresponding `.yaml` files
3. **Dependency resolution**: Build DAG, detect cycles, auto-enable
4. **Conflict checking**: Validate no conflicting patches enabled
5. **Variable substitution**: Replace `${...}` with resolved values
6. **Patch application**: Apply patches to base resources using Kure engine
7. **Phase organization**: Group resources by install phase

#### Dependency Graph Example
```
10-monitoring.kpatch
├── requires: 05-metrics-base.kpatch
└── requires: 15-monitoring-rbac.kpatch
    └── requires: 02-base-rbac.kpatch

Result: Enable 02-base-rbac → 05-metrics-base → 15-monitoring-rbac → 10-monitoring
```

### GitOps Manifest Generation

#### Phase-Based Organization
```
output/
├── pre-install/
│   ├── kustomization.yaml    # Phase resources only
│   ├── namespaces.yaml
│   ├── rbac.yaml
│   └── secrets.yaml
├── main/
│   ├── kustomization.yaml    # dependsOn: pre-install
│   ├── deployments.yaml
│   └── services.yaml
└── post-install/
    ├── kustomization.yaml    # dependsOn: main  
    ├── monitoring.yaml
    └── backups.yaml
```

#### Kustomization Generation
```yaml
# main/kustomization.yaml (auto-generated)
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

# Flux dependency
dependsOn:
  - name: my-app-pre-install

resources:
  - deployments.yaml
  - services.yaml

# Common labels applied to all resources
commonLabels:
  app.kubernetes.io/name: my-app
  app.kubernetes.io/managed-by: kurel
```

---

## Future Considerations

### Schema Generation Improvements

#### Enhanced K8s API Tracing
- **CRD discovery**: Automatic detection of CRD schemas
- **Version-aware tracing**: Handle multiple K8s API versions
- **Complex path resolution**: Better handling of array selectors and wildcards

#### Machine Learning Schema Enhancement
- **Pattern recognition**: Learn common parameter patterns from existing packages
- **Validation suggestions**: Suggest additional validation rules based on usage

### Package Ecosystem

#### Package Registry
- **Distribution**: Central registry for sharing kurel packages
- **Discovery**: Search and browse available packages
- **Versioning**: Semantic versioning for packages
- **Security**: Package signing and verification

#### Package Management Tools
- **Installation**: `kurel install prometheus-operator`
- **Updates**: `kurel upgrade --check`
- **Dependencies**: Automatic dependency resolution

### Advanced Patch Features

#### Patch Testing
- **Unit tests**: Test individual patches against known resources
- **Integration tests**: Test full package generation
- **Regression tests**: Ensure patches don't break existing functionality

#### Patch Composition
- **Mixins**: Reusable patch fragments
- **Inheritance**: Base patches that others extend
- **Conditional application**: More sophisticated enabling logic

### Developer Experience

#### IDE Integration
- **Language servers**: Completion and validation in editors
- **Schema integration**: Real-time parameter validation
- **Patch debugging**: Visual patch application tracing

#### Package Development Tools
- **Scaffolding**: Generate package templates
- **Validation**: Real-time package structure validation
- **Testing**: Framework for package testing

---

## Conclusion

This comprehensive design represents the result of extensive iteration and consideration of alternatives. The key insight that guided all decisions was the principle that "kurel just generates YAML" - keeping the system focused on its core purpose while providing powerful customization and validation capabilities.

The design balances several important concerns:

- **Simplicity vs Power**: Provide powerful features without overwhelming complexity
- **Flexibility vs Validation**: Allow maximum flexibility while catching common errors
- **Explicitness vs Convenience**: Prefer explicit configuration with convenient defaults
- **Standards vs Innovation**: Build on existing patterns (Kubernetes, GitOps) while solving real problems

The modular patch system, comprehensive parameter handling, and GitOps-native output make kurel well-suited for managing complex Kubernetes applications in a declarative, version-controlled manner. The extensive validation and schema generation capabilities help prevent common configuration errors while maintaining the flexibility that makes Kubernetes powerful.

The decision to reject certain features (package dependencies, complex templating, automatic RBAC) keeps kurel focused on its core competency while allowing the broader ecosystem (GitOps tools, policy engines, package managers) to handle their respective concerns.