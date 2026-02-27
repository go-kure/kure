package errors_test

import (
	"errors"
	"testing"

	kerrors "github.com/go-kure/kure/pkg/errors"
)

func TestParseErrors(t *testing.T) {
	t.Run("empty errors", func(t *testing.T) {
		pe := &kerrors.ParseErrors{Errors: nil}
		if pe.Error() != "" {
			t.Errorf("Expected empty string for nil errors, got %q", pe.Error())
		}

		pe = &kerrors.ParseErrors{Errors: []error{}}
		if pe.Error() != "" {
			t.Errorf("Expected empty string for empty errors, got %q", pe.Error())
		}
	})

	t.Run("single error", func(t *testing.T) {
		err := errors.New("single error")
		pe := &kerrors.ParseErrors{Errors: []error{err}}

		if pe.Error() != "single error" {
			t.Errorf("Expected 'single error', got %q", pe.Error())
		}
	})

	t.Run("multiple errors", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		err3 := errors.New("error 3")
		pe := &kerrors.ParseErrors{Errors: []error{err1, err2, err3}}

		result := pe.Error()
		if result != "multiple parse errors: error 1; error 2; error 3" {
			t.Errorf("Unexpected error message: %q", result)
		}
	})

	t.Run("unwrap returns errors", func(t *testing.T) {
		err1 := errors.New("error 1")
		err2 := errors.New("error 2")
		pe := &kerrors.ParseErrors{Errors: []error{err1, err2}}

		unwrapped := pe.Unwrap()
		if len(unwrapped) != 2 {
			t.Errorf("Expected 2 unwrapped errors, got %d", len(unwrapped))
		}

		if !errors.Is(unwrapped[0], err1) {
			t.Errorf("First unwrapped error doesn't match")
		}
		if !errors.Is(unwrapped[1], err2) {
			t.Errorf("Second unwrapped error doesn't match")
		}
	})

	t.Run("unwrap nil errors", func(t *testing.T) {
		pe := &kerrors.ParseErrors{Errors: nil}
		unwrapped := pe.Unwrap()
		if unwrapped != nil {
			t.Errorf("Expected nil for nil errors, got %v", unwrapped)
		}
	})
}
