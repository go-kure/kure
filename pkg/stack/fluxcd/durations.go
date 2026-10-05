package fluxcd

import (
	"fmt"
	"regexp"
	"time"

	"github.com/go-kure/kure/pkg/errors"
)

// fluxDurationPattern is the pattern the Flux API holds a duration to: the
// validation pattern on spec.interval, spec.timeout and spec.retryInterval of
// a Kustomization (kustomize-controller's api/v1), on spec.interval of a
// GitRepository and an OCIRepository (source-controller's api/v1) and on
// spec.sync.interval of a FluxInstance (flux-operator's api/v1), the objects
// this package writes a duration into. time.ParseDuration takes more: a sign,
// and the units ns and us.
var fluxDurationPattern = regexp.MustCompile(`^([0-9]+(\.[0-9]+)?(ms|s|m|h))+$`)

// fluxDuration returns the form d is written in and whether Flux takes it as
// a duration of one of those objects. A metav1.Duration is written as
// time.Duration.String, so that string is what the API server holds to
// fluxDurationPattern, not the value a caller authored: "90s" is written
// "1m30s" and "1us" is written "1µs". The durations it refuses are the
// negative ones ("-1s") and those under a millisecond ("1µs", "500ns"); zero
// is written "0s" and is taken.
func fluxDuration(d time.Duration) (written string, ok bool) {
	written = d.String()
	return written, fluxDurationPattern.MatchString(written)
}

// durationRefusal words the refusal of a duration Flux does not take: the
// field, the value as authored, where it comes from when that is not the
// object the error names (from, empty otherwise), and the form it is written
// in, which is the one Flux reads.
func durationRefusal(field, authored, from, written string) string {
	return fmt.Sprintf("%s %q%s is written as %q, which the Flux API does not take: it takes digits with a unit of ms, s, m or h, so no negative duration and no positive one under a millisecond",
		field, authored, from, written)
}

// checkDefaultInterval refuses a generator's DefaultInterval that Flux does
// not take as an interval. generator is the type that holds the field, as the
// error names it. It is called at each place the interval is written into an
// object: a bundle's Kustomization without an interval of its own, a
// per-layout Kustomization, the gotk bootstrap Kustomization, a generated
// GitRepository or OCIRepository and the FluxInstance sync. A generator whose
// DefaultInterval no generated object takes is therefore not refused.
func checkDefaultInterval(generator string, d time.Duration) error {
	written, ok := fluxDuration(d)
	if ok {
		return nil
	}
	return errors.ResourceValidationError(generator, "DefaultInterval", "interval",
		durationRefusal("interval", written, " (the generator's DefaultInterval)", written), nil)
}
