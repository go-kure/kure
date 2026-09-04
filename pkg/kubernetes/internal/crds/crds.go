// Package crds reads the CustomResourceDefinition manifests a pinned module
// ships alongside its Go source, so a kind's scope can be taken from what
// controller-gen actually emitted for it.
//
// It exists for the types whose own doc comment carries no
// +kubebuilder:resource marker. Upstream's default for those is Namespaced,
// but a parser cannot tell that default apart from a marker it failed to read,
// and treating the second as the first puts a metadata.namespace on an object
// that must not carry one. The shipped CRD is not a second guess at the same
// question: it is controller-gen's own answer, generated from the same source
// in the same module, and it states the scope explicitly whether or not a
// marker was needed to produce it.
//
// Only the manifests inside the module directory are read. Nothing is fetched,
// and a module that ships none simply yields no answers.
package crds

import (
	"bytes"
	stderrors "errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"k8s.io/apimachinery/pkg/util/yaml"

	"github.com/go-kure/kure/pkg/errors"
)

// Scope is the scope a CustomResourceDefinition declares.
type Scope string

const (
	// ScopeNamespaced is spec.scope: Namespaced.
	ScopeNamespaced Scope = "Namespaced"
	// ScopeCluster is spec.scope: Cluster.
	ScopeCluster Scope = "Cluster"
)

// skipDirs are directories a module's real CRDs are never in and whose
// contents would be misleading if read: vendored copies of other projects'
// CRDs, and test fixtures that deliberately contain malformed or contrived
// definitions.
var skipDirs = map[string]bool{
	"vendor":       true,
	"testdata":     true,
	"node_modules": true,
	".git":         true,
}

// maxFileSize bounds what is read into memory. Generated CRDs with a full
// OpenAPI schema reach a few megabytes; anything past this is not one.
const maxFileSize = 32 << 20

// definition is the part of a CRD manifest this package reads.
type definition struct {
	APIVersion string `json:"apiVersion"`
	Kind       string `json:"kind"`
	Spec       struct {
		Group string `json:"group"`
		Names struct {
			Kind string `json:"kind"`
		} `json:"names"`
		Scope string `json:"scope"`
	} `json:"spec"`
}

// Index maps "group/Kind" to the scope the module's own CRD manifests declare.
type Index map[string]Scope

// Load walks a module directory and indexes every CustomResourceDefinition it
// ships. An empty dir, or a module shipping no manifests, yields an empty index
// and no error: not shipping CRDs is ordinary, and the caller decides what to
// do about a kind it therefore cannot answer for.
//
// Two manifests declaring different scopes for the same group/kind is an error
// rather than a last-write-wins answer, since which one is authoritative is
// exactly what this package cannot judge.
func Load(dir string) (Index, error) {
	out := Index{}
	if dir == "" {
		return out, nil
	}
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDirs[d.Name()] {
				return fs.SkipDir
			}
			return nil
		}
		if ext := filepath.Ext(path); ext != ".yaml" && ext != ".yml" {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return errors.Wrapf(err, "crds: stat %s", path)
		}
		if info.Size() > maxFileSize {
			return nil
		}
		data, err := os.ReadFile(path) //nolint:gosec // path comes from walking the module directory the caller named, which is a read-only module cache entry
		if err != nil {
			return errors.Wrapf(err, "crds: reading %s", path)
		}
		if !declaresCRD(data) || isTemplated(data) {
			return nil
		}
		if err := indexFile(data, path, out); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// declaresCRD reports whether the file declares a CustomResourceDefinition
// document, as a cheap prefilter: most yaml in a module is not a CRD, and
// decoding all of it costs far more than this scan.
//
// It matches a `kind:` key at the start of a line rather than the bare word
// anywhere in the file. A plain substring match also selects every Helm
// template, kustomize overlay and piece of documentation that merely mentions
// the word — files this package cannot decode and, since [documents] refuses to
// read a declaration only partway, would now fail the whole walk over. The
// match is deliberately narrow in the other direction too: an indented `kind:`
// belongs to spec.names, not to the document.
func declaresCRD(data []byte) bool {
	for _, want := range crdKindLines {
		if bytes.HasPrefix(data, want[1:]) || bytes.Contains(data, want) {
			return true
		}
	}
	return false
}

// crdKindLines are the spellings of the document's own kind key, each with the
// leading newline that anchors it to the start of a line.
var crdKindLines = [][]byte{
	[]byte("\nkind: CustomResourceDefinition"),
	[]byte("\nkind: \"CustomResourceDefinition\""),
	[]byte("\nkind: 'CustomResourceDefinition'"),
}

// isTemplated reports whether the file is a template rather than a manifest.
//
// A Helm chart's templates are not definitions: they are the input a rendering
// step turns into one, and half of one is not a partial answer but a different
// document. metallb ships exactly this — charts/metallb/charts/crds/templates/
// crds.yaml, whose first documents are ordinary CRDs and whose third uses
// `{{ .Release.Namespace }}` as a map key. Reading it stops mid-file, which is
// how a templated file came to be a scope source at all; the real definitions
// are in config/crd/bases alongside it.
//
// A CRD whose OpenAPI descriptions happen to contain the delimiter is skipped
// too. That costs nothing it should not: a kind whose scope no readable source
// states is then an error, which is this package's whole stance, rather than a
// scope taken from a file it could only partly read.
func isTemplated(data []byte) bool { return bytes.Contains(data, []byte("{{")) }

// documents decodes every document in the manifest, or fails.
//
// End-of-file ends it. Anything else is an error naming the file and the
// document's index, never a short read: stopping quietly at a document this
// decoder cannot handle drops every definition after it, which loses a kind's
// only answer, or one half of a scope conflict, with nothing to show that
// anything was skipped. The file was selected by [declaresCRD], so reaching
// here at all means it declares a CustomResourceDefinition document, and a
// declaration this package cannot finish reading is a question to decline —
// the same stance as a kind no source can answer for.
func documents(data []byte, path string) ([]definition, error) {
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var out []definition
	for i := 0; ; i++ {
		var def definition
		err := dec.Decode(&def)
		if stderrors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, errors.Wrapf(err, "crds: %s: document %d does not decode, so the documents after it cannot be read", path, i)
		}
		out = append(out, def)
	}
}

// indexFile records every CRD in one possibly multi-document manifest.
func indexFile(data []byte, path string, out Index) error {
	docs, err := documents(data, path)
	if err != nil {
		return err
	}
	for _, def := range docs {
		if def.Kind != "CustomResourceDefinition" || !strings.HasPrefix(def.APIVersion, "apiextensions.k8s.io/") {
			continue
		}
		// A kustomize patch fragment carries the CRD's apiVersion, kind and
		// name and nothing else — cnpg ships one per kind under
		// config/crd/patches to inject a cert-manager annotation. It states no
		// group, no names.kind and no scope, so it is not a definition of
		// anything and must not be read as one, let alone rejected for the
		// scope it was never going to carry.
		if def.Spec.Group == "" || def.Spec.Names.Kind == "" {
			continue
		}
		scope, err := parseScope(def.Spec.Scope)
		if err != nil {
			return errors.Wrapf(err, "crds: %s", path)
		}
		key := def.Spec.Group + "/" + def.Spec.Names.Kind
		if prev, ok := out[key]; ok && prev != scope {
			return errors.Errorf("crds: %s: %s is declared %s here and %s elsewhere in the same module", path, key, scope, prev)
		}
		out[key] = scope
	}
	return nil
}

// parseScope rejects anything but the two scopes the API defines. A CRD with a
// missing or misspelled scope is a manifest this package must not guess about.
func parseScope(value string) (Scope, error) {
	switch Scope(value) {
	case ScopeNamespaced:
		return ScopeNamespaced, nil
	case ScopeCluster:
		return ScopeCluster, nil
	default:
		return "", errors.Errorf("unrecognised spec.scope %q, want %q or %q", value, ScopeNamespaced, ScopeCluster)
	}
}
