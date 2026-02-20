# CLI - Command-Line Interface Utilities

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/cli.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/cli)

The `cli` package provides foundational components for Kure's CLI tools (`kure` and `kurel`). It implements the Factory pattern for dependency injection, I/O stream abstraction, and output formatting.

## Overview

This package is used internally by the `pkg/cmd/` packages to build CLI commands. It provides a clean separation between command logic and I/O handling.

## Key Components

### Factory

Dependency injection container for CLI commands. Provides access to global options, I/O streams, and configuration.

```go
import "github.com/go-kure/kure/pkg/cli"

factory := cli.NewFactory(globalOpts)
streams := factory.IOStreams()
```

### IOStreams

Standard I/O stream abstraction for testable CLI commands:

```go
// Default streams (stdin/stdout/stderr)
streams := cli.NewIOStreams()

// Use in commands
fmt.Fprintln(streams.Out, "output")
fmt.Fprintln(streams.ErrOut, "error")
```

Fields:
- `In io.Reader` - Standard input
- `Out io.Writer` - Standard output
- `ErrOut io.Writer` - Standard error

### Printer

Output formatting for CLI commands with support for text, YAML, and JSON formats:

```go
printer := cli.NewPrinter(cli.PrintOptions{
    Format: "yaml",
})
```

### Config

Configuration file handling for CLI tools:

```go
config, err := cli.LoadConfig("~/.kure/config.yaml")
```

## Related Packages

- [pkg/cmd/kure](/api-reference/cli/) - kure CLI command implementation
- [pkg/cmd/kurel](https://pkg.go.dev/github.com/go-kure/kure/pkg/cmd/kurel) - kurel CLI command implementation
- [io](../io/) - Resource printing and serialization
