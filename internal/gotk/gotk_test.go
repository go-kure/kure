package gotk

import (
	"archive/tar"
	"compress/gzip"
	stderrors "errors"
	"io"
	"regexp"
	"testing"
)

// entries lists the names in one reading of the bundle.
func entries(t *testing.T) map[string]bool {
	t.Helper()
	gz, err := gzip.NewReader(Open())
	if err != nil {
		t.Fatalf("the bundle is not gzip: %v", err)
	}
	names := map[string]bool{}
	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if stderrors.Is(err, io.EOF) {
			return names
		}
		if err != nil {
			t.Fatalf("the bundle is not a tar: %v", err)
		}
		names[hdr.Name] = true
	}
}

func TestOpenYieldsTheFlatManifestBundle(t *testing.T) {
	names := entries(t)
	for _, want := range []string{"source-controller.yaml", "kustomize-controller.yaml", "helm-controller.yaml", "notification-controller.yaml", "rbac.yaml"} {
		if !names[want] {
			t.Errorf("bundle has no %s; entries: %v", want, names)
		}
	}
}

// A second Open starts over: the bundle is read by more than one consumer in
// one process, and a shared cursor would hand the second an empty archive.
func TestOpenStartsAtTheBeginningEveryTime(t *testing.T) {
	first := entries(t)
	second := entries(t)
	if len(first) == 0 || len(first) != len(second) {
		t.Errorf("first reading saw %d entries, second %d", len(first), len(second))
	}
}

func TestVersionIsARelease(t *testing.T) {
	if !regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+$`).MatchString(Version) {
		t.Errorf("Version = %q, want a vX.Y.Z release", Version)
	}
}
