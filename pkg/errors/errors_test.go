package errors_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func TestWrap(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		message  string
		expected string
		wantNil  bool
	}{
		{
			name:    "nil error",
			err:     nil,
			message: "test message",
			wantNil: true,
		},
		{
			name:     "wrap error",
			err:      errors.New("original error"),
			message:  "context message",
			expected: "context message: original error",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := kerrors.Wrap(test.err, test.message)

			if test.wantNil {
				if result != nil {
					t.Errorf("Expected nil error, got %v", result)
				}
				return
			}

			if result == nil {
				t.Fatal("Expected non-nil error")
			}

			if result.Error() != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result.Error())
			}

			// Test unwrapping
			if !errors.Is(result, test.err) {
				t.Errorf("Expected wrapped error to match original")
			}
		})
	}
}

func TestWrapf(t *testing.T) {
	originalErr := errors.New("original error")
	result := kerrors.Wrapf(originalErr, "context %s %d", "test", 123)

	expected := "context test 123: original error"
	if result.Error() != expected {
		t.Errorf("Expected %q, got %q", expected, result.Error())
	}

	if !errors.Is(result, originalErr) {
		t.Errorf("Expected wrapped error to match original")
	}
}

func TestValidationError(t *testing.T) {
	validValues := []string{"value1", "value2", "value3"}
	err := kerrors.NewValidationError("field", "invalid", "component", validValues)

	// Test type
	if err.Type() != kerrors.ErrorTypeValidation {
		t.Errorf("Expected validation error type, got %v", err.Type())
	}

	// Test message
	expectedMsg := "invalid field for component: invalid"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
	}

	// Test suggestion
	suggestion := err.Suggestion()
	if !strings.Contains(suggestion, "value1, value2, value3") {
		t.Errorf("Expected suggestion to contain valid values, got %q", suggestion)
	}

	// Test context
	ctx := err.Context()
	if ctx["field"] != "field" {
		t.Errorf("Expected field in context, got %v", ctx["field"])
	}
	if ctx["component"] != "component" {
		t.Errorf("Expected component in context, got %v", ctx["component"])
	}

	// Test KureError interface
	if !kerrors.IsKureError(err) {
		t.Error("Expected error to be identified as KureError")
	}
}

func TestResourceError(t *testing.T) {
	t.Run("resource not found", func(t *testing.T) {
		available := []string{"res1", "res2"}
		err := kerrors.ResourceNotFoundError("Deployment", "missing", "default", available)

		if err.Type() != kerrors.ErrorTypeResource {
			t.Errorf("Expected resource error type, got %v", err.Type())
		}

		expectedMsg := "Deployment 'missing' not found in namespace 'default'"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
		}

		suggestion := err.Suggestion()
		if !strings.Contains(suggestion, "res1, res2") {
			t.Errorf("Expected suggestion to contain available resources, got %q", suggestion)
		}
	})

	t.Run("resource validation error", func(t *testing.T) {
		cause := errors.New("field is required")
		err := kerrors.ResourceValidationError("Pod", "test-pod", "spec.containers", "missing containers", cause)

		if err.Type() != kerrors.ErrorTypeResource {
			t.Errorf("Expected resource error type, got %v", err.Type())
		}

		expectedMsg := "validation failed for Pod 'test-pod' field 'spec.containers': missing containers"
		if !strings.Contains(err.Error(), expectedMsg) {
			t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
		}

		// Test unwrapping
		if !errors.Is(err, cause) {
			t.Error("Expected error to wrap the cause")
		}
	})
}

func TestPatchError(t *testing.T) {
	cause := errors.New("path not found")
	err := kerrors.NewPatchError("replace", "spec.replicas", "my-deployment", "target missing", cause)

	if err.Type() != kerrors.ErrorTypePatch {
		t.Errorf("Expected patch error type, got %v", err.Type())
	}

	expectedMsg := "patch operation 'replace' failed on resource 'my-deployment' at path 'spec.replicas': target missing"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
	}

	// Check for graceful mode suggestion
	suggestion := err.Suggestion()
	if !strings.Contains(suggestion, "graceful mode") {
		t.Errorf("Expected suggestion to mention graceful mode, got %q", suggestion)
	}

	// Test unwrapping
	if !errors.Is(err, cause) {
		t.Error("Expected error to wrap the cause")
	}
}

func TestParseError(t *testing.T) {
	cause := errors.New("yaml: line 5: mapping values are not allowed in this context")
	err := kerrors.NewParseError("test.yaml", "invalid YAML syntax", 5, 12, cause)

	if err.Type() != kerrors.ErrorTypeParse {
		t.Errorf("Expected parse error type, got %v", err.Type())
	}

	expectedMsg := "parse error in test.yaml at line 5, column 12: invalid YAML syntax"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
	}

	// Check for YAML-specific suggestion
	suggestion := err.Suggestion()
	if !strings.Contains(suggestion, "YAML syntax") {
		t.Errorf("Expected YAML-specific suggestion, got %q", suggestion)
	}

	// Test context
	ctx := err.Context()
	if ctx["line"] != 5 {
		t.Errorf("Expected line 5 in context, got %v", ctx["line"])
	}
	if ctx["column"] != 12 {
		t.Errorf("Expected column 12 in context, got %v", ctx["column"])
	}
}

func TestFileError(t *testing.T) {
	tests := []struct {
		name               string
		operation          string
		path               string
		cause              error
		expectedSuggestion string
	}{
		{
			name:               "permission denied",
			operation:          "read",
			path:               "/etc/secret",
			cause:              errors.New("permission denied"),
			expectedSuggestion: "Check file permissions",
		},
		{
			name:               "file not found",
			operation:          "open",
			path:               "/nonexistent/file",
			cause:              errors.New("no such file"),
			expectedSuggestion: "Verify the file path exists",
		},
		{
			name:               "is directory",
			operation:          "read",
			path:               "/some/dir",
			cause:              errors.New("is a directory"),
			expectedSuggestion: "specify a file instead",
		},
		{
			name:               "disk space",
			operation:          "write",
			path:               "/tmp/file",
			cause:              errors.New("no space left on device"),
			expectedSuggestion: "Check available disk space",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := kerrors.NewFileError(test.operation, test.path, "test reason", test.cause)

			if err.Type() != kerrors.ErrorTypeFile {
				t.Errorf("Expected file error type, got %v", err.Type())
			}

			expectedMsg := fmt.Sprintf("file %s failed for '%s': test reason", test.operation, test.path)
			if !strings.Contains(err.Error(), expectedMsg) {
				t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
			}

			suggestion := err.Suggestion()
			if !strings.Contains(suggestion, test.expectedSuggestion) {
				t.Errorf("Expected suggestion to contain %q, got %q", test.expectedSuggestion, suggestion)
			}
		})
	}
}

func TestConfigError(t *testing.T) {
	validValues := []string{"option1", "option2"}
	err := kerrors.NewConfigError("config.yaml", "mode", "invalid", "unsupported value", validValues)

	if err.Type() != kerrors.ErrorTypeConfiguration {
		t.Errorf("Expected configuration error type, got %v", err.Type())
	}

	expectedMsg := "configuration error in config.yaml for field 'mode' with value 'invalid': unsupported value"
	if !strings.Contains(err.Error(), expectedMsg) {
		t.Errorf("Expected message to contain %q, got %q", expectedMsg, err.Error())
	}

	suggestion := err.Suggestion()
	if !strings.Contains(suggestion, "option1, option2") {
		t.Errorf("Expected suggestion to contain valid values, got %q", suggestion)
	}
}

func TestErrorTypeChecking(t *testing.T) {
	validationErr := kerrors.NewValidationError("field", "value", "component", nil)
	resourceErr := kerrors.ResourceNotFoundError("Pod", "missing", "", nil)

	// Test IsType function
	if !kerrors.IsType(validationErr, kerrors.ErrorTypeValidation) {
		t.Error("Expected validation error to match validation type")
	}

	if kerrors.IsType(validationErr, kerrors.ErrorTypeResource) {
		t.Error("Expected validation error to NOT match resource type")
	}

	// Test IsKureError
	if !kerrors.IsKureError(validationErr) {
		t.Error("Expected validation error to be identified as KureError")
	}

	standardErr := errors.New("standard error")
	if kerrors.IsKureError(standardErr) {
		t.Error("Expected standard error to NOT be identified as KureError")
	}

	// Test GetKureError
	kureErr := kerrors.GetKureError(resourceErr)
	if kureErr == nil {
		t.Error("Expected to extract KureError from resource error")
	}
	if kureErr.Type() != kerrors.ErrorTypeResource {
		t.Errorf("Expected resource error type, got %v", kureErr.Type())
	}

	kureErr = kerrors.GetKureError(standardErr)
	if kureErr != nil {
		t.Error("Expected nil when extracting KureError from standard error")
	}
}

func TestErrorChaining(t *testing.T) {
	// Create a chain: standard error -> KureError -> wrapped error
	originalErr := errors.New("original error")
	kureErr := kerrors.NewParseError("test.yaml", "syntax error", 1, 0, originalErr)
	wrappedErr := kerrors.Wrap(kureErr, "failed to load configuration")

	// Test that we can find the original error
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("Expected to find original error in wrapped chain")
	}

	// Test that we can find the KureError
	if !errors.Is(wrappedErr, kureErr) {
		t.Error("Expected to find KureError in wrapped chain")
	}

	// Test that we can extract the KureError from the chain
	extractedKureErr := kerrors.GetKureError(wrappedErr)
	if extractedKureErr == nil {
		t.Fatal("Expected to extract KureError from wrapped chain")
	}

	if extractedKureErr.Type() != kerrors.ErrorTypeParse {
		t.Errorf("Expected parse error type, got %v", extractedKureErr.Type())
	}
}

func TestErrorJSON(t *testing.T) {
	err := kerrors.NewValidationError("replicas", "-1", "Deployment", []string{"0", "1", "2", "3"})

	// Test that context is properly populated for JSON serialization
	ctx := err.Context()
	expectedFields := []string{"field", "value", "component"}

	for _, field := range expectedFields {
		if _, exists := ctx[field]; !exists {
			t.Errorf("Expected field %q in context", field)
		}
	}

	// Verify field values
	if ctx["field"] != "replicas" {
		t.Errorf("Expected field 'replicas', got %v", ctx["field"])
	}
	if ctx["value"] != "-1" {
		t.Errorf("Expected value '-1', got %v", ctx["value"])
	}
	if ctx["component"] != "Deployment" {
		t.Errorf("Expected component 'Deployment', got %v", ctx["component"])
	}
}

func TestBaseErrorInterface(t *testing.T) {
	err := kerrors.ResourceNotFoundError("Service", "missing-svc", "production", []string{"svc1", "svc2"})

	// Test that it implements error interface
	var _ error = err

	// Test that it implements KureError interface
	var _ kerrors.KureError = err

	// Test Error() method
	errMsg := err.Error()
	if errMsg == "" {
		t.Error("Expected non-empty error message")
	}

	// Test Type() method
	if err.Type() != kerrors.ErrorTypeResource {
		t.Errorf("Expected resource error type, got %v", err.Type())
	}

	// Test Suggestion() method
	suggestion := err.Suggestion()
	if suggestion == "" {
		t.Error("Expected non-empty suggestion")
	}

	// Test Context() method
	ctx := err.Context()
	if ctx == nil {
		t.Error("Expected non-nil context")
	}
}

func TestErrorf(t *testing.T) {
	tests := []struct {
		name     string
		format   string
		args     []interface{}
		expected string
	}{
		{
			name:     "simple message",
			format:   "test error",
			args:     nil,
			expected: "test error",
		},
		{
			name:     "formatted message",
			format:   "error: %s at line %d",
			args:     []interface{}{"syntax error", 42},
			expected: "error: syntax error at line 42",
		},
		{
			name:     "multiple args",
			format:   "%s %s %d",
			args:     []interface{}{"foo", "bar", 123},
			expected: "foo bar 123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := kerrors.Errorf(test.format, test.args...)
			if err == nil {
				t.Fatal("Expected non-nil error")
			}
			if err.Error() != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, err.Error())
			}
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{
			name:    "simple message",
			message: "test error",
		},
		{
			name:    "empty message",
			message: "",
		},
		{
			name:    "multiline message",
			message: "line 1\nline 2\nline 3",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := kerrors.New(test.message)
			if err == nil {
				t.Fatal("Expected non-nil error")
			}
			if err.Error() != test.message {
				t.Errorf("Expected %q, got %q", test.message, err.Error())
			}
		})
	}
}

func TestParseErrors_Error(t *testing.T) {
	tests := []struct {
		name     string
		errors   []error
		expected string
	}{
		{
			name:     "empty errors",
			errors:   []error{},
			expected: "",
		},
		{
			name:     "single error",
			errors:   []error{errors.New("error 1")},
			expected: "error 1",
		},
		{
			name:     "multiple errors",
			errors:   []error{errors.New("error 1"), errors.New("error 2"), errors.New("error 3")},
			expected: "multiple parse errors: error 1; error 2; error 3",
		},
		{
			name:     "two errors",
			errors:   []error{errors.New("first"), errors.New("second")},
			expected: "multiple parse errors: first; second",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			pe := &kerrors.ParseErrors{Errors: test.errors}
			result := pe.Error()
			if result != test.expected {
				t.Errorf("Expected %q, got %q", test.expected, result)
			}
		})
	}
}

func TestParseErrors_Unwrap(t *testing.T) {
	err1 := errors.New("error 1")
	err2 := errors.New("error 2")
	err3 := errors.New("error 3")

	pe := &kerrors.ParseErrors{
		Errors: []error{err1, err2, err3},
	}

	unwrapped := pe.Unwrap()
	if len(unwrapped) != 3 {
		t.Fatalf("Expected 3 errors, got %d", len(unwrapped))
	}

	if !errors.Is(unwrapped[0], err1) {
		t.Errorf("Expected first error to be err1")
	}
	if !errors.Is(unwrapped[1], err2) {
		t.Errorf("Expected second error to be err2")
	}
	if !errors.Is(unwrapped[2], err3) {
		t.Errorf("Expected third error to be err3")
	}
}
