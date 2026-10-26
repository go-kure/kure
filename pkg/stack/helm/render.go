package helm

import (
	"bytes"
	"maps"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/Masterminds/semver/v3"
	chartpkg "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/common"
	"helm.sh/helm/v4/pkg/chart/common/util"
	"helm.sh/helm/v4/pkg/chart/loader"
	v2chart "helm.sh/helm/v4/pkg/chart/v2"
	"helm.sh/helm/v4/pkg/engine"
	"helm.sh/helm/v4/pkg/getter"
	"helm.sh/helm/v4/pkg/registry"
	repov1 "helm.sh/helm/v4/pkg/repo/v1"

	"github.com/go-kure/kure/pkg/errors"
)

// renderOptions holds the release identity used to render a chart, the
// digest an OCI chart is pinned to, and whether an OCI chart renders at its
// digest-qualified version. It is kure's own type rather than Helm's
// common.ReleaseOptions so that the public RenderOption signature stays
// independent of Helm's package layout (the release-options type already
// moved once, from v3 chartutil to v4 common).
type renderOptions struct {
	releaseName   string
	namespace     string
	digest        string
	digestSet     bool
	digestVersion bool
}

// RenderOption customizes how a chart is rendered: the release identity, the
// digest an OCI chart must match, or the version an OCI chart renders at.
// Without options, rendering uses release "release" in namespace "default" —
// the defaults renderChart always used prior to this option's introduction.
type RenderOption func(*renderOptions)

// digestPattern is the one digest form WithDigest accepts: sha256, lowercase
// hex, as an OCI registry reports a manifest digest.
var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

// newRegistryClient creates the client renderOCI pulls with. It is a variable
// so tests can point it at a plain-HTTP registry served in-process.
var newRegistryClient = func() (*registry.Client, error) { return registry.NewClient() }

// WithReleaseName sets the release name used during rendering (e.g. for
// {{ .Release.Name }} and the generated resource name helpers).
func WithReleaseName(name string) RenderOption {
	return func(o *renderOptions) { o.releaseName = name }
}

// WithNamespace sets the release namespace used during rendering (e.g. for
// {{ .Release.Namespace }}).
func WithNamespace(namespace string) RenderOption {
	return func(o *renderOptions) { o.namespace = namespace }
}

// WithDigest pins an OCI chart to a manifest digest: "sha256:" followed by 64
// lowercase hex characters. RenderChart still pulls chartURL:version, then
// compares the digest of the manifest it received with digest and returns an
// error naming both when they differ, before rendering anything. A malformed
// digest, the empty string included, is refused before any pull. HTTP
// repositories have no manifest digest, so RenderChart refuses this option
// for them.
func WithDigest(digest string) RenderOption {
	return func(o *renderOptions) { o.digest, o.digestSet = digest, true }
}

// WithDigestVersion renders an OCI chart at the version Flux's helm-controller
// installs it as from an OCIRepository source: the chart's semver version with
// its build metadata replaced by the first 12 hex characters of the pulled
// manifest's digest, so "0.33.0" renders as "0.33.0+3f1c5a9e0b7d". Templates
// that print .Chart.Version, such as the conventional helm.sh/chart label,
// then match the release helm-controller makes. The digest is the one the pull
// received, checked against WithDigest first when both are given. A chart
// version that is not semver, a semver tag ("_" read as "+") naming another
// version than the chart's, or a chart that is not a Helm v2 API chart
// (apiVersion v1 or v2), is refused, as helm-controller refuses to install
// them. HTTP repositories have no manifest digest, so RenderChart refuses this
// option for them. The version is computed for the chart RenderChart loaded:
// when the reference is an image index holding several charts, which chart
// Helm's pull loads is not fixed and may not be the one helm-controller selects.
func WithDigestVersion() RenderOption {
	return func(o *renderOptions) { o.digestVersion = true }
}

// RenderChart pulls a Helm chart and renders it client-side (equivalent to `helm template`),
// returning multi-doc YAML.
//
// OCI registries: chartURL must start with "oci://". Authentication uses the
// local Docker credential store (~/.docker/config.json).
//
// HTTP repositories: chartURL must start with "http://" or "https://", with the
// chart name as the last path segment (e.g. "https://charts.example.com/myapp").
// Only public unauthenticated repositories are supported; basic auth, client TLS,
// and other credential mechanisms are not.
//
// version is the chart version tag (e.g. "1.16.5").
// values are merged on top of the chart's default values.
// opts customizes the release identity (name, namespace); see WithReleaseName
// and WithNamespace. Without opts, rendering uses release "release" in
// namespace "default". WithDigest pins an OCI chart to a manifest digest;
// WithDigestVersion renders it at its digest-qualified version.
func RenderChart(chartURL, version string, values map[string]any, opts ...RenderOption) ([]byte, error) {
	var ro renderOptions
	for _, opt := range opts {
		opt(&ro)
	}
	if ro.digestSet && !digestPattern.MatchString(ro.digest) {
		return nil, errors.Errorf("invalid chart digest %q: must be sha256: followed by 64 lowercase hex characters", ro.digest)
	}
	switch {
	case strings.HasPrefix(chartURL, "oci://"):
		return renderOCI(chartURL, version, values, ro, opts...)
	case strings.HasPrefix(chartURL, "http://"), strings.HasPrefix(chartURL, "https://"):
		if ro.digestSet {
			return nil, errors.Errorf("chart digest pinning is not supported for HTTP repository %q: only OCI charts have a manifest digest", chartURL)
		}
		if ro.digestVersion {
			return nil, errors.Errorf("chart digest version is not supported for HTTP repository %q: only OCI charts have a manifest digest", chartURL)
		}
		return renderHTTP(chartURL, version, values, opts...)
	default:
		return nil, errors.Errorf("unsupported chart URL %q: must start with oci://, http://, or https://", chartURL)
	}
}

func renderOCI(chartURL, version string, values map[string]any, ro renderOptions, opts ...RenderOption) ([]byte, error) {
	client, err := newRegistryClient()
	if err != nil {
		return nil, errors.Wrap(err, "create registry client")
	}
	result, err := client.Pull(chartURL + ":" + version)
	if err != nil {
		return nil, errors.Wrapf(err, "pull chart %s:%s", chartURL, version)
	}
	// result.Manifest.Digest is the digest of the manifest bytes received:
	// the registry client verifies the content against it while pulling.
	if ro.digestSet && result.Manifest.Digest != ro.digest {
		return nil, errors.Errorf("chart %s:%s: pulled manifest digest %s does not match pinned digest %s",
			chartURL, version, result.Manifest.Digest, ro.digest)
	}
	chrt, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		return nil, errors.Wrap(err, "load chart archive")
	}
	if ro.digestVersion {
		if err := setDigestVersion(chrt, version, result.Manifest.Digest); err != nil {
			return nil, errors.Wrapf(err, "chart %s:%s", chartURL, version)
		}
	}
	return renderChart(chrt, values, opts...)
}

// setDigestVersion sets the chart's version to its semver version with the
// build metadata replaced by the first 12 hex characters of digest. This is
// the computation of helm-controller v1.6.4 mutateChartWithSourceRevision
// (with parseSemverFromTag and extractDigestSubString), on the same semver
// library, so the result formats identically. Like helm-controller, it
// refuses a tag that is a semver version ("_" read as "+", the OCI tag form)
// other than the chart's, build metadata aside; a tag that is not semver is
// not compared.
func setDigestVersion(chrt chartpkg.Charter, tag, digest string) error {
	c, ok := chrt.(*v2chart.Chart)
	if !ok || c.Metadata == nil {
		return errors.New("chart digest version: only Helm v2 API charts (apiVersion v1 or v2) are supported")
	}
	ver, err := semver.NewVersion(c.Metadata.Version)
	if err != nil {
		return errors.Errorf("chart digest version: chart version %q is not semver", c.Metadata.Version)
	}
	if tagVer, err := semver.NewVersion(strings.ReplaceAll(tag, "_", "+")); err == nil && !tagVer.Equal(ver) {
		return errors.Errorf("chart digest version: tag %q does not match chart version %q", tag, c.Metadata.Version)
	}
	_, hexPart, found := strings.Cut(digest, ":")
	if !found || len(hexPart) < 12 {
		return errors.Errorf("chart digest version: invalid manifest digest %q", digest)
	}
	withDigest, err := ver.SetMetadata(hexPart[:12])
	if err != nil {
		return errors.Wrap(err, "chart digest version")
	}
	c.Metadata.Version = withDigest.String()
	return nil
}

func renderHTTP(chartURL, version string, values map[string]any, opts ...RenderOption) ([]byte, error) {
	last := strings.LastIndex(chartURL, "/")
	if last <= 0 {
		return nil, errors.Errorf("invalid HTTP chart URL %q: expected https://repo-base/chart-name", chartURL)
	}
	repoURL, chartName := chartURL[:last], chartURL[last+1:]
	if chartName == "" {
		return nil, errors.Errorf("invalid HTTP chart URL %q: missing chart name after last /", chartURL)
	}

	getters := getter.Getters()
	archiveURL, err := repov1.FindChartInRepoURL(repoURL, chartName, getters,
		repov1.WithChartVersion(version))
	if err != nil {
		return nil, errors.Wrapf(err, "find chart %s/%s@%s", repoURL, chartName, version)
	}

	parsed, err := url.Parse(archiveURL)
	if err != nil {
		return nil, errors.Wrapf(err, "parse archive URL %q", archiveURL)
	}
	g, err := getters.ByScheme(parsed.Scheme)
	if err != nil {
		return nil, errors.Wrapf(err, "no getter for scheme %q", parsed.Scheme)
	}
	buf, err := g.Get(archiveURL)
	if err != nil {
		return nil, errors.Wrapf(err, "download chart %s", archiveURL)
	}

	chrt, err := loader.LoadArchive(buf)
	if err != nil {
		return nil, errors.Wrap(err, "load chart archive")
	}
	return renderChart(chrt, values, opts...)
}

// renderChart renders an already-loaded chart with the given values.
// Kept as a separate function so tests can render without OCI connectivity.
func renderChart(chrt chartpkg.Charter, values map[string]any, opts ...RenderOption) ([]byte, error) {
	ro := renderOptions{
		releaseName: "release",
		namespace:   "default",
	}
	for _, opt := range opts {
		opt(&ro)
	}
	renderVals, err := util.ToRenderValues(chrt, values, common.ReleaseOptions{
		Name:      ro.releaseName,
		Namespace: ro.namespace,
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
		if filepath.Base(name) == "NOTES.txt" {
			continue // skip Helm info file — not a Kubernetes manifest
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
