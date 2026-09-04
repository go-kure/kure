package kinds

import (
	"sort"

	"github.com/go-kure/kure/pkg/errors"
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

// unmarkedModules are the non-built-in modules verified to ship no
// +kubebuilder:resource marker on any type. Their kinds take controller-gen's
// documented default, Namespaced — which is what controller-gen itself emitted
// into the CRD these modules ship.
//
// The entry is a recorded decision, not a default the derivation fell into: an
// absent marker is indistinguishable from a marker this parser failed to read,
// so a module reaching this state has to be looked at once, against the CRD it
// publishes, and written down here.
//
//   - github.com/cloudnative-pg/plugin-barman-cloud: its one registered kind is
//     ObjectStore (barmancloud.cnpg.io/v1). The module's api/v1 carries
//     +kubebuilder:object:root, :subresource:status and :storageversion but no
//     :resource marker, and the CRD it ships in manifest.yaml declares
//     scope: Namespaced. Verified against v0.14.0.
var unmarkedModules = map[string]bool{
	"github.com/cloudnative-pg/plugin-barman-cloud": true,
}

// UnmarkedModules returns the non-built-in modules among the loaded types that
// declare no +kubebuilder:resource marker on any of their types, sorted.
//
// A module in that state is the one case the derivation cannot see: every one
// of its kinds resolves to Namespaced, which is upstream's default and also
// what a total parse failure looks like. The result must be empty, and it is
// asserted to be — see TestNoRegisteredModuleIsUnmarked, which runs this over
// the real pinned sources, and TestUnmarkedModulesReportsAMarkerLessModule,
// which is the same check against a module that genuinely carries none.
func UnmarkedModules(types map[string]upstream.Type) []string {
	seen := map[string]bool{}
	marked := map[string]bool{}
	for _, t := range types {
		if t.Module == "" || builtinModules[t.Module] {
			continue
		}
		seen[t.Module] = true
		if markers.HasResource(t.Doc) {
			marked[t.Module] = true
		}
	}
	var out []string
	for m := range seen {
		if !marked[m] {
			out = append(out, m)
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
	out := make([]DerivedScope, 0, len(derived))
	for _, d := range derived {
		if !registered[d.Key] {
			return nil, errors.Errorf("kinds: derived a scope for %s, which is not registered", d.Key)
		}
		if builtinModules[d.Module] {
			d.Scope = markers.ScopeNamespaced
			if builtinClusterScoped[d.Key] {
				d.Scope = markers.ScopeCluster
			}
		}
		out = append(out, d)
	}
	return out, nil
}
