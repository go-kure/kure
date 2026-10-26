package helm

import (
	"bytes"
	"encoding/json"
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
// lowercase hex characters. RenderChart renders the manifest with that digest
// and nothing else. It still pulls chartURL:version. When the manifest it
// received has another digest and is an OCI image index listing an image
// manifest with digest, RenderChart pulls that child on its own, by digest,
// and renders it: this selects one chart out of a multi-chart index. The tag
// then only finds the index; the content is fixed by digest. Any other
// difference is an error naming both digests, returned before rendering
// anything, as is a digest the index lists as anything but an image manifest
// and a selected child that is itself an index, names a subject or names a
// manifest as its config or a layer. A malformed digest, the empty string
// included, is refused before any pull. HTTP repositories have no manifest
// digest, so RenderChart refuses this option for them.
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
// option for them. A root that reaches more than one manifest is refused
// before any chart is loaded (see RenderChart), so the version is not computed
// for a chart picked among several. A child WithDigest selects from an image
// index is versioned from its own digest, and a semver tag must name that
// child's chart version. The single child of an index naming one image
// manifest, rendered without WithDigest selecting it, is versioned from the
// index digest the tag resolved to.
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
//
// An OCI reference that resolves to an image index is rendered only when the
// index names exactly one image manifest, whether or not the index carries
// its optional mediaType field, or when WithDigest names one of the image
// manifests it lists, which is then pulled and rendered on its own. Any other
// index is refused with an error naming the reference, before any chart is
// loaded, with or without WithDigest or WithDigestVersion: one naming several
// manifests, one holding a nested index, and one holding a chart manifest
// beside a manifest that is no chart. A root manifest naming a subject, index
// or not, a root naming a manifest as its config or a layer, a root holding
// both an image manifest's config, layers or media type and an index's
// manifests list, and an index listing its single image manifest without a
// digest, are refused the same way. The single child of an index naming one
// image manifest is pulled on its own, by the digest the index lists, and
// rendered from that pull, after the checks a child WithDigest selects
// passes: one more pull of a child the tag's pull has already fetched.
// Reference a chart pushed as a single manifest instead, or pin the child to
// render with WithDigest.
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
	versionDigest := result.Manifest.Digest
	if ro.digestSet && result.Manifest.Digest != ro.digest {
		result, err = pullIndexChild(client, chartURL, version, result, ro.digest)
		if err != nil {
			return nil, err
		}
		versionDigest = result.Manifest.Digest
	} else {
		child, err := checkPullRoot(result.Manifest.Data)
		if err != nil {
			return nil, errors.Wrapf(err, "chart %s:%s", chartURL, version)
		}
		if child != "" {
			if result, err = pullChild(client, chartURL, version, child, false); err != nil {
				return nil, err
			}
		}
	}
	chrt, err := loader.LoadArchive(bytes.NewReader(result.Chart.Data))
	if err != nil {
		return nil, errors.Wrap(err, "load chart archive")
	}
	if ro.digestVersion {
		if err := setDigestVersion(chrt, version, versionDigest); err != nil {
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

// The OCI media types of an image index and an image manifest, as the
// image-spec defines them.
const (
	imageIndexMediaType    = "application/vnd.oci.image.index.v1+json"
	imageManifestMediaType = "application/vnd.oci.image.manifest.v1+json"
)

// checkPullRoot refuses a pulled root manifest that can reach more than one
// manifest. Helm's registry client copies every manifest under the root, a
// subject included, and keeps the last chart layer it meets, in an order that
// is not fixed, so such a root may render a different chart on each pull,
// even pinned to its digest. Refused are a root naming a subject, whatever
// its kind, a root naming a manifest as its config or a layer, whatever its
// JSON also holds, a root holding both an image manifest's config, layers or
// media type and an index's manifests list, and an OCI image index naming
// anything but a single image manifest with a digest.
// An index is recognized by its mediaType or, as that field is optional, by
// its manifests list. The index lists its children without their content, so
// a child that is no chart counts too, and a nested index is refused whatever
// it holds. For an index naming a single image manifest, the digest of that
// child is returned: the child's own content is checked once it is pulled on
// its own (pullChild). Any other root is accepted; one that is not JSON, which
// Helm's pull has already decoded, is an error.
func checkPullRoot(root []byte) (child string, err error) {
	doc, err := readManifest(root)
	if err != nil {
		return "", errors.Wrap(err, "read pulled root manifest")
	}
	if doc.Subject != nil {
		return "", subjectError("the reference's root manifest", doc)
	}
	// Checked whatever the JSON looks like: a root served as an image manifest
	// is traversed through its config and layers even if it also carries a
	// manifests list.
	if d, ok := manifestEdge(doc); ok {
		return "", manifestEdgeError("the reference's root manifest", d,
			"reference a chart pushed as a single manifest whose config and layers are blobs")
	}
	if doc.isHybrid() {
		return "", hybridError()
	}
	if !doc.isIndex() {
		return "", nil
	}
	manifests := doc.manifests()
	if len(manifests) == 1 && manifests[0].MediaType == imageManifestMediaType {
		if manifests[0].Digest == "" {
			return "", errors.New("the reference's root manifest lists its only image manifest without a digest, " +
				"so the chart Helm's registry client kept cannot be checked: reference a chart pushed as a single manifest, not as an index")
		}
		return manifests[0].Digest, nil
	}
	kinds := make([]string, 0, len(manifests))
	for _, m := range manifests {
		kinds = append(kinds, m.MediaType)
	}
	return "", errors.Errorf("the reference resolves to an OCI image index naming %d manifest(s) [%s]; "+
		"only an index naming exactly one image manifest is rendered, as Helm's registry client may render any chart under it: "+
		"reference a chart pushed as a single manifest, not as an index",
		len(manifests), strings.Join(kinds, ", "))
}

// pullIndexChild renders WithDigest's digest when the pulled root is not it:
// when root is an OCI image index listing an image manifest with that digest,
// it pulls that child on its own, by digest, and returns it. The chart Helm
// kept from the root's pull is never used: that pull copies every child and
// keeps the last chart layer it meets, in no fixed order. A root naming a
// subject, a root also holding an image manifest's config, layers or media
// type, a digest the index does not list or lists as anything but an image
// manifest, and a child that is itself an index, names a subject or names a
// manifest as its config or a layer, are refused. A root that is no index
// gives the digest mismatch error.
func pullIndexChild(client *registry.Client, chartURL, version string, root *registry.PullResult, digest string) (*registry.PullResult, error) {
	doc, err := readManifest(root.Manifest.Data)
	if err != nil {
		return nil, errors.Wrapf(err, "chart %s:%s: read pulled root manifest", chartURL, version)
	}
	if !doc.isIndex() {
		return nil, errors.Errorf("chart %s:%s: pulled manifest digest %s does not match pinned digest %s",
			chartURL, version, root.Manifest.Digest, digest)
	}
	if doc.Subject != nil {
		return nil, errors.Wrapf(subjectError("the reference's root manifest", doc), "chart %s:%s", chartURL, version)
	}
	if doc.isHybrid() {
		return nil, errors.Wrapf(hybridError(), "chart %s:%s", chartURL, version)
	}
	manifests := doc.manifests()
	i := slices.IndexFunc(manifests, func(m ociDescriptor) bool { return m.Digest == digest })
	if i < 0 {
		listed := make([]string, 0, len(manifests))
		for _, m := range manifests {
			listed = append(listed, m.Digest)
		}
		return nil, errors.Errorf("chart %s:%s: pulled manifest digest %s does not match pinned digest %s, "+
			"and that image index lists no manifest with the pinned digest [%s]",
			chartURL, version, root.Manifest.Digest, digest, strings.Join(listed, ", "))
	}
	if mt := manifests[i].MediaType; mt != imageManifestMediaType {
		return nil, errors.Errorf("chart %s:%s: pinned digest %s names a manifest of media type %s in the image index; "+
			"only an image manifest child is rendered", chartURL, version, digest, mt)
	}
	return pullChild(client, chartURL, version, digest, true)
}

// pullChild pulls the image index child digest on its own, by digest, and
// returns it once checkChild accepts its content. selected tells a child
// WithDigest selects from the single child of an index naming one image
// manifest, for the error to name it and its remedy.
func pullChild(client *registry.Client, chartURL, version, digest string, selected bool) (*registry.PullResult, error) {
	child, err := client.Pull(chartURL + "@" + digest)
	if err != nil {
		if !strings.HasPrefix(digest, "sha256:") {
			err = errors.Wrap(err, "a child listed with a digest other than sha256 fails its own pull when the registry sends it with a known length and no Docker-Content-Digest")
		}
		return nil, errors.Wrapf(err, "chart %s:%s: pull index child %s", chartURL, version, digest)
	}
	if child.Manifest.Digest != digest {
		return nil, errors.Errorf("chart %s:%s: pulled index child digest %s does not match requested digest %s",
			chartURL, version, child.Manifest.Digest, digest)
	}
	if err := checkChild(child.Manifest.Data, digest, selected); err != nil {
		return nil, errors.Wrapf(err, "chart %s:%s", chartURL, version)
	}
	return child, nil
}

// manifestMediaTypes are the media types oras-go's copy, under Helm's pull,
// fetches as a manifest and copies on from, wherever a descriptor names one.
var manifestMediaTypes = []string{
	imageIndexMediaType,
	imageManifestMediaType,
	"application/vnd.oci.artifact.manifest.v1+json",
	"application/vnd.docker.distribution.manifest.v2+json",
	"application/vnd.docker.distribution.manifest.list.v2+json",
}

// checkChild refuses an image index child, pulled on its own, that can reach
// more than one manifest: one whose content is an image index, by its
// mediaType or a manifests list of any length, one naming a subject, and one
// whose config or a layer has a manifest media type, which Helm's pull copies
// on from as another manifest. selected is as for pullChild: a child
// WithDigest selects is named "the selected manifest" and the remedy is to
// pin another; the single child of an index is named "the index's only
// manifest" and the remedy is to reference a chart pushed as a single
// manifest.
func checkChild(data []byte, digest string, selected bool) error {
	what := "the index's only manifest " + digest
	indexRemedy := "reference a chart pushed as a single manifest, not as an index"
	edgeRemedy := "reference a chart pushed as a single manifest whose config and layers are blobs"
	if selected {
		what = "the selected manifest " + digest
		indexRemedy = "pin an image manifest"
		edgeRemedy = "pin a chart manifest whose config and layers are blobs"
	}
	doc, err := readManifest(data)
	if err != nil {
		return errors.Wrapf(err, "read %s", what)
	}
	if doc.Subject != nil {
		return subjectError(what, doc)
	}
	if doc.isIndex() {
		return errors.Errorf("%s is itself an image index, which Helm's registry client may render any chart under: %s",
			what, indexRemedy)
	}
	if d, ok := manifestEdge(doc); ok {
		return manifestEdgeError(what, d, edgeRemedy)
	}
	return nil
}

// manifestEdge returns the first of doc's config and layers whose media type
// is a manifest media type, which Helm's pull copies on from as another
// manifest.
func manifestEdge(doc manifestDoc) (ociDescriptor, bool) {
	for _, d := range append([]ociDescriptor{doc.Config}, doc.Layers...) {
		if slices.Contains(manifestMediaTypes, d.MediaType) {
			return d, true
		}
	}
	return ociDescriptor{}, false
}

// manifestEdgeError refuses what, a manifest naming d, a manifest, as its
// config or a layer.
func manifestEdgeError(what string, d ociDescriptor, remedy string) error {
	return errors.Errorf("%s names %s of manifest media type %s as its config or a layer, "+
		"which Helm's registry client pulls too, so it may render the chart under either: %s",
		what, d.Digest, d.MediaType, remedy)
}

// manifestDoc is what RenderChart reads of a pulled manifest: its media type,
// its subject, for an image manifest its config and layers and, for an image
// index, the manifests it lists.
type manifestDoc struct {
	MediaType string `json:"mediaType"`
	Subject   *struct {
		MediaType string `json:"mediaType"`
		Digest    string `json:"digest"`
	} `json:"subject"`
	Config    ociDescriptor    `json:"config"`
	Layers    []ociDescriptor  `json:"layers"`
	Manifests *[]ociDescriptor `json:"manifests"`
}

// ociDescriptor is what RenderChart reads of an OCI descriptor.
type ociDescriptor struct {
	MediaType string `json:"mediaType"`
	Digest    string `json:"digest"`
}

func readManifest(data []byte) (manifestDoc, error) {
	var doc manifestDoc
	err := json.Unmarshal(data, &doc)
	return doc, err
}

// isIndex reports an OCI image index, recognized by its mediaType or, as that
// field is optional, by its manifests list.
func (d manifestDoc) isIndex() bool {
	return d.MediaType == imageIndexMediaType || d.Manifests != nil
}

// isHybrid reports a manifest whose JSON holds both an index's manifests
// list and an image manifest's config, layers or mediaType. Helm's registry
// client reads it as the media type the registry serves it under, so the
// chart it keeps and the child the list names may differ.
func (d manifestDoc) isHybrid() bool {
	return d.Manifests != nil &&
		(d.MediaType == imageManifestMediaType || d.Config != (ociDescriptor{}) || len(d.Layers) > 0)
}

// hybridError refuses a root manifest isHybrid reports.
func hybridError() error {
	return errors.New("the reference's root manifest holds both an image manifest's config, layers or media type " +
		"and an index's manifests list, so the chart Helm's registry client kept may not be the one its list names: " +
		"reference a chart pushed as a single manifest or as an index, not as both")
}

func (d manifestDoc) manifests() []ociDescriptor {
	if d.Manifests == nil {
		return nil
	}
	return *d.Manifests
}

// subjectError refuses what, a manifest naming a subject.
func subjectError(what string, d manifestDoc) error {
	return errors.Errorf("%s names a subject (%s %s), which Helm's registry client pulls too, "+
		"so it may render the chart under either: reference a chart pushed as a single manifest without a subject",
		what, d.Subject.MediaType, d.Subject.Digest)
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
