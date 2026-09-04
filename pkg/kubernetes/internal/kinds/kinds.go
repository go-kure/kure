// Package kinds enumerates the object kinds registered in the pkg/kubernetes
// scheme and states each one's scope. It is the single source both the
// constructor generator (internal/gen) and the identity test read, so the two
// can never disagree about which kinds exist.
//
// Every scope is derived from the pinned upstream module sources — the
// +kubebuilder:resource markers, the CustomResourceDefinitions the modules
// ship, and, for the built-in modules whose scope the API server rather than a
// generator defines, the explicit set in scope.go. Nothing here is a default:
// a kind no source can answer for is an error, so a scheme change that adds a
// kind fails generation until upstream states its scope. See
// pkg/kubernetes/README.md § 9.
package kinds

import (
	"reflect"
	"sort"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/errors"
	"github.com/go-kure/kure/pkg/kubernetes"
	"github.com/go-kure/kure/pkg/kubernetes/internal/markers"
)

// Kind is one registered object kind.
type Kind struct {
	GVK        schema.GroupVersionKind
	Type       reflect.Type // the struct type (not the pointer)
	ImportPath string       // Go import path of the type's package
	TypeName   string       // Go type name
	Package    string       // kure package directory under pkg/kubernetes ("" for the root)
	Namespaced bool         // derived; see [Registered]
}

// Key returns the group/kind key the scope lookups are indexed by ("apps/Deployment", "/Pod").
func (k Kind) Key() string { return k.GVK.Group + "/" + k.GVK.Kind }

// packageRoutes maps upstream import-path prefixes to the kure package that
// hosts the generated wrappers for their kinds.
var packageRoutes = []struct{ prefix, pkg string }{
	{"k8s.io/api/", ""},
	{"k8s.io/apiextensions-apiserver/", ""},
	{"sigs.k8s.io/gateway-api/", ""},
	{"github.com/cert-manager/", "certmanager"},
	{"github.com/cilium/", "cilium"},
	{"github.com/cloudnative-pg/", "cnpg"},
	{"github.com/external-secrets/", "externalsecrets"},
	{"github.com/fluxcd/", "fluxcd"},
	{"github.com/controlplaneio-fluxcd/", "fluxcd"},
	{"go.universe.tf/metallb/", "metallb"},
	{"github.com/prometheus-operator/", "prometheus"},
	{"github.com/backube/volsync/", "volsync"},
}

// memo caches the one derivation this package performs. Resolving the scopes
// parses every pinned module's Go source and, where a type carries no marker,
// walks that module for the CRD it ships. For a fixed set of pins the answer
// is the same every time, and the identity test alone asks for it twice.
var memo struct {
	once sync.Once
	all  []Kind
	err  error
}

// Registered returns every object kind in the scheme, sorted by package, kind,
// group and version, each with its scope derived from the pinned upstream
// sources. A kind is an object when its pointer type implements client.Object
// and not client.ObjectList; apimachinery's meta types are excluded.
//
// The result is memoized, and each caller gets its own copy of the slice: a
// test that rewrites a Kind must not change what a later caller in the same
// process sees.
func Registered() ([]Kind, error) {
	memo.once.Do(func() { memo.all, memo.err = derive() })
	if memo.err != nil {
		return nil, memo.err
	}
	out := make([]Kind, len(memo.all))
	copy(out, memo.all)
	return out, nil
}

// derive classifies the scheme and resolves every kind's scope. It is separate
// from [Registered] only so the memoization is not tangled with the work.
func derive() ([]Kind, error) {
	if err := kubernetes.RegisterSchemes(); err != nil {
		return nil, err
	}
	all, err := classify(kubernetes.Scheme.AllKnownTypes())
	if err != nil {
		return nil, err
	}
	types, err := LoadTypes(all)
	if err != nil {
		return nil, err
	}
	resolved, err := ResolveScopes(all, types)
	if err != nil {
		return nil, err
	}
	scopes := make(map[string]markers.Scope, len(resolved))
	for _, d := range resolved {
		scopes[d.Key] = d.Scope
	}
	for i := range all {
		scope, ok := scopes[all[i].Key()]
		if !ok {
			// ResolveScopes returns one entry per kind or an error, so this is
			// unreachable today. It stays because the alternative to noticing
			// is handing out Namespaced, which is a wrong answer that looks
			// exactly like a right one.
			return nil, errors.Errorf("kinds: no scope resolved for %s (%s)", all[i].Key(), all[i].GVK)
		}
		all[i].Namespaced = scope != markers.ScopeCluster
	}
	return all, nil
}

// classify filters and routes the scheme's known types into Kinds. The scope
// is left unset; [derive] fills it in from the upstream sources.
func classify(known map[schema.GroupVersionKind]reflect.Type) ([]Kind, error) {
	clientObject := reflect.TypeOf((*client.Object)(nil)).Elem()
	clientObjectList := reflect.TypeOf((*client.ObjectList)(nil)).Elem()
	var out []Kind
	for gvk, typ := range known {
		// A list kind is one whose type is a list (it carries ListMeta), not
		// one whose Kind happens to be spelled with a List suffix: a singular
		// resource named that way must still get a wrapper.
		if reflect.PointerTo(typ).Implements(clientObjectList) {
			continue
		}
		if !reflect.PointerTo(typ).Implements(clientObject) {
			continue
		}
		if strings.HasPrefix(typ.PkgPath(), "k8s.io/apimachinery/") {
			continue
		}
		k := Kind{GVK: gvk, Type: typ, ImportPath: typ.PkgPath(), TypeName: typ.Name()}
		pkg, ok := routePackage(k.ImportPath)
		if !ok {
			return nil, errors.Errorf("kinds: no kure package routes import path %s (kind %s); add it to packageRoutes", k.ImportPath, gvk)
		}
		k.Package = pkg
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].sortKey() < out[j].sortKey() })
	return out, nil
}

// sortKey orders kinds by package, kind, group, version.
func (k Kind) sortKey() string {
	return strings.Join([]string{k.Package, k.GVK.Kind, k.GVK.Group, k.GVK.Version}, "\x00")
}

func routePackage(importPath string) (string, bool) {
	for _, r := range packageRoutes {
		if strings.HasPrefix(importPath, r.prefix) {
			return r.pkg, true
		}
	}
	return "", false
}

// set builds a lookup set from group/kind keys.
func set(keys ...string) map[string]bool {
	m := make(map[string]bool, len(keys))
	for _, k := range keys {
		m[k] = true
	}
	return m
}
