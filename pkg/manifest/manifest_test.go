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
