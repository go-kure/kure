package manifest

import (
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
)

// IsCRD reports whether o is a CustomResourceDefinition, by type or GVK. Unlike
// CRDDefinedGroupKind it does not require spec.group/spec.names to be populated —
// an object is a CRD by virtue of its kind, even if its defined GroupKind can't
// be read.
func IsCRD(o client.Object) bool {
	if _, ok := o.(*apiextv1.CustomResourceDefinition); ok {
		return true
	}
	gvk := o.GetObjectKind().GroupVersionKind()
	return gvk.Group == "apiextensions.k8s.io" && gvk.Kind == "CustomResourceDefinition"
}

// CRDDefinedGroupKind returns the GroupKind a CustomResourceDefinition defines
// (spec.group + spec.names.kind) and whether o is a CRD.
func CRDDefinedGroupKind(o client.Object) (schema.GroupKind, bool) {
	switch c := o.(type) {
	case *apiextv1.CustomResourceDefinition:
		return schema.GroupKind{Group: c.Spec.Group, Kind: c.Spec.Names.Kind}, true
	case *unstructured.Unstructured:
		gvk := c.GroupVersionKind()
		if gvk.Kind != "CustomResourceDefinition" || gvk.Group != "apiextensions.k8s.io" {
			return schema.GroupKind{}, false
		}
		group, _, _ := unstructured.NestedString(c.Object, "spec", "group")
		kind, _, _ := unstructured.NestedString(c.Object, "spec", "names", "kind")
		if group == "" || kind == "" {
			return schema.GroupKind{}, false
		}
		return schema.GroupKind{Group: group, Kind: kind}, true
	}
	return schema.GroupKind{}, false
}

// ObjectGroupKind is the GroupKind of an emitted object.
func ObjectGroupKind(o client.Object) schema.GroupKind {
	gvk := o.GetObjectKind().GroupVersionKind()
	return schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}
}

// clusterScopedUnregisteredKinds lists the cluster-scoped kinds this package
// must recognise that the generated table cannot answer for, because kure's
// scheme does not register them and so nothing derives their scope from an
// upstream source. Without them an APIService or a PriorityClass would come
// back ScopeUnknown, and a caller failing closed would demand a
// metadata.namespace on an object that must not carry one.
//
// This set only shrinks. Every entry is a kind kure has no builders for;
// registering one moves its scope to the derived table, and
// TestClusterScopedUnregisteredKindsAreNotInTheGeneratedTable fires so the
// entry is removed rather than left behind as a second, competing answer.
var clusterScopedUnregisteredKinds = map[string]bool{
	"scheduling.k8s.io/PriorityClass":                             true,
	"apiregistration.k8s.io/APIService":                           true,
	"admissionregistration.k8s.io/ValidatingWebhookConfiguration": true,
	"admissionregistration.k8s.io/MutatingWebhookConfiguration":   true,
}

// IsNamespacedBuiltinKind reports whether a (group-aware) apiVersion+kind is a
// known namespaced built-in type that must declare metadata.namespace.
//
// Built-in means what the name says: a kind whose scope the Kubernetes API
// itself defines. The generated table records what declared each kind's scope,
// so that restriction is derived rather than kept as a list here — a kind whose
// scope came from a +kubebuilder:resource marker or from a shipped
// CustomResourceDefinition is a custom resource and answers false, whatever its
// scope. [IsNamespacedKind] asks the same question without the restriction, and
// is what a caller reasoning about every kind kure registers wants.
//
// Deriving the restriction widens the answer within the built-ins: the
// hand-kept map this replaced listed twelve namespaced built-in kinds, and the
// table knows every one the scheme registers. That is the same contract, more
// completely implemented.
//
// false means "not a known namespaced built-in", never "cluster-scoped".
func IsNamespacedBuiltinKind(apiVersion, kind string) bool {
	k, ok := kubernetes.KindForAnyVersion(apiVersion, kind)
	return ok && k.ScopeSource == kubernetes.ScopeSourceBuiltin && k.Namespaced
}

// IsNamespacedKind reports whether a (group-aware) apiVersion+kind is a kind
// kure registers and knows to be namespaced, built-in or custom resource
// alike.
//
// This is the general form of the scope question for callers that hold an
// apiVersion and kind rather than an object; [Scope] answers it for an object,
// and also for the unregistered kinds the residual sets and crdScopes cover.
// An unregistered kind answers false, which means "not known to be namespaced",
// never "cluster-scoped" — a caller that must tell those apart uses [Scope] or
// [kubernetes.IsNamespaced], both of which report whether the kind was known.
func IsNamespacedKind(apiVersion, kind string) bool {
	namespaced, known := kubernetes.IsNamespaced(apiVersion, kind)
	return known && namespaced
}

// ScopeResult is the determined namespacing of an object.
type ScopeResult int

const (
	// ScopeUnknown means the object's scope could not be determined (unknown
	// custom resource with no defining CRD in scope) — callers should fail
	// closed rather than guess.
	ScopeUnknown ScopeResult = iota
	ScopeNamespaced
	ScopeCluster
)

// Scope determines whether o is namespaced, cluster-scoped, or unknown, in this
// order:
//
//  1. A CustomResourceDefinition is cluster-scoped.
//  2. A built-in kind takes its scope from the generated table, derived from the
//     pinned upstream sources — see pkg/kubernetes/README.md § 9 — as do the few
//     cluster-scoped built-ins kure does not register, named above. The
//     Kubernetes API defines those scopes and no manifest can redefine them.
//  3. A custom resource takes its scope from crdScopes when a CRD in the same
//     context defines it. That definition governs even for a kind kure
//     registers: the CRD names the scope the target cluster will actually
//     serve, while kure's table only says what the module pinned at build time
//     declared, and a consumer shipping a different release of that operator is
//     entitled to differ.
//  4. Failing that, a kind kure registers falls back to the table.
//  5. Anything else is ScopeUnknown, which callers fail closed on rather than
//     guess.
func Scope(o client.Object, crdScopes map[schema.GroupKind]apiextv1.ResourceScope) ScopeResult {
	if IsCRD(o) {
		return ScopeCluster
	}
	gvk := o.GetObjectKind().GroupVersionKind()
	k, registered := kubernetes.KindForAnyVersion(gvk.GroupVersion().String(), gvk.Kind)
	if registered && k.ScopeSource == kubernetes.ScopeSourceBuiltin {
		return scopeOf(k.Namespaced)
	}
	if clusterScopedUnregisteredKinds[gvk.Group+"/"+gvk.Kind] {
		return ScopeCluster
	}
	if scope, ok := crdScopes[schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}]; ok {
		return scopeOf(scope != apiextv1.ClusterScoped)
	}
	if registered {
		return scopeOf(k.Namespaced)
	}
	return ScopeUnknown
}

// scopeOf turns a namespaced bool into the ScopeResult for it.
func scopeOf(namespaced bool) ScopeResult {
	if namespaced {
		return ScopeNamespaced
	}
	return ScopeCluster
}

// CRDScope returns a CRD's defined GroupKind and declared scope (defaulting to
// NamespaceScoped when spec.scope is absent, matching Kubernetes). ok is false
// when o is not a CRD.
func CRDScope(o client.Object) (schema.GroupKind, apiextv1.ResourceScope, bool) {
	gk, ok := CRDDefinedGroupKind(o)
	if !ok {
		return schema.GroupKind{}, "", false
	}
	scope := apiextv1.NamespaceScoped
	switch c := o.(type) {
	case *apiextv1.CustomResourceDefinition:
		if c.Spec.Scope != "" {
			scope = c.Spec.Scope
		}
	case *unstructured.Unstructured:
		if s, _, _ := unstructured.NestedString(c.Object, "spec", "scope"); s != "" {
			scope = apiextv1.ResourceScope(s)
		}
	}
	return gk, scope, true
}
