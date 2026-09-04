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
}

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
			Key:     k.Key(),
			Scope:   scope,
			Module:  t.Module,
			Version: t.Version,
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
