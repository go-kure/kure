package helm

import (
	"encoding/json"
	"strings"
	"testing"
)

// imageChild returns a container image manifest, no chart, as an index child.
func imageChild(t *testing.T) fakeManifest {
	t.Helper()
	config := []byte(`{"architecture":"amd64","os":"linux"}`)
	layer := []byte("not a chart")
	data, err := json.Marshal(map[string]any{
		"schemaVersion": 2,
		"mediaType":     imageManifestMediaType,
		"config":        map[string]any{"mediaType": "application/vnd.oci.image.config.v1+json", "digest": sha256Digest(config), "size": len(config)},
		"layers": []map[string]any{{
			"mediaType": "application/vnd.oci.image.layer.v1.tar", "digest": sha256Digest(layer), "size": len(layer),
		}},
	})
	if err != nil {
		t.Fatalf("marshal image manifest: %v", err)
	}
	return fakeManifest{
		mediaType: imageManifestMediaType,
		data:      data,
		blobs:     map[string][]byte{sha256Digest(config): config, sha256Digest(layer): layer},
	}
}

// editManifest returns m with its JSON document changed by edit.
func editManifest(t *testing.T, m fakeManifest, edit func(doc map[string]any)) fakeManifest {
	t.Helper()
	var doc map[string]any
	if err := json.Unmarshal(m.data, &doc); err != nil {
		t.Fatalf("unmarshal manifest: %v", err)
	}
	edit(doc)
	data, err := json.Marshal(doc)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	m.data = data
	return m
}

// withSubject returns m naming subject as its OCI subject.
func withSubject(t *testing.T, m, subject fakeManifest) fakeManifest {
	t.Helper()
	return editManifest(t, m, func(doc map[string]any) { doc["subject"] = descriptorOf(subject) })
}

// withoutMediaType returns m without the optional mediaType field in its
// JSON; the registry still serves it under its media type.
func withoutMediaType(t *testing.T, m fakeManifest) fakeManifest {
	t.Helper()
	return editManifest(t, m, func(doc map[string]any) { delete(doc, "mediaType") })
}

// TestRenderChart_OCI_IndexRoot pins which roots RenderChart renders. Helm's
// registry client copies every manifest under the root, its subject included,
// and keeps the last chart layer it meets, in an order that is not fixed, so
// a root reaching more than one manifest may render a different chart on each
// run (go-kure/kure#1102). Such a root is refused, naming the reference, with
// or without WithDigest or WithDigestVersion: an image index naming anything
// but one image manifest, whether or not its JSON carries the optional
// mediaType, and any root naming a subject. An index naming a single chart
// manifest renders as that manifest does.
func TestRenderChart_OCI_IndexRoot(t *testing.T) {
	usePlainHTTPRegistry(t)

	t.Run("one chart manifest renders", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, chartChild(t, "testchart", "0.1.0"))
		out, err := RenderChart(fr.chartURL(), fr.tag, nil)
		if err != nil {
			t.Fatalf("RenderChart of an index holding one chart: %v", err)
		}
		if !strings.Contains(string(out), "kind: ConfigMap") {
			t.Errorf("expected the chart's ConfigMap in output, got:\n%s", out)
		}
	})

	chart := func() fakeManifest { return chartChild(t, "testchart", "0.1.0") }
	other := func() fakeManifest { return chartChild(t, "otherchart", "0.2.0") }
	indexRegistry := func(children ...fakeManifest) func(t *testing.T) *fakeRegistry {
		return func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, children...) }
	}
	nested := chart()
	nested.unlisted = true

	type refusal struct {
		registry func(t *testing.T) *fakeRegistry
		names    string // what the error must name besides the reference
	}
	refused := map[string]refusal{
		"two chart manifests":                 {indexRegistry(chart(), other()), "image index"},
		"two manifests of one chart":          {indexRegistry(chart(), chartChild(t, "testchart", "0.1.1")), "image index"},
		"a nested index":                      {indexRegistry(indexOf(t, nested), nested), "image index"},
		"a chart beside a non-chart manifest": {indexRegistry(chart(), imageChild(t)), "image index"},
		"an index without its mediaType field": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			return newFakeRootRegistry(t, withoutMediaType(t, indexOf(t, a, b)), a, b)
		}, "image index"},
		"a chart manifest naming a subject": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			return newFakeRootRegistry(t, withSubject(t, a, b), b)
		}, "subject"},
		"a one-chart index naming a subject": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			return newFakeRootRegistry(t, withSubject(t, indexOf(t, a), b), a, b)
		}, "subject"},
	}
	variants := map[string]func(fr *fakeRegistry) []RenderOption{
		"":                    func(*fakeRegistry) []RenderOption { return nil },
		", pinned":            func(fr *fakeRegistry) []RenderOption { return []RenderOption{WithDigest(sha256Digest(fr.manifest))} },
		", WithDigestVersion": func(*fakeRegistry) []RenderOption { return []RenderOption{WithDigestVersion()} },
	}
	for name, r := range refused {
		for suffix, options := range variants {
			t.Run(name+suffix, func(t *testing.T) {
				fr := r.registry(t)
				out, err := RenderChart(fr.chartURL(), fr.tag, nil, options(fr)...)
				if err == nil {
					t.Fatalf("expected a root reaching more than one manifest to be refused, got output:\n%s", out)
				}
				if out != nil {
					t.Errorf("expected no output on refusal, got:\n%s", out)
				}
				for _, want := range []string{fr.chartURL() + ":" + fr.tag, r.names, "reference a chart pushed as a single manifest"} {
					if !strings.Contains(err.Error(), want) {
						t.Errorf("error %q does not name %q", err, want)
					}
				}
			})
		}
	}
}
