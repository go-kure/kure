package kinds

import (
	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
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

// ResolveScopes returns every kind's scope: read from the upstream
// +kubebuilder:resource marker for the CRD modules that carry them, and from
// builtinClusterScoped for the built-in modules that do not.
func ResolveScopes(all []Kind) ([]DerivedScope, error) {
	derived, err := DeriveScopes(all)
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
