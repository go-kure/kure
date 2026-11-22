package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"

	"helm.sh/helm/v4/pkg/registry"
)

// memSource is a ContentSource over a map, recording what it was asked.
type memSource struct {
	mu        sync.Mutex
	m         map[ContentKey]Content
	lookups   []ContentKey
	stores    []ContentKey
	lookupErr error
	storeErr  error
}

func newMemSource() *memSource { return &memSource{m: map[ContentKey]Content{}} }

func (s *memSource) Lookup(_ context.Context, key ContentKey) (Content, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lookups = append(s.lookups, key)
	// A failing source may still hand back a usable copy with ok set; the
	// error alone must make it a miss.
	c, ok := s.m[key]
	return c, ok, s.lookupErr
}

func (s *memSource) Store(_ context.Context, key ContentKey, c Content) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stores = append(s.stores, key)
	if s.storeErr != nil {
		return s.storeErr
	}
	s.m[key] = c
	return nil
}

func (s *memSource) get(key ContentKey) Content {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[key]
}

func (s *memSource) set(key ContentKey, c Content) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = c
}

func mustRender(t *testing.T, chartURL, version string, opts ...RenderOption) []byte {
	t.Helper()
	out, err := RenderChart(chartURL, version, nil, opts...)
	if err != nil {
		t.Fatalf("RenderChart: %v", err)
	}
	return out
}

func hostOf(srv *httptest.Server) string { return strings.TrimPrefix(srv.URL, "http://") }

// An OCI chart a source holds renders without the registry, byte for byte as
// the fetched render, and Store gets the manifest, the archive and the
// registry host.
func TestRenderChart_ContentSource_OCI(t *testing.T) {
	usePlainHTTPRegistry(t)
	chartTar := buildLabelledChartTar(t, "0.1.0")
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", chartTar)
	src := newMemSource()
	key := ContentKey{Kind: "chart", URL: fr.chartURL(), Version: fr.tag}

	want := mustRender(t, fr.chartURL(), fr.tag)
	if got := mustRender(t, fr.chartURL(), fr.tag, WithContentSource(src)); !bytes.Equal(got, want) {
		t.Fatalf("a render through a source that misses differs from the plain one:\n%s\nwant:\n%s", got, want)
	}
	c := src.get(key)
	if !bytes.Equal(c.Manifest, fr.manifest) || !bytes.Equal(c.Data, chartTar) {
		t.Fatalf("Store got manifest %d bytes and archive %d bytes, want the registry's (%d, %d)",
			len(c.Manifest), len(c.Data), len(fr.manifest), len(chartTar))
	}
	if !slices.Equal(c.Hosts, []string{hostOf(fr.srv)}) {
		t.Errorf("Store got hosts %v, want [%s]", c.Hosts, hostOf(fr.srv))
	}

	n := fr.requests.Load()
	if got := mustRender(t, fr.chartURL(), fr.tag, WithContentSource(src)); !bytes.Equal(got, want) {
		t.Errorf("a render from the stored copy differs from the fetched one:\n%s\nwant:\n%s", got, want)
	}
	if m := fr.requests.Load(); m != n {
		t.Errorf("the render from the stored copy sent %d requests to the registry, want 0", m-n)
	}
}

// A pinned call is keyed by the pin and accepts only a manifest with that
// digest whose chart layer is the archive; anything else is a miss and the
// fetch runs.
func TestRenderChart_ContentSource_OCI_Pinned(t *testing.T) {
	usePlainHTTPRegistry(t)
	chartTar := buildLabelledChartTar(t, "0.1.0")
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", chartTar)
	other := newFakeRegistryWithChart(t, "2", "0.1.0", "0.1.0", chartTar)
	pin := sha256Digest(fr.manifest)
	key := ContentKey{Kind: "chart", URL: fr.chartURL(), Version: fr.tag, Digest: pin}
	want := mustRender(t, fr.chartURL(), fr.tag, WithDigest(pin))

	t.Run("hit", func(t *testing.T) {
		src := newMemSource()
		mustRender(t, fr.chartURL(), fr.tag, WithDigest(pin), WithContentSource(src))
		if len(src.stores) != 1 || src.stores[0] != key {
			t.Fatalf("Store keys %v, want [%v]", src.stores, key)
		}
		n := fr.requests.Load()
		if got := mustRender(t, fr.chartURL(), fr.tag, WithDigest(pin), WithContentSource(src)); !bytes.Equal(got, want) {
			t.Errorf("hit differs from the fetched render:\n%s", got)
		}
		if m := fr.requests.Load(); m != n {
			t.Errorf("hit sent %d requests, want 0", m-n)
		}
	})
	cases := map[string]Content{
		"manifest of another digest":               {Data: chartTar, Manifest: other.manifest},
		"archive that is not the manifest's layer": {Data: buildLabelledChartTar(t, "0.9.9"), Manifest: fr.manifest},
		"no manifest": {Data: chartTar},
	}
	for name, c := range cases {
		t.Run("miss: "+name, func(t *testing.T) {
			src := newMemSource()
			src.set(key, c)
			n := fr.requests.Load()
			if got := mustRender(t, fr.chartURL(), fr.tag, WithDigest(pin), WithContentSource(src)); !bytes.Equal(got, want) {
				t.Errorf("render differs from the fetched one:\n%s", got)
			}
			if fr.requests.Load() == n {
				t.Error("the supplied bytes were used, want a fetch")
			}
		})
	}
}

// A stored manifest the checks of a pulled child refuse is a miss, pinned or
// not.
func TestRenderChart_ContentSource_OCI_RefusedManifest(t *testing.T) {
	usePlainHTTPRegistry(t)
	chartTar := buildLabelledChartTar(t, "0.1.0")
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", chartTar)
	key := ContentKey{Kind: "chart", URL: fr.chartURL(), Version: fr.tag}
	want := mustRender(t, fr.chartURL(), fr.tag)

	manifestJSON := func(v map[string]any) []byte {
		b, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	layer := map[string]any{"mediaType": registry.ChartLayerMediaType, "digest": sha256Digest(chartTar), "size": len(chartTar)}
	cfg := map[string]any{"mediaType": registry.ConfigMediaType, "digest": sha256Digest([]byte("{}")), "size": 2}
	cases := map[string][]byte{
		"an index": indexOf(t, chartChild(t, "testchart", "0.1.0")).data,
		"a subject": manifestJSON(map[string]any{
			"schemaVersion": 2, "mediaType": imageManifestMediaType, "config": cfg, "layers": []any{layer},
			"subject": map[string]any{"mediaType": imageManifestMediaType, "digest": sha256Digest([]byte("s")), "size": 1},
		}),
		"a manifest-typed layer": manifestJSON(map[string]any{
			"schemaVersion": 2, "mediaType": imageManifestMediaType, "config": cfg,
			"layers": []any{layer, map[string]any{"mediaType": imageManifestMediaType, "digest": sha256Digest([]byte("m")), "size": 1}},
		}),
		"two chart layers": manifestJSON(map[string]any{
			"schemaVersion": 2, "mediaType": imageManifestMediaType, "config": cfg, "layers": []any{layer, layer},
		}),
		// Helm's pull refuses a manifest whose config is not a Helm chart config.
		"a config that is not a chart config": manifestJSON(map[string]any{
			"schemaVersion": 2, "mediaType": imageManifestMediaType, "layers": []any{layer},
			"config": map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": sha256Digest([]byte("{}")), "size": 2},
		}),
		// Helm takes either media type as the chart layer and keeps the last, so
		// a fetch could render the other layer's bytes.
		"a chart layer and a legacy chart layer": manifestJSON(map[string]any{
			"schemaVersion": 2, "mediaType": imageManifestMediaType, "config": cfg,
			"layers": []any{layer, map[string]any{"mediaType": registry.LegacyChartLayerMediaType, "digest": sha256Digest([]byte("l")), "size": 1}},
		}),
		"not JSON": []byte("not json"),
	}
	for name, manifest := range cases {
		t.Run(name, func(t *testing.T) {
			src := newMemSource()
			src.set(key, Content{Data: chartTar, Manifest: manifest})
			n := fr.requests.Load()
			if got := mustRender(t, fr.chartURL(), fr.tag, WithContentSource(src)); !bytes.Equal(got, want) {
				t.Errorf("render differs from the fetched one:\n%s", got)
			}
			if fr.requests.Load() == n {
				t.Error("the stored manifest was accepted, want a fetch")
			}
		})
	}
}

// A hit and a fetch render the same .Chart.Version, with WithDigestVersion
// too: from the pinned digest when pinned, and a fetch when not (the version
// of an unpinned tag may come from an index digest a stored child does not
// carry).
func TestRenderChart_ContentSource_DigestVersion(t *testing.T) {
	usePlainHTTPRegistry(t)
	const tag = "0.1.0"

	t.Run("pin on a plain manifest is served and versioned from the pin", func(t *testing.T) {
		fr := newFakeRegistryWithChart(t, "1", tag, "0.1.0", buildLabelledChartTar(t, "0.1.0"))
		pin := sha256Digest(fr.manifest)
		src := newMemSource()
		opts := []RenderOption{WithDigest(pin), WithDigestVersion(), WithContentSource(src)}
		fetched := mustRender(t, fr.chartURL(), tag, opts...)
		if want := helmControllerForm("0.1.0", pin); !strings.Contains(string(fetched), `version: "`+want+`"`) {
			t.Fatalf("fetched version is not %q:\n%s", want, fetched)
		}
		n := fr.requests.Load()
		if got := mustRender(t, fr.chartURL(), tag, opts...); !bytes.Equal(got, fetched) {
			t.Errorf("hit differs from the fetch:\n%s\nwant:\n%s", got, fetched)
		}
		if fr.requests.Load() != n {
			t.Error("the pinned copy was not served")
		}
	})
	t.Run("pin on the child of a single-child index is served and versioned from the child", func(t *testing.T) {
		child, _ := identityChild(t, "testchart", "0.1.0")
		fr := newFakeIndexRegistry(t, child)
		pin := sha256Digest(child.data)
		src := newMemSource()
		opts := []RenderOption{WithDigest(pin), WithDigestVersion(), WithContentSource(src)}
		fetched := mustRender(t, fr.chartURL(), fr.tag, opts...)
		if want := helmControllerForm("0.1.0", pin); !strings.Contains(string(fetched), `version: "`+want+`"`) {
			t.Fatalf("fetched version is not %q:\n%s", want, fetched)
		}
		if c := src.get(ContentKey{Kind: "chart", URL: fr.chartURL(), Version: fr.tag, Digest: pin}); !bytes.Equal(c.Manifest, child.data) {
			t.Fatalf("Store got a manifest that is not the child's")
		}
		n := fr.requests.Load()
		if got := mustRender(t, fr.chartURL(), fr.tag, opts...); !bytes.Equal(got, fetched) {
			t.Errorf("hit differs from the fetch:\n%s\nwant:\n%s", got, fetched)
		}
		if fr.requests.Load() != n {
			t.Error("the pinned child was not served")
		}
	})
	t.Run("unpinned plain tag fetches and versions from the manifest", func(t *testing.T) {
		fr := newFakeRegistryWithChart(t, "1", tag, "0.1.0", buildLabelledChartTar(t, "0.1.0"))
		src := newMemSource()
		opts := []RenderOption{WithDigestVersion(), WithContentSource(src)}
		fetched := mustRender(t, fr.chartURL(), tag, opts...)
		if want := helmControllerForm("0.1.0", sha256Digest(fr.manifest)); !strings.Contains(string(fetched), `version: "`+want+`"`) {
			t.Fatalf("fetched version is not %q:\n%s", want, fetched)
		}
		n := fr.requests.Load()
		if got := mustRender(t, fr.chartURL(), tag, opts...); !bytes.Equal(got, fetched) {
			t.Errorf("second render differs:\n%s\nwant:\n%s", got, fetched)
		}
		if fr.requests.Load() == n {
			t.Error("an unpinned tag with WithDigestVersion was served from the source, want a fetch")
		}
	})
	t.Run("unpinned single-child-index tag fetches and versions from the index", func(t *testing.T) {
		child, _ := identityChild(t, "testchart", "0.1.0")
		fr := newFakeIndexRegistry(t, child)
		src := newMemSource()
		opts := []RenderOption{WithDigestVersion(), WithContentSource(src)}
		fetched := mustRender(t, fr.chartURL(), fr.tag, opts...)
		if want := helmControllerForm("0.1.0", sha256Digest(fr.manifest)); !strings.Contains(string(fetched), `version: "`+want+`"`) {
			t.Fatalf("fetched version is not the index's %q:\n%s", want, fetched)
		}
		n := fr.requests.Load()
		if got := mustRender(t, fr.chartURL(), fr.tag, opts...); !bytes.Equal(got, fetched) {
			t.Errorf("second render differs:\n%s\nwant:\n%s", got, fetched)
		}
		if fr.requests.Load() == n {
			t.Error("an unpinned tag with WithDigestVersion was served from the source, want a fetch")
		}
	})
}

// A chart from an HTTP repository is keyed by repository, chart and version;
// its hosts are the index host and the archive host, and a stored copy renders
// with both servers closed.
func TestRenderChart_ContentSource_HTTP(t *testing.T) {
	chartBuf := buildMinimalChartTar(t, "testchart", "0.1.0")
	archive := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/testchart-0.1.0.tgz" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(chartBuf)
	}))
	defer archive.Close()
	index := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/index.yaml" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(minimalIndexYAML("testchart", "0.1.0", archive.URL+"/testchart-0.1.0.tgz")))
	}))
	defer index.Close()

	src := newMemSource()
	key := ContentKey{Kind: "chart", URL: index.URL, Chart: "testchart", Version: "0.1.0"}
	want := mustRender(t, index.URL+"/testchart", "0.1.0")
	if got := mustRender(t, index.URL+"/testchart", "0.1.0", WithContentSource(src)); !bytes.Equal(got, want) {
		t.Fatalf("render through a missing source differs:\n%s", got)
	}
	c := src.get(key)
	if !bytes.Equal(c.Data, chartBuf) || len(c.Manifest) != 0 {
		t.Fatalf("Store got archive %d bytes and manifest %d bytes, want the archive alone", len(c.Data), len(c.Manifest))
	}
	hosts := slices.Clone(c.Hosts)
	slices.Sort(hosts)
	wantHosts := []string{hostOf(index), hostOf(archive)}
	slices.Sort(wantHosts)
	if !slices.Equal(hosts, wantHosts) {
		t.Errorf("Store got hosts %v, want %v", hosts, wantHosts)
	}

	index.Close()
	archive.Close()
	if got := mustRender(t, index.URL+"/testchart", "0.1.0", WithContentSource(src)); !bytes.Equal(got, want) {
		t.Errorf("render from the stored copy differs:\n%s", got)
	}
}

// A blob the registry redirects to another host is a host the fetch touched.
func TestRenderChart_ContentSource_OCI_BlobRedirectHost(t *testing.T) {
	usePlainHTTPRegistry(t)
	chartTar := buildLabelledChartTar(t, "0.1.0")
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", chartTar)
	store := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		blob, ok := fr.blobs[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(blob)
	}))
	defer store.Close()
	fr.blobRedirect = store.URL

	src := newMemSource()
	mustRender(t, fr.chartURL(), fr.tag, WithContentSource(src))
	c := src.get(ContentKey{Kind: "chart", URL: fr.chartURL(), Version: fr.tag})
	for _, h := range []string{hostOf(fr.srv), hostOf(store)} {
		if !slices.Contains(c.Hosts, h) {
			t.Errorf("Store hosts %v lack %s", c.Hosts, h)
		}
	}
}

// A registry that answers with a bearer challenge names its token service,
// and a request to it is a host the fetch touched: Hosts lists it beside the
// registry, so a caller checking Hosts against its allowed registries sees it.
func TestRenderChart_ContentSource_OCI_TokenServiceHost(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", buildLabelledChartTar(t, "0.1.0"))
	token := newFakeTokenService(t)
	fr.tokenRealm = token.URL

	src := newMemSource()
	mustRender(t, fr.chartURL(), fr.tag, WithContentSource(src))
	c := src.get(ContentKey{Kind: "chart", URL: fr.chartURL(), Version: fr.tag})
	hosts := slices.Clone(c.Hosts)
	slices.Sort(hosts)
	want := []string{hostOf(fr.srv), hostOf(token)}
	slices.Sort(want)
	if !slices.Equal(hosts, want) {
		t.Errorf("Store got hosts %v, want the registry and its token service %v", c.Hosts, want)
	}
}

// A source that fails never fails a render or changes its output.
func TestRenderChart_ContentSource_ErrorsAreMisses(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", buildLabelledChartTar(t, "0.1.0"))
	want := mustRender(t, fr.chartURL(), fr.tag)

	// The source holds a usable copy and reports it with ok set, beside an
	// error: the error alone makes it a miss, so the registry is still asked.
	src := newMemSource()
	mustRender(t, fr.chartURL(), fr.tag, WithContentSource(src))
	src.lookups, src.stores = nil, nil
	src.lookupErr = errors.New("lookup failed for " + fr.chartURL())
	src.storeErr = errors.New("store failed")
	n := fr.requests.Load()
	got, err := RenderChart(fr.chartURL(), fr.tag, nil, WithContentSource(src))
	if err != nil {
		t.Fatalf("RenderChart: %v", err)
	}
	if !bytes.Equal(got, want) {
		t.Errorf("output changed:\n%s", got)
	}
	if fr.requests.Load() == n {
		t.Error("a copy returned with an error was served, want a fetch")
	}
	if len(src.lookups) != 1 || len(src.stores) != 1 {
		t.Errorf("source asked %d times and stored %d times, want 1 and 1", len(src.lookups), len(src.stores))
	}
}

// A scheme other than oci, http and https is refused as before, and the source
// is never asked for it.
func TestRenderChart_ContentSource_SchemeStillRefused(t *testing.T) {
	src := newMemSource()
	src.set(ContentKey{Kind: "chart", URL: "ftp://example.com/chart", Version: "1.0.0"}, Content{Data: []byte("x")})
	_, err := RenderChart("ftp://example.com/chart", "1.0.0", nil, WithContentSource(src))
	if err == nil || !strings.Contains(err.Error(), "unsupported chart URL") {
		t.Fatalf("err = %v, want the unsupported scheme error", err)
	}
	if len(src.lookups) != 0 {
		t.Errorf("source asked %d times for a refused scheme", len(src.lookups))
	}
}

func TestRenderChartFromArchive(t *testing.T) {
	chartTar := buildLabelledChartTar(t, "0.1.0")
	got, err := RenderChartFromArchive(chartTar, nil)
	if err != nil {
		t.Fatalf("RenderChartFromArchive: %v", err)
	}
	if !strings.Contains(string(got), "helm.sh/chart: testchart-0.1.0\n") {
		t.Errorf("unexpected output:\n%s", got)
	}
	if _, err := RenderChartFromArchive([]byte("not an archive"), nil); err == nil {
		t.Error("expected an error for bytes that are no chart archive")
	}
	for name, opt := range map[string]RenderOption{
		"WithDigest":        WithDigest(sha256Digest([]byte("x"))),
		"WithDigestVersion": WithDigestVersion(),
	} {
		if _, err := RenderChartFromArchive(chartTar, nil, opt); err == nil {
			t.Errorf("%s: expected a refusal, an archive has no manifest digest", name)
		}
	}
	rel, err := RenderChartFromArchive(buildReleaseChartTar(t), nil, WithNamespace("other"), WithReleaseName("rel"))
	if err != nil {
		t.Fatalf("release options: %v", err)
	}
	for _, want := range []string{"name: rel\n", "namespace: other"} {
		if !strings.Contains(string(rel), want) {
			t.Errorf("release options not applied, output lacks %q:\n%s", want, rel)
		}
	}
}

// buildReleaseChartTar builds a chart archive whose one template prints the
// release name and namespace.
func buildReleaseChartTar(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		"relchart/Chart.yaml": "apiVersion: v2\nname: relchart\nversion: 0.1.0\n",
		"relchart/templates/cm.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: {{ .Release.Name }}\n" +
			"  namespace: {{ .Release.Namespace }}\n",
	}
	for path, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: path, Mode: 0o600, Size: int64(len(content))}); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatalf("tar close: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("gzip close: %v", err)
	}
	return buf.Bytes()
}
