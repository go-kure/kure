package fluxcd_test

import (
	"strings"
	"testing"

	fluxv1 "github.com/controlplaneio-fluxcd/flux-operator/api/v1"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/stack"
	fluxstack "github.com/go-kure/kure/pkg/stack/fluxcd"
)

// distributionCases are the four combinations of the two values a
// FluxInstance's spec.distribution is built from. Only the first is a usable
// distribution; the generator used to write the other three verbatim, as
// `version: ""` and `registry: ""`.
var distributionCases = []struct {
	name        string
	fluxVersion string
	registry    string
	// missing is the BootstrapConfig fields the error has to name. Empty means
	// the combination is valid.
	missing []string
}{
	{name: "both set", fluxVersion: "2.x", registry: "ghcr.io/fluxcd"},
	{name: "empty Registry", fluxVersion: "2.x", missing: []string{"Registry"}},
	{name: "empty FluxVersion", registry: "ghcr.io/fluxcd", missing: []string{"FluxVersion"}},
	{name: "both empty", missing: []string{"FluxVersion", "Registry"}},
}

// assertDistributionError checks that err is the typed validation error for an
// incomplete distribution and names exactly the fields in missing: a message
// that also named a field the caller did set would send them to the wrong one.
func assertDistributionError(t *testing.T, err error, missing []string) {
	t.Helper()
	if err == nil {
		t.Fatalf("error = nil, want one naming %v", missing)
	}
	if !errors.IsType(err, errors.ErrorTypeResource) {
		t.Errorf("error type = %T (%v), want a pkg/errors resource validation error", err, err)
	}
	for _, field := range []string{"FluxVersion", "Registry"} {
		want := false
		for _, m := range missing {
			if m == field {
				want = true
			}
		}
		if got := strings.Contains(err.Error(), field); got != want {
			t.Errorf("error names %s = %v, want %v: %v", field, got, want, err)
		}
	}
}

// TestGenerateFluxInstanceRequiresDistribution covers the public entry point
// that returns the FluxInstance alone.
func TestGenerateFluxInstanceRequiresDistribution(t *testing.T) {
	for _, tc := range distributionCases {
		t.Run(tc.name, func(t *testing.T) {
			fi, err := fluxstack.NewBootstrapGenerator().GenerateFluxInstance(&stack.BootstrapConfig{
				Enabled:     true,
				FluxVersion: tc.fluxVersion,
				Registry:    tc.registry,
			}, &stack.Node{Name: "prod"})

			if len(tc.missing) > 0 {
				assertDistributionError(t, err, tc.missing)
				if fi != nil {
					t.Errorf("GenerateFluxInstance() = %v alongside an error, want nil", fi)
				}
				return
			}
			if err != nil {
				t.Fatalf("GenerateFluxInstance() error = %v", err)
			}
			// The whole struct, so that a value the caller did not supply
			// (a defaulted artifact, say) fails here too.
			want := fluxv1.Distribution{Version: tc.fluxVersion, Registry: tc.registry}
			if fi.Spec.Distribution != want {
				t.Errorf("spec.distribution = %+v, want %+v", fi.Spec.Distribution, want)
			}
		})
	}
}

// TestFluxOperatorBootstrapRequiresDistribution covers the bootstrap path, under
// both spellings of the mode: an empty FluxMode resolves to flux-operator and
// must not be a way around the check.
func TestFluxOperatorBootstrapRequiresDistribution(t *testing.T) {
	for _, mode := range []string{"flux-operator", ""} {
		for _, tc := range distributionCases {
			t.Run("mode="+mode+"/"+tc.name, func(t *testing.T) {
				resources, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(&stack.BootstrapConfig{
					Enabled:     true,
					FluxMode:    mode,
					FluxVersion: tc.fluxVersion,
					Registry:    tc.registry,
				}, &stack.Node{Name: "prod"})

				if len(tc.missing) > 0 {
					assertDistributionError(t, err, tc.missing)
					// Not the install bundle without its FluxInstance either:
					// a partial bundle applies cleanly and installs no Flux.
					if len(resources) != 0 {
						t.Errorf("GenerateBootstrap() returned %d resources alongside an error, want none", len(resources))
					}
					return
				}
				if err != nil {
					t.Fatalf("GenerateBootstrap() error = %v", err)
				}
				want := fluxv1.Distribution{Version: tc.fluxVersion, Registry: tc.registry}
				if got := findFluxInstance(t, resources).Spec.Distribution; got != want {
					t.Errorf("spec.distribution = %+v, want %+v", got, want)
				}
			})
		}
	}
}

// TestGotkBootstrapAcceptsAnEmptyDistribution is the control: gotk mode reads
// the same two fields, and there both empty means an offline build from the
// vendored release with the upstream registry, which is valid.
func TestGotkBootstrapAcceptsAnEmptyDistribution(t *testing.T) {
	resources, err := fluxstack.NewBootstrapGenerator().GenerateBootstrap(&stack.BootstrapConfig{
		Enabled:  true,
		FluxMode: "gotk",
	}, &stack.Node{Name: "prod"})
	if err != nil {
		t.Fatalf("GenerateBootstrap() error = %v", err)
	}
	if len(resources) == 0 {
		t.Fatal("GenerateBootstrap() returned no resources")
	}
	for _, obj := range resources {
		if _, ok := obj.(*fluxv1.FluxInstance); ok {
			t.Fatal("gotk mode emitted a FluxInstance")
		}
	}
}
