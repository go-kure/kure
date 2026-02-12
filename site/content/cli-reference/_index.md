+++
title = "CLI Reference"
weight = 50
+++

# CLI Reference

Kure provides two command-line tools.

## kure

The main CLI for Kubernetes resource generation.

```bash
kure [command] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `generate` | Generate Kubernetes resources from configuration |
| `patch` | Apply patches to existing manifests |
| `validate` | Validate resource configurations |
| `config` | Manage kure configuration |
| `version` | Print version information |
| `completion` | Generate shell completion scripts |

## kurel

The package system CLI for building and managing reusable application packages.

```bash
kurel [command] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `build` | Build Kubernetes manifests from a kurel package |
| `validate` | Validate kurel package structure and configuration |
| `info` | Show package information |
| `schema` | Schema generation and validation commands |
| `config` | Manage kurel configuration |
| `version` | Print version information |
| `completion` | Generate shell completion scripts |

### kurel build

```bash
kurel build <package> [flags]
```

| Flag | Description |
|------|-------------|
| `-o, --output` | Output path (default: stdout) |
| `--values` | Values file for parameter overrides |
| `-p, --patch` | Enable specific patches |
| `--format` | Output format: yaml, json (default: yaml) |
| `--kind` | Filter by resource kind |
| `--name` | Filter by resource name |
| `--add-label` | Add labels to all resources |

### kurel validate

```bash
kurel validate <package> [flags]
```

| Flag | Description |
|------|-------------|
| `--values` | Values file for validation |
| `--schema` | Custom schema file |
| `--json` | Output validation results as JSON |

### kurel info

```bash
kurel info <package> [flags]
```

| Flag | Description |
|------|-------------|
| `-o, --output` | Output format: text, yaml, json (default: text) |
| `--all` | Show all details including resource content |

## Global Flags

Both tools support:

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable verbose output |
| `--debug` | Enable debug mode |
| `--strict` | Enable strict validation |
| `--config` | Configuration file path |
