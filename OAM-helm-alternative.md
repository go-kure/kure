# OAM-Based Helm Alternative: Design Overview (Revised)

> **Status:** Initial Design Draft (Revised)
> **Date:** January 2025

## Executive Summary

This document describes a Helm alternative built on OAM (Open Application Model) principles. The system separates concerns between application maintainers, application users, and platform administrators, using a layered architecture that renders declarative specifications to static Kubernetes YAML.

**Key characteristics:**
- Go library with CLI (no runtime operator required)
- OAM-compatible core with clearly marked extensions
- Layered ComponentDefinitions for composition
- Platform-aware trait resolution
- Staged output for GitOps workflows

---

## Table of Contents

1. [OAM Compliance Notes](#oam-compliance-notes)
2. [Core Architecture](#core-architecture)
3. [The Input Model](#the-input-model)
4. [Layered OAM: Compositional ComponentDefinitions](#layered-oam-compositional-componentdefinitions)
5. [Traits and Platform Capabilities](#traits-and-platform-capabilities)
6. [Platform Bootstrap](#platform-bootstrap)
7. [Staged Rendering for GitOps](#staged-rendering-for-gitops)
8. [Component Libraries](#component-libraries)
9. [CLI Interface](#cli-interface)
10. [Design Principles](#design-principles)

---

## OAM Compliance Notes

This design aims for OAM compatibility where possible. This section explicitly documents what is standard OAM and what are extensions.

### Standard OAM (core.oam.dev/v1beta1)

| Kind | Status | Notes |
|------|--------|-------|
| `Application` | ✅ Standard | Components with `type`, `properties`, and `traits` |
| `ComponentDefinition` | ✅ Standard | Defines component types, schemas via CUE, workload mapping |
| `TraitDefinition` | ✅ Standard | Defines traits and their schemas |
| `WorkloadDefinition` | ✅ Standard | Maps to Kubernetes workload kinds |
| `PolicyDefinition` | ✅ Standard | Defines policies |

### Extensions (Not OAM Spec)

| Kind | Purpose | Rationale |
|------|---------|-----------|
| `ApplicationTemplate` | Maintainer intent: wraps ComponentDefinitions + defaults + documentation | OAM has no "Helm chart equivalent" — the schema lives in ComponentDefinition, but maintainers need a packaging unit |
| `PlatformBundle` | Bundle of TraitDefinitions + PolicyDefinitions + cluster defaults | OAM doesn't define "cluster capability registry" — this is our extension using OAM primitives |

### Key Correction: Where Parameter Schemas Live

In OAM, parameter schemas live in **ComponentDefinition**, not Application:
```
┌─────────────────────────────────────────────────────────────────┐
│  ComponentDefinition                                            │
│    - Defines parameter schema (via CUE)                         │
│    - Defines how to render to K8s resources                     │
│    - Owned by: component author / library maintainer            │
├─────────────────────────────────────────────────────────────────┤
│  Application                                                    │
│    - References component types                                 │
│    - Passes `properties` (values matching the schema)           │
│    - Attaches traits                                            │
│    - Owned by: application deployer                             │
└─────────────────────────────────────────────────────────────────┘
```

The "values.yaml" in our system is simply a convenient way to populate `properties` — it compiles into a standard OAM Application.

---

## Core Architecture
```
┌─────────────────────────────────────────────────────────────────┐
│                         INPUTS                                  │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│  ApplicationTemplate        Extension: Maintainer packaging     │
│    - References ComponentDefinitions                            │
│    - Documents configurable properties                          │
│    - Provides defaults                                          │
│                                                                 │
│  values.yaml                User-provided values                │
│    - Populates component properties                             │
│    - Merged with defaults from ApplicationTemplate              │
│                                                                 │
│  PlatformBundle             Extension: Cluster capabilities     │
│    - Bundle of TraitDefinitions                                 │
│    - Bundle of PolicyDefinitions                                │
│    - Cluster-wide defaults                                      │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                       GO LIBRARY                                │
│                                                                 │
│  1. Load ComponentDefinitions (from libraries, platform)        │
│  2. Load TraitDefinitions (from PlatformBundle)                 │
│  3. Compile ApplicationTemplate + values → OAM Application      │
│  4. Resolve layered ComponentDefinitions                        │
│  5. Resolve traits using TraitDefinitions                       │
│  6. Render to Kubernetes resources                              │
│  7. Organize output by deployment stage                         │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│                         OUTPUTS                                 │
│                                                                 │
│  - Standard OAM Application (intermediate, optional)            │
│  - Static Kubernetes YAML, organized by stage                   │
│  - Optional: FluxCD Kustomizations / ArgoCD Applications        │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## The Input Model

### ComponentDefinition (OAM Standard)

Parameter schemas live here. This is where the "shape" of configuration is defined.
```yaml
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata:
  name: webservice
spec:
  workload:
    definition:
      apiVersion: apps/v1
      kind: Deployment
  schematic:
    cue:
      template: |
        output: {
          apiVersion: "apps/v1"
          kind:       "Deployment"
          metadata: name: context.name
          spec: {
            replicas: parameter.replicas
            selector: matchLabels: app: context.name
            template: {
              metadata: labels: app: context.name
              spec: containers: [{
                name:  context.name
                image: parameter.image
                ports: [{containerPort: parameter.port}]
                env: [
                  for k, v in parameter.env {
                    name:  k
                    value: v
                  }
                ]
              }]
            }
          }
        }

        // Parameter schema with defaults and types
        parameter: {
          image:    string
          port:     *8080 | int
          replicas: *1 | int
          env:      *{} | {[string]: string}
        }
```

### Application (OAM Standard)

References component types and passes properties. No schema definition here — that's in ComponentDefinition.
```yaml
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata:
  name: my-service
spec:
  components:
    - name: api
      type: webservice                    # References ComponentDefinition
      properties:                          # Must match ComponentDefinition schema
        image: registry.example.com/api:v1.2.3
        port: 8080
        replicas: 3
        env:
          DATABASE_URL: "postgres://db.example.com/myapp"
          LOG_LEVEL: "info"
      traits:
        - type: exposed                   # References TraitDefinition
          properties:
            host: api.example.com
        - type: observable
```

### ApplicationTemplate (Extension)

This is our packaging unit for maintainers — analogous to a Helm chart. It wraps ComponentDefinitions and provides documentation, defaults, and a values interface.
```yaml
# Extension kind - NOT OAM standard
apiVersion: oamx.example.dev/v1
kind: ApplicationTemplate
metadata:
  name: my-service
  version: "1.0.0"
spec:
  description: "My service deployment template"

  # Which ComponentDefinitions this template uses
  # These must exist in libraries or be bundled
  componentDefinitions:
    - webservice        # From standard library
    - worker            # From standard library

  # Which TraitDefinitions this template expects
  traitDefinitions:
    - exposed
    - observable
    - scalable

  # Template for the OAM Application
  # Values from values.yaml are substituted here
  applicationTemplate:
    components:
      - name: api
        type: webservice
        properties:
          image: "{{ .Values.image }}"
          port: "{{ .Values.port | default 8080 }}"
          replicas: "{{ .Values.replicas | default 2 }}"
          env:
            DATABASE_URL: "{{ .Values.databaseUrl }}"
            LOG_LEVEL: "{{ .Values.logLevel | default \"info\" }}"
        traits:
          - type: exposed
            properties:
              host: "{{ .Values.hostname }}"
          - type: observable
          - type: scalable
            properties:
              minReplicas: "{{ .Values.scaling.min | default 2 }}"
              maxReplicas: "{{ .Values.scaling.max | default 10 }}"

      - name: worker
        type: worker
        enabled: "{{ .Values.worker.enabled | default false }}"
        properties:
          image: "{{ .Values.image }}"
          concurrency: "{{ .Values.worker.concurrency | default 5 }}"
        traits:
          - type: observable
          - type: scalable

  # Documentation for values (like Helm's values.yaml comments)
  valuesSchema:
    image:
      type: string
      required: true
      description: "Container image for the service"

    databaseUrl:
      type: string
      required: true
      description: "PostgreSQL connection string"

    hostname:
      type: string
      required: true
      description: "Public hostname for ingress"

    replicas:
      type: integer
      default: 2
      description: "Number of API replicas"

    logLevel:
      type: string
      enum: [debug, info, warn, error]
      default: info

    scaling:
      type: object
      properties:
        min:
          type: integer
          default: 2
        max:
          type: integer
          default: 10

    worker:
      type: object
      properties:
        enabled:
          type: boolean
          default: false
        concurrency:
          type: integer
          default: 5
```

### values.yaml (User Input)

Simple values file — no Kubernetes knowledge required.
```yaml
image: registry.example.com/my-service:v2.3.1
databaseUrl: "postgres://db.prod.internal:5432/myapp"
hostname: api.example.com
replicas: 5
logLevel: debug

scaling:
  min: 3
  max: 20

worker:
  enabled: true
  concurrency: 10
```

### Compilation: ApplicationTemplate + values → OAM Application

The CLI compiles these into a standard OAM Application:
```bash
oamctl compile -t my-service -v values.yaml -o application.yaml
```

Output (standard OAM):
```yaml
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata:
  name: my-service
spec:
  components:
    - name: api
      type: webservice
      properties:
        image: registry.example.com/my-service:v2.3.1
        port: 8080
        replicas: 5
        env:
          DATABASE_URL: "postgres://db.prod.internal:5432/myapp"
          LOG_LEVEL: "debug"
      traits:
        - type: exposed
          properties:
            host: api.example.com
        - type: observable
        - type: scalable
          properties:
            minReplicas: 3
            maxReplicas: 20

    - name: worker
      type: worker
      properties:
        image: registry.example.com/my-service:v2.3.1
        concurrency: 10
      traits:
        - type: observable
        - type: scalable
```

---

## PlatformBundle (Extension)

The Platform concept is modeled as a **bundle of OAM primitives** (TraitDefinitions, PolicyDefinitions) plus metadata for the compiler.
```yaml
# Extension kind - NOT OAM standard
# But CONTAINS standard OAM kinds
apiVersion: oamx.example.dev/v1
kind: PlatformBundle
metadata:
  name: production-cluster
spec:
  description: "Production cluster platform configuration"

  # ─────────────────────────────────────────────────────────────
  # Standard OAM: TraitDefinitions
  # These define what traits are available and how they render
  # ─────────────────────────────────────────────────────────────
  traitDefinitions:
    # Trait: exposed (renders to Ingress with nginx annotations)
    - apiVersion: core.oam.dev/v1beta1
      kind: TraitDefinition
      metadata:
        name: exposed
      spec:
        appliesToWorkloads:
          - deployments.apps
          - statefulsets.apps
        schematic:
          cue:
            template: |
              outputs: {
                ingress: {
                  apiVersion: "networking.k8s.io/v1"
                  kind:       "Ingress"
                  metadata: {
                    name: context.name
                    annotations: {
                      "kubernetes.io/ingress.class":              "nginx"
                      "cert-manager.io/cluster-issuer":           "letsencrypt-prod"
                      "nginx.ingress.kubernetes.io/ssl-redirect": "true"
                    }
                  }
                  spec: {
                    ingressClassName: "nginx"
                    tls: [{
                      hosts: [parameter.host]
                      secretName: "\(context.name)-tls"
                    }]
                    rules: [{
                      host: parameter.host
                      http: paths: [{
                        path:     parameter.path
                        pathType: "Prefix"
                        backend: service: {
                          name: context.name
                          port: number: context.output.spec.template.spec.containers[0].ports[0].containerPort
                        }
                      }]
                    }]
                  }
                }
              }
              parameter: {
                host: string
                path: *"/" | string
              }

    # Trait: observable (renders to ServiceMonitor for prometheus-operator)
    - apiVersion: core.oam.dev/v1beta1
      kind: TraitDefinition
      metadata:
        name: observable
      spec:
        appliesToWorkloads:
          - deployments.apps
          - statefulsets.apps
        schematic:
          cue:
            template: |
              outputs: {
                serviceMonitor: {
                  apiVersion: "monitoring.coreos.com/v1"
                  kind:       "ServiceMonitor"
                  metadata: {
                    name: context.name
                    labels: {
                      prometheus: "main"
                    }
                  }
                  spec: {
                    selector: matchLabels: app: context.name
                    endpoints: [{
                      port:     "http"
                      path:     parameter.metricsPath
                      interval: parameter.interval
                    }]
                  }
                }
              }
              parameter: {
                metricsPath: *"/metrics" | string
                interval:    *"30s" | string
              }

    # Trait: scalable (renders to HPA)
    - apiVersion: core.oam.dev/v1beta1
      kind: TraitDefinition
      metadata:
        name: scalable
      spec:
        appliesToWorkloads:
          - deployments.apps
        schematic:
          cue:
            template: |
              outputs: {
                hpa: {
                  apiVersion: "autoscaling/v2"
                  kind:       "HorizontalPodAutoscaler"
                  metadata: name: context.name
                  spec: {
                    scaleTargetRef: {
                      apiVersion: "apps/v1"
                      kind:       "Deployment"
                      name:       context.name
                    }
                    minReplicas: parameter.minReplicas
                    maxReplicas: parameter.maxReplicas
                    metrics: [{
                      type: "Resource"
                      resource: {
                        name: "cpu"
                        target: {
                          type:               "Utilization"
                          averageUtilization: parameter.targetCPU
                        }
                      }
                    }]
                  }
                }
              }
              parameter: {
                minReplicas: *2 | int
                maxReplicas: *10 | int
                targetCPU:   *80 | int
              }

  # ─────────────────────────────────────────────────────────────
  # Standard OAM: PolicyDefinitions
  # ─────────────────────────────────────────────────────────────
  policyDefinitions:
    - apiVersion: core.oam.dev/v1beta1
      kind: PolicyDefinition
      metadata:
        name: namespace-placement
      spec:
        schematic:
          cue:
            template: |
              // Policy that sets namespace on all resources
              #PatchAll: {
                metadata: namespace: parameter.namespace
              }
              parameter: {
                namespace: *"applications" | string
              }

  # ─────────────────────────────────────────────────────────────
  # Extension: Compiler metadata (not OAM, but uses OAM info)
  # ─────────────────────────────────────────────────────────────
  compilerConfig:
    # Which CRDs exist on this cluster (affects trait availability)
    availableCRDs:
      - group: monitoring.coreos.com
        version: v1
        kind: ServiceMonitor
      - group: cert-manager.io
        version: v1
        kind: Certificate

    # Default values to inject
    defaults:
      namespace: applications
      imagePullSecrets:
        - name: registry-creds

    # Trait availability tiers (for staged rendering)
    traitTiers:
      builtin:
        - service
        - ingress
        - configmap
      requiresCRD:
        - observable    # Needs ServiceMonitor CRD
      requiresOperator:
        - tls-managed   # Needs cert-manager running
```

### Why This Structure?

| Aspect | How It's Modeled | Why |
|--------|------------------|-----|
| Trait implementations | Standard TraitDefinitions | OAM-compatible, can be used with KubeVela |
| Policy implementations | Standard PolicyDefinitions | OAM-compatible |
| Cluster capabilities | `compilerConfig.availableCRDs` | Extension, but references real K8s state |
| Defaults | `compilerConfig.defaults` | Extension for convenience |
| Staging hints | `compilerConfig.traitTiers` | Extension for GitOps rendering |

This keeps the **actual trait/policy definitions standard OAM**, while the bundle and compiler metadata are our extensions.

---

## Layered OAM: Compositional ComponentDefinitions

A key extension: ComponentDefinitions can reference other ComponentDefinitions, creating a layered composition model.

### Extension: OAM Schematic Type

Standard OAM schematics render to Kubernetes resources. We add a schematic type that renders to **more OAM**:
```yaml
schematic:
  # Standard: renders to K8s
  cue:
    template: |
      output: { apiVersion: "apps/v1", kind: "Deployment", ... }

  # Extension: renders to OAM components (recursive)
  oam:
    components:
      - type: some-other-component-definition
        properties: ...
```

This is **not standard OAM**. It's our extension for composition.

### Layer Structure
```
┌─────────────────────────────────────────────────────────────────┐
│  Application                                                    │
│  Standard OAM. References component types.                      │
│  Example: type: redis-sentinel                                  │
├─────────────────────────────────────────────────────────────────┤
│  Layer 2: Architectural Patterns (ComponentDefinitions)         │
│  Uses `schematic.oam` (extension) to compose Layer 1.           │
│  Examples: redis-standalone, redis-sentinel, redis-cluster      │
├─────────────────────────────────────────────────────────────────┤
│  Layer 1: Building Blocks (ComponentDefinitions)                │
│  Uses `schematic.oam` (extension) to compose Layer 0.           │
│  Examples: redis-node, sentinel-node, web-backend               │
├─────────────────────────────────────────────────────────────────┤
│  Layer 0: Primitives (ComponentDefinitions)                     │
│  Uses `schematic.cue` (standard) to render K8s resources.       │
│  Examples: stateless→Deployment, stateful→StatefulSet           │
└─────────────────────────────────────────────────────────────────┘
```

### Example: Redis Layers

**Layer 0: Primitive (Standard OAM)**
```yaml
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata:
  name: stateful
spec:
  workload:
    definition:
      apiVersion: apps/v1
      kind: StatefulSet
  schematic:
    cue:
      template: |
        output: {
          apiVersion: "apps/v1"
          kind:       "StatefulSet"
          metadata: name: context.name
          spec: {
            replicas:    parameter.replicas
            serviceName: context.name
            selector: matchLabels: app: context.name
            template: {
              metadata: labels: app: context.name
              spec: containers: [{
                name:    context.name
                image:   parameter.image
                ports:   [{containerPort: parameter.port}]
                command: parameter.command
                args:    parameter.args
              }]
            }
          }
        }
        parameter: {
          image:    string
          replicas: *1 | int
          port:     int
          command:  [...string] | *[]
          args:     [...string] | *[]
        }
```

**Layer 1: Building Block (Extension: oam schematic)**
```yaml
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata:
  name: redis-node
  annotations:
    oamx.example.dev/layer: "1"
    oamx.example.dev/schematic-type: "oam"  # Marks as extension
spec:
  schematic:
    # EXTENSION: This is not standard OAM
    oam:
      components:
        - name: "{{context.name}}"
          type: stateful
          properties:
            image: "redis:{{parameter.version}}"
            port: 6379
            replicas: parameter.replicas
            command: ["redis-server"]
            args:
              - "--requirepass"
              - "{{parameter.password}}"
      traits:
        - type: service-headless
        - type: persistent-volume
          properties:
            size: parameter.persistence.size

  # Parameter schema (standard OAM)
  schematic:
    cue:
      template: |
        parameter: {
          version:     *"7.2" | string
          replicas:    *1 | int
          password:    string
          persistence: size: *"10Gi" | string
        }
```

**Layer 2: Architectural Pattern (Extension)**
```yaml
apiVersion: core.oam.dev/v1beta1
kind: ComponentDefinition
metadata:
  name: redis-sentinel
  annotations:
    oamx.example.dev/layer: "2"
spec:
  schematic:
    oam:
      components:
        - name: redis
          type: redis-node
          properties:
            replicas: parameter.replicas
            password: parameter.password
            persistence:
              size: parameter.persistence.size

        - name: sentinel
          type: sentinel-node
          properties:
            masterHost: "{{context.name}}-redis-0.{{context.name}}-redis"
```

**Application (Standard OAM)**
```yaml
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata:
  name: my-cache
spec:
  components:
    - name: cache
      type: redis-sentinel   # Just pick the pattern
      properties:
        password: "${REDIS_PASSWORD}"
        replicas: 3
      traits:
        - type: observable   # Traits still work at any layer
```

### Recursion Termination

The compiler walks down until it hits a standard `cue` schematic:
```go
func (c *Compiler) ResolveComponent(comp Component) ([]KubeResource, error) {
    def := c.GetComponentDefinition(comp.Type)

    switch def.Spec.Schematic.Type {
    case "cue":
        // Terminal: render CUE to K8s resources
        return c.renderCUE(comp, def)

    case "oam":
        // Extension: recurse into nested OAM components
        var resources []KubeResource
        for _, nested := range def.Spec.Schematic.OAM.Components {
            resolved := c.substituteProperties(nested, comp.Properties)
            sub, err := c.ResolveComponent(resolved)
            if err != nil {
                return nil, err
            }
            resources = append(resources, sub...)
        }
        return resources, nil
    }
}
```

---

## Staged Rendering for GitOps

Rendering and applying are separate concerns:

- **Render time:** Pure transformation, no cluster access needed
- **Apply time:** Orchestration with dependency management

### Stage Assignment

Resources are assigned to stages based on their kind and dependencies:
```
Stage 0: CRDs
  - CustomResourceDefinitions
  - Wait condition: CRDs Established

Stage 1: Operators
  - Deployments for controllers
  - Wait condition: Deployments Available

Stage 2: Operator Resources
  - CRs managed by operators (ClusterIssuer, Prometheus, etc.)
  - Wait condition: Resources Ready

Stage 3: Platform Services
  - Services that depend on operators (Grafana, etc.)

Stage N: Applications
  - User applications
```

### Output Structure
```
output/
├── stage-0-crds/
│   ├── kustomization.yaml
│   └── *.yaml
├── stage-1-operators/
│   ├── kustomization.yaml
│   └── */
├── stage-2-operator-resources/
│   └── *.yaml
├── stage-3-platform-services/
│   └── */
├── applications/
│   └── my-service/
│       └── *.yaml
└── flux/
    └── kustomizations.yaml
```

### Key Insight: YAML Validity vs Reconciliation

CRs can be rendered before their operators exist. The YAML is valid Kubernetes — it just won't reconcile until the operator runs:
```yaml
# Valid YAML even without prometheus-operator
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: my-api
spec:
  selector:
    matchLabels:
      app: my-api
```

---

## Component Libraries

ComponentDefinitions can be packaged and distributed as libraries.

### Library Manifest
```yaml
apiVersion: oamx.example.dev/v1
kind: ComponentLibrary
metadata:
  name: redis
  version: 1.2.0
spec:
  description: "Redis deployment patterns for OAM"

  dependencies:
    - name: primitives
      source: builtin
      version: ">=1.0.0"

  # Standard OAM ComponentDefinitions
  componentDefinitions:
    - path: ./components/redis-node.yaml
    - path: ./components/sentinel-node.yaml
    - path: ./components/redis-standalone.yaml
    - path: ./components/redis-sentinel.yaml
    - path: ./components/redis-cluster.yaml

  # Standard OAM TraitDefinitions (if any)
  traitDefinitions: []
```

### Ecosystem
```
┌─────────────────────────────────────────────────────────────────┐
│  Your Application                                               │
│    type: redis-sentinel (from library)                          │
├─────────────────────────────────────────────────────────────────┤
│  Open Source Libraries                                          │
│    oam-components/redis                                         │
│    oam-components/postgresql                                    │
├─────────────────────────────────────────────────────────────────┤
│  Standard Library (builtin)                                     │
│    stateless, stateful, job, cronjob, daemonset                 │
└─────────────────────────────────────────────────────────────────┘
```

---

## CLI Interface

### Compilation Flow
```bash
# Step 1: Compile ApplicationTemplate + values → OAM Application
oamctl compile \
  -t ./my-service.apptemplate.yaml \
  -v ./values.yaml \
  -o ./application.yaml

# Step 2: Render OAM Application → Kubernetes YAML
oamctl render \
  -a ./application.yaml \
  -p ./platform-bundle.yaml \
  -o ./output/

# Or combined:
oamctl build \
  -t ./my-service.apptemplate.yaml \
  -v ./values.yaml \
  -p ./platform-bundle.yaml \
  -o ./output/ \
  --staged \
  --flux
```

### Commands

| Command | Description |
|---------|-------------|
| `compile` | ApplicationTemplate + values → OAM Application |
| `render` | OAM Application + PlatformBundle → Kubernetes YAML |
| `build` | Combined compile + render |
| `validate` | Check inputs without rendering |
| `explain` | Show what would be created |
| `diff` | Compare against live cluster |

---

## Design Principles

### 1. OAM Where Possible, Extensions Where Necessary

- Application, ComponentDefinition, TraitDefinition: **Standard OAM**
- ApplicationTemplate, PlatformBundle, layered schematics: **Clearly marked extensions**

### 2. Extensions Use OAM Primitives

PlatformBundle contains standard TraitDefinitions, not custom trait implementations.

### 3. Compilation Produces Standard OAM

ApplicationTemplate + values compiles to a **standard OAM Application** that could theoretically be used with KubeVela.

### 4. Simple Things Are Simple
```yaml
# Minimal application - no extensions needed
apiVersion: core.oam.dev/v1beta1
kind: Application
metadata:
  name: my-api
spec:
  components:
    - name: api
      type: webservice
      properties:
        image: myapp:v1
        env:
          DATABASE_URL: "postgres://..."
      traits:
        - type: exposed
          properties:
            host: api.example.com
```

### 5. Layered Composition Over Conditionals

Different architectures (standalone vs sentinel vs cluster) are different ComponentDefinition types, not conditionals within one type.

---

## Summary: What's OAM vs What's Extended

| Concept | OAM Standard? | Notes |
|---------|---------------|-------|
| Application | ✅ Yes | Components with properties and traits |
| ComponentDefinition | ✅ Yes | Schema via CUE, renders to K8s |
| TraitDefinition | ✅ Yes | Defines trait behavior |
| `schematic.cue` | ✅ Yes | Standard templating |
| `schematic.oam` | ❌ Extension | Layered composition |
| ApplicationTemplate | ❌ Extension | Maintainer packaging (like Helm chart) |
| PlatformBundle | ❌ Extension | Cluster capability bundle (contains standard TraitDefinitions) |
| ComponentLibrary | ❌ Extension | Package distribution |
| Staged rendering | ❌ Extension | GitOps integration |

---

## Glossary

| Term | Definition |
|------|------------|
| **Application** | OAM resource declaring components with properties and traits |
| **ComponentDefinition** | OAM resource defining a component type and its schema |
| **TraitDefinition** | OAM resource defining a trait and its schema |
| **ApplicationTemplate** | Extension: Maintainer packaging unit (like Helm chart) |
| **PlatformBundle** | Extension: Bundle of TraitDefinitions + cluster config |
| **Layer** | Level in ComponentDefinition composition hierarchy |
| **Primitive** | Layer 0 ComponentDefinition with `schematic.cue` (terminal) |
| **Stage** | Deployment phase based on resource dependencies |

---

## References

- [OAM Spec](https://github.com/oam-dev/spec)
- [KubeVela](https://kubevela.io/) - Reference OAM implementation
- [CUE Language](https://cuelang.org/)
- [FluxCD](https://fluxcd.io/)
- [ArgoCD](https://argoproj.github.io/cd/)
