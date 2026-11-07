# Logger - Logging Utilities

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/logger.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/logger)

The `logger` package provides a small printf-style logger with four levels. The library packages do not import it: the only non-test importer in this repository is `examples/getting-started/main.go`. It is there for programs that build on Kure and want a logger to hand around.

## Overview

The logger provides a simple printf-style interface with support for different log levels. It wraps an underlying logger and provides convenience functions.

## Usage

`logger.New` takes `Options` for the output, the minimum level, a prefix and whether to print a
timestamp; `logger.Default()` is `logger.New(logger.DefaultOptions())`.

The block below is the body of an `Example` function in `example_test.go`, which `go test` runs:
it imports this package as `logger`, `github.com/go-kure/kure/pkg/errors`, and `os`.

<!-- doc-example: pkg/logger ExampleNew -->
```go
// logger.Default() writes to stderr with a timestamp and drops Debug; these
// options log every level to stdout without one.
log := logger.New(logger.Options{Output: os.Stdout, Level: logger.LevelDebug})
log.Info("loading package: %s", "/path/to/package")
log.Error("failed to parse %s: %v", "config.yaml", errors.New("unexpected end of stream"))
log.Debug("parsed %d resources", 3)

// No-op logger for quiet mode
log = logger.Noop()
log.Info("discarded")
```
<!-- doc-example:end -->

## Log Levels

From least to most severe. A logger drops every message below its minimum level; the default
minimum is `LevelInfo`.

| Level constant | Method | Usage |
|----------------|--------|-------|
| `LevelDebug` | `Debug` | Detailed debugging information |
| `LevelInfo` | `Info` | Normal operational messages |
| `LevelWarn` | `Warn` | Warning conditions |
| `LevelError` | `Error` | Error conditions |

## Prefixes, levels and helpers

- `WithPrefix(prefix)` returns a new logger whose prefix is the old prefix followed by `prefix`,
  and a trailing space is added if there is none.
- `SetLevel(level)` changes the minimum level of that logger.
- `logger.FormatBytes(n)` formats a byte count (`1536` becomes `1.5 KB`), and
  `logger.FormatDuration(nanos)` formats a duration given in nanoseconds (`1500000` becomes `1.50ms`).

## Conventions

- Use printf-style format strings: `log.Info("processed %d items in %s", n, name)`
- Use `logger.Noop()` when verbose output is disabled
- Pass the logger through function parameters or options structs
- Use `logger.Default()` only at initialization points (CLI entry, tests)

## Related Packages

See the [errors](/api-reference/errors/) package for error handling patterns.
