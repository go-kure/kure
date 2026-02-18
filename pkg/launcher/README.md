# Launcher - Kubernetes Resources Launcher

This package provides the core functionality for Kurel, the Kubernetes Resources Launcher CLI tool.

Kurel is a package system for creating reusable, customizable Kubernetes applications without the complexity of templating engines. It uses a declarative patch-based approach to customize base Kubernetes manifests, making it perfect for GitOps workflows.

## ✨ Key Features

- **📦 Package-based** - Encapsulate applications in reusable `.kurel` packages
- **🎯 No Templating** - Use patches instead of complex template syntax  
- **🔧 Declarative Customization** - Simple parameter-driven configuration
- **🚀 GitOps Native** - Generate clean Kubernetes manifests for Flux/ArgoCD
- **📊 Schema Validation** - Auto-generated schemas with Kubernetes API integration
- **🏗️ Multi-Phase Deployment** - Support for ordered deployment phases
- **🌐 Multi-Namespace** - Deploy across multiple namespaces seamlessly
- **🎨 User Extensions** - Extend packages without modifying originals

## 🚀 Quick Start

### Installing a Package

```bash
# Download a kurel package (example)
git clone https://github.com/example/prometheus-operator.kurel

# Validate the package
kurel validate prometheus-operator.kurel/

# Customize with your parameters
cat > my-values.yaml << EOF
monitoring:
  enabled: true
  retention: 7d
persistence:
  enabled: true
  size: 50Gi
resources:
  requests:
    cpu: 200m
    memory: 512Mi
EOF

# Generate manifests
kurel build prometheus-operator.kurel/ \
  --values my-values.yaml \
  --output ./manifests/
```

### Using with GitOps

```bash
# Generated structure is GitOps-ready
ls manifests/
# pre-install/    - CRDs, namespaces, RBAC
# main/          - Main application (depends on pre-install)  
# post-install/  - Monitoring, backups (depends on main)

# Each phase includes kustomization.yaml with proper dependencies
cat manifests/main/kustomization.yaml
# apiVersion: kustomize.config.k8s.io/v1beta1
# kind: Kustomization
# dependsOn:
#   - name: prometheus-pre-install
# resources: [...]
```

## 📁 Package Structure

A kurel package is a directory with this structure:

```
my-app.kurel/
├── parameters.yaml          # Variables and package metadata
├── resources/               # Base Kubernetes manifests
│   ├── deployment.yaml
│   ├── service.yaml
│   └── namespace.yaml
├── patches/                 # Modular customization patches
│   ├── 00-base.kpatch      # Global settings
│   ├── features/
│   │   ├── 10-monitoring.kpatch
│   │   ├── 10-monitoring.yaml   # Patch conditions
│   │   └── 20-ingress.kpatch
│   └── profiles/
│       ├── 10-dev.kpatch
│       └── 20-prod.kpatch
└── README.md               # Package documentation
```

## ⚙️ Configuration

### Parameters File

The `parameters.yaml` file contains all configurable options:

```yaml
# Package metadata
kurel:
  name: my-application
  version: 1.0.0
  description: "A sample application package"

# Global defaults applied to all resources
global:
  labels:
    app.kubernetes.io/managed-by: kurel
  resources:
    requests:
      cpu: 100m
      memory: 128Mi

# Feature flags
monitoring:
  enabled: false           # Enable monitoring patches
  retention: 30d

# Application settings  
app:
  replicas: 3
  image:
    registry: docker.io
    repository: myapp
    tag: v1.0.0
```

### Patch System

Patches use simple TOML syntax to modify resources:

```toml
# patches/features/10-monitoring.kpatch
[deployment.myapp.spec.template.spec]
securityContext.runAsNonRoot: true

[deployment.myapp.spec.template.spec.containers.0]
resources: "${global.resources}"
image: "${app.image.registry}/${app.image.repository}:${app.image.tag}"

# Add monitoring sidecar
[deployment.myapp.spec.template.spec.containers.-]
name: "metrics-exporter"
image: "prom/node-exporter:latest"
ports:
  - containerPort: 9100
    name: "metrics"
```

### Conditional Patches

Control when patches are applied:

```yaml
# patches/features/10-monitoring.yaml  
enabled: "${monitoring.enabled}"      # Only apply if monitoring enabled
description: "Adds Prometheus monitoring"
requires:                            # Auto-enable these patches
  - "features/05-metrics-base.kpatch"
conflicts:                           # Cannot be used with these
  - "features/20-simple-monitoring.kpatch"
```

## 🔧 User Customization

### Local Extensions

Extend packages without modifying them using `.local.kurel`:

```
my-app.local.kurel/
├── parameters.yaml          # Override parameters
└── patches/
    └── 50-custom.kpatch    # Add custom patches
```

### Example Local Override

```yaml
# my-app.local.kurel/parameters.yaml
monitoring:
  enabled: true             # Enable monitoring
  retention: 7d            # Shorter retention

app:
  replicas: 5              # More replicas for production
```

## 🏗️ Multi-Phase Deployment

Support complex applications that need ordered deployment:

```yaml
# In your Kubernetes resources
apiVersion: v1
kind: Namespace
metadata:
  name: myapp
  annotations:
    kurel.gokure.dev/install-phase: "pre-install"

---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp-server
  annotations:
    kurel.gokure.dev/install-phase: "main"
    
---
apiVersion: v1
kind: Service  
metadata:
  name: myapp-monitor
  annotations:
    kurel.gokure.dev/install-phase: "post-install"
```

This generates three separate phases with proper dependencies for GitOps deployment.

## 🛠️ CLI Commands

### Validation
```bash
# Validate package structure and parameters
kurel validate my-app.kurel/ --values my-values.yaml

# Output shows enabled patches and validation results
# ✓ Package structure valid
# ✓ All variables resolved  
# Enabled patches:
#   ✓ patches/00-base.kpatch
#   ✓ patches/features/10-monitoring.kpatch (monitoring.enabled=true)
#   → patches/features/05-metrics-base.kpatch (auto-enabled)
```

### Schema Generation
```bash
# Generate validation schemas
kurel schema generate my-app.kurel/

# Creates schemas/parameters.schema.json with:
# - Type information from parameter values
# - Kubernetes validation rules where traceable
# - Custom validation patterns
```

### Building Manifests
```bash
# Generate final Kubernetes manifests
kurel build my-app.kurel/ \
  --values production.yaml \
  --output ./deploy/

# Generates phase-based directory structure:
# deploy/pre-install/   - CRDs, namespaces, RBAC
# deploy/main/         - Main application  
# deploy/post-install/ - Monitoring, cleanup
```

### Package Information
```bash
# Show package details
kurel info my-app.kurel/

# Package: my-application v1.0.0
# Description: A sample application package
# Patches: 8 total, 3 conditional
# Variables: 12 configurable parameters
# Phases: pre-install, main, post-install
```

## 🌟 Common Use Cases

### Platform Teams
Create standardized application packages with defined customization boundaries:

```yaml
# Standard web app package
kurel:
  name: webapp-standard
  version: 2.1.0

global:
  securityContext:
    runAsNonRoot: true
    fsGroup: 1000
  
# Teams can only customize these parameters
app:
  replicas: 3
  domain: ""              # Required override
  
monitoring:
  enabled: true           # Always enabled for compliance
```

### Multi-Environment Deployments
Same package, different configurations:

```bash
# Development
kurel build webapp.kurel/ --values environments/dev.yaml

# Staging  
kurel build webapp.kurel/ --values environments/staging.yaml

# Production
kurel build webapp.kurel/ --values environments/prod.yaml
```

### Complex Applications
Multi-namespace applications with dependencies:

```yaml
# Database in its own namespace (pre-install)
# Application in app namespace (main) 
# Monitoring in monitoring namespace (post-install)
global:
  namespaces:
    create: true
    
database:
  namespace: database
  
app:
  namespace: application
  
monitoring:
  namespace: monitoring
```

## 🤝 Contributing

Kurel is part of the [Kure](https://github.com/go-kure/kure) project. See the main repository for contribution guidelines.

## 📚 Documentation

- [Design Specification](https://github.com/go-kure/kure/blob/main/pkg/launcher/DESIGN.md) - Technical design and architecture
- [Detailed Design Document](https://github.com/go-kure/kure/blob/main/pkg/launcher/DESIGN-DETAILS.md) - Complete design discussion and decisions
- [Kure Documentation](https://github.com/go-kure/kure#readme) - Main project documentation

## 🎯 Philosophy

Kurel follows the principle **"kurel just generates YAML"**. It's not a runtime system or complex orchestrator - it's a focused tool that generates clean, validated Kubernetes manifests for your GitOps workflows.

This approach gives you:
- **Predictable output** - Same inputs always generate same manifests
- **GitOps compatibility** - Standard Kubernetes YAML that any tool can deploy
- **Debugging simplicity** - Generated manifests are human-readable
- **Tool independence** - Not locked into specific deployment tools