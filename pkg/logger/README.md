# Logger - Logging Utilities

[![Go Reference](https://pkg.go.dev/badge/github.com/go-kure/kure/pkg/logger.svg)](https://pkg.go.dev/github.com/go-kure/kure/pkg/logger)

The `logger` package provides structured logging for Kure. All logging in the project should use this package instead of `fmt.Print` or the standard `log` package.

## Overview

The logger provides a simple printf-style interface with support for different log levels. It wraps an underlying logger and provides convenience functions.

## Usage

```go
import "github.com/go-kure/kure/pkg/logger"

log := logger.Default()
log.Info("loading package: %s", "/path/to/package")
log.Error("failed to parse %s: %v", "config.yaml", err)
log.Debug("parsed %d resources", count)

// No-op logger for quiet mode
log = logger.Noop()
```

## Log Levels

| Level | Usage |
|-------|-------|
| `Info` | Normal operational messages |
| `Error` | Error conditions |
| `Debug` | Detailed debugging information |
| `Warn` | Warning conditions |

## Conventions

- Use printf-style format strings: `log.Info("processed %d items in %s", n, name)`
- Use `logger.Noop()` when verbose output is disabled
- Pass the logger through function parameters or options structs
- Use `logger.Default()` only at initialization points (CLI entry, tests)

## Related Packages

All Kure packages use this logger. See the [errors](/api-reference/errors/) package for error handling patterns.
