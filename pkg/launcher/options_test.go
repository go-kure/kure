package launcher

import (
	"testing"
	"time"

	"github.com/go-kure/kure/pkg/logger"
)

func TestDefaultOptions(t *testing.T) {
	opts := DefaultOptions()
	if opts == nil {
		t.Fatal("DefaultOptions returned nil")
	}
	if opts.Logger == nil {
		t.Error("Logger should not be nil")
	}
	if opts.MaxDepth != 10 {
		t.Errorf("MaxDepth = %d, want 10", opts.MaxDepth)
	}
	if opts.Timeout != 30*time.Second {
		t.Errorf("Timeout = %v, want 30s", opts.Timeout)
	}
	if opts.MaxWorkers < 1 {
		t.Errorf("MaxWorkers = %d, want >= 1", opts.MaxWorkers)
	}
	if opts.Debug {
		t.Error("Debug should default to false")
	}
	if opts.Verbose {
		t.Error("Verbose should default to false")
	}
}

func TestWithLogger(t *testing.T) {
	opts := DefaultOptions()
	l := logger.Default()
	result := opts.WithLogger(l)
	if result != opts {
		t.Error("WithLogger should return the same options for chaining")
	}
	if opts.Logger != l {
		t.Error("Logger not set")
	}
}

func TestWithTimeout(t *testing.T) {
	opts := DefaultOptions()
	result := opts.WithTimeout(5 * time.Minute)
	if result != opts {
		t.Error("WithTimeout should return the same options for chaining")
	}
	if opts.Timeout != 5*time.Minute {
		t.Errorf("Timeout = %v, want 5m", opts.Timeout)
	}
}

func TestWithDebug(t *testing.T) {
	opts := DefaultOptions()
	result := opts.WithDebug(true)
	if result != opts {
		t.Error("WithDebug should return the same options for chaining")
	}
	if !opts.Debug {
		t.Error("Debug should be true")
	}
}

func TestWithVerbose(t *testing.T) {
	opts := DefaultOptions()
	result := opts.WithVerbose(true)
	if result != opts {
		t.Error("WithVerbose should return the same options for chaining")
	}
	if !opts.Verbose {
		t.Error("Verbose should be true")
	}
}

func TestValidationResult_HasErrors(t *testing.T) {
	r := ValidationResult{}
	if r.HasErrors() {
		t.Error("empty result should not have errors")
	}
	r.Errors = append(r.Errors, ValidationError{Message: "test"})
	if !r.HasErrors() {
		t.Error("result with error should have errors")
	}
}

func TestValidationResult_HasWarnings(t *testing.T) {
	r := ValidationResult{}
	if r.HasWarnings() {
		t.Error("empty result should not have warnings")
	}
	r.Warnings = append(r.Warnings, ValidationWarning{Message: "test"})
	if !r.HasWarnings() {
		t.Error("result with warning should have warnings")
	}
}

func TestValidationResult_IsValid(t *testing.T) {
	r := ValidationResult{}
	if !r.IsValid() {
		t.Error("empty result should be valid")
	}
	r.Errors = append(r.Errors, ValidationError{Message: "test"})
	if r.IsValid() {
		t.Error("result with error should not be valid")
	}
}

func TestValidationError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  ValidationError
		want string
	}{
		{
			name: "resource and field",
			err:  ValidationError{Resource: "Deployment", Field: "spec.replicas", Message: "must be > 0"},
			want: "Deployment.spec.replicas: must be > 0",
		},
		{
			name: "resource only",
			err:  ValidationError{Resource: "Deployment", Message: "invalid"},
			want: "Deployment: invalid",
		},
		{
			name: "field only",
			err:  ValidationError{Field: "name", Message: "required"},
			want: "name: required",
		},
		{
			name: "message only",
			err:  ValidationError{Message: "something broke"},
			want: "something broke",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.err.Error()
			if got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidationWarning_String(t *testing.T) {
	tests := []struct {
		name string
		warn ValidationWarning
		want string
	}{
		{
			name: "resource and field",
			warn: ValidationWarning{Resource: "ConfigMap", Field: "data", Message: "empty"},
			want: "ConfigMap.data: empty",
		},
		{
			name: "resource only",
			warn: ValidationWarning{Resource: "Service", Message: "deprecated"},
			want: "Service: deprecated",
		},
		{
			name: "field only",
			warn: ValidationWarning{Field: "port", Message: "unusual"},
			want: "port: unusual",
		},
		{
			name: "message only",
			warn: ValidationWarning{Message: "warning text"},
			want: "warning text",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.warn.String()
			if got != tt.want {
				t.Errorf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}
