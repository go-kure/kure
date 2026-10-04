package argocd

import (
	"slices"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/go-kure/kure/pkg/stack"
	"github.com/go-kure/kure/pkg/stack/layout"
)

// TestGenerateFromCluster_KustomizationName: under the ArgoCD workflow
// Bundle.KustomizationName names the bundle's Application, a dependant's
// spec.dependencies names it the same way, and source.path stays the
// bundle's directory.
func TestGenerateFromCluster_KustomizationName(t *testing.T) {
	db := &stack.Bundle{Name: "db", KustomizationName: "db-cr"}
	web := &stack.Bundle{Name: "web", DependsOn: []*stack.Bundle{db}}
	dbn := &stack.Node{Name: "dbn", Bundle: db}
	webn := &stack.Node{Name: "webn", Bundle: web}
	r := &stack.Node{Name: "r", Children: []*stack.Node{dbn, webn}}
	dbn.SetParent(r)
	webn.SetParent(r)

	objs, err := Engine().GenerateFromCluster(&stack.Cluster{Name: "demo", Node: r}, layout.DefaultLayoutRules())
	if err != nil {
		t.Fatalf("GenerateFromCluster: %v", err)
	}
	want := map[string]string{"db-cr": "r/dbn", "web": "r/webn"}
	if got := appPaths(t, objs); !pathsEqual(got, want) {
		t.Errorf("Application paths = %v, want %v", got, want)
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
			t.Errorf("web spec.dependencies = %v, want [db-cr]", deps)
		}
	}
}

// TestApplicationForBundle_KustomizationName: the one-bundle builder names
// the Application and its dependencies the same way.
func TestApplicationForBundle_KustomizationName(t *testing.T) {
	db := &stack.Bundle{Name: "db", KustomizationName: "db-cr"}
	web := &stack.Bundle{Name: "web", KustomizationName: "web-cr", DependsOn: []*stack.Bundle{db}}
	app, err := Engine().applicationForBundle(web, "apps/web")
	if err != nil {
		t.Fatalf("applicationForBundle: %v", err)
	}
	u := app.(*unstructured.Unstructured)
	if u.GetName() != "web-cr" {
		t.Errorf("Application name = %q, want web-cr", u.GetName())
	}
	deps, _, err := unstructured.NestedStringSlice(u.Object, "spec", "dependencies")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(deps, []string{"db-cr"}) {
		t.Errorf("spec.dependencies = %v, want [db-cr]", deps)
	}
}

// TestGenerateFromCluster_KustomizationNameValue: a KustomizationName is
// checked as the name of the object it names. One between 64 and 253
// characters validates and names the Application, the 63-character limit
// being Flux's; one that is no DNS-1123 subdomain is refused before anything
// is generated, with the bundle and the field.
func TestGenerateFromCluster_KustomizationNameValue(t *testing.T) {
	cluster := func(name string) *stack.Cluster {
		return &stack.Cluster{Name: "c", Node: &stack.Node{Name: "platform", Children: []*stack.Node{
			{Name: "apps", Bundle: &stack.Bundle{Name: "web", KustomizationName: name}},
		}}}
	}
	for _, n := range []int{stack.KustomizationNameMaxLength + 1, 253} {
		long := strings.Repeat("a", n)
		objs, err := Engine().GenerateFromCluster(cluster(long), layout.DefaultLayoutRules())
		if err != nil {
			t.Fatalf("GenerateFromCluster with a %d-character KustomizationName = %v, want nil", n, err)
		}
		if got := appPaths(t, objs); len(got) != 1 || got[long] == "" {
			t.Errorf("Application paths = %v, want one Application named after the %d-character KustomizationName", got, n)
		}
	}

	objs, err := Engine().GenerateFromCluster(cluster("Web-CR"), layout.DefaultLayoutRules())
	if err == nil || objs != nil {
		t.Fatalf("GenerateFromCluster = %v, %v; want the refusal of an uppercase KustomizationName and nothing to write", objs, err)
	}
	for _, want := range []string{"'web'", "field 'kustomizationName'", `"Web-CR"`, "RFC 1123 subdomain"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %s", err, want)
		}
	}
}

// TestGenerateFromCluster_DuplicateKustomizationName: two bundles whose
// Applications would get one name are refused, naming both bundles.
func TestGenerateFromCluster_DuplicateKustomizationName(t *testing.T) {
	c := &stack.Cluster{Name: "c", Node: &stack.Node{Name: "platform", Children: []*stack.Node{
		{Name: "a", Bundle: &stack.Bundle{Name: "shop-a", KustomizationName: "shop"}},
		{Name: "b", Bundle: &stack.Bundle{Name: "shop-b", KustomizationName: "shop"}},
	}}}
	_, err := Engine().GenerateFromCluster(c, layout.DefaultLayoutRules())
	if err == nil {
		t.Fatal("GenerateFromCluster accepted two bundles with one Application name")
	}
	for _, want := range []string{`"shop-a"`, `"shop-b"`, `"shop"`} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %s", err, want)
		}
	}
}
