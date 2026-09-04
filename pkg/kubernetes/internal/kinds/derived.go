package kinds

import (
	"sort"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
	"github.com/go-kure/kure/pkg/kubernetes/internal/upstream"
)

// DerivedScope is one kind's scope as read from the pinned upstream sources,
// with the module the answer came from.
type DerivedScope struct {
	Key     string // group/kind, as Kind.Key() spells it
	Scope   markers.Scope
	Module  string
	Version string
	// Marked records whether this kind's own type declares a
	// +kubebuilder:resource marker. Scope alone cannot say: an unmarked type
	// and a type declaring scope=Namespaced both resolve to ScopeNamespaced,
	// and only the second is an answer rather than the absence of one.
	//
	// Per kind, not per module. A module can mark some of its root types and
	// not others — and internal/upstream deliberately drops a marker block
	// preceding a grouped `type (...)` declaration, since it cannot be
	// attributed to one spec, so those types read as unmarked too. A
	// module-level "this module has markers somewhere" test would pass while
	// exactly the kinds it was meant to protect fell to the default.
	Marked bool
	// ModuleDir is where the module was unpacked, so a kind its markers cannot
	// answer for can be answered from the CRD the module ships.
	ModuleDir string
	// Source records which of the three sources answered. Empty until
	// [ResolveScopes] fills it in: [DeriveScopes] reads markers only.
	Source ScopeSource
}

// ScopeSource names where a resolved scope came from. The generated table
// carries it so a wrong scope can be traced to the thing that claimed it.
type ScopeSource string

const (
	// SourceMarker is the kind's own +kubebuilder:resource marker.
	SourceMarker ScopeSource = "marker"
	// SourceBuiltinTable is builtinClusterScoped, for the modules whose scope
	// the API server rather than a marker defines.
	SourceBuiltinTable ScopeSource = "builtin"
	// SourceShippedCRD is the CustomResourceDefinition the module ships, read
	// when the type carries no marker of its own.
	SourceShippedCRD ScopeSource = "crd"
)

// DeriveScopes reads each kind's scope from the +kubebuilder:resource marker in
// the pinned upstream module sources, returning one entry per kind keyed by
// group/kind.
//
// Built-in k8s.io/api types carry no such marker: the API server, not a
// marker, defines their scope, and upstream's absent-marker default
// (Namespaced) is wrong for the sixteen cluster-scoped built-ins kure
// registers. DeriveScopes reports what the markers say and does not paper over
// that difference — [ResolveScopes] is the function that applies
// builtinClusterScoped on top and is what callers should use.
func DeriveScopes(all []Kind, types map[string]upstream.Type) ([]DerivedScope, error) {
	out := make([]DerivedScope, 0, len(all))
	for _, k := range all {
		t, ok := types[upstream.Key(k.ImportPath, k.TypeName)]
		if !ok {
			return nil, errors.Errorf("kinds: %s: type %s not found in %s; the pinned module source does not declare it", k.GVK, k.TypeName, k.ImportPath)
		}
		scope, err := markers.ResourceScope(t.Doc)
		if err != nil {
			return nil, errors.Wrapf(err, "kinds: %s (%s.%s)", k.GVK, k.ImportPath, k.TypeName)
		}
		out = append(out, DerivedScope{
			Key:       k.Key(),
			Scope:     scope,
			Module:    t.Module,
			Version:   t.Version,
			Marked:    markers.HasResource(t.Doc),
			ModuleDir: t.ModuleDir,
		})
	}
	return out, nil
}

// ImportPaths returns the distinct packages the kinds are declared in, sorted.
// It is the argument to [upstream.Load]: callers load once and pass the result
// to both [ResolveScopes] and the maturity walk, rather than parsing the same
// modules twice.
func ImportPaths(all []Kind) []string {
	paths := map[string]bool{}
	for _, k := range all {
		paths[k.ImportPath] = true
	}
	return sortedKeys(paths)
}

// LoadTypes loads the upstream types for every package the kinds live in.
func LoadTypes(all []Kind) (map[string]upstream.Type, error) {
	return upstream.Load(ImportPaths(all))
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
