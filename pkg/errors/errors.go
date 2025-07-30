package errors

import "fmt"

// CreateError returns an error that formats as the given text.
// Each call to CreateError returns a distinct error value even if the text is identical.
func CreateError(text string) error {
    return &errorString{text}
}

func New(err error, message string) error {
    return &KubeError{err, message}
}

// errorString is a trivial implementation of error.
type errorString struct {
    s string
}

type KubeError struct {
    err     error
    message string
}

func (e *errorString) Error() string {
    return e.s
}
func (e *KubeError) Error() string {
    return fmt.Sprintf("%s: %s", e.err, e.message)
}

// You may want to define your own errors
var (
    ErrGVKNotFound   = CreateError("could not determine GroupVersionKind")
    ErrGVKNotAllowed = CreateError("GroupVersionKind is not allowed")
    ErrNilObject     = CreateError("provided object is nil")
)
