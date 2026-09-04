// Package maturity reports which fields of the API types kure builds are
// gated, alpha, beta or deprecated upstream.
//
// kure does not warn, reject or filter on any of this: it reports, and a
// consumer with cluster knowledge decides. The table exists because the
// failure it describes is silent. For built-in types the API server does not
// reject a field whose feature gate is off — it clears the field and admits
// the object, so the manifest reads as applied and is not.
package maturity

import (
	"sort"
	"strings"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
	"github.com/go-kure/kure/pkg/kubernetes/internal/upstream"
)

// Stability is what a field's own documentation says about its maturity.
type Stability string

const (
	// StabilityStable is the absence of any maturity signal.
	StabilityStable Stability = ""
	// StabilityAlpha is declared by the field's doc comment.
	StabilityAlpha Stability = "alpha"
	// StabilityBeta is declared by the field's doc comment.
	StabilityBeta Stability = "beta"
	// StabilityDeprecated is a doc comment opening with Go's "Deprecated:".
	StabilityDeprecated Stability = "deprecated"
)

// Entry is one field of one reachable type that carries a maturity signal.
type Entry struct {
	ImportPath string    // package declaring the type
	TypeName   string    // Go type declaring the field
	Field      string    // the field's JSON name, or its Go name when untagged
	GoField    string    // the Go field name
	Gates      []string  // +featureGate names, in source order
	Stability  Stability // what the doc comment says
	Module     string
	Version    string
}

// Root is a type the walk starts from: a registered kind's own struct.
type Root struct {
	ImportPath string
	TypeName   string
}

// Walk returns every maturity-carrying field of every type reachable from
// roots, sorted by import path, type and field.
//
// Status types are skipped. A kind's status is reported by the cluster, never
// constructed by a caller, so a gate on a status field says nothing about
// whether a manifest kure builds will apply as written — and status subtrees
// carry the large majority of the markers in k8s.io/api.
//
// Types in packages that were not loaded are not followed. The loaded set is
// the packages the registered kinds live in; apimachinery's shared metadata
// types are outside it by design, since they are the same for every kind and
// carry no construction-side gates.
func Walk(roots []Root, types map[string]upstream.Type) ([]Entry, error) {
	if len(roots) == 0 {
		return nil, errors.Errorf("maturity: no roots to walk")
	}
	w := newWalker(types)
	for _, r := range roots {
		key := upstream.Key(r.ImportPath, r.TypeName)
		if _, ok := types[key]; !ok {
			return nil, errors.Errorf("maturity: root %s is not among the loaded types", key)
		}
		w.visit(key)
	}
	out := make([]Entry, 0, len(w.entries))
	for _, e := range w.entries {
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].ImportPath != out[j].ImportPath {
			return out[i].ImportPath < out[j].ImportPath
		}
		if out[i].TypeName != out[j].TypeName {
			return out[i].TypeName < out[j].TypeName
		}
		return out[i].Field < out[j].Field
	})
	return out, nil
}

// SkippedStatusTypes returns the distinct status types the walk declined to
// enter, so a test can assert the skip is doing what it claims rather than
// quietly swallowing half the API.
//
// Distinct, not one entry per reference: a status type is commonly named by
// several kinds (every Flux kind reaches meta.ReconcileRequestStatus), and
// counting each reference separately would report a number that says how
// densely the API cross-references its status types, not how many of them the
// walk refused.
func SkippedStatusTypes(roots []Root, types map[string]upstream.Type) []string {
	w := newWalker(types)
	for _, r := range roots {
		if _, ok := types[upstream.Key(r.ImportPath, r.TypeName)]; ok {
			w.visit(upstream.Key(r.ImportPath, r.TypeName))
		}
	}
	out := make([]string, 0, len(w.skipped))
	for k := range w.skipped {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

type walker struct {
	types   map[string]upstream.Type
	visited map[string]bool
	entries map[string]Entry
	skipped map[string]bool
}

func newWalker(types map[string]upstream.Type) *walker {
	return &walker{
		types:   types,
		visited: map[string]bool{},
		entries: map[string]Entry{},
		skipped: map[string]bool{},
	}
}

// visit records the maturity-carrying fields of one type and descends into the
// types its fields name. Each type is entered once: a type's fields do not
// change with the path taken to reach it, and entering it once terminates on
// the self-referential types the API contains (JSONSchemaProps names itself).
func (w *walker) visit(key string) {
	if w.visited[key] {
		return
	}
	w.visited[key] = true
	t, ok := w.types[key]
	if !ok {
		return
	}
	for _, f := range t.Fields {
		if e, ok := entryFor(t, f); ok {
			w.entries[key+"."+e.Field] = e
		}
		child, ok := resolve(t, f.TypeExpr)
		if !ok {
			continue
		}
		if isStatusType(child) {
			w.skipped[child] = true
			continue
		}
		w.visit(child)
	}
}

// entryFor builds an Entry when a field carries a maturity signal.
func entryFor(t upstream.Type, f upstream.Field) (Entry, bool) {
	gates := markers.FeatureGates(f.Doc)
	stability := stabilityOf(f.Doc)
	if len(gates) == 0 && stability == StabilityStable {
		return Entry{}, false
	}
	name := f.JSONName
	if name == "" {
		name = f.Name
	}
	return Entry{
		ImportPath: t.ImportPath,
		TypeName:   t.Name,
		Field:      name,
		GoField:    f.Name,
		Gates:      gates,
		Stability:  stability,
		Module:     t.Module,
		Version:    t.Version,
	}, true
}

// stabilityOf reads a field's declared maturity from its doc comment.
//
// The feature-gate marker is the precise signal; this prose scan is the
// best-effort complement for fields upstream documents as alpha or beta
// without gating them in a marker. It matches whole words only, so a field
// documented as "alphabetical" is not reported as alpha, and it requires
// Go's conventional "Deprecated:" prefix on a line rather than the word
// appearing anywhere. It can still over-report a field whose prose merely
// mentions another field's maturity; the gate list is what a consumer should
// act on.
func stabilityOf(doc string) Stability {
	for _, line := range strings.Split(doc, "\n") {
		if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "//")), "Deprecated:") {
			return StabilityDeprecated
		}
	}
	switch {
	case containsWord(doc, "alpha"):
		return StabilityAlpha
	case containsWord(doc, "beta"):
		return StabilityBeta
	default:
		return StabilityStable
	}
}

// containsWord reports whether doc contains word as a whole word, ignoring
// case and ASCII word characters on either side.
func containsWord(doc, word string) bool {
	lower := strings.ToLower(doc)
	for i := 0; ; {
		j := strings.Index(lower[i:], word)
		if j < 0 {
			return false
		}
		start := i + j
		end := start + len(word)
		if !isWordChar(byteAt(lower, start-1)) && !isWordChar(byteAt(lower, end)) {
			return true
		}
		i = end
	}
}

func byteAt(s string, i int) byte {
	if i < 0 || i >= len(s) {
		return 0
	}
	return s[i]
}

func isWordChar(b byte) bool {
	return b == '_' || (b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// isStatusType reports whether a type key names a status type, which the walk
// does not enter.
func isStatusType(key string) bool {
	name := key
	if i := strings.LastIndex(key, "."); i >= 0 {
		name = key[i+1:]
	}
	return strings.HasSuffix(name, "Status")
}

// resolve maps a field's type expression to the key of the named type it
// refers to, following pointers, slices and map values. It returns false for
// builtin types, for unnamed types, and for packages that were not loaded.
func resolve(t upstream.Type, typeExpr string) (string, bool) {
	expr := baseType(typeExpr)
	if expr == "" || isBuiltin(expr) {
		return "", false
	}
	pkg, name, qualified := strings.Cut(expr, ".")
	if !qualified {
		return upstream.Key(t.ImportPath, expr), true
	}
	path, ok := t.Imports[pkg]
	if !ok {
		return "", false
	}
	return upstream.Key(path, name), true
}

// baseType strips the pointer, slice and map-value wrappers off a field's type
// expression, leaving the named type the field ultimately refers to. A map's
// key is never a struct worth walking, so only its value is kept.
func baseType(expr string) string {
	for {
		switch {
		case strings.HasPrefix(expr, "*"):
			expr = expr[1:]
		case strings.HasPrefix(expr, "[]"):
			expr = expr[2:]
		case strings.HasPrefix(expr, "map["):
			end := strings.Index(expr, "]")
			if end < 0 {
				return ""
			}
			expr = expr[end+1:]
		default:
			return expr
		}
	}
}

// builtins are the predeclared type names an API struct field can name; none
// of them is a type the walk should try to resolve.
var builtins = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true, "int8": true,
	"int16": true, "int32": true, "int64": true, "rune": true, "string": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"uintptr": true, "any": true, "interface{}": true,
}

func isBuiltin(name string) bool { return builtins[name] }
