package fluxcd

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
)

// Tests for the shortening of a layout CR's "<unit>-<name>" default
// (unitLayoutCRName, go-kure/kure#1030).

// shortHash is the hash part of a shortened default, computed apart from the
// code under test.
func shortHash(composed string) string {
	sum := sha256.Sum256([]byte(composed))
	return hex.EncodeToString(sum[:])[:8]
}

// TestUnitLayoutCRName: a default that fits is returned unchanged; one over
// the limit keeps its "-<name>" tail behind a prefix of the unit name and a
// hash of the whole default, is at most 63 characters and a valid name; a name
// over 54 characters is not shortened.
func TestUnitLayoutCRName(t *testing.T) {
	for _, tc := range []struct {
		desc, unit, name, want string
	}{
		{
			desc: "fits",
			unit: "web", name: "api",
			want: "web-api",
		},
		{
			desc: "fits at exactly the limit",
			unit: "web", name: strings.Repeat("a", 59),
			want: "web-" + strings.Repeat("a", 59),
		},
		{
			// The README's example, its hash pinned so that it cannot move
			// between versions.
			desc: "over the limit",
			unit: "platform-services-payments", name: "checkout-service-payments-reconciler-worker",
			want: "platform-s-7365aca5-checkout-service-payments-reconciler-worker",
		},
		{
			desc: "one over the limit",
			unit: "platform-services", name: strings.Repeat("a", 46),
			want: "platfor-" + shortHash("platform-services-"+strings.Repeat("a", 46)) + "-" + strings.Repeat("a", 46),
		},
		{
			desc: "prefix ending in a hyphen",
			unit: "platform-services-payments", name: strings.Repeat("n", 44),
			want: "platform-" + shortHash("platform-services-payments-"+strings.Repeat("n", 44)) + "-" + strings.Repeat("n", 44),
		},
		{
			desc: "prefix ending in a dot",
			unit: "platform.services.payments", name: strings.Repeat("n", 44),
			want: "platform-" + shortHash("platform.services.payments-"+strings.Repeat("n", 44)) + "-" + strings.Repeat("n", 44),
		},
		{
			desc: "no byte of the unit left, name of 53",
			unit: "platform-services", name: strings.Repeat("n", 53),
			want: shortHash("platform-services-"+strings.Repeat("n", 53)) + "-" + strings.Repeat("n", 53),
		},
		{
			desc: "no byte of the unit left, name of 54",
			unit: "platform-services", name: strings.Repeat("n", 54),
			want: shortHash("platform-services-"+strings.Repeat("n", 54)) + "-" + strings.Repeat("n", 54),
		},
		{
			desc: "name of 55 not shortened",
			unit: "platform-services", name: strings.Repeat("n", 55),
			want: "platform-services-" + strings.Repeat("n", 55),
		},
	} {
		t.Run(tc.desc, func(t *testing.T) {
			got := unitLayoutCRName(tc.unit, tc.name)
			if got != tc.want {
				t.Fatalf("unitLayoutCRName(%q, %q) = %q, want %q", tc.unit, tc.name, got, tc.want)
			}
			if again := unitLayoutCRName(tc.unit, tc.name); again != got {
				t.Errorf("a second call gives %q, the first %q", again, got)
			}
			if len(tc.name) > layoutNameShortenMax {
				return
			}
			if err := stack.ValidateKustomizationName(got); err != nil {
				t.Errorf("%q is not a Flux Kustomization name: %v", got, err)
			}
			if !strings.HasSuffix(got, "-"+tc.name) {
				t.Errorf("%q does not keep the tail -%s", got, tc.name)
			}
		})
	}
}

// TestUnitLayoutCRName_DistinctUnits: two defaults over the limit that differ
// only in the part the shortening replaces get different names, except where
// their 8-character hashes collide (the integrator then refuses the tree,
// TestLayoutKustomization_ShortenedNameCollision).
func TestUnitLayoutCRName_DistinctUnits(t *testing.T) {
	name := "checkout-service-payments-reconciler-worker"
	a := unitLayoutCRName("platform-services-payments", name)
	b := unitLayoutCRName("platform-services-billing", name)
	if a == b {
		t.Fatalf("units platform-services-payments and platform-services-billing both give %q", a)
	}
	if !strings.HasPrefix(a, "platform-s-") || !strings.HasPrefix(b, "platform-s-") {
		t.Fatalf("got %q and %q; want both to keep the same unit prefix, so that only the hash tells them apart", a, b)
	}

	x := unitLayoutCRName("platform-services-65327", name)
	y := unitLayoutCRName("platform-services-103659", name)
	if want := "platform-s-826dabca-" + name; x != want || y != want {
		t.Errorf("the colliding units give %q and %q, want both %q", x, y, want)
	}
}
