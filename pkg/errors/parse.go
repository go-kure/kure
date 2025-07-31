package errors

import "strings"

// ParseErrors aggregates multiple errors returned during YAML decoding.
// It implements the error interface and unwraps to the underlying errors.
type ParseErrors struct {
    Errors []error
}

func (pe *ParseErrors) Error() string {
    if len(pe.Errors) == 0 {
        return ""
    }
    if len(pe.Errors) == 1 {
        return pe.Errors[0].Error()
    }
    var b strings.Builder
    b.WriteString("multiple parse errors:")
    for _, err := range pe.Errors {
        b.WriteString(" ")
        b.WriteString(err.Error())
        b.WriteString(";")
    }
    return strings.TrimSuffix(b.String(), ";")
}

func (pe *ParseErrors) Unwrap() []error {
    return pe.Errors
}
