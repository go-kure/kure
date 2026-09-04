// Package markers parses the controller-gen marker comments that upstream API
// types carry, so kure derives a kind's scope and a field's maturity from the
// pinned module sources instead of a hand-kept table.
//
// Only the markers kure actually reads are handled: +kubebuilder:resource (for
// scope) and +featureGate (for maturity). Anything else is ignored.
//
// The parser is deliberately strict. A marker it cannot understand is an
// error, never a default, because the one legitimate default — an absent
// +kubebuilder:resource:scope, which upstream defines as Namespaced — is
// indistinguishable from a marker whose spelling this parser failed to match.
// Silently treating the second as the first would emit a namespaced wrapper for
// a cluster-scoped kind and put a metadata.namespace on an object that must not
// carry one.
package markers

import (
	"strings"

	"github.com/go-kure/kure/pkg/errors"
)

// Scope is the namespacing an API type declares.
type Scope int

const (
	// ScopeNamespaced is upstream's default: objects live in a namespace.
	ScopeNamespaced Scope = iota
	// ScopeCluster objects carry no namespace.
	ScopeCluster
)

func (s Scope) String() string {
	if s == ScopeCluster {
		return "Cluster"
	}
	return "Namespaced"
}

const (
	resourcePrefix = "+kubebuilder:resource:"
	// resourceBare is the marker with no value list. It declares nothing about
	// scope, so it resolves to the default like an absent marker does.
	resourceBare    = "+kubebuilder:resource"
	featureGatePfx  = "+featureGate="
	scopeKey        = "scope"
	clusterValue    = "Cluster"
	namespacedValue = "Namespaced"
)

// ResourceScope returns the scope a type's doc comment declares.
//
// The two forms observed across the modules kure pins are the bare
//
//	+kubebuilder:resource:scope=Cluster
//
// and, with the value quoted and embedded in a list whose other values contain
// braced commas,
//
//	+kubebuilder:resource:categories={cilium,ciliumbgp},path="…",scope="Cluster",shortName={ccg}
//
// Both are accepted. A doc comment with no +kubebuilder:resource marker, or one
// whose marker declares no scope, returns ScopeNamespaced — upstream's default.
// A marker carrying an unrecognised scope value, or a malformed value list,
// returns an error.
func ResourceScope(doc string) (Scope, error) {
	scope := ScopeNamespaced
	found := false
	for _, line := range markerLines(doc) {
		if line == resourceBare {
			continue
		}
		if !strings.HasPrefix(line, resourcePrefix) {
			continue
		}
		values, err := splitValues(strings.TrimPrefix(line, resourcePrefix))
		if err != nil {
			return ScopeNamespaced, errors.Wrapf(err, "marker %q", line)
		}
		for _, v := range values {
			key, value, ok := strings.Cut(v, "=")
			if !ok || strings.TrimSpace(key) != scopeKey {
				continue
			}
			s, err := parseScope(unquote(strings.TrimSpace(value)))
			if err != nil {
				return ScopeNamespaced, errors.Wrapf(err, "marker %q", line)
			}
			if found && s != scope {
				return ScopeNamespaced, errors.Errorf("marker %q: scope declared twice with different values (%s and %s)", line, scope, s)
			}
			scope, found = s, true
		}
	}
	return scope, nil
}

// HasResource reports whether a doc comment carries a +kubebuilder:resource
// marker at all, in either form.
//
// [ResourceScope] cannot answer this: it returns ScopeNamespaced both for a
// type that declares scope=Namespaced and for a type that declares nothing,
// which is correct for deriving a scope and useless for deciding whether a
// module was generated with resource markers in the first place. A module
// carrying none needs an explicitly recorded exception rather than the default,
// so something has to be able to tell the two apart.
func HasResource(doc string) bool {
	for _, line := range markerLines(doc) {
		if line == resourceBare || strings.HasPrefix(line, resourcePrefix) {
			return true
		}
	}
	return false
}

// FeatureGates returns the feature-gate names a doc comment declares, in the
// order they appear. A field carries one marker per gate.
func FeatureGates(doc string) []string {
	var gates []string
	for _, line := range markerLines(doc) {
		if !strings.HasPrefix(line, featureGatePfx) {
			continue
		}
		if name := strings.TrimSpace(strings.TrimPrefix(line, featureGatePfx)); name != "" {
			gates = append(gates, name)
		}
	}
	return gates
}

// markerLines returns the marker comments in doc: the lines that, once the
// comment slashes and surrounding space are stripped, begin with "+".
func markerLines(doc string) []string {
	var out []string
	for _, raw := range strings.Split(doc, "\n") {
		line := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(raw), "//"))
		if strings.HasPrefix(line, "+") {
			out = append(out, line)
		}
	}
	return out
}

// parseScope maps a marker's scope value to a Scope. Only upstream's two
// spellings are accepted; anything else is an error rather than a default.
func parseScope(value string) (Scope, error) {
	switch value {
	case clusterValue:
		return ScopeCluster, nil
	case namespacedValue:
		return ScopeNamespaced, nil
	default:
		return ScopeNamespaced, errors.Errorf("unrecognised scope %q, want %q or %q", value, clusterValue, namespacedValue)
	}
}

// splitValues splits a marker's comma-separated value list, treating commas
// inside {braces} or "quotes" as part of a value rather than separators.
// Unbalanced braces or an unterminated quote are errors: the resulting split
// would be wrong in a way that could drop the scope entry entirely.
func splitValues(list string) ([]string, error) {
	var (
		out    []string
		cur    strings.Builder
		depth  int
		quoted bool
	)
	for _, r := range list {
		switch {
		case r == '"':
			quoted = !quoted
			cur.WriteRune(r)
		case quoted:
			cur.WriteRune(r)
		case r == '{':
			depth++
			cur.WriteRune(r)
		case r == '}':
			depth--
			if depth < 0 {
				return nil, errors.Errorf("unbalanced %q in value list %q", "}", list)
			}
			cur.WriteRune(r)
		case r == ',' && depth == 0:
			out = append(out, cur.String())
			cur.Reset()
		default:
			cur.WriteRune(r)
		}
	}
	if quoted {
		return nil, errors.Errorf("unterminated quote in value list %q", list)
	}
	if depth != 0 {
		return nil, errors.Errorf("unbalanced %q in value list %q", "{", list)
	}
	out = append(out, cur.String())
	return out, nil
}

// unquote strips one layer of surrounding double quotes.
func unquote(s string) string {
	if len(s) >= 2 && strings.HasPrefix(s, `"`) && strings.HasSuffix(s, `"`) {
		return s[1 : len(s)-1]
	}
	return s
}
