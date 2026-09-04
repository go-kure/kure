package kinds

import (
	"sort"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes/internal/crds"
	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
	"github.com/go-kure/kure/pkg/kubernetes/internal/upstream"
)

// builtinModules are the modules whose types carry no +kubebuilder:resource
// markers, because their scope is defined by the API server rather than by a
// generator marker. Their kinds take their scope from builtinClusterScoped
// below; every other module's kinds are derived from its markers.
//
// This set is deliberately explicit rather than inferred from "the module
// happens to have no markers": inferring it would silently move a module into
// the hand-kept table the day upstream reorganised its comments, which is the
// failure this whole work item exists to remove. TestBuiltinModulesCarryNoScopeMarkers
// fires if upstream ever does start marking these, so the switch is a
// decision, not a drift.
var builtinModules = map[string]bool{
	"k8s.io/api":                     true,
	"k8s.io/apiextensions-apiserver": true,
}

// builtinClusterScoped are the registered built-in kinds that carry no
// namespace. Every other built-in is namespaced.
//
// Sixteen entries replace the 128-entry hand-seeded pair of sets: the CRD
// modules that make up the rest are read from their own markers.
var builtinClusterScoped = set(
	"/ComponentStatus",
	"/Namespace",
	"/Node",
	"/PersistentVolume",
	"/RangeAllocation",
	"apiextensions.k8s.io/CustomResourceDefinition",
	"networking.k8s.io/IPAddress",
	"networking.k8s.io/IngressClass",
	"networking.k8s.io/ServiceCIDR",
	"rbac.authorization.k8s.io/ClusterRole",
	"rbac.authorization.k8s.io/ClusterRoleBinding",
	"storage.k8s.io/CSIDriver",
	"storage.k8s.io/CSINode",
	"storage.k8s.io/StorageClass",
	"storage.k8s.io/VolumeAttachment",
	"storage.k8s.io/VolumeAttributesClass",
)

// UnmarkedKinds returns the registered kinds whose own type declares no
// +kubebuilder:resource marker, outside the built-in modules, sorted.
//
// This is what the marker alone cannot answer: such a kind resolves to
// Namespaced, which is both upstream's documented default and what a failed
// parse looks like. [ResolveScopes] answers for these from the CRD the module
// ships and fails if it ships none.
func UnmarkedKinds(all []Kind, types map[string]upstream.Type) []string {
	var out []string
	for _, k := range all {
		t, ok := types[upstream.Key(k.ImportPath, k.TypeName)]
		if !ok {
			continue
		}
		if t.Module == "" || builtinModules[t.Module] {
			continue
		}
		if !markers.HasResource(t.Doc) {
			out = append(out, k.Key())
		}
	}
	sort.Strings(out)
	return out
}

// ResolveScopes returns every kind's scope: read from the upstream
// +kubebuilder:resource marker for the CRD modules that carry them, and from
// builtinClusterScoped for the built-in modules that do not.
//
// types is the loaded upstream source, from [LoadTypes]. It is an argument
// rather than something this function loads for itself because the maturity
// walk needs the same map, and loading the pinned module sources twice is the
// most expensive thing the generator does.
func ResolveScopes(all []Kind, types map[string]upstream.Type) ([]DerivedScope, error) {
	derived, err := DeriveScopes(all, types)
	if err != nil {
		return nil, err
	}
	registered := map[string]bool{}
	for _, k := range all {
		registered[k.Key()] = true
	}
	shipped := newCRDIndexes()
	out := make([]DerivedScope, 0, len(derived))
	for _, d := range derived {
		if !registered[d.Key] {
			return nil, errors.Errorf("kinds: derived a scope for %s, which is not registered", d.Key)
		}
		switch {
		case builtinModules[d.Module]:
			d.Scope = markers.ScopeNamespaced
			if builtinClusterScoped[d.Key] {
				d.Scope = markers.ScopeCluster
			}
			d.Source = SourceBuiltinTable
		case d.Marked:
			d.Source = SourceMarker
		default:
			scope, err := shipped.scopeOf(d)
			if err != nil {
				return nil, err
			}
			d.Scope = scope
			d.Source = SourceShippedCRD
		}
		out = append(out, d)
	}
	return out, nil
}

// crdIndexes caches one CRD index per module directory. Indexing walks the
// whole module, so it happens once per module and only when a kind actually
// needs it — most kinds are answered by their own marker.
type crdIndexes struct {
	byDir map[string]crds.Index
}

func newCRDIndexes() *crdIndexes { return &crdIndexes{byDir: map[string]crds.Index{}} }

// scopeOf answers for a kind whose own type carries no +kubebuilder:resource
// marker, from the CRD its module ships.
//
// It is an error, not the namespaced default, when the module ships no CRD for
// the kind. The default and a marker this parser failed to read produce the
// same answer, so an unmarked kind with no second source is a question kure
// cannot answer — and answering it wrong puts a metadata.namespace on an object
// that must not carry one. The same strictness the unparsable-marker rule
// applies.
func (c *crdIndexes) scopeOf(d DerivedScope) (markers.Scope, error) {
	if d.ModuleDir == "" {
		return markers.ScopeNamespaced, errors.Errorf("kinds: %s (%s) declares no +kubebuilder:resource marker on its own type and its module directory is unknown, so the shipped CRD cannot be read", d.Key, d.Module)
	}
	index, ok := c.byDir[d.ModuleDir]
	if !ok {
		var err error
		index, err = crds.Load(d.ModuleDir)
		if err != nil {
			return markers.ScopeNamespaced, errors.Wrapf(err, "kinds: %s (%s)", d.Key, d.Module)
		}
		c.byDir[d.ModuleDir] = index
	}
	scope, ok := index[d.Key]
	if !ok {
		return markers.ScopeNamespaced, errors.Errorf("kinds: %s (%s) declares no +kubebuilder:resource marker on its own type and its module ships no CustomResourceDefinition for it; the namespaced default cannot be told apart from a marker that was not read, so the scope has to come from somewhere", d.Key, d.Module)
	}
	if scope == crds.ScopeCluster {
		return markers.ScopeCluster, nil
	}
	return markers.ScopeNamespaced, nil
}
