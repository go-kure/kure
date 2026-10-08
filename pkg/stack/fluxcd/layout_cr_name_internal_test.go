package fluxcd

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/stack"
)

// Tests for the shortening of a layout CR's "<unit>-<name>" default
// (unitLayoutCRName, go-kure/kure#1030, and for a name over 54 characters
// go-kure/kure#1036).

// Layout names over 54 characters, composed from several parts as a caller
// composes a hook group's, of 76 and 113 characters.
const (
	name76  = "checkout-service-payments-reconciler-worker-pre-install-hooks-schema-migrate"
	name113 = "checkout-service-payments-reconciler-worker-pre-install-hooks-schema-migration-batch-jobs-and-post-install-checks"
	// prefix54 is the first 54 characters of both.
	prefix54 = "checkout-service-payments-reconciler-worker-pre-instal"
)

// shortHash is the hash part of a shortened default, computed apart from the
// code under test.
func shortHash(composed string) string {
	sum := sha256.Sum256([]byte(composed))
	return hex.EncodeToString(sum[:])[:8]
}

// TestUnitLayoutCRName: a default that fits is returned unchanged; one over
// the limit is at most 63 characters and a valid name, the same on every
// call. With a name of at most 54 characters it keeps its "-<name>" tail
// behind a prefix of the unit name and a hash of the whole default; with a
// longer name it is the name's first 54 characters, without the hyphens and
// dots that end them, and that hash.
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
			desc: "name of 55 that fits",
			unit: "web", name: strings.Repeat("n", 55),
			want: "web-" + strings.Repeat("n", 55),
		},
		{
			desc: "name of 55",
			unit: "platform-services", name: strings.Repeat("n", 55),
			want: strings.Repeat("n", 54) + "-" + shortHash("platform-services-"+strings.Repeat("n", 55)),
		},
		{
			desc: "name of 63, one-character unit",
			unit: "a", name: strings.Repeat("n", 63),
			want: strings.Repeat("n", 54) + "-" + shortHash("a-"+strings.Repeat("n", 63)),
		},
		{
			desc: "name of 76",
			unit: "platform-services-payments", name: name76,
			want: prefix54 + "-" + shortHash("platform-services-payments-"+name76),
		},
		{
			desc: "name of 76, short unit",
			unit: "web", name: name76,
			want: prefix54 + "-" + shortHash("web-"+name76),
		},
		{
			desc: "name of 113",
			unit: "platform-services-payments", name: name113,
			want: prefix54 + "-" + shortHash("platform-services-payments-"+name113),
		},
		{
			// The longest DNS-1123 subdomain; kure sets no limit of its own
			// on a layout name, and the rule has none either.
			desc: "name of 253",
			unit: "platform-services-payments", name: strings.Repeat("n", 253),
			want: strings.Repeat("n", 54) + "-" + shortHash("platform-services-payments-"+strings.Repeat("n", 253)),
		},
		{
			desc: "name prefix ending in a hyphen",
			unit: "web", name: strings.Repeat("n", 53) + "-tail-of-the-name",
			want: strings.Repeat("n", 53) + "-" + shortHash("web-"+strings.Repeat("n", 53)+"-tail-of-the-name"),
		},
		{
			desc: "name prefix ending in a dot and a hyphen",
			unit: "web", name: strings.Repeat("n", 52) + ".-tail-of-the-name",
			want: strings.Repeat("n", 52) + "-" + shortHash("web-"+strings.Repeat("n", 52)+".-tail-of-the-name"),
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
			if err := stack.ValidateKustomizationName(got); err != nil {
				t.Errorf("%q is not a Flux Kustomization name: %v", got, err)
			}
			if len(tc.name) <= layoutNameShortenMax && !strings.HasSuffix(got, "-"+tc.name) {
				t.Errorf("%q does not keep the tail -%s", got, tc.name)
			}
		})
	}
}

// TestUnitLayoutCRName_PinnedLongName pins the hash of one name over 54
// characters, so that the form cannot move between versions.
func TestUnitLayoutCRName_PinnedLongName(t *testing.T) {
	got := unitLayoutCRName("platform-services-payments", name76)
	if want := prefix54 + "-5c8f049d"; got != want {
		t.Fatalf("unitLayoutCRName = %q, want %q", got, want)
	}
}

// TestUnitLayoutCRName_DistinctLongNames: two names over 54 characters that
// share their first 54 below one unit get different names, by the hash.
func TestUnitLayoutCRName_DistinctLongNames(t *testing.T) {
	a := unitLayoutCRName("platform-services-payments", name76)
	b := unitLayoutCRName("platform-services-payments", name113)
	if a == b {
		t.Fatalf("%q and %q both give %q", name76, name113, a)
	}
	if !strings.HasPrefix(a, prefix54+"-") || !strings.HasPrefix(b, prefix54+"-") {
		t.Fatalf("got %q and %q; want both to start with %s-", a, b, prefix54)
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
