package launcher

import (
	"runtime"
	"time"

	"github.com/go-kure/kure/pkg/logger"
)

// LauncherOptions centralizes common configuration for all launcher components
type LauncherOptions struct {
	Logger       logger.Logger    // Logger instance
	MaxDepth     int              // Maximum variable resolution depth
	Timeout      time.Duration    // Operation timeout
	MaxWorkers   int              // Number of concurrent workers
	CacheDir     string           // Directory for caching schemas
	Debug        bool             // Enable debug output
	Verbose      bool             // Enable verbose logging
	ProgressFunc func(string)     // Progress callback function
}

// DefaultOptions returns sensible default options
func DefaultOptions() *LauncherOptions {
	return &LauncherOptions{
		Logger:     logger.Default(),
		MaxDepth:   10,
		Timeout:    30 * time.Second,
		MaxWorkers: runtime.NumCPU(),
		CacheDir:   "/tmp/kurel-cache",
		Debug:      false,
		Verbose:    false,
	}
}

// WithLogger sets the logger
func (o *LauncherOptions) WithLogger(l logger.Logger) *LauncherOptions {
	o.Logger = l
	return o
}

// WithTimeout sets the timeout
func (o *LauncherOptions) WithTimeout(timeout time.Duration) *LauncherOptions {
	o.Timeout = timeout
	return o
}

// WithDebug enables debug mode
func (o *LauncherOptions) WithDebug(debug bool) *LauncherOptions {
	o.Debug = debug
	return o
}

// WithVerbose enables verbose mode
func (o *LauncherOptions) WithVerbose(verbose bool) *LauncherOptions {
	o.Verbose = verbose
	return o
}

// BuildOptions configures the build process
type BuildOptions struct {
	OutputPath   string       // Output path (default: stdout)
	OutputFormat OutputFormat // Output format (single, by-kind, by-resource)
	OutputType   OutputType   // Output type (yaml, json)
}

// OutputFormat defines how to organize output files
type OutputFormat string

const (
	OutputFormatSingle     OutputFormat = "single"      // Single file
	OutputFormatByKind     OutputFormat = "by-kind"     // Group by resource kind
	OutputFormatByResource OutputFormat = "by-resource" // Separate file per resource
)

// OutputType defines the output serialization format
type OutputType string

const (
	OutputTypeYAML OutputType = "yaml" // YAML format
	OutputTypeJSON OutputType = "json" // JSON format
)

// ValidationResult contains validation errors and warnings
type ValidationResult struct {
	Errors   []ValidationError   `json:"errors,omitempty"`
	Warnings []ValidationWarning `json:"warnings,omitempty"`
}

// HasErrors returns true if there are any errors
func (r ValidationResult) HasErrors() bool {
	return len(r.Errors) > 0
}

// HasWarnings returns true if there are any warnings
func (r ValidationResult) HasWarnings() bool {
	return len(r.Warnings) > 0
}

// IsValid returns true if there are no errors
func (r ValidationResult) IsValid() bool {
	return !r.HasErrors()
}

// ValidationError represents a validation error that blocks processing
type ValidationError struct {
	Resource string `json:"resource,omitempty"`
	Field    string `json:"field,omitempty"`
	Path     string `json:"path,omitempty"`     // JSON path to the field
	Message  string `json:"message"`
	Severity string `json:"severity,omitempty"` // "error" or "warning"
}

// Error implements the error interface
func (e ValidationError) Error() string {
	if e.Resource != "" && e.Field != "" {
		return e.Resource + "." + e.Field + ": " + e.Message
	}
	if e.Resource != "" {
		return e.Resource + ": " + e.Message
	}
	if e.Field != "" {
		return e.Field + ": " + e.Message
	}
	return e.Message
}

// ValidationWarning represents a non-blocking validation issue
type ValidationWarning struct {
	Resource string `json:"resource,omitempty"`
	Field    string `json:"field,omitempty"`
	Message  string `json:"message"`
}

// String returns the warning message
func (w ValidationWarning) String() string {
	if w.Resource != "" && w.Field != "" {
		return w.Resource + "." + w.Field + ": " + w.Message
	}
	if w.Resource != "" {
		return w.Resource + ": " + w.Message
	}
	if w.Field != "" {
		return w.Field + ": " + w.Message
	}
	return w.Message
}

