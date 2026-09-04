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
// known namespaced type that must declare metadata.namespace.
//
// The answer comes from [kubernetes.IsNamespaced], the table derived from the
// pinned upstream sources, so it covers every kind kure registers rather than a
// list maintained here. That is wider than the "builtin" in the name suggests:
// a namespaced CRD kind kure registers now answers true, which is the correct
// answer to the question the function asks. An unregistered kind answers false,
// as before — false means "not known to be namespaced", never "cluster-scoped".
func IsNamespacedBuiltinKind(apiVersion, kind string) bool {
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

// Scope determines whether o is namespaced, cluster-scoped, or unknown. CRDs
// are cluster-scoped; a kind kure registers takes its scope from the generated
// table (derived from the pinned upstream sources — see
// pkg/kubernetes/README.md § 9); the few cluster-scoped kinds kure does not
// register are named above; and any remaining custom resource is resolved from
// crdScopes (the spec.scope of CRDs known in the same context). Anything else
// is ScopeUnknown, which callers fail closed on rather than guess.
func Scope(o client.Object, crdScopes map[schema.GroupKind]apiextv1.ResourceScope) ScopeResult {
	if IsCRD(o) {
		return ScopeCluster
	}
	gvk := o.GetObjectKind().GroupVersionKind()
	if namespaced, known := kubernetes.IsNamespaced(gvk.GroupVersion().String(), gvk.Kind); known {
		if namespaced {
			return ScopeNamespaced
		}
		return ScopeCluster
	}
	if clusterScopedUnregisteredKinds[gvk.Group+"/"+gvk.Kind] {
		return ScopeCluster
	}
	if scope, ok := crdScopes[schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}]; ok {
		if scope == apiextv1.ClusterScoped {
			return ScopeCluster
		}
		return ScopeNamespaced
	}
	return ScopeUnknown
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
