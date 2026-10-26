// Package stacktest checks an ApplicationConfig against the generation
// contract (stack.ApplicationConfig, pkg/stack README § Generation contract)
// from the config author's own tests.
//
// It catches the two mistakes the contract names that fail silently:
//
//   - Sharing. CheckGeneration calls Application.Generate twice and reports
//     every object, map, slice backing array or pointer the two results share,
//     and every one a single result reaches by two paths (a labels map that is
//     also a selector). kure and its caller write into what Generate returns,
//     so a shared value carries one result's edits into the other.
//   - Lost optional interfaces. InterfacesOf reports which of stack.Validator,
//     layout.LayoutAugmenter and layout.LayoutIntentAugmenter a config
//     implements, so a wrapper can be compared with the config it wraps.
//
// It checks two calls, not every call, and does not check concurrency safety
// or what an augmenter adds.
package stacktest

import (
	"testing"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// Interfaces records which optional interfaces kure finds on a config by type
// assertion.
type Interfaces struct {
	Validator             bool // stack.Validator
	LayoutAugmenter       bool // layout.LayoutAugmenter
	LayoutIntentAugmenter bool // layout.LayoutIntentAugmenter
}

// InterfacesOf reports the optional interfaces cfg implements. A nil cfg
// implements none.
func InterfacesOf(cfg stack.ApplicationConfig) Interfaces {
	_, validator := cfg.(stack.Validator)
	_, augmenter := cfg.(layout.LayoutAugmenter)
	_, intent := cfg.(layout.LayoutIntentAugmenter)
	return Interfaces{Validator: validator, LayoutAugmenter: augmenter, LayoutIntentAugmenter: intent}
}

// Missing returns the names of the interfaces want has and got lacks, in the
// order of the Interfaces fields: what a wrapper (got) hides of the config it
// wraps (want).
func (got Interfaces) Missing(want Interfaces) []string {
	var missing []string
	if want.Validator && !got.Validator {
		missing = append(missing, "stack.Validator")
	}
	if want.LayoutAugmenter && !got.LayoutAugmenter {
		missing = append(missing, "layout.LayoutAugmenter")
	}
	if want.LayoutIntentAugmenter && !got.LayoutIntentAugmenter {
		missing = append(missing, "layout.LayoutIntentAugmenter")
	}
	return missing
}

// Sharing is one value reached at two paths.
//
// A path starts at the result slice: "[1].(*v1.Deployment).Spec.Selector.MatchLabels"
// is the selector map of the Deployment the second entry holds. Indexing a
// slice, array or map adds "[i]" or "[key]", a struct field ".Field", an
// interface ".(T)" with the dynamic type; following a pointer adds nothing.
// The empty path is the result slice itself.
type Sharing struct {
	// Kind is what the value at First is: "object" (a pointer to a
	// client.Object, or a *client.Object entry), "map", "slice" (a slice's
	// backing array) or "pointer" (any other pointer).
	Kind string
	// First is the path in the first result, or for sharing within one
	// result the path the walk reached first.
	First string
	// Second is the path in the second result, or the later path within
	// one result.
	Second string
}

// Report is the outcome of CheckGeneration.
type Report struct {
	// Shared lists the values the two results share, sorted by First.
	Shared []Sharing
	// Internal lists the values one result reaches at two paths, sorted by
	// First. An entry found in both results is listed once.
	Internal []Sharing
	// Interfaces are the optional interfaces app.Config implements.
	Interfaces Interfaces
}

// CheckGeneration calls app.Generate twice, so Validate runs before each call
// as in a walk, and reports every value the two results share, every value
// one result reaches at two paths, and the config's optional interfaces.
//
// Storage counts as shared when it overlaps, not only when two pointers are
// equal: a pointer into another result's slice, or into a struct another
// result holds, is reported. A match inside a value already reported at both
// of its paths is not listed again. A time.Time (so a metav1.Time) is not
// walked: time.Now gives every value the same package-level location. Strings
// are immutable and are not checked. A value that contains itself is walked
// once and is not reported. A slice's elements are walked up to its length: a
// value held only between its length and capacity, which reslicing reaches,
// is not checked.
//
// An error from either call is returned wrapped with the call number, with no
// report. So is a func, channel or unsafe pointer value in a result, a map
// with a key that is not a string, bool, integer or float (directly or in an
// interface), or a map with two keys that print alike (two NaN keys), which a
// Kubernetes object never holds.
func CheckGeneration(app *stack.Application) (*Report, error) {
	if app == nil {
		return nil, errors.New("check generation: nil application")
	}
	var walks [2]*walk
	for i := range walks {
		objs, err := app.Generate()
		if err != nil {
			return nil, errors.Wrapf(err, "check generation: call %d", i+1)
		}
		w, err := walkResult(objs)
		if err != nil {
			return nil, errors.Wrapf(err, "check generation: result of call %d", i+1)
		}
		walks[i] = w
	}
	return &Report{
		Shared:     shared(walks[0], walks[1]),
		Internal:   internal(walks[0], walks[1]),
		Interfaces: InterfacesOf(app.Config),
	}, nil
}

// AssertGeneration runs CheckGeneration and fails t, with Errorf, for an
// error or for each value in the report's Shared and Internal lists, naming
// both paths. It returns the report, or nil after an error.
func AssertGeneration(t testing.TB, app *stack.Application) *Report {
	t.Helper()
	report, err := CheckGeneration(app)
	if err != nil {
		t.Errorf("%v", err)
		return nil
	}
	for _, s := range report.Shared {
		t.Errorf("the two results share a %s: %q in the first, %q in the second", s.Kind, s.First, s.Second)
	}
	for _, s := range report.Internal {
		t.Errorf("one result reaches a %s at two paths: %q and %q", s.Kind, s.First, s.Second)
	}
	return report
}
