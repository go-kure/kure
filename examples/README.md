# Examples

This directory contains examples for Kure, organized by purpose.

## Structure

### Reference Examples (docs site)

These directories have documentation mounted to the docs site:

- [`patches/`](patches/) — Declarative patching with TOML and YAML formats
- [`generators/`](generators/) — Resource generation using the GVK system
- [`kurel/frigate/`](kurel/frigate/) — Building a complete kurel package

### Go API Tutorial

- [`getting-started/`](getting-started/) — Complete cluster-to-disk pipeline using the Go API

### Demo Inputs

- [`demo/`](demo/) — YAML input configurations consumed by `cmd/demo`

## Running the Demo

```bash
make build-demo && ./bin/demo
```

The demo binary reads configs from `examples/demo/` and writes generated
manifests to `out/`. See [`demo/README.md`](demo/README.md) for details.
