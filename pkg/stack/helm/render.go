package helm

import (
	"bytes"
	"maps"
	"path/filepath"
	"slices"
	"strings"

	chartpkg "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/engine"
	"helm.sh/helm/v4/pkg/registry"

	"github.com/go-kure/kure/pkg/errors"
)

// RenderChart pulls a Helm chart from an OCI registry and renders it
// client-side (equivalent to `helm template`), returning multi-doc YAML.
//
// chartURL is an OCI URL of the form oci://registry/repo/chart.
// version is the chart version tag (e.g. "1.16.5").
// values are merged on top of the chart's default values.
func RenderChart(chartURL, version string, values map[string]any) ([]byte, error) {
	client, err := registry.NewClient()
	if err != nil {
		return nil, errors.Wrap(err, "create registry client")
	}
	result, err := client.Pull(chartURL + ":" + version)
	if err != nil {
		return nil, errors.Wrapf(err, "pull chart %s:%s", chartURL, version)
	}
	chrt, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		return nil, errors.Wrap(err, "load chart archive")
	}
	return renderChart(chrt, values)
}

// renderChart renders an already-loaded chart with the given values.
// Exported for testing without OCI connectivity.
func renderChart(chrt chartpkg.Charter, values map[string]any) ([]byte, error) {
	renderVals, err := util.ToRenderValues(chrt, values, common.ReleaseOptions{
		Name:      "release",
		Namespace: "default",
		IsInstall: true,
	}, common.DefaultCapabilities)
	if err != nil {
		return nil, errors.Wrap(err, "build render values")
	}
	rendered, err := engine.Render(chrt, renderVals)
	if err != nil {
		return nil, errors.Wrap(err, "render templates")
	}
	return assembleManifests(rendered), nil
}

// assembleManifests concatenates rendered template strings into multi-doc YAML.
// It skips Helm partial templates (basename starting with "_") and empty output.
// Keys are sorted to guarantee stable output across calls.
func assembleManifests(rendered map[string]string) []byte {
	var buf bytes.Buffer
	for _, name := range slices.Sorted(maps.Keys(rendered)) {
		if strings.HasPrefix(filepath.Base(name), "_") {
			continue
		}
		content := strings.TrimSpace(rendered[name])
		if content == "" {
			continue
		}
		if buf.Len() > 0 {
			buf.WriteString("\n---\n")
		}
		buf.WriteString(content)
	}
	return buf.Bytes()
}
