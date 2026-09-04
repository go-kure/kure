package manifest

import (
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/go-kure/kure/pkg/kubernetes"
)

func typedCRD(group, kind string) client.Object {
	crd := &apiextv1.CustomResourceDefinition{
		TypeMeta:   metav1.TypeMeta{Kind: "CustomResourceDefinition", APIVersion: "apiextensions.k8s.io/v1"},
		ObjectMeta: metav1.ObjectMeta{Name: kind + "s." + group},
	}
	crd.Spec.Group = group
	crd.Spec.Names.Kind = kind
	return crd
}

func unstructuredObj(apiVersion, kind, name string) client.Object {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(apiVersion)
	u.SetKind(kind)
	u.SetName(name)
	return u
}

func TestIsCRD(t *testing.T) {
	if !IsCRD(typedCRD("example.com", "Widget")) {
		t.Error("typed CRD should be recognized")
	}
	if !IsCRD(unstructuredObj("apiextensions.k8s.io/v1", "CustomResourceDefinition", "widgets.example.com")) {
		t.Error("unstructured CRD should be recognized")
	}
	if IsCRD(&appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"}}) {
		t.Error("Deployment is not a CRD")
	}
}

func TestCRDDefinedGroupKind(t *testing.T) {
	gk, ok := CRDDefinedGroupKind(typedCRD("example.com", "Widget"))
	if !ok || gk != (schema.GroupKind{Group: "example.com", Kind: "Widget"}) {
		t.Errorf("CRDDefinedGroupKind = %v,%v want example.com/Widget,true", gk, ok)
	}
	if _, ok := CRDDefinedGroupKind(&appsv1.Deployment{}); ok {
		t.Error("non-CRD should return ok=false")
	}
}

func TestIsNamespacedBuiltinKind(t *testing.T) {
	if !IsNamespacedBuiltinKind("apps/v1", "Deployment") {
		t.Error("apps/v1 Deployment is namespaced")
	}
	if !IsNamespacedBuiltinKind("v1", "ConfigMap") {
		t.Error("core ConfigMap is namespaced")
	}
	if IsNamespacedBuiltinKind("v1", "Namespace") {
		t.Error("Namespace is cluster-scoped, not in the namespaced set")
	}
	// A CRD kind is not a built-in however its scope is declared. Both
	// directions matter: answering true here would report a custom resource as
	// a built-in, which is what the function's name promises it does not do.
	if IsNamespacedBuiltinKind("cilium.io/v2", "CiliumNetworkPolicy") {
		t.Error("a namespaced CRD kind is not a built-in")
	}
	if IsNamespacedBuiltinKind("cilium.io/v2", "CiliumClusterwideNetworkPolicy") {
		t.Error("a cluster-scoped CRD kind is not a built-in")
	}
	// An unregistered kind answers false: false is "not known to be a
	// namespaced built-in", never "cluster-scoped".
	if IsNamespacedBuiltinKind("example.com/v1", "Widget") {
		t.Error("an unregistered kind must not answer true")
	}
	// The version is not part of the match: scope does not vary between the
	// versions of one group/kind, and neither does what declared it.
	if !IsNamespacedBuiltinKind("apps/v99", "Deployment") {
		t.Error("scope is version-insensitive, so an unregistered version of a registered kind still answers")
	}
}

// The general form: the same question over every kind kure registers, which is
// where a namespaced CRD kind does answer true.
func TestIsNamespacedKind(t *testing.T) {
	if !IsNamespacedKind("apps/v1", "Deployment") {
		t.Error("apps/v1 Deployment is namespaced")
	}
	if !IsNamespacedKind("cilium.io/v2", "CiliumNetworkPolicy") {
		t.Error("a registered namespaced CRD kind is namespaced")
	}
	if IsNamespacedKind("cilium.io/v2", "CiliumClusterwideNetworkPolicy") {
		t.Error("a registered cluster-scoped CRD kind is not namespaced")
	}
	if IsNamespacedKind("v1", "Namespace") {
		t.Error("Namespace is cluster-scoped")
	}
	if IsNamespacedKind("example.com/v1", "Widget") {
		t.Error("an unregistered kind must not answer true")
	}
}

// The residual set exists only for cluster-scoped kinds the generated table
// cannot answer for. Once kure registers one, the table answers and the entry
// here becomes a second source that can silently disagree — so it must go.
func TestClusterScopedUnregisteredKindsAreNotInTheGeneratedTable(t *testing.T) {
	if len(clusterScopedUnregisteredKinds) == 0 {
		return
	}
	for key := range clusterScopedUnregisteredKinds {
		if _, ok := kubernetes.KindByGroupKind(key); ok {
			t.Errorf("%s is now registered; drop it from clusterScopedUnregisteredKinds and let the derived table answer", key)
		}
	}
}

func TestCRDScope(t *testing.T) {
	// typed CRD with no spec.scope defaults to NamespaceScoped.
	gk, scope, ok := CRDScope(typedCRD("example.com", "Widget"))
	if !ok || gk.Kind != "Widget" || scope != apiextv1.NamespaceScoped {
		t.Errorf("typed CRD scope = %v,%v,%v want Widget,Namespaced,true", gk, scope, ok)
	}
	// unstructured CRD declaring Cluster scope.
	u := &unstructured.Unstructured{}
	u.SetAPIVersion("apiextensions.k8s.io/v1")
	u.SetKind("CustomResourceDefinition")
	u.SetName("clusters.example.com")
	_ = unstructured.SetNestedField(u.Object, "example.com", "spec", "group")
	_ = unstructured.SetNestedField(u.Object, "Cluster", "spec", "names", "kind")
	_ = unstructured.SetNestedField(u.Object, "Cluster", "spec", "scope")
	if _, scope, ok := CRDScope(u); !ok || scope != apiextv1.ClusterScoped {
		t.Errorf("unstructured CRD scope = %v,%v want Cluster,true", scope, ok)
	}
	if _, _, ok := CRDScope(&appsv1.Deployment{}); ok {
		t.Error("non-CRD should return ok=false")
	}
}

func TestObjectGroupKind(t *testing.T) {
	gk := ObjectGroupKind(unstructuredObj("example.com/v1", "Widget", "w"))
	if gk != (schema.GroupKind{Group: "example.com", Kind: "Widget"}) {
		t.Errorf("ObjectGroupKind = %v", gk)
	}
}

func TestScope(t *testing.T) {
	crdScopes := map[schema.GroupKind]apiextv1.ResourceScope{
		{Group: "example.com", Kind: "Widget"}:  apiextv1.NamespaceScoped,
		{Group: "example.com", Kind: "Cluster"}: apiextv1.ClusterScoped,
	}
	cases := []struct {
		name string
		obj  client.Object
		want ScopeResult
	}{
		{"crd-is-cluster", typedCRD("example.com", "Widget"), ScopeCluster},
		{"builtin-namespaced", &appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"}}, ScopeNamespaced},
		{"builtin-cluster", &corev1.Namespace{TypeMeta: metav1.TypeMeta{Kind: "Namespace", APIVersion: "v1"}}, ScopeCluster},
		{"cr-namespaced-from-crd", unstructuredObj("example.com/v1", "Widget", "w1"), ScopeNamespaced},
		{"cr-cluster-from-crd", unstructuredObj("example.com/v1", "Cluster", "c1"), ScopeCluster},
		{"unknown-gvk", unstructuredObj("unknown.io/v1", "Mystery", "m1"), ScopeUnknown},
	}
	for _, tc := range cases {
		if got := Scope(tc.obj, crdScopes); got != tc.want {
			t.Errorf("%s: Scope = %v, want %v", tc.name, got, tc.want)
		}
	}
}

// A CRD in the same context defines the scope its cluster will serve. For a
// custom resource that must win over the pinned table, which only records what
// the module kure built against declared — a consumer shipping a different
// release of that operator is entitled to differ, and answering from the pin
// would emit a namespace on a cluster-scoped object or drop one from a
// namespaced object with nothing to point at. A built-in is the other way
// round: the Kubernetes API defines its scope and a manifest cannot redefine it,
// so a CRD claiming otherwise is ignored rather than obeyed.
func TestScopeLetsASuppliedCRDGovernCustomResources(t *testing.T) {
	registered := unstructuredObj("cilium.io/v2", "CiliumNetworkPolicy", "cnp")
	if got := Scope(registered, nil); got != ScopeNamespaced {
		t.Fatalf("with no CRD supplied the table answers: Scope = %v, want %v", got, ScopeNamespaced)
	}
	disagreeing := map[schema.GroupKind]apiextv1.ResourceScope{
		{Group: "cilium.io", Kind: "CiliumNetworkPolicy"}: apiextv1.ClusterScoped,
	}
	if got := Scope(registered, disagreeing); got != ScopeCluster {
		t.Errorf("a supplied CRD must govern a custom resource: Scope = %v, want %v", got, ScopeCluster)
	}

	builtin := &appsv1.Deployment{TypeMeta: metav1.TypeMeta{Kind: "Deployment", APIVersion: "apps/v1"}}
	claimingBuiltin := map[schema.GroupKind]apiextv1.ResourceScope{
		{Group: "apps", Kind: "Deployment"}: apiextv1.ClusterScoped,
	}
	if got := Scope(builtin, claimingBuiltin); got != ScopeNamespaced {
		t.Errorf("a built-in's scope is not a CRD's to redefine: Scope = %v, want %v", got, ScopeNamespaced)
	}

	// The same for the cluster-scoped built-ins kure does not register: they
	// are named in the residual set for the same reason, and a CRD entry must
	// not move them either.
	priorityClass := unstructuredObj("scheduling.k8s.io/v1", "PriorityClass", "pc")
	claimingResidual := map[schema.GroupKind]apiextv1.ResourceScope{
		{Group: "scheduling.k8s.io", Kind: "PriorityClass"}: apiextv1.NamespaceScoped,
	}
	if got := Scope(priorityClass, claimingResidual); got != ScopeCluster {
		t.Errorf("the residual set outranks a CRD entry too: Scope = %v, want %v", got, ScopeCluster)
	}
}
