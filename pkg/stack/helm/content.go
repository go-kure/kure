package helm

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"net/http"
	"net/url"
	"slices"
	"sync"

	chartpkg "helm.sh/helm/v4/pkg/chart"
	"helm.sh/helm/v4/pkg/chart/loader"
	"helm.sh/helm/v4/pkg/registry"

	"github.com/go-kure/kure/pkg/errors"
)

// ContentKey names the chart a ContentSource is asked for. Every field is
// kept exactly as RenderChart received it, so the source can apply its own
// rules to a URL that may carry a credential.
type ContentKey struct {
	// Kind is "chart".
	Kind string
	// URL is the chart URL as given for an OCI chart ("oci://host/repo/name"),
	// and the repository URL, the chart URL without its last path segment, for
	// an HTTP chart.
	URL string
	// Chart is the chart name, the last path segment of an HTTP chart URL. It
	// is empty for an OCI chart.
	Chart string
	// Version is the version tag, as given.
	Version string
	// Digest is the manifest digest the call is pinned to (WithDigest), empty
	// when it is not.
	Digest string
}

// Content is a fetched chart as a ContentSource stores and returns it.
type Content struct {
	// Data is the chart archive, the .tgz bytes.
	Data []byte
	// Manifest is the manifest of an OCI chart, the bytes of the manifest that
	// names Data as its chart layer: for a chart WithDigest selects from an
	// image index, the selected child's. It is empty for an HTTP chart, which
	// has none.
	Manifest []byte
	// Hosts is every host, with its port where it has one, a request went to
	// during the fetch: the repository index host, the archive host and each
	// redirect hop for an HTTP chart; the registry host, the host of its token
	// service when it answers with a bearer challenge, and each blob redirect
	// for an OCI chart. kure applies no host policy; a caller checks Hosts
	// against its own before it serves a stored copy, and decides for itself
	// whether a token service host counts.
	Hosts []string
}

// ContentSource is a caller's store of charts RenderChart has fetched.
type ContentSource interface {
	// Lookup returns the stored copy of key, or false for none. An error is
	// treated as a miss.
	Lookup(ctx context.Context, key ContentKey) (c Content, ok bool, err error)
	// Store receives a chart after a fetch that succeeded. Its error is
	// ignored.
	Store(ctx context.Context, key ContentKey, c Content) error
}

// WithContentSource lets RenderChart serve an OCI or HTTP chart from src
// instead of the network, and hands src each chart it fetched.
//
// RenderChart asks src.Lookup before any network call and uses the bytes only
// when they pass the checks a fetched chart passes, as far as a stored copy
// allows. Otherwise it fetches as without the option and then calls src.Store
// with the archive, the manifest and the hosts the fetch touched. A failing
// Lookup is a miss and a failing Store is ignored, so a source never fails a
// render, and nothing about it changes the output except which bytes it was
// given. kure's render takes no context, so the source gets
// context.Background().
//
// Checks on a stored OCI copy: its manifest must not be an image index, name a
// subject or name a manifest as its config or a layer; its config must be a
// Helm chart config; it must have one chart layer, whose digest is the
// archive's; the archive must load; and a call
// pinned with WithDigest needs a manifest with the pinned digest, which makes
// the pin the trust root of the copy. The checks of the root of an unpinned
// tag cannot run, because the root is not stored: for an unpinned tag the
// source vouches for which bytes belong to it, as it does for any mutable
// reference. A call with WithDigestVersion and no WithDigest is never served
// from src: a tag that resolved to an image index naming one image manifest
// is versioned from the index digest, which a stored child does not carry. It
// is still fetched, and src.Store still receives it.
//
// The source is asked only for the URL schemes RenderChart accepts, and
// WithDigest and WithDigestVersion keep refusing an HTTP chart.
func WithContentSource(src ContentSource) RenderOption {
	return func(o *renderOptions) { o.source = src }
}

// RenderChartFromArchive renders a chart archive, the bytes of its .tgz, as
// RenderChart does a fetched one, and fetches nothing: kure makes no request.
// Rendering validates the values against the chart's values.schema.json, as
// every render does, and Helm resolves a remote $ref in that schema over the
// network; a chart that has none renders with the network closed. WithDigest
// and WithDigestVersion are refused: an archive has no manifest digest.
func RenderChartFromArchive(archive []byte, values map[string]any, opts ...RenderOption) ([]byte, error) {
	var ro renderOptions
	for _, opt := range opts {
		opt(&ro)
	}
	if ro.digestSet || ro.digestVersion {
		return nil, errors.New("WithDigest and WithDigestVersion are not supported for a chart archive: only an OCI chart has a manifest digest")
	}
	chrt, err := loader.LoadArchive(bytes.NewReader(archive))
	if err != nil {
		return nil, errors.Wrap(err, "load chart archive")
	}
	return renderChart(chrt, values, opts...)
}

// lookup asks the source for key; an error is a miss.
func (ro renderOptions) lookup(key ContentKey) (Content, bool) {
	c, ok, err := ro.source.Lookup(context.Background(), key)
	if err != nil || !ok {
		return Content{}, false
	}
	return c, true
}

// store hands the source a fetched chart; its error is ignored.
func (ro renderOptions) store(key ContentKey, c Content) {
	_ = ro.source.Store(context.Background(), key, c)
}

// asksSourceForOCI reports whether an OCI render may be served from the
// source: not one at a digest version without a pin, see WithContentSource.
func (ro renderOptions) asksSourceForOCI() bool {
	return ro.source != nil && (ro.digestSet || !ro.digestVersion)
}

// ociHit returns the chart a stored OCI copy holds, or false when the copy
// fails a check a fetch would have passed it through; see WithContentSource.
func ociHit(c Content, tag string, ro renderOptions) (chartpkg.Charter, bool) {
	if len(c.Manifest) == 0 {
		return nil, false
	}
	manifestDigest := blobDigest(c.Manifest)
	if ro.digestSet && manifestDigest != ro.digest {
		return nil, false
	}
	if checkChild(c.Manifest, manifestDigest, false) != nil {
		return nil, false
	}
	doc, err := readManifest(c.Manifest)
	if err != nil {
		return nil, false
	}
	// Helm's pull refuses a manifest without a Helm chart config.
	if doc.Config.MediaType != registry.ConfigMediaType {
		return nil, false
	}
	var layers []ociDescriptor
	for _, l := range doc.Layers {
		// Helm takes either media type as the chart layer and keeps the last.
		if l.MediaType == registry.ChartLayerMediaType || l.MediaType == registry.LegacyChartLayerMediaType {
			layers = append(layers, l)
		}
	}
	if len(layers) != 1 || layers[0].Digest != blobDigest(c.Data) {
		return nil, false
	}
	chrt, err := loader.LoadArchive(bytes.NewReader(c.Data))
	if err != nil {
		return nil, false
	}
	if ro.digestVersion && setDigestVersion(chrt, tag, ro.digest) != nil {
		return nil, false
	}
	return chrt, true
}

// blobDigest is the OCI digest of b.
func blobDigest(b []byte) string {
	sum := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// hostSet is the hosts a fetch sent requests to, in the order first seen.
type hostSet struct {
	mu    sync.Mutex
	hosts []string
}

func (s *hostSet) add(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !slices.Contains(s.hosts, host) {
		s.hosts = append(s.hosts, host)
	}
}

func (s *hostSet) list() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.hosts)
}

// recordingTransport records the host of every request, a redirect hop
// included, and sends it on through base.
type recordingTransport struct {
	hosts *hostSet
	base  http.RoundTripper
}

func (t *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.hosts.add(req.URL.Host)
	return t.base.RoundTrip(req)
}

// recordingHTTPTransport returns the transport Helm's HTTP getter would use
// itself, recording the host of every request in hosts: Helm takes a
// *http.Transport only, and its Proxy function is asked once per request,
// redirect hops included.
func recordingHTTPTransport(hosts *hostSet) *http.Transport {
	return &http.Transport{
		DisableCompression: true,
		Proxy: func(req *http.Request) (*url.URL, error) {
			hosts.add(req.URL.Host)
			return http.ProxyFromEnvironment(req)
		},
		TLSClientConfig: &tls.Config{},
	}
}
