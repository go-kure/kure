package launcher

import (
	stderrors "errors"
	"fmt"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
)

// LoadErrors represents errors during package loading with partial results
// It implements the KureError interface for structured error handling
type LoadErrors struct {
	*errors.BaseError
	PartialDefinition *PackageDefinition
	Issues            []error
}

// NewLoadErrors creates a new LoadErrors with partial results
func NewLoadErrors(partial *PackageDefinition, issues []error) *LoadErrors {
	message := "package loaded with issues"
	if len(issues) > 0 {
		messages := make([]string, len(issues))
		for i, err := range issues {
			messages[i] = err.Error()
		}
		message = fmt.Sprintf("package loaded with %d issues: %s",
			len(issues), strings.Join(messages, "; "))
	}

	return &LoadErrors{
		BaseError: &errors.BaseError{
			ErrType: errors.ErrorTypeParse,
			Message: message,
			Help:    "Fix the reported issues to fully load the package",
			ErrContext: map[string]interface{}{
				"issueCount": len(issues),
			},
		},
		PartialDefinition: partial,
		Issues:            issues,
	}
}

// Unwrap returns the underlying errors
func (e *LoadErrors) Unwrap() []error {
	return e.Issues
}

// HasCriticalErrors returns true if any critical errors prevent usage
func (e *LoadErrors) HasCriticalErrors() bool {
	for _, err := range e.Issues {
		if IsCriticalError(err) {
			return true
		}
	}
	return false
}

// PatchError represents an error during patch processing
// Uses the existing PatchError from pkg/errors but adds launcher-specific fields
type PatchError struct {
	*errors.PatchError
	PatchName    string
	ResourceKind string
	ResourceName string
	TargetPath   string
	Reason       string
}

// NewPatchError creates a patch processing error
func NewPatchError(patchName, resourceKind, resourceName, targetPath, reason string) *PatchError {
	resourceFull := resourceKind
	if resourceName != "" {
		resourceFull = fmt.Sprintf("%s/%s", resourceKind, resourceName)
	}

	return &PatchError{
		PatchError: errors.NewPatchError(
			"apply",
			targetPath,
			resourceFull,
			reason,
			nil,
		),
		PatchName:    patchName,
		ResourceKind: resourceKind,
		ResourceName: resourceName,
		TargetPath:   targetPath,
		Reason:       reason,
	}
}

// DependencyError represents a dependency resolution error
type DependencyError struct {
	*errors.ConfigError
	DepType  string   // "circular", "missing", "conflict"
	Source   string   // Source of the dependency
	Target   string   // Target of the dependency
	Chain    []string // Dependency chain for circular dependencies
}

// NewDependencyError creates a dependency resolution error
func NewDependencyError(depType, source, target string, chain []string) *DependencyError {
	var message string
	var help string
	
	switch depType {
	case "circular":
		message = fmt.Sprintf("circular dependency: %s", strings.Join(chain, " -> "))
		help = "Remove or reorganize dependencies to break the cycle"
	case "missing":
		message = fmt.Sprintf("%s requires non-existent %s", source, target)
		help = "Ensure all required dependencies are available"
	case "conflict":
		message = fmt.Sprintf("%s conflicts with %s", source, target)
		help = "Resolve conflicting dependencies or use patch ordering"
	default:
		message = fmt.Sprintf("dependency error: %s -> %s", source, target)
		help = "Check dependency configuration"
	}

	return &DependencyError{
		ConfigError: &errors.ConfigError{
			BaseError: &errors.BaseError{
				ErrType: errors.ErrorTypeConfiguration,
				Message: message,
				Help:    help,
				ErrContext: map[string]interface{}{
					"type":   depType,
					"source": source,
					"target": target,
					"chain":  chain,
				},
			},
			Source: "dependencies",
			Field:  source,
		},
		DepType: depType,
		Source:  source,
		Target:  target,
		Chain:   chain,
	}
}

// VariableError represents a variable resolution error
type VariableError struct {
	*errors.ValidationError
	Variable   string
	Expression string
	Reason     string
}

// NewVariableError creates a variable resolution error
func NewVariableError(variable, expression, reason string) *VariableError {
	message := fmt.Sprintf("variable %s: %s", variable, reason)
	if expression != "" {
		message = fmt.Sprintf("variable %s: failed to resolve '%s': %s",
			variable, expression, reason)
	}

	return &VariableError{
		ValidationError: &errors.ValidationError{
			BaseError: &errors.BaseError{
				ErrType: errors.ErrorTypeValidation,
				Message: message,
				Help:    "Check variable definition and expression syntax",
				ErrContext: map[string]interface{}{
					"variable":   variable,
					"expression": expression,
					"reason":     reason,
				},
			},
			Field:     variable,
			Value:     expression,
			Component: "launcher",
		},
		Variable:   variable,
		Expression: expression,
		Reason:     reason,
	}
}

// SchemaError represents a schema validation error
type SchemaError struct {
	*errors.ValidationError
	Path     string      // JSON path to the field
	Value    interface{} // Actual value
	Expected string      // Expected type or constraint
	Message  string      // Error message
}

// NewSchemaError creates a schema validation error
func NewSchemaError(path string, value interface{}, expected, message string) *SchemaError {
	fullMessage := fmt.Sprintf("schema validation: %s", message)
	if path != "" {
		fullMessage = fmt.Sprintf("schema validation at %s: %s (got %v, expected %s)",
			path, message, value, expected)
	}

	return &SchemaError{
		ValidationError: &errors.ValidationError{
			BaseError: &errors.BaseError{
				ErrType: errors.ErrorTypeValidation,
				Message: fullMessage,
				Help:    fmt.Sprintf("Expected %s", expected),
				ErrContext: map[string]interface{}{
					"path":     path,
					"value":    value,
					"expected": expected,
				},
			},
			Field:     path,
			Value:     fmt.Sprintf("%v", value),
			Component: "launcher-schema",
		},
		Path:     path,
		Value:    value,
		Expected: expected,
		Message:  message,
	}
}

// SizeError represents a package size violation
type SizeError struct {
	*errors.ValidationError
	ActualSize int64
	MaxSize    int64
	SizeType   string // "package", "resource", "patch"
}

// NewSizeError creates a size validation error
func NewSizeError(sizeType string, actualSize, maxSize int64) *SizeError {
	return &SizeError{
		ValidationError: &errors.ValidationError{
			BaseError: &errors.BaseError{
				ErrType: errors.ErrorTypeValidation,
				Message: fmt.Sprintf("%s size %d exceeds maximum %d bytes",
					sizeType, actualSize, maxSize),
				Help: fmt.Sprintf("Reduce %s size below %d bytes", sizeType, maxSize),
				ErrContext: map[string]interface{}{
					"type":       sizeType,
					"actualSize": actualSize,
					"maxSize":    maxSize,
				},
			},
			Field:     "size",
			Value:     fmt.Sprintf("%d", actualSize),
			Component: "launcher",
		},
		ActualSize: actualSize,
		MaxSize:    maxSize,
		SizeType:   sizeType,
	}
}

// IsCriticalError determines if an error is critical (prevents package usage)
func IsCriticalError(err error) bool {
	if err == nil {
		return false
	}

	// Check if it's a KureError with specific types
	if kErr := errors.GetKureError(err); kErr != nil {
		// File and parse errors are usually critical
		if errors.IsType(err, errors.ErrorTypeFile) ||
			errors.IsType(err, errors.ErrorTypeParse) {
			return true
		}
	}

	// Check for critical error types
	var sizeErr *SizeError
	if As(err, &sizeErr) {
		return true
	}

	// Check for specific critical error patterns
	if fileErr, ok := err.(*errors.FileError); ok {
		if fileErr.Operation == "read" || fileErr.Operation == "load" {
			return true
		}
	}

	return false
}

// IsWarning determines if an error should be treated as a warning
func IsWarning(err error) bool {
	if err == nil {
		return false
	}

	// Configuration errors are often warnings
	if errors.IsType(err, errors.ErrorTypeConfiguration) {
		return true
	}

	// Non-critical errors are warnings
	return !IsCriticalError(err)
}

// Helper function to wrap standard errors.As for local use
func As(err error, target interface{}) bool {
	if err == nil {
		return false
	}
	// Use standard errors.As
	return stderrors.As(err, target)
}