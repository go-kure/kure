+++
title = "Building Kurel Packages"
weight = 50
+++

# Building Kurel Packages

Kurel is the package system for creating reusable Kubernetes applications. A kurel package bundles base manifests, patches, and parameters into a self-contained unit that can be customized per deployment.

## Package Structure

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
│   │   └── 10-monitoring.yaml   # Patch conditions
│   └── profiles/
│       ├── 10-dev.kpatch
│       └── 20-prod.kpatch
└── README.md
```

## Creating a Package

### 1. Define Parameters

```yaml
# parameters.yaml
kurel:
  name: my-application
  version: 1.0.0
  description: "A sample application package"

app:
  replicas: 3
  image:
    repository: myapp
    tag: v1.0.0

monitoring:
  enabled: false
```

### 2. Add Base Resources

Place standard Kubernetes manifests in `resources/`. These are the starting point before patches are applied.

### 3. Write Patches

Patches customize the base resources. See the [Patching guide](patching) for the TOML patch format.

### 4. Add Conditional Patches

Control when patches are applied:

```yaml
# patches/features/10-monitoring.yaml
enabled: "${monitoring.enabled}"
description: "Adds Prometheus monitoring"
requires:
  - "features/05-metrics-base.kpatch"
```

## Building and Deploying

```bash
# Validate the package
kurel validate my-app.kurel/

# Build with custom values
kurel build my-app.kurel/ \
  --values production.yaml \
  --output ./manifests/

# Show package information
kurel info my-app.kurel/
```

## Multi-Phase Deployment

Annotate resources to control deployment ordering:

```yaml
metadata:
  annotations:
    kurel.gokure.dev/install-phase: "pre-install"  # or "main", "post-install"
```

This generates separate phase directories with proper dependencies for GitOps deployment.

## User Extensions

Extend packages without modifying them using `.local.kurel`:

```
my-app.local.kurel/
├── parameters.yaml          # Override parameters
└── patches/
    └── 50-custom.kpatch    # Add custom patches
```

## Further Reading

- [Launcher reference](/api-reference/launcher) for the package system API
- [Kurel Frigate example](/examples/kurel-frigate) for a complete package
- [Patching guide](patching) for the patch format
