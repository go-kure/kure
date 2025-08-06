// Package logger provides a structured logging interface for the Kure library.
// It supports different log levels (Debug, Info, Warn, Error) and can be
// configured with verbosity settings.
package logger

import (
	"fmt"
	"io"
	"log"
	"os"
)

// Level represents the severity level of a log message
type Level int

const (
	// LevelDebug is for detailed debugging information
	LevelDebug Level = iota
	// LevelInfo is for informational messages
	LevelInfo
	// LevelWarn is for warning messages
	LevelWarn
	// LevelError is for error messages
	LevelError
)

// Logger is the interface for structured logging in Kure
type Logger interface {
	// Debug logs a debug message (only shown in verbose/debug mode)
	Debug(format string, args ...interface{})
	// Info logs an informational message
	Info(format string, args ...interface{})
	// Warn logs a warning message
	Warn(format string, args ...interface{})
	// Error logs an error message
	Error(format string, args ...interface{})
	// WithPrefix returns a new logger with an additional prefix
	WithPrefix(prefix string) Logger
	// SetLevel sets the minimum log level
	SetLevel(level Level)
}

// Options configures a logger instance
type Options struct {
	// Output is where logs are written (default: os.Stderr)
	Output io.Writer
	// Level is the minimum log level (default: LevelInfo)
	Level Level
	// Prefix is the log prefix (default: empty)
	Prefix string
	// ShowTimestamp indicates whether to include timestamps (default: true)
	ShowTimestamp bool
}

// DefaultOptions returns the default logger options
func DefaultOptions() Options {
	return Options{
		Output:        os.Stderr,
		Level:         LevelInfo,
		Prefix:        "",
		ShowTimestamp: true,
	}
}

// defaultLogger is the standard implementation of Logger
type defaultLogger struct {
	logger *log.Logger
	level  Level
	prefix string
}

// New creates a new logger with the given options
func New(opts Options) Logger {
	flags := 0
	if opts.ShowTimestamp {
		flags = log.LstdFlags
	}
	
	prefix := opts.Prefix
	if prefix != "" && prefix[len(prefix)-1] != ' ' {
		prefix += " "
	}

	return &defaultLogger{
		logger: log.New(opts.Output, prefix, flags),
		level:  opts.Level,
		prefix: prefix,
	}
}

// Default creates a logger with default options
func Default() Logger {
	return New(DefaultOptions())
}

// Debug logs a debug message
func (l *defaultLogger) Debug(format string, args ...interface{}) {
	if l.level <= LevelDebug {
		l.logger.Printf("[DEBUG] "+format, args...)
	}
}

// Info logs an info message
func (l *defaultLogger) Info(format string, args ...interface{}) {
	if l.level <= LevelInfo {
		l.logger.Printf("[INFO] "+format, args...)
	}
}

// Warn logs a warning message
func (l *defaultLogger) Warn(format string, args ...interface{}) {
	if l.level <= LevelWarn {
		l.logger.Printf("[WARN] "+format, args...)
	}
}

// Error logs an error message
func (l *defaultLogger) Error(format string, args ...interface{}) {
	if l.level <= LevelError {
		l.logger.Printf("[ERROR] "+format, args...)
	}
}

// WithPrefix returns a new logger with an additional prefix
func (l *defaultLogger) WithPrefix(prefix string) Logger {
	newPrefix := l.prefix + prefix
	if newPrefix != "" && newPrefix[len(newPrefix)-1] != ' ' {
		newPrefix += " "
	}
	
	return &defaultLogger{
		logger: log.New(l.logger.Writer(), newPrefix, l.logger.Flags()),
		level:  l.level,
		prefix: newPrefix,
	}
}

// SetLevel sets the minimum log level
func (l *defaultLogger) SetLevel(level Level) {
	l.level = level
}

// noopLogger is a logger that does nothing (useful for testing)
type noopLogger struct{}

// Noop returns a logger that discards all messages
func Noop() Logger {
	return &noopLogger{}
}

func (l *noopLogger) Debug(format string, args ...interface{}) {}
func (l *noopLogger) Info(format string, args ...interface{})  {}
func (l *noopLogger) Warn(format string, args ...interface{})  {}
func (l *noopLogger) Error(format string, args ...interface{}) {}
func (l *noopLogger) WithPrefix(prefix string) Logger          { return l }
func (l *noopLogger) SetLevel(level Level)                     {}

// Helper function for formatting byte sizes
func FormatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// Helper function to format durations in a human-readable way
func FormatDuration(nanos int64) string {
	if nanos < 1000 {
		return fmt.Sprintf("%dns", nanos)
	} else if nanos < 1000000 {
		return fmt.Sprintf("%.2fµs", float64(nanos)/1000)
	} else if nanos < 1000000000 {
		return fmt.Sprintf("%.2fms", float64(nanos)/1000000)
	}
	return fmt.Sprintf("%.2fs", float64(nanos)/1000000000)
}