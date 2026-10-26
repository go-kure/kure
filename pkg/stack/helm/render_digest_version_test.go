package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

// labelledChartTemplate carries the conventional helm.sh/chart label, as
// `helm create` scaffolds it, and the raw chart version.
const labelledChartTemplate = `apiVersion: v1
kind: ConfigMap
metadata:
  name: test
  labels:
    helm.sh/chart: {{ printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
data:
  version: {{ .Chart.Version | quote }}
`

// buildLabelledChartTar builds a testchart archive at chartVersion whose one
// template is labelledChartTemplate.
func buildLabelledChartTar(t *testing.T, chartVersion string) []byte {
	t.Helper()
	return buildNamedLabelledChartTar(t, "testchart", chartVersion)
}

// buildNamedLabelledChartTar builds an archive of chart name at chartVersion
// whose one template is labelledChartTemplate, so the output names the chart
// that rendered it.
func buildNamedLabelledChartTar(t *testing.T, name, chartVersion string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		name + "/Chart.yaml":        "apiVersion: v2\nname: " + name + "\nversion: " + chartVersion + "\n",
		name + "/templates/cm.yaml": labelledChartTemplate,
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

// helmControllerForm is the version helm-controller installs an OCI chart at,
// written out for the test: the chart's version core, "+", then the first 12
// hex characters of the manifest digest.
func helmControllerForm(core, manifestDigest string) string {
	return core + "+" + strings.TrimPrefix(manifestDigest, "sha256:")[:12]
}

func TestRenderChart_OCI_WithDigestVersion_ChartLabel(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", buildLabelledChartTar(t, "0.1.0"))
	want := helmControllerForm("0.1.0", sha256Digest(fr.manifest))

	out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigestVersion())
	if err != nil {
		t.Fatalf("RenderChart with WithDigestVersion: %v", err)
	}
	wantLabel := "helm.sh/chart: testchart-" + strings.ReplaceAll(want, "+", "_")
	if !strings.Contains(string(out), wantLabel+"\n") {
		t.Errorf("expected label %q in output, got:\n%s", wantLabel, out)
	}
	if !strings.Contains(string(out), `version: "`+want+`"`) {
		t.Errorf("expected chart version %q in output, got:\n%s", want, out)
	}
}

func TestRenderChart_OCI_WithoutDigestVersion_PlainVersion(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", buildLabelledChartTar(t, "0.1.0"))

	out, err := RenderChart(fr.chartURL(), fr.tag, nil)
	if err != nil {
		t.Fatalf("RenderChart: %v", err)
	}
	if !strings.Contains(string(out), "helm.sh/chart: testchart-0.1.0\n") {
		t.Errorf("expected the plain chart version in the label, got:\n%s", out)
	}
}

func TestRenderChart_OCI_WithDigestVersion_ReplacesBuildMetadata(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistryWithChart(t, "1", "0.1.0-rc.1_build.7", "0.1.0-rc.1+build.7", buildLabelledChartTar(t, "0.1.0-rc.1+build.7"))
	want := helmControllerForm("0.1.0-rc.1", sha256Digest(fr.manifest))

	out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigestVersion())
	if err != nil {
		t.Fatalf("RenderChart with WithDigestVersion: %v", err)
	}
	if !strings.Contains(string(out), `version: "`+want+`"`) {
		t.Errorf("expected chart version %q in output, got:\n%s", want, out)
	}
	if strings.Contains(string(out), "build.7") {
		t.Errorf("expected the chart's own build metadata to be replaced, got:\n%s", out)
	}
}

// helm-controller compares a semver tag with the chart version, build
// metadata aside, and refuses a mismatch; a tag that is not semver is not
// compared.
func TestRenderChart_OCI_WithDigestVersion_Tag(t *testing.T) {
	usePlainHTTPRegistry(t)
	cases := map[string]struct {
		tag, chartVersion string
		wantCore          string // empty: refused
	}{
		"other semver tag refused":  {"0.2.0", "0.1.0", ""},
		"not semver, not compared":  {"latest", "0.1.0", "0.1.0"},
		"build metadata aside":      {"0.1.0", "0.1.0+build.7", "0.1.0"},
		"OCI tag form of the chart": {"0.1.0_build.7", "0.1.0+build.7", "0.1.0"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			fr := newFakeRegistryWithChart(t, "1", tc.tag, tc.chartVersion, buildLabelledChartTar(t, tc.chartVersion))
			out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigestVersion())
			if tc.wantCore == "" {
				if err == nil {
					t.Fatalf("expected a tag mismatch error, got output:\n%s", out)
				}
				if !strings.Contains(err.Error(), "does not match chart version") {
					t.Errorf("expected a tag mismatch error, got %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("RenderChart with WithDigestVersion: %v", err)
			}
			if want := helmControllerForm(tc.wantCore, sha256Digest(fr.manifest)); !strings.Contains(string(out), `version: "`+want+`"`) {
				t.Errorf("expected chart version %q in output, got:\n%s", want, out)
			}
		})
	}
}

func TestRenderChart_OCI_WithDigestVersion_AndWithDigest(t *testing.T) {
	usePlainHTTPRegistry(t)
	fr := newFakeRegistryWithChart(t, "1", "0.1.0", "0.1.0", buildLabelledChartTar(t, "0.1.0"))
	served := sha256Digest(fr.manifest)

	t.Run("match", func(t *testing.T) {
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigest(served), WithDigestVersion())
		if err != nil {
			t.Fatalf("RenderChart: %v", err)
		}
		if want := helmControllerForm("0.1.0", served); !strings.Contains(string(out), `version: "`+want+`"`) {
			t.Errorf("expected chart version %q in output, got:\n%s", want, out)
		}
	})
	t.Run("mismatch", func(t *testing.T) {
		pinned := "sha256:" + strings.Repeat("b", 64)
		out, err := RenderChart(fr.chartURL(), fr.tag, nil, WithDigestVersion(), WithDigest(pinned))
		if err == nil {
			t.Fatalf("expected a digest mismatch error, got output:\n%s", out)
		}
		if !strings.Contains(err.Error(), "does not match pinned digest") {
			t.Errorf("expected the WithDigest mismatch error, got %v", err)
		}
	})
}

func TestRenderChart_HTTP_WithDigestVersion_Refused(t *testing.T) {
	var requests atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		http.NotFound(w, r)
	}))
	defer srv.Close()

	_, err := RenderChart(srv.URL+"/testchart", "0.1.0", nil, WithDigestVersion())
	if err == nil {
		t.Fatal("expected WithDigestVersion to be refused for an HTTP repository")
	}
	if !strings.Contains(err.Error(), "not supported for HTTP") {
		t.Errorf("expected an HTTP refusal, got %v", err)
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("expected no request to the repository, got %d", n)
	}
}

// Helm's loader already refuses a non-semver chart version, so these paths of
// setDigestVersion are not reachable through RenderChart; they are pinned
// directly.
func TestSetDigestVersion_Refusals(t *testing.T) {
	digest := "sha256:" + strings.Repeat("c", 64)
	cases := map[string]struct {
		chrt    *v2chart.Chart
		digest  string
		wantErr string
	}{
		"not semver":     {&v2chart.Chart{Metadata: &v2chart.Metadata{Version: "latest"}}, digest, `chart version "latest" is not semver`},
		"no metadata":    {&v2chart.Chart{}, digest, "only Helm v2 API charts"},
		"digest no algo": {&v2chart.Chart{Metadata: &v2chart.Metadata{Version: "0.1.0"}}, strings.Repeat("c", 64), "invalid manifest digest"},
		"digest short":   {&v2chart.Chart{Metadata: &v2chart.Metadata{Version: "0.1.0"}}, "sha256:abc", "invalid manifest digest"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			err := setDigestVersion(tc.chrt, "0.1.0", tc.digest)
			if err == nil {
				t.Fatal("expected an error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("expected %q in error, got %v", tc.wantErr, err)
			}
		})
	}
}
