package errors

import (
	"errors"
	"fmt"
)

// Wrap wraps an error with a message using Go's standard error wrapping.
// Use this instead of the deprecated New function.
func Wrap(err error, message string) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s: %w", message, err)
}

// Deprecated: Use errors.New from standard library instead.
func CreateError(text string) error {
	return errors.New(text)
}

// Deprecated: Use Wrap function instead for proper error chaining.
func New(err error, message string) error {
	return Wrap(err, message)
}

// Standard error variables using the standard library
var (
	ErrGVKNotFound   = errors.New("could not determine GroupVersionKind")
	ErrGVKNotAllowed = errors.New("GroupVersionKind is not allowed")
	ErrNilObject     = errors.New("provided object is nil")
)
