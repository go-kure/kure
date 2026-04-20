+++
title = "CLI Reference"
weight = 50
+++

# CLI Reference

Kure provides one command-line tool.

## kure

The main CLI for Kubernetes resource generation.

```bash
kure [command] [flags]
```

### Commands

| Command | Description |
|---------|-------------|
| `generate` | Generate Kubernetes resources from configuration |
| `validate` | Validate resource configurations |
| `config` | Manage kure configuration |
| `version` | Print version information |
| `completion` | Generate shell completion scripts |

## Global Flags

kure supports:

| Flag | Description |
|------|-------------|
| `-v, --verbose` | Enable verbose output |
| `--debug` | Enable debug mode |
| `--strict` | Enable strict validation |
| `--config` | Configuration file path |
