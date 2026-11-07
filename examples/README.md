# Examples

This directory contains examples for Kure, organized by purpose.

## Structure

### Reference Examples (docs site)

These directories have documentation mounted to the docs site:

- [`getting-started/`](getting-started/) — Complete cluster-to-disk pipeline using the Go API
- [`patches/`](patches/) — Declarative patching with TOML and YAML formats

### Not on the docs site

- [`kurel/frigate/`](kurel/frigate/) — A sample package for the `kurel` CLI, which now lives in
  [go-kure/launcher](https://github.com/go-kure/launcher). Its README is a deprecation notice.
- [`demo/`](demo/) — Sample YAML files shaped like cluster, bootstrap and workload definitions.
  Kure has no command that reads them; they are kept as sample data.
