package helm

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"helm.sh/helm/v4/pkg/chart/common"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
)

// minimalChart builds a minimal v2 chart for use in tests without OCI connectivity.
func minimalChart(name string, templates []*common.File, defaults map[string]interface{}) *v2chart.Chart {
	return &v2chart.Chart{
		Metadata: &v2chart.Metadata{
			Name:       name,
			Version:    "0.1.0",
			APIVersion: v2chart.APIVersionV2,
		},
		Templates: templates,
		Values:    defaults,
	}
}

func TestRenderChart_ValuesOverrideDefaults(t *testing.T) {
	chrt := minimalChart("testchart", []*common.File{
		{
			Name: "templates/configmap.yaml",
			Data: []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: {{ .Values.name }}
data:
  color: {{ .Values.color }}`),
		},
	}, map[string]interface{}{
		"name":  "default-name",
		"color": "blue",
	})

	out, err := renderChart(chrt, map[string]any{"name": "override-name"})
	if err != nil {
		t.Fatalf("renderChart returned error: %v", err)
	}
	yaml := string(out)
	if !strings.Contains(yaml, "name: override-name") {
		t.Errorf("expected override value in output, got:\n%s", yaml)
	}
	// default value for color should be preserved when not overridden
	if !strings.Contains(yaml, "color: blue") {
		t.Errorf("expected default value for color in output, got:\n%s", yaml)
	}
}

func TestRenderChart_EmptyValues(t *testing.T) {
	chrt := minimalChart("testchart", []*common.File{
		{
			Name: "templates/ns.yaml",
			Data: []byte(`apiVersion: v1
kind: Namespace
metadata:
  name: test`),
		},
	}, nil)

	out, err := renderChart(chrt, nil)
	if err != nil {
		t.Fatalf("renderChart returned error: %v", err)
	}
	if len(out) == 0 {
		t.Error("expected non-empty output")
	}
}

func TestAssembleManifests_SkipsPartials(t *testing.T) {
	rendered := map[string]string{
		"mychart/templates/deployment.yaml": "kind: Deployment",
		"mychart/templates/_helpers.tpl":    "{{- define \"helper\" -}}helper{{- end -}}",
		"mychart/templates/service.yaml":    "kind: Service",
	}
	out := assembleManifests(rendered)
	yaml := string(out)
	if strings.Contains(yaml, "_helpers") || strings.Contains(yaml, "helper") {
		t.Errorf("partial template should not appear in output, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "Deployment") {
		t.Errorf("expected Deployment in output, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "Service") {
		t.Errorf("expected Service in output, got:\n%s", yaml)
	}
}

func TestAssembleManifests_SkipsEmptyTemplates(t *testing.T) {
	rendered := map[string]string{
		"mychart/templates/real.yaml":  "kind: ConfigMap",
		"mychart/templates/empty.yaml": "   \n   ",
	}
	out := assembleManifests(rendered)
	yaml := string(out)
	if !strings.Contains(yaml, "ConfigMap") {
		t.Errorf("expected ConfigMap in output, got:\n%s", yaml)
	}
	// only one resource, no separator needed
	if strings.Contains(yaml, "---") {
		t.Errorf("unexpected document separator for single resource, got:\n%s", yaml)
	}
}

func TestAssembleManifests_SeparatesMultipleResources(t *testing.T) {
	rendered := map[string]string{
		"mychart/templates/a.yaml": "kind: A",
		"mychart/templates/b.yaml": "kind: B",
	}
	out := assembleManifests(rendered)
	yaml := string(out)
	if !strings.Contains(yaml, "---") {
		t.Errorf("expected document separator between resources, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "kind: A") || !strings.Contains(yaml, "kind: B") {
		t.Errorf("expected both resources in output, got:\n%s", yaml)
	}
}

func TestAssembleManifests_EmptyInput(t *testing.T) {
	out := assembleManifests(map[string]string{})
	if len(out) != 0 {
		t.Errorf("expected empty output for empty input, got: %q", out)
	}
}

func TestRenderChart_HTTP(t *testing.T) {
	chartBuf := buildMinimalChartTar(t, "testchart", "0.1.0")
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			w.Header().Set("Content-Type", "application/yaml")
			fmt.Fprint(w, minimalIndexYAML("testchart", "0.1.0", srvURL+"/testchart-0.1.0.tgz"))
		case "/testchart-0.1.0.tgz":
			w.Header().Set("Content-Type", "application/gzip")
			w.Write(chartBuf)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	got, err := RenderChart(srvURL+"/testchart", "0.1.0", nil)
	if err != nil {
		t.Fatalf("RenderChart HTTP: %v", err)
	}
	if len(got) == 0 {
		t.Error("expected non-empty YAML output")
	}
}

func TestRenderChart_UnsupportedScheme_ReturnsError(t *testing.T) {
	_, err := RenderChart("ftp://example.com/chart", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for unsupported scheme")
	}
}

func TestRenderChart_HTTP_EmptyChartName(t *testing.T) {
	_, err := RenderChart("https://charts.example.com/", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for URL with trailing slash and empty chart name")
	}
}

func buildMinimalChartTar(t *testing.T, name, version string) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	files := map[string]string{
		name + "/Chart.yaml":        fmt.Sprintf("apiVersion: v2\nname: %s\nversion: %s\n", name, version),
		name + "/templates/cm.yaml": "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: test\n",
	}
	for path, content := range files {
		hdr := &tar.Header{Name: path, Mode: 0o600, Size: int64(len(content))}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatalf("tar header: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("tar write: %v", err)
		}
	}
	tw.Close()
	gz.Close()
	return buf.Bytes()
}

func minimalIndexYAML(name, version, url string) string {
	return fmt.Sprintf(
		"apiVersion: v1\nentries:\n  %s:\n  - name: %s\n    version: %s\n    urls:\n      - %s\ngenerated: \"2024-01-01T00:00:00Z\"\n",
		name, name, version, url,
	)
}

func TestRenderChart_OCI_PullError(t *testing.T) {
	_, err := RenderChart("oci://localhost:0/nonexistent/chart", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for unreachable OCI registry")
	}
}

func TestRenderChart_HTTP_SlashOnlyPath(t *testing.T) {
	_, err := RenderChart("http://", "1.0.0", nil)
	if err == nil {
		t.Fatal("expected error for bare http:// URL with no path")
	}
}

func TestRenderChart_HTTP_LoadArchiveError(t *testing.T) {
	corrupt := []byte("not a gzip archive")
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			fmt.Fprint(w, minimalIndexYAML("badchart", "0.1.0", srvURL+"/badchart-0.1.0.tgz"))
		default:
			w.Write(corrupt)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	_, err := RenderChart(srvURL+"/badchart", "0.1.0", nil)
	if err == nil {
		t.Fatal("expected error for corrupt chart archive")
	}
}

func TestRenderChart_TemplateError(t *testing.T) {
	chrt := minimalChart("testchart", []*common.File{
		{
			Name: "templates/bad.yaml",
			Data: []byte(`{{ .Values.undefined | required "must set undefined" }}`),
		},
	}, nil)

	_, err := renderChart(chrt, nil)
	if err == nil {
		t.Fatal("expected error for template that calls required on missing value")
	}
}

func TestRenderChart_HTTP_BadIndexYAML(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/index.yaml" {
			fmt.Fprint(w, "this: is: not: valid: yaml: !!!")
		} else {
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	_, err := RenderChart(srv.URL+"/anychart", "0.1.0", nil)
	if err == nil {
		t.Fatal("expected error for malformed index.yaml")
	}
}

func TestRenderChart_HTTP_DownloadError(t *testing.T) {
	var srvURL string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/index.yaml":
			fmt.Fprint(w, minimalIndexYAML("mychart", "0.1.0", srvURL+"/mychart-0.1.0.tgz"))
		default:
			// return HTTP 500 to cause download error
			http.Error(w, "internal server error", http.StatusInternalServerError)
		}
	}))
	defer srv.Close()
	srvURL = srv.URL

	_, err := RenderChart(srvURL+"/mychart", "0.1.0", nil)
	if err == nil {
		t.Fatal("expected error when chart download fails")
	}
}

func TestAssembleManifests_SkipsNotesTxt(t *testing.T) {
	rendered := map[string]string{
		"mychart/templates/deployment.yaml": "kind: Deployment",
		"mychart/NOTES.txt":                 "Some helpful notes",
	}
	out := assembleManifests(rendered)
	yaml := string(out)
	if strings.Contains(yaml, "helpful notes") {
		t.Errorf("NOTES.txt should not appear in output, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "Deployment") {
		t.Errorf("expected Deployment in output, got:\n%s", yaml)
	}
}

func TestAssembleManifests_StableOrder(t *testing.T) {
	rendered := map[string]string{
		"mychart/templates/z-last.yaml":   "kind: Z",
		"mychart/templates/a-first.yaml":  "kind: A",
		"mychart/templates/m-middle.yaml": "kind: M",
	}

	out1 := assembleManifests(rendered)
	out2 := assembleManifests(rendered)

	if string(out1) != string(out2) {
		t.Errorf("assembleManifests is non-deterministic:\nfirst:  %s\nsecond: %s", out1, out2)
	}

	// sorted key order: a-first < m-middle < z-last
	s := string(out1)
	posA := strings.Index(s, "kind: A")
	posM := strings.Index(s, "kind: M")
	posZ := strings.Index(s, "kind: Z")
	if posA < 0 || posM < 0 || posZ < 0 {
		t.Fatalf("expected all three kinds in output, got:\n%s", s)
	}
	if !(posA < posM && posM < posZ) {
		t.Errorf("expected sorted order A < M < Z, got positions A=%d M=%d Z=%d in:\n%s", posA, posM, posZ, s)
	}
}
