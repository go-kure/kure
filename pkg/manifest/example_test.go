package manifest_test

import (
	"fmt"

	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/go-kure/kure/pkg/manifest"
)

// The README's Go blocks are generated from these functions
// (scripts/gen-doc-examples.sh); each prints what it found so go test checks
// the example as well as compiling it.

func ExampleCRDScope() {
	obj := &apiextv1.CustomResourceDefinition{}
	obj.SetName("widgets.example.com")
	obj.Spec.Group = "example.com"
	obj.Spec.Names.Kind = "Widget"
	obj.Spec.Scope = apiextv1.ClusterScoped

	gk, scope, ok := manifest.CRDScope(obj)
	fmt.Println(gk, scope, ok)
	// Output: Widget.example.com Cluster true
}

func ExampleScope() {
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": "my-widget"},
	}}

	// The spec.scope of the CRDs known in the same context
	crdScopes := map[schema.GroupKind]apiextv1.ResourceScope{
		{Group: "example.com", Kind: "Widget"}: apiextv1.NamespaceScoped,
	}

	switch manifest.Scope(obj, crdScopes) {
	case manifest.ScopeNamespaced:
		// must declare metadata.namespace
		fmt.Println("namespaced")
	case manifest.ScopeCluster:
		// cluster-scoped
		fmt.Println("cluster-scoped")
	case manifest.ScopeUnknown:
		// unknown custom resource with no defining CRD in scope
		fmt.Println("unknown")
	}
	// Output: namespaced
}
