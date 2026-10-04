package argocd

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/stack"
)

// TestGenerateFromCluster_DependencyCopy: a DependsOn bundle that is a copy of
// a bundle of the cluster names the Application that bundle gets. A copy with
// the bundle's KustomizationName or with none resolves to it; a copy that sets
// another one is refused, naming the dependant, the bundle and both names.
func TestGenerateFromCluster_DependencyCopy(t *testing.T) {
	cluster := func(copyName string) *stack.Cluster {
		db := &stack.Bundle{Name: "db", KustomizationName: "db-cr"}
		web := &stack.Bundle{Name: "web", DependsOn: []*stack.Bundle{{Name: "db", KustomizationName: copyName}}}
		dbn := &stack.Node{Name: "dbn", Bundle: db}
		webn := &stack.Node{Name: "webn", Bundle: web}
		r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, webn}}
		dbn.SetParent(r)
		webn.SetParent(r)
		return &stack.Cluster{Name: "demo", Node: r}
	}

	for _, copyName := range []string{"db-cr", ""} {
		objs, err := Engine().GenerateFromCluster(cluster(copyName))
		if err != nil {
			t.Fatalf("copy with KustomizationName %q: GenerateFromCluster: %v", copyName, err)
		}
		for _, o := range objs {
			u := o.(*unstructured.Unstructured)
			if u.GetName() != "web" {
				continue
			}
			deps, _, err := unstructured.NestedStringSlice(u.Object, "spec", "dependencies")
			if err != nil {
				t.Fatal(err)
			}
			if !slices.Equal(deps, []string{"db-cr"}) {
				t.Errorf("copy with KustomizationName %q: web spec.dependencies = %v, want [db-cr]", copyName, deps)
			}
		}
	}

	objs, err := Engine().GenerateFromCluster(cluster("x"))
	if err == nil {
		t.Fatal("GenerateFromCluster accepted a dependency copy that names another Application than the bundle")
	}
	for _, want := range []string{`bundle "web"`, `copy of bundle "db"`, `KustomizationName "x"`, `KustomizationName "db-cr"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %s", err, want)
		}
	}
	if objs != nil {
		t.Errorf("got %d objects next to the error", len(objs))
	}
}
