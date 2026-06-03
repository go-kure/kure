package manifest

import (
	"strings"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// namespacedBuiltinKinds lists the namespaced (group, kind) pairs we recognize,
// keyed "<group>/<kind>" (core group is the empty string). A kind not listed
// here is treated as unknown scope (callers then fail closed unless
// metadata.namespace is set), rather than being silently widened.
var namespacedBuiltinKinds = map[string]bool{
	"/ConfigMap":                            true,
	"/Secret":                               true,
	"/Service":                              true,
	"/ServiceAccount":                       true,
	"/PersistentVolumeClaim":                true,
	"/Pod":                                  true,
	"apps/Deployment":                       true,
	"apps/DaemonSet":                        true,
	"apps/StatefulSet":                      true,
	"apps/ReplicaSet":                       true,
	"batch/Job":                             true,
	"batch/CronJob":                         true,
	"networking.k8s.io/Ingress":             true,
	"networking.k8s.io/NetworkPolicy":       true,
	"rbac.authorization.k8s.io/Role":        true,
	"rbac.authorization.k8s.io/RoleBinding": true,
}

// clusterScopedBuiltinKinds lists the cluster-scoped built-in kinds we
// recognize, so an object like a Namespace is treated as cluster-scoped rather
// than unknown (which would otherwise require an explicit metadata.namespace).
var clusterScopedBuiltinKinds = map[string]bool{
	"/Namespace":                            true,
	"/Node":                                 true,
	"/PersistentVolume":                     true,
	"rbac.authorization.k8s.io/ClusterRole": true,
	"rbac.authorization.k8s.io/ClusterRoleBinding":                true,
	"apiextensions.k8s.io/CustomResourceDefinition":               true,
	"storage.k8s.io/StorageClass":                                 true,
	"scheduling.k8s.io/PriorityClass":                             true,
	"networking.k8s.io/IngressClass":                              true,
	"apiregistration.k8s.io/APIService":                           true,
	"admissionregistration.k8s.io/ValidatingWebhookConfiguration": true,
	"admissionregistration.k8s.io/MutatingWebhookConfiguration":   true,
}

func groupKey(apiVersion, kind string) string {
	group := ""
	if g, _, ok := strings.Cut(apiVersion, "/"); ok {
		group = g // "apps/v1" -> "apps"; core "v1" -> ""
	}
	return group + "/" + kind
}

// IsNamespacedBuiltinKind reports whether a (group-aware) apiVersion+kind is a
// known namespaced built-in type that must declare metadata.namespace.
func IsNamespacedBuiltinKind(apiVersion, kind string) bool {
	return namespacedBuiltinKinds[groupKey(apiVersion, kind)]
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

// Scope determines whether o is namespaced, cluster-scoped, or unknown. CRDs are
// cluster-scoped; built-in kinds use the namespaced/cluster maps; custom
// resources are resolved from crdScopes (the spec.scope of CRDs known in the
// same context). Anything else is ScopeUnknown.
func Scope(o client.Object, crdScopes map[schema.GroupKind]apiextv1.ResourceScope) ScopeResult {
	if IsCRD(o) {
		return ScopeCluster
	}
	gvk := o.GetObjectKind().GroupVersionKind()
	key := gvk.Group + "/" + gvk.Kind
	switch {
	case namespacedBuiltinKinds[key]:
		return ScopeNamespaced
	case clusterScopedBuiltinKinds[key]:
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
