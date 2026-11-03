package built

import (
	"encoding/json"
	stderrors "errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
)

// This file is the one place a method of an object a caller holds is called.
// A caller's object may panic in its methods, or hold a value that formats
// without end, so a call into it is guarded here, and a refusal names the Go
// type of the object and nothing it holds: a caller's value is never formatted
// and never has a method called on it to describe it.

// refusedError is the refusal of an object whose method panicked, as opposed
// to an error that a method returned.
type refusedError struct{ msg string }

func (e *refusedError) Error() string { return e.msg }

// isRefusal reports whether err is the refusal of an object whose method panicked.
func isRefusal(err error) bool {
	var r *refusedError
	return stderrors.As(err, &r)
}

// kindOf returns the kind obj reports, or the refusal of an object whose
// method panics.
func kindOf(obj runtime.Object) (kind string, err error) {
	err = callHeld(obj, func() { kind = obj.GetObjectKind().GroupVersionKind().Kind })
	return kind, err
}

// marshalHeld returns obj as JSON, or the refusal of an object whose method
// panics. An error that marshalling itself returns is returned as it is.
func marshalHeld(obj runtime.Object) (data []byte, err error) {
	var marshalErr error
	if err := callHeld(obj, func() { data, marshalErr = json.Marshal(obj) }); err != nil {
		return nil, err
	}
	return data, marshalErr
}

// callHeld calls f, which calls a method of obj, an object a caller holds, and
// returns a panic of it as a refusal that names the Go type of obj: a type
// that embeds a nil pointer panics in the methods it promotes from it. It
// guards only the calls into the caller's value, never kure's own code, so a
// defect of kure's still panics. The panic value is reported by its type: it
// may be a nil error, or a value that holds itself, and either fails when
// described.
func callHeld(obj runtime.Object, f func()) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &refusedError{fmt.Sprintf("a method of the %T it holds panicked with a %T, so the object cannot be read", obj, p)}
		}
	}()
	f()
	return nil
}
