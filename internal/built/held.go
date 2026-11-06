package built

import (
	"encoding/json"
	stderrors "errors"
	"fmt"
	"reflect"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// This file is the guard for a method of an object a caller holds. The paths
// below go through it, and each is guarded where it calls the object, never in
// kure's own code:
//   - the read of a built tree (kindOf, marshalHeld);
//   - the labels and annotations Bundle.Generate writes (CallHeld);
//   - the entry probe (ProbeHeld): an entry point that reads the objects of a
//     layout calls GetObjectKind, GetName, GetNamespace, GetLabels and
//     GetAnnotations of each once, before it reads any, so that an object
//     whose metadata methods panic is refused at entry, whether or not a
//     later phase would have called the method that panics. The metadata of
//     an item that is no client.Object is the one it exposes through
//     ObjectMetaAccessor, as the writers read it;
//   - the annotations a delivery intent sets and takes back
//     (AddAnnotationsHeld), which probing cannot cover: a setter changes the
//     object.
//
// Other public helpers call such a method too, and say in their doc comment
// that they do not recover a panic of it. A caller's object may panic in its
// methods, or hold a value that formats without end, so a call into it is
// guarded here, and a refusal names the Go type of the object and nothing it
// holds: a caller's value is never formatted and never has a method called on
// it to describe it.
//
// The guard has limits. The probe calls each method once: a method that
// answers or panics differently on a later call is not covered, and neither is
// a method that never returns. A setter that panics part-way may leave the
// object changed, and then the annotations delivery set cannot be taken back:
// the refusal says so.

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

// CallHeld calls f, which calls the methods of obj, an object a caller holds,
// and returns a panic of one as the refusal callHeld returns. It is the entry
// for a reader outside this package: f holds the calls into obj and as little
// else as it can, because a panic anywhere in f is reported as the object's.
func CallHeld(obj runtime.Object, f func()) error {
	return callHeld(obj, f)
}

// callHeld calls f, which calls a method of obj, an object a caller holds, and
// returns a panic of it as a refusal that names the Go type of obj: a type
// that embeds a nil pointer panics in the methods it promotes from it. It
// guards only the calls into the caller's value, never kure's own code, so a
// defect of kure's still panics. The panic value is reported by its type: it
// may be a nil error, or a value that holds itself, and either fails when
// described.
func callHeld(obj runtime.Object, f func()) error {
	return callHeldFor(obj, f, "so the object cannot be read")
}

// callHeldFor is callHeld with the consequence the refusal states.
func callHeldFor(obj runtime.Object, f func(), consequence string) (err error) {
	defer func() {
		if p := recover(); p != nil {
			err = &refusedError{fmt.Sprintf("a method of the %T it holds panicked with a %T, %s", obj, p, consequence)}
		}
	}()
	f()
	return nil
}

// ProbeHeld calls GetObjectKind, GetName, GetNamespace, GetLabels and
// GetAnnotations once on each of objs and on each object they hold as items,
// directly or through the typed Lists they hold (HeldObjects), and returns the
// refusal of the first whose method panics. It reads nothing else and changes
// nothing. An unstructured object and a nil one are skipped: the first is
// apimachinery's, and kure refuses the second elsewhere, in its own words. For
// an object that is no client.Object the metadata is what it exposes through
// ObjectMetaAccessor, which is what the writers read; one with neither is the
// reader's to report.
//
// It is the entry probe of a reader that goes on to call such methods at many
// places: those calls are not each guarded, so the one probe stands for them.
func ProbeHeld(objs ...runtime.Object) error {
	for _, o := range objs {
		if err := probeObject(o); err != nil {
			return err
		}
		// An error of the item read is the reader's to report, in its words.
		_, items, _ := HeldObjects(o)
		for _, item := range items {
			if err := probeObject(item); err != nil {
				return err
			}
		}
	}
	return nil
}

func probeObject(o runtime.Object) error {
	if o == nil {
		return nil
	}
	if _, ok := o.(*unstructured.Unstructured); ok {
		return nil
	}
	if v := reflect.ValueOf(o); v.Kind() == reflect.Pointer && v.IsNil() {
		return nil
	}
	// The metadata is resolved as the writers resolve it, through
	// meta.Accessor (not imported here: its list readers format the object
	// they read, see boundary_test.go): an object that is no client.Object may
	// still expose metadata through ObjectMetaAccessor, and then it is that
	// metadata whose methods are called. An object with neither is the
	// reader's to report.
	return callHeld(o, func() {
		_ = o.GetObjectKind().GroupVersionKind()
		var acc metav1.Object
		switch t := o.(type) {
		case metav1.Object:
			acc = t
		case metav1.ObjectMetaAccessor:
			acc = t.GetObjectMeta()
		}
		if acc == nil {
			return
		}
		_ = acc.GetName()
		_ = acc.GetNamespace()
		_ = acc.GetLabels()
		_ = acc.GetAnnotations()
	})
}

// AddAnnotationsHeld is AddAnnotations for an object a caller holds. It
// returns the refusal of an object whose method panics, and then the object has
// not been annotated by this call. The undo it returns takes the annotations
// back, returns the refusal of an object whose method panics and never
// panics itself; it is nil when AddAnnotations changed nothing.
//
// The whole call is guarded, as Bundle.Generate guards it, and not the setter
// alone: the helper is a few map operations around the object's
// GetAnnotations and SetAnnotations, and on malformed unstructured metadata
// it falls back to GetAnnotations, which can format a caller's value. So a
// panic in the helper's own map operations on such an object is reported as
// the object's too.
func AddAnnotationsHeld(obj client.Object, want map[string]string) (undo func() error, err error) {
	var raw func()
	if err := callHeldFor(obj, func() { raw = AddAnnotations(obj, want) }, "so the object cannot be annotated"); err != nil {
		return nil, err
	}
	if raw == nil {
		return nil, nil
	}
	return func() error {
		return callHeldFor(obj, raw, "so the annotations kure set on it could not be restored")
	}, nil
}
