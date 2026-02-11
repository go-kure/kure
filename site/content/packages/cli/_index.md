+++
title = "CLI"
weight = 70
+++

# CLI Package

The cli package provides shared utilities and abstractions for building command-line interfaces in the Kure and kurel tools.

## Components

- **Factory** — Dependency injection container for CLI commands, making them easier to test and configure
- **IOStreams** — Abstraction for stdin/stdout/stderr, enabling testable commands
- **Printer** — Output formatting supporting YAML, JSON, table, wide, and name formats (compatible with kubectl conventions)
- **Config** — Configuration file handling with merging from defaults, config files, environment variables, and flags

## API Reference

- [pkg.go.dev/github.com/go-kure/kure/pkg/cli](https://pkg.go.dev/github.com/go-kure/kure/pkg/cli)
