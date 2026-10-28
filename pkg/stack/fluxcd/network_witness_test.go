package fluxcd

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
)

// NetworkWitness records every attempt a build makes to reach the network
// (go-kure/kure#1168): a fake git first on PATH, which kustomize's cloner runs
// (sigs.k8s.io/kustomize/api v0.21.1 internal/git/gitrunner.go:24-43) and
// which writes its arguments to a marker file and fails, and a loopback HTTP
// server, which counts the requests kustomize's loader sends
// (internal/loader/fileloader.go:293-332). It sets PATH, so a test using it
// cannot run in parallel. Nothing leaves the host.
type NetworkWitness struct {
	// URL is the loopback server's base URL.
	URL    string
	marker string
	hits   atomic.Int64
}

// NewNetworkWitness starts the server and puts the fake git first on PATH for
// the rest of the test.
func NewNetworkWitness(t *testing.T) *NetworkWitness {
	t.Helper()
	w := &NetworkWitness{}
	bin := t.TempDir()
	w.marker = filepath.Join(bin, "git-calls")
	script := "#!/bin/sh\necho \"$@\" >> '" + w.marker + "'\nexit 1\n"
	if err := os.WriteFile(filepath.Join(bin, "git"), []byte(script), 0o755); err != nil { //nolint:gosec // G306: the fake git must be executable
		t.Fatal(err)
	}
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
	srv := httptest.NewServer(http.HandlerFunc(func(rw http.ResponseWriter, _ *http.Request) {
		w.hits.Add(1)
		_, _ = rw.Write([]byte("apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: fetched\n  namespace: default\n"))
	}))
	t.Cleanup(srv.Close)
	w.URL = srv.URL
	return w
}

// Check fails the test if git ran or the server was asked for anything.
func (w *NetworkWitness) Check(t *testing.T) {
	t.Helper()
	if n := w.hits.Load(); n != 0 {
		t.Errorf("the build sent %d HTTP request(s)", n)
	}
	if calls, err := os.ReadFile(w.marker); err == nil {
		t.Errorf("the build ran git:\n%s", calls)
	} else if !os.IsNotExist(err) {
		t.Fatal(err)
	}
}
