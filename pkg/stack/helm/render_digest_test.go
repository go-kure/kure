package helm

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"helm.sh/helm/v4/pkg/registry"
)

func sha256Digest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// fakeRegistry is an in-process OCI distribution endpoint serving one root
// manifest under one tag, over plain HTTP: a chart manifest, or an image
// index whose child manifests it serves by digest.
type fakeRegistry struct {
	srv      *httptest.Server
	repo     string // e.g. "charts/testchart"
	tag      string
	manifest []byte
	// mediaType is the root manifest's media type; empty means an image
	// manifest.
	mediaType string
	// children are the manifests an index root lists, by digest, each with
	// its media type.
	children map[string]fakeManifest
	blobs    map[string][]byte
	// headerDigest, when set, is sent as Docker-Content-Digest instead of the
	// manifest's real digest.
	headerDigest string
	requests     atomic.Int64
	// blobGets counts the GET requests for each blob, by digest.
	blobGetsMu sync.Mutex
	blobGets   map[string]int
}

// blobGetCount returns how many times the blob with digest d was fetched.
func (fr *fakeRegistry) blobGetCount(d string) int {
	fr.blobGetsMu.Lock()
	defer fr.blobGetsMu.Unlock()
	return fr.blobGets[d]
}

// fakeManifest is a manifest the fake registry serves: its media type, its
// bytes and the blobs it names. An unlisted one is served by digest but left
// out of the index, as a nested index's own child is.
type fakeManifest struct {
	mediaType string
	data      []byte
	blobs     map[string][]byte
	unlisted  bool
}

// chartManifest returns the chart manifest of chartTar, whose Chart.yaml
// names name at chartVersion, and the blobs it names (config and chart
// layer). annotation goes into the manifest, so two manifests built with
// different annotations have different digests.
func chartManifest(t *testing.T, annotation, name, chartVersion string, chartTar []byte) ([]byte, map[string][]byte) {
	t.Helper()
	config, err := json.Marshal(map[string]string{"apiVersion": "v2", "name": name, "version": chartVersion})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	manifest, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     imageManifestMediaType,
		"config": map[string]any{
			"mediaType": registry.ConfigMediaType,
			"digest":    sha256Digest(config),
			"size":      len(config),
		},
		"layers": []map[string]any{{
			"mediaType": registry.ChartLayerMediaType,
			"digest":    sha256Digest(chartTar),
			"size":      len(chartTar),
		}},
		"annotations": map[string]string{"test.kure/revision": annotation},
	})
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	return manifest, map[string][]byte{sha256Digest(config): config, sha256Digest(chartTar): chartTar}
}

// newFakeIndexRegistry serves under charts/testchart:0.1.0 an image index
// listing children, each a manifest it serves by digest. A child is a chart
// manifest (chartManifest) or any other manifest with its media type.
func newFakeIndexRegistry(t *testing.T, children ...fakeManifest) *fakeRegistry {
	t.Helper()
	var listed []fakeManifest
	for _, c := range children {
		if !c.unlisted {
			listed = append(listed, c)
		}
	}
	return newFakeRootRegistry(t, indexOf(t, listed...), children...)
}

// newFakeRootRegistry serves root under charts/testchart:0.1.0 with its media
// type as Content-Type, and each of served by digest, with every blob they
// name.
func newFakeRootRegistry(t *testing.T, root fakeManifest, served ...fakeManifest) *fakeRegistry {
	t.Helper()
	fr := &fakeRegistry{
		repo:      "charts/testchart",
		tag:       "0.1.0",
		manifest:  root.data,
		mediaType: root.mediaType,
		children:  map[string]fakeManifest{},
		blobs:     maps.Clone(root.blobs),
	}
	if fr.blobs == nil {
		fr.blobs = map[string][]byte{}
	}
	for _, c := range served {
		fr.children[sha256Digest(c.data)] = c
		maps.Copy(fr.blobs, c.blobs)
	}
	fr.srv = httptest.NewServer(http.HandlerFunc(fr.serve))
	t.Cleanup(fr.srv.Close)
	return fr
}

// indexOf returns an image index listing children.
func indexOf(t *testing.T, children ...fakeManifest) fakeManifest {
	t.Helper()
	descs := []map[string]any{}
	for _, c := range children {
		descs = append(descs, descriptorOf(c))
	}
	data, err := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": imageIndexMediaType, "manifests": descs})
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	return fakeManifest{mediaType: imageIndexMediaType, data: data}
}

// descriptorOf returns the OCI descriptor naming m.
func descriptorOf(m fakeManifest) map[string]any {
	return map[string]any{"mediaType": m.mediaType, "digest": sha256Digest(m.data), "size": len(m.data)}
}

// chartChild is the manifest of a chart built by buildMinimalChartTar as an
// index child.
func chartChild(t *testing.T, name, version string) fakeManifest {
	t.Helper()
	manifest, blobs := chartManifest(t, "1", name, version, buildMinimalChartTar(t, name, version))
	return fakeManifest{mediaType: imageManifestMediaType, data: manifest, blobs: blobs}
}

// newFakeRegistry serves the chart archive built by buildMinimalChartTar
// under charts/testchart:0.1.0. annotation goes into the manifest, so two
// registries built with different annotations serve the same chart under
// different manifest digests, as a re-pushed tag does.
func newFakeRegistry(t *testing.T, annotation string) *fakeRegistry {
	t.Helper()
	return newFakeRegistryWithChart(t, annotation, "0.1.0", "0.1.0", buildMinimalChartTar(t, "testchart", "0.1.0"))
}

// newFakeRegistryWithChart serves chartTar, whose Chart.yaml version is
// chartVersion, as charts/testchart under tag.
func newFakeRegistryWithChart(t *testing.T, annotation, tag, chartVersion string, chartTar []byte) *fakeRegistry {
	t.Helper()
	manifest, blobs := chartManifest(t, annotation, "testchart", chartVersion, chartTar)
	fr := &fakeRegistry{
		repo:     "charts/testchart",
		tag:      tag,
		manifest: manifest,
		blobs:    blobs,
	}
	fr.srv = httptest.NewServer(http.HandlerFunc(fr.serve))
	t.Cleanup(fr.srv.Close)
	return fr
}

func (fr *fakeRegistry) serve(w http.ResponseWriter, r *http.Request) {
	fr.requests.Add(1)
	manifestPrefix := "/v2/" + fr.repo + "/manifests/"
	blobPrefix := "/v2/" + fr.repo + "/blobs/"
	switch {
	case r.URL.Path == "/v2/":
		w.WriteHeader(http.StatusOK)
	case strings.HasPrefix(r.URL.Path, manifestPrefix):
		ref := strings.TrimPrefix(r.URL.Path, manifestPrefix)
		m := fakeManifest{mediaType: fr.mediaType, data: fr.manifest}
		digest := sha256Digest(fr.manifest)
		if ref == fr.tag || ref == digest {
			if fr.headerDigest != "" {
				digest = fr.headerDigest
			}
		} else if child, ok := fr.children[ref]; ok {
			m, digest = child, ref
		} else {
			http.NotFound(w, r)
			return
		}
		if m.mediaType == "" {
			m.mediaType = imageManifestMediaType
		}
		w.Header().Set("Content-Type", m.mediaType)
		w.Header().Set("Docker-Content-Digest", digest)
		w.Header().Set("Content-Length", strconv.Itoa(len(m.data)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(m.data)
		}
	case strings.HasPrefix(r.URL.Path, blobPrefix):
		d := strings.TrimPrefix(r.URL.Path, blobPrefix)
		blob, ok := fr.blobs[d]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if r.Method == http.MethodGet {
			fr.blobGetsMu.Lock()
			if fr.blobGets == nil {
				fr.blobGets = map[string]int{}
			}
			fr.blobGets[d]++
			fr.blobGetsMu.Unlock()
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(blob)))
		if r.Method != http.MethodHead {
			_, _ = w.Write(blob)
		}
	default:
		http.NotFound(w, r)
	}
}

func (fr *fakeRegistry) chartURL() string {
	return "oci://" + strings.TrimPrefix(fr.srv.URL, "http://") + "/" + fr.repo
}

// usePlainHTTPRegistry makes renderOCI pull over plain HTTP, as the fake
// registry requires, for the duration of the test.
func usePlainHTTPRegistry(t *testing.T) {
	t.Helper()
	orig := newRegistryClient
	newRegistryClient = func() (*registry.Client, error) {
		return registry.NewClient(registry.ClientOptPlainHTTP(), registry.ClientOptWriter(io.Discard))
	}
	t.Cleanup(func() { newRegistryClient = orig })
}

func TestRenderChart_OCI_FakeRegistry_NoDigest(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistry(t, "1")

	out, err := RenderChart(fr.chartURL(), fr.tag, nil)
	if err != nil {
		t.Fatalf("RenderChart without WithDigest: %v", err)
	}
	if !strings.Contains(string(out), "kind: ConfigMap") {
		t.Errorf("expected the chart's ConfigMap in output, got:\n%s", out)
	}
}

func TestRenderChart_OCI_WithDigest_Match(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistry(t, "1")

	out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(sha256Digest(fr.manifest)))
	if err != nil {
		t.Fatalf("RenderChart with the served manifest's digest: %v", err)
	}
	if !strings.Contains(string(out), "kind: ConfigMap") {
		t.Errorf("expected the chart's ConfigMap in output, got:\n%s", out)
	}
}

func TestRenderChart_OCI_WithDigest_Mismatch(t *testing.T) {
	usePlainHTTPRegistry(t)
	pinned := newFakeRegistry(t, "1")
	repushed := newFakeRegistry(t, "2") // same tag, different manifest
	want := sha256Digest(pinned.manifest)
	got := sha256Digest(repushed.manifest)
	if want == got {
		t.Fatal("test setup: the two manifests must differ")
	}

	out, err := RenderChart(repushed.chartURL(), repushed.tag, nil, WithDigest(want))
	if err == nil {
		t.Fatalf("expected a digest mismatch error, got output:\n%s", out)
	}
	if out != nil {
		t.Errorf("expected no output on mismatch, got:\n%s", out)
	}
	for _, d := range []string{want, got} {
		if !strings.Contains(err.Error(), d) {
			t.Errorf("error %q does not name digest %s", err, d)
		}
	}
}

// A registry that claims the pinned digest in Docker-Content-Digest while
// serving other bytes must not pass. The registry client verifies the
// received bytes against the digest it reports, so the pull itself fails
// before WithDigest's comparison runs; this test pins that premise.
func TestRenderChart_OCI_WithDigest_HeaderClaimsPinnedDigest(t *testing.T) {
	usePlainHTTPRegistry(t)
	pinned := newFakeRegistry(t, "1")
	repushed := newFakeRegistry(t, "2")
	repushed.headerDigest = sha256Digest(pinned.manifest)

	out, err := RenderChart(repushed.chartURL(), repushed.tag, nil, WithDigest(sha256Digest(pinned.manifest)))
	if err == nil {
		t.Fatalf("expected an error for content that does not match the claimed digest, got output:\n%s", out)
	}
	if !strings.Contains(err.Error(), "mismatched digest") {
		t.Errorf("expected the pull to fail content verification, got %v", err)
	}
}

func TestRenderChart_WithDigest_Malformed_NoPull(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistry(t, "1")
	valid := sha256Digest(fr.manifest)
	hexPart := strings.TrimPrefix(valid, "sha256:")

	cases := map[string]string{
		"empty":           "",
		"no algorithm":    hexPart,
		"uppercase hex":   "sha256:" + strings.ToUpper(hexPart),
		"short":           valid[:len(valid)-1],
		"long":            valid + "0",
		"other algorithm": "sha512:" + hexPart,
		"tag form":        "0.1.0@" + valid,
	}
	for name, d := range cases {
		t.Run(name, func(t *testing.T) {
			before := fr.requests.Load()
			_, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(d))
			if err == nil {
				t.Fatalf("expected an error for digest %q", d)
			}
			if !strings.Contains(err.Error(), "invalid chart digest") {
				t.Errorf("expected an invalid-digest error, got %v", err)
			}
			if n := fr.requests.Load() - before; n != 0 {
				t.Errorf("expected no registry request, got %d", n)
			}
		})
	}
}

func TestRenderChart_HTTP_WithDigest_Refused(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	digest := "sha256:" + strings.Repeat("a", 64)
	_, err := RenderChart(srv.URL+"/testchart", "0.1.0", nil, WithDigest(digest))
	if err == nil {
		t.Fatal("expected WithDigest to be refused for an HTTP repository")
	}
	if !strings.Contains(err.Error(), "not supported for HTTP") {
		t.Errorf("expected an HTTP refusal, got %v", err)
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("expected no request to the repository, got %d", n)
	}
}
