package helm

import (
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
