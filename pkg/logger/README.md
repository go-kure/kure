# Logger - Logging Utilities

The `logger` package provides structured logging for Kure. All logging in the project should use this package instead of `fmt.Print` or the standard `log` package.

## Overview

The logger provides a simple interface for structured key-value logging with support for different log levels. It wraps an underlying structured logger and provides convenience functions.

## Usage

```go
import "github.com/go-kure/kure/pkg/logger"

// Default logger
log := logger.Default()

// Log with context
log.Info("loading package", "path", "/path/to/package")
log.Error("failed to parse", "error", err, "file", "config.yaml")

// No-op logger (for quiet mode)
log := logger.Noop()
```

## Log Levels

| Level | Usage |
|-------|-------|
| `Info` | Normal operational messages |
| `Error` | Error conditions |
| `Debug` | Detailed debugging information |
| `Warn` | Warning conditions |

## Conventions

- Use key-value pairs for structured data: `log.Info("msg", "key1", val1, "key2", val2)`
- Use `logger.Noop()` when verbose output is disabled
- Pass the logger through function parameters or options structs
- Use `logger.Default()` only at initialization points (CLI entry, tests)

## Related Packages

All Kure packages use this logger. See the [errors](../errors/) package for error handling patterns.
