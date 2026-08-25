package versions_test

import (
	"bytes"
	"go/format"
	"os"
	"strings"
	"testing"

	"github.com/go-kure/kure/pkg/versions"
)

// The infrastructure.kubernetes entry is a published contract: an external
// consumer asserts its own Kubernetes support range against it.
func TestGetKubernetes(t *testing.T) {
	d, ok := versions.Get("kubernetes")
	if !ok {
		t.Fatal(`Get("kubernetes") not found`)
	}
	if d.SupportedRange == "" {
		t.Error("kubernetes SupportedRange is empty")
	}
	if d.VersionBasis != "kubernetes" {
		t.Errorf(`VersionBasis = %q, want "kubernetes"`, d.VersionBasis)
	}
}

func TestGetMissing(t *testing.T) {
	if _, ok := versions.Get("no-such-dependency"); ok {
		t.Error("Get returned ok for an absent key")
	}
}

func TestAllEntriesPopulated(t *testing.T) {
	all := versions.All()
	if len(all) == 0 {
		t.Fatal("All() is empty")
	}
	for _, d := range all {
		if d.Name == "" || d.GoModule == "" || d.SupportedRange == "" || d.VersionBasis == "" {
			t.Errorf("incomplete entry: %+v", d)
		}
	}
}

// The generator must split bounds by the same rule the range guard in
// scripts/sync-versions.sh uses: " - " separates a range, otherwise Min == Max.
func TestRangeBoundsMatchSupportedRange(t *testing.T) {
	for _, d := range versions.All() {
		lo, hi, found := strings.Cut(d.SupportedRange, " - ")
		if !found {
			lo, hi = d.SupportedRange, d.SupportedRange
		}
		if d.Min != lo || d.Max != hi {
			t.Errorf("%s: %q gave Min=%q Max=%q, want Min=%q Max=%q",
				d.Name, d.SupportedRange, d.Min, d.Max, lo, hi)
		}
	}
}

func TestAllReturnsCopy(t *testing.T) {
	a := versions.All()
	if len(a) == 0 {
		t.Fatal("All() is empty")
	}
	a[0].Name = "mutated"
	if versions.All()[0].Name == "mutated" {
		t.Error("All() exposes the package-level slice; callers can corrupt it")
	}
}

func TestGoVersionSet(t *testing.T) {
	if versions.GoVersion == "" {
		t.Error("GoVersion is empty")
	}
}

// A byte-comparison drift guard breaks permanently if a later `gofmt -w`
// "fixes" the generated file. Catch non-canonical output at the source.
func TestGeneratedFileIsGofmted(t *testing.T) {
	const path = "versions_gen.go"
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	want, err := format.Source(src)
	if err != nil {
		t.Fatalf("format %s: %v", path, err)
	}
	if !bytes.Equal(src, want) {
		t.Errorf("%s is not gofmt-canonical; the generator must emit canonical Go", path)
	}
}
