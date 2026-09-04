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
		// Cheap prefilter: most yaml in a module is not a CRD, and decoding
		// every one of them costs far more than this scan.
		if !bytes.Contains(data, []byte("CustomResourceDefinition")) {
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

// documents decodes the manifest's documents, stopping at the first one that
// does not decode.
//
// That is end-of-file in the ordinary case and, in the other, yaml this
// decoder cannot read: the prefilter that selected the file is a substring
// match, so a Helm template or a kustomize fragment mentioning
// CustomResourceDefinition reaches here legitimately. The two are not
// distinguished because neither yields a definition, and failing the walk over
// a Helm template would make this package unusable against the modules it
// exists to read.
func documents(data []byte) []definition {
	dec := yaml.NewYAMLOrJSONDecoder(bytes.NewReader(data), 4096)
	var out []definition
	for {
		var def definition
		if dec.Decode(&def) != nil {
			return out
		}
		out = append(out, def)
	}
}

// indexFile records every CRD in one possibly multi-document manifest.
func indexFile(data []byte, path string, out Index) error {
	for _, def := range documents(data) {
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
