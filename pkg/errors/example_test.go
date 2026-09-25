package errors_test

import (
	"fmt"

	"github.com/go-kure/kure/pkg/errors"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints the errors it built so go test
// checks their messages as well as compiling the calls.

func ExampleWrap() {
	err := errors.New("connection refused")

	// Wrap with context
	wrapped := errors.Wrap(err, "failed to load cluster config")

	// Wrap with formatted message
	formatted := errors.Wrapf(err, "failed to fetch %s/%s", "Deployment", "my-app")

	fmt.Println(wrapped)
	fmt.Println(formatted)
	// Output:
	// failed to load cluster config: connection refused
	// failed to fetch Deployment/my-app: connection refused
}

func ExampleErrorf() {
	// Simple error
	simple := errors.New("invalid configuration")

	// Formatted error
	formatted := errors.Errorf("unknown generator: %s", "AppWorkload")

	fmt.Println(simple)
	fmt.Println(formatted)
	// Output:
	// invalid configuration
	// unknown generator: AppWorkload
}

func ExampleNewValidationError() {
	originalErr := errors.New("original cause")

	// Validation error with suggestion
	validation := errors.NewValidationError(
		"replicas",         // field
		"-1",               // value
		"Deployment",       // component
		[]string{"1", "3"}, // valid values
	)

	// Resource not found
	notFound := errors.ResourceNotFoundError(
		"Deployment",                   // resource type
		"my-app",                       // name
		"default",                      // namespace
		[]string{"web-app", "api-app"}, // available resources
	)

	// Patch error
	patch := errors.NewPatchError(
		"set",             // operation
		"spec.replicas",   // path
		"my-deployment",   // resource name
		"field not found", // reason
		originalErr,       // cause
	)

	// Parse error with location
	parse := errors.NewParseError(
		"config.yaml",  // source file
		"invalid YAML", // reason
		42,             // line
		10,             // column
		originalErr,    // cause
	)

	// File error
	file := errors.NewFileError("read", "/path/to/file", "permission denied", originalErr)

	// Configuration error
	config := errors.NewConfigError(
		"mise.toml",              // source
		"go",                     // field
		"1.21",                   // value
		"version too old",        // reason
		[]string{"1.23", "1.24"}, // valid values
	)

	for _, err := range []error{validation, notFound, patch, parse, file, config} {
		fmt.Println(err)
	}
	// Output:
	// invalid replicas for Deployment: -1
	// Deployment 'my-app' not found in namespace 'default'
	// patch operation 'set' failed on resource 'my-deployment' at path 'spec.replicas': field not found: original cause
	// parse error in config.yaml at line 42, column 10: invalid YAML: original cause
	// file read failed for '/path/to/file': permission denied: original cause
	// configuration error in mise.toml for field 'go' with value '1.21': version too old
}

func ExampleIsKureError() {
	err := errors.Wrap(
		errors.NewValidationError("replicas", "-1", "Deployment", []string{"1", "3"}),
		"load cluster config",
	)

	// Check if error is a Kure error
	if errors.IsKureError(err) {
		kErr := errors.GetKureError(err)
		fmt.Println(kErr.Type())
		fmt.Println(kErr.Suggestion())
	}

	// Check specific error type
	if errors.IsType(err, errors.ErrorTypeValidation) {
		// Handle validation error
		fmt.Println("validation error:", err)
	}
	// Output:
	// validation
	// Valid values are: 1, 3
	// validation error: load cluster config: invalid replicas for Deployment: -1
}
