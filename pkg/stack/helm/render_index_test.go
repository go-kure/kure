package helm

import (
	"crypto/sha512"
	"encoding/hex"
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

// withManifestLayer returns m with layer, a manifest, appended to its layers.
func withManifestLayer(t *testing.T, m, layer fakeManifest) fakeManifest {
	t.Helper()
	return editManifest(t, m, func(doc map[string]any) { doc["layers"] = append(doc["layers"].([]any), descriptorOf(layer)) })
}

// withManifestConfig returns m with config, a manifest, as its config.
func withManifestConfig(t *testing.T, m, config fakeManifest) fakeManifest {
	t.Helper()
	return editManifest(t, m, func(doc map[string]any) { doc["config"] = descriptorOf(config) })
}

// withEmptyManifestsEntry returns m with a manifests list holding one image
// manifest descriptor without a digest, as an index would list its child.
func withEmptyManifestsEntry(t *testing.T, m fakeManifest) fakeManifest {
	t.Helper()
	return editManifest(t, m, func(doc map[string]any) {
		doc["manifests"] = []any{map[string]any{"mediaType": imageManifestMediaType, "digest": "", "size": 0}}
	})
}

// withManifestsEntry returns m with a manifests list naming child, as an
// index would list it.
func withManifestsEntry(t *testing.T, m, child fakeManifest) fakeManifest {
	t.Helper()
	return editManifest(t, m, func(doc map[string]any) { doc["manifests"] = []any{descriptorOf(child)} })
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
// mediaType, any root naming a subject, any root naming a manifest as its
// config or a layer, which Helm's pull also copies, and an index listing its
// single image manifest without a digest (go-kure/kure#1131). An index naming a single chart manifest renders as
// that manifest does, and is refused when that child's own content is an
// index, names a subject or names a manifest as its config or a layer.
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
		"a chart manifest naming a manifest as a layer": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			return newFakeRootRegistry(t, withManifestLayer(t, a, b), b)
		}, "config or a layer"},
		"a chart manifest naming a manifest as its config": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			return newFakeRootRegistry(t, withManifestConfig(t, a, b), b)
		}, "config or a layer"},
		// Served as an image manifest, these roots are traversed through
		// their config and layers; the manifests list Helm's pull ignores.
		"a chart manifest with a manifests list naming a manifest as a layer": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			return newFakeRootRegistry(t, withEmptyManifestsEntry(t, withManifestLayer(t, a, b)), b)
		}, "config or a layer"},
		"a chart manifest with a manifests list entry without a digest": {func(t *testing.T) *fakeRegistry {
			return newFakeRootRegistry(t, withEmptyManifestsEntry(t, chart()))
		}, "holds both"},
		// Helm's pull keeps a's chart; the list names b, which a child pull
		// would render.
		"a chart manifest with a manifests list naming another chart": {func(t *testing.T) *fakeRegistry {
			a, _ := identityChild(t, "testchart", "0.1.0")
			b, _ := identityChild(t, "otherchart", "0.2.0")
			return newFakeRootRegistry(t, withManifestsEntry(t, a, b), b)
		}, "holds both"},
		"a one-chart index whose child names a subject": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			b.unlisted = true
			return newFakeIndexRegistry(t, withSubject(t, a, b), b)
		}, "subject"},
		"a one-chart index whose child is an index": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			b.unlisted = true
			return newFakeIndexRegistry(t, editManifest(t, a, func(doc map[string]any) { doc["manifests"] = []any{descriptorOf(b)} }), b)
		}, "image index"},
		"a one-chart index whose child names a manifest as a layer": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			b.unlisted = true
			return newFakeIndexRegistry(t, withManifestLayer(t, a, b), b)
		}, "config or a layer"},
		"a one-chart index whose child names a manifest as its config": {func(t *testing.T) *fakeRegistry {
			a, b := chart(), other()
			b.unlisted = true
			return newFakeIndexRegistry(t, withManifestConfig(t, a, b), b)
		}, "config or a layer"},
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

// TestRenderChart_OCI_OneChildIndex pins go-kure/kure#1131: an image index
// naming one image manifest renders that child pulled on its own, by the
// digest the index lists, after the same checks as a child WithDigest
// selects. The tag's pull fetches the chart archive once and the child's own
// pull fetches it again. WithDigestVersion versions it from the index digest
// the tag resolved to, which is what the reference pins.
func TestRenderChart_OCI_OneChildIndex(t *testing.T) {
	usePlainHTTPRegistry(t)
	a, aLayer := identityChild(t, "testchart", "0.1.0")

	t.Run("renders the child's own pull", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, a)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil)
		if err != nil {
			t.Fatalf("RenderChart of a one-chart index: %v", err)
		}
		if !strings.Contains(string(out), "helm.sh/chart: testchart-0.1.0\n") {
			t.Errorf("expected the child's chart testchart-0.1.0, got:\n%s", out)
		}
		if n := fr.blobGetCount(aLayer); n != 2 {
			t.Errorf("chart archive fetched %d times, want 2 (the tag's pull, then the child's own)", n)
		}
	})
	t.Run("WithDigestVersion uses the index digest", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, a)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigestVersion())
		if err != nil {
			t.Fatalf("RenderChart: %v", err)
		}
		if want := helmControllerForm("0.1.0", sha256Digest(fr.manifest)); !strings.Contains(string(out), `version: "`+want+`"`) {
			t.Errorf("expected chart version %q in output, got:\n%s", want, out)
		}
	})
	// The child's own pull resolves chartURL@digest: Helm's registry client
	// takes the digest from Docker-Content-Digest or, without it, computes a
	// sha256 one of a response of known length (the fake's), which a sha512
	// digest cannot match.
	t.Run("sha512-listed child with Docker-Content-Digest", func(t *testing.T) {
		fr, _ := sha512IndexRegistry(t, a)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil)
		if err != nil {
			t.Fatalf("RenderChart: %v", err)
		}
		if !strings.Contains(string(out), "helm.sh/chart: testchart-0.1.0\n") {
			t.Errorf("expected the child's chart testchart-0.1.0, got:\n%s", out)
		}
	})
	t.Run("sha512-listed child without Docker-Content-Digest fails closed", func(t *testing.T) {
		fr, d := sha512IndexRegistry(t, a)
		fr.omitChildDigestHeader = true
		out, err := RenderChart(fr.chartURL(), fr.tag, nil)
		if err == nil {
			t.Fatalf("expected the child's pull to fail, got output:\n%s", out)
		}
		if out != nil {
			t.Errorf("expected no output on failure, got:\n%s", out)
		}
		for _, want := range []string{fr.chartURL() + ":" + fr.tag, d, "digest other than sha256", "known length and no Docker-Content-Digest"} {
			if !strings.Contains(err.Error(), want) {
				t.Errorf("error %q does not name %q", err, want)
			}
		}
	})
}

// sha512IndexRegistry serves an image index listing child under its sha512
// digest, which it returns, and child by that digest.
func sha512IndexRegistry(t *testing.T, child fakeManifest) (*fakeRegistry, string) {
	t.Helper()
	sum := sha512.Sum512(child.data)
	d := "sha512:" + hex.EncodeToString(sum[:])
	data, err := json.Marshal(map[string]any{"schemaVersion": 2, "mediaType": imageIndexMediaType, "manifests": []map[string]any{
		{"mediaType": child.mediaType, "digest": d, "size": len(child.data)},
	}})
	if err != nil {
		t.Fatalf("marshal index: %v", err)
	}
	fr := newFakeRootRegistry(t, fakeManifest{mediaType: imageIndexMediaType, data: data}, child)
	fr.children[d] = child
	return fr, d
}

// identityChild is, as an index child, the manifest of chart name at version
// whose template prints both, so the output says which chart rendered. layer
// is the digest of its chart archive.
func identityChild(t *testing.T, name, version string) (m fakeManifest, layer string) {
	t.Helper()
	chartTar := buildNamedLabelledChartTar(t, name, version)
	manifest, blobs := chartManifest(t, "1", name, version, chartTar)
	return fakeManifest{mediaType: imageManifestMediaType, data: manifest, blobs: blobs}, sha256Digest(chartTar)
}

// TestRenderChart_OCI_IndexChild pins go-kure/kure#1111: WithDigest naming an
// image manifest an image index lists renders that child. The child is pulled
// on its own by its digest: Helm's pull of the tag copies every child and keeps
// the last chart layer it meets, in no fixed order, so its chart is never the
// one rendered. The tag's pull fetches each child's chart archive once and the
// child's own pull fetches that child's again, and no other.
func TestRenderChart_OCI_IndexChild(t *testing.T) {
	usePlainHTTPRegistry(t)
	a, aLayer := identityChild(t, "testchart", "0.1.0")
	b, bLayer := identityChild(t, "otherchart", "0.2.0")

	for name, children := range map[string][]fakeManifest{"listed a, b": {a, b}, "listed b, a": {b, a}} {
		t.Run(name, func(t *testing.T) {
			fr := newFakeIndexRegistry(t, children...)
			out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(sha256Digest(b.data)))
			if err != nil {
				t.Fatalf("RenderChart pinned to an index child: %v", err)
			}
			if !strings.Contains(string(out), "helm.sh/chart: otherchart-0.2.0\n") || strings.Contains(string(out), "testchart") {
				t.Errorf("expected the selected chart otherchart-0.2.0 and no other, got:\n%s", out)
			}
			if n := fr.blobGetCount(bLayer); n != 2 {
				t.Errorf("selected chart archive fetched %d times, want 2 (the tag's pull, then the child's own)", n)
			}
			if n := fr.blobGetCount(aLayer); n != 1 {
				t.Errorf("other chart archive fetched %d times, want 1 (the tag's pull only)", n)
			}
		})
	}

	t.Run("one-chart index", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, b)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(sha256Digest(b.data)))
		if err != nil {
			t.Fatalf("RenderChart pinned to the only child: %v", err)
		}
		if !strings.Contains(string(out), "helm.sh/chart: otherchart-0.2.0\n") {
			t.Errorf("expected the child's chart, got:\n%s", out)
		}
	})
}

// With WithDigestVersion, the version is computed from the selected child's
// digest, and a semver tag must name that child's chart version, build
// metadata aside: a child of another version is selected through a tag that
// is not semver.
func TestRenderChart_OCI_IndexChild_WithDigestVersion(t *testing.T) {
	usePlainHTTPRegistry(t)
	a, _ := identityChild(t, "testchart", "0.1.0")
	b, _ := identityChild(t, "otherchart", "0.2.0")

	t.Run("semver tag of the selected chart", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, a, b)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(sha256Digest(a.data)), WithDigestVersion())
		if err != nil {
			t.Fatalf("RenderChart: %v", err)
		}
		if want := helmControllerForm("0.1.0", sha256Digest(a.data)); !strings.Contains(string(out), `version: "`+want+`"`) {
			t.Errorf("expected chart version %q in output, got:\n%s", want, out)
		}
	})
	t.Run("semver tag of another chart refused", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, a, b)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(sha256Digest(b.data)), WithDigestVersion())
		if err == nil {
			t.Fatalf("expected a tag mismatch error, got output:\n%s", out)
		}
		if !strings.Contains(err.Error(), "does not match chart version") {
			t.Errorf("expected a tag mismatch error, got %v", err)
		}
	})
	t.Run("tag that is not semver", func(t *testing.T) {
		fr := newFakeIndexRegistry(t, a, b)
		fr.tag = "charts"
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(sha256Digest(b.data)), WithDigestVersion())
		if err != nil {
			t.Fatalf("RenderChart: %v", err)
		}
		if want := helmControllerForm("0.2.0", sha256Digest(b.data)); !strings.Contains(string(out), `version: "`+want+`"`) {
			t.Errorf("expected chart version %q in output, got:\n%s", want, out)
		}
	})
}

// A WithDigest digest that differs from an image index's own selects only an
// image manifest the index lists whose content is not an index, names no
// subject and names no manifest as its config or a layer, under a root that
// names no subject; anything else is refused, naming the reference.
func TestRenderChart_OCI_IndexChild_Refused(t *testing.T) {
	usePlainHTTPRegistry(t)
	a, _ := identityChild(t, "testchart", "0.1.0")
	b, _ := identityChild(t, "otherchart", "0.2.0")
	nested := b
	nested.unlisted = true
	nestedIndex := indexOf(t, nested)
	indexShaped := withManifestsEntry(t, b, a)
	hybridRoot := withManifestsEntry(t, a, b)
	subjectChild := withSubject(t, b, a)
	layerManifest := withManifestLayer(t, b, a)
	configManifest := withManifestConfig(t, b, a)
	unlisted := "sha256:" + strings.Repeat("c", 64)

	type refusal struct {
		registry func(t *testing.T) *fakeRegistry
		digest   string
		names    []string // what the error must name besides the reference
	}
	refused := map[string]refusal{
		"a digest the index does not list": {
			func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, a, b) },
			unlisted, []string{unlisted, "lists no manifest"},
		},
		"a nested index child": {
			func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, a, nestedIndex, nested) },
			sha256Digest(nestedIndex.data), []string{sha256Digest(nestedIndex.data), imageIndexMediaType},
		},
		"an image manifest child whose content is an index": {
			func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, a, indexShaped) },
			sha256Digest(indexShaped.data), []string{sha256Digest(indexShaped.data), "selected manifest", "image index"},
		},
		"a child naming a subject": {
			func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, a, subjectChild) },
			sha256Digest(subjectChild.data), []string{sha256Digest(subjectChild.data), "subject"},
		},
		"a child naming a manifest as a layer": {
			func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, a, layerManifest) },
			sha256Digest(layerManifest.data),
			[]string{sha256Digest(layerManifest.data), sha256Digest(a.data), imageManifestMediaType, "config or a layer"},
		},
		"a child naming a manifest as its config": {
			func(t *testing.T) *fakeRegistry { return newFakeIndexRegistry(t, a, configManifest) },
			sha256Digest(configManifest.data),
			[]string{sha256Digest(configManifest.data), sha256Digest(a.data), imageManifestMediaType, "config or a layer"},
		},
		"an index naming a subject": {
			func(t *testing.T) *fakeRegistry {
				return newFakeRootRegistry(t, withSubject(t, indexOf(t, a, b), a), a, b)
			},
			sha256Digest(b.data), []string{"subject"},
		},
		"a chart manifest root listing the pinned chart": {
			func(t *testing.T) *fakeRegistry { return newFakeRootRegistry(t, hybridRoot, b) },
			sha256Digest(b.data), []string{"holds both"},
		},
	}
	for name, r := range refused {
		t.Run(name, func(t *testing.T) {
			fr := r.registry(t)
			out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(r.digest))
			if err == nil {
				t.Fatalf("expected a refusal, got output:\n%s", out)
			}
			if out != nil {
				t.Errorf("expected no output on refusal, got:\n%s", out)
			}
			for _, want := range append([]string{fr.chartURL() + ":" + fr.tag}, r.names...) {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not name %q", err, want)
				}
			}
		})
	}
}

// TestCheckPullRoot_Hybrid pins each way a root's JSON holds both an image
// manifest's fields and an index's manifests list, and the refusal of an
// index listing its single image manifest without a digest. Helm's pull
// fails before RenderChart reaches the latter when the registry serves the
// root as an index, so it is checked here directly.
func TestCheckPullRoot_Hybrid(t *testing.T) {
	child := `[{"mediaType":"` + imageManifestMediaType + `","digest":"sha256:` + strings.Repeat("a", 64) + `"}]`
	blob := `{"mediaType":"application/vnd.cncf.helm.chart.content.v1.tar+gzip","digest":"sha256:` +
		strings.Repeat("b", 64) + `"}`
	for name, tc := range map[string]struct{ root, want string }{
		"a config":                     {`{"schemaVersion":2,"config":` + blob + `,"manifests":` + child + `}`, "holds both"},
		"a layer":                      {`{"schemaVersion":2,"layers":[` + blob + `],"manifests":` + child + `}`, "holds both"},
		"the image manifest mediaType": {`{"schemaVersion":2,"mediaType":"` + imageManifestMediaType + `","manifests":` + child + `}`, "holds both"},
		"an index child without a digest": {`{"schemaVersion":2,"mediaType":"` + imageIndexMediaType +
			`","manifests":[{"mediaType":"` + imageManifestMediaType + `","digest":""}]}`, "without a digest"},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := checkPullRoot([]byte(tc.root))
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("checkPullRoot = %q, %v; want an error naming %q", got, err, tc.want)
			}
		})
	}
}
