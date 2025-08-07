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
	StrictMode   bool             // Treat warnings as errors
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
	// Output configuration
	Output       OutputDest   // Where to write (stdout, file, directory)
	OutputPath   string       // Output path for file/directory
	OutputFormat OutputFormat // How to organize files
	
	// Serialization
	Format      SerializationFormat // YAML or JSON
	PrettyPrint bool               // Pretty print JSON
	
	// File organization
	SeparateFiles    bool // Write each resource to its own file
	IncludeIndex     bool // Add numeric prefix to filenames
	IncludeNamespace bool // Add namespace to filenames
	
	// Filtering
	FilterKind      string // Only output resources of this kind
	FilterName      string // Only output resources with this name
	FilterNamespace string // Only output resources in this namespace
	
	// Transformations
	AddLabels      map[string]string // Add these labels to all resources
	AddAnnotations map[string]string // Add these annotations to all resources
}

// OutputDest defines where to write output
type OutputDest string

const (
	OutputStdout    OutputDest = "stdout"    // Write to stdout
	OutputFile      OutputDest = "file"      // Write to single file
	OutputDirectory OutputDest = "directory"  // Write to directory
)

// OutputFormat defines how to organize output files
type OutputFormat string

const (
	OutputFormatSingle     OutputFormat = "single"      // Single file
	OutputFormatByKind     OutputFormat = "by-kind"     // Group by resource kind
	OutputFormatByResource OutputFormat = "by-resource" // Separate file per resource
)

// SerializationFormat defines the output serialization format
type SerializationFormat string

const (
	FormatYAML SerializationFormat = "yaml" // YAML format
	FormatJSON SerializationFormat = "json" // JSON format
)

// OutputType is deprecated, use SerializationFormat instead
type OutputType = SerializationFormat

const (
	OutputTypeYAML OutputType = FormatYAML // Deprecated: use FormatYAML
	OutputTypeJSON OutputType = FormatJSON // Deprecated: use FormatJSON
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

